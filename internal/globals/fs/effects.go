package internal

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"

	core "github.com/inoxlang/inox/internal/core"
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
	return core.CreatePerm
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
	perm := core.FilesystemPermission{Kind_: core.CreatePerm, Entity: e.path}
	return ctx.CheckHasPermission(perm)
}

//

type AppendBytesToFile struct {
	path    core.Path
	applied bool
	content []byte
}

func (e AppendBytesToFile) PermissionKind() core.PermissionKind {
	return core.UpdatePerm
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

	e.applied = true

	f, err := os.OpenFile(string(e.path), os.O_APPEND|os.O_WRONLY, 0o600)
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

	f, err := os.OpenFile(string(e.path), os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to reverse data append to file: failed to open file: %s", err.Error())
	}

	defer f.Close()

	info, _ := f.Stat()

	previousSize := info.Size() - int64(len(e.content))
	return f.Truncate(previousSize)
}

func (e AppendBytesToFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: core.UpdatePerm, Entity: e.path}
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
	return core.CreatePerm
}

func (e CreateDir) Reversability(*core.Context) core.Reversability {
	return core.SomewhatReversible
}

func (e CreateDir) IsApplied() bool {
	return e.applied
}

func (e *CreateDir) Apply(ctx *core.Context) error {
	if e.applied {
		return nil
	}
	if err := ctx.CheckHasPermission(core.FilesystemPermission{Kind_: core.CreatePerm, Entity: e.path}); err != nil {
		return err
	}
	e.applied = true
	return os.Mkdir(e.path.UnderlyingString(), fs.FileMode(e.fmode))
}

func (e CreateDir) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	return Remove(ctx, e.path)
}

func (e CreateDir) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: core.CreatePerm, Entity: e.path}
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
	return core.CreatePerm
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

	if e.reversible { //if the effect is reversible we move the folder instead of deleting it
		tempDir := ctx.GetTempDir()
		name := url.PathEscape(e.path.UnderlyingString())
		e.save = core.Path(fls.Join(tempDir.UnderlyingString(), name))
		return os.Rename(e.path.UnderlyingString(), e.save.UnderlyingString())
	} else {
		return os.RemoveAll(e.path.UnderlyingString())
	}
}

func (e *RemoveFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}
	e.applied = true

	if e.reversible {
		return os.Rename(e.save.UnderlyingString(), e.path.UnderlyingString())
	}

	return core.ErrIrreversible
}

func (e RemoveFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: core.DeletePerm, Entity: e.path}
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
	return core.CreatePerm
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

	if _, err := os.Stat(string(e.old)); os.IsNotExist(err) {
		return fmt.Errorf("rename: %w", err)
	}

	if _, err := os.Stat(string(e.new)); err == nil {
		return fmt.Errorf("rename: a file already exists at %s", e.new.ResourceName())
	}

	return os.Rename(string(e.old), string(e.new))
}

func (e RenameFile) Reverse(ctx *core.Context) error {
	if !e.applied {
		return nil
	}

	return os.Rename(string(e.new), string(e.old))
}

func (e RenameFile) CheckPermissions(ctx *core.Context) error {
	perm := core.FilesystemPermission{Kind_: core.ReadPerm, Entity: e.old}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	perm = core.FilesystemPermission{Kind_: core.CreatePerm, Entity: e.new}
	return ctx.CheckHasPermission(perm)
}
