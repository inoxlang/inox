package core

import (
	"errors"
	"io"

	"github.com/inoxlang/inox/internal/afs"
)

var (
	ErrSnapshotEntryPathMustBeAbsolute = errors.New("snapshot file path must be absolute")
	ErrSnapshotEntryNotAFile           = errors.New("filesystem entry is not a file")

	_ = Value((*FilesystemSnapshotIL)(nil))
	_ = Serializable((*FilesystemSnapshotIL)(nil))
)

type SnapshotableFilesystem interface {
	afs.Filesystem
	TakeFilesystemSnapshot(getContent func(ChecksumSHA256 [32]byte) AddressableContent) (FilesystemSnapshot, error)
}

// A FilesystemSnapshot represents an immutable snapshot of a filesystem,
// the data & metadata can be stored anywhere (memory, disk, object storage).
type FilesystemSnapshot interface {
	//Metadata returns the metadata of an entry inside the snapshot.
	//If the given path is not absolute ErrSnapshotEntryPathMustBeAbsolute should be returned.
	//If the file does not exist os.ErrNotExist should be returned.
	Metadata(path string) (EntrySnapshotMetadata, error)

	RootDirEntries() []EntrySnapshotMetadata

	ForEachEntry(func(m EntrySnapshotMetadata) error) error

	//Content returns an AddressableContent value that should be able to retrieve the content of a file inside the snapshot.
	//If the given path is not absolute ErrSnapshotFilePathMustBeAbsolute should be returned.
	//If the file does not exist os.ErrNotExist should be returned.
	//If the entry at the path is not a file ErrSnapshotEntryNotAFile should be returned.
	Content(path string) (AddressableContent, error)

	//IsStoredLocally should return true if all of the data & metadata is stored in memory and/or on disk.
	IsStoredLocally() bool

	//NewAdaptedFilesystem creates a filesystem from the snapshot,
	//it should be adapted to the FilesystemSnapshot implementation.
	NewAdaptedFilesystem(maxTotalStorageSizeHint ByteCount) (SnapshotableFilesystem, error)
}

type AddressableContent interface {
	ChecksumSHA256() [32]byte
	Reader() io.Reader
}

type EntrySnapshotMetadata struct {
	AbsolutePath     Path
	Size             ByteCount
	CreationTime     Date
	ModificationTime Date
	Mode             FileMode
	ChildNames       []string
	ChecksumSHA256   [32]byte //empty if directory
}

func (m EntrySnapshotMetadata) IsDir() bool {
	return m.Mode.FileMode().Type().IsDir()
}

func (m EntrySnapshotMetadata) IsRegularFile() bool {
	return m.Mode.FileMode().Type().IsRegular()
}

// FilesystemSnapshotIL wraps a FilesystemSnapshot and implements Value.
type FilesystemSnapshotIL struct {
	underlying FilesystemSnapshot
}

func WrapFsSnapshot(snapshot FilesystemSnapshot) *FilesystemSnapshotIL {
	return &FilesystemSnapshotIL{
		underlying: snapshot,
	}
}

func (s *FilesystemSnapshotIL) Underlying() FilesystemSnapshot {
	return s.underlying
}
