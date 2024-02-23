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
	"github.com/inoxlang/inox/internal/core"
)

var (
	osFs = &OsFilesystem{
		ChrootOS: osfs.Default,
	}

	_ afs.Filesystem = osFs
	_                = core.IWithSecondaryContext((*OsFilesystem)(nil))
)

type OsFilesystem struct {
	*osfs.ChrootOS
	ctx *core.Context //used in .DoWithContext
}

func (fs *OsFilesystem) OsFs() {}

// we override Rename because osfs.ChrootOS.Rename is not the same as os.Rename
func (fs *OsFilesystem) Rename(from, to string) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
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
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	//we don't call fs.ChrootOS.Create because it uses a default create mode of 600.
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, afs.DEFAULT_CREATE_FPERM)
}

func (fs *OsFilesystem) Open(filename string) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.Open(filename)
}

func (fs *OsFilesystem) OpenFile(filename string, flag int, perm fs.FileMode) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.OpenFile(filename, flag, perm)
}

func (fs *OsFilesystem) Remove(filename string) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.Remove(filename)
}

func (fs *OsFilesystem) Stat(filename string) (fs.FileInfo, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.Lstat(filename)
}

func (fs *OsFilesystem) Lstat(filename string) (fs.FileInfo, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.Lstat(filename)
}

func (fs *OsFilesystem) MkdirAll(filename string, perm fs.FileMode) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.MkdirAll(filename, perm)
}

func (fs *OsFilesystem) ReadDir(path string) ([]fs.FileInfo, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.ReadDir(path)
}

func (fs *OsFilesystem) Readlink(link string) (string, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.Readlink(link)
}

func (fs *OsFilesystem) Symlink(target string, link string) error {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.Symlink(target, link)
}

func (fs *OsFilesystem) TempFile(dir string, prefix string) (billy.File, error) {
	if fs.ctx != nil {
		fs.ctx.PauseCPUTimeDepletion()
		defer fs.ctx.ResumeCPUTimeDepletion()
	}
	return fs.ChrootOS.TempFile(dir, prefix)
}

func GetOsFilesystem() *OsFilesystem {
	return osFs
}
