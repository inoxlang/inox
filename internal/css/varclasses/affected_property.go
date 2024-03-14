package varclasses

import (
	"errors"
	"strings"
)

var (
	//Base names of 'background-XXX' properties that have no diminutives.
	BACKGROUND_PROP_BASES_WITHOUT_DIMINUTIVES = []string{
		"attachment",
		"blend-mode",
		"clip",
		"origin",
		"position",
		"repeat",
		"size",
	}

	//Base names of 'background-image-XXX' properties.
	BORDER_IMAGE_PROP_BASES = []string{
		"outset",
		"repeat",
		"slice",
		"source",
		"width",
	}

	BORDER_TYPES = []string{
		//The names are listed in a way that makes it possible to determine the border type present in a string by checking
		//for types at low indices first.

		"border-block-start",
		"border-block-end",
		"border-block",

		"border-inline-start",
		"border-inline-end",
		"border-inline",

		"border-left",
		"border-right",
		"border-top",
		"border-bottom",

		"border",
	}

	//Properties common to all border types.
	COMMON_BORDER_PROP_BASES = []string{
		"color",
		"style",
		"width",
	}

	BORDER_UNIQUE_PROPS = []string{
		"border-boundary",
		"border-collapse",
	}

	BORDER_RADIUS_PROPS = []string{
		"border-bottom-left-radius",
		"border-bottom-right-radius",

		"border-end-end-radius",
		"border-end-start-radius",

		"border-start-end-radius",
		"border-start-start-radius",

		"border-top-left-radius",
		"border-top-right-radius",
	}
)

func inferAffectedProperty(varname string) string {
	if !strings.HasPrefix(varname, "--") {
		panic(errors.New("missing prefix `--`"))
	}

	//Note for the future: the defaults (substring -> affected property) mapping rules should be kept relatively simple.
	//TODO: add a configuration file allowing the developers to define custom rules.

	//===== Background =====

	if strings.Contains(varname, "bg") || strings.Contains(varname, "background") {

		for _, baseName := range BACKGROUND_PROP_BASES_WITHOUT_DIMINUTIVES {
			if strings.Contains(varname, "background-"+baseName) || strings.Contains(varname, "bg-"+baseName) {
				return "background-" + baseName
			}
		}

		for _, propName := range []string{"background-color", "bg-color", "color-bg"} {
			if strings.Contains(varname, propName) {
				return "background-color"
			}
		}

		for _, substring := range []string{"background-image", "background-img", "bg-image", "bg-img", "image-bg", "img-bg"} {
			if strings.Contains(varname, substring) {
				return "background-image"
			}
		}

		for _, substring := range []string{"background", "bg-", "-bg"} {
			if strings.Contains(varname, substring) {
				return "background"
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

		for _, substring := range []string{"font-color", "text-color", "fg-", "-fg", "foreground"} {
			if strings.Contains(varname, substring) {
				return "color"
			}
		}

		for _, substring := range []string{"font-size", "text-size", "fs-", "-fs", "ts-", "-ts"} {
			if strings.Contains(varname, substring) {
				return "font-size"
			}
		}

		for _, substring := range []string{"font-weight", "text-weight", "fw-", "-fw"} {
			/* Note: Never add `tw-`, it's a common prefix for tailwind. */
			if strings.Contains(varname, substring) {
				return "font-weight"
			}
		}
	}

	//===== Border =====

	if strings.Contains(varname, "border-image") || strings.Contains(varname, "border-img") {

		for _, baseName := range BORDER_IMAGE_PROP_BASES {
			if strings.Contains(varname, "border-image-"+baseName) || strings.Contains(varname, "border-img-"+baseName) {
				return "border-image-" + baseName
			}
		}

		return "border-image"

	} else if strings.Contains(varname, "border") {

		for _, completePropName := range BORDER_UNIQUE_PROPS {
			if strings.Contains(varname, completePropName) {
				return completePropName
			}
		}

		for _, completePropName := range BORDER_RADIUS_PROPS {
			if strings.Contains(varname, completePropName) {
				return completePropName
			}
		}

		var actualBorderType string

		for _, borderType := range BORDER_TYPES {
			if strings.Contains(varname, borderType) {
				actualBorderType = borderType
				break
			}
		}

		for _, baseName := range COMMON_BORDER_PROP_BASES {
			completePropName := actualBorderType + "-" + baseName
			if strings.Contains(varname, completePropName) {
				return completePropName
			}
		}
	}

	return ""
}
