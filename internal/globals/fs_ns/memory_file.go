package fs_ns

import (
	"errors"
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/core"
)

type inMemfile struct {
	basename     string
	originalPath string
	absPath      core.Path
	content      *inMemFileContent
	position     int64
	flag         int
	mode         os.FileMode

	isClosed bool
}

func (f *inMemfile) Name() string {
	return f.originalPath
}

func (f *inMemfile) Read(b []byte) (int, error) {
	n, err := f.ReadAt(b, f.position)
	f.position += int64(n)

	if err == io.EOF && n != 0 {
		err = nil
	}

	return n, err
}

func (f *inMemfile) ReadAt(b []byte, off int64) (int, error) {
	if f.isClosed {
		return 0, os.ErrClosed
	}

	if !IsReadAndWrite(f.flag) && !isReadOnly(f.flag) {
		return 0, errors.New("read not supported")
	}

	n, err := f.content.ReadAt(b, off)

	return n, err
}

func (f *inMemfile) Seek(offset int64, whence int) (int64, error) {
	if f.isClosed {
		return 0, os.ErrClosed
	}

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

func (f *inMemfile) Write(p []byte) (int, error) {
	if f.isClosed {
		return 0, os.ErrClosed
	}

	if !IsReadAndWrite(f.flag) && !isWriteOnly(f.flag) {
		return 0, errors.New("write not supported")
	}

	n, err := f.content.WriteAt(p, f.position)
	f.position += int64(n)

	return n, err
}

func (f *inMemfile) Close() error {
	if f.isClosed {
		return os.ErrClosed
	}

	f.isClosed = true
	return nil
}

func (f *inMemfile) Truncate(size int64) error {
	return f.content.Truncate(size)
}

func (f *inMemfile) Duplicate(originalPath string, mode os.FileMode, flag int) billy.File {
	new := &inMemfile{
		basename:     f.basename,
		absPath:      core.PathFrom(NormalizeAsAbsolute(originalPath)),
		originalPath: originalPath,
		content:      f.content,
		mode:         mode,
		flag:         flag,
	}

	if IsTruncate(flag) {
		new.content.Truncate(0)
	}

	if IsAppend(flag) {
		new.position = int64(new.content.Len())
	}

	return new
}

func (f *inMemfile) FileInfo() core.FileInfo {
	return core.FileInfo{
		BaseName_:       f.basename,
		AbsPath_:        f.absPath,
		Mode_:           core.FileMode(f.mode),
		Size_:           core.ByteCount(f.content.Len()),
		ModTime_:        core.Date(f.content.ModifTime()),
		HasCreationTime: true,
		CreationTime_:   core.Date(f.content.creationTime),
	}
}

func (f *inMemfile) Stat() (os.FileInfo, error) {
	return f.FileInfo(), nil
}

// Lock is a no-op in memfs.
func (f *inMemfile) Lock() error {
	return nil
}

// Unlock is a no-op in memfs.
func (f *inMemfile) Unlock() error {
	return nil
}
