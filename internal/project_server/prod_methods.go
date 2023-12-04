package project_server

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/lsp"
)

const DEPLOY_PROD_APP_METHOD = "prod/deployApplication"

type DeployAppParams struct {
	AppModule        string `json:"module"`
	AppName          string `json:"name"`
	UpdateRunningApp bool   `json:"updateRunningApp"`
}

type DeployAppResponse struct {
	Error string `json:"error,omitempty"`
}

func registerProdMethodHandlers(server *lsp.Server, opts LSPServerConfiguration, projectRegistry *project.Registry) {

	server.OnCustom(jsonrpc.MethodInfo{
		Name:          DEPLOY_PROD_APP_METHOD,
		SensitiveData: true,
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
				ModulePath:       core.Path(params.AppModule),
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
}
