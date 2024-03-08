package analysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/parse"
)

func analyzeHyperscriptAtributeShortand(node *parse.HyperscriptAttributeShorthand, state *inoxFileAnalysisState, result *Result) {
	addUsedHyperscriptFeaturesAndCommands(node, result)
}

func addUsedHyperscriptFeaturesAndCommands(node parse.Node, result *Result) {
	var tokens []hscode.Token

	switch node := node.(type) {
	case *parse.HyperscriptAttributeShorthand:
		if node.HyperscriptParsingResult != nil {
			tokens = append(tokens, node.HyperscriptParsingResult.Tokens...)
		} else if node.HyperscriptParsingError != nil {
			tokens = append(tokens, node.HyperscriptParsingError.Tokens...)
		}
	case *parse.XMLElement:
		if node.EstimatedRawElementType == parse.HyperscriptScript {
			result, ok := node.RawElementParsingResult.(*hscode.ParsingResult)
			if ok {
				tokens = append(tokens, result.Tokens...)
			} else if err, ok := node.RawElementParsingResult.(*hscode.ParsingError); ok {
				tokens = append(tokens, err.Tokens...)
			}
		}
	}

	for _, token := range tokens {
		if token.Type != hscode.IDENTIFIER {
			continue
		}
		if hsgen.IsBuiltinFeatureName(token.Value) || hsgen.IsBuiltinCommandName(token.Value) {
			def, ok := hsgen.GetBuiltinDefinition(token.Value)
			if ok {
				switch def.Kind {
				case hsgen.CommandDefinition:
					result.UsedHyperscriptCommands[def.Name] = struct{}{}
				case hsgen.FeatureDefinition:
					result.UsedHyperscriptFeatures[def.Name] = struct{}{}
				}
			}
		}
	}
}
