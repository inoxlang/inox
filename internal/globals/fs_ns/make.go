package fs_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
)

// this file contains functions that create files & directories.

const (
	MAX_FILE_HIERARCHY_DEPTH = 5
)

var (
	ErrTooDeepFileHierarchy = errors.New("file hierarchy is too deep")

	MKDIR_ARG_NAMES   = []string{"dirpath", "content"}
	MKDIR_SYMB_PARAMS = &[]symbolic.SymbolicValue{symbolic.ANY_DIR_PATH, symbolic.ANY_DICT}

	MKFILE_ARG_NAMES   = []string{"filepath", "content"}
	MKFILE_SYMB_PARAMS = &[]symbolic.SymbolicValue{symbolic.ANY_NON_DIR_PATH, symbolic.ANY_READABLE}
)

// Mkdir expects a core.Path argument and creates a directory.
// If a dictionary is passed a file hiearchy will also be created.
func Mkdir(ctx *core.Context, dirpath core.Path, content *core.OptionalParam[*core.Dictionary]) error {
	var contentDesc *core.Dictionary

	if !dirpath.IsDirPath() {
		return fmt.Errorf("path argument is a file path : %s, the path should end with '/'", string(dirpath))
	}

	if content != nil && content.Value != nil {
		contentDesc = content.Value
		visit := func(val core.Value) (parse.TraversalAction, error) {
			switch val.(type) {
			case *core.List, core.Path, *core.Dictionary, core.StringLike, core.BytesLike:
			default:
				return parse.Continue, fmt.Errorf("invalid content description: it contains a value of type %T", val)
			}
			return parse.Continue, nil
		}

		err := contentDesc.ForEachEntry(ctx, func(keyRepr string, key, v core.Serializable) error {
			return core.Traverse(v, visit, core.TraversalConfiguration{MaxDepth: MAX_FILE_HIERARCHY_DEPTH})
		})
		if err != nil {
			return err
		}
	}

	return makeFileHierarchy(ctx, makeFileHieararchyParams{
		key:     dirpath,
		content: contentDesc,
		depth:   0,
	})
}

type makeFileHieararchyParams struct {
	//The filesystem where to create the hierarchy, if not nil permissions are not checked and the current transaction is ignored.
	//If nil the context's filesystem will be used and perlission will be checked.
	fls *MemFilesystem

	// key represents the filename (or dirname) and content described how to make the file.
	key core.Path

	// The content of a directory is described by a List or a Dictionary.
	// The content of a regular file is described by a string (Str) or bytes (IBytes).
	content core.Value

	depth int
}

// makeFileHierarchy recursively creates folders and files.
func makeFileHierarchy(ctx *core.Context, args makeFileHieararchyParams) error {
	if args.depth > MAX_FILE_HIERARCHY_DEPTH {
		return ErrTooDeepFileHierarchy
	}

	key := args.key
	content := args.content
	var fls afs.Filesystem = args.fls
	if args.fls == nil {
		fls = ctx.GetFileSystem()
	}

	if key.IsDirPath() {
		absKey, err := key.ToAbs(fls)
		if err != nil {
			return err
		}

		if args.fls == nil {
			effect := &CreateDir{path: absKey, fmode: core.FileMode(DEFAULT_DIR_FMODE)}

			if err := effect.CheckPermissions(ctx); err != nil {
				return err
			}

			if tx := ctx.GetTx(); tx != nil {
				if err := tx.AddEffect(ctx, effect); err != nil {
					return err
				}
			} else if err := effect.Apply(ctx); err != nil {
				return err
			}
		} else {
			err := fls.MkdirAll(string(absKey), DEFAULT_DIR_FMODE)
			if err != nil {
				return err
			}
		}
	}

	switch v := content.(type) {
	case *core.List:
		if !key.IsDirPath() {
			return fmt.Errorf("value for file keys (key %s) should not be a dictionary, dir keys must end with '/'", key)
		}

		length := v.Len()
		for i := 0; i < length; i++ {
			pth := v.At(ctx, i).(core.Path)
			s := fls.Join(string(key), string(pth))
			if pth.IsDirPath() {
				s += "/"
			}

			if err := makeFileHierarchy(ctx, makeFileHieararchyParams{
				fls:     args.fls,
				key:     core.Path(s),
				content: nil,
				depth:   args.depth + 1,
			}); err != nil {
				return err
			}
		}
	case *core.Dictionary:
		if v == nil {
			return nil
		}
		if !key.IsDirPath() {
			return fmt.Errorf("value for file keys (key %s) should not be a dictionary, dir keys must end with '/'", key)
		}
		err := v.ForEachEntry(ctx, func(keyRepr string, k, v core.Serializable) error {
			pth := fls.Join(string(key), keyRepr)
			if k.(core.Path).IsDirPath() {
				pth += "/"
			}
			if err := makeFileHierarchy(ctx, makeFileHieararchyParams{
				fls:     args.fls,
				key:     core.Path(pth),
				content: v,
				depth:   args.depth + 1,
			}); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	case nil: //file with no specified content
		if args.fls == nil {
			return Mkfile(ctx, key)
		} else {
			absKey, err := key.ToAbs(fls)
			if err != nil {
				return err
			}

			f, err := args.fls.Create(absKey.UnderlyingString())
			if err != nil {
				return err
			}
			f.Close()
		}
	case core.Readable:
		if args.fls == nil {
			return Mkfile(ctx, key, v)
		} else {
			absKey, err := key.ToAbs(fls)
			if err != nil {
				return err
			}

			f, err := args.fls.Create(absKey.UnderlyingString())
			if err != nil {
				return err
			}
			bytes, err := v.Reader().ReadAll()
			if err != nil {
				return fmt.Errorf("failed to read content of file")
			}
			_, err = f.Write(bytes.Bytes)
			if err != nil {
				return fmt.Errorf("failed to write content of file %q", absKey)
			}
			f.Close()
		}
	default:
		return fmt.Errorf("invalid value of type %T for key %s", v, key)
	}

	return nil
}

// Mkfile creates a regular file, if an additional argument is passed it will be used as the content of the file.
func Mkfile(ctx *core.Context, fpath core.Path, args ...core.Value) error {
	var b []byte

	fileMode := DEFAULT_FILE_FMODE

	for _, arg := range args {
		switch a := arg.(type) {
		case core.Readable:
			if b != nil {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("content")
			}
			var err error
			b, err = a.Reader().ReadAllBytes()
			if err != nil {
				return err
			}
		case core.Option:
			switch a.Name {
			case "readonly":
				if a.Value == nil || a.Value != core.True {
					return core.FmtErrInvalidArgument(a)
				}
				fileMode = DEFAULT_R_FILE_FMODE
			default:
				return core.FmtErrInvalidArgument(a)
			}
		default:
			return core.FmtErrInvalidArgument(a)
		}

	}

	if fpath == "" {
		return commonfmt.FmtMissingArgument("path")
	}

	fls := ctx.GetFileSystem()

	absFpath, err := fpath.ToAbs(fls)
	if err != nil {
		return err
	}

	effect := &CreateFile{
		path:    absFpath,
		content: []byte(b),
		fmode:   core.FileMode(fileMode),
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

// Copy copy a single file or copy a list of files in destination directory.
// arguments: ./src ./copy -> copy the ./src file into ./copy that SHOULD NOT exist.
// arguments [./file, ./dir] ./dest_dir -> copy the provided files into ./dest_dir.
// Copy never overwrites a file or directory and returns an error if there is already a file at any destination path.
func Copy(ctx *core.Context, args ...core.Value) error {
	var dest core.Path
	var src core.Path
	var srcPaths []core.Path
	listProvided := false
	fls := ctx.GetFileSystem()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case *core.List:
			listProvided = true

			length := v.Len()
			for i := 0; i < length; i++ {
				pth, ok := v.At(ctx, i).(core.Path)
				if !ok {
					return errors.New("the list of file paths should only contain paths")
				}
				srcPaths = append(srcPaths, pth)
			}
		case core.Path:
			if src == "" && !listProvided {
				src = v
			} else if dest == "" {
				dest = v
			} else {
				return errors.New("at least three paths have been provided, only two paths or a list of paths followed by a destination dir are expected")
			}
		default:
			return errors.New("invalid argument " + fmt.Sprintf("%#v", v))
		}
	}

	if listProvided && src != "" {
		return errors.New("a list AND a source path shouldn't be provided at the same time")
	}

	if dest == "" {
		return errors.New("missing destination path")
	}

	{
		var err error
		dest, err = dest.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return err
		}

		for i, src := range srcPaths {
			srcPaths[i], err = src.ToAbs(ctx.GetFileSystem())
			if err != nil {
				return err
			}
		}
	}

	//dest is the name of the copy
	if src != "" {
		var err error
		src, err = src.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return err
		}

		if src.IsDirPath() != dest.IsDirPath() {
			return errors.New("source and destination should be two file paths or two directory paths")
		}
	} else if !dest.IsDirPath() {
		return errors.New("destination should be a directory path")
	}

	srcToDest := map[string]string{}
	srcFolderToDest := map[string]string{}
	srcToFileInfo := map[string]fs.FileInfo{}

	var getFiles func(src string, destDir string, newBasename string) error
	getFiles = func(src string, destDir string, newBasename string) error {
		basename := filepath.Base(src)
		if newBasename != "" {
			basename = newBasename
		}
		stat, err := fls.Lstat(src)
		srcToFileInfo[src] = stat

		if err != nil {
			return err
		}

		if stat.IsDir() {
			srcFolderToDest[string(src)] = fls.Join(string(destDir), basename)

			entries, err := ReadDir(ctx, core.Path(src))
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := getFiles(fls.Join(src, entry.Name()), fls.Join(destDir, basename), ""); err != nil {
					return err
				}
			}
		} else if stat.Mode().IsRegular() {
			srcToDest[string(src)] = fls.Join(string(destDir), basename)
		} else {
			return errors.New("only dirs and regular files are supported for now")
		}
		return nil
	}

	if !listProvided {
		cleanDest := filepath.Clean(string(dest))
		if err := getFiles(string(src), filepath.Dir(cleanDest), filepath.Base(cleanDest)); err != nil {
			return err
		}
	} else {
		for _, srcPath := range srcPaths {
			if err := getFiles(string(srcPath), string(dest), ""); err != nil {
				return err
			}
		}
	}
	var errs []string

	//we check that we will not overwrite a file or dir before making changes to the filesystem.

	for srcFile, destFile := range srcToDest {
		_, err := fls.Lstat(destFile)
		if err == nil {
			errs = append(errs, fmt.Sprintf("cannot copy file %s -> %s: destination already exists", srcFile, destFile))
		}
	}

	for srcFolder, destFolder := range srcFolderToDest {
		_, err := fls.Lstat(destFolder)
		if err == nil {
			errs = append(errs, fmt.Sprintf("cannot copy dir %s -> %s: destination already exists", srcFolder, destFolder))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	var wg sync.WaitGroup
	wg.Add(len(srcToDest))
	var errorLock sync.Mutex

	//we first create the folder structure
	for srcFolder, destFolder := range srcFolderToDest {
		perm := srcToFileInfo[srcFolder].Mode().Perm()

		if err := fls.MkdirAll(destFolder, perm); err != nil {
			errs = append(errs, err.Error())
		}
	}

	//we copy the files
	for srcFile, destFile := range srcToDest {
		//TODO: do not read too many big files

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		go func(ctx *core.Context, srcFile, destFile string) {
			defer wg.Done()

			b, err := ReadEntireFile(ctx, core.Path(srcFile))

			if err == nil {
				err = __createFile(ctx, core.Path(destFile), b, srcToFileInfo[srcFile].Mode().Perm())
			}

			if err != nil {
				errorLock.Lock()
				defer errorLock.Unlock()
				errs = append(errs, err.Error())
			}
		}(ctx.BoundChild(), srcFile, destFile)
	}

	wg.Wait()
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func __createFile(ctx *core.Context, fpath core.Path, b []byte, fmode fs.FileMode) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	alreadyClosed := false
	fls := ctx.GetFileSystem()

	fpath, err := fpath.ToAbs(ctx.GetFileSystem())
	if err != nil {
		return err
	}

	perm := core.FilesystemPermission{Kind_: permkind.Create, Entity: fpath}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	ctx.Take(FS_TOTAL_NEW_FILE_LIMIT_NAME, 1)
	ctx.Take(FS_NEW_FILE_RATE_LIMIT_NAME, 1)

	rate, err := ctx.GetByteRate(FS_WRITE_LIMIT_NAME)
	if err != nil {
		return err
	}

	chunkSize := computeChunkSize(rate, len(b))
	f, err := fls.OpenFile(string(fpath), os.O_CREATE|os.O_WRONLY, fmode)
	if err != nil {
		return err
	}

	defer func() {
		if !alreadyClosed {
			f.Close()
		}
	}()

	for len(b) != 0 {
		select {
		case <-ctx.Done():
			f.Close()
			alreadyClosed = true
			return ctx.Err()
		default:
		}
		ctx.Take(FS_WRITE_LIMIT_NAME, int64(chunkSize))

		_, err = f.Write(b[0:chunkSize])

		if err != nil {
			return err
		}
		b = b[chunkSize:]
		chunkSize = min(len(b), chunkSize)
	}

	return nil
}
