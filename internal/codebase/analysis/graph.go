package analysis

import (
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/memds"
)

type Result struct {
	inner *memds.DirectedGraph[Node, Edge, additionalGraphData]

	UsedHtmxExtensions map[string]struct{}

	UsedHyperscriptCommands map[string]hsgen.Definition
	UsedHyperscriptFeatures map[string]hsgen.Definition

	UsedTailwindRules    map[string]tailwind.Ruleset
	CssVariables         map[CssVarName]CssVariable
	UsedVarBasedCssRules map[CssVarName]CssVariable

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

		CssVariables:         make(map[CssVarName]CssVariable),
		UsedVarBasedCssRules: make(map[CssVarName]CssVariable),
	}

	return result
}
