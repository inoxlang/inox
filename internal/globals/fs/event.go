package internal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	core "github.com/inoxlang/inox/internal/core"
)

func init() {
	core.RegisterEventSourceFactory(core.Scheme("file"), func(ctx *core.Context, resourceNameOrPattern core.Value) (core.EventSource, error) {
		return NewEventSource(ctx, resourceNameOrPattern)
	})
}

type FilesystemEventSource struct {
	path       core.Path
	pathFilter core.PathPattern //ignored if empty

	core.EventSourceHandlerManagement
	watcher  *fsnotify.Watcher
	lock     sync.RWMutex
	isClosed bool
}

func (evs *FilesystemEventSource) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "close":
		return core.WrapGoMethod(evs.Close), true
	}
	return nil, false
}

func (evs *FilesystemEventSource) Prop(ctx *core.Context, name string) core.Value {
	method, ok := evs.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, evs))
	}
	return method
}

func (*FilesystemEventSource) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*FilesystemEventSource) PropertyNames(ctx *core.Context) []string {
	return []string{"close"}
}

func (evs *FilesystemEventSource) Close() {
	evs.lock.Lock()
	defer evs.lock.Unlock()
	evs.isClosed = true
	evs.watcher.Close()
}

func (evs *FilesystemEventSource) IsClosed() bool {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	return evs.isClosed
}

func (evs *FilesystemEventSource) GetPath() core.Path {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	return evs.path
}

func (evs *FilesystemEventSource) GetFilter() core.PathPattern {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	return evs.pathFilter
}

func (evs *FilesystemEventSource) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEventSourceIterator(evs, config)
}

func NewEventSource(ctx *core.Context, resourceNameOrPattern core.Value) (*FilesystemEventSource, error) {
	eventSource := &FilesystemEventSource{}

	recursive := false
	var permissionEntity core.WrappedString

	switch v := resourceNameOrPattern.(type) {
	case core.PathPattern:
		patt := v.ToAbs(ctx.GetFileSystem())
		if !patt.IsPrefixPattern() {
			return nil, errors.New("only prefix path patterns can be used to create a filesystem event source")
		}
		eventSource.path = core.Path(patt.Prefix())
		recursive = true
		permissionEntity = patt
	case core.Path:
		pth := v.ToAbs(ctx.GetFileSystem())
		permissionEntity = pth

		strPath := strings.TrimRight(string(pth), "/") //we remove any trailing / because os.LStat will return an error for a file
		info, err := os.Lstat(strPath)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			if !pth.IsDirPath() {
				return nil, core.ErrDirPathShouldEndInSlash
			}
			eventSource.path = pth
			permissionEntity = pth

		} else {
			if pth.IsDirPath() {
				return nil, core.ErrFilePathShouldNotEndInSlash
			}
			dir := filepath.Dir(string(pth))
			eventSource.path = core.Path(dir + "/")
			eventSource.pathFilter = core.PathPattern(pth)
		}
	}

	info, err := os.Lstat(string(eventSource.path))
	if err != nil {
		return nil, err
	}
	if info.Mode().Type() == fs.ModeSymlink {
		return nil, errors.New("cannot watch a symlinked directory")
	}

	perm := core.FilesystemPermission{Kind_: core.ReadPerm, Entity: permissionEntity}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	//create watcher & add paths

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	eventSource.watcher = watcher

	// TODO: prevent watching on special filesystems
	err = watcher.Add(string(eventSource.path))
	if err != nil {
		return nil, err
	}

	watchedDirPaths := map[core.Path]bool{}

	if recursive {
		_, paths := core.GetWalkEntries(ctx.GetFileSystem(), eventSource.path)
		for _, pathList := range paths[1:] {
			err = watcher.Add(pathList[0])
			watchedDirPaths[core.Path(pathList[0])] = true
			if err != nil {
				return nil, err
			}
		}
	}

	// start listening for events.
	go func() {
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

				if eventPath[len(eventPath)-1] != '/' {
					dirPath := eventPath + "/"

					if watchedDirPaths[dirPath] {
						eventPath = dirPath
					} else {
						info, err := os.Lstat(event.Name)
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
						watchedDirPaths[eventPath] = true
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

				fsEvent := core.NewEvent(core.NewRecordFromMap(core.ValMap{
					"path":      eventPath,
					"write_op":  core.Bool(writeOp),
					"create_op": core.Bool(createOp),
					"remove_op": core.Bool(removeOp),
					"chmod_op":  core.Bool(chmodOp),
					"rename_op": core.Bool(renameOp),
				}), core.Date(now), eventPath)

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
	}()

	return eventSource, nil
}
