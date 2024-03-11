package varclasses

import (
	"fmt"

	"github.com/inoxlang/inox/internal/css"
)

func FmtNoAssociatedRuleset(varname css.VarName) string {
	return fmt.Sprintf("The CSS variable `%s` has not associated ruleset because it does not affect any CSS property."+
		" You can learn about variable-based utilities here: https://github.com/inoxlang/inox/blob/main/docs/frontend-development/utility-classes.md#variable-based-utilities.", varname)
}
