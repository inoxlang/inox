package projectserver

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
)

const (
	CURRENT_PROJECT_CTX_DATA_PATH = core.Path("/current-project")
	LSP_FS_CTX_DATA_PATH          = core.Path("/current-filesystem")

	OPEN_PROJECT_METHOD              = "project/open"
	CREATE_PROJECT_METHOD            = "project/create"
	REGISTER_APPLICATION_METHOD      = "project/registerApplication"
	LIST_APPLICATION_STATUSES_METHOD = "project/listApplicationStatuses"
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
				ExposeWebServers: opts.ExposeWebServers,
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
				if err != nil {
					return nil, jsonrpc.ResponseError{
						Code:    jsonrpc.InternalError.Code,
						Message: "failed to get the working filesystem (working tree)",
					}
				}
			}

			lspFilesystem := NewFilesystem(workingFs, fs_ns.NewMemFilesystem(10_000_000))

			//Update session data.

			sessionData := getLockedSessionData(session)
			defer sessionData.lock.Unlock()

			sessionData.memberAuthToken = memberAuthToken
			sessionData.projectDevSessionKey = http_ns.RandomDevSessionKey()
			sessionCtx.PutUserData(http_ns.CTX_DATA_KEY_FOR_DEV_SESSION_KEY, core.String(sessionData.projectDevSessionKey))

			sessionData.filesystem = lspFilesystem
			sessionData.project = project
			sessionData.serverAPI = newServerAPI(lspFilesystem, session, memberAuthToken)

			go sessionData.serverAPI.tryUpdateAPI() //use a goroutine to avoid deadlock

			err = startNotifyingFilesystemStructureEvents(session, workingFs, func(event fs_ns.Event) {
				sessionData.serverAPI.acknowledgeStructureChangeEvent(event)
			})

			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			return OpenProjectResponse{
				TempProjectTokens:   tokens,
				CanBeDeployedInProd: node.IsAgentSet(),
			}, nil
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
