package internal

import (
	"fmt"
	"reflect"
	"strconv"

	core "github.com/inox-project/inox/internal/core"
	_html_symbolic "github.com/inox-project/inox/internal/globals/html/symbolic"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	CLASS_KEY    = "class"
	CHILDREN_KEY = "children"
	JOBS_KEY     = "jobs"
	MODEL_KEY    = "model"

	S_NODE_ALREADY_HAS_A_PARENT                    = "node that already has a parent"
	S_NODE_ALREADY_HAS_SIBLINGS                    = "node that already has siblings"
	S_CHILDREN_ALREADY_PROVIDED_WITH_CHILDREN_PROP = "children already provided with .children"
)

var (
	NODE_PATTERN = &core.TypePattern{
		Name:          "html.node",
		Type:          reflect.TypeOf(&HTMLNode{}),
		SymbolicValue: _html_symbolic.NewHTMLNode(),
	}
)

func _a(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "a", desc)
}

func _div(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "div", desc)
}

func _span(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "span", desc)
}

func _ul(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "ul", desc)
}

func _ol(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "ol", desc)
}

func _li(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "li", desc)
}

func _svg(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "svg", desc)
}

func _h1(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "h1", desc)
}

func _h2(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "h2", desc)
}

func _h3(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "h3", desc)
}

func _h4(ctx *core.Context, desc *core.Object) *HTMLNode {
	return NewNode(ctx, "h4", desc)
}

func NewNode(ctx *core.Context, tag core.Str, desc *core.Object) *HTMLNode {
	var class string
	var children []*HTMLNode
	var jobs []*core.LifetimeJob
	dataAtom := atom.Lookup([]byte(tag))

	if dataAtom == 0 {
		panic(fmt.Errorf("provided tag '%s' is invalid", tag))
	}

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
		k := string(it.Key(ctx).(core.Str))
		if core.IsIndexKey(k) {
			continue
		}

		v := it.Value(ctx)
		switch k {
		case CLASS_KEY:
			s, ok := v.(core.Str)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(CLASS_KEY, "description", "string", v))
			}
			class = string(s)
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
					panic(core.FmtUnexpectedElementInPropIterable("children", S_NODE_ALREADY_HAS_A_PARENT))
				}

				if child.node.NextSibling != nil || child.node.PrevSibling != nil {
					panic(core.FmtUnexpectedElementInPropIterable("children", S_NODE_ALREADY_HAS_SIBLINGS))
				}

				addChild(child)
			}
		case JOBS_KEY:
			iterable, ok := v.(core.Iterable)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(JOBS_KEY, "description", "iterable", v))
			}
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)
				job, ok := elem.(*core.LifetimeJob)
				if !ok {
					panic(core.FmtUnexpectedElementInPropIterableShowVal(elem, JOBS_KEY))
				}
				jobs = append(jobs, job)
			}
		default:
			panic(core.FmtUnexpectedPropInArgX(k, "description"))
		}
	}

	childrenAlreadyProvided := len(children) != 0
	length := desc.Len()

	//second iteration: index keys
	if length > 0 {
		for i := 0; i < int(length); i++ {
			k := strconv.Itoa(i)
			v := desc.Prop(ctx, k)

			if childrenAlreadyProvided {
				panic(core.FmtUnexpectedElementAtIndeKeyXofArg(k, "description", S_CHILDREN_ALREADY_PROVIDED_WITH_CHILDREN_PROP))
			}

			strLike, ok := v.(core.StringLike)
			if ok {
				addChild(strLike)
				continue
			}

			childNode, ok := v.(*HTMLNode)
			if !ok {
				panic(core.FmtUnexpectedElementAtIndexKeyxofArgShowVal(v, k, "description"))
			}

			if childNode.node.Parent != nil {
				panic(core.FmtUnexpectedElementAtIndeKeyXofArg(k, "description", S_NODE_ALREADY_HAS_A_PARENT))
			}

			if childNode.node.NextSibling != nil || childNode.node.PrevSibling != nil {
				panic(core.FmtUnexpectedElementAtIndeKeyXofArg(k, "description", S_NODE_ALREADY_HAS_SIBLINGS))
			}

			addChild(childNode)
		}
	}

	//TODO: merge text nodes that are siblings

	node := &HTMLNode{
		node: &html.Node{
			Type:     html.ElementNode,
			DataAtom: dataAtom,
			Data:     dataAtom.String(),
		},
	}

	// set parent & siblings of all children
	for i, child := range children {
		child.node.Parent = node.node
		if i != len(children)-1 {
			nextSibliging := children[i+1]
			nextSibliging.node.PrevSibling = child.node
			child.node.NextSibling = nextSibliging.node
		}
	}

	if class != "" {
		node.node.Attr = append(node.node.Attr, html.Attribute{Key: "class", Val: class})
	}

	if len(children) > 0 {
		node.node.FirstChild = children[0].node
		node.node.LastChild = children[len(children)-1].node
	}

	return node
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
