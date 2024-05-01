package symbolic

import (
	"reflect"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

const (
	FROM_MARKUP_FACTORY_NAME = "from_markup_elem"
)

var (
	ANY_MARKUP_ELEM = &NonInterpretedMarkupElement{}
	ANY_MARKUP_NODE = &AnyMarkupNode{}

	_ MarkupNode = ANY_MARKUP_NODE

	markupInterpolationCheckingFunctions = map[uintptr] /* go symbolic function pointer*/ MarkupInterpolationCheckingFunction{}
)

type MarkupInterpolationCheckingFunction func(n parse.Node, value Value) (errorMsg string)

func RegisterMarkupInterpolationCheckingFunction(factory any, fn MarkupInterpolationCheckingFunction) {
	markupInterpolationCheckingFunctions[reflect.ValueOf(factory).Pointer()] = fn
}

func UnregisterMarkupCheckingFunction(factory any) {
	delete(markupInterpolationCheckingFunctions, reflect.ValueOf(factory).Pointer())
}

// A NonInterpretedMarkupElement represents a symbolic NonInterpretedMarkupElement.
type NonInterpretedMarkupElement struct {
	name       string //if "" matches any node value
	attributes map[string]Value
	children   []Value

	sourceNode *parse.MarkupElement
}

func NewNonInterpretedMarkupElement(name string, attributes map[string]Value, children []Value) *NonInterpretedMarkupElement {
	return &NonInterpretedMarkupElement{name: name, children: children, attributes: attributes}
}

func (e *NonInterpretedMarkupElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *NonInterpretedMarkupElement) Attributes() map[string]Value {
	return e.attributes
}

// result should not be modified.
func (e *NonInterpretedMarkupElement) Children() []Value {
	return e.children
}

// result should not be modified.
func (e *NonInterpretedMarkupElement) SourceNode() (*parse.MarkupElement, bool) {
	if e.sourceNode == nil {
		return nil, false
	}
	return e.sourceNode, true
}

func (r *NonInterpretedMarkupElement) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *NonInterpretedMarkupElement) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("non-interpreted-markup-element")
}

func (r *NonInterpretedMarkupElement) Writer() *Writer {
	return &Writer{}
}

func (r *NonInterpretedMarkupElement) WidestOfType() Value {
	return ANY_MARKUP_ELEM
}

type MarkupNode interface {
	Value
	_MarkupNode()
}

type AnyMarkupNode struct {
	MarkupNodeMixin
}

func (n *AnyMarkupNode) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return ImplOrMultivaluesImplementing[MarkupNode](v)
}

func (n *AnyMarkupNode) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("markup-node")
}

func (n *AnyMarkupNode) WidestOfType() Value {
	return ANY_MARKUP_NODE
}

type MarkupNodeMixin struct {
}

func (MarkupNodeMixin) _MarkupNode() {

}
