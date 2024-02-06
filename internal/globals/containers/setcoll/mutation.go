package setcoll

import (
	"github.com/inoxlang/inox/internal/core"
)

const (
	AddElem core.SpecificMutationKind = iota + 1
	RemoveElem
)

func NewAddElemMutation(path core.Path) core.Mutation {
	return core.NewSpecificIncompleteNoDataMutation(core.SpecificMutationMetadata{
		Version: 1,
		Kind:    AddElem,
		Depth:   core.ShallowWatching,
		Path:    path,
	})
}

func NewRemoveElemMutation(path core.Path) core.Mutation {
	return core.NewSpecificIncompleteNoDataMutation(core.SpecificMutationMetadata{
		Version: 1,
		Kind:    RemoveElem,
		Depth:   core.ShallowWatching,
		Path:    path,
	})
}

func (set *Set) informAboutMutation(ctx *core.Context, mutation core.Mutation) {
	if set.mutationCallbacks != nil {
		set.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

		if set.mutationCallbacks != nil {
			set.mutationCallbacks.CallMicrotasks(ctx, mutation)
		}
	}
}

func (set *Set) OnMutation(ctx *core.Context, microtask core.MutationCallbackMicrotask, config core.MutationWatchingConfiguration) (core.CallbackHandle, error) {
	state := ctx.GetClosestState()
	set._lock(state)
	defer set._unlock(state)

	if config.Depth == core.UnspecifiedWatchingDepth {
		config.Depth = DEFAULT_WATCHING_DEPTH
	}

	if config.Depth >= core.IntermediateDepthWatching {
		if config.Depth > set.watchingDepth {
			set.watchingDepth = config.Depth

			//TODO or ignore deep watching for Sets
			// if set.propMutationCallbacks == nil {
			// 	set.propMutationCallbacks = make([]core.CallbackHandle, len(set.keys))
			// }

			// for i, val := range set.core.Values {
			// 	if err := set.addPropMutationCallbackNoLock(ctx, i, val); err != nil {
			// 		return core.FIRST_VALID_CALLBACK_HANDLE - 1, err
			// 	}
			// }
		}
	}

	if set.mutationCallbacks == nil {
		set.mutationCallbacks = core.NewMutationCallbacks()
	}

	handle := set.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (set *Set) RemoveMutationCallbackMicrotasks(ctx *core.Context) {
	state := ctx.GetClosestState()
	set._lock(state)
	defer set._unlock(state)

	if set.mutationCallbacks == nil {
		return
	}

	set.mutationCallbacks.RemoveMicrotasks()
}

func (set *Set) RemoveMutationCallback(ctx *core.Context, handle core.CallbackHandle) {
	state := ctx.GetClosestState()
	set._lock(state)
	defer set._unlock(state)

	set.mutationCallbacks.RemoveMicrotask(handle)
}
