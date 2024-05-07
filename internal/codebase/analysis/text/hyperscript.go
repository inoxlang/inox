package text

import "fmt"

const (
	VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD = "this variable is not in the element scope of the element referenced by the tell command"

	ELEMENT_SCOPE_VARS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT = "element scope variables are not allowed here because there is no component"
	ATTR_REFS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT          = "attribute references are not allowed here because there is no component"
)

func FmtElementScopeVarMayNotBeDefined(name string, inComponentCtx bool) string {
	ctxInfo := ""
	if inComponentCtx {
		ctxInfo = " (from component)"
	}

	return fmt.Sprintf("element-scoped variable `%s`%s may not be defined", name, ctxInfo)
}
