package projectserver

import (
	"maps"

	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func publishWorkspaceDiagnostics(session *jsonrpc.Session, projSession *additionalSessionData, lastAnalysis *analysis.Result) {

	projSession.lock.Lock()
	docDiagnostics := maps.Clone(projSession.documentDiagnostics)
	projSession.lock.Unlock()

	for absPath, diagnostics := range docDiagnostics {
		uri, err := getFileURI(absPath, projSession.projectMode)
		if err != nil {
			continue
		}

		go func(uri defines.DocumentUri, diagnostics *documentDiagnostics) {
			diagnostics.lock.Lock()
			defer diagnostics.lock.Unlock()

			if diagnostics.containsWorkspaceDiagnostics {
				return
			}

			// diagnostics.items = append(diagnostics.items, defines.Diagnostic{
			// 	Range:   defines.Range{Start: defines.Position{1, 2}, End: defines.Position{1, 2}},
			// 	Message: "LOL",
			// })
			diagnostics.containsWorkspaceDiagnostics = true
			sendDocumentDiagnostics(session, uri, diagnostics.items)
		}(uri, diagnostics)

	}

}
