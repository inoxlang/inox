package analysis

import (
	"strings"

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

type HyperscriptComponent struct {
	Name                        string
	Element                     *parse.MarkupElement
	AttributeShorthand          *parse.HyperscriptAttributeShorthand
	ChunkSource                 *parse.ParsedChunkSource
	HandledEvents               []DOMEvent
	InitialElementScopeVarNames []string // example: {":a", ":b"}
}

type DOMEvent struct {
	Type string
}

func (a *analyzer) preanalyzeHyperscriptComponent(
	componentName string,
	elem *parse.MarkupElement,
	attribute *parse.HyperscriptAttributeShorthand,
	chunkSource *parse.ParsedChunkSource,
) {

	//Add component in the result.

	component := &HyperscriptComponent{
		Name:               componentName,
		Element:            elem,
		AttributeShorthand: attribute,
		ChunkSource:        chunkSource,
	}

	a.result.HyperscriptComponents[componentName] = append(a.result.HyperscriptComponents[componentName], component)

	//Pre-analyze

	if attribute.HyperscriptParsingResult == nil {
		return
	}

	program := attribute.HyperscriptParsingResult.NodeData
	features, ok := hscode.GetProgramFeatures(program)
	if !ok {
		return
	}

	walk := func(node hscode.Map, inInit bool) {
		hscode.Walk(node, func(node hscode.Map, nodeType hscode.NodeType, parent hscode.Map, ancestorChain []hscode.Map, _ bool) (hscode.AstTraversalAction, error) {
			switch nodeType {
			case hscode.SetCommand:
				name, _ := hscode.GetSetCommandTargetName(node)
				if inInit && strings.HasPrefix(name, ":") {
					component.InitialElementScopeVarNames = append(component.InitialElementScopeVarNames, name)
				}
			}
			return hscode.ContinueAstTraversal, nil
		}, nil)
	}

	for _, feature := range features {
		feature := feature.(hscode.Map)
		switch hscode.GetTypeIfNode(feature) {
		case hscode.InitFeature: //init
			walk(feature, true)
		case hscode.OnFeature: //on
			onFeature := feature
			events, _ := hscode.GetOnFeatureEvents(onFeature)
			for _, event := range events {
				component.HandledEvents = append(component.HandledEvents, DOMEvent{
					Type: event.Name,
				})
			}
			walk(feature, false)
		}
	}
}
