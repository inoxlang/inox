package html_ns

import (
	"slices"

	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

func StripUntrustedHyperscriptAttributes(node *html.Node) {
	walkHTMLNode(node, func(n *html.Node) error {
		n.Attr = slices.DeleteFunc(n.Attr, func(e html.Attribute) bool {
			return e.Key == inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME && !utils.SameIdentityStrings(e.Key, trustedTyperscriptAttrName)
		})
		return nil
	}, 0)
}
