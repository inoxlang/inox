package html_ns

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"

	_html_symbolic "github.com/inoxlang/inox/internal/html/symbolic"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	CLASS_KEY       = "class"
	ID_KEY          = "id"
	CHILDREN_KEY    = "children"
	ANCHOR_HREF_KEY = "href"
	MODEL_KEY       = "model"

	S_NODE_ALREADY_HAS_A_PARENT                    = "node that already has a parent"
	S_NODE_ALREADY_HAS_SIBLINGS                    = "node that already has siblings"
	S_CHILDREN_ALREADY_PROVIDED_WITH_CHILDREN_PROP = "children already provided with .children"
)

var (
	NODE_PATTERN = &core.TypePattern{
		Name:          "html.node",
		Type:          reflect.TypeOf(&HTMLNode{}),
		SymbolicValue: _html_symbolic.ANY_HTML_NODE,
	}
)

var tagnameToSpecificDescPropHandler = map[string]map[string]func(value core.Value, node *HTMLNode){
	"a": {
		ANCHOR_HREF_KEY: func(value core.Value, node *HTMLNode) {
			var href string
			switch val := value.(type) {
			case core.StringLike:
				s := val.GetOrBuildString()
				if s == "" {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(ANCHOR_HREF_KEY, "description", "empty string"))
				}
				href = s
			case core.Path:
				href = val.UnderlyingString()
			case core.URL:
				href = val.UnderlyingString()
			default:
				panic(core.FmtPropOfArgXShouldBeOfTypeY(ANCHOR_HREF_KEY, "description", "string or path or URL", val))
			}

			node.node.Attr = append(node.node.Attr, html.Attribute{Key: "href", Val: href})
		},
	},
}

func NewNode(ctx *core.Context, tag core.String, desc *core.Object) (finalNode *HTMLNode) {
	var class, id string
	var children []*HTMLNode

	it := desc.Iterator(ctx, core.IteratorConfiguration{})

	addChild := func(v core.Value) {
		var child *HTMLNode
		switch val := v.(type) {
		case core.StringLike:
			child = CreateTextNode(val)
		case *HTMLNode:
			child = val
		default:
			panic(fmt.Errorf("invalid child for html node: %#v", child))
		}
		children = append(children, child)
	}

	//first iteration: non-index keys
	for it.Next(ctx) {
		k := string(it.Key(ctx).(core.String))
		if k == inoxconsts.IMPLICIT_PROP_NAME {
			continue
		}

		v := it.Value(ctx)
		switch k {
		case CLASS_KEY:
			s, ok := v.(core.String)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(CLASS_KEY, "description", "string", v))
			}
			class = string(s)
		case ID_KEY:
			switch idVal := v.(type) {
			case core.String:
				id = string(idVal)
			case core.Int:
				id = strconv.Itoa(int(idVal))
			default:
				panic(core.FmtPropOfArgXShouldBeOfTypeY(ID_KEY, "description", "string or int", v))
			}
		case CHILDREN_KEY:
			iterable, ok := v.(core.Iterable)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(CHILDREN_KEY, "description", "iterable", v))
			}
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)
				strLike, ok := elem.(core.StringLike)
				if ok {
					addChild(strLike)
					continue
				}

				child, ok := elem.(*HTMLNode)
				if !ok {
					panic(core.FmtUnexpectedElementInPropIterableShowVal(elem, CHILDREN_KEY))
				}
				if child.node.Parent != nil {
					panic(commonfmt.FmtUnexpectedElementInPropIterable("children", S_NODE_ALREADY_HAS_A_PARENT))
				}

				if child.node.NextSibling != nil || child.node.PrevSibling != nil {
					panic(commonfmt.FmtUnexpectedElementInPropIterable("children", S_NODE_ALREADY_HAS_SIBLINGS))
				}

				addChild(child)
			}
		default:
			// handle description property specific to the node's tag

			handlers, ok := tagnameToSpecificDescPropHandler[string(tag)]
			if ok {
				if handler, ok := handlers[k]; ok {
					defer func() {
						if finalNode != nil {
							handler(v, finalNode)
						}
					}()
					continue
				}
			}
			panic(commonfmt.FmtUnexpectedPropInArgX(k, "description"))
		}
	}

	childrenAlreadyProvided := len(children) != 0

	desc.ForEachElement(ctx, func(index int, v core.Serializable) error {

		if childrenAlreadyProvided {
			panic(commonfmt.FmtUnexpectedElementAtIndexOfArgShowVal(index, "description", S_CHILDREN_ALREADY_PROVIDED_WITH_CHILDREN_PROP))
		}

		strLike, ok := v.(core.StringLike)
		if ok {
			addChild(strLike)
			return nil
		}

		childNode, ok := v.(*HTMLNode)
		if !ok {
			panic(core.FmtUnexpectedElementAtIndexOfArgShowVal(v, index, "description"))
		}

		if childNode.node.Parent != nil {
			panic(commonfmt.FmtUnexpectedElementAtIndexOfArgShowVal(index, "description", S_NODE_ALREADY_HAS_A_PARENT))
		}

		if childNode.node.NextSibling != nil || childNode.node.PrevSibling != nil {
			panic(commonfmt.FmtUnexpectedElementAtIndexOfArgShowVal(index, "description", S_NODE_ALREADY_HAS_SIBLINGS))
		}

		addChild(childNode)
		return nil
	})

	return NewNodeFromGoDescription(NodeDescription{
		Tag:      string(tag),
		Class:    class,
		Children: children,
		Id:       id,
	})
}

type NodeDescription struct {
	Tag                    string
	Children               []*HTMLNode
	Class                  string
	Id                     string
	Attributes             []html.Attribute
	DoNoTransformUntrusted bool
}

func NewNodeFromGoDescription(desc NodeDescription) *HTMLNode {
	//TODO: merge text nodes that are siblings

	dataAtom := atom.Lookup(utils.StringAsBytes(desc.Tag))

	if dataAtom == 0 {
		panic(fmt.Errorf("provided tag '%s' is invalid", desc.Tag))
	}

	node := &HTMLNode{
		node: &html.Node{
			Type:     html.ElementNode,
			DataAtom: dataAtom,
		},
	}

	//Do not keep a reference to the tag string.

	if utils.SameIdentityStrings(desc.Tag, trustedScriptElementTagName) {
		node.node.Data = trustedScriptElementTagName
	} else {
		node.node.Data = dataAtom.String()
	}

	// Set parent & siblings of all children.
	for i, child := range desc.Children {
		child.node.Parent = node.node
		if i != len(desc.Children)-1 {
			nextSibliging := desc.Children[i+1]
			nextSibliging.node.PrevSibling = child.node
			child.node.NextSibling = nextSibliging.node
		}
	}

	//Add attributes of parent.

	if desc.Class != "" {
		node.node.Attr = append(node.node.Attr, html.Attribute{Key: "class", Val: desc.Class})
	}

	if desc.Id != "" {
		node.node.Attr = append(node.node.Attr, html.Attribute{Key: "id", Val: desc.Id})
	}

	node.node.Attr = append(node.node.Attr, desc.Attributes...)

	if len(desc.Children) > 0 {
		node.node.FirstChild = desc.Children[0].node
		node.node.LastChild = desc.Children[len(desc.Children)-1].node
	}

	//Handle untrusted elements and attributes.

	if !desc.DoNoTransformUntrusted {
		TransformUntrustedScriptsAndHyperscriptAttributes(node.node)
	}

	return node
}

type HTML5DocumentDescription struct {
	HtmlTagNode *HTMLNode //this value should not be referenced elsewhere
}

func NewHTML5DocumentNodeFromGoDescription(desc HTML5DocumentDescription) *HTMLNode {
	if desc.HtmlTagNode.node.Type != html.ElementNode || desc.HtmlTagNode.node.DataAtom != atom.Html {
		panic(fmt.Errorf("argument is not a <html> element node"))
	}

	docTypeNode := &html.Node{
		Type:        html.DoctypeNode,
		Data:        "html",
		NextSibling: desc.HtmlTagNode.node,
	}

	desc.HtmlTagNode.node.PrevSibling = docTypeNode

	docNode := &HTMLNode{
		node: &html.Node{
			Type:       html.DocumentNode,
			FirstChild: docTypeNode,
			LastChild:  desc.HtmlTagNode.node,
		},
	}

	return docNode
}

func CreateTextNode(strLike core.StringLike) *HTMLNode {
	return NewHTMLNode(createTextNode(strLike.GetOrBuildString()))
}

func createTextNode(s string) *html.Node {
	return &html.Node{
		Type:     html.TextNode,
		DataAtom: 0,
		Data:     s,
	}
}

func CreateTextLikeElem(strLike core.StringLike, atom atom.Atom) *HTMLNode {
	child := createTextNode(strLike.GetOrBuildString())
	node := &html.Node{
		Type:       html.ElementNode,
		DataAtom:   atom,
		Data:       atom.String(),
		FirstChild: child,
		LastChild:  child,
	}
	child.Parent = node

	return NewHTMLNode(node)
}

func CreateSpanElem(strLike core.StringLike) *HTMLNode {
	return CreateTextLikeElem(strLike, atom.Span)
}

func CreateTimeElem(strLike core.StringLike) *HTMLNode {
	return CreateTextLikeElem(strLike, atom.Time)
}
