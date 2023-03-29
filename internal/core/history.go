package internal

import (
	"sync"
	"time"

	"github.com/inox-project/inox/internal/commonfmt"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	"github.com/inox-project/inox/internal/utils"
)

const (
	DEFAULT_MAX_HISTORY_LEN = 5
)

var (
	VALUE_HISTORY_PROPNAMES = []string{"value_at", "forget_last", "last_value"}
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
	start      Date

	lock                  sync.Mutex
	maxItemCount          int
	renderCurrentToHTMLFn *InoxFunction // can be nil

	NotClonableMixin
	NoReprMixin
}

func NewValueHistory(ctx *Context, v InMemorySnapshotable, config *Object) *ValueHistory {
	current := v
	history := &ValueHistory{
		maxItemCount: DEFAULT_MAX_HISTORY_LEN,
	}

	config.ForEachEntry(func(k string, v Value) error {
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

	registerMutationCallback := func(ctx *Context) {
		var err error
		handle, err = current.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			history.AddChange(ctx, NewChange(mutation, Date(time.Now())))

			return
		}, MutationWatchingConfiguration{
			Depth: ShallowWatching,
		})

		if err != nil {
			panic(err)
		}
	}

	if dyn, ok := v.(*DynamicValue); ok {
		current = dyn.Resolve(ctx).(InMemorySnapshotable)

		dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			newVal := dyn.Resolve(ctx).(InMemorySnapshotable)
			if current != newVal {
				if handle.Valid() {
					current.RemoveMutationCallback(ctx, handle)
				}
				current = newVal
				registerMutationCallback(ctx)
			}

			return
		}, MutationWatchingConfiguration{Depth: ShallowWatching})
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

func (h *ValueHistory) ValueAt(ctx *Context, d Date) Value {
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

func (h *ValueHistory) ForgetChangesBeforeDate(ctx *Context, d Date) {
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
	h.start = lastForgottenChange.date
}

func (h *ValueHistory) indexAtOrAfterMoment(d Date) (int, bool) {
	//TODO: use binary search
	t := time.Time(d)

	for i, c := range h.changes {
		if time.Time(c.date).Equal(t) || time.Time(c.date).After(t) {
			return i, true
		}
	}

	return -1, false
}

func (h *ValueHistory) indexAtOrBeforeMoment(d Date) (int, bool) {
	//TODO: use binary search
	t := time.Time(d)

	for i := len(h.changes) - 1; i >= 0; i-- {
		c := h.changes[i]
		if time.Time(c.date).Equal(t) || time.Time(c.date).Before(t) {
			return i, true
		}
	}

	return -1, false
}

func (h *ValueHistory) IsSharable(originState *GlobalState) (bool, string) {
	return true, ""
}

func (h *ValueHistory) Share(originState *GlobalState) {

}

func (h *ValueHistory) IsShared() bool {
	return true
}

func (h *ValueHistory) ForceLock() {

}

func (h *ValueHistory) ForceUnlock() {

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
	case "last_value":
		return h.LastValue(ctx)
	}
	method, ok := h.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, h))
	}
	return method
}

func (*ValueHistory) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*ValueHistory) PropertyNames(ctx *Context) []string {
	return VALUE_HISTORY_PROPNAMES
}

// -------------------------------
