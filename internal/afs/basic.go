package afs

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
)

var _ billy.Basic = (*GenericBasic)(nil)
var _ billy.File = (*GenericFile)(nil)

// GenericBasic is an implementation of billy.Basic. All the fields are optional:
// for example if .CreateFn is not provided the Create method will always return billy.ErrNotSupported.
type GenericBasic struct {
	CreateFn   func(filename string) (File, error)
	OpenFn     func(filename string) (File, error)
	OpenFileFn func(filename string, flag int, perm os.FileMode) (File, error)
	StatFn     func(filename string) (os.FileInfo, error)
	RenameFn   func(oldpath, newpath string) error
	RemoveFn   func(filename string) error
	JoinFn     func(elem ...string) string
}

func (i GenericBasic) Create(filename string) (File, error) {
	if i.CreateFn == nil {
		return nil, billy.ErrNotSupported
	}
	return i.CreateFn(filename)
}

func (i GenericBasic) Open(filename string) (File, error) {
	if i.OpenFn == nil {
		return nil, billy.ErrNotSupported
	}
	return i.OpenFn(filename)
}

func (i GenericBasic) OpenFile(filename string, flag int, perm os.FileMode) (File, error) {
	if i.OpenFileFn == nil {
		return nil, billy.ErrNotSupported
	}
	return i.OpenFileFn(filename, flag, perm)
}

func (i GenericBasic) Stat(filename string) (os.FileInfo, error) {
	if i.StatFn == nil {
		return nil, billy.ErrNotSupported
	}
	return i.StatFn(filename)
}

func (i GenericBasic) Rename(oldpath, newpath string) error {
	if i.RenameFn == nil {
		return billy.ErrNotSupported
	}
	return i.RenameFn(oldpath, newpath)
}

func (i GenericBasic) Remove(filename string) error {
	if i.RemoveFn == nil {
		return billy.ErrNotSupported
	}
	return i.RemoveFn(filename)
}

func (i GenericBasic) Join(elem ...string) string {
	if i.JoinFn == nil {
		return filepath.Join(elem...)
	}
	return i.JoinFn(elem...)
}

// GenericFile is an implementation of billy.File. All the fields are optional:
// for example if .WriteFn is not provided the Write method will always return billy.ErrNotSupported.
type GenericFile struct {
	Filename   string
	WriteFn    func(p []byte) (n int, err error)
	ReadFn     func(p []byte) (n int, err error)
	ReadAtFn   func(p []byte, off int64) (n int, err error)
	SeekFn     func(offset int64, whence int) (int64, error)
	CloseFn    func() error
	LockFn     func() error
	UnlockFn   func() error
	TruncateFn func(size int64) error
}

func (f *GenericFile) Name() string {
	return f.Filename
}

func (f *GenericFile) Write(p []byte) (n int, err error) {
	if f.WriteFn == nil {
		return 0, billy.ErrNotSupported
	}
	return f.WriteFn(p)
}

func (f *GenericFile) Read(p []byte) (n int, err error) {
	if f.ReadFn == nil {
		return 0, billy.ErrNotSupported
	}
	return f.ReadFn(p)
}

func (f *GenericFile) ReadAt(p []byte, off int64) (n int, err error) {
	if f.ReadAtFn == nil {
		return 0, billy.ErrNotSupported
	}
	return f.ReadAtFn(p, off)
}

func (f *GenericFile) Seek(offset int64, whence int) (int64, error) {
	if f.SeekFn == nil {
		return 0, billy.ErrNotSupported
	}
	return f.SeekFn(offset, whence)
}

func (f *GenericFile) Close() error {
	if f.CloseFn == nil {
		return billy.ErrNotSupported
	}
	return f.CloseFn()
}

func (f *GenericFile) Lock() error {
	if f.LockFn == nil {
		return billy.ErrNotSupported
	}
	return f.LockFn()
}

func (f *GenericFile) Unlock() error {
	if f.UnlockFn == nil {
		return billy.ErrNotSupported
	}
	return f.UnlockFn()
}

func (f *GenericFile) Truncate(size int64) error {
	if f.UnlockFn == nil {
		return billy.ErrNotSupported
	}
	return f.TruncateFn(size)
}
