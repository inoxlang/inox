package fs_ns

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"

	fsutil "github.com/go-git/go-billy/v5/util"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
)

var _ = []core.Effect{&CreateFile{}, &CreateDir{}, &RemoveFile{}}

//

type CreateFile struct {
	path    core.Path
	applied bool
	content []byte
	fmode   core.FileMode
}

func (e CreateFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e CreateFile) PermissionKind() core.PermissionKind {
	return permkind.Create
}

func (e CreateFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e CreateFile) IsApplied() bool {
	return e.applied
}

func (e *CreateFile) Apply(ctx *core.Context) error {
	if e.applied {
		return nil
	}
	e.applied = true
	return __createFile(ctx, e.path, e.content, fs.FileMode(e.fmode))
}

func (e CreateFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	return Remove(ctx, e.path)
}

func (e CreateFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permkind.Create, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type AppendBytesToFile struct {
	path    core.Path
	applied bool
	content []byte
}

func (e AppendBytesToFile) PermissionKind() core.PermissionKind {
	return permkind.Update
}

func (e AppendBytesToFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e AppendBytesToFile) IsApplied() bool {
	return e.applied
}

func (e *AppendBytesToFile) Apply(ctx *core.Context) error {

	if e.applied {
		return nil
	}

	if err := e.CheckPermissions(ctx); err != nil {
		return err
	}

	fls := ctx.GetFileSystem()
	e.applied = true

	f, err := fls.OpenFile(string(e.path), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to append to file: failed to open file: %s", err.Error())
	}

	defer f.Close()

	_, err = f.Write(e.content)
	if err != nil {
		return fmt.Errorf("failed to append to file: %s", err.Error())
	}

	return nil
}

func (e AppendBytesToFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	fls := ctx.GetFileSystem()

	f, err := fls.OpenFile(string(e.path), os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to reverse data append to file: failed to open file: %s", err.Error())
	}

	defer f.Close()

	info, err := core.FileStat(f)
	if err != nil {
		return fmt.Errorf("failed to reverse data append to file: failed to get information for file: %s", err.Error())
	}

	previousSize := info.Size() - int64(len(e.content))
	return f.Truncate(previousSize)
}

func (e AppendBytesToFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permkind.Update, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type CreateDir struct {
	path    core.Path
	applied bool
	fmode   core.FileMode
}

func (e CreateDir) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e CreateDir) PermissionKind() core.PermissionKind {
	return permkind.Create
}

func (e CreateDir) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e CreateDir) IsApplied() bool {
	return e.applied
}

func (e *CreateDir) Apply(ctx *core.Context) error {
	fls := ctx.GetFileSystem()

	if e.applied {
		return nil
	}
	if err := ctx.CheckHasPermission(core.FilesystemPermission{Kind_: permkind.Create, Entity: e.path}); err != nil {
		return err
	}
	e.applied = true
	return fls.MkdirAll(e.path.UnderlyingString(), fs.FileMode(e.fmode))
}

func (e CreateDir) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	return Remove(ctx, e.path)
}

func (e CreateDir) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permkind.Create, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

// RemoveFile is an effect removing a file (regular file, directory, ...).
type RemoveFile struct {
	path       core.Path
	applied    bool
	reversible bool
	save       core.Path
}

func (e RemoveFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e RemoveFile) PermissionKind() core.PermissionKind {
	return permkind.Create
}

func (e RemoveFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e RemoveFile) IsApplied() bool {
	return e.applied
}

func (e *RemoveFile) Apply(ctx *core.Context) error {
	if e.applied {
		return nil
	}
	if err := e.CheckPermissions(ctx); err != nil {
		return err
	}

	e.applied = true
	fls := ctx.GetFileSystem()

	if e.reversible { //if the effect is reversible we move the file instead of deleting it
		tempDir := ctx.GetTempDir()
		name := url.PathEscape(e.path.UnderlyingString())
		e.save = core.Path(fls.Join(tempDir.UnderlyingString(), name))
		return fls.Rename(e.path.UnderlyingString(), e.save.UnderlyingString())
	} else {
		return fsutil.RemoveAll(fls, e.path.UnderlyingString())
	}
}

func (e *RemoveFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}
	e.applied = true
	fls := ctx.GetFileSystem()

	if e.reversible {
		return fls.Rename(e.save.UnderlyingString(), e.path.UnderlyingString())
	}

	return core.ErrIrreversible
}

func (e RemoveFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permkind.Delete, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type RenameFile struct {
	old, new core.Path
	applied  bool
}

func (e RenameFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.old}
}

func (e RenameFile) PermissionKind() core.PermissionKind {
	return permkind.Create
}

func (e RenameFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e RenameFile) IsApplied() bool {
	return e.applied
}

func (e *RenameFile) Apply(ctx *core.Context) error {

	if e.applied {
		return nil
	}
	e.applied = true

	fls := ctx.GetFileSystem()

	if _, err := fls.Stat(string(e.old)); os.IsNotExist(err) {
		return fmt.Errorf("rename: %w", err)
	}

	if _, err := fls.Stat(string(e.new)); err == nil {
		return fmt.Errorf("rename: a file already exists at %s", e.new.ResourceName())
	}

	return fls.Rename(string(e.old), string(e.new))
}

func (e RenameFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	fls := ctx.GetFileSystem()

	return fls.Rename(string(e.new), string(e.old))
}

func (e RenameFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permkind.Read, Entity: e.old}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	perm = core.FilesystemPermission{Kind_: permkind.Create, Entity: e.new}
	return ctx.CheckHasPermission(perm)
}
