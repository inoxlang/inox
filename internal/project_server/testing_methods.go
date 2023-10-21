package project_server

import (
	"context"

	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/lsp"
)

const (
	ENABLE_TEST_DISCOVERY_METHOD = "testing/enableContinousDiscovery"
)

type EnableContinuousTestDiscoveryParams struct {
}

func registerTestingMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {

	server.OnCustom(jsonrpc.MethodInfo{
		Name: ENABLE_TEST_DISCOVERY_METHOD,
		NewRequest: func() interface{} {
			return &EnableContinuousTestDiscoveryParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			// params := req.(*EnableContinuousTestDiscoveryParams)

			// data := getLockedSessionData(session)

			_ = session
			return nil, nil
		},
	})
}
