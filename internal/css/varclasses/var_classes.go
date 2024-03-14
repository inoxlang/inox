package varclasses

import (
	"github.com/inoxlang/inox/internal/css"
)

type Variable struct {
	Name             css.VarName //example: "--primary-bg"
	AffectedProperty string      //example: "background", can be empty
	AutoRuleset      css.Node    //empty if not property is affected
}

func GetByVarname(name css.VarName) Variable {
	varname := string(name)
	cssVar := Variable{
		Name: css.VarName(varname),
	}

	affectedProperty := inferAffectedProperty(varname)

	if affectedProperty != "" {
		cssVar.AffectedProperty = affectedProperty
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
