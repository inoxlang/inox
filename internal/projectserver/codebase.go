package projectserver

import (
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/utils"
)

func analyzeCodebaseAndRegen(initial bool, session *Session) {
	defer utils.Recover()

	rpcSessionCtx := session.rpcSession.Context()

	analysisResult, err := analysis.AnalyzeCodebase(rpcSessionCtx, session.filesystem, analysis.Configuration{
		TopDirectories:     []string{"/"},
		InoxChunkCache:     session.inoxChunkCache,
		CssStylesheetCache: session.stylesheetCache,
	})
	if err != nil {
		session.Logger().Println(session.rpcSession.Client(), err)
		return
	}

	session.lock.Lock()
	session.lastCodebaseAnalysis = analysisResult
	session.lock.Unlock()

	if initial {
		session.cssGenerator.InitialGenAndSetup(rpcSessionCtx, analysisResult)
		session.jsGenerator.InitialGenAndSetup(rpcSessionCtx, analysisResult)
	} else {
		session.cssGenerator.RegenAll(rpcSessionCtx, analysisResult)
		session.jsGenerator.RegenAll(rpcSessionCtx, analysisResult)
	}

	publishWorkspaceDiagnostics(session, analysisResult)
}
