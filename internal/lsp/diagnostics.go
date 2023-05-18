package internal

import (
	"encoding/json"
	"io"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	globals "github.com/inoxlang/inox/internal/globals"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func notifyDiagnostics(session *jsonrpc.Session, docURI defines.DocumentUri, compilationCtx *core.Context) error {
	fpath := getFilePath(docURI)

	errSeverity := defines.DiagnosticSeverityError
	warningSeverity := defines.DiagnosticSeverityWarning

	state, mod, _, err := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       io.Discard,
		IgnoreNonCriticalIssues:   true,
		AllowMissingEnvVars:       true,
	})

	if mod == nil { //unrecoverable parsing error
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil
	}

	//we need the diagnostics list to be present in the notification so diagnostics should not be nil
	diagnostics := make([]defines.Diagnostic, 0)

	if err == nil {
		logs.Println("no errors")
		goto send_diagnostics
	}

	if err != nil && state == nil && mod == nil {
		logs.Println("err", err)
		goto send_diagnostics
	}

	{

		i := -1
		parsingDiagnostics := utils.MapSlice(mod.ParsingErrors, func(err core.Error) defines.Diagnostic {
			i++

			return defines.Diagnostic{
				Message:  err.Text(),
				Severity: &errSeverity,
				Range:    rangeToLspRange(mod.ParsingErrorPositions[i]),
			}
		})

		diagnostics = append(diagnostics, parsingDiagnostics...)

		if state != nil && state.StaticCheckData != nil {
			i := -1
			staticCheckDiagnostics := utils.MapSlice(state.StaticCheckData.Errors(), func(err *core.StaticCheckError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, staticCheckDiagnostics...)

			i = -1
			symbolicCheckErrorDiagnostics := utils.MapSlice(state.SymbolicData.Errors(), func(err symbolic.SymbolicEvaluationError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckErrorDiagnostics...)

			symbolicCheckWarningDiagnostics := utils.MapSlice(state.SymbolicData.Warnings(), func(err symbolic.SymbolicEvaluationWarning) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &warningSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckWarningDiagnostics...)
		}
	}

send_diagnostics:
	session.Notify(jsonrpc.NotificationMessage{
		BaseMessage: jsonrpc.BaseMessage{
			Jsonrpc: JSONRPC_VERSION,
		},
		Method: "textDocument/publishDiagnostics",
		Params: utils.Must(json.Marshal(defines.PublishDiagnosticsParams{
			Uri:         docURI,
			Diagnostics: diagnostics,
		})),
	})

	return nil
}
