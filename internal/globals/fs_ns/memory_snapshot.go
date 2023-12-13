package fs_ns

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
)

// An InMemorySnapshot is an implementation of FilesystemSnapshot that
// stores all the data & metadata in memory.
type InMemorySnapshot struct {
	MetadataMap  map[string]*core.EntrySnapshotMetadata
	FileContents map[string]core.AddressableContent
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

func (s *InMemorySnapshot) RootDirEntries() []string {
	return slices.Clone(s.MetadataMap["/"].ChildNames)
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

func (s *InMemorySnapshot) NewAdaptedFilesystem(maxTotalStorageSizeHint core.ByteCount) (core.SnapshotableFilesystem, error) {
	maxTotalStorageSize := maxTotalStorageSizeHint
	fls := NewMemFilesystemFromSnapshot(s, maxTotalStorageSize)
	return fls, nil
}

func (s *InMemorySnapshot) WriteTo(fls afs.Filesystem, params core.SnapshotWriteToFilesystem) error {
	return s.ForEachEntry(func(m core.EntrySnapshotMetadata) error {
		path := string(m.AbsolutePath)
		stat, err := fls.Stat(path)
		needOverwrite := err == nil

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if needOverwrite {
			if !params.Overwrite {
				return fmt.Errorf("found a file or directory at %q but overwriting is not allowed", path)
			}
			if !stat.IsDir() {
				err = fls.Remove(path)
				if err != nil {
					return err
				}
			} else {
				//TODO: if the permissions of the directory are different from the ones specified in the snapshot,
				//change the permissions by creating a new dir and renaming it.
			}
		}

		if m.IsDir() {
			return fls.MkdirAll(path, m.Mode.FileMode().Perm())
		} else {
			content, err := s.Content(path)
			if err != nil {
				return err
			}

			f, err := fls.OpenFile(path, os.O_WRONLY|os.O_CREATE, m.Mode.FileMode().Perm())
			if err != nil {
				return err
			}

			_, err = io.Copy(f, content.Reader())
			return err
		}
	})
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
