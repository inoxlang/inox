package threadcoll

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/oklog/ulid/v2"
)

var (
	ErrOnlyObjectAreSupported = errors.New("only objects are supported as elements")
	ErrObjectAlreadyHaveURL   = errors.New("object already have a URL")

	_ core.Collection           = (*MessageThread)(nil)
	_ core.PotentiallySharable  = (*MessageThread)(nil)
	_ core.SerializableIterable = (*MessageThread)(nil)
	_ core.MigrationCapable     = (*MessageThread)(nil)
)

func init() {
	core.RegisterLoadFreeEntityFn(reflect.TypeOf((*ThreadPattern)(nil)), loadThread)

	core.RegisterDefaultPattern(MSG_THREAD_PATTERN.Name, MSG_THREAD_PATTERN)
	core.RegisterDefaultPattern(MSG_THREAD_PATTERN_PATTERN.Name, MSG_THREAD_PATTERN_PATTERN)
	core.RegisterPatternDeserializer(MSG_THREAD_PATTERN_PATTERN, DeserializeMessageThreadPattern)
}

type MessageThread struct {
	config            ThreadConfig
	pattern           *ThreadPattern
	lock              core.SmartLock
	elements          []internalElement //insertion order is preserved.
	pendingInclusions []pendingInclusions

	url      core.URL
	urlAsDir core.URL

	storage core.DataStore
	path    core.Path

	//TODO: only load last elements + support search
	//Mutation handlers should be registered on elements that are not loaded initially.
	//Lazy loading support will impact several methods.
}

type internalElement struct {
	actualElement *core.Object
	txID          core.ULID //zero if not added by a transaction
	ulid          core.ULID
	commitedTx    bool //true if the transaction is commited or if not added by a transaciton
}

func newEmptyThread(ctx *core.Context, url core.URL, pattern *ThreadPattern) *MessageThread {
	if url == "" {
		panic(errors.New("empty URL provided to initialize thread"))
	}

	thread := &MessageThread{
		config:   pattern.config,
		pattern:  pattern,
		url:      url,
		urlAsDir: url.ToDirURL(),
	}

	thread.Share(ctx.GetClosestState())

	return thread
}

func (t *MessageThread) URL() (core.URL, bool) {
	return t.url, true
}

func (t *MessageThread) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}

func (t *MessageThread) GetElementByKey(ctx *core.Context, elemKey core.ElementKey) (core.Serializable, error) {
	ulid, err := ulid.Parse(string(elemKey))
	if err != nil {
		return nil, core.ErrCollectionElemNotFound
	}

	for _, e := range t.elements {
		if e.ulid == core.ULID(ulid) {
			return e.actualElement, nil
		}
	}

	return nil, core.ErrCollectionElemNotFound
}

func (t *MessageThread) Contains(ctx *core.Context, value core.Serializable) bool {
	obj, ok := value.(*core.Object)
	if !ok {
		return false
	}

	tx := ctx.GetTx()

	for _, e := range t.elements {
		if e.actualElement == obj {
			return e.commitedTx || (tx != nil && e.txID == tx.ID())
		}
	}

	return false
}

func (t *MessageThread) Add(ctx *core.Context, elem *core.Object) {
	closestState := ctx.GetClosestState()
	t.lock.Lock(closestState, t)
	defer t.lock.Unlock(closestState, t)

	t.addNoLock(ctx, ctx.GetTx(), elem, false)
}

func (t *MessageThread) addNoLock(ctx *core.Context, tx *core.Transaction, e *core.Object, init bool) {
	if init {
		tx = nil
	}
	elemURL, ok := e.URL()

	var elemULID core.ULID

	if ok {
		if init {
			if yes, _ := t.urlAsDir.IsDirOf(elemURL); !yes {
				panic(fmt.Errorf("invalid initial thread element: invalid URL: %s", elemURL))
			}
			id, err := getElementID(elemURL)
			if err != nil {
				panic(fmt.Errorf("invalid initial thread element: invalid URL: %s", elemURL))
			}
			elemULID = id
		} else {
			panic(ErrObjectAlreadyHaveURL)
		}
	} else {
		elemULID = core.NewULID()
		elemURL = t.urlAsDir.AppendAbsolutePath(core.Path("/" + elemULID.String()))
		err := e.SetURLOnce(ctx, elemURL)
		if err != nil {
			panic(err)
		}
	}

	t.elements = append(t.elements, internalElement{
		actualElement: e,
		ulid:          elemULID,
		commitedTx:    tx == nil,
		txID: func() core.ULID {
			if tx == nil {
				return core.ULID{}
			}
			return tx.ID()
		}(),
	})

	//update distance in all pending changes and search for the first element added by $tx.
	isFirstElemAddedByTx := true
	for i := range t.pendingInclusions {
		t.pendingInclusions[i].firstElemDistance++
		if tx != nil && t.pendingInclusions[i].tx == tx {
			isFirstElemAddedByTx = false
		}
	}

	if tx == nil {
		return
	}

	if isFirstElemAddedByTx {
		err := tx.OnEnd(t, t.makeTransactionEndCallback(ctx, ctx.GetClosestState()))
		if err != nil {
			panic(err)
		}

		t.pendingInclusions = append(t.pendingInclusions, pendingInclusions{
			tx:                tx,
			firstElemDistance: 0,
		})
	}
}

func (t *MessageThread) makeTransactionEndCallback(ctx *core.Context, closestState *core.GlobalState) core.TransactionEndCallbackFn {
	return func(tx *core.Transaction, success bool) {
		txID := tx.ID()

		//note: closestState is passed instead of being retrieved from ctx because ctx.GetClosestState()
		//will panic if the context is done.

		t.lock.Lock(closestState, t)
		defer t.lock.Unlock(closestState, t)

		inclusionsIndex := -1

		for i, changes := range t.pendingInclusions {
			if changes.tx == tx {
				inclusionsIndex = i
				break
			}
		}

		if inclusionsIndex < 0 {
			return
		}

		defer func() {
			//Remove the pending change information of $tx.
			t.pendingInclusions = slices.DeleteFunc(t.pendingInclusions, func(changes pendingInclusions) bool {
				return changes.tx == tx
			})
		}()

		if !success {
			//Remove the elements added by $tx.
			t.elements = slices.DeleteFunc(t.elements, func(e internalElement) bool {
				return e.txID == txID
			})

			//Update the distance (from the end) of each first element in pending inclusions.
			for itemIndex, inclusionsInfo := range t.pendingInclusions {
				if inclusionsInfo.tx == tx {
					continue
				}

				changesTxID := inclusionsInfo.tx.ID()

				//Search for the first element starting from the previous position.
				prevFirstElemIndex := max(0, len(t.elements)-inclusionsInfo.firstElemDistance)
				for i := prevFirstElemIndex; i < len(t.elements); i++ {
					if t.elements[i].txID != (core.ULID{}) && t.elements[i].txID == changesTxID {
						t.pendingInclusions[itemIndex].firstElemDistance = i
						break
					}
				}
			}

			return
		}

		firstElemDistance := t.pendingInclusions[inclusionsIndex].firstElemDistance

		for i := len(t.elements) - firstElemDistance - 1; i < len(t.elements); i++ {
			t.elements[i].commitedTx = true
		}

		//TODO: persist
	}
}

// GetElementsBefore returns a list containing up to $maxElemCount elements starting from the last message
// prior to $exclusiveEnd. The oldest element is at the end of the returned list.
func (t *MessageThread) GetElementsBefore(ctx *core.Context, exclusiveEnd core.DateTime, maxElemCount int) *core.List {
	if maxElemCount <= 0 {
		return nil
	}

	end := exclusiveEnd.AsGoTime()
	tx := ctx.GetTx()

	//TODO: when implempenting loading: if elements are old they should not be prepended to t.elements.

	for i := len(t.elements) - 1; i >= 0; i-- {
		if t.elements[i].ulid.Time().Before(end) {
			lastReturnedElemIndex := i
			oldestReturnedElemIndex := max(0, i-(maxElemCount-1))

			slice := t.elements[oldestReturnedElemIndex : lastReturnedElemIndex+1]
			elemCount := 0
			includeElem := func(e internalElement) bool {
				return e.commitedTx || e.txID == (core.ULID{}) || (tx != nil && e.txID == tx.ID())
			}

			for _, internalElem := range slice {
				if includeElem(internalElem) {
					elemCount++
				}
			}

			elements := make([]core.Serializable, 0, elemCount)

			for _, internalElem := range slice {
				if includeElem(internalElem) {
					elements = append(elements, internalElem.actualElement)
				}
			}

			slices.Reverse(elements)

			return core.NewWrappedValueListFrom(elements)
		}
	}

	return core.NewWrappedValueList()
}

func (t *MessageThread) GetElementsInTimeRange(start, end core.DateTime) []core.Value {
	//TODO
	return nil
}

type pendingInclusions struct {
	tx                *core.Transaction
	firstElemDistance int //distance from the last thread element (t.elements).
}

func getElementID(elemURL core.URL) (core.ULID, error) {
	return core.ParseULID(elemURL.GetLastPathSegment())
}
