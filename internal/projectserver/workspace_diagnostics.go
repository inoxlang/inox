package projectserver

import (
	"maps"
	"slices"

	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func publishWorkspaceDiagnostics(projSession *Session, lastAnalysis *analysis.Result) {

	projSession.lock.Lock()
	docDiagnostics := maps.Clone(projSession.documentDiagnostics)
	projSession.lock.Unlock()

	//Find documents that are not in the docDiagnostics map but that contain errors that need to be reported.

	trackedDocs := map[string]struct{}{}
	nonTrackedDocs := map[string]struct{}{}

	for absPath := range docDiagnostics {
		trackedDocs[absPath] = struct{}{}
	}

	addNonTrackedDoc := func(path string) {
		if _, ok := trackedDocs[path]; ok {
			return
		}
		nonTrackedDocs[path] = struct{}{}
	}

	for _, err := range lastAnalysis.InoxJsErrors {
		addNonTrackedDoc(err.Location.SourceName)
	}

	for _, err := range lastAnalysis.HyperscriptErrors {
		addNonTrackedDoc(err.Location.SourceName)
	}

	for _, warning := range lastAnalysis.HyperscriptWarnings {
		addNonTrackedDoc(warning.Location.SourceName)
	}

	publisSingleDocWorkspaceDiagnostics := func(absPath string, uri defines.DocumentUri, items []defines.Diagnostic) {
		//Add InoxJS diagnostics.

		for _, err := range lastAnalysis.InoxJsErrors {

			if err.Location.SourceName != absPath { //ignore errors concerning other documents.
				continue
			}

			items = append(items, makeDiagnosticFromLocatedError(err))
		}

		//Add Hyperscript diagnostics.

		for _, err := range lastAnalysis.HyperscriptErrors {

			if err.Location.SourceName != absPath { //ignore errors concerning other documents.
				continue
			}

			items = append(items, makeDiagnosticFromLocatedError(err))
		}

		for _, warning := range lastAnalysis.HyperscriptWarnings {
			if warning.Location.SourceName != absPath { //ignore warnings concerning other documents.
				continue
			}

			items = append(items, defines.Diagnostic{
				Range:    rangeToLspRange(warning.Location),
				Severity: &warningSeverity,
				Message:  warning.Message,
			})
		}

		sendDocumentDiagnostics(projSession.rpcSession, uri, items)
	}

	for absPath, diagnostics := range docDiagnostics {
		uri, err := getFileURI(absPath, projSession.inProjectMode)
		if err != nil {
			continue
		}

		{
			diagnostics.lock.Lock()
			containsWorkspaceDiagnostics := diagnostics.containsWorkspaceDiagnostics

			if containsWorkspaceDiagnostics {
				diagnostics.lock.Unlock()
				continue
			}
			diagnostics.containsWorkspaceDiagnostics = true
			diagnostics.lock.Unlock()
		}

		initialItems := slices.Clone(diagnostics.items)

		go func(absPath string, uri defines.DocumentUri, diagnostics *singleDocumentDiagnostics) {
			publisSingleDocWorkspaceDiagnostics(absPath, uri, initialItems)
		}(absPath, uri, diagnostics)
	}

	for nonTrackedDocPath := range nonTrackedDocs {
		uri, err := getFileURI(nonTrackedDocPath, projSession.inProjectMode)
		if err != nil {
			continue
		}

		publisSingleDocWorkspaceDiagnostics(nonTrackedDocPath, uri, nil)
	}

}

func makeDiagnosticFromLocatedError(e parse.LocatedError) defines.Diagnostic {
	return defines.Diagnostic{
		Range:    rangeToLspRange(e.LocationRange()),
		Severity: &errSeverity,
		Message:  e.MessageWithoutLocation(),
	}
}
