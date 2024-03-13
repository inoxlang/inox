package projectserver

import (
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/codebase/gen"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/dev"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/sourcecontrol"
)

type additionalSessionData struct {
	lock sync.RWMutex

	didSaveCapabilityRegistrationIds map[defines.DocumentUri]uuid.UUID

	unsavedDocumentSyncData  map[string] /* fpath */ *unsavedDocumentSyncData
	preparedSourceFilesCache *preparedFileCache

	filesystem           *Filesystem
	repository           *sourcecontrol.GitRepository //Git repository on the project server.
	repositoryLock       sync.Mutex
	clientCapabilities   defines.ClientCapabilities
	serverCapabilities   defines.ServerCapabilities
	projectMode          bool
	project              *project.Project
	memberAuthToken      string
	projectDevSessionKey http_ns.DevSessionKey //set after project is open
	devSession           *dev.Session

	serverAPI                  *serverAPI //set during project opening
	lastCodebaseAnalysis       *analysis.Result
	cssGenerator               *gen.CssGenerator
	jsGenerator                *gen.JsGenerator
	fsEventSource              *fs_ns.FilesystemEventSource
	postEditDiagnosticDebounce func(f func()) //Used to debounce the computation of diagnostics after the user stops making edits.
	documentDiagnostics        map[ /*absolute path */ string]*documentDiagnostics

	//testing
	testRuns map[TestRunId]*TestRun

	//debug adapter protocol
	debugSessions *DebugSessions

	//server-side HTTP client
	secureHttpClient   *http.Client
	insecureHttpClient *http.Client //used for requests to localhost
}

func (d *additionalSessionData) Scheme() string {
	if d.projectMode {
		return INOX_FS_SCHEME
	}
	return "file"
}

func getLockedSessionData(session *jsonrpc.Session) *additionalSessionData {
	sessionData := getSessionData(session)
	sessionData.lock.Lock()
	return sessionData
}

func getSessionData(session *jsonrpc.Session) *additionalSessionData {
	sessionToAdditionalDataLock.Lock()
	sessionData := sessionToAdditionalData[session]
	if sessionData == nil {
		sessionData = &additionalSessionData{
			didSaveCapabilityRegistrationIds: make(map[defines.DocumentUri]uuid.UUID, 0),
			unsavedDocumentSyncData:          make(map[string]*unsavedDocumentSyncData, 0),
			testRuns:                         make(map[TestRunId]*TestRun, 0),
			documentDiagnostics:              make(map[string]*documentDiagnostics),
		}
		sessionToAdditionalData[session] = sessionData
	}

	sessionToAdditionalDataLock.Unlock()
	return sessionData
}
