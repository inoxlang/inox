package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_WATCHABLE = &AnyWatchable{}
	ANY_WATCHER   = &Watcher{}

	_ = []Watchable{
		(*Object)(nil), (*Dictionary)(nil), (*List)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*DynamicValue)(nil),
		(*InoxFunction)(nil), (*SynchronousMessageHandler)(nil),

		(*watchableMultivalue)(nil),
	}
	_ = []StreamSource{(*Watcher)(nil)}
)

// An Watchable represents a symbolic Watchable.
type Watchable interface {
	Value
	WatcherElement() Value
}

// An AnyWatchable represents a symbolic Watchable we do not know the concrete type.
type AnyWatchable struct {
	_ int
}

func (r *AnyWatchable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Watchable)

	return ok
}

func (r *AnyWatchable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%watchable")))
}

func (r *AnyWatchable) WidestOfType() Value {
	return ANY_WATCHABLE
}

func (r *AnyWatchable) WatcherElement() Value {
	return ANY
}

// An Watcher represents a symbolic Watcher.
type Watcher struct {
	filter Pattern //if nil matches any
	//after any update make sure ANY_WATCHER is still valid

	_ int
}

func NewWatcher(filter Pattern) *Watcher {
	return &Watcher{filter: filter}
}

func (r *Watcher) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	it, ok := v.(*Watcher)
	if !ok {
		return false
	}
	if r.filter == nil {
		return true
	}
	return r.filter.Test(it.filter, state)
}

func (r *Watcher) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%watcher")))
}

func (r *Watcher) WatcherElement() Value {
	if r.filter == nil {
		return ANY
	}
	return r.filter.SymbolicValue()
}

func (r *Watcher) StreamElement() Value {
	if r.filter == nil {
		return ANY
	}
	return r.filter.SymbolicValue()
}

func (r *Watcher) ChunkedStreamElement() Value {
	if r.filter == nil {
		return ANY
	}
	return NewTupleOf(r.filter.SymbolicValue().(Serializable))
}

func (r *Watcher) WidestOfType() Value {
	return ANY_WATCHER
}
