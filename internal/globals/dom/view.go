package internal

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	core "github.com/inox-project/inox/internal/core"
)

// A View represents the live view of a resource.
type View struct {
	core.NoReprMixin
	core.NotClonableMixin

	resource core.Path
	model    *core.Object
	domNode  *Node

	nodeWatcher core.Watcher

	lock     sync.Mutex
	watchers []*core.PeriodicWatcher
	ctx      *core.Context
}

func NewView(ctx *core.Context, resource core.Path, model *core.Object, domNode *Node) *View {
	if domNode.hasView() {
		panic(errors.New("failed to create new render: dom node already has an associated view"))
	}
	view := &View{
		resource: resource,
		model:    model,
		domNode:  domNode,
		ctx:      ctx,
	}
	domNode.attachToView(ctx, view)

	view.nodeWatcher = domNode.Watcher(ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})
	view.startUpdateGoroutine(ctx)
	return view
}

func (v *View) Node() *Node {
	return v.domNode
}

func (v *View) Context() *core.Context {
	return v.ctx
}

func (v *View) ModelIs(ctx *core.Context, val *core.Object) bool {
	return v.model == val
}

func (v *View) startUpdateGoroutine(ctx *core.Context) {

	go func() {
		defer func() {
			//cleanup
			v.nodeWatcher.Stop()

			v.lock.Lock()
			defer v.lock.Unlock()
			for _, w := range v.watchers {
				w.Stop()
			}
		}()

		for {
			select {
			case <-ctx.Done():
				v.domNode.detachFromView()
				return
			default:
			}

			_, err := v.nodeWatcher.WaitNext(ctx, nil, CHANGE_WATCH_TIMEOUT)
			if errors.Is(err, core.ErrStoppedWatcher) || errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, core.ErrWatchTimeout) {
				continue
			}

			if err == nil {

				// inform watchers about the update

				mutation := core.NewUnspecifiedMutation(0, "")

				func() {
					v.lock.Lock()
					defer v.lock.Unlock()
					for _, w := range v.watchers {
						w.InformAboutAsync(ctx, mutation)
					}
				}()
			}

		}
	}()
}

func (v *View) Watcher(ctx *core.Context, config core.WatcherConfiguration) core.Watcher {
	if config.Depth >= core.IntermediateDepthWatching {
		panic(core.ErrDeepWatchingNotSupported)
	}

	if config.Filter != core.MUTATION_PATTERN { //TODO: or combinations of mutation patterns
		return core.NewStoppedWatcher(config)
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	watcher := core.NewPeriodicWatcher(core.WatcherConfiguration{Filter: core.MUTATION_PATTERN}, NODE_WATCHER_PERIOD)
	v.watchers = append(v.watchers, watcher)

	return watcher
}

func (*View) OnMutation(ctx *core.Context, microtask core.MutationCallbackMicrotask, config core.MutationWatchingConfiguration) (core.CallbackHandle, error) {
	panic(core.ErrNotImplementedYet)
}

func (*View) RemoveMutationCallbackMicrotasks() {
	panic(core.ErrNotImplementedYet)
}

func (v *View) RemoveMutationCallback(handle int) {
	panic(core.ErrNotImplementedYet)
}

func (v *View) SendDOMEventToForwader(ctx *core.Context, eventData *core.Record, t time.Time) {
	forwarderData, ok := eventData.Prop(ctx, "forwarderData").(*core.Record)
	if !ok {
		return
	}

	idString, ok := forwarderData.Prop(ctx, "forwarderId").(core.Str)
	if !ok {
		return
	}

	id, err := strconv.ParseUint(string(idString), FORWARDER_ID_ENCODING_BASE, FORWARDER_ID_BITSIZE)
	if err != nil {
		return
	}

	v.domNode.SendDOMEventToForwader(ctx, id, eventData, t)
}
