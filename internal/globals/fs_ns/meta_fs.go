package fs_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/buntdb"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/pathutils"
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
	METAFS_AUTO_CREATED_DIR_PERM           = fs.FileMode(0700)

	METAFS_FILES_KEY   = "/files"
	METAFS_KV_FILENAME = "metadata.kv"

	METAFS_MIN_USABLE_SPACE                             = 10_000_000
	METAFS_USED_SPACE_CHECK_INTERVAL                    = time.Second / 2
	METAFS_ALWAYS_CHECK_USED_SPACE_BYTE_COUNT_THRESHOLD = 100_000
	METAFS_DEFAULT_MAX_FILE_COUNT                       = 1000
	METAFS_DEFAULT_MAX_PARALLEL_FILE_CREATION_COUNT     = 10

	METAFS_MAX_SNAPSHOTABLE_SIZE                 = core.ByteCount(100_000_000)
	METAFS_DEFAULT_MAX_UNTRACK_CLOSED_FILE_COUNT = 10
)

var (
	REQUIRED_METAFS_FILE_METADATA_PROPNAMES = []string{METAFS_FILE_MODE_PROPNAME, METAFS_CREATION_TIME_PROPNAME, METAFS_MODIF_TIME_PROPNAME}

	_ = core.SnapshotableFilesystem((*MetaFilesystem)(nil))
)

// MetaFilesystem is a filesystem that works on top of another filesystem, it stores its metadata in a file and file contents
// in regular files.
type MetaFilesystem struct {
	maxUsableSpace           core.ByteCount //maximum space usable in the underyling filesystem
	maxFileCount             int32          //maximum number of files stored by MetaFilesystem in the underyling filesystem
	maxParallelCreationCount int32

	//underlying afs.Filesystem
	underlying billy.Basic
	dir        *string //optional, if set underlying is an afs.Filesytem
	openFiles  map[ /*normalized path*/ string]map[*metaFsFile]struct{}

	// last modification times of non-dir files
	lastModificationTimes     map[ /*normalized path*/ string]core.DateTime
	lastModificationTimesLock sync.RWMutex

	eventQueue     *memds.TSArrayQueue[Event] //periodically emptied
	fsWatchers     []*VirtualFilesystemWatcher
	fsWatchersLock sync.Mutex

	//all the metadata about files is stored in this Key value store.
	metadata *buntdb.DB
	ctx      *core.Context

	lock            sync.RWMutex
	closedOrClosing atomic.Bool
	closed          atomic.Bool
	snapshoting     atomic.Bool

	pendingFileCreations atomic.Int32

	usedSpaceCache     core.ByteCount
	usedSpaceCacheLock sync.RWMutex
	lastSpaceCheckTime atomic.Int64 //unix milli (the millisecond precision is required)

}

type MetaFilesystemParams struct {
	//used if underlying is a filesystem
	Dir string

	//maximum space usable in the underlying filesystem, ignored if dir is false.
	//The value should be greater or equal to METAFS_MIN_USABLE_SPACE, it defaults to METAFS_MIN_USABLE_SPACE.
	MaxUsableSpace core.ByteCount

	//The value defaults to METAFS_DEFAULT_MAX_FILE_COUNT, ignored if dir is false.
	MaxFileCount int32

	//The value defaults to METAFS_DEFAULT_MAX_PARALLEL_FILE_CREATION_COUNT, ignored if dir is false.
	MaxParallelCreationCount int16
}

func OpenMetaFilesystem(ctx *core.Context, underlying billy.Basic, opts MetaFilesystemParams) (*MetaFilesystem, error) {
	if opts.MaxUsableSpace > 0 && opts.MaxUsableSpace < METAFS_MIN_USABLE_SPACE {
		return nil, ErrMaxUsableSpaceTooSmall
	}

	maxUsableSpace := max(opts.MaxUsableSpace, METAFS_MIN_USABLE_SPACE)

	maxFileCount := opts.MaxFileCount
	if maxFileCount <= 0 {
		maxFileCount = METAFS_DEFAULT_MAX_FILE_COUNT
	}

	maxParallelCreationCount := opts.MaxParallelCreationCount
	if maxParallelCreationCount <= 0 {
		maxParallelCreationCount = METAFS_DEFAULT_MAX_PARALLEL_FILE_CREATION_COUNT
	}

	var buntDBPath string

	if opts.Dir != "" {
		fls, ok := underlying.(afs.Filesystem)
		if !ok {
			return nil,
				fmt.Errorf("impossible to create directory for meta filesystem since the underlying storage is not a full-fledge filesystem")
		}

		if err := fls.MkdirAll(opts.Dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create directory for meta filesystem: %w", err)
		}
		buntDBPath = underlying.Join(opts.Dir, METAFS_KV_FILENAME)
	} else {
		buntDBPath = "/" + METAFS_KV_FILENAME
	}

	kv, err := buntdb.OpenBuntDBNoPermCheck(buntDBPath, underlying)

	if err != nil {
		return nil, fmt.Errorf("failed to open/create single-file KV store for storing metadata of meta filesystem: %w", err)
	}

	fls := &MetaFilesystem{
		ctx:                   ctx,
		underlying:            underlying,
		openFiles:             map[string]map[*metaFsFile]struct{}{},
		lastModificationTimes: map[string]core.DateTime{},
		eventQueue: memds.NewTSArrayQueueWithConfig(memds.TSArrayQueueConfig[Event]{
			AutoRemoveCondition: isOldEvent,
		}),

		metadata:                 kv,
		maxUsableSpace:           maxUsableSpace,
		maxFileCount:             maxFileCount,
		maxParallelCreationCount: int32(maxParallelCreationCount),
	}

	dir := opts.Dir
	if dir != "" {
		fls.dir = &dir
	}

	//create metadata for root directory '/' if not present

	rootPath := core.DirPathFrom("/")
	_, exists, err := fls.getFileMetadata(rootPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata from KV: %w", err)
	}

	if !exists {
		now := core.DateTime(time.Now())
		metadata := &metaFsFileMetadata{
			path:             rootPath,
			mode:             0o700 | fs.ModeDir,
			creationTime:     now,
			modificationTime: now,
		}

		if err := fls.setFileMetadata(metadata, nil); err != nil {
			return nil, err
		}
	}

	// make sure the used space is not greater than allowed
	used, err := fls.computeUsedSpace(false)

	if err == nil && used > fls.maxUsableSpace {
		return nil, ErrNoRemainingSpaceUsableByFS
	} else if err != nil {
		return nil, fmt.Errorf("failed to check used space: %w", err)
	}

	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return fls.Close(ctx)
	})

	// update modification time of files
	err = fls.Walk(func(normalizedPath string, path core.Path, metadata *metaFsFileMetadata) error {
		if metadata.mode.IsDir() {
			return nil
		}

		info, err := fls.underlying.Stat(metadata.concreteFile.UnderlyingString())
		if err != nil {
			return err
		}

		if time.Time(metadata.modificationTime).Before(info.ModTime()) {
			metadata.modificationTime = core.DateTime(info.ModTime())
			return fls.setFileMetadata(metadata, nil)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update modification times during opening of meta filesystem: %w", err)
	}

	return fls, nil
}

func (fls *MetaFilesystem) IsClosedOrClosing() bool {
	return fls.closedOrClosing.Load()
}

func (fls *MetaFilesystem) IsClosed() bool {
	return fls.closed.Load()
}

func (fls *MetaFilesystem) Close(ctx *core.Context) error {
	if !fls.closedOrClosing.CompareAndSwap(false, true) {
		return nil
	}

	defer fls.closed.Store(true)

	//unregister the filesystem from the watched filesystems.
	watchedVirtualFilesystemsLock.Lock()
	delete(watchedVirtualFilesystems, fls)
	watchedVirtualFilesystemsLock.Unlock()

	//stop and remove all watchers
	func() {
		defer utils.Recover()

		fls.fsWatchersLock.Lock()
		for _, watcher := range fls.fsWatchers {
			watcher.Close()
		}
		fls.fsWatchers = nil
		fls.fsWatchersLock.Unlock()
	}()

	//remove all events
	fls.eventQueue.Clear()

	fls.lock.Lock()
	openFiles := fls.openFiles
	fls.openFiles = nil
	fls.lock.Unlock()

	//close all files
	for _, files := range openFiles {
		for sameFile := range files {
			func() {
				defer utils.Recover()
				sameFile.Close()
			}()
		}
	}

	//close the key-value store
	return fls.metadata.Close()
}

func (fls *MetaFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fls *MetaFilesystem) Root() string {
	return "/"
}

// DoWithContext implements core.IDoWithContext.
func (fls *MetaFilesystem) DoWithContext(ctx *core.Context, fn func() error) error {
	if fls.closedOrClosing.Load() {
		return ErrClosedFilesystem
	}
	return fn()
}

func (fls *MetaFilesystem) Absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return "", core.ErrNotImplemented
}

func (fls *MetaFilesystem) Walk(visit func(normalizedPath string, path core.Path, metadata *metaFsFileMetadata) error) error {
	return fls.walk("/", visit)
}

func (fls *MetaFilesystem) walk(path core.Path, visit func(normalizedPath string, path core.Path, metadata *metaFsFileMetadata) error) error {
	meta, _, err := fls.getFileMetadata(path, nil)
	if err != nil {
		return err
	}

	if meta.mode.IsDir() && !path.IsDirPath() {
		path += "/"
	}

	err = visit(NormalizeAsAbsolute(string(path)), path, meta)
	if err != nil {
		return err
	}

	if len(meta.children) > 0 {
		childrenNames := slices.Clone(meta.children)
		slices.Sort(childrenNames)

		for _, childName := range childrenNames {
			childPath := path.JoinEntry(string(childName))
			if err := fls.walk(childPath, visit); err != nil {
				return fmt.Errorf("%q: %w", childPath, err)
			}
		}
	}

	return nil
}

// untrackSomeClosedFiles untracks up to maxRemovalCount closed files, if maxRemovalCount is <= 0
// up to METAFS_DEFAULT_MAX_UNTRACK_CLOSED_FILE_COUNT are untracked.
func (fls *MetaFilesystem) untrackSomeClosedFiles(maxRemovalCount int) {
	//in order for this function to execute as fast as possible we only remove a few tracked files.

	if maxRemovalCount <= 0 {
		maxRemovalCount = METAFS_DEFAULT_MAX_UNTRACK_CLOSED_FILE_COUNT
	}
	removedCount := 0

	for _, files := range fls.openFiles {
		for sameFile := range files {
			if sameFile.closed.Load() {
				delete(files, sameFile)
				removedCount++
				if removedCount >= maxRemovalCount {
					return
				}
			}
		}
	}
}

func (fls *MetaFilesystem) getUnderlyingFileCount() (int32, error) {
	if fls.dir == nil {
		//TODO: iterate over files and call Stat()
		// this should not necessitate a global locking
		return 0, nil
	}

	dir := *fls.dir
	underlying := fls.underlying.(afs.Filesystem)

	//we assume that the underlying directory only contain files created by the meta filesystem.
	entries, err := underlying.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("impossible to read concrete directory")
	}

	return int32(len(entries)), nil
}

func (fls *MetaFilesystem) computeUsedSpace(useCache bool, add ...core.ByteCount) (core.ByteCount, error) {
	// WIP

	lastUsedSpaceCheckTime := fls.lastSpaceCheckTime.Load()

	if !useCache && time.Since(time.UnixMilli(lastUsedSpaceCheckTime)) < METAFS_USED_SPACE_CHECK_INTERVAL {
		fls.usedSpaceCacheLock.Lock()
		defer fls.usedSpaceCacheLock.Unlock()

		for n := range add {
			if n > 0 {
				fls.usedSpaceCache += core.ByteCount(n)
			}
		}
		return core.ByteCount(fls.usedSpaceCache), nil
	}

	fls.usedSpaceCacheLock.Lock()
	defer fls.usedSpaceCacheLock.Unlock()

	// we read again lastUsedSpaceCheckTime because during the time to acquire the lock another thread
	// may have updated the value.
	{
		lastUsedSpaceCheckTime = fls.lastSpaceCheckTime.Load()
		if time.Since(time.UnixMilli(lastUsedSpaceCheckTime)) < METAFS_USED_SPACE_CHECK_INTERVAL {
			return core.ByteCount(fls.usedSpaceCache), nil
		}
	}

	fls.lastSpaceCheckTime.Store(time.Now().UnixMilli())

	if fls.dir == nil {
		//TODO: iterate over files and call Stat()
		// this should not necessitate a global locking
		return 0, nil
	}
	dir := *fls.dir
	underlying := fls.underlying.(afs.Filesystem)

	entries, err := underlying.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("impossible to read concrete directory")
	}

	usedSpace := int64(0)
	for _, e := range entries {
		usedSpace += e.Size()
	}

	fls.usedSpaceCache = core.ByteCount(usedSpace)

	for n := range add {
		if n > 0 {
			fls.usedSpaceCache += core.ByteCount(n)
		}
	}

	return fls.usedSpaceCache, nil
}

func (fls *MetaFilesystem) computeFreeSpace(useCache bool, add ...core.ByteCount) (core.ByteCount, error) {
	// WIP

	usedSpace, err := fls.computeUsedSpace(useCache, add...)

	if err != nil {
		return 0, err
	}

	if usedSpace > fls.maxUsableSpace {
		return 0, nil
	}

	return fls.maxUsableSpace - usedSpace, nil
}

func (fls *MetaFilesystem) checkAddedByteCount(size core.ByteCount) (bool, error) {
	// WIP

	freeSpace, err := fls.computeFreeSpace(size < METAFS_ALWAYS_CHECK_USED_SPACE_BYTE_COUNT_THRESHOLD, size)

	fls.usedSpaceCacheLock.Lock()
	fls.usedSpaceCache += size
	defer fls.usedSpaceCacheLock.Unlock()

	if err != nil {
		return true, err
	}

	if freeSpace < 0 {
		return false, nil
	}

	return freeSpace >= size, nil
}

func (fls *MetaFilesystem) Create(filename string) (billy.File, error) {
	defer fls.pendingFileCreations.Add(-1)

	if fls.pendingFileCreations.Add(1) > fls.maxParallelCreationCount {
		return nil, ErrTooManyParallelFileCreation
	}

	//properly taking into account files being deleted is not trivial,
	//especially since we know nothing about the underyling file system.

	count, err := fls.getUnderlyingFileCount()
	if err != nil {
		return nil, err
	}

	if count+fls.pendingFileCreations.Load() > int32(fls.maxFileCount) {
		return nil, ErrMaxFileNumberAlreadyReached
	}

	return fls.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, afs.DEFAULT_CREATE_FPERM)
}

func (fls *MetaFilesystem) Open(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDONLY, 0)
}

func (fls *MetaFilesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if fls.closedOrClosing.Load() {
		return nil, ErrClosedFilesystem
	}

	fls.lock.Lock()
	locked := true
	defer func() {
		if locked {
			fls.lock.Unlock()
		}
	}()

	originalPath := filename
	filename = NormalizeAsAbsolute(filename)

	pth := core.PathFrom(filename)
	metadata, exists, err := fls.getFileMetadata(pth, nil)
	if err != nil {
		return nil, err
	}

	created := false

	if !exists {
		if !IsCreate(flag) {
			return nil, os.ErrNotExist
		}

		//create file

		//create a read-write transaction
		tx, err := fls.metadata.Begin(true)
		if err != nil {
			return nil, err
		}
		txCommited := false
		defer func() {
			if !txCommited {
				tx.Rollback()
			}
		}()

		//return an error if the file has been created in the meantime
		_, exists, _ := fls.getFileMetadata(pth, tx)
		if exists {
			return nil, errors.New("file was created in the meantime")
		}

		dir := filepath.Dir(filename)
		if dir != "/" {
			//make sure parent exists
			err := fls.MkdirAllNoLock_(dir, METAFS_AUTO_CREATED_DIR_PERM, tx)
			if err != nil {
				return nil, fmt.Errorf("failed to create %s", dir)
			}
		}

		//get & update metadata of parent directory
		dirPath := filepath.Dir(string(pth))
		dirMetadata, found, err := fls.getFileMetadata(core.DirPathFrom(dirPath), tx)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, fmt.Errorf("failed to create %s: parent directory %s does not exist", pth, dirPath)
		}
		dirMetadata.children = append(dirMetadata.children, pth.Basename())
		dirMetadata.modificationTime = core.DateTime(time.Now())
		if err := fls.setFileMetadata(dirMetadata, tx); err != nil {
			return nil, err
		}

		//create & store metadata for new file
		var underlyingFilePath core.Path

		if fls.dir != nil {
			underlyingFilePath = core.Path(fls.underlying.Join(*fls.dir, ulid.Make().String()))
		} else {
			underlyingFilePath = core.Path(NormalizeAsAbsolute(ulid.Make().String()))
		}

		creationTime := core.DateTime(time.Now())

		mode := fs.FileMode(perm)

		newFileMetadata := &metaFsFileMetadata{
			path:             pth,
			concreteFile:     &underlyingFilePath,
			mode:             mode,
			creationTime:     creationTime,
			modificationTime: creationTime,
		}

		if err := fls.setFileMetadata(newFileMetadata, tx); err != nil {
			return nil, err
		}
		created = true
		metadata = newFileMetadata

		//commit metada changes
		txCommited = true
		err = tx.Commit()

		if err != nil {
			return nil, err
		}
	} else {
		//file exists

		if isSymlink(metadata.mode) {
			//
			return nil, errors.New("symlinks not supported")
		}

		if IsExclusive(flag) {
			return nil, os.ErrExist
		}
	}

	if metadata.mode.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrCannotOpenDir, filename)
	}

	underlyingFile, err := fls.underlying.OpenFile(metadata.concreteFile.UnderlyingString(), flag, METAFS_UNDERLYING_UNDERLYING_FILE_PERM)

	if err != nil {
		//TODO: give more info about the error without leaking information about the underlying filesystem.
		return nil, fmt.Errorf("failed to open %s", pth)
	}

	if _, ok := underlyingFile.(afs.SyncCapable); !ok {
		return nil, errors.New("file returned by the underlying filesystem is not sync-capable")
	}

	files, ok := fls.openFiles[filename]
	if !ok {
		files = map[*metaFsFile]struct{}{}
		fls.openFiles[filename] = files
	}

	fls.untrackSomeClosedFiles(-1)

	file := &metaFsFile{
		path:           pth,
		fs:             fls,
		originalPath:   originalPath,
		normalizedPath: filename,
		flag:           flag,
		metadata:       metadata,
		underlying:     underlyingFile.(afs.SyncCapable),
	}

	files[file] = struct{}{}

	//we unlock because adding an event to fls.eventQueue is thread safe.
	fls.lock.Unlock()
	locked = false

	if created {
		//add event and remove old events.
		fls.eventQueue.EnqueueAutoRemove(Event{
			path:     core.Path(file.path),
			createOp: true,
			dateTime: metadata.creationTime,
		})
	}

	return file, nil
}

func (fls *MetaFilesystem) Stat(filename string) (os.FileInfo, error) {
	if fls.closedOrClosing.Load() {
		return nil, ErrClosedFilesystem
	}

	fls.lock.RLock()
	defer fls.lock.RUnlock()

	return fls.statNoLock(filename)
}

func (fls *MetaFilesystem) statNoLock(filename string) (os.FileInfo, error) {
	if fls.closedOrClosing.Load() {
		return nil, ErrClosedFilesystem
	}

	filename = NormalizeAsAbsolute(filename)

	metadata, exists, err := fls.getFileMetadata(core.PathFrom(filename), nil)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	var size core.ByteCount

	if metadata.concreteFile != nil {
		underlyingFilePath := *metadata.concreteFile
		stat, err := fls.underlying.Stat(string(underlyingFilePath))
		if err != nil {
			return nil, fmt.Errorf("failed to get stat of %s", filename)
		}
		size = core.ByteCount(stat.Size())
	}

	return core.FileInfo{
		BaseName_:       string(metadata.path.Basename()),
		AbsPath_:        metadata.path,
		Mode_:           core.FileMode(metadata.mode),
		CreationTime_:   metadata.creationTime,
		ModTime_:        metadata.modificationTime,
		HasCreationTime: true,
		Size_:           size,
	}, nil
}

func (fls *MetaFilesystem) Lstat(filename string) (os.FileInfo, error) {
	if fls.closedOrClosing.Load() {
		return nil, ErrClosedFilesystem
	}

	filename = NormalizeAsAbsolute(filename)

	fls.lock.RLock()
	defer fls.lock.RUnlock()

	metadata, exists, err := fls.getFileMetadata(core.PathFrom(filename), nil)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, os.ErrNotExist
	}

	if isSymlink(metadata.mode) {
		return nil, errors.New("symlinks not supported")
	}

	return fls.statNoLock(filename)
}

func (fls *MetaFilesystem) ReadDir(path string) ([]os.FileInfo, error) {
	if fls.closedOrClosing.Load() {
		return nil, ErrClosedFilesystem
	}

	fls.lock.RLock()
	defer fls.lock.RUnlock()

	path = NormalizeAsAbsolute(path)

	metadata, exists, err := fls.getFileMetadata(core.PathFrom(path), nil)

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
	for _, child := range metadata.ChildrenPaths() {
		stat, err := fls.statNoLock(child.UnderlyingString())
		if err != nil {
			return nil, err
		}
		entries = append(entries, stat)
	}

	sort.Sort(SortableFileInfo(entries))

	return entries, nil
}

func (fls *MetaFilesystem) MkdirAll(path string, perm os.FileMode) error {
	if fls.closedOrClosing.Load() {
		return ErrClosedFilesystem
	}

	fls.lock.Lock()
	defer fls.lock.Unlock()

	return fls.MkdirAllNoLock(path, perm)
}

func (fls *MetaFilesystem) MkdirAllNoLock(path string, perm os.FileMode) error {
	return fls.MkdirAllNoLock_(path, perm, nil)
}

func (fls *MetaFilesystem) MkdirAllNoLock_(path string, perm os.FileMode, tx *buntdb.Tx) error {
	if fls.closedOrClosing.Load() {
		return ErrClosedFilesystem
	}

	if path == "/" {
		return nil
	}

	path = NormalizeAsAbsolute(path)
	perm |= fs.ModeDir

	pth := core.DirPathFrom(path)

	metadata, exists, err := fls.getFileMetadata(pth, tx)

	if err != nil {
		return err
	}

	//TODO: use transaction

	if !exists { //create the directory

		//make sure the parent exists
		dir := filepath.Dir(path)
		dirPath := core.DirPathFrom(dir)

		if dir != "/" && dir != "." {
			err := fls.MkdirAllNoLock_(dir, perm, tx)
			if err != nil {
				return err
			}
		}

		//update metadata of parent
		dirMetadata, found, err := fls.getFileMetadata(dirPath, tx)
		if err != nil {
			return err
		}

		if !found {
			panic(core.ErrUnreachable)
		}

		dirMetadata.children = append(dirMetadata.children, pth.Basename())
		dirMetadata.modificationTime = core.DateTime(time.Now())
		if err := fls.setFileMetadata(dirMetadata, tx); err != nil {
			return err
		}

		//create metadata for new directory & store it
		creationTime := core.DateTime(time.Now())

		newDirMetadata := &metaFsFileMetadata{
			path:             pth,
			mode:             perm,
			creationTime:     creationTime,
			modificationTime: creationTime,
		}

		if err := fls.setFileMetadata(newDirMetadata, tx); err != nil {
			return err
		}

		//add event and remove old events.
		fls.eventQueue.EnqueueAutoRemove(Event{
			path:     newDirMetadata.path,
			createOp: true,
			dateTime: newDirMetadata.creationTime,
		})
	} else if !metadata.mode.IsDir() {
		//if there is a non-dir file we return an error
		return fmt.Errorf("%w at %q", os.ErrExist, path)
	}

	//TODO: support creating intermediary directories

	return nil
}

func (fls *MetaFilesystem) TempFile(dir, prefix string) (billy.File, error) {
	return util.TempFile(fls, dir, prefix)
}

func (fls *MetaFilesystem) Rename(from, to string) error {
	if fls.closedOrClosing.Load() {
		return ErrClosedFilesystem
	}

	fls.lock.Lock()
	defer fls.lock.Unlock()

	from = NormalizeAsAbsolute(from)
	to = NormalizeAsAbsolute(to)

	_, exists, err := fls.getFileMetadata(core.PathFrom(from), nil)

	if err != nil {
		return err
	}

	if !exists {
		return os.ErrNotExist
	}

	fromPath := core.PathFrom(from)
	toPath := core.PathFrom(to)

	from = fromPath.UnderlyingString()
	to = toPath.UnderlyingString()

	move := [][2]core.Path{{fromPath, toPath}}

	filesPrefix := METAFS_FILES_KEY

	//TODO: use a single transaction to search for move operations & do the update

	//iterate the metadata database to find all files & directories to move.

	noIssue := false
	tx, err := fls.metadata.Begin(true)
	if err != nil {
		return err
	}
	defer func() {
		if noIssue {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	err = tx.Ascend("", func(key, value string) (_continue bool) {
		_continue = true
		path := strings.TrimPrefix(string(key), filesPrefix)

		if path == string(key) { //prefix not present
			return
		}

		if path == from || !filepath.HasPrefix(path, from) {
			return
		}

		rel, _ := filepath.Rel(from, path)
		pathTo := filepath.Join(to, rel)

		move = append(move, [2]core.Path{core.PathFrom(path), core.PathFrom(pathTo)})
		return
	})

	if err != nil {
		return err
	}

	noCheckFuel := 10

	fromDir := filepath.Dir(from)
	// get metadata of previous parent directory
	fromDirPath := core.DirPathFrom(fromDir)

	fromDirMetadata, found, err := fls.getFileMetadata(fromDirPath, tx)
	if err != nil {
		return err
	}

	if !found {
		panic(core.ErrUnreachable)
	}

	// remove moved file from children of previous parent
	indexFound := false
	for index, child := range fromDirMetadata.children {
		if child == fromPath.Basename() {
			indexFound = true
			fromDirMetadata.children = utils.RemoveIndexOfSlice(fromDirMetadata.children, index)
			break
		}
	}

	if !indexFound {
		return fmt.Errorf("failed to remove %s from children of %s", fromPath.Basename(), fromDirPath)
	}

	fromDirMetadata.modificationTime = core.DateTime(time.Now())
	if err := fls.setFileMetadata(fromDirMetadata, tx); err != nil {
		return err
	}

	//make sure the parent of the the destination exists
	toDir := filepath.Dir(to)
	if err := fls.MkdirAllNoLock_(toDir, METAFS_AUTO_CREATED_DIR_PERM, tx); err != nil {
		return err
	}

	//add file in children of new parent
	toDirPath := core.DirPathFrom(toDir)

	toDirMetadata, found, err := fls.getFileMetadata(toDirPath, tx)
	if err != nil {
		return err
	}

	if !found {
		panic(core.ErrUnreachable)
	}

	toDirMetadata.children = append(toDirMetadata.children, toPath.Basename())
	toDirMetadata.modificationTime = core.DateTime(time.Now())

	if err := fls.setFileMetadata(toDirMetadata, tx); err != nil {
		return err
	}

	//update metadata of moved files & directories

	for opIndex, ops := range move {

		if noCheckFuel <= 0 { //check context
			select {
			case <-fls.ctx.Done():
				return fls.ctx.Err()
			default:
			}
			noCheckFuel = 10
		} else {
			noCheckFuel--
		}

		from := ops[0]
		to := ops[1]

		//get current metadata
		metadata, exists, err := fls.getFileMetadata(from, tx)
		if err != nil {
			return err
		}
		if !exists {
			panic(core.ErrUnreachable)
		}

		//update the metadata.
		//note that we do not need to update the underlying file since it
		//only contains the content.
		metadata.path = to

		err = fls.setFileMetadata(metadata, tx)
		if err != nil {
			return err
		}

		//delete previous metadata
		if err := fls.deleteFileMetadata(from, tx); err != nil {
			return err
		}

		//add event
		if opIndex == 0 {
			event := Event{
				path:     core.Path(metadata.path),
				renameOp: true,
				dateTime: core.DateTime(time.Now()),
			}

			//TODO: remove if unecessary
			if metadata.mode.IsDir() {
				event.path = pathutils.AppendTrailingSlashIfNotPresent(event.path)
			}

			//add event and remove old events.
			fls.eventQueue.EnqueueAutoRemove(event)
		}
	}
	noIssue = true
	return nil
}

func (fls *MetaFilesystem) Remove(filename string) error {
	if fls.closedOrClosing.Load() {
		return ErrClosedFilesystem
	}

	fls.lock.Lock()
	defer fls.lock.Unlock()

	filename = NormalizeAsAbsolute(filename)

	pth := core.PathFrom(filename)
	metadata, exists, err := fls.getFileMetadata(pth, nil)
	if err != nil {
		return err
	}
	if !exists {
		return os.ErrNotExist
	}

	if metadata.mode.IsDir() && len(metadata.children) > 0 {
		return errors.New(fmtDirContainFiles(filename))
	}

	noCheckFuel := 10

	var removed []core.Path
	var removalTimes []time.Time

	defer func() {
		defer utils.Recover()

		//add events
		//note: the events are not added one by one in order to reduce the number of lockings.
		events := make([]Event, len(removed))

		for i, path := range removed {
			events[i] = Event{
				path:     core.Path(path),
				removeOp: true,
				dateTime: core.DateTime(removalTimes[i]),
			}
		}

		//add event and remove old events.
		fls.eventQueue.EnqueueAllAutoRemove(events...)
	}()

	noIssue := false
	//create a temporary transaction
	tx, err := fls.metadata.Begin(true)
	if err != nil {
		return err
	}
	defer func() {
		if noIssue {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	dir := filepath.Dir(filename)
	dirPath := core.DirPathFrom(dir)

	//remove entry from parent
	parentMetadata, exists, err := fls.getFileMetadata(dirPath, tx)
	if err != nil {
		return err
	}
	if !exists {
		panic(core.ErrUnreachable)
	}

	found := false
	for index, childName := range parentMetadata.children {
		if childName == pth.Basename() {
			found = true
			parentMetadata.children = utils.RemoveIndexOfSlice(parentMetadata.children, index)
			break
		}
	}
	if !found {
		panic(core.ErrUnreachable)
	}

	parentMetadata.modificationTime = core.DateTime(time.Now())
	if err := fls.setFileMetadata(parentMetadata, tx); err != nil {
		return err
	}

	//remove concrete file (error is ignored for now)
	if metadata.concreteFile != nil {
		fls.underlying.Remove((*metadata.concreteFile).UnderlyingString())
	}

	//delete metadata
	if err := fls.deleteFileMetadata(metadata.path, tx); err != nil {
		return err
	}

	removed = append(removed, metadata.path)
	removalTimes = append(removalTimes, time.Time(parentMetadata.modificationTime))

	if !metadata.mode.IsDir() {
		noIssue = true
		return nil
	}

	fls.lastModificationTimesLock.Lock()
	delete(fls.lastModificationTimes, filename)
	fls.lastModificationTimesLock.Unlock()

	//remove descendants recursively (the code is not used yet because .Remove is not recursive)
	queue := slices.Clone(metadata.ChildrenPaths())

	for len(queue) > 0 {
		if noCheckFuel <= 0 { //check context
			select {
			case <-fls.ctx.Done():
				return fls.ctx.Err()
			default:
			}
			noCheckFuel = 10
		} else {
			noCheckFuel--
		}

		current := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		currentMetadata, exists, err := fls.getFileMetadata(current, tx)

		if err != nil {
			return err
		}

		if !exists {
			//the metadata should exist, continue anyway
			continue
		}

		//delete current descendant & add its own descendants to the queue
		if currentMetadata.mode.IsDir() {
			queue = append(queue, currentMetadata.ChildrenPaths()...)
		}

		//remove concrete file (error is ignored for now)
		if metadata.concreteFile != nil {
			fls.underlying.Remove((*metadata.concreteFile).UnderlyingString())
		}

		if err := fls.deleteFileMetadata(current, tx); err != nil {
			return err
		}

		removed = append(removed, metadata.path)
		removalTimes = append(removalTimes, time.Now())
	}
	noIssue = err == nil
	return err
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

func getKvKeyFromPath(pth core.Path) core.Path {
	key := METAFS_FILES_KEY + pth
	//remove trailing slash
	if key[len(key)-1] == '/' {
		key = key[:len(key)-1]
	}

	return key
}
