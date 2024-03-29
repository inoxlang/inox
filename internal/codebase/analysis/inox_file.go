package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
)

func (a *analyzer) preAnalyzeInoxFile(path string, fileContent string, chunk *parse.Chunk) error {

	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

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
		a.result.InoxModules[path] = info

		if a.ctx.IsDoneSlowCheck() {
			return a.ctx.Err()
		}
	}

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		switch node := node.(type) {
		//XML
		case *parse.XMLAttribute:
			a.preAnalyzeXmlAttribute(node)
		case *parse.HyperscriptAttributeShorthand:
			a.preAnalyzeHyperscriptAtributeShortand(node)
		case *parse.XMLElement:
			a.preAnalyzeXmlElement(node)
		case *parse.XMLText:
			if strings.Contains(node.Value, inoxjs.TEXT_INTERPOLATION_OPENING_DELIMITER) && !a.result.IsInoxComponentLibUsed {
				a.result.IsInoxComponentLibUsed = true
				a.result.UsedInoxJsLibs = append(a.result.UsedInoxJsLibs, inoxjs.INOX_COMPONENT_LIB_NAME)
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)

	return nil
}
