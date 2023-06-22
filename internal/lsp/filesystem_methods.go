package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	fs_ns "github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/lsp"
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	fsutil "github.com/go-git/go-billy/v5/util"
)

var (
	sessionToFilesystem     = map[*jsonrpc.Session]*Filesystem{}
	sessionToFilesystemLock sync.RWMutex
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

//delete file operation

type FsDeleteFileParams struct {
	FileURI defines.URI `json:"uri"`
}

//create dir operation

type FsCreateDirParams struct {
	DirURI defines.URI `json:"uri"`
}

//basic errors

type FsNonCriticalError string

const (
	FsFileNotFound = "not-found"
	FsFileExists   = "exists"
	FsFileIsDir    = "is-dir"
	FsFileIsNotDir = "is-not-dir"
	FsNoFilesystem = "no-filesystem"
)

func registerFilesystemMethodHandlers(server *lsp.Server) {
	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/fileStat",
		NewRequest: func() interface{} {
			return &FsFileStatParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsFileStatParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			fpath, err := getPath(params.FileURI, true)
			if err != nil {
				return nil, err
			}

			stat, err := fls.Stat(fpath)
			if err != nil {
				if os.IsNotExist(err) {
					return FsFileNotFound, nil
				}
				return nil, fmt.Errorf("failed to get stat for file %s: %w", fpath, err)
			}

			ctime, mtime, err := fs_ns.GetCreationAndModifTime(stat)
			if err != nil {
				return nil, fmt.Errorf("failed to get the creation/modification time for file %s", fpath)
			}

			return &FsFileStat{
				CreationTime:     ctime.UnixMilli(),
				ModificationTime: mtime.UnixMilli(),
				Size:             stat.Size(),
				FileType:         FileTypeFromInfo(stat),
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/readFile",
		NewRequest: func() interface{} {
			return &FsReadFileParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsReadFileParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			fpath, err := getPath(params.FileURI, true)
			if err != nil {
				if os.IsNotExist(err) {
					return FsFileNotFound, nil
				}
				return nil, err
			}

			content, err := fsutil.ReadFile(fls, fpath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", fpath, err)
			}

			return FsFileContentBase64{Content: base64.StdEncoding.EncodeToString(content)}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/writeFile",
		NewRequest: func() interface{} {
			return &FsWriteFileParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsWriteFileParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			fpath, err := getPath(params.FileURI, true)
			if err != nil {
				return nil, err
			}

			content, err := base64.StdEncoding.DecodeString(string(params.ContentBase64))
			if err != nil {
				return nil, fmt.Errorf("failed to decode received content for file %s: %w", fpath, err)
			}

			if params.Create {
				f, err := fls.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, fs_ns.DEFAULT_FILE_FMODE)

				defer func() {
					if f != nil {
						f.Close()
					}
				}()

				if err != nil && !os.IsNotExist(err) {
					return nil, fmt.Errorf("failed to create file %s: %w", fpath, err)
				}

				alreadyExists := err == nil

				if alreadyExists {
					if !params.Overwrite {
						return nil, fmt.Errorf("failed to create file %s: already exists and overwrite option is false", fpath)
					}

					if err := f.Truncate(int64(len(content))); err != nil {
						return nil, fmt.Errorf("failed to truncate file before write %s: %w", fpath, err)
					}
				}

				_, err = f.Write(content)

				if err != nil {
					return nil, fmt.Errorf("failed to create file %s: failed to write: %w", fpath, err)
				}
			} else {
				f, err := fls.OpenFile(fpath, os.O_WRONLY, 0)

				defer func() {
					if f != nil {
						f.Close()
					}
				}()

				if os.IsNotExist(err) {
					return FsFileNotFound, nil
				} else if err != nil {
					return nil, fmt.Errorf("failed to write file %s: failed to open: %w", fpath, err)
				}

				if err := f.Truncate(int64(len(content))); err != nil {
					return nil, fmt.Errorf("failed to truncate file before write: %s: %w", fpath, err)
				}

				_, err = f.Write(content)

				if err != nil {
					return nil, fmt.Errorf("failed to create file %s: failed to write: %w", fpath, err)
				}
			}

			return nil, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/renameFile",
		NewRequest: func() interface{} {
			return &FsRenameFileParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsRenameFileParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			path, err := getPath(params.FileURI, true)
			if err != nil {
				return nil, err
			}

			newPath, err := getPath(params.NewFileURI, true)
			if err != nil {
				return nil, err
			}

			_, err = fls.Stat(path)
			if os.IsNotExist(err) {
				return FsFileNotFound, nil
			}

			newPathStat, err := fls.Stat(newPath)

			if os.IsNotExist(err) {
				//there is no file at the desination path so we can rename it.
				return nil, fls.Rename(path, newPath)
			} else { //exists
				if params.Overwrite {
					if err == nil && newPathStat.IsDir() {
						if err := fls.Remove(newPath); err != nil {
							return nil, fmt.Errorf("failed to rename %s to %s: deletion of found dir failed: %w", path, newPath, err)
						}
					}

					//TODO: return is-dir error if there is a directory.
					return nil, fls.Rename(path, newPath)
				}
				return nil, fmt.Errorf("failed to rename %s to %s: file or dir found at new path and overwrite option is false ", path, newPath)
			}
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/deleteFile",
		NewRequest: func() interface{} {
			return &FsDeleteFileParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsDeleteFileParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			path, err := getPath(params.FileURI, true)
			if err != nil {
				return nil, err
			}

			err = fls.Remove(path)

			if os.IsNotExist(err) {
				return FsFileNotFound, nil
			} else if err != nil { //exists
				return nil, fmt.Errorf("failed to delete %s: %w", path, err)
			}

			return nil, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/readDir",
		NewRequest: func() interface{} {
			return &FsReadirParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsReadirParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			dpath, err := getPath(params.DirURI, true)
			if err != nil {
				return nil, err
			}

			entries, err := fls.ReadDir(dpath)
			if err != nil {
				if os.IsNotExist(err) {
					return FsFileNotFound, nil
				}
				return nil, fmt.Errorf("failed to read dir %s", dpath)
			}

			fsDirEntries := FsDirEntries{}
			for _, e := range entries {
				fsDirEntries = append(fsDirEntries, FsDirEntry{
					Name:     e.Name(),
					FileType: FileTypeFromInfo(e),
				})
			}

			return fsDirEntries, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "fs/createDir",
		NewRequest: func() interface{} {
			return &FsCreateDirParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*FsCreateDirParams)
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			path, err := getPath(params.DirURI, true)
			if err != nil {
				return nil, err
			}

			err = fls.MkdirAll(path, fs_ns.DEFAULT_DIR_FMODE)
			if err != nil {
				return nil, err
			}

			return nil, nil
		},
	})
}

func getLspFilesystem(session *jsonrpc.Session) (*Filesystem, bool) {
	sessionToFilesystemLock.RLock()
	defer sessionToFilesystemLock.RUnlock()

	fls, ok := sessionToFilesystem[session]
	return fls, ok
}
