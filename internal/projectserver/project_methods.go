package projectserver

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/codebase/gen"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/dev"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	CURRENT_PROJECT_CTX_DATA_PATH = core.Path("/current-project")
	LSP_FS_CTX_DATA_PATH          = core.Path("/current-filesystem")

	OPEN_PROJECT_METHOD              = "project/open"
	CREATE_PROJECT_METHOD            = "project/create"
	REGISTER_APPLICATION_METHOD      = "project/registerApplication"
	LIST_APPLICATION_STATUSES_METHOD = "project/listApplicationStatuses"
	DEFAULT_DEV_TOOLS_PORT           = inoxconsts.DEV_PORT_2
)

type CreateProjectParams struct {
	Name       string `json:"name"`
	AddTutFile bool   `json:"addTutFile"`
	Template   string `json:"template"`
}

type CreateProjectResponse struct {
	ProjectID string `json:"projectId"`
	OwnerID   string `json:"ownerId"`
}

type OpenProjectParams struct {
	ProjectID     string                       `json:"projectId"`
	MemberID      string                       `json:"memberId"`
	DevSideConfig project.DevSideProjectConfig `json:"config"`
	TempTokens    *project.TempProjectTokens   `json:"tempTokens,omitempty"`
}

type OpenProjectResponse struct {
	project.TempProjectTokens `json:"tempTokens"`
	CanBeDeployedInProd       bool `json:"canBeDeployedInProd"`
}

type RegisterApplicationParams struct {
	Name       string `json:"name"`
	ModulePath string `json:"modulePath"`
}

type RegisterApplicationResponse struct {
	Error string `json:"error,omitempty"`
}

type ListApplicationStatusesParams struct {
}

type ListApplicationStatusesResponse struct {
	Statuses map[node.ApplicationName]string `json:"statuses"`
}

func registerProjectMethodHandlers(server *lsp.Server, opts LSPServerConfiguration, projectRegistry *project.Registry) {
	server.OnCustom(jsonrpc.MethodInfo{
		Name:          CREATE_PROJECT_METHOD,
		SensitiveData: true,
		NewRequest: func() interface{} {
			return &CreateProjectParams{}
		},
		RateLimits: []int{0, 0, 2},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			params := req.(*CreateProjectParams)

			projectId, ownerID, err := projectRegistry.CreateProject(sessionCtx, project.CreateProjectParams{
				Name:       params.Name,
				AddTutFile: params.AddTutFile,
				Template:   params.Template,
			})

			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			return CreateProjectResponse{
				ProjectID: string(projectId),
				OwnerID:   string(ownerID),
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name:          OPEN_PROJECT_METHOD,
		SensitiveData: true,
		NewRequest: func() interface{} {
			return &OpenProjectParams{}
		},
		RateLimits: []int{0, 2, 5},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			return handleOpenProject(ctx, req, projectRegistry, opts.ExposeWebServers)
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name:          REGISTER_APPLICATION_METHOD,
		SensitiveData: true,
		NewRequest: func() interface{} {
			return &RegisterApplicationParams{}
		},
		RateLimits: []int{0, 0, 2},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*RegisterApplicationParams)

			proj, ok := getProject(session)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "project not open",
				}
			}

			err := proj.RegisterApplication(session.Context(), params.Name, params.ModulePath)

			if err != nil {
				return RegisterApplicationResponse{
					Error: err.Error(),
				}, nil
			}

			return RegisterApplicationResponse{}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: LIST_APPLICATION_STATUSES_METHOD,
		NewRequest: func() interface{} {
			return &ListApplicationStatusesParams{}
		},
		RateLimits: []int{3, 10, 50},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*ListApplicationStatusesParams)
			_ = params

			proj, ok := getProject(session)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "project not open",
				}
			}

			return ListApplicationStatusesResponse{Statuses: proj.ApplicationStatusNames(session.Context())}, nil
		},
	})
}

func getProject(session *jsonrpc.Session) (*project.Project, bool) {
	p := session.Context().ResolveUserData(CURRENT_PROJECT_CTX_DATA_PATH)
	project, ok := p.(*project.Project)
	return project, ok
}

func handleOpenProject(ctx context.Context, req interface{}, projectRegistry *project.Registry, exposeWebServers bool) (interface{}, error) {
	session := jsonrpc.GetSession(ctx)
	sessionCtx := session.Context()
	params := req.(*OpenProjectParams)

	_, ok := getProject(session)
	if ok {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "a project is already open",
		}
	}

	projectId := core.ProjectID(params.ProjectID)
	if err := projectId.Validate(); err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "invalid project ID",
		}
	}

	memberId := access.MemberID(params.MemberID)
	if err := memberId.Validate(); err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "invalid member ID",
		}
	}

	project, err := projectRegistry.OpenProject(sessionCtx, project.OpenProjectParams{
		Id:               projectId,
		DevSideConfig:    params.DevSideConfig,
		TempTokens:       params.TempTokens,
		ExposeWebServers: exposeWebServers,
	})

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	_, err = project.AuthenticateMember(sessionCtx, memberId)
	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	memberAuthToken := string(memberId)

	//TODO: limit the number of concurrent sessions for the same member.

	sessionCtx.PutUserData(CURRENT_PROJECT_CTX_DATA_PATH, project)

	tokens, err := project.TempProjectTokens(sessionCtx)
	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	//Update the project copy of the member.

	developerCopy, err := project.DevCopy(sessionCtx, memberAuthToken)

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	workingFs, ok := developerCopy.WorkingFilesystem()
	if !ok {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "failed to get the working filesystem (working tree)",
		}
	}

	gitRepo, ok := developerCopy.Repository()
	if !ok {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "failed to get local git repository",
		}
	}

	//Create filesystem and FS event source.

	lspFilesystem := NewFilesystem(workingFs, fs_ns.NewMemFilesystem(10_000_000))

	evs, err := fs_ns.NewEventSourceWithFilesystem(sessionCtx, lspFilesystem, core.PathPattern("/..."))
	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "failed to get create an event source from the filesystem",
		}
	}

	//Create a development session.

	devSessionKey := http_ns.RandomDevSessionKey()
	toolsServerPort := DEFAULT_DEV_TOOLS_PORT

	devSessionCtx := core.NewContextWithEmptyState(core.ContextConfig{
		ParentContext: sessionCtx,
		Filesystem:    lspFilesystem,
		Permissions: append(core.GetDefaultGlobalVarPermissions(),
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			core.HttpPermission{Kind_: permkind.Provide, Entity: core.Host("https://localhost:" + toolsServerPort)},
			core.LThreadPermission{Kind_: permkind.Create},
		),
	}, nil)

	devSession, err := dev.NewDevSession(dev.SessionParams{
		WorkingFS:       lspFilesystem,
		Project:         project,
		SessionContext:  devSessionCtx,
		ToolsServerPort: toolsServerPort,
		DevSessionKey:   devSessionKey,
		MemberAuthToken: memberAuthToken,
	})

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: fmt.Sprintf("failed to create the development session: %s", err.Error()),
		}
	}

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				err := utils.ConvertPanicValueToError(e)
				err = fmt.Errorf("%w: %s", err, debug.Stack())
				logs.Println(session.Client(), err)
			}
		}()

		time.Sleep(time.Second) //Wait a bit because a lot of computations are performed after the goroutine creation.

		handlerCtx := core.NewContextWithEmptyState(core.ContextConfig{
			ParentContext: session.Context(),
		}, nil)
		defer handlerCtx.CancelGracefully()

		result, ok := prepareSourceFileInExtractionMode(handlerCtx, filePreparationParams{
			fpath:           layout.MAIN_PROGRAM_PATH,
			session:         session,
			memberAuthToken: memberAuthToken,
			requiresState:   true,
		})

		if ok {
			devSession.InitWithPreparedMainModule(result.state)
		}

		err := devSession.DevToolsServer()
		if err != nil {
			logs.Println(session.Client(), "failed to start dev tools server:", err)
		} else {
			logs.Println(session.Client(), "dev tools server started")
		}
	}()

	//Update session data.

	sessionData := getLockedSessionData(session)
	defer sessionData.lock.Unlock()

	sessionData.memberAuthToken = memberAuthToken
	sessionData.projectDevSessionKey = devSessionKey
	sessionCtx.PutUserData(http_ns.CTX_DATA_KEY_FOR_DEV_SESSION_KEY, core.String(sessionData.projectDevSessionKey))

	sessionData.filesystem = lspFilesystem
	sessionData.repository = gitRepo
	sessionData.project = project
	sessionData.fsEventSource = evs
	sessionData.devSession = devSession

	//Create the server API (application).

	sessionData.serverAPI = newServerAPI(lspFilesystem, session, memberAuthToken)

	go sessionData.serverAPI.tryUpdateAPI() //use a goroutine to avoid deadlock

	//Notify the LSP client about FS events and refresh the server API on certain events.

	err = startNotifyingFilesystemStructureEvents(session, workingFs, func(event fs_ns.Event) {
		sessionData.serverAPI.acknowledgeStructureChangeEvent(event)
	})

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	//Create static CSS and JS generators.

	sessionData.cssGenerator = gen.NewCssGenerator(lspFilesystem, "/static", session.Client())
	sessionData.jsGenerator = gen.NewJSGenerator(lspFilesystem, "/static", session.Client())

	chunkCache := parse.NewChunkCache()
	stylesheetParseCache := css.NewParseCache()

	analyzeCodebaseAndRegen := func(initial bool) {
		defer utils.Recover()
		analysisResult, err := analysis.AnalyzeCodebase(sessionCtx, lspFilesystem, analysis.Configuration{
			TopDirectories:     []string{"/"},
			InoxChunkCache:     chunkCache,
			CssStylesheetCache: stylesheetParseCache,
		})
		if err != nil {
			logs.Println(session.Client(), err)
			return
		}

		sessionData.lock.Lock()
		sessionData.lastCodebaseAnalysis = analysisResult
		sessionData.lock.Unlock()

		if initial {
			sessionData.cssGenerator.InitialGenAndSetup(sessionCtx, analysisResult)
			sessionData.jsGenerator.InitialGenAndSetup(sessionCtx, analysisResult)
		} else {
			sessionData.cssGenerator.RegenAll(sessionCtx, analysisResult)
			sessionData.jsGenerator.RegenAll(sessionCtx, analysisResult)
		}
	}

	go func() {
		//Initial generation.
		analyzeCodebaseAndRegen(true)
	}()

	//Each time an Inox file changes we analyze the codebase en regenerate static CSS & JS.
	evs.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: 2 * fs_ns.OLD_EVENT_MIN_AGE,
		IsIgnoredEvent: func(e *core.Event) (ignore bool) {
			fsEvent := e.SourceValue().(fs_ns.Event)

			ignore = !fsEvent.IsStructureOrContentChange() || fsEvent.Path().Extension() != inoxconsts.INOXLANG_FILE_EXTENSION
			return
		},
		Microtask: func() {
			go analyzeCodebaseAndRegen(false)
		},
	})

	return OpenProjectResponse{
		TempProjectTokens:   tokens,
		CanBeDeployedInProd: node.IsAgentSet(),
	}, nil
}
