package threadcoll

import (
	"errors"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrOnlyObjectAreSupported = errors.New("only objects are supported as elements")
	ErrObjectAlreadyHaveURL   = errors.New("object already have a URL")
)

func init() {
	//core.RegisterLoadFreeEntityFn(reflect.TypeOf((*ThreadPattern)(nil)), loadSet)

	core.RegisterDefaultPattern(MSG_THREAD_PATTERN.Name, MSG_THREAD_PATTERN)
	core.RegisterDefaultPattern(MSG_THREAD_PATTERN_PATTERN.Name, MSG_THREAD_PATTERN_PATTERN)
	core.RegisterPatternDeserializer(MSG_THREAD_PATTERN_PATTERN, DeserializeMessageThreadPattern)
}

type MessageThread struct {
	config            ThreadConfig
	lock              core.SmartLock
	elements          []internalElement //insertion order is preserved.
	pendingInclusions []pendingInclusions

	url      core.URL
	urlAsDir core.URL

	//TODO: only load last elements + support search
}

type internalElement struct {
	actualElement *core.Object
	txID          core.ULID //zero if not added by a transaction
	ulid          core.ULID
	commitedTx    bool //true if the transaction is commited or if not added by a transaciton
}

func newEmptyThread(ctx *core.Context, url core.URL, config ThreadConfig) *MessageThread {
	if url == "" {
		panic(errors.New("empty URL provided to initialize thread"))
	}

	if config.Element == nil {
		panic(errors.New("missing .Element in thread configuration"))
	}

	thread := &MessageThread{
		config:   config,
		url:      url,
		urlAsDir: url.ToDirURL(),
	}

	thread.Share(ctx.GetClosestState())

	return thread
}

func (t *MessageThread) URL() (core.URL, bool) {
	return t.url, true
}
func (set *MessageThread) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}
func (t *MessageThread) Add(ctx *core.Context, elem *core.Object) {
	closestState := ctx.GetClosestState()
	t.lock.Lock(closestState, t)
	defer t.lock.Unlock(closestState, t)

	t.addNoLock(ctx, ctx.GetTx(), elem)
}

func (t *MessageThread) addNoLock(ctx *core.Context, tx *core.Transaction, e *core.Object) {
	_, ok := e.URL()
	if ok {
		panic(ErrObjectAlreadyHaveURL)
	}

	elemULID := core.NewULID()
	elemURL := t.urlAsDir.AppendAbsolutePath(core.Path("/" + elemULID.String()))
	err := e.SetURLOnce(ctx, elemURL)
	if err != nil {
		panic(err)
	}

	t.elements = append(t.elements, internalElement{
		actualElement: e,
		ulid:          elemULID,
		commitedTx:    tx == nil,
		txID:          utils.If(tx == nil, core.ULID{}, tx.ID()),
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
