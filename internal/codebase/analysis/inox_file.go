package analysis

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

func (a *analyzer) prepareIfDatabaseProvidingModule(path string, _ string, sourcedChunk *parse.ParsedChunkSource, phase string) error {

	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

	if phase != "0" || sourcedChunk.Node.Manifest == nil {
		return nil
	}

	chunk := sourcedChunk.Node

	if obj, ok := chunk.Manifest.Object.(*parse.ObjectLiteral); ok {
		node, _ := obj.PropValue(inoxconsts.MANIFEST_DATABASES_SECTION_NAME)

		if _, ok := node.(*parse.ObjectLiteral); ok {
			a.getOrPrepareLocalModule(path, sourcedChunk)
		}
	}

	return nil
}

func (a *analyzer) preAnalyzeInoxFile(path string, fileContent string, sourcedChunk *parse.ParsedChunkSource, phase string) error {

	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

	if phase != "1" {
		return nil
	}

	chunk := sourcedChunk.Node //may be replaced by the chunk obtained in the module preparation below.

	if chunk.Manifest != nil {
		modInfo, ok, criticalErr := a.getOrPrepareLocalModule(path, sourcedChunk)
		if criticalErr != nil {
			return criticalErr
		}

		if ok {
			//Update the chunk to make sure to walk the same AST as the symbolic data.
			sourcedChunk = modInfo.Module.MainChunk
			chunk = sourcedChunk.Node
		}

	} else {
		fileInfo, ok, criticalErr := a.prepareIncludableFile(path, sourcedChunk)

		if criticalErr != nil {
			return criticalErr
		}

		if ok {
			//Update the chunk to make sure to walk the same AST as the symbolic data.
			sourcedChunk = fileInfo.IncludedChunk.ParsedChunkSource
			chunk = sourcedChunk.Node
		}
	}

	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

	return parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		if a.ctx.IsDoneSlowCheck() {
			return parse.StopTraversal, a.ctx.Err()
		}

		switch node := node.(type) {
		//markup
		case *parse.MarkupAttribute:
			a.preAnalyzeMarkupAttribute(node)
		case *parse.MarkupElement:
			err := a.preAnalyzeMarkupElement(node, ancestorChain, sourcedChunk)
			if err != nil {
				return parse.StopTraversal, err
			}
		case *parse.MarkupText:
			if inoxjs.ContainsClientSideInterpolation(node.Value) {
				a.result.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
				a.result.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
				a.result.ClientSideInterpolationsFound = true
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)
}

func (a *analyzer) getOrPrepareLocalModule(path string, chunk *parse.ParsedChunkSource) (_ InoxModuleInfo, _ bool, criticalError error) {
	modInfo, ok := a.result.LocalModules[path]
	if ok {
		return modInfo, true, nil
	}

	var parentCtx *core.Context
	if chunk.Node.Manifest != nil {
		if obj, ok := chunk.Node.Manifest.Object.(*parse.ObjectLiteral); ok {
			node, _ := obj.PropValue(inoxconsts.MANIFEST_DATABASES_SECTION_NAME)

			if pathLiteral, ok := node.(*parse.AbsolutePathLiteral); ok {
				mod, ok := a.result.LocalModules[pathLiteral.Value]
				if !ok { //cannot prepare module
					return InoxModuleInfo{}, false, nil
				}
				parentCtx = mod.state.Ctx
			}
		}
	}

	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			criticalError = fmt.Errorf("%w: %s", err, debug.Stack())
		}
	}()

	state, mod, manifest, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                   path,
		DataExtractionMode:      true,
		AllowMissingEnvVars:     true,
		ScriptContextFileSystem: a.fls,
		PreinitFilesystem:       a.fls,
		Project:                 a.Project,
		MemberAuthToken:         a.Configuration.MemberAuthToken,

		ParsingCompilationContext: a.ctx,
		StdlibCtx: func() context.Context {
			if parentCtx == nil {
				return a.ctx //cancel the preparation when a.ctx is done.
			}
			return nil
		}(),
		ParentContext: parentCtx,

		SingleFileParsingTimeout: a.fileParsingTimeout,
		Cache:                    a.ModuleCache,
		InoxChunkCache:           a.InoxChunkCache,

		Out:    io.Discard,
		LogOut: io.Discard,
	})

	if mod == nil {
		return InoxModuleInfo{}, false, nil
	}

	info := InoxModuleInfo{
		Manifest:         manifest,
		PreparationError: err,
		Module:           mod,
	}

	if state != nil {
		info.StaticCheckData = state.StaticCheckData
		info.SymbolicData = state.SymbolicData
		info.state = state
	}

	a.result.LocalModules[path] = info
	return info, true, nil
}

type InoxIncludableFileInfo struct {
	PreparationError error
	SymbolicData     *core.SymbolicData
	IncludedChunk    *inoxmod.IncludedChunk
}

func (a *analyzer) prepareIncludableFile(path string, chunk *parse.ParsedChunkSource) (_ InoxIncludableFileInfo, _ bool, criticalError error) {

	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			criticalError = fmt.Errorf("%w: %s", err, debug.Stack())
		}
	}()

	state, _, includedChunk, err := core.PrepareExtractionModeIncludableFile(core.IncludableFilePreparationArgs{
		Fpath:                          path,
		InoxChunkCache:                 a.InoxChunkCache,
		ParsingContext:                 a.ctx,
		StdlibCtx:                      a.ctx,
		IncludedChunkContextFileSystem: a.fls,
		SingleFileParsingTimeout:       a.fileParsingTimeout,

		Out:    io.Discard,
		LogOut: io.Discard,
	})

	if state == nil {
		return InoxIncludableFileInfo{}, false, nil
	}

	info := InoxIncludableFileInfo{
		PreparationError: err,
		SymbolicData:     state.SymbolicData,
		IncludedChunk:    includedChunk,
	}
	a.result.IncludableFiles[path] = info

	return info, true, nil
}
