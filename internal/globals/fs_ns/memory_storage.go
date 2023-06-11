package fs_ns

//slight modification of https://github.com/go-git/go-billy/blob/master/memfs/memory.go (Apache 2.0 license)

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/inoxlang/inox/internal/core"
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

	return storage
}

func (s *inMemStorage) Has(path string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.hasNoLock(path)
}

func (s *inMemStorage) hasNoLock(path string) bool {
	path = clean(path)

	_, ok := s.files[path]
	return ok
}

func (s *inMemStorage) New(path string, mode os.FileMode, flag int) (*inMemfile, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.newNoLock(path, mode, flag)
}

func (s *inMemStorage) newNoLock(path string, mode os.FileMode, flag int) (*inMemfile, error) {
	path = clean(path)
	if s.hasNoLock(path) {
		if !s.mustGetNoLock(path).mode.IsDir() {
			return nil, fmt.Errorf("file already exists %q", path)
		}

		return nil, nil
	}

	name := filepath.Base(path)
	now := time.Now()

	f := &inMemfile{
		name: name,
		content: &inMemFileContent{
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
	base = clean(base)
	if f.Name() == string(separator) {
		return nil
	}

	if _, err := s.newNoLock(base, mode.Perm()|os.ModeDir, 0); err != nil {
		return err
	}

	if _, ok := s.children[base]; !ok {
		s.children[base] = make(map[string]*inMemfile, 0)
	}

	s.children[base][f.Name()] = f
	return nil
}

func (s *inMemStorage) Children(path string) []*inMemfile {
	s.lock.RLock()
	defer s.lock.RUnlock()

	path = clean(path)

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
	path = clean(path)
	if !s.hasNoLock(path) {
		return nil, false
	}

	file, ok := s.files[path]
	return file, ok
}

func (s *inMemStorage) Rename(from, to string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	from = clean(from)
	to = clean(to)

	if !s.hasNoLock(from) {
		return os.ErrNotExist
	}

	move := [][2]string{{from, to}}

	for pathFrom := range s.files {
		if pathFrom == from || !filepath.HasPrefix(pathFrom, from) {
			continue
		}

		rel, _ := filepath.Rel(from, pathFrom)
		pathTo := filepath.Join(to, rel)

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
	s.files[to].name = filepath.Base(to)
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

	path = clean(path)

	f, has := s.getNoLock(path)
	if !has {
		return os.ErrNotExist
	}

	if f.mode.IsDir() && len(s.children[path]) != 0 {
		return fmt.Errorf("dir: %s contains files", path)
	}

	base, file := filepath.Split(path)
	base = filepath.Clean(base)

	delete(s.children[base], file)
	delete(s.files, path)
	return nil
}

func clean(path string) string {
	return filepath.Clean(filepath.FromSlash(path))
}

type inMemFileContent struct {
	name                     string
	bytes                    []byte
	creationTime             time.Time
	modificationTime         atomic.Value //time.Time
	filesystemMaxStorageSize int64
	filesystemStorageSize    *atomic.Int64

	m sync.RWMutex
}

func (c *inMemFileContent) ModifTime() time.Time {
	return c.modificationTime.Load().(time.Time)
}

func (c *inMemFileContent) Truncate(size int64) error {
	if size <= int64(len(c.bytes)) {
		c.filesystemStorageSize.Add(-int64(len(c.bytes)))
		c.bytes = c.bytes[:size]
	} else {
		more := int(size) - len(c.bytes)

		if c.filesystemStorageSize.Add(int64(more)) > c.filesystemMaxStorageSize {
			return ErrInMemoryStorageLimitExceededDuringWrite
		}

		c.filesystemStorageSize.Add(-int64(len(c.bytes)))

		c.bytes = append(c.bytes, make([]byte, more)...)
	}

	return nil
}

func (c *inMemFileContent) Len() int {
	return len(c.bytes)
}

func (c *inMemFileContent) WriteAt(p []byte, off int64) (int, error) {
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

func (c *inMemFileContent) ReadAt(b []byte, off int64) (n int, err error) {
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
