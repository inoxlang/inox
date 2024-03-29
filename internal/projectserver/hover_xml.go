package projectserver

import (
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/css/varclasses"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hshelp"
	"github.com/inoxlang/inox/internal/parse"
)

func getTagOrAttributeHoverHelp(
	node parse.Node,
	ancestors []parse.Node,
	cursorIndex int32,
	hoverContentParams hoverContentParams,
) (result string, shouldSpecificValBeIgnored bool, hasResult bool) {
	if len(ancestors) < 3 {
		return
	}

	xmlExpr, _, ok := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !ok {
		return
	}

	var ident *parse.IdentifierLiteral

	switch n := node.(type) {
	case *parse.DoubleQuotedStringLiteral:
		//Determine if the string is the value of an attribute.
		parent := ancestors[len(ancestors)-1]
		xmlAttr, ok := parent.(*parse.XMLAttribute)
		if !ok {
			return
		}
		return getAttributeValueHoverHelp(n, xmlAttr, xmlExpr, ancestors, cursorIndex, hoverContentParams)
	case *parse.IdentifierLiteral:
		ident = n
	default:
		return
	}

	//Help for tag or attribute name.

	var (
		attribute   *parse.XMLAttribute
		openingElem *parse.XMLOpeningElement
		parent      parse.Node
		tagIdent    *parse.IdentifierLiteral
	)

	parent = ancestors[len(ancestors)-1]
	attribute, ok = parent.(*parse.XMLAttribute)

	if ok {
		openingElem, ok = ancestors[len(ancestors)-2].(*parse.XMLOpeningElement)
		if !ok { //invalid state
			return
		}
		tagIdent, ok = openingElem.Name.(*parse.IdentifierLiteral)
		if !ok { //parsing error
			return
		}
	} else {
		openingElem, ok = parent.(*parse.XMLOpeningElement)
		if !ok { //invalid state
			return
		}

		if ident != openingElem.Name {
			return
		}
		tagIdent = ident
	}

	namespaceName := xmlExpr.EffectiveNamespaceName()

	//TODO: use symbolic data in order to support aliases
	switch namespaceName {
	case "html":

		if parent == openingElem {
			tagData, ok := html_ns.GetTagData(tagIdent.Name)
			if ok {
				return tagData.DescriptionContent(), false, true
			}
		} else if parent == attribute {

			//Get data for standard attributes.

			attributes, ok := html_ns.GetAllTagAttributes(tagIdent.Name)
			if !ok {
				break
			}

			attrName := ident.Name

			for _, attr := range attributes {
				if attr.Name == attrName {
					result = attr.DescriptionContent()
					hasResult = true
					return
				}
			}
		}

	}

	return
}

func getAttributeValueHoverHelp(
	node parse.Node,
	parent *parse.XMLAttribute,
	xmlExpr *parse.XMLExpression,
	ancestors []parse.Node,
	index int32,
	hoverContentParams hoverContentParams,
) (help string, shouldSpecificValBeIgnored bool, hasResult bool) {

	attrNameIdent, ok := parent.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	//Only help for the values of HTML attributes is supported for now.
	if xmlExpr.EffectiveNamespaceName() != globalnames.HTML_NS {
		return
	}

	attrName := attrNameIdent.Name

	switch attrName {
	case "class":
		help = getCssClassHoverHelp(node, index, hoverContentParams)
		hasResult = help != ""
		shouldSpecificValBeIgnored = hasResult
	}

	return
}

func getHyperscriptHelpMarkdown(attribute *parse.HyperscriptAttributeShorthand, span parse.NodeSpan) string {
	parsingResult := attribute.HyperscriptParsingResult
	cursorIndexInHsCode := span.Start - attribute.Span.Start - 1
	return hshelp.GetHoverHelpMarkdown(parsingResult.Tokens, cursorIndexInHsCode)
}

func getCssClassHoverHelp(attrValue parse.Node, index int32, hoverContentParams hoverContentParams) string {
	help := ""

	//Determine the hovered class name.

	quotedStrLit, ok := attrValue.(*parse.DoubleQuotedStringLiteral)
	if !ok {
		return ""
	}

	cut, ok := parse.CutQuotedStringLiteral(index, quotedStrLit)
	if !ok {
		return ""
	}

	if cut.HasSpaceAfterIndex || cut.HasSpaceBeforeIndex || cut.IsIndexAtStart || cut.IsIndexAtEnd {
		return ""
	}

	leftNamePart := ""
	if index := strings.LastIndex(cut.BeforeIndex, " "); index > 0 {
		leftNamePart = cut.BeforeIndex[index+1:]
	} else {
		leftNamePart = cut.BeforeIndex
	}

	rightNamePart := ""
	if index := strings.Index(cut.AfterIndex, " "); index > 0 {
		rightNamePart = cut.AfterIndex[:index]
	} else {
		rightNamePart = cut.AfterIndex
	}

	className := leftNamePart + rightNamePart

	ruleset, ok := tailwind.GetBaseRuleset("." + className)
	if ok {
		help += fmt.Sprintf("```css\n%s\n```", ruleset.Ruleset.String())
	} else if analysis := hoverContentParams.lastCodebaseAnalysis; analysis != nil {

		if css.HasValidVarNamePrefix(className) {
			varname := css.VarName(className)

			cssVar, isDefined := analysis.CssVariables[varname]
			isUsed := isDefined

			if !isDefined {
				cssVar, isUsed = analysis.UsedVarBasedCssRules[varname]
			}

			if isUsed {
				if cssVar.AffectedProperty == "" {
					help += varclasses.FmtNoAssociatedRuleset(varname)
				} else {
					if !isDefined {
						help += fmt.Sprintf(
							"_The utility has been generated but the CSS variable `%s` is not defined in the codebase. "+
								"This is fine if the variable is provided externally._\n", varname)
					}
					help += fmt.Sprintf("```css\n%s\n```", cssVar.AutoRuleset.String())
				}
			}
		}
	}

	return help
}

func getRawXMLelementContentHelpMarkdown(element *parse.XMLElement, span parse.NodeSpan) string {
	switch parsingResult := element.RawElementParsingResult.(type) {
	case *hscode.ParsingResult:
		cursorIndexInHsCode := span.Start - element.RawElementContentStart
		return hshelp.GetHoverHelpMarkdown(parsingResult.Tokens, cursorIndexInHsCode)
	}

	return ""
}
