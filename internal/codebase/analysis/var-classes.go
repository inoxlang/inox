package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/css/varclasses"
	"github.com/inoxlang/inox/internal/parse"
)

func addCssVariables(stylesheet css.Node, result *Result) {

	css.WalkAST(stylesheet, func(node, parent css.Node, ancestorChain []css.Node, after bool) (css.AstTraversalAction, error) {
		if node.Type == css.CustomProperty {
			varname := css.VarName(node.Data)

			if _, ok := result.CssVariables[varname]; !ok {
				cssVar := varclasses.GetByVarname(varname)
				result.CssVariables[cssVar.Name] = cssVar
			}
		}

		return css.ContinueAstTraversal, nil
	}, nil)
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
			varname := css.VarName(name)
			result.UsedVarBasedCssRules[varname] = varclasses.GetByVarname(varname)
		}
	}
}
