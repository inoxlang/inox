package internal

import (
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
)

type FsFileStatParams struct {
	FileURI defines.URI `json:"uri"`
}

type FsFileStat struct {
	CTime    uint64     `json:"ctime"`
	MTime    uint64     `json:"mtime"`
	Size     uint64     `json:"size"`
	FileType FsFileType `json:"type"`
}

type FsFileType int

const (
	UnknownFsFile = 0
	FsFile        = 1
	FsDir         = 2
	FsSymLink     = 64
)

type FsReadirParams struct {
	DirURI defines.URI `json:"uri"`
}

type FsDirEntry struct {
	Name     string     `json:"name"`
	FileType FsFileType `json:"type"`
}

type FsDirEntries []FsDirEntry
