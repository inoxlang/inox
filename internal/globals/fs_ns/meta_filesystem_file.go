package fs_ns

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_SMALL_CHANGE_SIZE = 1000 //byte count

	META_FS_FILE_SNAPSHOTING_WAIT_TIMEOUT = 100 * time.Millisecond
)

var (
	_ billy.File = (*metaFsFile)(nil)
)

type metaFsFile struct {
	fs           *MetaFilesystem
	originalPath string
	path         core.Path
	flag         int
	underlying   afs.SyncCapable
	metadata     *metaFsFileMetadata

	snapshoting atomic.Bool
	closed      atomic.Bool
}

func (f *metaFsFile) Name() string {
	return f.originalPath
}

func (f *metaFsFile) checkUsableSpace(addedBytes int) error {
	// TODO: take position into account
	if yes, err := f.fs.checkAddedByteCount(core.ByteCount(addedBytes)); err != nil {
		return err
	} else if !yes {
		return ErrNoRemainingSpaceToApplyChange
	}

	return nil
}

func (f *metaFsFile) Write(p []byte) (n int, err error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	if !utils.InefficientlyWaitUntilFalse(&f.snapshoting, META_FS_FILE_SNAPSHOTING_WAIT_TIMEOUT) {
		return 0, ErrFileBeingSnapshoted
	}

	if err := f.checkUsableSpace(len(p)); err != nil {
		return 0, err
	}

	//TODO: prevent leaks about underlying file
	return f.underlying.Write(p)
}

func (f *metaFsFile) Read(p []byte) (n int, err error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	//TODO: prevent leaks about underlying file
	return f.underlying.Read(p)
}

func (f *metaFsFile) ReadAt(p []byte, off int64) (n int, err error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	//TODO: prevent leaks about underlying file
	return f.underlying.ReadAt(p, off)
}

func (f *metaFsFile) Seek(offset int64, whence int) (int64, error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	//TODO: prevent leaks about underlying file
	return f.underlying.Seek(offset, whence)
}

func (f *metaFsFile) Close() error {
	err := f.underlying.Close()
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			f.closed.Store(true)
		}

		f.fs.ctx.Logger().Err(err).Msg("failed to close metafs file " + string(f.metadata.path))
		return fmt.Errorf("failed to close %s", f.metadata.path)
	} else {
		f.closed.Store(true)
	}
	return nil
}

func (f *metaFsFile) Lock() error {
	return core.ErrNotImplemented
}

func (f *metaFsFile) Unlock() error {
	return core.ErrNotImplemented
}

func (f *metaFsFile) Truncate(size int64) error {
	if f.closed.Load() {
		return os.ErrClosed
	}

	if !utils.InefficientlyWaitUntilFalse(&f.snapshoting, META_FS_FILE_SNAPSHOTING_WAIT_TIMEOUT) {
		return ErrFileBeingSnapshoted
	}

	if f.metadata.concreteFile != nil {
		stat, err := core.FileStat(f.underlying, f.fs)
		if err != nil {
			return err
		}

		// if the new size is greater than the current size we check the usable space.
		// obviously this is not robust code
		if currSize := stat.Size(); size > stat.Size() {
			if err := f.checkUsableSpace(int(size - currSize)); err != nil {
				return err
			}
		}
	}

	err := f.underlying.Truncate(size)
	if err != nil {
		f.fs.ctx.Logger().Err(err).Msg("failed to close metafs file " + string(f.metadata.path))
		return fmt.Errorf("failed to truncate %s", f.metadata.path)
	}
	return nil
}

func (f *metaFsFile) Sync() error {
	if f.closed.Load() {
		return os.ErrClosed
	}
	return f.underlying.Sync()
}
