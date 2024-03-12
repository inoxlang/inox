package analysis

import (
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/css/varclasses"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/memds"
)

type Result struct {
	inner *memds.DirectedGraph[Node, Edge, additionalGraphData]

	UsedHtmxExtensions map[string]struct{}

	UsedHyperscriptCommands map[string]hsgen.Definition
	UsedHyperscriptFeatures map[string]hsgen.Definition

	UsedTailwindRules    map[ /* name with modifiers */ string]tailwind.Ruleset
	CssVariables         map[css.VarName]varclasses.Variable
	UsedVarBasedCssRules map[css.VarName]varclasses.Variable

	UsedInoxJsLibs         []string
	IsSurrealUsed          bool
	IsCssScopeInlineUsed   bool
	IsPreactSignalsLibUsed bool
	IsInoxComponentLibUsed bool
}

type additionalGraphData struct {
}

func newEmptyResult() *Result {
	result := &Result{
		inner:                   memds.NewDirectedGraphWithAdditionalData[Node, Edge](memds.ThreadSafe, additionalGraphData{}),
		UsedHtmxExtensions:      make(map[string]struct{}),
		UsedHyperscriptCommands: make(map[string]hsgen.Definition),
		UsedHyperscriptFeatures: make(map[string]hsgen.Definition),
		UsedTailwindRules:       make(map[string]tailwind.Ruleset),

		CssVariables:         make(map[css.VarName]varclasses.Variable),
		UsedVarBasedCssRules: make(map[css.VarName]varclasses.Variable),
	}

	return result
}
