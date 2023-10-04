package project_server

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
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

type projectEditionState struct {
	lock sync.Mutex

	files map[string]*projectFileState
}

func getCreateProjectEditionState(id core.ProjectID) *projectEditionState {
	projectEditionStatesLock.Lock()
	defer projectEditionStatesLock.Unlock()

	state, ok := projectEditionStates[id]
	if ok {
		return state
	}

	state = &projectEditionState{
		files: map[string]*projectFileState{},
	}
	projectEditionStates[id] = state
	return state
}

func (s *projectEditionState) startFileUpload(fpath string, firstPart []byte, info uploadInfo, session *jsonrpc.Session) (uploadId, error) {
	s.lock.Lock()
	s.cleanupInactiveFilesNoLock("")
	file, ok := s.files[fpath]

	if !ok {
		file = &projectFileState{}
		s.files[fpath] = file
	}
	s.lock.Unlock()
	return file.startFileUpload(session, firstPart, info)
}

func (s *projectEditionState) continueFileUpload(fpath string, part []byte, id uploadId, session *jsonrpc.Session) (uploadInfo, error) {
	s.lock.Lock()
	s.cleanupInactiveFilesNoLock(fpath)
	file, ok := s.files[fpath]

	if !ok {
		file = &projectFileState{}
		s.files[fpath] = file
	}
	s.lock.Unlock()
	return file.continueFileUpload(session, part, id)
}

func (s *projectEditionState) finishFileUpload(fpath string, lastPart []byte, id uploadId, session *jsonrpc.Session) ([][]byte, uploadInfo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.cleanupInactiveFilesNoLock(fpath)
	file, ok := s.files[fpath]

	if !ok {
		file = &projectFileState{}
		s.files[fpath] = file
	}

	parts, info, err := file.finishFileUpload(session, lastPart, id)
	if err == nil {
		delete(s.files, fpath)
		return parts, info, nil
	}

	return nil, uploadInfo{}, err
}

func (s *projectEditionState) cleanupInactiveFilesNoLock(ignoredPath string) {
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

func (s *projectFileState) startFileUpload(session *jsonrpc.Session, firstPart []byte, info uploadInfo) (uploadId, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastActivity.Store(time.Now())

	if s.modifyingSession == nil {
		s.modifyingSession = session
	} else if s.modifyingSession != session {
		return "", ErrFileBeingCreatedOrModifiedByAnotherSession
	} else if info != s.uploadInfo {
		return "", ErrFileBeingCreatedBySameSession
	}

	s.uploadInfo = uploadInfo{
		id:        newUploadId(),
		create:    info.create,
		overwrite: info.overwrite,
	}
	s.uploadParts = append(s.uploadParts, slices.Clone(firstPart))
	return s.uploadInfo.id, nil
}

func (s *projectFileState) continueFileUpload(session *jsonrpc.Session, part []byte, id uploadId) (uploadInfo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastActivity.Store(time.Now())

	if s.modifyingSession != session {
		return uploadInfo{}, ErrFileBeingCreatedOrModifiedByAnotherSession
	}

	if s.uploadInfo.id != id {
		return uploadInfo{}, ErrFileBeingCreatedBySameSession
	}

	s.uploadParts = append(s.uploadParts, slices.Clone(part))
	return s.uploadInfo, nil
}

func (s *projectFileState) finishFileUpload(session *jsonrpc.Session, part []byte, id uploadId) ([][]byte, uploadInfo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastActivity.Store(time.Now())

	if s.modifyingSession != session {
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
