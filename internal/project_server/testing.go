package project_server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

type TestRun struct {
	id    TestRunId
	state *core.GlobalState
}

type TestRunId string

func testFileAsync(path string, filters core.TestFilters, session *jsonrpc.Session) (TestFileResponse, error) {

	fls, ok := getLspFilesystem(session)
	if !ok {
		return TestFileResponse{}, errors.New(string(FsNoFilesystem))
	}

	project, ok := getProject(session)
	if !ok {
		return TestFileResponse{}, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: "testing using the LSP only works in project mode for now",
		}
	}

	handlingCtx := session.Context().BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	// data := getLockedSessionData(session)

	state, _, _, err := core.PrepareLocalScript(core.ScriptPreparationArgs{
		Fpath:                     path,
		ParsingCompilationContext: handlingCtx,
		ParentContext:             handlingCtx,
		ParentContextRequired:     true,
		DefaultLimits:             core.GetDefaultScriptLimits(),

		PreinitFilesystem: fls,

		AllowMissingEnvVars:   false,
		FullAccessToDatabases: true,
		EnableTesting:         true,
		TestFilters:           filters,

		Project: project,

		Out: utils.FnWriter{
			WriteFn: func(p []byte) (n int, err error) {
				sendTestOutput(p, session)
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
	data := getLockedSessionData(session)
	data.testRuns[testRun.id] = testRun
	data.lock.Unlock()

	go func() {
		defer utils.Recover()

		defer func() {
			sendTestRunFinished(session)
		}()

		twState := core.NewTreeWalkStateWithGlobal(state)

		_, err := core.TreeWalkEval(state.Module.MainChunk.Node, twState)
		if err != nil {
			sendTestOutput(utils.StringAsBytes(err.Error()), session)
			return
		}

		if state == nil || len(state.TestSuiteResults) == 0 {
			return
		}

		buf := bytes.NewBufferString("TEST RESULTS\r\n\r\n")

		colorized := false
		backgroundIsDark := true

		for _, suiteResult := range state.TestSuiteResults {
			msg := utils.AddCarriageReturnAfterNewlines(suiteResult.MostAdaptedMessage(colorized, backgroundIsDark))
			fmt.Fprint(buf, msg)
		}

		sendTestOutput(buf.Bytes(), session)
	}()

	return TestFileResponse{
		TestRunId: testRun.id,
	}, nil
}

func sendTestOutput(bytesOrStringBytes []byte, session *jsonrpc.Session) {
	//TODO: split in chunks

	//improve output
	msg := bytes.ReplaceAll(bytesOrStringBytes, []byte{'\r'}, nil)

	outputEvent := TestOutputEvent{
		DataBase64: base64.StdEncoding.EncodeToString(msg),
	}

	session.Notify(jsonrpc.NotificationMessage{
		Method: TEST_OUTPUT_EVENT_METHOD,
		Params: utils.Must(json.Marshal(outputEvent)),
	})
}

func sendTestRunFinished(session *jsonrpc.Session) {
	runFinished := RunFinishedParams{}

	session.Notify(jsonrpc.NotificationMessage{
		Method: TEST_RUN_FINISHED_METHOD,
		Params: utils.Must(json.Marshal(runFinished)),
	})
}

func makeTestRunId() TestRunId {
	return TestRunId(ulid.Make().String())
}
