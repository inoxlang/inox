package projectserver

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/inoxlang/inox/internal/codebase/gen"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/devtools"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
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
	ProjectID                               string                       `json:"projectId"`
	MemberID                                string                       `json:"memberId"`
	DevSideConfig                           project.DevSideProjectConfig `json:"config"`
	TempTokens                              *project.TempProjectTokens   `json:"tempTokens,omitempty"`
	IsProjectServerAccessedThroughLocalhost bool                         `json:"isProjectServerAccessedThroughLocalhost,omitempty"`
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
			rpcSession := jsonrpc.GetSession(ctx)
			sessionCtx := rpcSession.Context()
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
			rpcSession := jsonrpc.GetSession(ctx)
			params := req.(*RegisterApplicationParams)

			proj, ok := getProject(rpcSession)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "project not open",
				}
			}

			err := proj.RegisterApplication(rpcSession.Context(), params.Name, params.ModulePath)

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
			rpcSession := jsonrpc.GetSession(ctx)
			params := req.(*ListApplicationStatusesParams)
			_ = params

			proj, ok := getProject(rpcSession)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "project not open",
				}
			}

			return ListApplicationStatusesResponse{Statuses: proj.ApplicationStatusNames(rpcSession.Context())}, nil
		},
	})
}

func getProject(rpcSession *jsonrpc.Session) (*project.Project, bool) {
	p := rpcSession.Context().ResolveUserData(CURRENT_PROJECT_CTX_DATA_PATH)
	project, ok := p.(*project.Project)
	return project, ok
}

func handleOpenProject(ctx context.Context, req interface{}, projectRegistry *project.Registry, exposeWebServers bool) (interface{}, error) {
	rpcSession := jsonrpc.GetSession(ctx)
	sessionCtx := rpcSession.Context()
	params := req.(*OpenProjectParams)

	_, ok := getProject(rpcSession)
	if ok {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "a project is already open",
		}
	}

	//Check parameters

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

	//Open the project

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

	//Authenticate the project member

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

	//Open the project copy of the member.

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

	//Create a LSP filesystem and a FS event source. The event source is primarly used to trigger codebase analysis & code generation
	//when the developer stops making file changes (IDLE state).

	lspFilesystem := NewFilesystem(workingFs, fs_ns.NewMemFilesystem(10_000_000))

	evs, err := fs_ns.NewEventSourceWithFilesystem(sessionCtx, lspFilesystem, core.PathPattern("/..."))
	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "failed to get create an event source from the filesystem",
		}
	}

	//Create a dev tools instance.

	devSessionKey := http_ns.RandomDevSessionKey()
	toolsServerPort := DEFAULT_DEV_TOOLS_PORT

	devtoolsCtx := core.NewContextWithEmptyState(core.ContextConfig{
		ParentContext: sessionCtx,
		Filesystem:    lspFilesystem,
		Permissions: append(core.GetDefaultGlobalVarPermissions(),
			core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			core.HttpPermission{Kind_: permbase.Provide, Entity: core.Host("https://localhost:" + toolsServerPort)},
			core.LThreadPermission{Kind_: permbase.Create},
		),
	}, nil)

	devtoolsInstance, err := devtools.NewInstance(devtools.InstanceParams{
		WorkingFS:       lspFilesystem,
		Project:         project,
		SessionContext:  devtoolsCtx,
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

	//In a separate goroutine initialize the devtools instance with information about the program /main.ix
	//and start the devtools web application.

	go func() {

		defer func() {
			e := recover()
			if e != nil {
				err := utils.ConvertPanicValueToError(e)
				err = fmt.Errorf("%w: %s", err, debug.Stack())
				rpcSession.LoggerPrintln(rpcSession.Client(), err)
			}
		}()

		time.Sleep(time.Second) //Wait a bit because a lot of computations are performed after the goroutine creation.

		handlerCtx := rpcSession.Context().BoundChildWithOptions(core.BoundChildContextOptions{
			Filesystem: lspFilesystem,
		})

		defer handlerCtx.CancelGracefully()

		result, ok := prepareSourceFileInExtractionMode(handlerCtx, filePreparationParams{
			fpath:         layout.MAIN_PROGRAM_PATH,
			requiresState: true,

			rpcSession:      rpcSession,
			project:         project,
			lspFilesystem:   lspFilesystem,
			memberAuthToken: memberAuthToken,
		})

		if ok {
			devtoolsInstance.InitWithPreparedMainModule(result.state)
		}

		err := devtoolsInstance.StartWebApp()
		if err != nil {
			rpcSession.LoggerPrintln(rpcSession.Client(), "failed to start dev tools server:", err)
		} else {
			rpcSession.LoggerPrintln(rpcSession.Client(), "dev tools server started")
		}
	}()

	//Update the session with the retrieved and created components.

	session := getCreateLockedProjectSession(rpcSession)
	defer session.lock.Unlock()

	session.memberAuthToken = memberAuthToken
	session.devSessionKey = devSessionKey
	sessionCtx.PutUserData(http_ns.CTX_DATA_KEY_FOR_DEV_SESSION_KEY, core.String(session.devSessionKey))

	session.filesystem = lspFilesystem
	session.repository = gitRepo
	session.project = project
	session.fsEventSource = evs
	session.devtools = devtoolsInstance

	//Create the server API.

	//Notify the LSP client about FS events.

	err = startNotifyingFilesystemStructureEvents(rpcSession, workingFs, func(event fs_ns.Event) {})

	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: err.Error(),
		}
	}

	//Create static CSS and JS generators.

	session.cssGenerator = gen.NewCssGenerator(lspFilesystem, "/static", rpcSession.Client())
	session.jsGenerator = gen.NewJSGenerator(lspFilesystem, "/static", rpcSession.Client())

	session.inoxChunkCache = parse.NewChunkCache()
	session.stylesheetCache = css.NewParseCache()

	//Initial generation.
	go analyzeCodebaseAndRegen(true, session)

	//Each time an Inox file changes we analyze the codebase and regenerate static CSS & JS.
	evs.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: 2 * fs_ns.OLD_EVENT_MIN_AGE,
		IsIgnoredEvent: func(e *core.Event) (ignore bool) {
			fsEvent := e.SourceValue().(fs_ns.Event)

			ignore = !fsEvent.IsStructureOrContentChange() || fsEvent.Path().Extension() != inoxconsts.INOXLANG_FILE_EXTENSION
			return
		},
		Microtask: func() {
			go analyzeCodebaseAndRegen(false, session)
		},
	})

	//Update the fallback session key of development servers.

	if params.IsProjectServerAccessedThroughLocalhost {
		http_ns.SetFallbackDevSessionKey(devSessionKey)
	} else {
		//If at least one developer does not access the project server through localhost
		//we remove the fallback key.
		http_ns.RemoveFallbackDevSessionKey(devSessionKey)
	}

	rpcSession.Context().OnDone(func(timeoutCtx context.Context, teardownStatus core.GracefulTeardownStatus) error {
		go func() {
			defer utils.Recover()
			http_ns.RemoveFallbackDevSessionKey(devSessionKey)
		}()
		return nil
	})

	return OpenProjectResponse{
		TempProjectTokens:   tokens,
		CanBeDeployedInProd: node.IsAgentSet(),
	}, nil
}
