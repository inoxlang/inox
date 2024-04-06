package fs_ns

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"

	fsutil "github.com/go-git/go-billy/v5/util"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
)

var _ = []core.Effect{(*CreateFile)(nil), (*CreateDir)(nil), (*RemoveFile)(nil), (*RenameFile)(nil), (*AppendBytesToFile)(nil)}

//

type CreateFile struct {
	path              core.Path
	applying, applied bool
	content           []byte
	fmode             core.FileMode
}

func (e CreateFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e CreateFile) PermissionKind() core.PermissionKind {
	return permbase.Create
}

func (e CreateFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e CreateFile) IsApplied() bool {
	return e.applied
}

func (e CreateFile) IsApplying() bool {
	return e.applying
}

func (e *CreateFile) Apply(ctx *core.Context) error {
	if e.applied || e.applying {
		return nil
	}
	defer func() {
		e.applying = false
	}()
	e.applying = true
	err := __createFile(ctx, e.path, e.content, fs.FileMode(e.fmode))

	if err == nil {
		e.applied = true
	}
	return err
}

func (e CreateFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	return Remove(ctx, e.path)
}

func (e CreateFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permbase.Create, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type AppendBytesToFile struct {
	path              core.Path
	applying, applied bool
	content           []byte
}

func (e AppendBytesToFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e AppendBytesToFile) PermissionKind() core.PermissionKind {
	return permbase.Update
}

func (e AppendBytesToFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e AppendBytesToFile) IsApplied() bool {
	return e.applied
}

func (e AppendBytesToFile) IsApplying() bool {
	return e.applying
}

func (e *AppendBytesToFile) Apply(ctx *core.Context) error {
	if e.applied || e.applying {
		return nil
	}
	e.applying = true
	defer func() {
		e.applying = false
	}()

	if err := e.CheckPermissions(ctx); err != nil {
		return err
	}

	fls := ctx.GetFileSystem()

	f, err := fls.OpenFile(string(e.path), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to append to file: failed to open file: %s", err.Error())
	}

	defer f.Close()

	_, err = f.Write(e.content)
	if err != nil {
		return fmt.Errorf("failed to append to file: %s", err.Error())
	}

	e.applied = true
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

	info, err := afs.FileStat(f, fls)
	if err != nil {
		return fmt.Errorf("failed to reverse data append to file: failed to get information for file: %s", err.Error())
	}

	previousSize := info.Size() - int64(len(e.content))
	return f.Truncate(previousSize)
}

func (e AppendBytesToFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permbase.Update, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type CreateDir struct {
	path              core.Path
	applying, applied bool
	fmode             core.FileMode
}

func (e CreateDir) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e CreateDir) PermissionKind() core.PermissionKind {
	return permbase.Create
}

func (e CreateDir) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e CreateDir) IsApplied() bool {
	return e.applied
}

func (e CreateDir) IsApplying() bool {
	return e.applying
}

func (e *CreateDir) Apply(ctx *core.Context) error {
	fls := ctx.GetFileSystem()

	if e.applied || e.applying {
		return nil
	}
	e.applying = true
	defer func() {
		e.applying = false
	}()

	if err := ctx.CheckHasPermission(core.FilesystemPermission{Kind_: permbase.Create, Entity: e.path}); err != nil {
		return err
	}

	err := fls.MkdirAll(e.path.UnderlyingString(), fs.FileMode(e.fmode))

	if err == nil {
		e.applied = true
	}
	return err
}

func (e CreateDir) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	return Remove(ctx, e.path)
}

func (e CreateDir) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permbase.Create, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

// RemoveFile is an effect removing a file (regular file, directory, ...).
type RemoveFile struct {
	path              core.Path
	applying, applied bool
	reversible        bool
	save              core.Path
}

func (e RemoveFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.path}
}

func (e RemoveFile) PermissionKind() core.PermissionKind {
	return permbase.Create
}

func (e RemoveFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e RemoveFile) IsApplied() bool {
	return e.applied
}

func (e RemoveFile) IsApplying() bool {
	return e.applying
}

func (e *RemoveFile) Apply(ctx *core.Context) (finalErr error) {
	if e.applied || e.applying {
		return nil
	}
	if err := e.CheckPermissions(ctx); err != nil {
		return err
	}

	e.applying = true
	defer func() {
		e.applying = false
	}()

	fls := ctx.GetFileSystem()

	defer func() {
		if finalErr == nil {
			e.applied = true
		}
	}()

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
	perm := core.FilesystemPermission{Kind_: permbase.Delete, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type RenameFile struct {
	old, new          core.Path
	applying, applied bool
}

func (e RenameFile) Resources() []core.ResourceName {
	return []core.ResourceName{e.old}
}

func (e RenameFile) PermissionKind() core.PermissionKind {
	return permbase.Create
}

func (e RenameFile) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e RenameFile) IsApplied() bool {
	return e.applied
}

func (e RenameFile) IsApplying() bool {
	return e.applying
}

func (e *RenameFile) Apply(ctx *core.Context) error {
	if e.applied || e.applying {
		return nil
	}
	e.applying = true
	defer func() {
		e.applying = false
	}()

	fls := ctx.GetFileSystem()

	if _, err := fls.Stat(string(e.old)); os.IsNotExist(err) {
		return fmt.Errorf("rename: %w", err)
	}

	if _, err := fls.Stat(string(e.new)); err == nil {
		return fmt.Errorf("rename: a file already exists at %s", e.new.ResourceName())
	}

	err := fls.Rename(string(e.old), string(e.new))
	if err == nil {
		e.applied = true
	}
	return err
}

func (e RenameFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	fls := ctx.GetFileSystem()

	return fls.Rename(string(e.new), string(e.old))
}

func (e RenameFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: permbase.Read, Entity: e.old}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	perm = core.FilesystemPermission{Kind_: permbase.Create, Entity: e.new}
	return ctx.CheckHasPermission(perm)
}
