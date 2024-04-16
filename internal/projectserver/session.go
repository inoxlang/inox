package projectserver

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/codebase/gen"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/devtools"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/sourcecontrol"
	"github.com/rs/zerolog"
)

var (
	sessions     = make(map[*jsonrpc.Session]*Session)
	sessionsLock sync.Mutex
)

// getCreateProjectSession retrieves or creates a *Session. When getCreateProjectSession
// creates a *Session it only sets its rpcSession field and map fields. Other fields
// are initialized by the LSP handler for 'project/open'.
func getCreateProjectSession(rpcSession *jsonrpc.Session) *Session {
	sessionsLock.Lock()
	session := sessions[rpcSession]
	if session == nil {
		session = &Session{
			rpcSession:                          rpcSession,
			didSaveCapabilityRegistrationIds:    make(map[defines.DocumentUri]uuid.UUID, 0),
			unsavedDocumentSyncData:             make(map[string]*unsavedDocumentSyncData, 0),
			testRuns:                            make(map[TestRunId]*TestRun, 0),
			documentDiagnostics:                 make(map[string]*documentDiagnostics),
			lastDiagnosticComputationStartTimes: make(map[defines.DocumentUri]time.Time),
		}
		sessions[rpcSession] = session
	}

	sessionsLock.Unlock()
	return session
}

func getCreateLockedProjectSession(rpcSession *jsonrpc.Session) *Session {
	session := getCreateProjectSession(rpcSession)
	session.lock.Lock()
	return session
}

// A Session represents the state of a development session on the project server, it also holds references to major
// components such as a devtools instance, a Git repository, and the open project. During a LSP session, method
// handlers retrieve and store data from/to a *Session and interact with the referenced components.
type Session struct {
	lock            sync.RWMutex
	removed         atomic.Bool
	memberAuthToken string
	devSessionKey   http_ns.DevSessionKey //set after project is open

	//LSP

	rpcSession                       *jsonrpc.Session
	clientCapabilities               defines.ClientCapabilities
	serverCapabilities               defines.ServerCapabilities
	didSaveCapabilityRegistrationIds map[defines.DocumentUri]uuid.UUID
	unsavedDocumentSyncData          map[string] /* fpath */ *unsavedDocumentSyncData

	//Project

	inProjectMode bool
	project       *project.Project

	//Working copy

	filesystem      *Filesystem
	fsEventSource   *fs_ns.FilesystemEventSource
	inoxChunkCache  *parse.ChunkCache
	stylesheetCache *css.StylesheetCache

	//Analysis and diagnostics

	preparedSourceFilesCache            *preparedFileCache
	lastCodebaseAnalysis                *analysis.Result
	postEditDiagnosticDebounce          func(f func()) //Used to debounce the computation of diagnostics after the user stops making edits.
	documentDiagnostics                 map[ /*absolute path */ string]*documentDiagnostics
	lastDiagnosticComputationStartTimes map[defines.DocumentUri]time.Time //used to ignore some diagnostic pulls, protected by .lock

	//Automated code generation

	cssGenerator *gen.CssGenerator
	jsGenerator  *gen.JsGenerator

	//Dev tools

	devtools *devtools.Instance

	//Testing

	testRuns map[TestRunId]*TestRun

	//Debug adapter protocol - https://microsoft.github.io/debug-adapter-protocol

	debugSessions *DebugSessions

	//Source control

	repository     *sourcecontrol.GitRepository //Git repository on the project server.
	repositoryLock sync.Mutex

	//Server-side HTTP client

	secureHttpClient   *http.Client
	insecureHttpClient *http.Client //used for requests to localhost
}

func (s *Session) Scheme() string {
	if s.inProjectMode {
		return INOX_FS_SCHEME
	}
	return "file"
}

// Logger returns the logger of the RPC session.
func (d *Session) Logger() zerolog.Logger {
	return d.rpcSession.Logger()
}

func (s *Session) remove(_ *jsonrpc.Session) {
	if !s.removed.CompareAndSwap(false, true) {
		return
	}
	logger := s.Logger()

	logger.Println("remove one session that has just finished: " + s.rpcSession.Client())
	sessionsLock.Lock()
	session, ok := sessions[s.rpcSession]
	delete(sessions, s.rpcSession)
	sessionsLock.Unlock()

	if ok {
		func() {
			session.lock.Lock()
			defer session.lock.Unlock()
			session.preparedSourceFilesCache.acknowledgeSessionEnd()
			session.preparedSourceFilesCache = nil
		}()
	}
}

func removeClosedSessions(serverLogger zerolog.Logger) {
	//remove closed sessions
	sessionsLock.Lock()
	for s, session := range sessions {
		sessionToRemove := s
		if sessionToRemove.Closed() {
			serverLogger.Println("remove one closed session: " + s.Client())
			delete(sessions, sessionToRemove)
			func() {
				session.lock.Lock()
				defer session.lock.Unlock()
				if session.preparedSourceFilesCache != nil {
					session.preparedSourceFilesCache.acknowledgeSessionEnd()
					session.preparedSourceFilesCache = nil
				}
			}()
		}
	}
	newCount := len(sessions)
	sessionsLock.Unlock()
	serverLogger.Println("current session count:", newCount)
}
