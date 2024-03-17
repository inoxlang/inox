package projectserver

import (
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

func analyzeCodebaseAndRegen(initial bool, session *Session) {
	defer utils.Recover()

	session.lock.Lock()
	project := session.project
	lspFilesystem := session.filesystem
	session.lock.Unlock()

	if project == nil {
		return
	}

	handlingCtx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions:   []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN}},
		Filesystem:    lspFilesystem,
		ParentContext: session.rpcSession.Context(),
	}, nil)

	defer handlingCtx.CancelGracefully()

	analysisResult, err := analysis.AnalyzeCodebase(handlingCtx, session.filesystem, analysis.Configuration{
		TopDirectories:     []string{"/"},
		InoxChunkCache:     session.inoxChunkCache,
		CssStylesheetCache: session.stylesheetCache,
		Project:            project,
	})
	if err != nil {
		session.Logger().Println(session.rpcSession.Client(), err)
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
