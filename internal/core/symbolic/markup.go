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
	ANY_MARKUP_ELEM = &MarkupElement{}

	markupInterpolationCheckingFunctions = map[uintptr] /* go symbolic function pointer*/ MarkupInterpolationCheckingFunction{}
)

type MarkupInterpolationCheckingFunction func(n parse.Node, value Value) (errorMsg string)

func RegisterMarkupInterpolationCheckingFunction(factory any, fn MarkupInterpolationCheckingFunction) {
	markupInterpolationCheckingFunctions[reflect.ValueOf(factory).Pointer()] = fn
}

func UnregisterMarkupCheckingFunction(factory any) {
	delete(markupInterpolationCheckingFunctions, reflect.ValueOf(factory).Pointer())
}

// A MarkupElement represents a symbolic MarkupElement.
type MarkupElement struct {
	name       string //if "" matches any node value
	attributes map[string]Value
	children   []Value

	sourceNode *parse.MarkupElement
}

func NewMarkupElement(name string, attributes map[string]Value, children []Value) *MarkupElement {
	return &MarkupElement{name: name, children: children, attributes: attributes}
}

func (e *MarkupElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *MarkupElement) Attributes() map[string]Value {
	return e.attributes
}

// result should not be modified.
func (e *MarkupElement) Children() []Value {
	return e.children
}

// result should not be modified.
func (e *MarkupElement) SourceNode() (*parse.MarkupElement, bool) {
	if e.sourceNode == nil {
		return nil, false
	}
	return e.sourceNode, true
}

func (r *MarkupElement) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *MarkupElement) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("markup-element")
}

func (r *MarkupElement) Writer() *Writer {
	return &Writer{}
}

func (r *MarkupElement) WidestOfType() Value {
	return ANY_MARKUP_ELEM
}
