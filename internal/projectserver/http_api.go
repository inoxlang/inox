package projectserver

import (
	"fmt"
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
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	SERVER_API_UPDATE_DEBOUNCE_DURATION = time.Second / 2
)

// serverAPI stores the API of a FS-routing HTTP Server.
// It is primarilyt used for code completions.
type serverAPI struct {
	lock     sync.Mutex
	debounce func(f func())

	api        *httpspec.API //context that will to passed to httpspec.GetFSRoutingServerAPI on each update
	dynamicDir string
	appModPath string

	fls     *Filesystem
	session *jsonrpc.Session
}

func newServerAPI(fls *Filesystem, session *jsonrpc.Session) *serverAPI {
	api := &serverAPI{
		dynamicDir: "/routes",
		appModPath: "/main.ix",
		fls:        fls,
		session:    session,
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

func (a *serverAPI) acknowledgeSourceFileChange(fpath string) {
	//ignore changes in non .ix files and in files not located in the dynamic dir.
	if !strings.HasSuffix(fpath, inoxconsts.INOXLANG_FILE_EXTENSION) || !strings.HasPrefix(fpath, a.dynamicDir) {
		return
	}

	a.debounce(func() {
		defer utils.Recover()
		a.tryUpdateAPI()
	})
}

func (a *serverAPI) acknowledgeStructureChangeEvent(event fs_ns.Event) {
	path := event.Path().UnderlyingString()

	//ignore changes in non .ix files and in files not located in the dynamic dir.
	if !strings.HasSuffix(path, inoxconsts.INOXLANG_FILE_EXTENSION) || !strings.HasPrefix(path, a.dynamicDir) {
		return
	}

	a.debounce(func() {
		defer utils.Recover()
		a.tryUpdateAPI()
	})
}

func (a *serverAPI) acknowledgeSessionEnd() {

}

// tryUpdateAPI tries to update the API by calling httpspec.GetFSRoutingServerAPI on a.dynamiDir.
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

	if utils.IsContextDone(a.session.Context()) {
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
		ParentContext: a.session.Context(),
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		},
		Filesystem: a.fls,
	})
	defer handlingCtx.CancelGracefully()

	state := core.NewGlobalState(handlingCtx)
	state.OutputFieldsInitialized.Store(true)
	state.Project, _ = getProject(a.session)

	state, _, _, _, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:                              a.appModPath,
		session:                            a.session,
		requiresState:                      true,
		requiresCache:                      true,
		forcePrepareIfNoVeryRecentActivity: true,
	})

	if !ok {
		return
	}

	api, err := httpspec.GetFSRoutingServerAPI(state.Ctx, a.dynamicDir)
	if err != nil {
		logs.Println(err)
		return
	}

	a.lock.Lock()
	defer a.lock.Unlock()
	a.api = api
}
