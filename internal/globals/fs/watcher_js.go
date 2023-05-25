//go:build js

package internal

import (
	"errors"
	afs "github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
)

var (
	ErrFsWatcherNotAvailable = errors.New("filesystem watcher not available")
)

type fsWatcher struct {
}

func newFsWatcher() (fsWatcher, error) {
	panic(ErrFsWatcherNotAvailable)
}

func (fsWatcher) Add(name string) error {
	panic(ErrFsWatcherNotAvailable)
}

func (fsWatcher) Remove(name string) error {
	panic(ErrFsWatcherNotAvailable)
}

func (fsWatcher) Close() {
	panic(ErrFsWatcherNotAvailable)
}

func (fsWatcher) listenForEventsSync(
	ctx *core.Context,
	fls afs.Filesystem,
	eventSource *FilesystemEventSource,
	watchedDirPaths map[core.Path]bool,
) {
	panic(ErrFsWatcherNotAvailable)
}
