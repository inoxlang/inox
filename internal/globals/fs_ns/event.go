package fs_ns

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

func init() {
	core.RegisterEventSourceFactory(core.Scheme("file"), func(ctx *core.Context, resourceNameOrPattern core.Value) (core.EventSource, error) {
		return NewEventSource(ctx, resourceNameOrPattern)
	})
}

type FilesystemEventSource struct {
	path       core.Path
	pathFilter core.PathPattern

	core.EventSourceHandlerManagement
	watcher  interface{ Close() error }
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

// NewEventSource calls NewEventSourceWithFilesystem with ctx's filesystem.
func NewEventSource(ctx *core.Context, resourceNameOrPattern core.Value) (*FilesystemEventSource, error) {
	fls := ctx.GetFileSystem()

	return NewEventSourceWithFilesystem(ctx, fls, resourceNameOrPattern)
}

func NewEventSourceWithFilesystem(ctx *core.Context, fls afs.Filesystem, resourceNameOrPattern core.Value) (*FilesystemEventSource, error) {
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
		eventSource.pathFilter = patt //prefix pattern
		recursive = true
		permissionEntity = patt
	case core.Path:
		pth, err := v.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return nil, err
		}
		permissionEntity = pth

		strPath := strings.TrimRight(string(pth), "/") //we remove any trailing / because os.LStat will return an error for a file
		info, err := fls.Lstat(strPath)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			if !pth.IsDirPath() {
				return nil, core.ErrDirPathShouldEndInSlash
			}
			eventSource.path = pth
			permissionEntity = pth
			eventSource.pathFilter = core.PathPattern(pth) //globbing pattern
		} else {
			if pth.IsDirPath() {
				return nil, core.ErrFilePathShouldNotEndInSlash
			}
			dir := filepath.Dir(string(pth))
			eventSource.path = core.Path(dir + "/")
			eventSource.pathFilter = core.PathPattern(pth) //globbing pattern
		}
	}

	info, err := fls.Lstat(string(eventSource.path))
	if err != nil {
		return nil, err
	}
	if info.Mode().Type() == fs.ModeSymlink {
		return nil, errors.New("cannot watch a symlinked directory")
	}

	perm := core.FilesystemPermission{Kind_: permkind.Read, Entity: permissionEntity}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	//create watcher & add paths

	watchedDirPaths := map[core.Path]struct{}{}
	if recursive {
		_, paths := core.GetWalkEntries(ctx.GetFileSystem(), eventSource.path)
		for _, pathList := range paths[1:] {
			watchedDirPaths[core.Path(pathList[0])] = struct{}{}
			if err != nil {
				return nil, err
			}
		}
	}

	if watchable, ok := fls.(watchableVirtualFilesystem); ok {
		watcher := watchable.watcher(eventSource)

		eventSource.watcher = watcher
	} else {
		watcher, err := newFsWatcher()
		if err != nil {
			return nil, err
		}

		// TODO: prevent watching on special filesystems
		err = watcher.Add(string(eventSource.path))
		if err != nil {
			return nil, err
		}

		eventSource.watcher = watcher
		watchedDirPaths := map[core.Path]struct{}{}

		if recursive {
			_, paths := core.GetWalkEntries(ctx.GetFileSystem(), eventSource.path)
			for _, pathList := range paths[1:] {
				err = watcher.Add(pathList[0])
				watchedDirPaths[core.Path(pathList[0])] = struct{}{}
				if err != nil {
					return nil, err
				}
			}
		}

		go func() {
			defer utils.Recover()
			watcher.listenForEventsSync(ctx, fls, eventSource, watchedDirPaths)
		}()

	}
	return eventSource, nil
}

type Event struct {
	path                                           core.Path
	writeOp, createOp, removeOp, chmodOp, renameOp bool
	dateTime                                       core.DateTime
}

func (e Event) Path() core.Path {
	return e.path
}

func (e Event) Time() core.DateTime {
	return e.dateTime
}

func (e Event) HasWriteOp() bool {
	return e.writeOp
}

func (e Event) HasCreateOp() bool {
	return e.createOp
}
func (e Event) HasRemoveOp() bool {
	return e.removeOp
}

func (e Event) HasChmodOp() bool {
	return e.chmodOp
}

func (e Event) HasRenameOp() bool {
	return e.chmodOp
}

func (e Event) CreateCoreEvent() *core.Event {
	val := core.NewRecordFromMap(core.ValMap{
		"path":      e.path,
		"write_op":  core.Bool(e.writeOp),
		"create_op": core.Bool(e.createOp),
		"remove_op": core.Bool(e.removeOp),
		"chmod_op":  core.Bool(e.chmodOp),
		"rename_op": core.Bool(e.renameOp),
	})
	return core.NewEvent(e, val, e.dateTime, e.path)
}
