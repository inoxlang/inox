package internal

var (
	ANY_WATCHABLE = &AnyWatchable{}

	_ = []Watchable{&Object{}, &List{}, &RuneSlice{}, &DynamicValue{}}
	_ = []StreamSource{&Watcher{}}
)

// An Watchable represents a symbolic Watchable.
type Watchable interface {
	SymbolicValue
	WatcherElement() SymbolicValue
}

// An AnyWatchable represents a symbolic Watchable we do not know the concrete type.
type AnyWatchable struct {
	_ int
}

func (r *AnyWatchable) Test(v SymbolicValue) bool {
	_, ok := v.(Watchable)

	return ok
}

func (r *AnyWatchable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyWatchable) IsWidenable() bool {
	return false
}

func (r *AnyWatchable) String() string {
	return "%watchable"
}

func (r *AnyWatchable) WidestOfType() SymbolicValue {
	return &AnyWatchable{}
}

func (r *AnyWatchable) WatcherElement() SymbolicValue {
	return ANY
}

// An Watcher represents a symbolic Watcher.
type Watcher struct {
	filter Pattern //if nil matches any
	_      int
}

func NewWatcher(filter Pattern) *Watcher {
	return &Watcher{filter: filter}
}

func (r *Watcher) Test(v SymbolicValue) bool {
	it, ok := v.(*Watcher)
	if !ok {
		return false
	}
	if r.filter == nil {
		return true
	}
	return r.filter.Test(it.filter)
}

func (r *Watcher) Widen() (SymbolicValue, bool) {
	if !r.IsWidenable() {
		return nil, false
	}
	return &Watcher{}, true
}

func (r *Watcher) IsWidenable() bool {
	return r.filter != nil
}

func (r *Watcher) String() string {
	return "%watcher"
}

func (r *Watcher) WatcherElement() SymbolicValue {
	if r.filter == nil {
		return ANY
	}
	return r.filter.SymbolicValue()
}

func (r *Watcher) StreamElement() SymbolicValue {
	if r.filter == nil {
		return ANY
	}
	return r.filter.SymbolicValue()
}

func (r *Watcher) ChunkedStreamElement() SymbolicValue {
	if r.filter == nil {
		return ANY
	}
	return NewTupleOf(r.filter.SymbolicValue())
}

func (r *Watcher) WidestOfType() SymbolicValue {
	return &Watcher{}
}
