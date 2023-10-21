package symbolic

import (
	"reflect"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

const (
	FROM_XML_FACTORY_NAME = "from_xml_elem"
)

var (
	ANY_XML_ELEM = &XMLElement{}

	xmlInterpolationCheckingFunctions = map[uintptr] /* go symbolic function pointer*/ XMLInterpolationCheckingFunction{}
)

type XMLInterpolationCheckingFunction func(n parse.Node, value Value) (errorMsg string)

func RegisterXMLInterpolationCheckingFunction(factory any, fn XMLInterpolationCheckingFunction) {
	xmlInterpolationCheckingFunctions[reflect.ValueOf(factory).Pointer()] = fn
}

func UnregisterXMLCheckingFunction(factory any) {
	delete(xmlInterpolationCheckingFunctions, reflect.ValueOf(factory).Pointer())
}

// A XMLElement represents a symbolic XMLElement.
type XMLElement struct {
	name       string //if "" matches any node value
	attributes map[string]Value
	children   []Value
}

func NewXmlElement(name string, attributes map[string]Value, children []Value) *XMLElement {
	return &XMLElement{name: name, children: children, attributes: attributes}
}

func (e *XMLElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *XMLElement) Attributes() map[string]Value {
	return e.attributes
}

// result should not be modified.
func (e *XMLElement) Children() []Value {
	return e.children
}

func (r *XMLElement) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *XMLElement) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("xml-element")
	return
}

func (r *XMLElement) Writer() *Writer {
	return &Writer{}
}

func (r *XMLElement) WidestOfType() Value {
	return ANY_XML_ELEM
}
