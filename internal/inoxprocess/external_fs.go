package inoxprocess

import (
	"io/fs"

	billy "github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
)

var (
	_ = afs.Filesystem((*ExternalFilesystem)(nil))
)

// A ExternalFilesystem is a filesystem provided by another process.
type ExternalFilesystem struct {
	client *ControlClient
}

func (fls *ExternalFilesystem) Absolute(path string) (string, error) {
	if core.PathFrom(path).IsAbsolute() {
		return path, nil
	}
	return "", core.ErrNotImplemented
}

// Create implements afs.Filesystem.
func (*ExternalFilesystem) Create(filename string) (billy.File, error) {
	panic("unimplemented")
}

// Join implements afs.Filesystem.
func (*ExternalFilesystem) Join(elem ...string) string {
	panic("unimplemented")
}

// Lstat implements afs.Filesystem.
func (*ExternalFilesystem) Lstat(filename string) (fs.FileInfo, error) {
	panic("unimplemented")
}

// MkdirAll implements afs.Filesystem.
func (*ExternalFilesystem) MkdirAll(filename string, perm fs.FileMode) error {
	panic("unimplemented")
}

// Open implements afs.Filesystem.
func (*ExternalFilesystem) Open(filename string) (billy.File, error) {
	panic("unimplemented")
}

// OpenFile implements afs.Filesystem.
func (*ExternalFilesystem) OpenFile(filename string, flag int, perm fs.FileMode) (billy.File, error) {
	panic("unimplemented")
}

// ReadDir implements afs.Filesystem.
func (*ExternalFilesystem) ReadDir(path string) ([]fs.FileInfo, error) {
	panic("unimplemented")
}

// Readlink implements afs.Filesystem.
func (*ExternalFilesystem) Readlink(link string) (string, error) {
	panic("unimplemented")
}

// Remove implements afs.Filesystem.
func (*ExternalFilesystem) Remove(filename string) error {
	panic("unimplemented")
}

// Rename implements afs.Filesystem.
func (*ExternalFilesystem) Rename(oldpath string, newpath string) error {
	panic("unimplemented")
}

// Stat implements afs.Filesystem.
func (*ExternalFilesystem) Stat(filename string) (fs.FileInfo, error) {
	panic("unimplemented")
}

// Symlink implements afs.Filesystem.
func (*ExternalFilesystem) Symlink(target string, link string) error {
	panic("unimplemented")
}

// TempFile implements afs.Filesystem.
func (*ExternalFilesystem) TempFile(dir string, prefix string) (billy.File, error) {
	panic("unimplemented")
}

func (*ExternalFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

// Root implements afs.Filesystem.
func (*ExternalFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}
