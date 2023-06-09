package fs_ns

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_FILE_HIERARCHY_DEPTH = 5
)

var (
	TooDeepFileHierarchy = errors.New("file hierarchy is too deep")
)

// makeFileHierarchy recursively creates folders and files.
// Key represents the filename (or dirname) and content described how to make the file.
// The content of a directory  is described by a List or a Dictionary.
// The content of a regular file is described by a string (Str) or bytes (IBytes).
func makeFileHierarchy(ctx *core.Context, key core.Path, content core.Value, depth int) error {
	if depth > MAX_FILE_HIERARCHY_DEPTH {
		return TooDeepFileHierarchy
	}

	if key.IsDirPath() {
		absKey, err := key.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return err
		}

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
	}

	fls := ctx.GetFileSystem()

	switch v := content.(type) {
	case *core.List:
		length := v.Len()
		for i := 0; i < length; i++ {
			pth := v.At(ctx, i).(core.Path)
			s := fls.Join(string(key), string(pth))
			if pth.IsDirPath() {
				s += "/"
			}

			if err := makeFileHierarchy(ctx, core.Path(s), nil, depth+1); err != nil {
				return err
			}
		}
	case *core.Dictionary:
		if v == nil {
			return nil
		}
		if !key.IsDirPath() {
			return fmt.Errorf("value for file keys (key %s) should not be a dictionary", key)
		}
		for keyRepr, val := range v.Entries {
			k := v.Keys[keyRepr].(core.Path)
			pth := fls.Join(string(key), keyRepr)
			if k.IsDirPath() {
				pth += "/"
			}
			if err := makeFileHierarchy(ctx, core.Path(pth), val, depth+1); err != nil {
				return err
			}
		}
	case nil: //file with not specified content
		return Mkfile(ctx, key)
	case core.Readable:
		return Mkfile(ctx, key, v)
	default:
		return fmt.Errorf("invalid value of type %T for key %s", v, key)
	}

	return nil
}

// Mkdir expects a core.Path argument and creates a directory.
// If an additional argument of type Dictionnary is passed a file hiearchy will also be created.
func Mkdir(ctx *core.Context, args ...core.Value) error {

	var dirpath core.Path
	var contentDesc *core.Dictionary

	for _, arg := range args {

		switch v := arg.(type) {
		case core.Path:
			if dirpath != "" {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			dirpath = v
			if !dirpath.IsDirPath() {
				return fmt.Errorf("path argument is a file path : %s, the path should end with '/'", string(dirpath))
			}
		case *core.Dictionary:
			if contentDesc != nil {
				return commonfmt.FmtErrArgumentProvidedAtLeastTwice("content")
			}
			contentDesc = v

			visit := func(val core.Value) (parse.TraversalAction, error) {
				switch val.(type) {
				case *core.List, core.Path, *core.Dictionary, core.StringLike, core.BytesLike:
				default:
					return parse.Continue, fmt.Errorf("invalid content description: it contains a value of type %T", val)
				}
				return parse.Continue, nil
			}

			for _, e := range contentDesc.Entries {
				if err := core.Traverse(e, visit, core.TraversalConfiguration{MaxDepth: MAX_FILE_HIERARCHY_DEPTH}); err != nil {
					return err
				}
			}
			//TODO: check that the hiearchy is not too deep
		default:
			return core.FmtErrInvalidArgument(v)
		}
	}

	if dirpath == "" {
		return commonfmt.FmtMissingArgument("path")
	}

	return makeFileHierarchy(ctx, dirpath, contentDesc, 0)
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

	absFpath, err := fpath.ToAbs(ctx.GetFileSystem())
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

// ReadFile expects a core.Path argument, it reads the whole content of a file.
func ReadFile(ctx *core.Context, args ...core.Value) (*core.ByteSlice, error) {
	var fpath core.Path

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Path:
			if fpath != "" {
				return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			fpath = v
		default:
			return &core.ByteSlice{}, errors.New("invalid argument " + fmt.Sprintf("%#v", v))
		}
	}

	if fpath == "" {
		return &core.ByteSlice{}, commonfmt.FmtMissingArgument("path")
	}

	b, err := ReadEntireFile(ctx, fpath)
	return &core.ByteSlice{Bytes: b, IsDataMutable: true}, err
}

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

func Find(ctx *core.Context, dir core.Path, filters ...core.Pattern) (*core.List, error) {
	if !dir.IsDirPath() {
		return nil, errors.New("find: first argument should be a directory path")
	}

	fls := ctx.GetFileSystem()

	//we check patterns & convert globbing patterns to absolute globbing path patterns.
	for i, filter := range filters {
		switch filt := filter.(type) {
		case core.StringPattern:
		case core.PathPattern:
			if !filt.IsGlobbingPattern() {
				return nil, errors.New("find: path filters should be globbing path patterns")
			}
			if !filt.IsAbsolute() {
				filt = core.PathPattern(fls.Join(string(dir), string(filt)))
				filters[i] = filt.ToAbs(fls)
			}
		default:
			return nil, fmt.Errorf("invalid pattern for filtering files: %s", core.Stringify(filt, ctx))
		}
	}

	var found []core.Serializable
	var paths []string

	//we first get matching paths
	for _, filter := range filters {
		switch filt := filter.(type) {
		case core.PathPattern:
			matches, err := glob(fls, string(filt))
			if err != nil {
				return nil, err
			}
			paths = append(paths, matches...)
		}
	}

	//we get the information for each matched file
	for _, pth := range paths {
		info, err := fls.Lstat(pth)
		if err != nil {
			return nil, err
		}
		found = append(found, makeFileInfo(info, pth, fls))
	}

	return core.NewWrappedValueList(found...), nil
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
		Kind_:  permkind.Update,
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
		chunkSize = utils.Min(len(b), chunkSize)
	}

	return nil
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
		ModTime_:  core.Date(info.ModTime()),
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
		_res, lsErr := ListFiles(ctx, path)
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

		t, ok := core.FILE_EXTENSION_TO_MIMETYPE[filepath.Ext(string(path))]
		if ok {
			contentType = t
		}
		val, _, err := core.ParseOrValidateResourceContent(ctx, b, contentType, doParse, validateRaw)
		return val, err
	}
}

func ListFiles(ctx *core.Context, args ...core.Value) ([]core.FileInfo, error) {
	var pth core.Path
	var patt core.PathPattern
	ERR := "a single path (or path pattern) argument is expected"
	fls := ctx.GetFileSystem()

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Path:
			if pth != "" {
				return nil, errors.New(ERR)
			}
			pth = v
		case core.PathPattern:
			if patt != "" {
				return nil, errors.New(ERR)
			}
			patt = v
		default:
			return nil, errors.New("invalid argument " + fmt.Sprintf("%#v", v))
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

	if pth != "" && patt != "" {
		return nil, errors.New(ERR)
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

func OpenExisting(ctx *core.Context, args ...core.Value) (*File, error) {
	var pth core.Path
	var write bool

	for _, arg := range args {

		switch a := arg.(type) {
		case core.Path:
			if pth != "" {
				return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			pth = a
		case core.Option:
			switch a.Name {
			case "w":
				if boolean, ok := a.Value.(core.Bool); ok {
					write = bool(boolean)
				} else {
					return nil, errors.New("-w should have a boolean value")
				}
			}
		default:
			return nil, fmt.Errorf("invalid argument %v", a)
		}
	}

	return openExistingFile(ctx, pth, write)
}

func Glob(ctx *core.Context, patt core.PathPattern) []core.Path {

	if !patt.IsGlobbingPattern() {
		panic(errors.New("cannot call glob function on non-globbing pattern"))
	}

	fls := ctx.GetFileSystem()
	absPtt := patt.ToAbs(fls)

	res, err := glob(fls, string(absPtt))
	if err != nil {
		panic(err)
	}

	list := make([]core.Path, len(res))
	for i, e := range res {
		stat, err := fls.Stat(e)
		if err != nil {
			panic(err)
		}

		if e[0] != '/' {
			e = "./" + e
		}

		if stat.IsDir() {
			e += "/"
		}
		list[i] = core.Path(e)
	}
	return list
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

func GetTreeData(ctx *core.Context, path core.Path) *core.UData {
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
	chunkSize = utils.Min(fileSize, chunkSize)

	return chunkSize
}
