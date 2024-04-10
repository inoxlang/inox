package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_HISTORY_LEN = 5
)

var (
	VALUE_HISTORY_PROPNAMES = []string{"value_at", "forget_last", "last-value", "selected-datetime", "value-at-selection"}
)

func init() {
	RegisterSymbolicGoFunction(NewValueHistory, func(ctx *symbolic.Context, v symbolic.InMemorySnapshotable, config *symbolic.Object) *symbolic.ValueHistory {
		//TODO: check config
		return symbolic.NewValueHistory()
	})
}

// ValueHistory stores the history about a single value, it implements Value.
type ValueHistory struct {
	startValue *Snapshot
	changes    []Change
	start      DateTime

	selectedDate      DateTime
	providedSelection bool // if no date is provided the selected date is the current date

	lock                  sync.Mutex
	shared                atomic.Bool
	maxItemCount          int
	renderCurrentToHTMLFn *InoxFunction // can be nil

}

func NewValueHistory(ctx *Context, v InMemorySnapshotable, config *Object) *ValueHistory {
	current := v
	history := &ValueHistory{
		maxItemCount: DEFAULT_MAX_HISTORY_LEN,
	}

	config.ForEachEntry(func(k string, v Serializable) error {
		switch k {
		case "max-length":
			history.maxItemCount = int(v.(Int))
		case "render":
			history.renderCurrentToHTMLFn = v.(*InoxFunction)
		default:
			panic(commonfmt.FmtUnexpectedPropInArgX(k, "config"))
		}

		return nil
	})

	var handle CallbackHandle

	_ = handle

	registerMutationCallback := func(ctx *Context) {
		var err error
		handle, err = current.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			history.AddChange(ctx, NewChange(mutation, DateTime(time.Now())))

			return
		}, MutationWatchingConfiguration{
			Depth: ShallowWatching,
		})

		if err != nil {
			panic(err)
		}
	}

	history.startValue = utils.Must(TakeSnapshot(ctx, current, false))
	registerMutationCallback(ctx)

	return history
}

func (h *ValueHistory) RenderCurrentToHTMLFn() *InoxFunction {
	return h.renderCurrentToHTMLFn
}

func (h *ValueHistory) AddChange(ctx *Context, c Change) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.changes = append(h.changes, c)

	if len(h.changes) == h.maxItemCount {
		h.forgetChangesBeforeIndex(ctx, 1)
	}
}

func (h *ValueHistory) ValueAt(ctx *Context, d DateTime) Value {
	h.lock.Lock()
	defer h.lock.Unlock()

	v, err := h.startValue.InstantiateValue(ctx)
	if err != nil {
		panic(err)
	}

	index, ok := h.indexAtOrBeforeMoment(d)
	if !ok {
		return v
	}

	for i := 0; i <= index; i++ {
		if err := h.changes[i].mutation.ApplyTo(ctx, v); err != nil {
			panic(err)
		}
	}

	return v
}

func (h *ValueHistory) LastValue(ctx *Context) Value {
	h.lock.Lock()
	defer h.lock.Unlock()

	v, err := h.startValue.InstantiateValue(ctx)
	if err != nil {
		panic(err)
	}

	for _, c := range h.changes {
		if err := c.mutation.ApplyTo(ctx, v); err != nil {
			panic(err)
		}
	}

	return v
}

func (h *ValueHistory) SelectDate(ctx *Context, d DateTime) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.selectedDate = d
	h.providedSelection = true
}

func (h *ValueHistory) ValueAtSelection(ctx *Context) Value {
	if h.providedSelection {
		return h.ValueAt(ctx, h.selectedDate)
	}
	return Nil
}

func (h *ValueHistory) ForgetChangesBeforeDate(ctx *Context, d DateTime) {
	h.lock.Lock()
	defer h.lock.Unlock()

	index, ok := h.indexAtOrAfterMoment(d)

	if !ok {
		return
	}

	h.forgetChangesBeforeIndex(ctx, index)
}

func (h *ValueHistory) ForgetLast(ctx *Context) {
	h.lock.Lock()
	defer h.lock.Unlock()

	if len(h.changes) == 0 {
		return
	}

	newLength := len(h.changes) - 1
	h.changes = h.changes[:newLength]
}

func (h *ValueHistory) forgetChangesBeforeIndex(ctx *Context, index int) {
	if index == 0 {
		return
	}

	newLength := len(h.changes) - index

	for i := 0; i < index; i++ {
		newSnap, err := h.startValue.WithChangeApplied(ctx, h.changes[i])
		if err != nil {
			panic(err)
		}
		h.startValue = newSnap
	}

	lastForgottenChange := h.changes[index]

	copy(h.changes[0:], h.changes[index:])
	h.changes = h.changes[:newLength]
	h.start = lastForgottenChange.datetime
}

func (h *ValueHistory) indexAtOrAfterMoment(d DateTime) (int, bool) {
	//TODO: use binary search
	t := time.Time(d)

	for i, c := range h.changes {
		if time.Time(c.datetime).Equal(t) || time.Time(c.datetime).After(t) {
			return i, true
		}
	}

	return -1, false
}

func (h *ValueHistory) indexAtOrBeforeMoment(d DateTime) (int, bool) {
	//TODO: use binary search
	t := time.Time(d)

	for i := len(h.changes) - 1; i >= 0; i-- {
		c := h.changes[i]
		if time.Time(c.datetime).Equal(t) || time.Time(c.datetime).Before(t) {
			return i, true
		}
	}

	return -1, false
}

func (h *ValueHistory) IsSharable(originState *GlobalState) (bool, string) {
	if h.renderCurrentToHTMLFn == nil {
		return true, ""
	}
	ok, expl := h.renderCurrentToHTMLFn.IsSharable(originState)
	if ok {
		return true, ""
	}
	return false, fmt.Sprintf("value history is not sharable because rendering fn is not sharable: %s", expl)
}

func (h *ValueHistory) Share(originState *GlobalState) {
	if h.shared.CompareAndSwap(false, true) {
		if h.renderCurrentToHTMLFn != nil {
			h.renderCurrentToHTMLFn.Share(originState)
		}
	}
}

func (h *ValueHistory) IsShared() bool {
	return h.shared.Load()
}

func (h *ValueHistory) SmartLock(state *GlobalState) {

}

func (h *ValueHistory) SmartUnlock(state *GlobalState) {

}

func (h *ValueHistory) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "value_at":
		return WrapGoMethod(h.ValueAt), true
	case "forget_last":
		return WrapGoMethod(h.ForgetLast), true
	}
	return nil, false
}

func (h *ValueHistory) Prop(ctx *Context, name string) Value {
	switch name {
	case "last-value":
		return h.LastValue(ctx)
	case "value-at-selection":
		return h.ValueAtSelection(ctx)
	case "selected-datetime":
		h.lock.Lock()
		defer h.lock.Unlock()
		if h.providedSelection {
			return h.selectedDate
		}
		return Nil
	}
	method, ok := h.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, h))
	}
	return method
}

func (h *ValueHistory) SetProp(ctx *Context, name string, value Value) error {
	switch name {
	case "selected-datetime":
		date, ok := value.(DateTime)
		if !ok {
			return commonfmt.FmtFailedToSetPropXAcceptXButZProvided(name, "datetime", fmt.Sprintf("%T", value))
		}
		h.SelectDate(ctx, date)
		return nil
	}
	return ErrCannotSetProp
}

func (*ValueHistory) PropertyNames(ctx *Context) []string {
	return VALUE_HISTORY_PROPNAMES
}

// -------------------------------
