package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

const (
	FIRST_VALID_CALLBACK_HANDLE = 1
)

var (
	_                               = []Watchable{&Object{}, &List{}, &RuneSlice{}, &DynamicValue{}}
	_                               = []Watcher{stoppedWatcher{}, &GenericWatcher{}, &joinedWatchers{}, &PeriodicWatcher{}}
	periodicWatcherGoroutineStarted = atomic.Bool{}
	periodicWatcherSubscribeChan    = make(chan *PeriodicWatcher)
	periodicWatcherUnsuscribeChan   = make(chan *PeriodicWatcher)
	subscriptionAckWaitGroup        = new(sync.WaitGroup)
	subscriptionAckWaitGroupLock    = sync.Mutex{}

	PERIODIC_WATCHER_GOROUTINE_TICK_INTERVAL = 100 * time.Microsecond

	ErrManagedWatchersNotSupported           = errors.New("managed watchers are not supported")
	ErrWatchTimeout                          = errors.New("watch timeout")
	ErrStoppedWatcher                        = errors.New("stopped watcher")
	ErrIntermediateDepthWatchingNotSupported = errors.New("intermediate (and deeper) watching is not supported by the watchable")
	ErrDeepWatchingNotSupported              = errors.New("deep watching is not supported by the watchable")
)

func init() {
	RegisterSymbolicGoFunction(WatchReceivedMessages, func(ctx *symbolic.Context, watchable symbolic.Watchable) *symbolic.Watcher {
		pattern, _ := MSG_PATTERN.ToSymbolicValue(nil, nil)
		return symbolic.NewWatcher(pattern.(symbolic.Pattern))
	})
}

type Watchable interface {
	Value

	// Watcher creates a watcher managed by the watched value, callers should only call the .WaitNext & .Stop methods,
	// if watching depth is unspecified the watched value is free to use any depth as long as it consistent with .OnMutation.
	Watcher(*Context, WatcherConfiguration) Watcher

	//OnMutation registers a microtask to be called on mutations, the mutations should be same as the one returned by the watcher.
	OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (handle CallbackHandle, err error)

	RemoveMutationCallback(ctx *Context, handle CallbackHandle)

	RemoveMutationCallbackMicrotasks(ctx *Context)
}

// see FIRST_VALID_CALLBACK_HANDLE
type CallbackHandle int

func (h CallbackHandle) Valid() bool {
	return h >= FIRST_VALID_CALLBACK_HANDLE
}

type MutationCallbackMicrotask func(
	ctx *Context,
	mutation Mutation, //relocalized mutation
) (registerAgain bool)

type Watcher interface {
	Value
	StreamSource

	// WaitNext should be called by a single goroutine, filter can be nil
	WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error)
	Stop()
	IsStopped() bool
	Config() WatcherConfiguration

	// InformAboutAsync is called by the watched value, InformAboutAsync should be thread safe and should inform
	// asynchronously the user of the watcher about the new value during the current/next call to WaitNext.
	InformAboutAsync(ctx *Context, v Value)
}

type WatcherConfiguration struct {
	Filter Pattern
	Depth  WatchingDepth
	Path   Path
}

type MutationWatchingConfiguration struct {
	Depth WatchingDepth
}

type WatchingDepth int

const (
	UnspecifiedWatchingDepth WatchingDepth = iota
	ShallowWatching
	IntermediateDepthWatching
	DeepWatching
)

func (d WatchingDepth) MinusOne() (WatchingDepth, bool) {
	switch d {
	case ShallowWatching, UnspecifiedWatchingDepth:
		return -1, false
	case IntermediateDepthWatching:
		return ShallowWatching, true
	case DeepWatching:
		return DeepWatching, true
	default:
		panic(ErrUnreachable)
	}
}

func (d WatchingDepth) Plus(n uint) (WatchingDepth, bool) {
	switch d {
	case UnspecifiedWatchingDepth:
		return -1, false
	case ShallowWatching, IntermediateDepthWatching, DeepWatching:
		break
	default:
		panic(ErrUnreachable)
	}

	new := d + WatchingDepth(n)

	switch new {
	case ShallowWatching, IntermediateDepthWatching, DeepWatching:
		return new, true
	default:
		if new > DeepWatching {
			return DeepWatching, true
		}
		panic(ErrUnreachable)
	}
}

func (d WatchingDepth) MustMinusOne() WatchingDepth {
	less, ok := d.MinusOne()
	if !ok {
		panic(errors.New("failed to get .MinusOne() of a watching depth"))
	}
	return less
}

func (d WatchingDepth) IsSpecified() bool {
	return d > UnspecifiedWatchingDepth
}

type stoppedWatcher struct {
	NotClonableMixin

	config WatcherConfiguration
}

func NewStoppedWatcher(config WatcherConfiguration) stoppedWatcher {
	return stoppedWatcher{config: config}
}

func (w stoppedWatcher) InformAboutAsync(ctx *Context, v Value) {
	//ignore
}

func (w stoppedWatcher) WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error) {
	return nil, ErrStoppedWatcher
}

func (w stoppedWatcher) Stop() {
}

func (w stoppedWatcher) IsStopped() bool {
	return true
}

func (w stoppedWatcher) Config() WatcherConfiguration {
	return w.config
}

type joinedWatchers struct {
	NotClonableMixin
	config WatcherConfiguration

	watchers  []Watcher
	remaining []Value
	stopped   atomic.Bool
	wg        *sync.WaitGroup
}

func joinWatchers(config WatcherConfiguration, watchers ...Watcher) Watcher {
	var list []Watcher

	for _, w := range watchers {
		if _, ok := w.(stoppedWatcher); !ok {
			list = append(list, w)
		}
	}

	switch len(list) {
	case 0:
		return stoppedWatcher{}
	case 1:
		return list[0] //TODO: wrap watcher if its config is not config
	}

	return &joinedWatchers{
		watchers: watchers,
		wg:       new(sync.WaitGroup),
		config:   config,
	}
}

func (w *joinedWatchers) InformAboutAsync(ctx *Context, v Value) {
	//ignore
}

func (w *joinedWatchers) WaitNext(ctx *Context, additionalFilter Pattern, timeout time.Duration) (next Value, err error) {
	stopped := w.stopped.Load()
	if stopped {
		return nil, ErrStoppedWatcher
	}

	if len(w.remaining) > 0 {
		r := w.remaining[0]
		copy(w.remaining[0:len(w.remaining)-1], w.remaining[1:])
		w.remaining = w.remaining[:len(w.remaining)-1]
		return r, nil
	}

	results := make(chan Value, len(w.watchers))
	w.wg.Add(len(w.watchers))

	for _, watcher := range w.watchers {
		go func(watcher Watcher) {
			defer w.wg.Done()

			v, err := watcher.WaitNext(ctx, w.config.Filter, timeout)
			if err == nil {
				if additionalFilter == nil || additionalFilter.Test(ctx, v) {
					results <- v
					return
				}
			} else {
				return
			}
		}(watcher)
	}

	defer func() {
		if err != nil {
			close(results)
			return
		}

		//we append other results to .remaining
		w.wg.Wait()
		i := 0
		count := len(results)
		if count > 0 {
			for e := range results {
				w.remaining = append(w.remaining, e)
				i++
				if i == count {
					break
				}
			}
		}
		close(results)
	}()

	select {
	case v := <-results: //we wait a value matching the filter, other results will be saved.
		return v, nil
	case <-time.After(timeout):
		return nil, ErrStoppedWatcher
	case <-ctx.Done():
		w.Stop()
		return nil, ErrStoppedWatcher
	}
}

func (w *joinedWatchers) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		for _, watcher := range w.watchers {
			watcher.Stop()
		}
	}
}

func (w *joinedWatchers) IsStopped() bool {
	return w.stopped.Load()
}

func (w *joinedWatchers) Config() WatcherConfiguration {
	return w.config
}

type GenericWatcher struct {
	NotClonableMixin

	config WatcherConfiguration
	values chan Value // watchable valie writes interesting values to this channel by calling .AddValueAsync

	stopped atomic.Bool
}

func NewGenericWatcher(config WatcherConfiguration) *GenericWatcher {
	return &GenericWatcher{
		config: config,
		values: make(chan Value, 10),
	}
}

// InformAboutAsync adds a value to the channel of interesting values.
func (w *GenericWatcher) InformAboutAsync(ctx *Context, v Value) {
	//TODO: prevent blocking
	w.values <- v
}

func (w *GenericWatcher) WaitNext(ctx *Context, additionalFilter Pattern, timeout time.Duration) (Value, error) {
	stopped := w.stopped.Load()
	if stopped {
		return nil, ErrStoppedWatcher
	}

	start := time.Now()

	for {
		select {
		case v, open := <-w.values:
			if !open {
				return nil, ErrStoppedWatcher
			}
			if additionalFilter == nil || additionalFilter.Test(ctx, v) {
				return v, nil
			}
			timeout -= time.Since(start)
		case <-time.After(timeout): // TODO: rework
			return nil, ErrWatchTimeout
		case <-ctx.Done():
			w.Stop()
			return nil, ErrStoppedWatcher
		}
	}

}

func (w *GenericWatcher) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		close(w.values)
	}
}

func (w *GenericWatcher) IsStopped() bool {
	return w.stopped.Load()
}

func (w *GenericWatcher) Config() WatcherConfiguration {
	return w.config
}

func (obj *Object) Watcher(ctx *Context, config WatcherConfiguration) Watcher {
	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	closestState := ctx.GetClosestState()
	watcher := NewGenericWatcher(config)

	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	if obj.watchers == nil {
		obj.watchers = NewValueWatchers()
	}

	obj.watchers.Add(watcher)

	return watcher
}

func (list *List) Watcher(ctx *Context, config WatcherConfiguration) Watcher {
	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		panic(ErrIntermediateDepthWatchingNotSupported)
	}

	watcher := NewGenericWatcher(config)

	list.lock.Lock()
	defer list.lock.Unlock()

	if list.watchers == nil {
		list.watchers = NewValueWatchers()
	}

	list.watchers.Add(watcher)

	return watcher
}

func (s *RuneSlice) Watcher(ctx *Context, config WatcherConfiguration) Watcher {
	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		panic(ErrIntermediateDepthWatchingNotSupported)
	}

	watcher := NewGenericWatcher(config)

	s.lock.Lock()
	defer s.lock.Unlock()

	if s.watchers == nil {
		s.watchers = NewValueWatchers()
	}

	s.watchers.Add(watcher)

	return watcher
}

// Watcher creates a watcher that watches deeply by default, the watcher only watches mutations.
func (dyn *DynamicValue) Watcher(ctx *Context, config WatcherConfiguration) Watcher {
	depth := config.Depth
	if depth == UnspecifiedWatchingDepth {
		depth = DeepWatching
	}

	watcher := NewGenericWatcher(config)

	_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
		if watcher.IsStopped() {
			registerAgain = false
			return
		}

		registerAgain = true

		if !config.Filter.Test(ctx, mutation) {
			return
		}

		watcher.InformAboutAsync(ctx, Mutation{
			Kind:  UnspecifiedMutation,
			Path:  config.Path,
			Depth: ShallowWatching,
		})
		return
	}, MutationWatchingConfiguration{Depth: config.Depth})

	if err != nil {
		panic(err)
	}

	return watcher
}

func (*SystemGraph) Watcher(ctx *Context, config WatcherConfiguration) Watcher {
	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	panic(ErrNotImplementedYet)
}
