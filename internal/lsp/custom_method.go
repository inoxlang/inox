package internal

import (
	"io/fs"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
)

//get file stat operation

type FsFileStatParams struct {
	FileURI defines.URI `json:"uri"`
}

type FsFileStat struct {
	//The creation timestamp in milliseconds elapsed since January 1, 1970 00:00:00 UTC.
	CreationTime int64 `json:"ctime"`

	//The modification timestamp in milliseconds elapsed since January 1, 1970 00:00:00 UTC.
	ModificationTime int64 `json:"mtime"`

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

//read dir operation

type FsReadirParams struct {
	DirURI defines.URI `json:"uri"`
}

type FsDirEntry struct {
	Name     string     `json:"name"`
	FileType FsFileType `json:"type"`
}

type FsDirEntries []FsDirEntry

//read file operation

type FsReadFileParams struct {
	FileURI defines.URI `json:"uri"`
}

type FsFileContentBase64 struct {
	Content string `json:"content"`
}

//write file operation

type FsWriteFileParams struct {
	FileURI       defines.URI `json:"uri"`
	ContentBase64 string      `json:"content"`
	Create        bool        `json:"create"`
	Overwrite     bool        `json:"overwrite"`
}

//rename file operation

type FsRenameFileParams struct {
	FileURI    defines.URI `json:"uri"`
	NewFileURI defines.URI `json:"newUri"`
	Overwrite  bool        `json:"overwrite"`
}

//basis errors

type FsNonCriticalError string

const (
	FsFileNotFound = "not-found"
	FsFileExists   = "exists"
	FsFileIsDir    = "is-dir"
	FsFileIsNotDir = "is-not-dir"
)
