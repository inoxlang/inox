package internal

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	FROM_XML_FACTORY_NAME = "from_xml_elem"
)

var (
	ANY_XML_ELEM = &XMLElement{}
)

// A XMLElement represents a symbolic XMLElement.
type XMLElement struct {
	name       string //if "" matches any node value
	attributes map[string]SymbolicValue
	children   []SymbolicValue
}

func NewXmlElement(name string, attributes map[string]SymbolicValue, children []SymbolicValue) *XMLElement {
	return &XMLElement{name: name, children: children, attributes: attributes}
}

func (r *XMLElement) Test(v SymbolicValue) bool {
	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *XMLElement) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *XMLElement) IsWidenable() bool {
	return false
}

func (r *XMLElement) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%xml-element")))
	return
}

func (r *XMLElement) Writer() *Writer {
	return &Writer{}
}

func (r *XMLElement) WidestOfType() SymbolicValue {
	return ANY_XML_ELEM
}
