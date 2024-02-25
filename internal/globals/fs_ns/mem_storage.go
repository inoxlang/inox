package fs_ns

//slight modification of https://github.com/go-git/go-billy/blob/main/memfs/memory.go (Apache 2.0 license)

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	TRUE_MAX_IN_MEM_STORAGE = core.ByteCount(100_000_000)
)

var (
	ErrInMemoryStorageLimitExceededDuringWrite = errors.New("in-memory file storage limit exceeded during write operation")
)

type inMemStorage struct {
	lock             sync.RWMutex
	files            map[string]*InMemfile
	children         map[string]map[ /* basename */ string]*InMemfile
	totalContentSize atomic.Int64
	maxStorageSize   int64

	eventQueue *memds.TSArrayQueue[Event] //periodically emptied
}

func newInMemoryStorage(maxStorageSize core.ByteCount) *inMemStorage {
	if maxStorageSize <= 50 {
		panic(errors.New("given max. total content size should be > 50"))
	}

	if int64(maxStorageSize) > int64(TRUE_MAX_IN_MEM_STORAGE) {
		panic(errors.New("given max. total content size is greater than the true maximum"))
	}

	storage := &inMemStorage{
		files:          make(map[string]*InMemfile, 0),
		children:       make(map[string]map[string]*InMemfile, 0),
		maxStorageSize: int64(maxStorageSize),

		eventQueue: memds.NewTSArrayQueueWithConfig(memds.TSArrayQueueConfig[Event]{
			AutoRemoveCondition: isOldEvent,
		}),
	}

	f, err := storage.newNoLock("/", 0700|fs.ModeDir, 0, true)
	if err != nil {
		panic(err)
	}
	f.Close()

	return storage
}

func newInMemoryStorageFromSnapshot(snapshot core.FilesystemSnapshot, maxStorageSize core.ByteCount) *inMemStorage {
	storage := newInMemoryStorage(maxStorageSize)

	//create all files & directories
	snapshot.ForEachEntry(func(metadata core.EntrySnapshotMetadata) error {
		path := NormalizeAsAbsolute(string(metadata.AbsolutePath))

		file := &InMemfile{
			basename:          filepath.Base(path),
			originalPath:      path,
			absPath:           metadata.AbsolutePath,
			flag:              0,
			mode:              fs.FileMode(metadata.Mode),
			storageLastEvents: storage.eventQueue,
		}
		storage.files[path] = file

		file.content = &InMemFileContent{
			name:                     file.basename,
			creationTime:             time.Time(metadata.CreationTime),
			filesystemMaxStorageSize: storage.maxStorageSize,
			filesystemStorageSize:    &storage.totalContentSize,
		}

		file.content.modificationTime.Store(time.Time(metadata.ModificationTime))

		if !metadata.IsRegularFile() {
			return nil
		}

		content, err := snapshot.Content(path)
		if err != nil {
			return err
		}

		file.content.bytes = utils.Must(io.ReadAll(content.Reader()))

		if len(file.content.bytes) != int(metadata.Size) {
			return fmt.Errorf("failed to create filesystem from snapshot, inconsistency: size of file %s is %d but size of content is %d",
				path, metadata.Size, len(file.content.bytes))
		}

		return nil
	})

	//create structure
	children := map[string]*InMemfile{}
	storage.children["/"] = children

	snapshot.ForEachEntry(func(metadata core.EntrySnapshotMetadata) error {
		path := NormalizeAsAbsolute(string(metadata.AbsolutePath))
		file := storage.files[path]

		if !file.mode.IsDir() {
			return nil
		}

		children := map[string]*InMemfile{}
		storage.children[path] = children

		for _, child := range metadata.ChildNames {
			childPath := filepath.Join(path, child)
			children[child] = storage.files[childPath]
		}
		return nil
	})

	return storage
}

func (s *inMemStorage) Has(path string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.hasNoLock(path)
}

func (s *inMemStorage) hasNoLock(path string) bool {
	path = NormalizeAsAbsolute(path)

	_, ok := s.files[path]
	return ok
}

func (s *inMemStorage) New(path string, mode os.FileMode, flag int) (*InMemfile, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.newNoLock(path, mode, flag, false)
}

func (s *inMemStorage) newNoLock(path string, mode os.FileMode, flag int, ignoreEvent bool) (*InMemfile, error) {
	originalPath := path
	path = NormalizeAsAbsolute(path)
	if s.hasNoLock(path) {
		//if there is a non-dir file we return an error
		if !s.mustGetNoLock(path).mode.IsDir() {
			return nil, fmt.Errorf("%w at %q", os.ErrExist, path)
		}

		return nil, nil
	}

	name := filepath.Base(path)
	now := time.Now()

	f := &InMemfile{
		basename:     name,
		originalPath: originalPath,
		content: &InMemFileContent{
			name:         name,
			creationTime: now,
		},
		mode:              mode,
		flag:              flag,
		storageLastEvents: s.eventQueue,
	}

	if f.mode.IsDir() {
		f.absPath = core.DirPathFrom(path)
	} else {
		f.absPath = core.NonDirPathFrom(path)
	}

	f.content.modificationTime.Store(now)

	f.content.filesystemStorageSize = &s.totalContentSize
	f.content.filesystemMaxStorageSize = s.maxStorageSize

	s.files[path] = f
	s.createUpdateParentNoLock(path, mode, f)

	if !ignoreEvent {
		func() {
			defer utils.Recover()

			event := Event{
				path:     core.Path(f.absPath),
				createOp: true,
				dateTime: core.DateTime(now),
			}

			if f.mode.IsDir() {
				event.path = core.AppendTrailingSlashIfNotPresent(event.path)
			}

			//add event and remove old events.
			s.eventQueue.EnqueueAutoRemove(event)
		}()
	}

	return f, nil
}

func (s *inMemStorage) createUpdateParentNoLock(path string, mode os.FileMode, f *InMemfile) error {
	base := filepath.Dir(path)
	if base == "." {
		base = "/"
	}
	base = NormalizeAsAbsolute(base)

	dir, ok := s.files[base]
	if ok {
		dir.content.modificationTime.Store(time.Now())
	}

	if f.Name() == "/" {
		return nil
	}

	if base != "/" {
		if _, err := s.newNoLock(base, mode.Perm()|os.ModeDir, 0, true); err != nil {
			return err
		}
	}

	if _, ok := s.children[base]; !ok {
		s.children[base] = make(map[string]*InMemfile, 0)
	}

	s.children[base][f.basename] = f
	return nil
}

func (s *inMemStorage) Children(path string) []*InMemfile {
	s.lock.RLock()
	defer s.lock.RUnlock()

	path = NormalizeAsAbsolute(path)

	l := make([]*InMemfile, 0)
	for _, f := range s.children[path] {
		l = append(l, f)
	}

	return l
}

func (s *inMemStorage) MustGet(path string) *InMemfile {
	s.lock.RLock()
	defer s.lock.RUnlock()

	f, ok := s.getNoLock(path)
	if !ok {
		panic(fmt.Errorf("couldn't find %q", path))
	}

	return f
}

func (s *inMemStorage) Get(path string) (*InMemfile, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.getNoLock(path)
}

func (s *inMemStorage) mustGetNoLock(path string) *InMemfile {
	f, ok := s.getNoLock(path)
	if !ok {
		panic(fmt.Errorf("couldn't find %q", path))
	}

	return f
}

func (s *inMemStorage) getNoLock(path string) (*InMemfile, bool) {
	path = NormalizeAsAbsolute(path)
	if !s.hasNoLock(path) {
		return nil, false
	}

	file, ok := s.files[path]
	if ok {
		file.isClosed.Store(false)
	}
	return file, ok
}

func (s *inMemStorage) Rename(from, to string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	from = NormalizeAsAbsolute(from)
	to = NormalizeAsAbsolute(to)

	if !s.hasNoLock(from) {
		return os.ErrNotExist
	}

	move := [][2]string{{from, to}}

	for pathFrom := range s.files {
		if pathFrom == from || !strings.HasPrefix(pathFrom, from) {
			continue
		}

		rel, _ := filepath.Rel(from, pathFrom)
		pathTo := NormalizeAsAbsolute(filepath.Join(to, rel))

		move = append(move, [2]string{pathFrom, pathTo})
	}

	for i, ops := range move {
		//ignore events for all move operations expected the first one.
		ignoreEvent := i != 0

		opFrom := ops[0]
		opTo := ops[1]

		if err := s.moveNoLock(opFrom, opTo, ignoreEvent); err != nil {
			return err
		}
	}

	return nil
}

func (s *inMemStorage) moveNoLock(from, to string, ignoreEvent bool) error {
	s.files[to] = s.files[from]
	f := s.files[to]
	f.basename = filepath.Base(to)
	f.absPath = core.PathFrom(to)
	f.originalPath = to

	s.children[to] = s.children[from]

	//add event
	if !ignoreEvent {
		defer func() {
			defer utils.Recover()
			event := Event{
				path:     core.Path(f.absPath),
				renameOp: true,
				dateTime: core.DateTime(time.Now()),
			}

			if f.mode.IsDir() {
				event.path = core.AppendTrailingSlashIfNotPresent(event.path)
			}

			//add event and remove old events.
			s.eventQueue.EnqueueAutoRemove(event)
		}()
	}

	defer func() {
		delete(s.children, from)
		delete(s.files, from)
		delete(s.children[filepath.Dir(from)], filepath.Base(from))
	}()

	return s.createUpdateParentNoLock(to, 0644, s.files[to])
}

func (s *inMemStorage) Remove(path string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	path = NormalizeAsAbsolute(path)

	f, has := s.getNoLock(path)
	if !has {
		return os.ErrNotExist
	}

	isDir := f.mode.IsDir()

	if isDir && len(s.children[path]) != 0 {
		return errors.New(fmtDirContainFiles(path))
	}

	now := time.Now()

	base, file := filepath.Split(path)
	base = filepath.Clean(base)
	s.files[base].content.modificationTime.Store(now)

	delete(s.children[base], file)
	delete(s.files, path)

	func() {
		defer utils.Recover()
		event := Event{
			path:     core.Path(f.absPath),
			removeOp: true,
			dateTime: core.DateTime(now),
		}

		if isDir {
			event.path = core.AppendTrailingSlashIfNotPresent(event.path)
		}

		//add event and remove old events.
		s.eventQueue.EnqueueAutoRemove(event)
	}()

	return nil
}

func NormalizeAsAbsolute(path string) string {
	if path == "" { //An empty path can be passed by go-git.
		return "/"
	}

	path = filepath.Clean(path)

	if path != "/" && path[0] != '/' {
		return "/" + path
	}

	return path
}

type InMemFileContent struct {
	name                      string
	bytes                     []byte
	creationTime              time.Time
	modificationTime          atomic.Value //time.Time
	filesystemMaxStorageSize  int64
	filesystemStorageSize     *atomic.Int64
	dirty                     bool
	beingPersisted            bool
	modifiedDuringPersistence bool

	lock sync.RWMutex
}

func NewInMemFileContent(
	name string,
	content []byte,
	creationTime time.Time,
	maxStorage int64, storageSize *atomic.Int64,
) *InMemFileContent {
	c := &InMemFileContent{
		name:                     name,
		bytes:                    content,
		creationTime:             creationTime,
		filesystemMaxStorageSize: maxStorage,
		filesystemStorageSize:    storageSize,
	}
	c.modificationTime.Store(creationTime)
	return c
}

func (c *InMemFileContent) IsDirty() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.dirty
}

// ShouldBePersisted returns true if the content is dirty AND is not being persisted.
func (c *InMemFileContent) ShouldBePersisted() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.dirty && !c.beingPersisted
}

// If the file is not dirty Persist snapshots the content of the file & invokes persistFn,
// if persistFn returns an error or panics an error is returned.
func (c *InMemFileContent) Persist(persistFn func(p []byte) error) (finalErr error) {
	c.lock.Lock()
	if !c.dirty {
		c.lock.Unlock()
		return
	}
	bytes := slices.Clone(c.bytes)
	c.beingPersisted = true
	c.lock.Unlock()

	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			finalErr = fmt.Errorf("%w: %s", err, string(debug.Stack()))
		}

		c.lock.Lock()
		defer c.lock.Unlock()

		if finalErr == nil && !c.modifiedDuringPersistence {
			c.dirty = false
		}

		c.modifiedDuringPersistence = false
	}()

	return persistFn(bytes)
}

func (c *InMemFileContent) ModifTime() time.Time {
	return c.modificationTime.Load().(time.Time)
}

func (c *InMemFileContent) Truncate(size int64) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	defer func() {
		c.dirty = true
		if c.beingPersisted {
			c.modifiedDuringPersistence = true
		}
	}()
	c.modificationTime.Store(time.Now())

	if size <= int64(len(c.bytes)) {
		//TODO: re-use free space, otherwise the actual memory usage
		//of the filesystem could be way larger than c.filesystemStorageSize.

		c.filesystemStorageSize.Add(-int64(len(c.bytes)))
		c.bytes = c.bytes[:size]
	} else {
		more := int(size) - len(c.bytes)

		if c.filesystemStorageSize.Add(int64(more)) > c.filesystemMaxStorageSize {
			return ErrInMemoryStorageLimitExceededDuringWrite
		}

		c.filesystemStorageSize.Add(-int64(len(c.bytes)))

		if cap(c.bytes)-len(c.bytes) > more {
			c.bytes = c.bytes[:len(c.bytes)+more]
		} else {
			c.bytes = append(c.bytes, make([]byte, more)...)
		}
	}

	return nil
}

func (c *InMemFileContent) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.lenNoLock()
}

func (c *InMemFileContent) lenNoLock() int {
	return len(c.bytes)
}

func (c *InMemFileContent) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, &os.PathError{
			Op:   "writeat",
			Path: c.name,
			Err:  errors.New("negative offset"),
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	defer func() {
		c.dirty = true
		if c.beingPersisted {
			c.modifiedDuringPersistence = true
		}
	}()
	prev := len(c.bytes)

	diff := int(off) - prev
	if diff > 0 {
		if c.filesystemStorageSize.Add(int64(diff)) > c.filesystemMaxStorageSize {
			return 0, ErrInMemoryStorageLimitExceededDuringWrite
		}

		c.bytes = append(c.bytes, make([]byte, diff)...)
	}

	destSliceLength := int64(len(c.bytes[:off]))
	allocationSize := int64(len(p)) - destSliceLength

	if allocationSize > 0 && c.filesystemStorageSize.Add(allocationSize) > c.filesystemMaxStorageSize {
		return 0, ErrInMemoryStorageLimitExceededDuringWrite
	}

	c.modificationTime.Store(time.Now())

	c.bytes = append(c.bytes[:off], p...)
	if len(c.bytes) < prev {
		c.bytes = c.bytes[:prev]
	}

	return len(p), nil
}

func (c *InMemFileContent) ReadAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, &os.PathError{
			Op:   "readat",
			Path: c.name,
			Err:  errors.New("negative offset"),
		}
	}

	c.lock.RLock()
	size := int64(len(c.bytes))
	if off >= size {
		c.lock.RUnlock()
		return 0, io.EOF
	}

	defer c.lock.RUnlock()

	l := int64(len(b))
	if off+l > size {
		l = size - off
	}

	btr := c.bytes[off : off+l]
	n = copy(b, btr)

	if len(btr) < len(b) {
		err = io.EOF
	}

	return
}
