package internal

import (
	"fmt"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

func CreateHTMLNodeFromXMLElement(ctx *core.Context, arg *core.XMLElement) *HTMLNode {

	children := arg.Children()

	childNodes := make([]*HTMLNode, len(children))

	for i, child := range children {
		switch c := child.(type) {
		case *core.XMLElement:
			childNodes[i] = CreateHTMLNodeFromXMLElement(ctx, c)
		case core.StringLike:
			childNodes[i] = CreateTextNode(c)
		default:
			panic(core.ErrUnreachable)
		}
	}

	attributes := make([]html.Attribute, len(arg.Attributes()))

	for i, attr := range arg.Attributes() {
		attributes[i].Key = attr.Name()

		switch val := attr.Value().(type) {
		case core.StringLike:
			attributes[i].Val = val.GetOrBuildString()
		case core.Int:
			attributes[i].Val = utils.BytesAsString(core.GetRepresentation(val, ctx))
		default:
			panic(fmt.Errorf("failed to convert value of attribute '%s' to string", attr.Name()))
		}
	}

	return NewNodeFromGoDescription(NodeDescription{
		Tag:        arg.Name(),
		Children:   childNodes,
		Attributes: attributes,
	})
}
