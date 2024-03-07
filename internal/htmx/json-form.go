package htmx

import "regexp"

const (
	JSONFORM_EXT_NAME = "json-form"
)

var (
	JSONFORM_SHORTHAND_ATTRIBUTE_PATTERN = regexp.MustCompile("hx-(post|patch|put)-json")
)
