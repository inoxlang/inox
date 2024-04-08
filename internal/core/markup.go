package core

const DEFAULT_MARKUP_ATTR_VALUE = String("")

// A MarkupElement represents the result of the evaluation of an MarkupElement node in Inox code.
type MarkupElement struct {
	name       string //if "" matches any node value
	attributes []MarkupAttribute
	children   []Value
	rawContent string //only set for raw text elements
}

func (e *MarkupElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *MarkupElement) Children() []Value {
	return e.children[0:len(e.children):len(e.children)]
}

func (e *MarkupElement) RawContent() string {
	return e.rawContent
}

// result should not be modified.
func (e *MarkupElement) Attributes() []MarkupAttribute {
	return e.attributes[0:len(e.attributes):len(e.attributes)]
}

type MarkupAttribute struct {
	name  string
	value Value
}

func NewMarkupAttribute(name string, value Value) MarkupAttribute {
	return MarkupAttribute{
		name:  name,
		value: value,
	}
}

func (a MarkupAttribute) Name() string {
	return a.name
}

func (a MarkupAttribute) Value() Value {
	return a.value
}

func NewMarkupElement(name string, attributes []MarkupAttribute, children []Value) *MarkupElement {
	return &MarkupElement{name: name, children: children, attributes: attributes}
}

func NewRawTextMarkupElement(name string, attributes []MarkupAttribute, rawContent string) *MarkupElement {
	return &MarkupElement{name: name, rawContent: rawContent, attributes: attributes}
}
