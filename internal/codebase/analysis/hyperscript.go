package analysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
)

func (a *analyzer) addUsedHyperscriptFeaturesAndCommands(node parse.Node) {
	//Get tokens

	allUsedFeatures := a.result.UsedHyperscriptFeatures
	allUsedCommands := a.result.UsedHyperscriptCommands

	switch node := node.(type) {
	case *parse.HyperscriptAttributeShorthand:
		if node.HyperscriptParsingResult != nil {
			hsanalysis.AddUsedFeaturesAndCommands(node.HyperscriptParsingResult.NodeData, allUsedFeatures, allUsedCommands)

		} else if node.HyperscriptParsingError != nil {
			err := node.HyperscriptParsingError.Tokens
			hsanalysis.GuessUsedFeaturesAndCommandsFromTokens(err, allUsedFeatures, allUsedCommands)
		}
	case *parse.MarkupElement:
		if node.EstimatedRawElementType == parse.HyperscriptScript {
			result, ok := node.RawElementParsingResult.(*hscode.ParsingResult)
			if ok {
				hsanalysis.AddUsedFeaturesAndCommands(result.NodeData, allUsedFeatures, allUsedCommands)
			} else if err, ok := node.RawElementParsingResult.(*hscode.ParsingError); ok {
				hsanalysis.GuessUsedFeaturesAndCommandsFromTokens(err.Tokens, allUsedFeatures, allUsedCommands)
			}
		}
	}
}

func (a *analyzer) analyzeHyperscriptComponent(component *hsanalysis.Component) (criticalError error) {
	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

	//Retrieval symbolic data about the markup.

	//Analyze

	params := hsanalysis.Parameters{
		Node:             component.AttributeShorthand.HyperscriptParsingResult.NodeData,
		LocationKind:     hsanalysis.UnderscoreAttribute,
		Component:        component,
		Chunk:            component.ChunkSource,
		InoxNodePosition: component.ChunkSource.GetSourcePosition(component.AttributeShorthand.Span),
	}

	errors, warnings, criticalError := hsanalysis.Analyze(params)

	if criticalError != nil {
		return
	}

	a.result.HyperscriptErrors = append(a.result.HyperscriptErrors, errors...)
	a.result.HyperscriptWarnings = append(a.result.HyperscriptWarnings, warnings...)

	return
}
