package fs_ns

//slight modification of https://github.com/go-git/go-billy/blob/master/memfs/memory.go (Apache 2.0 license)

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/inoxlang/inox/internal/core"
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
	files            map[string]*inMemfile
	children         map[string]map[string]*inMemfile
	totalContentSize atomic.Int64
	maxStorageSize   int64
}

func newInMemoryStorage(maxStorageSize core.ByteCount) *inMemStorage {
	if maxStorageSize <= 50 {
		panic(errors.New("given max. total content size should be > 50"))
	}

	if int64(maxStorageSize) > int64(TRUE_MAX_IN_MEM_STORAGE) {
		panic(errors.New("given max. total content size is greater than the true maximum"))
	}

	storage := &inMemStorage{
		files:          make(map[string]*inMemfile, 0),
		children:       make(map[string]map[string]*inMemfile, 0),
		maxStorageSize: int64(maxStorageSize),
	}

	f, err := storage.newNoLock("/", 0700|fs.ModeDir, 0)
	if err != nil {
		panic(err)
	}
	f.Close()

	return storage
}

func newInMemoryStorageFromSnapshot(snapshot FilesystemSnapshot, maxStorageSize core.ByteCount) *inMemStorage {
	storage := newInMemoryStorage(maxStorageSize)

	//create all files & directories
	for path, metadata := range snapshot.Metadata {
		file := &inMemfile{
			basename:     filepath.Base(path),
			originalPath: path,
			absPath:      metadata.AbsolutePath,
			flag:         0,
			mode:         fs.FileMode(metadata.Mode),
		}
		storage.files[path] = file
		content, ok := snapshot.FileContents[path]

		file.content = &InMemFileContent{
			name:                     file.basename,
			creationTime:             time.Time(metadata.CreationTime),
			filesystemMaxStorageSize: storage.maxStorageSize,
			filesystemStorageSize:    &storage.totalContentSize,
		}

		file.content.modificationTime.Store(time.Time(metadata.ModificationTime))

		if ok {
			file.content.bytes = utils.Must(io.ReadAll(content.Reader()))

			if len(file.content.bytes) != int(metadata.Size) {
				panic(fmt.Errorf("failed to create filesystem from snapshot, inconsistency: size of file %s is %d but size of content is %d",
					path, metadata.Size, len(file.content.bytes)))
			}
		} else {

		}
	}

	//create structure

	children := map[string]*inMemfile{}
	storage.children["/"] = children

	for path, metadata := range snapshot.Metadata {
		file := storage.files[path]

		if !file.mode.IsDir() {
			continue
		}

		children := map[string]*inMemfile{}
		storage.children[path] = children

		for _, child := range metadata.ChildNames {
			childPath := filepath.Join(path, child)
			children[child] = storage.files[childPath]
		}
	}
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

func (s *inMemStorage) New(path string, mode os.FileMode, flag int) (*inMemfile, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.newNoLock(path, mode, flag)
}

func (s *inMemStorage) newNoLock(path string, mode os.FileMode, flag int) (*inMemfile, error) {
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

	f := &inMemfile{
		basename:     name,
		originalPath: originalPath,
		absPath:      core.PathFrom(path),
		content: &InMemFileContent{
			name:         name,
			creationTime: now,
		},
		mode: mode,
		flag: flag,
	}

	f.content.modificationTime.Store(now)

	f.content.filesystemStorageSize = &s.totalContentSize
	f.content.filesystemMaxStorageSize = s.maxStorageSize

	s.files[path] = f
	s.createParentNoLock(path, mode, f)
	return f, nil
}

func (s *inMemStorage) createParentNoLock(path string, mode os.FileMode, f *inMemfile) error {
	base := filepath.Dir(path)
	if base == "." {
		base = "/"
	}
	base = NormalizeAsAbsolute(base)

	if f.Name() == "/" {
		return nil
	}

	if base != "/" {
		if _, err := s.newNoLock(base, mode.Perm()|os.ModeDir, 0); err != nil {
			return err
		}
	}

	if _, ok := s.children[base]; !ok {
		s.children[base] = make(map[string]*inMemfile, 0)
	}

	s.children[base][f.basename] = f
	return nil
}

func (s *inMemStorage) Children(path string) []*inMemfile {
	s.lock.RLock()
	defer s.lock.RUnlock()

	path = NormalizeAsAbsolute(path)

	l := make([]*inMemfile, 0)
	for _, f := range s.children[path] {
		l = append(l, f)
	}

	return l
}

func (s *inMemStorage) MustGet(path string) *inMemfile {
	s.lock.RLock()
	defer s.lock.RUnlock()

	f, ok := s.getNoLock(path)
	if !ok {
		panic(fmt.Errorf("couldn't find %q", path))
	}

	return f
}

func (s *inMemStorage) Get(path string) (*inMemfile, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.getNoLock(path)
}

func (s *inMemStorage) mustGetNoLock(path string) *inMemfile {
	f, ok := s.getNoLock(path)
	if !ok {
		panic(fmt.Errorf("couldn't find %q", path))
	}

	return f
}

func (s *inMemStorage) getNoLock(path string) (*inMemfile, bool) {
	path = NormalizeAsAbsolute(path)
	if !s.hasNoLock(path) {
		return nil, false
	}

	file, ok := s.files[path]
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

	for _, ops := range move {
		from := ops[0]
		to := ops[1]

		if err := s.moveNoLock(from, to); err != nil {
			return err
		}
	}

	return nil
}

func (s *inMemStorage) moveNoLock(from, to string) error {
	s.files[to] = s.files[from]
	f := s.files[to]
	f.basename = filepath.Base(to)
	f.absPath = core.PathFrom(to)
	f.originalPath = to

	s.children[to] = s.children[from]

	defer func() {
		delete(s.children, from)
		delete(s.files, from)
		delete(s.children[filepath.Dir(from)], filepath.Base(from))
	}()

	return s.createParentNoLock(to, 0644, s.files[to])
}

func (s *inMemStorage) Remove(path string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	path = NormalizeAsAbsolute(path)

	f, has := s.getNoLock(path)
	if !has {
		return os.ErrNotExist
	}

	if f.mode.IsDir() && len(s.children[path]) != 0 {
		return errors.New(fmtDirContainFiles(path))
	}

	base, file := filepath.Split(path)
	base = filepath.Clean(base)

	delete(s.children[base], file)
	delete(s.files, path)
	return nil
}

func NormalizeAsAbsolute(path string) string {
	path = filepath.Clean(path)

	if path != "/" && path[0] != '/' {
		return "/" + path
	}

	return path
}

type InMemFileContent struct {
	name                     string
	bytes                    []byte
	creationTime             time.Time
	modificationTime         atomic.Value //time.Time
	filesystemMaxStorageSize int64
	filesystemStorageSize    *atomic.Int64

	m sync.RWMutex
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

func (c *InMemFileContent) BytesToNotModify() []byte {
	return c.bytes
}

func (c *InMemFileContent) ModifTime() time.Time {
	return c.modificationTime.Load().(time.Time)
}

func (c *InMemFileContent) Truncate(size int64) error {
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

	c.m.Lock()
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
	c.m.Unlock()

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

	c.m.RLock()
	size := int64(len(c.bytes))
	if off >= size {
		c.m.RUnlock()
		return 0, io.EOF
	}

	l := int64(len(b))
	if off+l > size {
		l = size - off
	}

	btr := c.bytes[off : off+l]
	n = copy(b, btr)

	if len(btr) < len(b) {
		err = io.EOF
	}
	c.m.RUnlock()

	return
}
