package threadcoll

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
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

func (e internalElement) isVisibleByTx(optTx *core.Transaction) bool {
	return e.commitedTx || e.txID == (core.ULID{}) || (optTx != nil && e.txID == optTx.ID())
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
	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

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
	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

	obj, ok := value.(*core.Object)
	if !ok {
		return false
	}

	tx := ctx.GetTx()

	for _, e := range t.elements {
		if e.actualElement == obj && e.isVisibleByTx(tx) {
			return true
		}
	}

	return false
}

func (t *MessageThread) IsEmpty(ctx *core.Context) bool {
	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

	tx := ctx.GetTx()

	for _, e := range t.elements {
		if e.isVisibleByTx(tx) {
			return false //not empty
		}
	}

	return true
}

func (t *MessageThread) Add(ctx *core.Context, elem *core.Object) {
	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

	t.addNoLock(ctx, ctx.GetTx(), elem, false)
}

func (t *MessageThread) addNoLock(ctx *core.Context, tx *core.Transaction, e *core.Object, init bool) {
	if init {
		tx = nil
	}

	if tx != nil && tx.IsReadonly() {
		panic(core.ErrEffectsNotAllowedInReadonlyTransaction)
	}

	elemURL, ok := e.URL()

	var elemULID core.ULID

	if ok {
		if init {
			if yes, _ := t.urlAsDir.IsDirOf(elemURL); !yes {
				panic(fmt.Errorf("invalid initial thread element: invalid URL: %s", elemURL))
			}
			id, err := getElementIDFromURL(elemURL)
			if err != nil {
				panic(fmt.Errorf("invalid initial thread element: invalid URL: %s", elemURL))
			}
			elemULID = id
		} else {
			panic(ErrObjectAlreadyHaveURL)
		}
	} else {
		//set URL
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

		t._lock(closestState)
		defer t._unlock(closestState)

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

		if t.storage != nil {
			utils.PanicIfErr(persistThread(ctx, t, t.path, t.storage))
		}
	}
}

// GetElementsBefore returns a slice containing up to $maxElemCount elements starting from the last message
// prior to $exclusiveEnd. The oldest element is at the end of the returned slice.
func (t *MessageThread) GetElementsBefore(ctx *core.Context, exclusiveEnd core.ULID, maxElemCount int) *core.List {
	elements := t.getElementsBefore(ctx, exclusiveEnd, maxElemCount, nil)
	return core.NewWrappedValueListFrom(elements)
}

// getElementsBefore returns a slice containing up to $maxElemCount elements starting from the last message prior to $exclusiveEnd.
// The oldest element is at the end of the returned slice. If $destination is not nil the elements are added to it and nil is returned,
// $destination should have a length of zero and a capacity >= $maxElemCount.
func (t *MessageThread) getElementsBefore(
	ctx *core.Context,
	exclusiveEnd core.ULID,
	maxElemCount int,
	destination *[]internalElement,
) []core.Serializable {
	if maxElemCount <= 0 {
		return nil
	}

	if destination != nil { //store elements in $destination
		if cap(*destination) < maxElemCount {
			panic(fmt.Errorf("the capacity of the provided destination slice is too small"))
		}
		if len(*destination) != 0 {
			panic(fmt.Errorf("the length of the provided destination slice should be zero"))
		}
	}

	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

	end := exclusiveEnd
	tx := ctx.GetTx()

	//TODO: When implementing loading: if elements are old they should not be prepended to t.elements.
	//      The following logic could act on large loaded []internalElement segments instead of t.elements.

	for i := len(t.elements) - 1; i >= 0; i-- {
		if t.elements[i].ulid.Before(end) {
			var visibleElementIndices []int32

			for elemIndex := i; elemIndex >= 0; elemIndex-- {
				if t.elements[elemIndex].isVisibleByTx(tx) {
					visibleElementIndices = append(visibleElementIndices, int32(elemIndex))
				}
				if len(visibleElementIndices) == maxElemCount {
					break
				}
			}

			elemCount := len(visibleElementIndices)

			if destination != nil { //store elements in $destination
				*destination = (*destination)[:elemCount]

				for i := 0; i < len(visibleElementIndices); i++ {
					(*destination)[i] = t.elements[visibleElementIndices[i]]
				}

				return nil
			}

			elements := make([]core.Serializable, elemCount)

			for j, elemIndex := range visibleElementIndices {
				elements[j] = t.elements[elemIndex].actualElement
			}

			return elements
		}
	}

	return nil
}

func (t *MessageThread) GetElementsInTimeRange(ctx *core.Context, start, end core.DateTime) []core.Value {
	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

	//TODO
	return nil
}

type pendingInclusions struct {
	tx                *core.Transaction
	firstElemDistance int //distance from the last thread element (t.elements).
}

func getElementIDFromURL(elemURL core.URL) (core.ULID, error) {
	return core.ParseULID(elemURL.GetLastPathSegment())
}

func getElementCreationTime(elem *core.Object) (core.ULID, error) {
	url, ok := elem.URL()
	if !ok {
		return core.ULID{}, errors.New("element does not have a URL")
	}
	return getElementIDFromURL(url)
}
