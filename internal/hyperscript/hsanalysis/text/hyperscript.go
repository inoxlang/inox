package text

import "fmt"

const (
	VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD = "this variable is not in the element scope of the element referenced by the tell command"
	ATTR_NOT_REF_TO_ATTR_OF_ELEM_REF_BY_TELL_CMD  = "this attribute does not refer to an attribute of the element referenced by the tell command"

	ELEMENT_SCOPE_VARS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT = "element scope variables are not allowed here because there is no component"
	ATTR_REFS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT          = "attribute references are not allowed here because there is no component"

	BEHAVIOR_CAN_ONLY_BE_INSTALLED_FROM_HS_ATTR_OR_BEHAVIOR = "behaviors can only be installed from an Hyperscript attribute or behavior"

	BEHAVIORS_SHOULD_BE_DEFINED_IN_HS_FILES = "behaviors should be defined in Hyperscript files or <script> elements"
)

func FmtElementScopeVarMayNotBeDefined(name string, inComponentCtx bool) string {
	ctxInfo := ""
	if inComponentCtx {
		ctxInfo = " (from component)"
	}

	return fmt.Sprintf("element-scoped variable `%s`%s may not be defined", name, ctxInfo)
}

func FmtAttributeMayNotBeInitialized(name string, inComponentCtx bool) string {
	ctxInfo := ""
	if inComponentCtx {
		ctxInfo = " (from component)"
	}

	return fmt.Sprintf("attribute `%s`%s may not be initialized", name, ctxInfo)
}
