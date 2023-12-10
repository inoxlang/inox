package projectserver

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
)

const (
	CURRENT_PROJECT_CTX_KEY = core.Identifier("current-project")
	LSP_FS_CTX_KEY          = core.Identifier("current-filesystem")

	OPEN_PROJECT_METHOD              = "project/open"
	CREATE_PROJECT_METHOD            = "project/create"
	REGISTER_APPLICATION_METHOD      = "project/registerApplication"
	LIST_APPLICATION_STATUSES_METHOD = "project/listApplicationStatuses"
)

type CreateProjectParams struct {
	Name        string `json:"name"`
	AddTutFile  bool   `json:"addTutFile"`
	AddMainFile bool   `json:"addMainFile"`
}

type OpenProjectParams struct {
	ProjectId     core.ProjectID               `json:"projectId"`
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

			projectId, err := projectRegistry.CreateProject(sessionCtx, project.CreateProjectParams{
				Name:        params.Name,
				AddTutFile:  params.AddTutFile,
				AddMainFile: params.AddMainFile,
			})

			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			return projectId, nil
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

			project, err := projectRegistry.OpenProject(sessionCtx, project.OpenProjectParams{
				Id:            params.ProjectId,
				DevSideConfig: params.DevSideConfig,
				TempTokens:    params.TempTokens,
			})

			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			sessionCtx.AddUserData(CURRENT_PROJECT_CTX_KEY, project)

			tokens, err := project.TempProjectTokens(sessionCtx)
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			lspFilesystem := NewFilesystem(project.LiveFilesystem(), fs_ns.NewMemFilesystem(10_000_000))

			sessionData := getLockedSessionData(session)
			defer sessionData.lock.Unlock()
			sessionData.filesystem = lspFilesystem
			sessionData.project = project

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
	p := session.Context().ResolveUserData(CURRENT_PROJECT_CTX_KEY)
	project, ok := p.(*project.Project)
	return project, ok
}
