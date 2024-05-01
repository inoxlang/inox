package analysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/parse"
)

func (a *analyzer) addUsedHyperscriptFeaturesAndCommands(node parse.Node) {
	var tokens []hscode.Token

	//Get tokens

	switch node := node.(type) {
	case *parse.HyperscriptAttributeShorthand:
		if node.HyperscriptParsingResult != nil {
			tokens = append(tokens, node.HyperscriptParsingResult.Tokens...)
		} else if node.HyperscriptParsingError != nil {
			tokens = append(tokens, node.HyperscriptParsingError.Tokens...)
		}
	case *parse.MarkupElement:
		if node.EstimatedRawElementType == parse.HyperscriptScript {
			result, ok := node.RawElementParsingResult.(*hscode.ParsingResult)
			if ok {
				tokens = append(tokens, result.Tokens...)
			} else if err, ok := node.RawElementParsingResult.(*hscode.ParsingError); ok {
				tokens = append(tokens, err.Tokens...)
			}
		}
	}

	//Find what features and commands are used.

	for _, token := range tokens {
		if token.Type != hscode.IDENTIFIER {
			continue
		}

		if hsgen.IsBuiltinFeatureName(token.Value) {
			def, ok := hsgen.GetBuiltinFeatureDefinition(token.Value)
			if ok {
				a.result.UsedHyperscriptFeatures[def.Name] = def
			}
		}

		if hsgen.IsBuiltinCommandName(token.Value) {
			def, ok := hsgen.GetBuiltinCommandDefinition(token.Value)
			if ok {
				a.result.UsedHyperscriptCommands[def.Name] = def
			}
		}

		//Note: some commands are also features (e.g. 'set').
	}
}

func (a *analyzer) preanalyzeHyperscriptComponent(
	elem *parse.MarkupElement,
	attribute *parse.HyperscriptAttributeShorthand,
	chunkSource *parse.ParsedChunkSource,
) {

	component := &HyperscriptComponent{
		Element:            elem,
		AttributeShorthand: attribute,
		ChunkSource:        chunkSource,
	}

	a.result.HyperscriptComponents[elem.Span] = component
}

type HyperscriptComponent struct {
	Element            *parse.MarkupElement
	AttributeShorthand *parse.HyperscriptAttributeShorthand
	ChunkSource        *parse.ParsedChunkSource
}
