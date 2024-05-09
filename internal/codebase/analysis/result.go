package analysis

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/css/varclasses"
	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/parse"
)

type Result struct {
	inner *memds.DirectedGraph[Node, Edge, additionalGraphData]

	//Backend

	LocalModules       map[ /*absolute path*/ string]InoxModuleInfo
	IncludableFiles    map[ /*absolute path */ string]InoxIncludableFileInfo
	ServerAPI          *spec.API //may be nil
	ServerStaticDir    string    //may be empty
	ServerDynamicDir   string    //may be empty
	MajorBackendErrors []error

	//Frontend

	UsedHtmxExtensions map[string]struct{}

	UsedHyperscriptCommands       map[string]hsgen.Definition
	UsedHyperscriptFeatures       map[string]hsgen.Definition
	HyperscriptComponents         map[ /*name*/ string][]*hsanalysis.Component
	HyperscriptErrors             []hsanalysis.Error
	HyperscriptWarnings           []hsanalysis.Warning
	ClientSideInterpolationsFound bool

	UsedTailwindRules    map[ /* name with modifiers */ string]tailwind.Ruleset
	CssVariables         map[css.VarName]varclasses.Variable
	UsedVarBasedCssRules map[css.VarName]varclasses.Variable

	UsedInoxJsLibs map[string]struct{}
}

type additionalGraphData struct {
}

func NewEmptyResult() *Result {
	result := &Result{
		inner: memds.NewDirectedGraphWithAdditionalData[Node, Edge](memds.ThreadSafe, additionalGraphData{}),

		//Backend

		LocalModules:    make(map[string]InoxModuleInfo),
		IncludableFiles: make(map[string]InoxIncludableFileInfo),

		//Frontend

		UsedHtmxExtensions:      make(map[string]struct{}),
		UsedHyperscriptCommands: make(map[string]hsgen.Definition),
		UsedHyperscriptFeatures: make(map[string]hsgen.Definition),
		HyperscriptComponents:   make(map[string][]*hsanalysis.Component),
		UsedTailwindRules:       make(map[string]tailwind.Ruleset),
		UsedInoxJsLibs:          make(map[string]struct{}),

		CssVariables:         make(map[css.VarName]varclasses.Variable),
		UsedVarBasedCssRules: make(map[css.VarName]varclasses.Variable),
	}

	return result
}

func (r *Result) GraphNodeCount() int {
	return r.inner.NodeCount()
}

func (r *Result) GraphEdgeCount() int64 {
	return r.inner.EdgeCount()
}

func (r *Result) IsSurrealUsed() bool {
	_, ok := r.UsedInoxJsLibs[inoxjs.SURREAL_LIB_NAME]
	return ok
}

func (r *Result) IsCssScopeInlineUsed() bool {
	_, ok := r.UsedInoxJsLibs[inoxjs.CSS_INLINE_SCOPE_LIB_NAME]
	return ok
}

func (r *Result) IsPreactSignalsLibUsed() bool {
	_, ok := r.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME]
	return ok
}

func (r *Result) IsInoxComponentLibUsed() bool {
	_, ok := r.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME]
	return ok
}

func (r *Result) GetSymbolicDataForFile(chunk *parse.ParsedChunkSource) (*core.SymbolicData, bool) {
	for _, info := range r.LocalModules {
		if info.Module.MainChunk == chunk {
			return info.state.SymbolicData, true
		}
		for _, includedFileChunk := range info.Module.InclusionStatementMap {
			if includedFileChunk.ParsedChunkSource == chunk {
				return info.state.SymbolicData, true
			}
		}
	}

	for _, includableFile := range r.IncludableFiles {
		if includableFile.IncludedChunk.ParsedChunkSource == chunk {
			return includableFile.SymbolicData, true
		}
	}

	return nil, false
}
