package html_ns

import (
	"errors"
	"fmt"
	"slices"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/htmldata"
	"github.com/inoxlang/inox/internal/parse"
)

const INTERPOLATION_LIMITATION_ERROR_MSG = "only HTML nodes, strings, integers, and resource names (e.g. paths, URLs) are allowed"

func init() {
	symbolic.RegisterMarkupInterpolationCheckingFunction(
		CreateHTMLNodeFromMarkupElement,
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
	case symbolic.ImplOrMultivaluesImplementing[symbolic.StringLike](value),
		symbolic.ImplOrMultivaluesImplementing[symbolic.GoString](value),
		symbolic.ImplOrMultivaluesImplementing[*HTMLNode](value),
		symbolic.ImplOrMultivaluesImplementing[*symbolic.Int](value):
		return ""
	}

	if list, ok := value.(*symbolic.List); ok {
		elem := list.Element()
		switch {
		case symbolic.ImplOrMultivaluesImplementing[symbolic.StringLike](elem),
			symbolic.ImplOrMultivaluesImplementing[symbolic.GoString](value),
			symbolic.ImplOrMultivaluesImplementing[*HTMLNode](elem),
			symbolic.ImplOrMultivaluesImplementing[*symbolic.Int](elem):
			return ""
		}
	}
	return INTERPOLATION_LIMITATION_ERROR_MSG
}

func CreateHTMLNodeFromMarkupElement(ctx *symbolic.Context, elem *symbolic.NonInterpretedMarkupElement) *HTMLNode {

	var requiredAttributes []HTMLAttribute

	for name, val := range elem.Attributes() {
		hasAttrWithSameName := slices.ContainsFunc(requiredAttributes, func(a HTMLAttribute) bool { return a.name == name })

		if hasAttrWithSameName {
			continue
		}

		if htmldata.IsPseudoHtmxAttribute(name) {
			names, count := htmldata.GetEquivalentAttributesNamesToPseudoHTMXAttribute(name)

			for _, name := range names[:count] {
				hasAttrWithSameName := slices.ContainsFunc(requiredAttributes, func(a HTMLAttribute) bool { return a.name == name })
				if !hasAttrWithSameName {
					requiredAttributes = append(requiredAttributes, NewHTMLAttribute(name, symbolic.ANY_STRING))
				}
			}
			continue
		}

		switch val := val.(type) {
		case symbolic.GoString:
			requiredAttributes = append(requiredAttributes, NewHTMLAttribute(name, val.UnderlyingString()))
		case symbolic.StringLike:
			requiredAttributes = append(requiredAttributes, NewHTMLAttribute(name, val.GetOrBuildString()))
		case *symbolic.Int:
			requiredAttributes = append(requiredAttributes, NewHTMLAttribute(name, symbolic.ANY_STRING))
		default:
			errMsg := fmtAttrValueNotAccepted(val, name)
			sourceNode, ok := elem.SourceNode()
			if ok {
				ctx.AddLocatedSymbolicGoFunctionError(sourceNode.Opening, errMsg)
			} else {
				ctx.AddSymbolicGoFunctionError(errMsg)
			}
		}
	}

	var requiredChildren []*HTMLNode

	//handleChild adds an HTML node to $requiredChildren if $child can be converted to an HTML node
	//by the concrete HTML factory.
	handleChild := func(child symbolic.Value) {
		if nonInterpretedMarkupElement, ok := child.(*symbolic.NonInterpretedMarkupElement); ok {
			requiredChildren = append(requiredChildren, CreateHTMLNodeFromMarkupElement(ctx, nonInterpretedMarkupElement))
			return
		}

		if htmlNode, ok := child.(*HTMLNode); ok {
			if !htmlNode.Test(ANY_HTML_NODE, symbolic.RecTestCallState{}) {
				requiredChildren = append(requiredChildren, htmlNode)
			}
			return
		}

		//Text nodes from markup text or interpolations with a string result are ignored.
	}

	for _, child := range elem.Children() {
		if list, ok := child.(*symbolic.List); ok {
			if list.HasKnownLen() {
				for i := range list.KnownLen() {
					listElem := list.ElementAt(i)
					handleChild(listElem)
				}
			} else {
				handleChild(list.Element())
			}
		} else {
			handleChild(child)
		}
	}

	htmlNode := &HTMLNode{
		tagName:            elem.Name(),
		requiredAttributes: requiredAttributes,
		requiredChildren:   requiredChildren,
	}

	if sourceNode, ok := elem.SourceNode(); ok {
		htmlNode.sourceNode = sourceNode
	}

	return htmlNode
}

func fmtAttrValueNotAccepted(val symbolic.Value, name string) string {
	return fmt.Sprintf(
		"The value provided for the attribute '%s' is not accepted (%s)."+
			"Only HTML nodes, strings, integers, and resource names (e.g. paths, URLs) are allowed", name, symbolic.Stringify(val))
}
