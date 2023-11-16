package fs_ns

import (
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/in_mem_ds"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	WATCHER_MANAGEMENT_TICK_INTERVAL = 10 * time.Millisecond
)

var (
	watchedVirtualFilesystems     = map[watchableVirtualFilesystem]struct{}{}
	watchedVirtualFilesystemsLock sync.Mutex

	watcherManagingGoroutineStarted atomic.Bool

	_ = []watchableVirtualFilesystem{(*MemFilesystem)(nil)}
)

// watchableVirtualFilesystem is implemented by non-OS filesystems that can track FS events.
type watchableVirtualFilesystem interface {
	//watcher creates a new watcher.
	watcher(evs *FilesystemEventSource) *virtualFilesystemWatcher

	//getWatchers returns a copy of the list of current watchers, it is preferrable to not return
	//stopped watchers.
	getWatchers() []*virtualFilesystemWatcher

	//events() returns the ACTUAL queue of events.
	//It will be emptied by the watcher managing goroutine.
	events() *in_mem_ds.TSArrayQueue[fsEventInfo]
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

func (fls *MemFilesystem) watcher(evs *FilesystemEventSource) *virtualFilesystemWatcher {
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

func (fls *MemFilesystem) events() *in_mem_ds.TSArrayQueue[fsEventInfo] {
	return fls.s.eventQueue
}

func (fls *MemFilesystem) getWatchers() []*virtualFilesystemWatcher {
	fls.watchersLock.Lock()
	defer fls.watchersLock.Unlock()

	//remove stopped watchers
	startIndex := 0
outer:
	for startIndex < len(fls.watchers) {
		for i, watcher := range fls.watchers[startIndex:] {
			if watcher.stopped.Load() {
				fls.watchers = slices.Delete(fls.watchers, i, i+1)
				startIndex = i //don't continue checking from the start (perf)
				continue outer
			}
			startIndex = i + 1 //don't continue checking from the start (perf)
		}
	}

	watchers := slices.Clone(fls.watchers)

	if len(watchers) == 0 {
		watchedVirtualFilesystemsLock.Lock()
		delete(watchedVirtualFilesystems, fls)
		watchedVirtualFilesystemsLock.Unlock()
	}

	return watchers
}

func startWatcherManagingGoroutine() {
	if !watcherManagingGoroutineStarted.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer utils.Recover()
		ticker := time.NewTicker(WATCHER_MANAGEMENT_TICK_INTERVAL)
		defer ticker.Stop()

		manageSingleFs := func(vfs watchableVirtualFilesystem) {
			defer utils.Recover()

			watchers := vfs.getWatchers()
			queue := vfs.events()
			events := queue.DequeueAll()
			coreEvents := utils.MapSlice(events, newFsEvent)

			//inform watchers about the events
			for _, w := range watchers {
				handlers := w.eventSource.GetHandlers()

				if w.eventSource.IsClosed() {
					w.Close()
					continue
				}

				for eventIndex, event := range events {
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

		filesystems := map[watchableVirtualFilesystem]struct{}{}

		for range ticker.C {
			clear(filesystems)

			watchedVirtualFilesystemsLock.Lock()
			//we copy the filesystems to avoid locking the map for too long.
			maps.Copy(filesystems, watchedVirtualFilesystems)
			watchedVirtualFilesystemsLock.Unlock()

			for vfs := range filesystems {
				manageSingleFs(vfs)
			}
		}
	}()

}
