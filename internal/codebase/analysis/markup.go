package analysis

import (
	"regexp"
	"strings"

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

	if name == inoxjs.CONDITIONAL_DISPLAY_ATTR_NAME {
		a.result.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
		a.result.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
	}

}

func (a *analyzer) preAnalyzeMarkupElement(node *parse.MarkupElement) {
	result := a.result

	switch node.EstimatedRawElementType {
	case parse.HyperscriptScript:
		a.addUsedHyperscriptFeaturesAndCommands(node)
	case parse.JsScript:
		if SURREAL_DETECTION_PATTERN.MatchString(node.RawElementContent) {
			result.UsedInoxJsLibs[inoxjs.SURREAL_LIB_NAME] = struct{}{}
		}
		if PREACT_SIGNALS_DETECTION_PATTERN.MatchString(node.RawElementContent) {
			result.UsedInoxJsLibs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
		}
		if strings.Contains(node.RawElementContent, inoxjs.INIT_COMPONENT_FN_NAME+"(") {
			result.UsedInoxJsLibs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
		}
	case parse.CssStyleElem:
		if CSS_SCOPE_INLINE_DETECTION_PATTERN.MatchString(node.RawElementContent) {
			result.UsedInoxJsLibs[inoxjs.CSS_INLINE_SCOPE_LIB_NAME] = struct{}{}
		}
	}
}
