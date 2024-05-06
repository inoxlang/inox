package inoxjs

import "strings"

const (
	TEXT_INTERPOLATION_OPENING_DELIMITER = "(("
	TEXT_INTERPOLATION_CLOSING_DELIMITER = "))"
	CONDITIONAL_DISPLAY_ATTR_NAME        = "x-if"
	FOR_LOOP_ATTR_NAME                   = "x-for"
	INIT_COMPONENT_FN_NAME               = "initComponent"
)

func ContainsClientSideInterpolation(s string) bool {
	openingDelimIndex := strings.Index(s, TEXT_INTERPOLATION_OPENING_DELIMITER)
	if openingDelimIndex < 0 {
		return false
	}
	closingDelimIndex := strings.Index(s, TEXT_INTERPOLATION_CLOSING_DELIMITER)
	return closingDelimIndex > openingDelimIndex
}
