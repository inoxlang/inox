package setcoll

import "github.com/inoxlang/inox/internal/core"

const (
	DEFAULT_WATCHING_DEPTH = core.ShallowWatching
)

func (set *Set) Watcher(ctx *core.Context, config core.WatcherConfiguration) core.Watcher {
	if config.Depth == core.UnspecifiedWatchingDepth {
		config.Depth = DEFAULT_WATCHING_DEPTH
	}

	closestState := ctx.GetClosestState()
	watcher := core.NewGenericWatcher(config)

	set.Lock(closestState)
	defer set.Unlock(closestState)

	if set.watchers == nil {
		set.watchers = core.NewValueWatchers()
	}

	set.watchers.Add(watcher)

	return watcher
}
