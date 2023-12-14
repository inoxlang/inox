package fs_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"

	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

// this file contains functions to read & search files and directories.

var (
	READFILE_ARG_NAMES   = []string{"filepath"}
	READFILE_SYMB_PARAMS = &[]symbolic.Value{symbolic.ANY_DIR_PATH}

	LISTFILES_ARG_NAMES   = []string{"path-or-pattern"}
	LISTFILES_SYMB_PARAMS = &[]symbolic.Value{symbolic.NewMultivalue(symbolic.ANY_PATH, symbolic.ANY_PATH_PATTERN)}
)

// ReadFile expects a core.Path argument, it reads the whole content of a file.
func ReadFile(ctx *core.Context, fpath core.Path) (*core.ByteSlice, error) {
	if fpath == "" {
		return &core.ByteSlice{}, commonfmt.FmtMissingArgument("path")
	}

	b, err := ReadEntireFile(ctx, fpath)
	return &core.ByteSlice{Bytes: b, IsDataMutable: true}, err
}

func ReadEntireFile(ctx *core.Context, fpath core.Path) ([]byte, error) {
	fpath, err := fpath.ToAbs(ctx.GetFileSystem())
	if err != nil {
		return nil, err
	}

	perm := core.FilesystemPermission{Kind_: permkind.Read, Entity: fpath}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	f, err := openExistingFile(ctx, fpath, false)
	if err != nil {
		return nil, err
	}

	return f.doRead(ctx, true, -1)
}

func ReadDir(ctx *core.Context, pth core.Path) ([]fs.DirEntry, error) {
	fls := ctx.GetFileSystem()

	pth, err := pth.ToAbs(fls)
	if err != nil {
		return nil, err
	}

	perm := core.FilesystemPermission{
		Kind_:  permkind.Read,
		Entity: pth,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	entries, err := fls.ReadDir(string(pth))

	if err != nil {
		return nil, err
	}

	return utils.MapSlice(entries, func(i fs.FileInfo) fs.DirEntry {
		return core.NewStatDirEntry(i)
	}), nil
}

func makeFileInfo(info fs.FileInfo, pth string, fls afs.Filesystem) core.FileInfo {
	if info.IsDir() {
		pth = core.AppendTrailingSlashIfNotPresent(pth)
	}

	absPath, err := core.Path(pth).ToAbs(fls)
	if err != nil {
		panic(err)
	}

	return core.FileInfo{
		BaseName_: info.Name(),
		AbsPath_:  absPath,
		Size_:     core.ByteCount(info.Size()),
		Mode_:     core.FileMode(info.Mode()),
		ModTime_:  core.DateTime(info.ModTime()),
	}
}

func Read(ctx *core.Context, path core.Path, args ...core.Value) (result core.Value, finalErr error) {
	doParse := true
	validateRaw := false
	var contentType core.Mimetype

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Mimetype:
			if contentType != "" {
				finalErr = commonfmt.FmtErrXProvidedAtLeastTwice("content type")
				return
			}
			contentType = v
		case core.Option:
			if v.Name == "raw" {
				if v.Value == core.True {
					doParse = false
				}
			} else {
				return nil, fmt.Errorf("invalid argument %#v", arg)
			}
		default:
			return nil, fmt.Errorf("invalid argument %#v", arg)
		}
	}

	if path.IsDirPath() {
		_res, lsErr := ListFiles(ctx, &core.OptionalParam[core.Value]{Value: path})
		if lsErr != nil {
			finalErr = lsErr
			return
		}

		result = core.ConvertReturnValue(reflect.ValueOf(_res))
		return
	} else {
		var _err error
		b, _err := ReadEntireFile(ctx, path)
		if _err != nil {
			finalErr = _err
			return
		}

		t, ok := core.GetMimeTypeFromExtension(filepath.Ext(string(path)))
		if ok {
			contentType = t
		}
		val, _, err := core.ParseOrValidateResourceContent(ctx, b, contentType, doParse, validateRaw)
		return val, err
	}
}

func ListFiles(ctx *core.Context, pathOrPatt *core.OptionalParam[core.Value]) ([]core.FileInfo, error) {
	fls := ctx.GetFileSystem()

	var pth core.Path
	var patt core.PathPattern

	if pathOrPatt != nil {
		path, ok := pathOrPatt.Value.(core.Path)
		if ok {
			pth = path
		} else {
			patt = pathOrPatt.Value.(core.PathPattern)
		}
	}

	if pth == "" && patt == "" {
		pth = "./"
	}

	if pth != "" {
		var err error
		pth, err = pth.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return nil, err
		}

		if !pth.IsDirPath() {
			return nil, errors.New("only directory paths are supported : " + string(pth))
		}
	}

	resultFileInfo := make([]core.FileInfo, 0)

	if pth != "" {

		entries, err := ReadDir(ctx, pth)
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			fpath := path.Join(string(pth), entry.Name())
			info, err := fls.Stat(fpath)
			if err != nil {
				return nil, err
			}

			resultFileInfo = append(resultFileInfo, makeFileInfo(info, fpath, fls))
		}
	} else { //pattern
		absPatt := patt.ToAbs(ctx.GetFileSystem())
		perm := core.FilesystemPermission{
			Kind_:  permkind.Read,
			Entity: absPatt,
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			return nil, err
		}

		matches, err := glob(fls, string(absPatt))

		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			info, err := fls.Stat(match)
			if err != nil {
				return nil, err
			}

			resultFileInfo = append(resultFileInfo, makeFileInfo(info, match, fls))
		}
	}

	return resultFileInfo, nil
}

func IsDir(ctx *core.Context, pth core.Path) core.Bool {
	fls := ctx.GetFileSystem()
	pth, err := pth.ToAbs(fls)
	if err != nil {
		panic(err)
	}

	perm := core.FilesystemPermission{
		Kind_:  permkind.Read,
		Entity: pth,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		panic(err)
	}

	stat, err := fls.Lstat(string(pth))
	return core.Bool(!os.IsNotExist(err) && stat.IsDir())
}

func IsFile(ctx *core.Context, pth core.Path) core.Bool {
	fls := ctx.GetFileSystem()
	pth, err := pth.ToAbs(fls)
	if err != nil {
		panic(err)
	}

	perm := core.FilesystemPermission{
		Kind_:  permkind.Read,
		Entity: pth,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		panic(err)
	}

	stat, err := fls.Lstat(string(pth))
	return core.Bool(!os.IsNotExist(err) && stat.Mode().IsRegular())
}

func Exists(ctx *core.Context, pth core.Path) core.Bool {
	fls := ctx.GetFileSystem()
	pth, err := pth.ToAbs(fls)
	if err != nil {
		panic(err)
	}

	perm := core.FilesystemPermission{
		Kind_:  permkind.Read,
		Entity: pth,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		panic(err)
	}

	_, err = fls.Lstat(string(pth))
	return core.Bool(!os.IsNotExist(err))
}

func GetTreeData(ctx *core.Context, path core.Path) *core.Treedata {
	if !path.IsDirPath() {
		//TODO: improve error
		panic(core.FmtErrInvalidArgumentAtPos(path, 0))
	}
	return core.GetDirTreeData(ctx.GetFileSystem(), path)
}

func computeChunkSize(rate core.ByteRate, fileSize int) int {
	//we divide the rate to allow cancellation //TODO: update
	chunkSize := int(rate / 10)

	//we cannot read more bytes than the size of file | write more bytes than the final file's size
	chunkSize = min(fileSize, chunkSize)

	return chunkSize
}
