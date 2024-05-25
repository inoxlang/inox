package html_ns

import (
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TransformUntrustedScriptsAndHyperscriptAttributes(node *html.Node) {
	walkHTMLNode(node, func(n *html.Node) error {
		//Turn untrusted <script> elements into <div>s
		if n.Data == "script" && !utils.SameIdentityStrings(n.Data, trustedScriptElementTagName) && n.Type == html.ElementNode {
			n.Data = "div"
			n.DataAtom = atom.Div
			//Do not return early because the element may have attributes that should be removed.
		}

		// //Strip untrusted '_' attributes.
		// n.Attr = slices.DeleteFunc(n.Attr, func(e html.Attribute) bool {
		// 	return e.Key == inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME && !utils.SameIdentityStrings(e.Key, trustedHyperscriptAttrName)
		// })
		return nil
	}, 0)
}
