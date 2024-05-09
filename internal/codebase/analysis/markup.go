package analysis

import (
	"regexp"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
)

var (
	CSS_SCOPE_INLINE_DETECTION_PATTERN = regexp.MustCompile(`(\b|^\s*)me\b`)
	SURREAL_DETECTION_PATTERN          = regexp.MustCompile(`(\b|^\s*)(me|any)\(`)
	PREACT_SIGNALS_DETECTION_PATTERN   = regexp.MustCompile(`(signal|computed|effect|batch|untracked)\(`)
)

func (a *analyzer) preAnalyzeMarkupAttribute(markupAddr *parse.MarkupAttribute) {

	ident, ok := markupAddr.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	name := ident.Name
	result := a.result

	//InoxJS
	switch {
	case inoxjs.ContainsClientSideInterpolation(markupAddr.ValueIfStringLiteral()):
		a.result.ClientSideInterpolationsFound = true
		fallthrough
	case name == inoxjs.CONDITIONAL_DISPLAY_ATTR_NAME ||
		name == inoxjs.FOR_LOOP_ATTR_NAME:
		a.result.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
		a.result.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
	}

	//Tailwind
	if name == "class" {
		addUsedTailwindRulesets(markupAddr.Value, result)
		addUsedVarBasedCssClasses(markupAddr.Value, result)
		return
	}

	//HTMX
	if strings.HasPrefix(name, "hx-") {
		addUsedHtmxExtensions(markupAddr, name, result)
		return
	}

}

func (a *analyzer) preAnalyzeMarkupElement(markupElement *parse.MarkupElement, ancestorChain []parse.Node, sourcedChunk *parse.ParsedChunkSource) error {
	result := a.result

	switch markupElement.EstimatedRawElementType {
	case parse.HyperscriptScript:
		a.addUsedHyperscriptFeaturesAndCommands(markupElement)
		return nil
	case parse.JsScript:
		if SURREAL_DETECTION_PATTERN.MatchString(markupElement.RawElementContent) {
			result.UsedInoxJsLibs[inoxjs.SURREAL_LIB_NAME] = struct{}{}
		}
		if PREACT_SIGNALS_DETECTION_PATTERN.MatchString(markupElement.RawElementContent) {
			result.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
		}
		if strings.Contains(markupElement.RawElementContent, inoxjs.INIT_COMPONENT_FN_NAME+"(") {
			result.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
		}
		return nil
	case parse.CssStyleElem:
		if CSS_SCOPE_INLINE_DETECTION_PATTERN.MatchString(markupElement.RawElementContent) {
			result.UsedInoxJsLibs[inoxjs.CSS_INLINE_SCOPE_LIB_NAME] = struct{}{}
		}
		return nil
	}

	attrShorthand, _ := markupElement.HyperscriptAttributeShorthand()

	if attrShorthand != nil {
		a.addUsedHyperscriptFeaturesAndCommands(attrShorthand)
	}

	isComponentBecauseOfAttributes, introducedElemScopedVarNames, inoxjsErrors, err := inoxjs.AnalyzeInoxJsAttributes(a.ctx, markupElement, sourcedChunk)
	if err != nil {
		return err
	}

	for _, err := range inoxjsErrors {
		if err.IsHyperscriptParsingError {
			hyperscriptError := hsanalysis.MakeError(err.Message, err.Location)
			a.result.addHyperscriptErrors(hyperscriptError)
		} else {
			a.result.addInoxJsError(err)
		}
	}

	if attrShorthand != nil || isComponentBecauseOfAttributes {

		a.addUsedHyperscriptFeaturesAndCommands(markupElement)

		closestMarkupExpr, _, ok := parse.FindClosest(ancestorChain, (*parse.MarkupExpression)(nil))
		if !ok {
			return nil
		}

		componentName, hasComponentName := hsanalysis.GetHyperscriptComponentName(markupElement)
		if !hasComponentName {
			return nil
		}

		component := hsanalysis.PreanalyzeHyperscriptComponent(componentName, markupElement, closestMarkupExpr, attrShorthand, sourcedChunk)
		a.result.HyperscriptComponents[componentName] = append(a.result.HyperscriptComponents[componentName], component)

		for _, varname := range introducedElemScopedVarNames {
			if !slices.Contains(component.InitialElementScopeVarNames, varname) {
				component.InitialElementScopeVarNames = append(component.InitialElementScopeVarNames, varname)
			}
		}

	}

	return nil
}
