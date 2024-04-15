package projectserver

import (
	"errors"
	"os"

	"github.com/go-git/go-billy/v5"
	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/memds"
)

const (
	DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE = 10_000_000
)

var _ = fs_ns.WatchableVirtualFilesystem((*Filesystem)(nil))

// Filesystem is a filesystem that stores the unsaved documents in a separate filesystem.
type Filesystem struct {
	afs.Filesystem //working tree
	//osFs *fs_ns.OsFilesystem

	unsavedDocuments afs.Filesystem
}

// NewFilesystem creates a new Filesystem with a persistsed filesystem and a filesystem
// for storing the state of unsave documents. unsavedDocumentFs should be fast.
func NewFilesystem(workingTree afs.Filesystem, unsavedDocumentFs afs.Filesystem) *Filesystem {
	if unsavedDocumentFs == nil {
		panic(errors.New("unsavedDocumentFs is nil"))
	}
	return &Filesystem{
		Filesystem:       workingTree,
		unsavedDocuments: unsavedDocumentFs,
	}
}

// OpenFile opens the unsaved document if flag is os.O_RDONLY, otherwise the persisted file is open.
func (fs *Filesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if fs_ns.IsReadOnly(flag) {
		f, err := fs.unsavedDocuments.OpenFile(filename, flag, 0)
		if os.IsNotExist(err) {
			return fs.Filesystem.OpenFile(filename, flag, 0)
		}
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return fs.Filesystem.OpenFile(filename, flag, perm)
}

func (fs *Filesystem) unsavedDocumentsFS() afs.Filesystem {
	return fs.unsavedDocuments
}

func (fs *Filesystem) Close(ctx *core.Context) error {
	return fs.Filesystem.((interface{ Close(*core.Context) error })).Close(ctx)
}

func (fs *Filesystem) Events() *memds.TSArrayQueue[fs_ns.Event] {
	return fs.Filesystem.(fs_ns.WatchableVirtualFilesystem).Events()
}

func (fs *Filesystem) GetWatchers() []*fs_ns.VirtualFilesystemWatcher {
	return fs.Filesystem.(fs_ns.WatchableVirtualFilesystem).GetWatchers()
}

func (fs *Filesystem) Watcher(evs *fs_ns.FilesystemEventSource) *fs_ns.VirtualFilesystemWatcher {
	return fs.Filesystem.(fs_ns.WatchableVirtualFilesystem).Watcher(evs)
}
