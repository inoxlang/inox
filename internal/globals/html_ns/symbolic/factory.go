package html_ns

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

const INTERPOLATION_LIMITATION_ERROR_MSG = "only HTML nodes, strings, integers, and resource names (e.g. paths, URLs) are allowed"

func init() {
	symbolic.RegisterXMLInterpolationCheckingFunction(
		CreateHTMLNodeFromXMLElement,
		func(n parse.Node, value symbolic.Value) (errorMsg string) {

			return checkInterpolationValue(value)
		},
	)
}

func checkInterpolationValue(value symbolic.Value) (errMsg string) {

	mv, ok := value.(*symbolic.Multivalue)

	if ok {
		err := mv.ForEachValue(func(v symbolic.Value) error {
			if errMsg := checkInterpolationValue(v); errMsg != "" {
				return errors.New(errMsg)
			}
			return nil
		})

		if err != nil {
			return err.Error()
		}
		return ""
	}

	switch {
	case symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[symbolic.StringLike](value),
		symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[symbolic.GoString](value),
		symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*HTMLNode](value),
		symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*symbolic.Int](value):
		return ""
	}

	if list, ok := value.(*symbolic.List); ok {
		elem := list.IteratorElementValue()
		switch {
		case symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[symbolic.StringLike](elem),
			symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[symbolic.GoString](value),
			symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*HTMLNode](elem),
			symbolic.ImplementsOrIsMultivalueWithAllValuesImplementing[*symbolic.Int](elem):
			return ""
		}
	}
	return INTERPOLATION_LIMITATION_ERROR_MSG
}

func CreateHTMLNodeFromXMLElement(ctx *symbolic.Context, elem *symbolic.XMLElement) *HTMLNode {

	var checkElem func(e *symbolic.XMLElement)
	checkElem = func(e *symbolic.XMLElement) {
		for name, val := range e.Attributes() {
			switch val.(type) {
			case symbolic.GoString, symbolic.StringLike, *symbolic.Int:
			default:
				errMsg := fmtAttrValueNotAccepted(val, name)
				sourceNode, ok := e.SourceNode()
				if ok {
					ctx.AddLocatedSymbolicGoFunctionError(sourceNode.Opening, errMsg)
				} else {
					ctx.AddSymbolicGoFunctionError(errMsg)
				}
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
	return fmt.Sprintf(
		"The value provided for the attribute '%s' is not accepted (%s)."+
			"Only HTML nodes, strings, integers, and resource names (e.g. paths, URLs) are allowed", name, symbolic.Stringify(val))
}
