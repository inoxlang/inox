package internal

import (
	"io/fs"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
)

type FsFileStatParams struct {
	FileURI defines.URI `json:"uri"`
}

type FsFileStat struct {
	//The creation timestamp in milliseconds elapsed since January 1, 1970 00:00:00 UTC.
	CTime int64 `json:"ctime"`

	//The modification timestamp in milliseconds elapsed since January 1, 1970 00:00:00 UTC.
	MTime int64 `json:"mtime"`

	//The size in bytes.
	Size int64 `json:"size"`

	FileType FsFileType `json:"type"`
}

type FsFileType int

const (
	UnknownFsFile = 0
	FsFile        = 1
	FsDir         = 2
	FsSymLink     = 64
)

func FileTypeFromInfo(i fs.FileInfo) FsFileType {
	mode := i.Mode()

	switch {
	case mode.IsDir():
		return FsDir
	case mode.IsRegular():
		return FsFile
	case mode&fs.ModeSymlink != 0:
		return FsSymLink
	default:
		return UnknownFsFile
	}
}

type FsReadirParams struct {
	DirURI defines.URI `json:"uri"`
}

type FsDirEntry struct {
	Name     string     `json:"name"`
	FileType FsFileType `json:"type"`
}

type FsDirEntries []FsDirEntry
