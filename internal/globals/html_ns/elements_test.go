package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestElementFunction(t *testing.T) {

	testCases := map[string]struct {
		tag    string
		desc   func(ctx *core.Context) *core.Object
		panics bool
		result func(ctx *core.Context) *HTMLNode
	}{
		"invalid tag": {
			tag:    "?",
			panics: true,
		},
		"empty description": {
			tag:  "div",
			desc: func(ctx *core.Context) *core.Object { return core.NewObject() },
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
					},
				}
			},
		},
		"empty .class": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{CLASS_KEY: core.String("")}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
					},
				}
			},
		},
		"non empty .class": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{CLASS_KEY: core.String("x")}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
						Attr:     []html.Attribute{{Key: "class", Val: "x"}},
					},
				}
			},
		},
		"empty .children": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{CHILDREN_KEY: core.NewWrappedValueList()}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
					},
				}
			},
		},
		".children : single-element list : *HTMLNode": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{
					CHILDREN_KEY: core.NewWrappedValueList(&HTMLNode{
						node: &html.Node{
							Type:     html.ElementNode,
							DataAtom: atom.Div,
							Data:     "div",
							Attr:     []html.Attribute{{Key: "child"}},
						},
					}),
				}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",

						FirstChild: &html.Node{
							Type:     html.ElementNode,
							DataAtom: atom.Div,
							Data:     "div",
							Attr:     []html.Attribute{{Key: "child"}},
						},
					},
				}
			},
		},
		".children : single-element list: string": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{
					CHILDREN_KEY: core.NewWrappedValueList(core.String("text")),
				}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
						FirstChild: &html.Node{
							Type:     html.TextNode,
							DataAtom: 0,
							Data:     "text",
						},
					},
				}
			},
		},
		"*HTMLNode at key 0": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{
					"": core.NewWrappedValueList(
						&HTMLNode{
							node: &html.Node{
								Type:     html.ElementNode,
								DataAtom: atom.Div,
								Data:     "div",
								Attr:     []html.Attribute{{Key: "child"}},
							},
						},
					),
				}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
						FirstChild: &html.Node{
							Type:     html.ElementNode,
							DataAtom: atom.Div,
							Data:     "div",
							Attr:     []html.Attribute{{Key: "child"}},
						},
					},
				}
			},
		},
		"string at key 0": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{
					"": core.NewWrappedValueList(core.String("text")),
				}, ctx)
			},
			result: func(ctx *core.Context) *HTMLNode {
				return &HTMLNode{
					node: &html.Node{
						Type:     html.ElementNode,
						DataAtom: atom.Div,
						Data:     "div",
						FirstChild: &html.Node{
							Type:     html.TextNode,
							DataAtom: 0,
							Data:     "text",
						},
					},
				}
			},
		},
		".children : single-element list (string) AND string at key 0": {
			tag: "div",
			desc: func(ctx *core.Context) *core.Object {
				return core.NewObjectFromMap(core.ValMap{
					CHILDREN_KEY: core.NewWrappedValueList(core.String("text1")),
					"0":          core.NewWrappedValueList(core.NewWrappedValueList(core.String("text1"))),
				}, ctx)
			},
			panics: true,
		},
	}

	for name, testCase := range testCases {

		t.Run(name, func(t *testing.T) {
			ctx := core.NewContext(core.ContextConfig{})
			core.NewGlobalState(ctx)

			if testCase.panics {
				assert.Panics(t, func() {
					NewNode(ctx, core.String(testCase.tag), testCase.desc(ctx))
				})
			} else {
				expectedResult := testCase.result(ctx)
				walkHTMLNode(expectedResult.node, func(n *html.Node) error {
					if n.FirstChild != nil {
						n.FirstChild.Parent = n
					}

					if n.LastChild != nil && n.LastChild != n.FirstChild {
						n.LastChild.Parent = n
						n.LastChild.PrevSibling = n.FirstChild
						n.FirstChild.NextSibling = n.LastChild
					}

					if n.LastChild == nil {
						n.LastChild = n.FirstChild
					}

					return nil
				}, 0)

				result := NewNode(ctx, core.String(testCase.tag), testCase.desc(ctx))
				assert.NotNil(t, result)
				assert.Equal(t, expectedResult, result)
			}
		})
	}

}
