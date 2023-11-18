//go:build unix

package fs_ns

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
)

type fsWatcher struct {
	*fsnotify.Watcher
}

func newFsWatcher() (fsWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fsWatcher{}, err
	}

	return fsWatcher{watcher}, nil
}

// start listening for events.
func (watcher fsWatcher) listenForEventsSync(
	ctx *core.Context,
	fls afs.Filesystem,
	eventSource *FilesystemEventSource,
	watchedDirPaths map[core.Path]struct{},
) {

	for {
		select {
		case <-ctx.Done():
			eventSource.Close()
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			eventPath := core.Path(event.Name)

			//update eventPath if event.Name is a directory.
			if eventPath[len(eventPath)-1] != '/' {
				dirPath := eventPath + "/"

				if _, ok := watchedDirPaths[dirPath]; ok {
					eventPath = dirPath
				} else {
					info, err := fls.Lstat(event.Name)
					if err == nil && info.IsDir() {
						eventPath = dirPath
					}
				}
			}

			filter := eventSource.GetFilter()
			if filter != "" && !filter.Test(nil, eventPath) {
				continue
			}

			now := time.Now()

			var (
				writeOp, createOp, removeOp, renameOp, chmodOp bool
			)

			if event.Has(fsnotify.Write) {
				writeOp = true
			}

			if event.Has(fsnotify.Create) {
				createOp = true

				if eventPath.IsDirPath() {
					watcher.Add(event.Name)
					watchedDirPaths[eventPath] = struct{}{}
				}
			}

			if event.Has(fsnotify.Remove) {
				removeOp = true
				if eventPath.IsDirPath() {
					watcher.Remove(event.Name)
					delete(watchedDirPaths, eventPath)
				}
			}

			if event.Has(fsnotify.Chmod) {
				chmodOp = true
			}

			if event.Has(fsnotify.Rename) {
				renameOp = true
			}

			fsEvent := FsEvent{
				path:     eventPath,
				writeOp:  writeOp,
				createOp: createOp,
				removeOp: removeOp,
				chmodOp:  chmodOp,
				renameOp: renameOp,
				dateTime: core.DateTime(now),
			}.CreateCoreEvent()

			for _, handler := range eventSource.GetHandlers() {
				func() {
					defer func() { recover() }()
					handler(fsEvent)
				}()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			//TODO: handle error
			_ = err
		}
	}

}
