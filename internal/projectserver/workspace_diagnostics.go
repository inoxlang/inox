package projectserver

import (
	"maps"

	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func publishWorkspaceDiagnostics(projSession *Session, lastAnalysis *analysis.Result) {

	projSession.lock.Lock()
	docDiagnostics := maps.Clone(projSession.documentDiagnostics)
	projSession.lock.Unlock()

	for absPath, diagnostics := range docDiagnostics {
		uri, err := getFileURI(absPath, projSession.inProjectMode)
		if err != nil {
			continue
		}

		go func(uri defines.DocumentUri, diagnostics *documentDiagnostics) {
			diagnostics.lock.Lock()
			defer diagnostics.lock.Unlock()

			if diagnostics.containsWorkspaceDiagnostics {
				return
			}

			diagnostics.containsWorkspaceDiagnostics = true

			//Add InoxJS diagnostics.

			for _, err := range lastAnalysis.InoxJsErrors {
				if err.Location.SourceName != absPath { //ignore errors concerning other documents.
					continue
				}

				diagnostics.items = append(diagnostics.items, defines.Diagnostic{
					Range:    rangeToLspRange(err.Location),
					Severity: &errSeverity,
					Message:  err.Message,
				})
			}

			//Add Hyperscript diagnostics.

			for _, err := range lastAnalysis.HyperscriptErrors {
				if err.Location.SourceName != absPath { //ignore errors concerning other documents.
					continue
				}

				diagnostics.items = append(diagnostics.items, defines.Diagnostic{
					Range:    rangeToLspRange(err.Location),
					Severity: &errSeverity,
					Message:  err.Message,
				})
			}

			for _, warning := range lastAnalysis.HyperscriptWarnings {
				if warning.Location.SourceName != absPath { //ignore errors concerning other documents.
					continue
				}

				diagnostics.items = append(diagnostics.items, defines.Diagnostic{
					Range:    rangeToLspRange(warning.Location),
					Severity: &warningSeverity,
					Message:  warning.Message,
				})
			}

			sendDocumentDiagnostics(projSession.rpcSession, uri, diagnostics.items)
		}(uri, diagnostics)

	}

}
