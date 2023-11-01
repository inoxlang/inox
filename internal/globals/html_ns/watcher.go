package html_ns

import "github.com/inoxlang/inox/internal/core"

var (
	_ core.Watchable = (*HTMLNode)(nil)
)

func (n *HTMLNode) Watcher(ctx *core.Context, config core.WatcherConfiguration) core.Watcher {
	if config.Depth == core.UnspecifiedWatchingDepth {
		config.Depth = core.ShallowWatching
	}

	if config.Depth >= core.IntermediateDepthWatching {
		panic(core.ErrIntermediateDepthWatchingNotSupported)
	}

	watcher := core.NewGenericWatcher(config)

	n.mutationFieldsLock.Lock()
	defer n.mutationFieldsLock.Unlock()

	if n.watchers == nil {
		n.watchers = core.NewValueWatchers()
	}

	n.watchers.Add(watcher)

	return watcher
}

func (n *HTMLNode) OnMutation(ctx *core.Context, microtask core.MutationCallbackMicrotask, config core.MutationWatchingConfiguration) (core.CallbackHandle, error) {
	n.mutationFieldsLock.Lock()
	defer n.mutationFieldsLock.Unlock()

	if config.Depth >= core.IntermediateDepthWatching {
		return core.FIRST_VALID_CALLBACK_HANDLE - 1, core.ErrIntermediateDepthWatchingNotSupported
	}

	if n.mutationCallbacks == nil {
		n.mutationCallbacks = core.NewMutationCallbacks()
	}

	handle := n.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (n *HTMLNode) RemoveMutationCallbackMicrotasks(ctx *core.Context) {
	n.mutationFieldsLock.Lock()
	defer n.mutationFieldsLock.Unlock()

	if n.mutationCallbacks == nil {
		return
	}

	n.mutationCallbacks.RemoveMicrotasks()
}

func (n *HTMLNode) RemoveMutationCallback(ctx *core.Context, handle core.CallbackHandle) {
	n.mutationFieldsLock.Lock()
	defer n.mutationFieldsLock.Unlock()

	n.mutationCallbacks.RemoveMicrotask(handle)
}
