package projectserver

import (
	"context"

	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
)

const DEPLOY_PROD_APP_METHOD = "prod/deployApplication"
const STOP_PROD_APP_METHOD = "prod/stopApplication"

type DeployAppParams struct {
	AppName          string `json:"name"`
	UpdateRunningApp bool   `json:"updateRunningApp"`
}

type DeployAppResponse struct {
	Error string `json:"error,omitempty"`
}

type StopAppParams struct {
	AppName string `json:"name"`
}

type StopAppResponse struct {
	Error string `json:"error,omitempty"`
}

func registerProdMethodHandlers(server *lsp.Server, opts LSPServerConfiguration, projectRegistry *project.Registry) {

	server.OnCustom(jsonrpc.MethodInfo{
		Name: DEPLOY_PROD_APP_METHOD,
		NewRequest: func() interface{} {
			return &DeployAppParams{}
		},
		RateLimits: []int{0, 2, 5},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DeployAppParams)

			proj, ok := getProject(session)
			if !ok {
				return jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "project is not open",
				}, nil
			}

			deployment, err := proj.PrepareApplicationDeployment(project.ApplicationDeploymentPreparationParams{
				AppName:          params.AppName,
				UpdateRunningApp: params.UpdateRunningApp,
			})

			if err != nil {
				return DeployAppResponse{
					Error: err.Error(),
				}, nil
			}

			err = deployment.Perform()

			if err != nil {
				return DeployAppResponse{
					Error: err.Error(),
				}, nil
			}

			return DeployAppResponse{}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: STOP_PROD_APP_METHOD,
		NewRequest: func() interface{} {
			return &StopAppParams{}
		},
		RateLimits: []int{0, 5, 20},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*StopAppParams)

			proj, ok := getProject(session)
			if !ok {
				return jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "project is not open",
				}, nil
			}

			err := proj.StopApplication(project.StopApplicationParams{
				AppName: params.AppName,
			})

			if err != nil {
				return StopAppResponse{
					Error: err.Error(),
				}, nil
			}

			return StopAppResponse{}, nil
		},
	})
}
