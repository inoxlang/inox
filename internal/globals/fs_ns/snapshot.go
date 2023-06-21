package fs_ns

import (
	"bytes"
	"io"

	"github.com/inoxlang/inox/internal/core"
)

type SnapshotableFilesystem interface {
	TakeFilesystemSnapshot(getContent func(ChecksumSHA256 [32]byte) AddressableContent) FilesystemSnapshot
}

type FilesystemSnapshot struct {
	Metadata     map[string]*FileMetadata
	FileContents map[string]AddressableContent
}

type FileMetadata struct {
	AbsolutePath     core.Path
	Size             core.ByteCount
	CreationTime     core.Date
	ModificationTime core.Date
	Mode             core.FileMode
	ChildNames       []string
	ChecksumSHA256   [32]byte //empty if directory
}

type AddressableContent interface {
	ChecksumSHA256() [32]byte
	Reader() io.Reader
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
