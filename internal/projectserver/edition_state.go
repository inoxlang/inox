package projectserver

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/oklog/ulid/v2"
)

const (
	//we set the timeout to a small value so that:
	// - inactive file states are garbage collected quickly
	// - users do not have too wait long to retry an upload if they encountered an error
	PROJECT_FILE_STATE_CLEANUP_TIMEOUT = 5 * time.Second
)

var (
	projectEditionStates     = map[core.ProjectID]*projectEditionState{}
	projectEditionStatesLock sync.Mutex
)

// There is one projectEditionState per open project. For now the primary use of projectEditionState
// is enabling multipart file upload. Parallel editing will be implemented using this struct by
// tracking the state of all the files being edited in a project.
type projectEditionState struct {
	lock sync.Mutex

	files map[absoluteFilePath]*projectFileState
}

func getCreateProjectEditionState(id core.ProjectID) *projectEditionState {
	projectEditionStatesLock.Lock()
	defer projectEditionStatesLock.Unlock()

	state, ok := projectEditionStates[id]
	if ok {
		return state
	}

	state = &projectEditionState{
		files: map[absoluteFilePath]*projectFileState{},
	}
	projectEditionStates[id] = state
	return state
}

func (s *projectEditionState) startFileUpload(fpath absoluteFilePath, firstPart []byte, info uploadInfo, rpcSession *jsonrpc.Session) (uploadId, error) {
	s.lock.Lock()
	s.cleanupInactiveFilesNoLock("")
	file, ok := s.files[fpath]

	if !ok {
		file = &projectFileState{}
		s.files[fpath] = file
	}
	s.lock.Unlock()
	return file.startFileUpload(rpcSession, firstPart, info)
}

func (s *projectEditionState) continueFileUpload(fpath absoluteFilePath, part []byte, id uploadId, rpcSession *jsonrpc.Session) (uploadInfo, error) {
	s.lock.Lock()
	s.cleanupInactiveFilesNoLock(fpath)
	file, ok := s.files[fpath]

	if !ok {
		file = &projectFileState{}
		s.files[fpath] = file
	}
	s.lock.Unlock()
	return file.continueFileUpload(rpcSession, part, id)
}

func (s *projectEditionState) finishFileUpload(fpath absoluteFilePath, lastPart []byte, id uploadId, rpcSession *jsonrpc.Session) ([][]byte, uploadInfo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.cleanupInactiveFilesNoLock(fpath)
	file, ok := s.files[fpath]

	if !ok {
		file = &projectFileState{}
		s.files[fpath] = file
	}

	parts, info, err := file.finishFileUpload(rpcSession, lastPart, id)
	if err == nil {
		delete(s.files, fpath)
		return parts, info, nil
	}

	return nil, uploadInfo{}, err
}

func (s *projectEditionState) cleanupInactiveFilesNoLock(ignoredPath absoluteFilePath) {
	for path, file := range s.files {
		if (path == "" || path != ignoredPath) && time.Since(file.LastActivity()) >= PROJECT_FILE_STATE_CLEANUP_TIMEOUT {
			delete(s.files, path)
		}
	}
}

type uploadId string

func newUploadId() uploadId {
	return uploadId(ulid.Make().String())
}

type projectFileState struct {
	lock sync.Mutex

	modifyingSession *jsonrpc.Session
	uploadInfo       uploadInfo

	uploadParts [][]byte

	lastActivity atomic.Value //time.Time
}

type uploadInfo struct {
	id        uploadId //should never be set if the uploadInfo is passed as an argument
	create    bool
	overwrite bool
}

func (s *projectFileState) LastActivity() time.Time {
	n := s.lastActivity.Load()
	if n == nil {
		return time.Time{}
	}

	t := n.(time.Time)
	return t
}

func (s *projectFileState) startFileUpload(rpcSession *jsonrpc.Session, firstPart []byte, info uploadInfo) (uploadId, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastActivity.Store(time.Now())

	if s.modifyingSession == nil {
		s.modifyingSession = rpcSession
	} else if s.modifyingSession != rpcSession {
		return "", ErrFileBeingCreatedOrModifiedByAnotherSession
	} else {
		return "", ErrFileBeingCreatedBySameSession
	}

	s.uploadInfo = uploadInfo{
		id:        newUploadId(),
		create:    info.create,
		overwrite: info.overwrite,
	}
	s.uploadParts = append(s.uploadParts, slices.Clone(firstPart))
	s.modifyingSession = rpcSession
	return s.uploadInfo.id, nil
}

func (s *projectFileState) continueFileUpload(rpcSession *jsonrpc.Session, part []byte, id uploadId) (uploadInfo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastActivity.Store(time.Now())

	if s.modifyingSession != rpcSession {
		return uploadInfo{}, ErrFileBeingCreatedOrModifiedByAnotherSession
	}

	if s.uploadInfo.id != id {
		return uploadInfo{}, ErrFileBeingCreatedBySameSession
	}

	s.uploadParts = append(s.uploadParts, slices.Clone(part))
	return s.uploadInfo, nil
}

func (s *projectFileState) finishFileUpload(rpcSession *jsonrpc.Session, part []byte, id uploadId) ([][]byte, uploadInfo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastActivity.Store(time.Now())

	if s.modifyingSession != rpcSession {
		return nil, uploadInfo{}, ErrFileBeingCreatedOrModifiedByAnotherSession
	}

	if s.uploadInfo.id != id {
		return nil, uploadInfo{}, ErrFileBeingCreatedBySameSession
	}

	s.uploadParts = append(s.uploadParts, slices.Clone(part))

	parts := s.uploadParts
	info := s.uploadInfo

	s.uploadParts = nil
	s.modifyingSession = nil
	s.uploadInfo = uploadInfo{}

	return parts, info, nil
}
