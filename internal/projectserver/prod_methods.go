package projectserver

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
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

func registerProdMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {

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

			return DeployAppResponse{
				Error: "Sorry, production deployment will be made available in the next minor release. If you like Inox consider donating and joining the Discord server :)",
			}, nil

			//TODO: in nodeimpl/app.go return an error on startup burst and add a specific status.

			handlerCtx := core.NewContexWithEmptyState(core.ContextConfig{
				ParentContext: session.Context(),
			}, nil)
			defer handlerCtx.CancelGracefully()

			deployment, err := proj.PrepareApplicationDeployment(handlerCtx, project.ApplicationDeploymentPreparationParams{
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

			handlerCtx := core.NewContexWithEmptyState(core.ContextConfig{
				ParentContext: session.Context(),
			}, nil)
			defer handlerCtx.CancelGracefully()

			err := proj.StopApplication(handlerCtx, project.StopApplicationParams{
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
