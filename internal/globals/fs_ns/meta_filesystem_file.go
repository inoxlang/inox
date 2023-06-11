package fs_ns

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/core"
)

var (
	_ billy.File = (*metaFsFile)(nil)
)

type metaFsFile struct {
	fs         *MetaFilesystem
	path       core.Path
	underlying billy.File
	metadata   *metaFsFileMetadata
}

func (f *metaFsFile) Name() string {
	return string(f.path)
}

func (f *metaFsFile) Write(p []byte) (n int, err error) {
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
	err := f.underlying.Truncate(size)
	if err != nil {
		f.fs.ctx.Logger().Err(err).Msg("failed to close metafs file " + string(f.metadata.path))
		return fmt.Errorf("failed to truncate %s", f.metadata.path)
	}
	return nil
}
