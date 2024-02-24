package fs_ns

import (
	"testing"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"gopkg.in/check.v1"

	"github.com/stretchr/testify/assert"
)

var _ = check.Suite(&MemoryFsTestSuite{})

type MemoryFsTestSuite struct {
	BasicTestSuite
	DirTestSuite
}

func (s *MemoryFsTestSuite) SetUpTest(c *check.C) {
	s.BasicTestSuite = BasicTestSuite{
		FS: NewMemFilesystem(100_000_00),
	}
	s.DirTestSuite = DirTestSuite{
		FS: NewMemFilesystem(100_000_00),
	}
}

func TestMemoryFilesystem(t *testing.T) {
	result := check.Run(&MemoryFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
	}
}

func TestMemoryFilesystemCapabilities(t *testing.T) {
	cases := []struct {
		name     string
		caps     billy.Capability
		expected bool
	}{
		{
			name:     "not lock capable",
			caps:     billy.LockCapability,
			expected: false,
		},
		{
			name:     "read capable",
			caps:     billy.ReadCapability,
			expected: true,
		},
		{
			name:     "read+write capable",
			caps:     billy.ReadCapability | billy.WriteCapability,
			expected: true,
		},
		{
			name:     "read+write+truncate capable",
			caps:     billy.ReadCapability | billy.WriteCapability | billy.ReadAndWriteCapability | billy.TruncateCapability,
			expected: true,
		},
		{
			name:     "not read+write+truncate+lock capable",
			caps:     billy.ReadCapability | billy.WriteCapability | billy.ReadAndWriteCapability | billy.TruncateCapability | billy.LockCapability,
			expected: false,
		},
		{
			name:     "not truncate+lock capable",
			caps:     billy.TruncateCapability | billy.LockCapability,
			expected: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fs := NewMemFilesystem(10_000_000)
			assert.Equal(t, testCase.expected, billy.CapabilityCheck(fs, testCase.caps))
		})
	}
}

func TestMemoryFilesystemTakeFilesystemSnapshot(t *testing.T) {
	const MAX_STORAGE_SIZE = 10_000

	alsoTestWriteTo := true

	testSnapshot(t, alsoTestWriteTo, func(t *testing.T) (*core.Context, core.SnapshotableFilesystem) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

		return ctx, NewMemFilesystem(MAX_STORAGE_SIZE)
	})
}

func TestNewMemFilesystemFromSnapshot(t *testing.T) {
	const MAX_STORAGE_SIZE = 10_000
	snapshotConfig := core.FilesystemSnapshotConfig{
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			//no cache
			return nil
		},
		InclusionFilters: []core.PathPattern{"/..."},
	}

	t.Run("empty filesystem", func(t *testing.T) {
		originalFS := NewMemFilesystem(MAX_STORAGE_SIZE)
		snapshot := utils.Must(originalFS.TakeFilesystemSnapshot(snapshotConfig))

		fs := NewMemFilesystemFromSnapshot(snapshot, MAX_STORAGE_SIZE)

		entries, err := fs.ReadDir("/")
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("file at root level", func(t *testing.T) {
		originalFS := NewMemFilesystem(MAX_STORAGE_SIZE)

		f, err := originalFS.Create("/file.txt")
		assert.NoError(t, err)
		f.Write([]byte("hello"))
		f.Close()

		snapshot := utils.Must(originalFS.TakeFilesystemSnapshot(snapshotConfig))
		fs := NewMemFilesystemFromSnapshot(snapshot, MAX_STORAGE_SIZE)

		entries, err := fs.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, entries, 1) {
			return
		}

		originalEntries, _ := originalFS.ReadDir("/")
		assert.Equal(t, originalEntries, entries)

		//check content of file
		content, err := util.ReadFile(fs, "/file.txt")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []byte("hello"), content)
	})

	t.Run("two files at root level", func(t *testing.T) {
		originalFS := NewMemFilesystem(MAX_STORAGE_SIZE)

		f, err := originalFS.Create("/file1.txt")
		assert.NoError(t, err)
		f.Write([]byte("hello1"))
		f.Close()

		f2, err := originalFS.Create("/file2.txt")
		assert.NoError(t, err)
		f2.Write([]byte("hello2"))
		f2.Close()

		snapshot := utils.Must(originalFS.TakeFilesystemSnapshot(snapshotConfig))
		fs := NewMemFilesystemFromSnapshot(snapshot, MAX_STORAGE_SIZE)

		entries, err := fs.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, entries, 2) {
			return
		}

		originalEntries, _ := originalFS.ReadDir("/")
		assert.Equal(t, originalEntries, entries)

		//check content of file 1
		content1, err := util.ReadFile(fs, "/file1.txt")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []byte("hello1"), content1)

		//check content1 of file 2
		content2, err := util.ReadFile(fs, "/file2.txt")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []byte("hello2"), content2)
	})

	t.Run("empty dir at root level", func(t *testing.T) {
		originalFS := NewMemFilesystem(MAX_STORAGE_SIZE)

		err := originalFS.MkdirAll("/dir", DEFAULT_DIR_FMODE)
		assert.NoError(t, err)

		snapshot := utils.Must(originalFS.TakeFilesystemSnapshot(snapshotConfig))
		fs := NewMemFilesystemFromSnapshot(snapshot, MAX_STORAGE_SIZE)

		//check the dir exists in the new filesystem
		entries, err := fs.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, entries, 1) {
			return
		}

		originalEntries, _ := originalFS.ReadDir("/")
		assert.Equal(t, originalEntries, entries)

		assert.Equal(t, "dir", entries[0].Name())
		assert.True(t, entries[0].IsDir())

		dirEntries, err := fs.ReadDir("/dir")
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, dirEntries)
	})

	t.Run("dir with file", func(t *testing.T) {
		originalFS := NewMemFilesystem(MAX_STORAGE_SIZE)

		//create dir
		err := originalFS.MkdirAll("/dir", DEFAULT_DIR_FMODE)
		assert.NoError(t, err)

		//create file
		f, err := originalFS.Create("/dir/file.txt")
		assert.NoError(t, err)
		f.Write([]byte("hello"))

		snapshot := utils.Must(originalFS.TakeFilesystemSnapshot(snapshotConfig))
		fs := NewMemFilesystemFromSnapshot(snapshot, MAX_STORAGE_SIZE)

		//check the dir exists in the new filesystem
		entries, err := fs.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, entries, 1) {
			return
		}

		originalEntries, _ := originalFS.ReadDir("/")
		assert.Equal(t, originalEntries, entries)

		assert.Equal(t, "dir", entries[0].Name())
		assert.True(t, entries[0].IsDir())

		//check the file
		originalDirEntries, err := originalFS.ReadDir("/dir")
		if !assert.NoError(t, err) {
			return
		}

		dirEntries, err := fs.ReadDir("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, originalDirEntries, dirEntries)

		//check content of file
		content1, err := util.ReadFile(fs, "/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []byte("hello"), content1)
	})
}
