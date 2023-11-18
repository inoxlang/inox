package fs_ns

import (
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/in_mem_ds"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	WATCHER_MANAGEMENT_TICK_INTERVAL = 25 * time.Millisecond
	OLD_EVENT_MIN_AGE                = max(50*time.Millisecond, 2*WATCHER_MANAGEMENT_TICK_INTERVAL)
)

var (
	watchedVirtualFilesystems     = map[WatchableVirtualFilesystem]struct{}{}
	watchedVirtualFilesystemsLock sync.Mutex

	watcherManagingGoroutineStarted atomic.Bool

	_ = []WatchableVirtualFilesystem{(*MemFilesystem)(nil), (*MetaFilesystem)(nil)}
)

// WatchableVirtualFilesystem is implemented by non-OS filesystems that can track FS events.
type WatchableVirtualFilesystem interface {
	ClosableFilesystem

	//Watcher creates a new Watcher.
	Watcher(evs *FilesystemEventSource) *virtualFilesystemWatcher

	//GetWatchers returns a copy of the list of current watchers, it is preferrable to not return
	//stopped watchers.
	GetWatchers() []*virtualFilesystemWatcher

	//Events() returns the ACTUAL queue of Events.
	//If the filesystem is properly added to the watchedVirtualFilesystems, it is periodically emptied by the watcher managing goroutine.
	//Wathever it is watched, the filesystem is responsible for removing old Events, especially after a recent event.
	//Old is specified as being >= OLD_EVENT_MIN_AGE.
	Events() *in_mem_ds.TSArrayQueue[Event]
}

func isOldEvent(v Event) bool {
	return time.Time(v.dateTime).Before(time.Now().Add(-OLD_EVENT_MIN_AGE))
}

type virtualFilesystemWatcher struct {
	eventSource  *FilesystemEventSource
	creationTime time.Time
	stopped      atomic.Bool
}

func (w *virtualFilesystemWatcher) Close() error {
	w.stopped.Store(true)
	return nil
}

func (fls *MemFilesystem) Watcher(evs *FilesystemEventSource) *virtualFilesystemWatcher {
	watcher := &virtualFilesystemWatcher{
		eventSource:  evs,
		creationTime: time.Now(),
	}

	startWatcherManagingGoroutine()

	fls.watchersLock.Lock()
	fls.watchers = append(fls.watchers, watcher)
	fls.watchersLock.Unlock()

	watchedVirtualFilesystemsLock.Lock()
	watchedVirtualFilesystems[fls] = struct{}{}
	watchedVirtualFilesystemsLock.Unlock()

	return watcher
}

func (fls *MemFilesystem) Events() *in_mem_ds.TSArrayQueue[Event] {
	return fls.s.eventQueue
}

func (fls *MemFilesystem) GetWatchers() []*virtualFilesystemWatcher {
	fls.watchersLock.Lock()
	defer fls.watchersLock.Unlock()

	removeStoppedWatchers(&fls.watchers)

	watchers := slices.Clone(fls.watchers)

	if len(watchers) == 0 {
		watchedVirtualFilesystemsLock.Lock()
		delete(watchedVirtualFilesystems, fls)
		watchedVirtualFilesystemsLock.Unlock()
	}

	return watchers
}

func (fls *MetaFilesystem) Watcher(evs *FilesystemEventSource) *virtualFilesystemWatcher {
	watcher := &virtualFilesystemWatcher{
		eventSource:  evs,
		creationTime: time.Now(),
	}

	startWatcherManagingGoroutine()

	fls.fsWatchersLock.Lock()
	fls.fsWatchers = append(fls.fsWatchers, watcher)
	fls.fsWatchersLock.Unlock()

	watchedVirtualFilesystemsLock.Lock()
	watchedVirtualFilesystems[fls] = struct{}{}
	watchedVirtualFilesystemsLock.Unlock()

	return watcher
}

func (fls *MetaFilesystem) Events() *in_mem_ds.TSArrayQueue[Event] {
	return fls.eventQueue
}

func (fls *MetaFilesystem) GetWatchers() []*virtualFilesystemWatcher {
	fls.fsWatchersLock.Lock()
	defer fls.fsWatchersLock.Unlock()

	removeStoppedWatchers(&fls.fsWatchers)

	watchers := slices.Clone(fls.fsWatchers)

	if len(watchers) == 0 {
		watchedVirtualFilesystemsLock.Lock()
		delete(watchedVirtualFilesystems, fls)
		watchedVirtualFilesystemsLock.Unlock()
	}

	return watchers
}

func removeStoppedWatchers(watchers *[]*virtualFilesystemWatcher) {
	//remove stopped watchers
	startIndex := 0
outer:
	for startIndex < len(*watchers) {
		for i, watcher := range (*watchers)[startIndex:] {
			if watcher.stopped.Load() {
				*watchers = slices.Delete(*watchers, i, i+1)
				startIndex = i //don't continue checking from the start (perf)
				continue outer
			}
			startIndex = i + 1 //don't continue checking from the start (perf)
		}
	}
}

func startWatcherManagingGoroutine() {
	if !watcherManagingGoroutineStarted.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer utils.Recover()
		ticker := time.NewTicker(WATCHER_MANAGEMENT_TICK_INTERVAL)
		defer ticker.Stop()

		for range ticker.C {
			filesystems := map[WatchableVirtualFilesystem]struct{}{}

			watchedVirtualFilesystemsLock.Lock()
			//we copy the filesystems to avoid locking the map for too long.
			maps.Copy(filesystems, watchedVirtualFilesystems)
			watchedVirtualFilesystemsLock.Unlock()

			go informWatchersAboutEvents(filesystems)
		}
	}()

}

func informWatchersAboutEvents(filesystems map[WatchableVirtualFilesystem]struct{}) {
	defer utils.Recover()

	var deduplicatedEvents []Event
	var writtenFiles []core.Path
	//these slice are re-used accross all invocations of manageSingleFs to minimize allocations.

	manageSingleFS := func(vfs WatchableVirtualFilesystem) {
		defer utils.Recover()

		watchers := vfs.GetWatchers()
		queue := vfs.Events()
		events := queue.DequeueAll()

		deduplicatedEvents = deduplicatedEvents[:0]
		writtenFiles = writtenFiles[:0]

		defer func() {
			deduplicatedEvents = deduplicatedEvents[:0]
			writtenFiles = writtenFiles[:0]
		}()

		//collapse write-only events on the same file
		for _, event := range events {

			if !event.writeOp || event.createOp || event.removeOp || event.renameOp {
				deduplicatedEvents = append(deduplicatedEvents, event)
				continue
			}

			if !slices.Contains(writtenFiles, event.path) {
				deduplicatedEvents = append(deduplicatedEvents, event)
				writtenFiles = append(writtenFiles, event.path)
			}
		}

		events = nil
		coreEvents := utils.MapSlice(deduplicatedEvents, func(e Event) *core.Event { return e.CreateCoreEvent() })

		//inform watchers about the events
		for _, w := range watchers {
			handlers := w.eventSource.GetHandlers()

			if w.eventSource.IsClosed() {
				w.Close()
				continue
			}

			for eventIndex, event := range deduplicatedEvents {
				filter := w.eventSource.GetFilter()
				if filter != "" && !filter.Test(nil, event.path) {
					continue
				}

				//if the event happened before the watcher creation we ignore it.
				if time.Time(event.dateTime).Before(w.creationTime) {
					continue
				}

				coreEvent := coreEvents[eventIndex]

				for _, handler := range handlers {
					handler(coreEvent)
				}
			}
		}

	}

	for vfs := range filesystems {
		manageSingleFS(vfs)
	}

}
