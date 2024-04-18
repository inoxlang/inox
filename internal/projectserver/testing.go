package projectserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

type TestRun struct {
	id    TestRunId
	state *core.GlobalState
}

type TestRunId string

// testModuleAsync creates a goroutine that executes the module at $path in testing mode, testModuleAsync immediately returns
// without waiting for the tests to finish. The goroutine notifies the LSP client with TEST_RUN_FINISHED_METHOD when it is done.
// testModuleAsync should NOT be called while the session data is locked because it acquires the lock in order to
// store the testRunId in Session.testRuns.
func testModuleAsync(
	callCtx context.Context,
	path string,
	filters core.TestFilters,
	rpcSession *jsonrpc.Session,
	memberAuthToken string,
) (
	TestFileResponse,
	error,
) {

	fls, ok := getLspFilesystem(rpcSession)
	if !ok {
		return TestFileResponse{}, errors.New(string(FsNoFilesystem))
	}

	project, ok := getProject(rpcSession)
	if !ok {
		return TestFileResponse{}, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "testing using the LSP only works in project mode for now",
		}
	}

	handlingCtx := rpcSession.Context().BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem:              fls,
		AdditionalParentContext: callCtx,
	})

	//Set or override the dev session key entry of context data.
	handlingCtx.PutUserData(http_ns.CTX_DATA_KEY_FOR_DEV_SESSION_KEY, core.String(http_ns.RandomDevSessionKey()))

	// data := getLockedSessionData(session)

	state, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                    path,
		SingleFileParsingTimeout: SINGLE_FILE_PARSING_TIMEOUT,

		ParsingCompilationContext: handlingCtx,
		ParentContext:             handlingCtx,
		ParentContextRequired:     true,
		DefaultLimits:             core.GetDefaultScriptLimits(),

		PreinitFilesystem: fls,

		AllowMissingEnvVars:   false,
		FullAccessToDatabases: true,
		EnableTesting:         true,
		TestFilters:           filters,

		Project:         project,
		MemberAuthToken: memberAuthToken,

		Out: utils.FnWriter{
			WriteFn: func(p []byte) (n int, err error) {
				p = utils.StripANSISequencesInBytes(p)
				sendTestOutput(p, rpcSession)
				return len(p), nil
			},
		},
	})

	if err != nil {
		return TestFileResponse{}, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: fmt.Sprintf("failed to prepare %q: %s", path, err.Error()),
		}
	}

	testRun := &TestRun{
		id:    makeTestRunId(),
		state: state,
	}

	//-----------------------------------------------
	session := getCreateLockedProjectSession(rpcSession)
	session.testRuns[testRun.id] = testRun
	session.lock.Unlock()
	//-----------------------------------------------

	go func() {
		defer utils.Recover()

		defer func() {
			sendTestRunFinished(rpcSession)
		}()

		twState := core.NewTreeWalkStateWithGlobal(state)

		_, err := core.TreeWalkEval(state.Module.MainChunk.Node, twState)
		if err != nil {
			sendTestOutput(utils.StringAsBytes(err.Error()), rpcSession)
			return
		}

		if state == nil || len(state.TestingState.SuiteResults) == 0 {
			return
		}

		buf := bytes.NewBufferString("TEST RESULTS\r\n\r\n")

		colorized := false
		backgroundIsDark := true

		for _, suiteResult := range state.TestingState.SuiteResults {
			msg := utils.AddCarriageReturnAfterNewlines(suiteResult.MostAdaptedMessage(colorized, backgroundIsDark))
			fmt.Fprint(buf, msg)
		}

		sendTestOutput(buf.Bytes(), rpcSession)
	}()

	return TestFileResponse{
		TestRunId: testRun.id,
	}, nil
}

func sendTestOutput(bytesOrStringBytes []byte, rpcSession *jsonrpc.Session) {
	//TODO: split in chunks

	//improve output
	msg := bytes.ReplaceAll(bytesOrStringBytes, []byte{'\r'}, nil)

	outputEvent := TestOutputEvent{
		DataBase64: base64.StdEncoding.EncodeToString(msg),
	}

	rpcSession.Notify(jsonrpc.NotificationMessage{
		Method: TEST_OUTPUT_EVENT_METHOD,
		Params: utils.Must(json.Marshal(outputEvent)),
	})
}

func sendTestRunFinished(rpcSession *jsonrpc.Session) {
	runFinished := RunFinishedParams{}

	rpcSession.Notify(jsonrpc.NotificationMessage{
		Method: TEST_RUN_FINISHED_METHOD,
		Params: utils.Must(json.Marshal(runFinished)),
	})
}

func makeTestRunId() TestRunId {
	return TestRunId(ulid.Make().String())
}
