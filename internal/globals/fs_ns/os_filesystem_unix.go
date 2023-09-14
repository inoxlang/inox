//go:build unix

package fs_ns

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"

	afs "github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
)

var (
	osFs = &OsFilesystem{
		OS: *osfs.Default,
	}

	_ afs.Filesystem = osFs
	_                = core.IWithSecondaryContext((*OsFilesystem)(nil))
)

type OsFilesystem struct {
	osfs.OS
	ctx *core.Context //used in .DoWithContext
}

// we override Rename because osfs.OS.Rename is not the same as os.Rename
func (fs *OsFilesystem) Rename(from, to string) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return os.Rename(from, to)
}

func (fs OsFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fs OsFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}

func (fs OsFilesystem) Absolute(path string) (string, error) {
	return filepath.Abs(path)
}

func (fs *OsFilesystem) WithSecondaryContext(ctx *core.Context) any {
	if ctx == nil {
		panic(errors.New("nil context"))
	}
	if fs.ctx == ctx {
		return fs
	}
	osWithCtx := new(OsFilesystem)
	*osWithCtx = *fs
	osWithCtx.ctx = ctx
	return osWithCtx
}

func (fs *OsFilesystem) WithoutSecondaryContext() any {
	return osFs
}

func (fs *OsFilesystem) Create(filename string) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Create(filename)
}

func (fs *OsFilesystem) Open(filename string) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Open(filename)
}

func (fs *OsFilesystem) OpenFile(filename string, flag int, perm fs.FileMode) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.OpenFile(filename, flag, perm)
}

func (fs *OsFilesystem) Remove(filename string) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Remove(filename)
}

func (fs *OsFilesystem) Stat(filename string) (fs.FileInfo, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Lstat(filename)
}

func (fs *OsFilesystem) Lstat(filename string) (fs.FileInfo, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Lstat(filename)
}

func (fs *OsFilesystem) MkdirAll(filename string, perm fs.FileMode) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.MkdirAll(filename, perm)
}

func (fs *OsFilesystem) ReadDir(path string) ([]fs.FileInfo, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.ReadDir(path)
}

func (fs *OsFilesystem) Readlink(link string) (string, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Readlink(link)
}

func (fs *OsFilesystem) Symlink(target string, link string) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.Symlink(target, link)
}

func (fs *OsFilesystem) TempFile(dir string, prefix string) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDecrementation()
		defer fs.ctx.ResumeCPUTimeDecrementation()
	}
	return fs.OS.TempFile(dir, prefix)
}

func GetOsFilesystem() *OsFilesystem {
	return osFs
}
