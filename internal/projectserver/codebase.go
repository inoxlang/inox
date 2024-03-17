package projectserver

import (
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func analyzeCodebaseAndRegen(initial bool, session *Session) {
	defer utils.Recover()

	session.lock.Lock()
	project := session.project
	lspFilesystem := session.filesystem
	memberAuthToken := session.memberAuthToken
	inoxChunkCache := session.inoxChunkCache
	stylesheetCache := session.stylesheetCache
	session.lock.Unlock()

	if project == nil {
		return
	}

	rpcSessionCtx := session.rpcSession.Context()
	handlingCtx := rpcSessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: lspFilesystem,
	})

	defer handlingCtx.CancelGracefully()

	analysisResult, err := analysis.AnalyzeCodebase(handlingCtx, analysis.Configuration{
		TopDirectories:     []string{"/"},
		InoxChunkCache:     inoxChunkCache,
		CssStylesheetCache: stylesheetCache,
		Project:            project,
		MemberAuthToken:    memberAuthToken,
	})
	if err != nil {
		logger := session.Logger()
		logger.Println(session.rpcSession.Client(), err)
		return
	}

	session.lock.Lock()
	session.lastCodebaseAnalysis = analysisResult
	session.lock.Unlock()

	if initial {
		session.cssGenerator.InitialGenAndSetup(handlingCtx, analysisResult)
		session.jsGenerator.InitialGenAndSetup(handlingCtx, analysisResult)
	} else {
		session.cssGenerator.RegenAll(handlingCtx, analysisResult)
		session.jsGenerator.RegenAll(handlingCtx, analysisResult)
	}

	publishWorkspaceDiagnostics(session, analysisResult)
}
