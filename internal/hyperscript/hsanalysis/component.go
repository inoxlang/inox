package hsanalysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

type Component struct {
	Name                          string
	Element                       *parse.MarkupElement
	ClosestMarkupExpr             *parse.MarkupExpression
	AttributeShorthand            *parse.HyperscriptAttributeShorthand
	ChunkSource                   *parse.ParsedChunkSource
	HandledEvents                 []DOMEvent
	InitialElementScopeVarNames   []string // example: {":a", ":b"}
	InitializedDataAttributeNames []string // data-xxx attributes that are properly initialized, example: {"data-count", "data-x"}
}

type DOMEvent struct {
	Type string
}

func GetHyperscriptComponentName(markupElement *parse.MarkupElement) (name string, isComponent bool) {

	componentClassName := ""
	hasHyperscriptAttributeShorthand := false

	//Determine if the element is the root of a hyperscript component.
	for _, attr := range markupElement.Opening.Attributes {

		if componentClassName == "" {
			if attr, ok := attr.(*parse.MarkupAttribute); ok {
				if attr.IsNameEqual("class") && css.DoesClassListStartWithUppercaseLetter(attr.ValueIfStringLiteral()) {
					componentClassName = utils.MustGet(css.GetFirstClassNameInList(attr.ValueIfStringLiteral()))
				}
			}
		}

		if !hasHyperscriptAttributeShorthand {
			if _, ok := attr.(*parse.HyperscriptAttributeShorthand); ok {
				hasHyperscriptAttributeShorthand = true
			}
		}

		if componentClassName != "" && hasHyperscriptAttributeShorthand {
			break
		}
	}

	isComponent = componentClassName != "" && hasHyperscriptAttributeShorthand
	if isComponent {
		name = componentClassName
	}
	return
}

func IsHyperscriptComponent(markupElement *parse.MarkupElement) bool {
	_, ok := GetHyperscriptComponentName(markupElement)
	return ok
}

func PreanalyzeHyperscriptComponent(
	componentName string,
	elem *parse.MarkupElement,
	closestMarkupExpr *parse.MarkupExpression,
	attribute *parse.HyperscriptAttributeShorthand,
	chunkSource *parse.ParsedChunkSource,
) (component *Component) {

	component = &Component{
		Name:               componentName,
		Element:            elem,
		ClosestMarkupExpr:  closestMarkupExpr,
		AttributeShorthand: attribute,
		ChunkSource:        chunkSource,
	}

	if attribute.HyperscriptParsingResult == nil {
		return
	}

	//Pre-analyze Hyperscript attribute shorthand.

	program := attribute.HyperscriptParsingResult.NodeData
	features, ok := hscode.GetProgramFeatures(program)
	if !ok {
		return
	}

	walk := func(node hscode.JSONMap, inInit bool) {
		hscode.Walk(node, func(node hscode.JSONMap, nodeType hscode.NodeType, _ hscode.JSONMap, _ hscode.NodeType, _ []hscode.JSONMap, _ bool) (hscode.AstTraversalAction, error) {
			switch nodeType {
			case hscode.SetCommand:
				target, _ := hscode.GetSetCommandTarget(node)
				switch hscode.GetTypeIfNode(target) {
				case hscode.Symbol:
					name := hscode.GetSymbolName(target)
					if inInit && strings.HasPrefix(name, ":") && !slices.Contains(component.InitialElementScopeVarNames, name) {
						component.InitialElementScopeVarNames = append(component.InitialElementScopeVarNames, name)
					}
				case hscode.AttributeRef:
					name := hscode.GetAttributeRefName(target)
					if inInit && strings.HasPrefix(name, "data-") && !slices.Contains(component.InitializedDataAttributeNames, name) {
						component.InitializedDataAttributeNames = append(component.InitializedDataAttributeNames, name)
					}
				}

			}
			return hscode.ContinueAstTraversal, nil
		}, nil)
	}

	for _, feature := range features {
		feature := feature.(hscode.JSONMap)
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

	return
}
