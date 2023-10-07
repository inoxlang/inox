package html_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

func init() {
	symbolic.RegisterXMLInterpolationCheckingFunction(
		CreateHTMLNodeFromXMLElement,
		func(n parse.Node, value symbolic.SymbolicValue) (errorMsg string) {
			switch value.(type) {
			case symbolic.StringLike, *symbolic.Int, *HTMLNode:
				return ""
			default:
				const ERROR_MSG = "only HTML nodes, string-like and integer values are allowed"

				if list, ok := value.(*symbolic.List); ok {
					elem := list.IteratorElementValue()

					switch e := elem.(type) {
					case *HTMLNode:
						return ""
					case *symbolic.Multivalue:
						ok := e.AllValues(func(v symbolic.SymbolicValue) bool {
							_, ok := elem.(*HTMLNode)
							return ok
						})

						if ok {
							return ""
						}
					}
				}

				return ERROR_MSG
			}
		},
	)
}

func CreateHTMLNodeFromXMLElement(ctx *symbolic.Context, elem *symbolic.XMLElement) *HTMLNode {

	var checkElem func(e *symbolic.XMLElement)
	checkElem = func(e *symbolic.XMLElement) {
		for name, val := range e.Attributes() {
			switch val.(type) {
			case symbolic.StringLike, *symbolic.Int:
			default:
				ctx.AddFormattedSymbolicGoFunctionError("value of attribute '%s' is not accepted for now (%s), use a string or an integer", name, symbolic.Stringify(val))
			}
		}

		for _, child := range e.Children() {
			switch c := child.(type) {
			case *symbolic.XMLElement:
				checkElem(c)
			case symbolic.StringLike, *symbolic.Int, *HTMLNode:
			default:
				ctx.AddFormattedSymbolicGoFunctionError("value of interpolation is not accepted for now (%s), use a string or an integer", symbolic.Stringify(c))
			}
		}
	}

	checkElem(elem)

	return NewHTMLNode()
}
