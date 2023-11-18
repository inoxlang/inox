package fs_ns

import (
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/in_mem_ds"
)

var (
	_ = afs.SyncCapable((*InMemfile)(nil))
)

type InMemfile struct {
	basename     string
	originalPath string
	absPath      core.Path
	content      *InMemFileContent
	flag         int
	mode         os.FileMode

	isClosed atomic.Bool

	position     int64
	positionLock sync.Mutex

	//last events of the inMemStorage containing the file.
	storageLastEvents *in_mem_ds.TSArrayQueue[fsEventInfo]
}

func (f *InMemfile) Name() string {
	return f.originalPath
}

func (f *InMemfile) Read(b []byte) (int, error) {
	f.positionLock.Lock()
	defer f.positionLock.Unlock()

	n, err := f.ReadAt(b, f.position)
	f.position += int64(n)

	if err == io.EOF && n != 0 {
		err = nil
	}

	return n, err
}

func (f *InMemfile) ReadAt(b []byte, off int64) (int, error) {
	if f.isClosed.Load() {
		return 0, os.ErrClosed
	}

	if !IsReadAndWrite(f.flag) && !IsReadOnly(f.flag) {
		return 0, errors.New("read not supported")
	}

	n, err := f.content.ReadAt(b, off)

	return n, err
}

func (f *InMemfile) Seek(offset int64, whence int) (int64, error) {
	if f.isClosed.Load() {
		return 0, os.ErrClosed
	}

	f.positionLock.Lock()
	defer f.positionLock.Unlock()

	switch whence {
	case io.SeekCurrent:
		f.position += offset
	case io.SeekStart:
		f.position = offset
	case io.SeekEnd:
		f.position = int64(f.content.Len()) + offset
	}

	return f.position, nil
}

func (f *InMemfile) Write(p []byte) (int, error) {
	if f.isClosed.Load() {
		return 0, os.ErrClosed
	}

	if !IsReadAndWrite(f.flag) && !isWriteOnly(f.flag) {
		return 0, errors.New("write not supported")
	}

	f.positionLock.Lock()
	defer f.positionLock.Unlock()

	n, err := f.content.WriteAt(p, f.position)
	f.position += int64(n)

	//add event and remove old events.
	f.storageLastEvents.EnqueueAutoRemove(fsEventInfo{
		path:     core.Path(f.absPath),
		writeOp:  true,
		dateTime: core.DateTime(f.content.ModifTime()),
	})

	return n, err
}

func (*InMemfile) Sync() error {
	//no-op
	return nil
}

func (f *InMemfile) Close() error {
	if !f.isClosed.CompareAndSwap(false, true) {
		return os.ErrClosed
	}

	return nil
}

func (f *InMemfile) Truncate(size int64) error {
	err := f.content.Truncate(size)

	if err == nil {
		//add event and remove old events.
		f.storageLastEvents.EnqueueAutoRemove(fsEventInfo{
			path:     core.Path(f.absPath),
			writeOp:  true,
			dateTime: core.DateTime(f.content.ModifTime()),
		})
	}

	return err
}

func (f *InMemfile) Duplicate(originalPath string, mode os.FileMode, flag int) billy.File {
	new := &InMemfile{
		basename:     f.basename,
		absPath:      core.PathFrom(NormalizeAsAbsolute(originalPath)),
		originalPath: originalPath,
		content:      f.content,
		mode:         mode,
		flag:         flag,

		storageLastEvents: f.storageLastEvents,
	}

	if IsTruncate(flag) {
		new.content.Truncate(0)
	}

	if IsAppend(flag) {
		new.position = int64(new.content.Len())
	}

	return new
}

func (f *InMemfile) FileInfo() core.FileInfo {
	f.content.lock.RLock()
	defer f.content.lock.RUnlock()
	return f.FileInfoContentNotLocked()
}

func (f *InMemfile) FileInfoContentNotLocked() core.FileInfo {
	return core.FileInfo{
		BaseName_:       f.basename,
		AbsPath_:        f.absPath,
		Mode_:           core.FileMode(f.mode),
		Size_:           core.ByteCount(f.content.lenNoLock()),
		ModTime_:        core.DateTime(f.content.ModifTime()),
		HasCreationTime: true,
		CreationTime_:   core.DateTime(f.content.creationTime),
	}
}

func (f *InMemfile) Stat() (os.FileInfo, error) {
	return f.FileInfo(), nil
}

// Lock is a no-op in memfs.
func (f *InMemfile) Lock() error {
	return nil
}

// Unlock is a no-op in memfs.
func (f *InMemfile) Unlock() error {
	return nil
}
