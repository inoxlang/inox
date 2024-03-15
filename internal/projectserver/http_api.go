package projectserver

import (
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	httpspec "github.com/inoxlang/inox/internal/globals/http_ns/spec"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	SERVER_API_UPDATE_DEBOUNCE_DURATION = time.Second / 2
)

// serverAPI stores the API of a FS-routing HTTP Server. It is primarily used for code completion.
// It is not shared between LSP sessions.
type serverAPI struct {
	lock     sync.Mutex
	debounce func(f func())

	api        *httpspec.API
	dynamicDir string
	appModPath string

	fls             *Filesystem
	rpcSession      *jsonrpc.Session
	project         *project.Project
	memberAuthToken string
}

func newServerAPI(project *project.Project, fls *Filesystem, rpcSession *jsonrpc.Session, memberAuthToken string) *serverAPI {
	api := &serverAPI{
		dynamicDir:      "/routes",
		appModPath:      "/main.ix",
		fls:             fls,
		rpcSession:      rpcSession,
		project:         project,
		memberAuthToken: memberAuthToken,
	}
	api.dynamicDir = core.AppendTrailingSlashIfNotPresent(api.dynamicDir)
	api.debounce = debounce.New(SERVER_API_UPDATE_DEBOUNCE_DURATION)

	return api
}

func (a *serverAPI) API() *httpspec.API {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.api
}

func (a *serverAPI) isInoxFileInHandlerModulesDir(fpath string) bool {
	return strings.HasSuffix(fpath, inoxconsts.INOXLANG_FILE_EXTENSION) && strings.HasPrefix(fpath, a.dynamicDir)
}

func (a *serverAPI) acknowledgeSourceFileChange(fpath string, prevContent string, events []defines.TextDocumentContentChangeEvent) {
	if !a.isInoxFileInHandlerModulesDir(fpath) {
		return
	}

	//Ignore the changes if the file is not a module or if the change is located after the manifest.

	firstChangeLine := int32(math.MaxInt32 / 10)

	for _, event := range events {
		line, _ := getLineColumn(event.Range.Start)
		if line < firstChangeLine {
			firstChangeLine = line
		}
	}

	chunk, err := parse.ParseChunk(prevContent, fpath, parse.ParserOptions{Start: true})
	if err != nil {
		return
	}

	if chunk.IncludableChunkDesc != nil || chunk.Manifest == nil {
		return
	}

	//Determine if the first change is after the manifest.
	manifestEndPos := chunk.Manifest.Base().Span.End
	lastManifestLine := 1

	for _, token := range chunk.Tokens {
		if token.Type == parse.NEWLINE && token.Span.Start < manifestEndPos {
			lastManifestLine++
		}
	}

	if lastManifestLine < int(firstChangeLine) {
		return
	}

	a.debounce(func() {
		defer utils.Recover()
		a.tryUpdateAPI()
	})
}

func (a *serverAPI) acknowledgeStructureChangeEvent(event fs_ns.Event) {
	path := event.Path().UnderlyingString()

	if !a.isInoxFileInHandlerModulesDir(path) {
		return
	}

	a.debounce(func() {
		defer utils.Recover()
		a.tryUpdateAPI()
	})
}

func (a *serverAPI) acknowledgeSessionEnd() {

}

// tryUpdateAPI tries to update the API by calling httpspec.GetFSRoutingServerAPI on .dynamicDir.
// tryUpdateAPI should be called in a separate goroutine because it calls prepareSourceFileInExtractionMode
// that locks the session data.
func (a *serverAPI) tryUpdateAPI() {
	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())
			logs.Println(err)
		}
	}()

	if utils.IsContextDone(a.rpcSession.Context()) {
		return
	}

	_, err := a.fls.ReadDir(a.dynamicDir)
	if err != nil {
		return
	}

	_, err = a.fls.Stat(a.appModPath)
	if err != nil {
		return
	}

	handlingCtx := core.NewContext(core.ContextConfig{
		ParentContext: a.rpcSession.Context(),
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		},
		Filesystem: a.fls,
	})
	defer handlingCtx.CancelGracefully()

	// state := core.NewGlobalState(handlingCtx)
	// state.OutputFieldsInitialized.Store(true)
	// state.Project, _ = getProject(a.session)

	prepResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:                              a.appModPath,
		requiresState:                      true,
		requiresCache:                      true,
		forcePrepareIfNoVeryRecentActivity: true,

		rpcSession:      a.rpcSession,
		memberAuthToken: a.memberAuthToken,
		lspFilesystem:   a.fls,
		project:         a.project,
	})

	if !ok {
		return
	}

	state := prepResult.state

	api, err := httpspec.GetFSRoutingServerAPI(state.Ctx, a.dynamicDir, httpspec.ServerApiResolutionConfig{
		IgnoreModulesWithErrors: true,
	})

	if err != nil {
		logs.Println(err)
		return
	}

	a.lock.Lock()
	defer a.lock.Unlock()
	a.api = api
}
