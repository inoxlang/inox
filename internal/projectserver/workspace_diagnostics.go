package projectserver

import (
	"maps"
	"slices"

	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

// publishWorkspaceDiagnostics adds diagnostics to files, these diagnostics are created from errors and warnings reported
// by the last codebase analysis. Diagnostics are pushed using "textDocument/publishDiagnostics", the standard LSP method
// "workspace/diagnostic" is not handled.
func publishWorkspaceDiagnostics(projSession *Session, lastAnalysis *analysis.Result) {

	projSession.lock.Lock()
	docDiagnostics := maps.Clone(projSession.documentDiagnostics)
	projSession.lock.Unlock()

	//Find documents that are not in the docDiagnostics map but that contain errors that need to be reported.

	trackedDocs := map[absoluteFilePath]struct{}{}
	nonTrackedDocs := map[absoluteFilePath]struct{}{}

	for absPath := range docDiagnostics {
		trackedDocs[absPath] = struct{}{}
	}

	addNonTrackedDoc := func(path absoluteFilePath) {
		if _, ok := trackedDocs[path]; ok {
			return
		}
		nonTrackedDocs[path] = struct{}{}
	}

	for _, err := range lastAnalysis.InoxJsErrors {
		path, ok := absoluteFilePathFrom(err.Location.SourceName)
		if ok {
			addNonTrackedDoc(path)
		}
	}

	for _, err := range lastAnalysis.HyperscriptErrors {
		path, ok := absoluteFilePathFrom(err.Location.SourceName)
		if ok {
			addNonTrackedDoc(path)
		}
	}

	for _, warning := range lastAnalysis.HyperscriptWarnings {
		path, ok := absoluteFilePathFrom(warning.Location.SourceName)
		if ok {
			addNonTrackedDoc(path)
		}
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

		items := slices.Clone(diagnostics.items)

		go func(absPath absoluteFilePath, uri defines.DocumentUri, items *[]defines.Diagnostic) {
			addErrorsAndWarningsAboutFileFromCodebaseAnalysis(lastAnalysis, absPath, items)
			sendDocumentDiagnostics(projSession.rpcSession, uri, *items)
		}(absPath, uri, &items)
	}

	for nonTrackedDocName := range nonTrackedDocs {
		uri, err := getFileURI(nonTrackedDocName, projSession.inProjectMode)
		if err != nil {
			continue
		}

		items := &[]defines.Diagnostic{}
		addErrorsAndWarningsAboutFileFromCodebaseAnalysis(lastAnalysis, nonTrackedDocName, items)
		sendDocumentDiagnostics(projSession.rpcSession, uri, *items)
	}

}

func makeDiagnosticFromLocatedError(e parse.LocatedError) defines.Diagnostic {
	return defines.Diagnostic{
		Range:    rangeToLspRange(e.LocationRange()),
		Severity: &errSeverity,
		Message:  e.MessageWithoutLocation(),
	}
}

func addErrorsAndWarningsAboutFileFromCodebaseAnalysis(lastAnalysis *analysis.Result, absPath absoluteFilePath, items *[]defines.Diagnostic) {
	//Add InoxJS diagnostics.

	for _, err := range lastAnalysis.InoxJsErrors {

		if err.Location.SourceName != string(absPath) { //ignore errors concerning other documents.
			continue
		}

		*items = append(*items, makeDiagnosticFromLocatedError(err))
	}

	//Add Hyperscript diagnostics.

	for _, err := range lastAnalysis.HyperscriptErrors {

		if err.Location.SourceName != string(absPath) { //ignore errors concerning other documents.
			continue
		}

		*items = append(*items, makeDiagnosticFromLocatedError(err))
	}

	for _, warning := range lastAnalysis.HyperscriptWarnings {
		if warning.Location.SourceName != string(absPath) { //ignore warnings concerning other documents.
			continue
		}

		*items = append(*items, defines.Diagnostic{
			Range:    rangeToLspRange(warning.Location),
			Severity: &warningSeverity,
			Message:  warning.Message,
		})
	}
}
