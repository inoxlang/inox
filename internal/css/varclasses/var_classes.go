package varclasses

import (
	"strings"

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

	//Note for the future: the defaults (substring -> affected property) mapping rules should be kept relatively simple.
	//TODO: add a configuration file allowing the developers to define custom rules.

	//===== Background =====

	if strings.Contains(varname, "bg") || strings.Contains(varname, "background") {

		for _, cssPropName := range []string{
			"attachment",
			"blend-mode",
			"clip",
			"origin",
			"position",
			"repeat",
			"size",
		} {
			if strings.Contains(varname, "background-"+cssPropName) || strings.Contains(varname, "bg-"+cssPropName) {
				cssVar.AffectedProperty = cssPropName
				goto end
			}
		}

		for _, propName := range []string{"background-color", "bg-color", "color-bg"} {
			if strings.Contains(varname, propName) {
				cssVar.AffectedProperty = "background-color"
				goto end
			}
		}

		for _, propName := range []string{"background-image", "background-img", "bg-image", "bg-img", "image-bg", "img-bg"} {
			if strings.Contains(varname, propName) {
				cssVar.AffectedProperty = "background-image"
				goto end
			}
		}

		for _, propName := range []string{"background", "bg-", "-bg"} {
			if strings.Contains(varname, propName) {
				cssVar.AffectedProperty = "background"
				goto end
			}
		}

	}

	//===== Font and text =====

	if strings.Contains(varname, "font") ||
		strings.Contains(varname, "fg") ||
		strings.Contains(varname, "foreground") ||
		strings.Contains(varname, "text") ||
		strings.Contains(varname, "fs") ||
		strings.Contains(varname, "fw") ||
		strings.Contains(varname, "ts") {

		for _, propName := range []string{"font-color", "text-color", "fg-", "-fg", "foreground"} {
			if strings.Contains(varname, propName) {
				cssVar.AffectedProperty = "color"
				goto end
			}
		}

		for _, propName := range []string{"font-size", "text-size", "fs-", "-fs", "ts-", "-ts"} {
			if strings.Contains(varname, propName) {
				cssVar.AffectedProperty = "font-size"
				goto end
			}
		}

		for _, propName := range []string{"font-weight", "text-weight", "fw-", "-fw"} {
			/* Note: Never add `tw-`, it's a common prefix for tailwind. */
			if strings.Contains(varname, propName) {
				cssVar.AffectedProperty = "font-weight"
				goto end
			}
		}

	}

end:

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
