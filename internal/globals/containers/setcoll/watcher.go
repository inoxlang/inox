package setcoll

import "github.com/inoxlang/inox/internal/core"

const (
	DEFAULT_WATCHING_DEPTH = core.ShallowWatching
)

func (set *Set) Watcher(ctx *core.Context, config core.WatcherConfiguration) core.Watcher {
	if config.Depth == core.UnspecifiedWatchingDepth {
		config.Depth = DEFAULT_WATCHING_DEPTH
	}

	closestState := ctx.MustGetClosestState()
	watcher := core.NewGenericWatcher(config)

	set._lock(closestState)
	defer set._unlock(closestState)

	if set.watchers == nil {
		set.watchers = core.NewValueWatchers()
	}

	set.watchers.Add(watcher)

	return watcher
}
