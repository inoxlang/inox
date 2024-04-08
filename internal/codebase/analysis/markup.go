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

	result := a.result

	//Tailwind
	if ident.Name == "class" {
		addUsedTailwindRulesets(markupAddr.Value, result)
		addUsedVarBasedCssClasses(markupAddr.Value, result)
		return
	}

	//HTMX
	if strings.HasPrefix(ident.Name, "hx-") {
		addUsedHtmxExtensions(markupAddr, ident.Name, result)
		return
	}

}

func (a *analyzer) preAnalyzeMarkupElement(node *parse.MarkupElement) {
	result := a.result

	switch node.EstimatedRawElementType {
	case parse.HyperscriptScript:
		a.addUsedHyperscriptFeaturesAndCommands(node)
	case parse.JsScript:
		if SURREAL_DETECTION_PATTERN.MatchString(node.RawElementContent) && !result.IsSurrealUsed {
			result.IsSurrealUsed = true
			result.UsedInoxJsLibs = append(result.UsedInoxJsLibs, inoxjs.SURREAL_LIB_NAME)
		}
		if PREACT_SIGNALS_DETECTION_PATTERN.MatchString(node.RawElementContent) && !result.IsPreactSignalsLibUsed {
			result.IsPreactSignalsLibUsed = true
			result.UsedInoxJsLibs = append(result.UsedInoxJsLibs, inoxjs.PREACT_SIGNALS_LIB_NAME)
		}
		if strings.Contains(node.RawElementContent, inoxjs.INIT_COMPONENT_FN_NAME+"(") && !result.IsPreactSignalsLibUsed {
			result.IsInoxComponentLibUsed = true
			result.UsedInoxJsLibs = append(result.UsedInoxJsLibs, inoxjs.INOX_COMPONENT_LIB_NAME)
		}
	case parse.CssStyleElem:
		if CSS_SCOPE_INLINE_DETECTION_PATTERN.MatchString(node.RawElementContent) && !result.IsCssScopeInlineUsed {
			result.IsCssScopeInlineUsed = true
			result.UsedInoxJsLibs = append(result.UsedInoxJsLibs, inoxjs.CSS_INLINE_SCOPE_LIB_NAME)
		}
	}
}
