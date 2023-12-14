package core

const DEFAULT_XML_ATTR_VALUE = Str("")

// A XMLElement represents the result of the evaluation of an XMLElement node in Inox code.
type XMLElement struct {
	name       string //if "" matches any node value
	attributes []XMLAttribute
	children   []Value
	rawContent string //only set for raw text elements
}

func (e *XMLElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *XMLElement) Children() []Value {
	return e.children[0:len(e.children):len(e.children)]
}

// result should not be modified.
func (e *XMLElement) Attributes() []XMLAttribute {
	return e.attributes[0:len(e.attributes):len(e.attributes)]
}

type XMLAttribute struct {
	name  string
	value Value
}

func (a XMLAttribute) Name() string {
	return a.name
}

func (a XMLAttribute) Value() Value {
	return a.value
}

func NewXmlElement(name string, attributes []XMLAttribute, children []Value) *XMLElement {
	return &XMLElement{name: name, children: children, attributes: attributes}
}

func NewRawTextXmlElement(name string, attributes []XMLAttribute, rawContent string) *XMLElement {
	return &XMLElement{name: name, rawContent: rawContent, attributes: attributes}
}
