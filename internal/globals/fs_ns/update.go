package fs_ns

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
)

// this file contains functions to update files.

// Rename renames a file, it returns an error if the renamed file does not exist or it a file already has the new name.
func Rename(ctx *core.Context, old, new core.Path) error {
	{
		var err error
		old, err = old.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return err
		}
		new, err = new.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return err
		}
	}

	effect := &RenameFile{
		old: old,
		new: new,
	}

	if err := effect.CheckPermissions(ctx); err != nil {
		return err
	}

	if tx := ctx.GetTx(); tx != nil {
		return tx.AddEffect(ctx, effect)
	} else {
		return effect.Apply(ctx)
	}
}

func AppendToFile(ctx *core.Context, args ...core.Value) error {
	var fpath core.Path
	var content *core.Reader
	fls := ctx.GetFileSystem()

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Path:
			if fpath != "" {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			fpath = v
		case core.Readable:
			if content != nil {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("content")
			}
			content = v.Reader()
		default:
			return errors.New("invalid argument " + fmt.Sprintf("%#v", v))
		}
	}

	if fpath == "" {
		return errors.New("missing path argument")
	}

	fpath, err := fpath.ToAbs(ctx.GetFileSystem())
	if err != nil {
		return err
	}

	b, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("cannot append to file: %s", err.Error())
	}

	_, err = fls.Stat(string(fpath))
	if os.IsNotExist(err) {
		return fmt.Errorf("cannot append to file: %s does not exist", fpath)
	}

	effect := AppendBytesToFile{path: fpath, content: b}

	if err := effect.CheckPermissions(ctx); err != nil {
		return err
	}

	return effect.Apply(ctx)
}

func ReplaceFileContent(ctx *core.Context, args ...core.Value) error {
	var fpath core.Path
	var content *core.Reader
	fls := ctx.GetFileSystem()

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Path:
			if fpath != "" {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			fpath = v
		case core.Readable:
			if content != nil {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("content")
			}
			content = v.Reader()
		default:
			return errors.New("invalid argument " + fmt.Sprintf("%#v", v))
		}
	}

	if fpath == "" {
		return commonfmt.FmtMissingArgument("path")
	}

	fpath, err := fpath.ToAbs(ctx.GetFileSystem())
	if err != nil {
		return err
	}

	b, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("cannot append to file: %s", err.Error())
	}

	//TODO: use an effect

	perm := core.FilesystemPermission{
		Kind_:  permbase.Update,
		Entity: fpath,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	f, err := fls.OpenFile(string(fpath), os.O_WRONLY|os.O_TRUNC, 0)
	defer func() {
		f.Close()
	}()

	if err != nil {
		return err
	}

	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	_, err = f.Write(b)
	return err
}

func Remove(ctx *core.Context, args ...core.Value) error {

	var fpath core.Path
	for _, arg := range args {
		switch v := arg.(type) {
		case core.Path:
			if fpath != "" {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			var err error
			fpath, err = v.ToAbs(ctx.GetFileSystem())
			if err != nil {
				return err
			}
		default:
			return errors.New("invalid argument " + fmt.Sprintf("%#v", v))
		}
	}

	if fpath == "" {
		return commonfmt.FmtMissingArgument("path")
	}

	effect := RemoveFile{path: fpath}
	if err := effect.CheckPermissions(ctx); err != nil {
		return err
	}

	if tx := ctx.GetTx(); tx != nil {
		effect.reversible = true
		return tx.AddEffect(ctx, &effect)
	} else {
		return effect.Apply(ctx)
	}
}
