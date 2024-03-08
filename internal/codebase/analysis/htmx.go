package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/htmx"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

func addUsedHtmxExtensions(xmlAttr *parse.XMLAttribute, attrName string, result *Result) {

	switch {
	case htmx.JSONFORM_SHORTHAND_ATTRIBUTE_PATTERN.MatchString(attrName):
		extName := htmx.JSONFORM_EXT_NAME

		_, ok := result.UsedHtmxExtensions[extName]
		if !ok {
			result.UsedHtmxExtensions[extName] = struct{}{}
		}
	case attrName == "hx-ext":
		names := strings.Split(xmlAttr.ValueIfStringLiteral(), ",")
		names = utils.MapSlice(names, strings.TrimSpace)
		for _, extName := range names {
			_, ok := result.UsedHtmxExtensions[extName]
			if !ok {
				result.UsedHtmxExtensions[extName] = struct{}{}
			}
		}
	}

}
