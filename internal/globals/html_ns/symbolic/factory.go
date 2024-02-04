package html_ns

import (
	"fmt"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

const INTERPOLATION_LIMITATION_ERROR_MSG = "only HTML nodes, string-like and integer values are allowed"

func init() {
	symbolic.RegisterXMLInterpolationCheckingFunction(
		CreateHTMLNodeFromXMLElement,
		func(n parse.Node, value symbolic.Value) (errorMsg string) {

			switch {
			case symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[symbolic.StringLike](value),
				symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*HTMLNode](value),
				symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*symbolic.Int](value):
				return ""
			}

			if list, ok := value.(*symbolic.List); ok {
				elem := list.IteratorElementValue()
				switch {
				case symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[symbolic.StringLike](elem),
					symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*HTMLNode](elem),
					symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*symbolic.Int](elem):
					return ""
				}
			}
			return INTERPOLATION_LIMITATION_ERROR_MSG
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
				ctx.AddSymbolicGoFunctionError(fmtAttrValueNotAccepted(val, name))
			}
		}

		for _, child := range e.Children() {
			switch c := child.(type) {
			case *symbolic.XMLElement:
				checkElem(c)
			default:
				//already checked during interpolation checks
				//ctx.AddFormattedSymbolicGoFunctionError("value of interpolation is not accepted for now (%s), use a string or an integer", symbolic.Stringify(c))
			}
		}
	}

	checkElem(elem)

	return NewHTMLNode()
}

func fmtAttrValueNotAccepted(val symbolic.Value, name string) string {
	return fmt.Sprintf("value of attribute '%s' is not accepted for now (%s), use a string or an integer", name, symbolic.Stringify(val))
}
