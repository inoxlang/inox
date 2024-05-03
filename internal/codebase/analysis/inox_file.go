package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
)

func (a *analyzer) preAnalyzeInoxFile(path string, fileContent string, chunkSource *parse.ParsedChunkSource) error {

	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

	chunk := chunkSource.Node

	if chunk.Manifest != nil {
		state, mod, manifest, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                   path,
			DataExtractionMode:      true,
			AllowMissingEnvVars:     true,
			ScriptContextFileSystem: a.fls,
			PreinitFilesystem:       a.fls,
			Project:                 a.Project,
			MemberAuthToken:         a.Configuration.MemberAuthToken,

			ParsingCompilationContext: a.ctx,
			StdlibCtx:                 a.ctx, //Cancel the preparation when a.ctx is done.

			SingleFileParsingTimeout: a.fileParsingTimeout,
			Cache:                    a.ModuleCache,
			InoxChunkCache:           a.InoxChunkCache,
		})

		info := InoxModule{
			Manifest:         manifest,
			PreparationError: err,
			Module:           mod,
		}
		if state != nil {
			info.StaticCheckData = state.StaticCheckData
			info.SymbolicData = state.SymbolicData
		}
		a.result.LocalModules[path] = info

		if a.ctx.IsDoneSlowCheck() {
			return a.ctx.Err()
		}
	}

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		switch node := node.(type) {
		//markup
		case *parse.MarkupAttribute:
			a.preAnalyzeMarkupAttribute(node)
		case *parse.HyperscriptAttributeShorthand:
			a.addUsedHyperscriptFeaturesAndCommands(node)

			markupElement, _, ok := parse.FindClosest(ancestorChain, (*parse.MarkupElement)(nil))
			if ok {
				componentName, isComponent := hsanalysis.GetHyperscriptComponentName(markupElement)
				if isComponent {
					component := hsanalysis.PreanalyzeHyperscriptComponent(componentName, markupElement, node, chunkSource)
					a.result.HyperscriptComponents[componentName] = append(a.result.HyperscriptComponents[componentName], component)
				}
			}
		case *parse.MarkupElement:
			a.preAnalyzeMarkupElement(node)
		case *parse.MarkupText:
			if strings.Contains(node.Value, inoxjs.TEXT_INTERPOLATION_OPENING_DELIMITER) {
				a.result.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
				a.result.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)

	return nil
}
