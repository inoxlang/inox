package core

type ValueWatchers struct {
	watchers  []Watcher //TODO: use pool
	nextIndex int
}

func NewValueWatchers() *ValueWatchers {
	return &ValueWatchers{}
}

func (l *ValueWatchers) Add(watcher Watcher) {
	if l.nextIndex >= len(l.watchers) {
		l.watchers = append(l.watchers, watcher)
		l.nextIndex++
	} else {
		l.watchers[l.nextIndex] = watcher
		l.updateNextIndex()
	}
}

func (l *ValueWatchers) InformAboutAsync(ctx *Context, v Value, depth WatchingDepth, relocalize bool) {
	if l == nil {
		return
	}

	for _, watcher := range l.watchers {
		if watcher.IsStopped() || watcher.Config().Depth < depth {
			continue
		}
		if watcher.Config().Filter.Test(ctx, v) {
			value := v
			if relocalize {
				if mutation, ok := v.(Mutation); ok {
					value = mutation.Relocalized(watcher.Config().Path)
				}
			}

			watcher.InformAboutAsync(ctx, value)
		}
	}
}

func (l *ValueWatchers) StopAll() {
	if l == nil {
		return
	}

	for _, watcher := range l.watchers {
		watcher.Stop()
	}
}

func (l *ValueWatchers) updateNextIndex() {
	for i, w := range l.watchers {
		if w.IsStopped() {
			l.nextIndex = i
			return
		}
	}
	l.nextIndex = len(l.watchers)
}
