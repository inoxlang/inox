package hsanalysis

import (
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
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
	Installs                      []*InstallFeature
	AppliedInstalls               []*InstallFeature

	//Note: applying an install updates InitialElementScopeVarNames and InitializedDataAttributeNames.
}

type DOMEvent struct {
	Type string
}

func GetHyperscriptComponentName(markupElement *parse.MarkupElement) (name string, hasComponentName bool) {

	componentClassName := ""

	//Determine if the element is the root of a hyperscript component.
	for _, attr := range markupElement.Opening.Attributes {

		if attr, ok := attr.(*parse.MarkupAttribute); ok {
			if attr.IsNameEqual("class") && css.DoesClassListStartWithUppercaseLetter(attr.ValueIfStringLiteral()) {
				componentClassName = utils.MustGet(css.GetFirstClassNameInList(attr.ValueIfStringLiteral()))
				break
			}
		}

	}

	hasComponentName = componentClassName != ""
	if hasComponentName {
		name = componentClassName
	}
	return
}

func LooksLikeHyperscriptComponent(markupElement *parse.MarkupElement) bool {
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

	if attribute == nil || attribute.HyperscriptParsingResult == nil {
		return
	}

	//Pre-analyze Hyperscript attribute shorthand.

	program := attribute.HyperscriptParsingResult.NodeData
	features, ok := hscode.GetProgramFeatures(program)
	if !ok {
		return
	}

	preAnalyzeFeaturesOfBehaviorOrComponent(
		&component.InitialElementScopeVarNames,
		&component.InitializedDataAttributeNames,
		&component.HandledEvents,
		&component.Installs,
		features,
	)

	return
}
