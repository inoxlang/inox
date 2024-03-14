package projectserver

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
)

const (
	//request methods

	ENABLE_TEST_DISCOVERY_METHOD = "testing/enableContinousDiscovery"
	TEST_FILE_METHOD             = "testing/testFileAsync"
	STOP_TEST_RUN_METHOD         = "testing/stopRun"

	//notification methods

	TEST_OUTPUT_EVENT_METHOD = "testing/outputEvent"
	TEST_RUN_FINISHED_METHOD = "testing/runFinished"
)

type EnableContinuousTestDiscoveryParams struct {
}

type TestOutputEvent struct {
	DataBase64 string `json:"data"`
}

type RunFinishedParams struct {
}

type TestFileParams struct {
	Path            string       `json:"path"`
	PositiveFilters []TestFilter `json:"positiveFilters"`
}

func (p TestFileParams) Filters() core.TestFilters {
	var positiveFilters []core.TestFilter

	for _, filter := range p.PositiveFilters {
		positiveFilters = append(positiveFilters, filter.Filter())
	}

	return core.TestFilters{
		PositiveTestFilters: positiveFilters,
	}
}

type TestFileResponse struct {
	TestRunId TestRunId `json:"testRunId"`
}

type StopTestRunParams struct {
	TestRunId TestRunId `json:"testRunId"`
}

type TestFilter struct {
	Regex        string         `json:"regex"`
	AbsolutePath string         `json:"path,omitempty"`
	NodeSpan     parse.NodeSpan `json:"span,omitempty"`
}

func (f TestFilter) Filter() core.TestFilter {
	return core.TestFilter{
		AbsolutePath: f.AbsolutePath,
		NameRegex:    f.Regex,
		NodeSpan:     f.NodeSpan,
	}
}

func registerTestingMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {

	server.OnCustom(jsonrpc.MethodInfo{
		Name: ENABLE_TEST_DISCOVERY_METHOD,
		NewRequest: func() interface{} {
			return &EnableContinuousTestDiscoveryParams{}
		},
		RateLimits: []int{0, 2},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			rpcSession := jsonrpc.GetSession(ctx)
			//TODO
			// params := req.(*EnableContinuousTestDiscoveryParams)

			// data := getLockedSessionData(session)

			_ = rpcSession
			return nil, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: TEST_FILE_METHOD,
		NewRequest: func() interface{} {
			return &TestFileParams{}
		},
		RateLimits: []int{2, 10, 30},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			rpcSession := jsonrpc.GetSession(ctx)
			params := req.(*TestFileParams)

			//-----------------------------------------------
			session := getCreateLockedProjectSession(rpcSession)
			memberAuthToken := session.memberAuthToken
			session.lock.Unlock()
			//-----------------------------------------------

			return testModuleAsync(params.Path, params.Filters(), rpcSession, memberAuthToken)
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: STOP_TEST_RUN_METHOD,
		NewRequest: func() interface{} {
			return &StopTestRunParams{}
		},
		RateLimits: []int{2, 10, 30},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			rpcSession := jsonrpc.GetSession(ctx)
			params := req.(*StopTestRunParams)

			session := getCreateLockedProjectSession(rpcSession)
			run, ok := session.testRuns[params.TestRunId]
			delete(session.testRuns, params.TestRunId)
			session.lock.Unlock()

			if ok {
				run.state.Ctx.CancelGracefully()
			}

			return nil, nil
		},
	})
}
