package fs_ns

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/core"
)

const (
	MAX_SMALL_CHANGE_SIZE = 1000 //byte count
)

var (
	_ billy.File = (*metaFsFile)(nil)
)

type metaFsFile struct {
	fs           *MetaFilesystem
	originalPath string
	path         core.Path
	underlying   billy.File
	metadata     *metaFsFileMetadata
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
	if err := f.checkUsableSpace(len(p)); err != nil {
		return 0, err
	}

	//TODO: prevent leaks about underlying file
	return f.underlying.Write(p)
}

func (f *metaFsFile) Read(p []byte) (n int, err error) {
	//TODO: prevent leaks about underlying file
	return f.underlying.Read(p)
}

func (f *metaFsFile) ReadAt(p []byte, off int64) (n int, err error) {
	//TODO: prevent leaks about underlying file
	return f.underlying.ReadAt(p, off)
}

func (f *metaFsFile) Seek(offset int64, whence int) (int64, error) {
	//TODO: prevent leaks about underlying file
	return f.underlying.Seek(offset, whence)
}

func (f *metaFsFile) Close() error {
	err := f.underlying.Close()
	if err != nil {
		f.fs.ctx.Logger().Err(err).Msg("failed to close metafs file " + string(f.metadata.path))
		return fmt.Errorf("failed to close %s", f.metadata.path)
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
	if f.metadata.concreteFile != nil {
		stat, err := core.FileStat(f.underlying, f.fs)
		if err != nil {
			return err
		}

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
