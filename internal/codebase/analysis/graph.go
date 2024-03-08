package analysis

import (
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/memds"
)

type Result struct {
	inner                   *memds.DirectedGraph[Node, Edge, additionalGraphData]
	UsedHtmxExtensions      map[string]struct{}
	UsedHyperscriptCommands map[string]struct{}
	UsedHyperscriptFeatures map[string]struct{}
	UsedTailwindRules       map[string]tailwind.Ruleset
	IsSurrealUsed           bool
	IsCssScopeInlineUsed    bool
	IsPreactSignalsLibUsed  bool
	IsInoxComponentLibUsed  bool
}

type additionalGraphData struct {
}

func newEmptyResult() *Result {
	result := &Result{
		inner:                   memds.NewDirectedGraphWithAdditionalData[Node, Edge](memds.ThreadSafe, additionalGraphData{}),
		UsedHtmxExtensions:      make(map[string]struct{}),
		UsedHyperscriptCommands: make(map[string]struct{}),
		UsedHyperscriptFeatures: make(map[string]struct{}),
		UsedTailwindRules:       make(map[string]tailwind.Ruleset),
	}

	return result
}
