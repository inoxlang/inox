package projectserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	fs_ns "github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"

	fsutil "github.com/go-git/go-billy/v5/util"
)

const (
	FILE_STAT_METHOD         = "fs/fileStat"
	READ_FILE_METHOD         = "fs/readFile"
	WRITE_FILE_METHOD        = "fs/writeFile"
	START_UPLOAD_METHOD      = "fs/startUpload"
	WRITE_UPLOAD_PART_METHOD = "fs/writeUploadPart"
	RENAME_FILE_METHOD       = "fs/renameFile"
	DELETE_FILE_METHOD       = "fs/deleteFile"

	CREATE_DIR_METHOD = "fs/createDir"
	READ_DIR_METHOD   = "fs/readDir"

	FS_STRUCTURE_EVENT_NOTIF_METHOD = "fs/structureEvent"

	MIN_LAST_CHANGE_AGE_FOR_UNSAVED_DOC_SYNC               = time.Second / 2
	MIN_LAST_FILE_WRITE_AGE_FOR_UNSAVED_DOC_SYNC_REVERSING = time.Second / 2

	FS_EVENT_BATCH_NOTIF_INTERVAL = time.Second
)

var (
	ErrFileBeingCreatedOrModifiedByAnotherSession = errors.New("file is being created or edited by another session")
	ErrFileBeingCreatedBySameSession              = errors.New("file is being created by the same session")
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

	//The modification timestamp in milliseconds elapsed since January 1, 1970 00:00:00 UTC.
	ModificationTime int64 `json:"mtime"`
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

//write upload part operation
//there is no separate method to start an upload in order to start the upload immediately.

type FsStartUploadParams struct {
	Create            bool        `json:"create"`
	Overwrite         bool        `json:"overwrite"`
	FileURI           defines.URI `json:"uri"`
	PartContentBase64 string      `json:"content,omitempty"`
	Last              bool        `json:"last"`
}

type FsStartUploadResponse struct {
	UploadId uploadId `json:"uploadId,omitempty"`
	Done     bool     `json:"done"`
}

type FsWriteUploadPartParams struct {
	UploadId uploadId    `json:"uploadId,omitempty"`
	FileURI  defines.URI `json:"uri"`

	PartContentBase64 string `json:"content,omitempty"`
	Last              bool   `son:"content,last"`
}

//rename file operation

type FsRenameFileParams struct {
	FileURI    defines.URI `json:"uri"`
	NewFileURI defines.URI `json:"newUri"`
	Overwrite  bool        `json:"overwrite"`
}

//delete file operation

type FsDeleteFileParams struct {
	FileURI   defines.URI `json:"uri"`
	Recursive bool        `json:"recursive,omitempty"`
}

//create dir operation

type FsCreateDirParams struct {
	DirURI defines.URI `json:"uri"`
}

//basic errors

type FsNonCriticalError string

const (
	FsFileNotFound FsNonCriticalError = "not-found"
	FsFileExists   FsNonCriticalError = "exists"
	FsFileIsDir    FsNonCriticalError = "is-dir"
	FsFileIsNotDir FsNonCriticalError = "is-not-dir"
	FsNoFilesystem FsNonCriticalError = "no-filesystem"
)

//events

type FsStructureEvent struct {
	Path     string    `json:"path"`
	CreateOp bool      `json:"createOp,omitempty"`
	RemoveOp bool      `json:"removeOp,omitempty"`
	ChmodOp  bool      `json:"chmodOp,omitempty"`
	RenameOp bool      `json:"renameOp,omitempty"`
	DateTime time.Time `json:"datetime,omitempty"`
}

func registerFilesystemMethodHandlers(server *lsp.Server) {
	server.OnCustom(jsonrpc.MethodInfo{
		Name: FILE_STAT_METHOD,
		NewRequest: func() interface{} {
			return &FsFileStatParams{}
		},
		RateLimits: []int{30, 100, 300},
		Handler:    handleFileStat,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: READ_FILE_METHOD,
		NewRequest: func() interface{} {
			return &FsReadFileParams{}
		},
		SensitiveData: true,
		RateLimits:    []int{20, 100, 300},
		Handler:       handleReadFile,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: WRITE_FILE_METHOD,
		NewRequest: func() interface{} {
			return &FsWriteFileParams{}
		},
		SensitiveData: true,
		RateLimits:    []int{20, 100, 300},
		Handler:       handleWriteFile,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name:       START_UPLOAD_METHOD,
		RateLimits: []int{5, 20, 50},
		NewRequest: func() interface{} {
			return &FsStartUploadParams{}
		},
		SensitiveData: true,
		Handler:       handleStartFileUpload,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name:       WRITE_UPLOAD_PART_METHOD,
		RateLimits: []int{10, 100, 300},
		NewRequest: func() interface{} {
			return &FsWriteUploadPartParams{}
		},
		SensitiveData: true,
		Handler:       writeFileUploadPart,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: RENAME_FILE_METHOD,
		NewRequest: func() interface{} {
			return &FsRenameFileParams{}
		},
		Handler: handleRenameFile,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: DELETE_FILE_METHOD,
		NewRequest: func() interface{} {
			return &FsDeleteFileParams{}
		},
		RateLimits: []int{20, 100, 500},
		Handler:    handleDeleteFile,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: READ_DIR_METHOD,
		NewRequest: func() interface{} {
			return &FsReadirParams{}
		},
		RateLimits: []int{20, 100, 500},
		Handler:    handleReadDir,
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: CREATE_DIR_METHOD,
		NewRequest: func() interface{} {
			return &FsCreateDirParams{}
		},
		Handler: handleCreateDir,
	})
}

func getLspFilesystem(session *jsonrpc.Session) (*Filesystem, bool) {
	sessionData := getLockedSessionData(session)
	defer sessionData.lock.Unlock()

	return sessionData.filesystem, sessionData.filesystem != nil
}

// unsavedDocumentSyncData contains data about the synchronization of an unsaved document.
// The handler of WRITE_FILE_METHOD always attempts to update the unsaved document with the new content of the file.
type unsavedDocumentSyncData struct {
	lock sync.Mutex

	path          string
	prevContent   []byte
	reversed      bool
	lastFileWrite time.Time
	lastDidChange time.Time
}

// reactToDidChange updates the unsavedDocumentSyncData & reverse the last synchronization
// if it happened too recently (likely during a sequence of LSP changes).
func (d *unsavedDocumentSyncData) reactToDidChange(fls *Filesystem) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer func() {
		d.lastDidChange = time.Now()
	}()

	unsavedDocFS := fls.unsavedDocumentsFS()
	if unsavedDocFS == fls {
		return
	}

	if time.Since(d.lastFileWrite) < MIN_LAST_FILE_WRITE_AGE_FOR_UNSAVED_DOC_SYNC_REVERSING && !d.reversed {
		d.reversed = true
		logs.Println("reverse 'unsaved' doc synchronization")

		f, err := unsavedDocFS.Open(d.path)
		if err != nil {
			return
		}
		defer f.Close()
		f.Truncate(0)
		f.Seek(0, 0)
		f.Write(d.prevContent)
	}
}

func updateFile(fpath string, contentParts [][]byte, create, overwrite bool, fls *Filesystem, session *jsonrpc.Session) (FsNonCriticalError, error) {
	unsavedDocumentsFS := fls.unsavedDocumentsFS()

	// attempt to synchronize the unsaved document with the new content,
	// we only do that if the unsaved document filesystem is a separate filesystem.
	if fls != unsavedDocumentsFS && false {
		func() {
			//get & read the synchronization data
			sessionData := getLockedSessionData(session)
			syncData := sessionData.unsavedDocumentSyncData[fpath]
			if syncData == nil {
				syncData = &unsavedDocumentSyncData{path: fpath}
				sessionData.unsavedDocumentSyncData[fpath] = syncData
			}
			sessionData.lock.Unlock()

			updated := false
			defer func() {
				if updated {
					//we log after to reduce the time spent locked
					logs.Printf("'unsaved' document %q was updated with the content of the persisted file\n", fpath)
				}
			}()

			syncData.lock.Lock()
			defer syncData.lock.Unlock()

			if time.Since(syncData.lastDidChange) < MIN_LAST_CHANGE_AGE_FOR_UNSAVED_DOC_SYNC {
				return
			}

			// read the file & save the previous content
			doc, err := unsavedDocumentsFS.OpenFile(fpath, os.O_RDWR, 0)
			if err != nil {
				return
			}

			closed := false

			defer func() {
				if !closed {
					doc.Close()
				}
			}()

			prevContent, err := io.ReadAll(doc)
			if err != nil {
				return
			}

			syncData.prevContent = prevContent
			syncData.lastFileWrite = time.Now()

			//write the new content
			doc.Truncate(0)
			doc.Seek(0, 0)

			for _, part := range contentParts {
				_, _ = doc.Write(part)
			}

			closed = true
			updated = true
			doc.Close()
		}()
	}

	if create {
		f, err := fls.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, fs_ns.DEFAULT_FILE_FMODE)

		defer func() {
			if f != nil {
				f.Close()
			}
		}()

		if err != nil && !os.IsNotExist(err) {
			return "", fmtInternalError("failed to create file %s: %s", fpath, err)
		}

		alreadyExists := err == nil
		if alreadyExists {
			if !overwrite {
				return "", fmtInternalError("failed to create file %s: already exists and overwrite option is false", fpath)
			}

			if err := f.Truncate(int64(len(contentParts))); err != nil {
				return "", fmtInternalError("failed to truncate file before write %s: %s", fpath, err)
			}
		}

		for _, part := range contentParts {
			_, err = f.Write(part)
		}

		if err != nil {
			return "", fmt.Errorf("failed to create file %s: failed to write: %w", fpath, err)
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
			return "", fmtInternalError("failed to write file %s: failed to open: %s", fpath, err)
		}

		if err := f.Truncate(int64(len(contentParts))); err != nil {
			return "nil", fmtInternalError("failed to truncate file before write: %s: %s", fpath, err)
		}

		for _, part := range contentParts {
			_, err = f.Write(part)
		}

		if err != nil {
			return "", fmtInternalError("failed to create file %s: failed to write: %s", fpath, err)
		}
	}

	return "", nil
}

func startNotifyingFilesystemStructureEvents(session *jsonrpc.Session, fls afs.Filesystem) error {
	sessionCtx := session.Context()
	evs, err := fs_ns.NewEventSourceWithFilesystem(sessionCtx, fls, core.PathPattern("/..."))
	if err != nil {
		return fmt.Errorf("failed to create filesystem event source: %s", err)
	}

	sessionCtx.OnDone(func(timeoutCtx context.Context, teardownStatus core.GracefulTeardownStatus) error {
		evs.Close()
		return nil
	})

	evs.OnEvent(func(event *core.Event) {
		defer func() {
			e := recover()
			if e != nil {
				err := utils.ConvertPanicValueToError(err)
				err = fmt.Errorf("%w: %s", err, debug.Stack())
				logs.Println(err)
			}
		}()

		fsEvent := event.SourceValue().(fs_ns.Event)
		if !fsEvent.IsStructureChange() {
			return
		}

		//notify event

		ev := FsStructureEvent{
			Path:     fsEvent.Path().UnderlyingString(),
			CreateOp: fsEvent.HasCreateOp(),
			RemoveOp: fsEvent.HasRemoveOp(),
			ChmodOp:  fsEvent.HasChmodOp(),
			RenameOp: fsEvent.HasRenameOp(),
			DateTime: time.Time(fsEvent.Time()),
		}

		session.Notify(jsonrpc.NotificationMessage{
			Method: FS_STRUCTURE_EVENT_NOTIF_METHOD,
			Params: utils.Must(json.Marshal(ev)),
		})
	})

	return nil
}

func handleFileStat(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsFileStatParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
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
		return nil, fmtInternalError("failed to get stat for file %s: %s", fpath, err)
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
}

func handleReadFile(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsReadFileParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
	}

	fpath, err := getPath(params.FileURI, true)
	if err != nil {
		return nil, err
	}

	content, err := fsutil.ReadFile(fls, fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return FsFileNotFound, nil
		}
		return nil, fmtInternalError("failed to read file %s: %s", fpath, err)
	}

	return FsFileContentBase64{Content: base64.StdEncoding.EncodeToString(content)}, nil
}

func handleWriteFile(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsWriteFileParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
	}

	fpath, err := getPath(params.FileURI, true)
	if err != nil {
		return nil, err
	}

	content, err := base64.StdEncoding.DecodeString(string(params.ContentBase64))
	if err != nil {
		return nil, fmt.Errorf("failed to decode received content for file %s: %w", fpath, err)
	}

	return updateFile(fpath, [][]byte{content}, params.Create, params.Overwrite, fls, session)
}

func handleStartFileUpload(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsStartUploadParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
	}

	data := getLockedSessionData(session)
	var projectId core.ProjectID
	if data.project != nil {
		projectId = data.project.Id()
	}
	data.lock.Unlock()

	if projectId == "" {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "the method " + WRITE_UPLOAD_PART_METHOD + " is only supported in project mode for now",
		}
	}

	fpath, err := getPath(params.FileURI, true)
	if err != nil {
		return nil, err
	}

	editionState := getCreateProjectEditionState(projectId)

	firstPart, err := base64.StdEncoding.DecodeString(string(params.PartContentBase64))
	if err != nil {
		return nil, fmt.Errorf("failed to decode received content for the first part of the file %s: %w", fpath, err)
	}

	info := uploadInfo{create: params.Create, overwrite: params.Overwrite}
	uploadId, err := editionState.startFileUpload(fpath, firstPart, info, session)

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	if params.Last {
		parts, info, err := editionState.finishFileUpload(fpath, nil, uploadId, session)

		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}

		nonCritialErr, err := updateFile(fpath, parts, info.create, info.overwrite, fls, session)

		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}

		if nonCritialErr == "" {
			return FsStartUploadResponse{
				UploadId: uploadId,
				Done:     true,
			}, nil
		}

		return nonCritialErr, nil
	}

	return FsStartUploadResponse{
		UploadId: uploadId,
	}, nil
}

func writeFileUploadPart(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsWriteUploadPartParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
	}

	data := getLockedSessionData(session)
	var projectId core.ProjectID
	if data.project != nil {
		projectId = data.project.Id()
	}
	data.lock.Unlock()

	if projectId == "" {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "the method " + WRITE_UPLOAD_PART_METHOD + " is only supported in project mode for now",
		}
	}

	fpath, err := getPath(params.FileURI, true)
	if err != nil {
		return nil, err
	}

	editionState := getCreateProjectEditionState(projectId)

	part, err := base64.StdEncoding.DecodeString(string(params.PartContentBase64))
	if err != nil {
		return nil, fmt.Errorf("failed to decode received content for a part of the file %s: %w", fpath, err)
	}

	if params.Last {
		parts, info, err := editionState.finishFileUpload(fpath, part, params.UploadId, session)

		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}

		return updateFile(fpath, parts, info.create, info.overwrite, fls, session)
	} else {
		_, err := editionState.continueFileUpload(fpath, part, params.UploadId, session)

		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: err.Error(),
			}
		}
	}

	return nil, nil
}

func handleRenameFile(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsRenameFileParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
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
		err := fls.Rename(path, newPath)
		if err != nil {
			return nil, fmtInternalError(err.Error())
		}
		return nil, nil
	} else { //exists
		if params.Overwrite {
			if err == nil && newPathStat.IsDir() {
				if err := fls.Remove(newPath); err != nil {
					return nil, fmtInternalError("failed to rename %s to %s: deletion of found dir failed: %s", path, newPath, err)
				}
			}

			//TODO: return is-dir error if there is a directory.
			return nil, fls.Rename(path, newPath)
		}
		return nil, fmtInternalError("failed to rename %s to %s: file or dir found at new path and overwrite option is false ", path, newPath)
	}
}

func handleDeleteFile(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsDeleteFileParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
	}

	path, err := getPath(params.FileURI, true)
	if err != nil {
		return nil, err
	}

	if params.Recursive {
		//TODO: add implementation of the { RemoveAll(string) error } interface to MetaFilesystem & MemoryFilesystem.
		err = fsutil.RemoveAll(fls, path)

		if os.IsNotExist(err) {
			return FsFileNotFound, nil
		} else if err != nil { //exists
			return nil, fmtInternalError("failed to recursively delete %s: %s", path, err)
		}
	} else {
		err = fls.Remove(path)

		if os.IsNotExist(err) {
			return FsFileNotFound, nil
		} else if err != nil { //exists
			return nil, fmtInternalError("failed to delete %s: %s", path, err)
		}
	}

	return nil, nil
}

func handleReadDir(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsReadirParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
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
		return nil, fmtInternalError("failed to read dir %s", dpath)
	}

	fsDirEntries := FsDirEntries{}
	for _, e := range entries {
		fsDirEntries = append(fsDirEntries, FsDirEntry{
			Name:             e.Name(),
			FileType:         FileTypeFromInfo(e),
			ModificationTime: e.ModTime().UnixMilli(),
		})
	}

	return fsDirEntries, nil
}

func handleCreateDir(ctx context.Context, req interface{}) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	params := req.(*FsCreateDirParams)
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil, errors.New(string(FsNoFilesystem))
	}

	path, err := getPath(params.DirURI, true)
	if err != nil {
		return nil, err
	}

	err = fls.MkdirAll(path, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return nil, fmtInternalError("failed to create dir %s: %s", path, err)
	}

	return nil, nil
}
