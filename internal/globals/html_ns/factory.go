package html_ns

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/htmldata"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

var (
	trustedHyperscriptAttrName  string
	trustedScriptElementTagName string
)

func init() {
	b := strings.Builder{}
	b.WriteString(inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME)
	trustedHyperscriptAttrName = b.String()

	b.Reset()
	b.WriteString("script")
	trustedScriptElementTagName = b.String()

	if utils.SameIdentityStrings(trustedHyperscriptAttrName, inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME) {
		//Guard against any future optimization.
		panic("")
	}

	if utils.SameIdentityStrings(trustedScriptElementTagName, "script") {
		//Guard against any future optimization.
		panic("")
	}
}

func CreateHTMLNodeFromMarkupElement(ctx *core.Context, arg *core.NonInterpretedMarkupElement) *HTMLNode {
	transformUntrusted := true
	return createHTMLNodeFromMarkupElement(ctx, arg, transformUntrusted)
}

func createHTMLNodeFromMarkupElement(ctx *core.Context, arg *core.NonInterpretedMarkupElement, transformUntrusted bool) *HTMLNode {
	children := arg.Children()
	childNodes := make([]*HTMLNode, 0, len(children))

	rawContent := arg.RawContent() //content inside <script>, <style> tags.
	if rawContent != "" {
		childNodes = append(childNodes, CreateTextNode(core.String(rawContent)))
	}

	for _, child := range children {
		createChildNodesFromValue(ctx, child, &childNodes)
	}

	tagName := arg.Name()
	if tagName == "script" {
		//The script element is trusted because it is created from Inox markup.
		tagName = trustedScriptElementTagName
	}

	attributes := getAttributes(arg)

	node := NewNodeFromGoDescription(NodeDescription{
		Tag:                    tagName,
		Children:               childNodes,
		Attributes:             attributes,
		DoNoTransformUntrusted: !transformUntrusted,
	})

	if tagName == "html" {
		return NewHTML5DocumentNodeFromGoDescription(HTML5DocumentDescription{
			HtmlTagNode: node,
		})
	}
	return node
}

func getAttributes(arg *core.NonInterpretedMarkupElement) (attributes []html.Attribute) {
	attributes = make([]html.Attribute, 0, len(arg.Attributes()))
	tagName := arg.Name()

	for _, attr := range arg.Attributes() {
		attrName := attr.Name()

		//handle pseudo htmx attributes
		if htmldata.IsPseudoHtmxAttribute(attrName) {
			equivalentAttrs, count, err := htmldata.GetEquivalentsToPseudoHtmxAttribute(attr.Name(), attr.Value())

			//TODO: handle errors
			if err != nil {
				panic(err)
			}
			attributes = append(attributes, equivalentAttrs[:count]...)
			continue
		}

		switch attrName {
		case inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME:
			if attr.CreatedFromHyperscriptAttributeShorthand() {
				attrName = trustedHyperscriptAttrName
			} else {
				//Do not add untrusted '_' attributes.
				continue
			}
		case inoxjs.FOR_LOOP_ATTR_NAME:
			//Disable Hyperscript scripting for template elements.
			//https://hyperscript.org/docs/#security
			attributes = append(attributes, html.Attribute{
				Key: inoxconsts.HYPERSCRIPT_DISABLED_SCRIPTING_ATTR_NAME,
				Val: "",
			})
		}

		attrValue := attr.Value()

		//turn "h" attribute in script elements into type=<hyperscript media type>
		if tagName == "script" && attrName == inoxconsts.HYPERSCRIPT_SCRIPT_MARKER {
			attrName = "type"
			attrValue = core.String(mimeconsts.HYPERSCRIPT_CTYPE)
		}

		attributes = append(attributes, html.Attribute{Key: attrName})
		index := len(attributes) - 1

		switch val := attrValue.(type) {
		case core.StringLike:
			attributes[index].Val = val.GetOrBuildString()
		case core.GoString:
			attributes[index].Val = val.UnderlyingString()
		case core.Int:
			attributes[index].Val = strconv.FormatInt(int64(val), 10)
		default:
			panic(fmt.Errorf("failed to convert value of attribute '%s' to string", attrName))
		}
	}

	return
}

func createChildNodesFromValue(ctx *core.Context, child core.Value, childNodes *[]*HTMLNode) {
	switch c := child.(type) {
	case *core.NonInterpretedMarkupElement:
		transformUntrusted := false
		*childNodes = append(*childNodes, createHTMLNodeFromMarkupElement(ctx, c, transformUntrusted))
	case *HTMLNode:
		if c.HasParent() {
			panic(core.ErrUnreachable)
		}
		*childNodes = append(*childNodes, c)
	case core.StringLike:
		*childNodes = append(*childNodes, CreateTextNode(c))
	case core.GoString:
		*childNodes = append(*childNodes, CreateTextNode(core.String(c.UnderlyingString())))
	case core.Int:
		*childNodes = append(*childNodes, CreateTextNode(core.String(strconv.FormatInt(int64(c), 10))))
	case *core.List:
		length := c.Len()
		for i := 0; i < length; i++ {
			elem := c.At(ctx, i)
			createChildNodesFromValue(ctx, elem, childNodes)
		}
	default:
		panic(core.ErrUnreachable)
	}
}
