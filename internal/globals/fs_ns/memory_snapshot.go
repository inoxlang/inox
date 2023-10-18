package fs_ns

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

// An InMemorySnapshot is an implementation of FilesystemSnapshot that
// stores all the data & metadata in memory.
type InMemorySnapshot struct {
	MetadataMap      map[string]*core.EntrySnapshotMetadata
	FileContents     map[string]core.AddressableContent
	RootDirEntryList []*core.EntrySnapshotMetadata
}

func (s *InMemorySnapshot) Content(path string) (core.AddressableContent, error) {
	path = filepath.Clean(path)
	if path[0] != '/' {
		return nil, core.ErrSnapshotEntryPathMustBeAbsolute
	}
	metadata, ok := s.FileContents[path]
	if !ok {
		return nil, os.ErrNotExist
	}

	return metadata, nil
}

func (s *InMemorySnapshot) Metadata(path string) (core.EntrySnapshotMetadata, error) {
	path = filepath.Clean(path)
	if path[0] != '/' {
		return core.EntrySnapshotMetadata{}, core.ErrSnapshotEntryPathMustBeAbsolute
	}
	metadata, ok := s.MetadataMap[path]
	if !ok {
		return core.EntrySnapshotMetadata{}, os.ErrNotExist
	}

	return *metadata, nil
}

func (s *InMemorySnapshot) RootDirEntries() []core.EntrySnapshotMetadata {
	return utils.MapSlice(s.RootDirEntryList, func(m *core.EntrySnapshotMetadata) core.EntrySnapshotMetadata {
		return *m
	})
}
func (s *InMemorySnapshot) ForEachEntry(fn func(m core.EntrySnapshotMetadata) error) error {
	for _, metadata := range s.MetadataMap {
		err := fn(*metadata)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *InMemorySnapshot) IsStoredLocally() bool {
	return true
}

func (s *InMemorySnapshot) NewAdaptedFilesystem(maxTotalStorageSizeHint core.ByteCount) (afs.Filesystem, error) {
	maxTotalStorageSize := maxTotalStorageSizeHint
	fls := NewMemFilesystemFromSnapshot(s, maxTotalStorageSize)
	return fls, nil
}

type AddressableContentBytes struct {
	Sha256 [32]byte
	Data   []byte
}

func (b AddressableContentBytes) ChecksumSHA256() [32]byte {
	return b.Sha256
}

func (b AddressableContentBytes) Reader() io.Reader {
	return bytes.NewReader(b.Data)
}
