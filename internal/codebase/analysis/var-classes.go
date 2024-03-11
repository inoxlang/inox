package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/parse"
)

type CssVariable struct {
	Name             CssVarName //example: "--primary-bg"
	AffectedProperty string     //example: "background", can be empty
	AutoRuleset      css.Node   //empty if not property is affected
}

// Example: "--primary-bg"
type CssVarName string

func addCssVariables(stylesheet css.Node, result *Result) {

	css.WalkAST(stylesheet, func(node, parent css.Node, ancestorChain []css.Node, after bool) (css.AstTraversalAction, error) {
		if node.Type == css.CustomProperty {
			varname := CssVarName(node.Data)

			if _, ok := result.CssVariables[varname]; !ok {
				cssVar := getCssVar(varname)
				result.CssVariables[cssVar.Name] = cssVar
			}
		}

		return css.ContinueAstTraversal, nil
	}, nil)
}

func getCssVar(name CssVarName) CssVariable {
	varname := string(name)
	parts := strings.Split(varname[2:], "-")
	cssVar := CssVariable{
		Name: CssVarName(varname),
	}

	for _, part := range parts {
		switch part {
		case "bg", "background":
			cssVar.AffectedProperty = "background"
		case "fg", "foreground":
			cssVar.AffectedProperty = "color"
		}
	}

	if cssVar.AffectedProperty != "" {
		cssVar.AutoRuleset = css.Node{
			Type: css.Ruleset,
			Children: []css.Node{
				css.MakeClassNameSelector(varname),
				css.MakeDeclaration(cssVar.AffectedProperty, css.MakeVarCall(varname)),
			},
		}
	}
	return cssVar

}

func addUsedVarBasedCssClasses(classAttributeValue parse.Node, result *Result) {
	attrValue := ""

	switch v := classAttributeValue.(type) {
	case *parse.DoubleQuotedStringLiteral:
		attrValue = v.Value
	case *parse.MultilineStringLiteral:
		attrValue = v.Value
		//TODO: support string templates
	default:
		return
	}

	classNames := strings.Split(attrValue, " ")
	for _, name := range classNames {
		name = strings.TrimSpace(name)

		if strings.HasPrefix(name, "--") {
			varname := CssVarName(name)
			result.UsedVarBasedCssRules[varname] = getCssVar(varname)
		}
	}
}
