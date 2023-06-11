package fs_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/oklog/ulid/v2"
)

const (
	METAFS_UNDERLYING_FILE_PROPNAME = "underlying-file"
	METAFS_FILE_MODE_PROPNAME       = "file-mode"
	METAFS_CREATION_TIME_PROPNAME   = "creation-time"
	METAFS_MODIF_TIME_PROPNAME      = "modification-time"
	METAFS_SYMLINK_TARGET_PROPNAME  = "symlink-target"
	METAFS_CHILDREN_PROPNAME        = "children"

	METAFS_UNDERLYING_UNDERLYING_FILE_PERM = 0600
)

var (
	REQUIRED_METAFS_FILE_METADATA_PROPNAMES = []string{METAFS_FILE_MODE_PROPNAME, METAFS_CREATION_TIME_PROPNAME, METAFS_MODIF_TIME_PROPNAME}
)

// MetaFilesystem is a filesystem that works on top of another filesystem, it stores its metadata in a file and file contents
// in regular files.
type MetaFilesystem struct {
	underlying afs.Filesystem
	dir        string
	metadata   *filekv.SingleFileKV //all the metadata about file is stores in this Key value store.
	ctx        *core.Context
}

func NewPortableOfsBackedFilesystem(ctx *core.Context, maxTotalStorageSize core.ByteCount) *MetaFilesystem {
	return &MetaFilesystem{}
}

func (fls *MetaFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fls *MetaFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}

func (fls *MetaFilesystem) Absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return "", core.ErrNotImplemented
}

func (fls *MetaFilesystem) getFileMetadata(pth core.Path) (*metaFsFileMetadata, bool, error) {
	if pth.IsAbsolute() {
		return nil, false, errors.New("file's path should be absolute")
	}
	key := "/files" + pth

	//remove trailing slash
	if pth[len(pth)-1] == '/' {
		pth = pth[:len(pth)-1]
	}

	info, ok, err := fls.metadata.Get(fls.ctx, key, fls)
	if err != nil {
		return nil, false, fmtFailedToGetFileMetadataError(pth, err)
	}

	if !ok {
		return nil, false, nil
	}

	record, ok := info.(*core.Record)
	if !ok {
		return nil, false, fmt.Errorf("invalid type for metadata of file %s: %T", pth, info)
	}

	for _, propName := range REQUIRED_METAFS_FILE_METADATA_PROPNAMES {
		if !record.HasProp(fls.ctx, propName) {
			return nil, false,
				fmt.Errorf("invalid record for metadata of file %s, missing .%s property: %s", pth, propName, core.Stringify(record, fls.ctx))
		}
	}

	fileMode := record.Prop(fls.ctx, METAFS_FILE_MODE_PROPNAME).(core.FileMode)
	creationTime := record.Prop(fls.ctx, METAFS_CREATION_TIME_PROPNAME).(core.Date)
	modifTime := record.Prop(fls.ctx, METAFS_MODIF_TIME_PROPNAME).(core.Date)

	var symlinkTarget *core.Path
	if record.HasProp(fls.ctx, METAFS_SYMLINK_TARGET_PROPNAME) {
		symlinkTarget = new(core.Path)
		*symlinkTarget = record.Prop(fls.ctx, METAFS_SYMLINK_TARGET_PROPNAME).(core.Path)
	}

	var underlyingFilePath *core.Path
	if record.HasProp(fls.ctx, METAFS_UNDERLYING_FILE_PROPNAME) {
		underylingFile := record.Prop(fls.ctx, METAFS_UNDERLYING_FILE_PROPNAME).(core.Str)

		underlyingFilePath = new(core.Path)
		*underlyingFilePath = core.PathFrom(fls.underlying.Join(fls.dir, string(underylingFile)))
	}

	metadata := &metaFsFileMetadata{
		path:             pth,
		concreteFile:     underlyingFilePath,
		mode:             fs.FileMode(fileMode),
		creationTime:     creationTime,
		modificationTime: modifTime,

		symlinkTarget: symlinkTarget,
	}

	return metadata, true, nil
}

func (fls *MetaFilesystem) setFileMetadata(metadata *metaFsFileMetadata) error {
	recordPropertyNames := []string{
		METAFS_UNDERLYING_FILE_PROPNAME,
		METAFS_FILE_MODE_PROPNAME,
		METAFS_CREATION_TIME_PROPNAME,
		METAFS_MODIF_TIME_PROPNAME,
	}

	recordPropertyValues := []core.Value{
		core.Str(metadata.concreteFile.Basename()),
		core.FileMode(metadata.mode),
		metadata.creationTime,
		metadata.modificationTime,
	}

	if metadata.mode.IsDir() {
		var children []core.Value

		for _, path := range metadata.children {
			children = append(children, path)
		}

		recordPropertyNames = append(recordPropertyNames, METAFS_CHILDREN_PROPNAME)
		recordPropertyValues = append(recordPropertyValues, core.NewTuple(children))
	}

	metadataRecord := core.NewRecordFromKeyValLists(recordPropertyNames, recordPropertyValues)

	key := metadata.path
	//remove trailing slash
	if key[len(key)-1] == '/' {
		key = key[:len(key)-1]
	}

	fls.metadata.Set(fls.ctx, key, metadataRecord, fls)
	return nil
}

func (fls *MetaFilesystem) Create(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (fls *MetaFilesystem) Open(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDONLY, 0)
}

func (fls *MetaFilesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	pth := core.PathFrom(filename)

	metadata, exists, err := fls.getFileMetadata(pth)
	if err != nil {
		return nil, err
	}

	if isSymlink(metadata.mode) {
		//
		return nil, errors.New("symlinks not supported")
	}

	if !exists {
		if !isCreate(flag) {
			return nil, os.ErrNotExist
		}

		underlyingFilePath := core.Path(fls.underlying.Join(fls.dir, ulid.Make().String()))
		creationTime := core.Date(time.Now())

		mode := fs.FileMode(fs.ModeDir)
		mode |= fs.FileMode(perm)

		newFileMetadata := &metaFsFileMetadata{
			path:             pth,
			concreteFile:     &underlyingFilePath,
			mode:             mode,
			creationTime:     creationTime,
			modificationTime: creationTime,
		}

		if err := fls.setFileMetadata(newFileMetadata); err != nil {
			return nil, err
		}
		metadata = newFileMetadata
	} else {
		if isExclusive(flag) {
			return nil, os.ErrExist
		}
	}

	underlyingFile, err := fls.underlying.OpenFile(metadata.concreteFile.UnderlyingString(), flag, METAFS_UNDERLYING_UNDERLYING_FILE_PERM)

	if err != nil {
		//TODO: give more info about the error without leaking information about the underlying filesystem.
		return nil, fmt.Errorf("failed to open %s", pth)
	}

	if metadata.mode.IsDir() {
		return nil, fmt.Errorf("cannot open directory: %s", filename)
	}

	file := &metaFsFile{
		path:       pth,
		metadata:   metadata,
		underlying: underlyingFile,
	}

	return file, nil
}

func (fs *MetaFilesystem) Stat(filename string) (os.FileInfo, error) {
	metadata, exists, err := fs.getFileMetadata(core.PathFrom(filename))

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	return core.FileInfo{
		BaseName_:       string(metadata.path.Basename()),
		AbsPath_:        metadata.path,
		Mode_:           core.FileMode(metadata.mode),
		CreationTime_:   metadata.creationTime,
		ModTime_:        metadata.modificationTime,
		HasCreationTime: true,
	}, nil
}

func (fs *MetaFilesystem) Lstat(filename string) (os.FileInfo, error) {
	metadata, exists, err := fs.getFileMetadata(core.PathFrom(filename))

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	if isSymlink(metadata.mode) {
		return nil, errors.New("symlinks not supported")
	}

	return fs.Stat(filename)
}

func (fs *MetaFilesystem) ReadDir(path string) ([]os.FileInfo, error) {
	metadata, exists, err := fs.getFileMetadata(core.PathFrom(path))

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	if !metadata.mode.IsDir() {
		return nil, errors.New("not a dir")
	}

	var entries []os.FileInfo
	for _, child := range metadata.children {
		stat, err := fs.Stat(child.UnderlyingString())
		if err != nil {
			return nil, err
		}
		entries = append(entries, stat)
	}

	sort.Sort(ByName(entries))

	return entries, nil
}

func (fls *MetaFilesystem) MkdirAll(path string, perm os.FileMode) error {
	_, exists, err := fls.getFileMetadata(core.PathFrom(path))

	if err != nil {
		return err
	}

	pth := core.DirPathFrom(path)

	if !exists { //create the directory
		creationTime := core.Date(time.Now())

		newFileMetadata := &metaFsFileMetadata{
			path:             pth,
			mode:             perm,
			creationTime:     creationTime,
			modificationTime: creationTime,
		}

		if err := fls.setFileMetadata(newFileMetadata); err != nil {
			return err
		}
	}

	//TODO: support creating intermediary directories

	return nil
}

func (fls *MetaFilesystem) TempFile(dir, prefix string) (billy.File, error) {
	return nil, core.ErrNotImplementedYet
}

func (fls *MetaFilesystem) Rename(from, to string) error {
	_, exists, err := fls.getFileMetadata(core.PathFrom(from))

	if err != nil {
		return err
	}

	if !exists {
		return os.ErrNotExist
	}

	panic(core.ErrNotImplementedYet)
}

func (fls *MetaFilesystem) Remove(filename string) error {
	panic(core.ErrNotImplementedYet)
}

func (fls *MetaFilesystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fls *MetaFilesystem) Symlink(target, link string) error {
	return core.ErrNotImplementedYet
}

func (fls *MetaFilesystem) Readlink(link string) (string, error) {
	return "", core.ErrNotImplementedYet
}

type metaFsFileMetadata struct {
	path             core.Path
	concreteFile     *core.Path //nil if dir
	mode             fs.FileMode
	creationTime     core.Date
	modificationTime core.Date

	//the targets of symlinks are directly stored in the metadata,
	//there is no underlying file.
	symlinkTarget *core.Path

	//children files if directory
	children []core.Path
}

func fmtFailedToGetFileMetadataError(pth core.Path, err error) error {
	return fmt.Errorf("failed to get metadata for file %s: %w", pth, err)
}
