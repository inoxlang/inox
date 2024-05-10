package core

import (
	"sync"

	"github.com/inoxlang/inox/internal/inoxconsts"
)

const DEFAULT_MARKUP_ATTR_VALUE = String("")

// A NonInterpretedMarkupElement represents the result of the evaluation of an NonInterpretedMarkupElement node in Inox code.
type NonInterpretedMarkupElement struct {
	name       string //if "" matches any node value
	attributes []NonInterpretedMarkupAttribute
	children   []Value
	rawContent string //only set for raw text elements
}

func (e *NonInterpretedMarkupElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *NonInterpretedMarkupElement) Children() []Value {
	return e.children[0:len(e.children):len(e.children)]
}

func (e *NonInterpretedMarkupElement) RawContent() string {
	return e.rawContent
}

// result should not be modified.
func (e *NonInterpretedMarkupElement) Attributes() []NonInterpretedMarkupAttribute {
	return e.attributes[0:len(e.attributes):len(e.attributes)]
}

type NonInterpretedMarkupAttribute struct {
	name                                     string
	value                                    Value
	createdFromHyperscriptAttributeShorthand bool
}

func NewMarkupAttribute(name string, value Value) NonInterpretedMarkupAttribute {
	return NonInterpretedMarkupAttribute{
		name:  name,
		value: value,
	}
}

func NewMarkupAttributeCreatedFromHyperscriptAttributeShorthand(value Value) NonInterpretedMarkupAttribute {
	return NonInterpretedMarkupAttribute{
		name:                                     inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME,
		value:                                    value,
		createdFromHyperscriptAttributeShorthand: true,
	}
}

func (a NonInterpretedMarkupAttribute) Name() string {
	return a.name
}

func (a NonInterpretedMarkupAttribute) Value() Value {
	return a.value
}

func (a NonInterpretedMarkupAttribute) CreatedFromHyperscriptAttributeShorthand() bool {
	return a.createdFromHyperscriptAttributeShorthand
}

func NewNonInterpretedMarkupElement(name string, attributes []NonInterpretedMarkupAttribute, children []Value) *NonInterpretedMarkupElement {
	return &NonInterpretedMarkupElement{name: name, children: children, attributes: attributes}
}

func NewNonInterpretedRawTextMarkupElement(name string, attributes []NonInterpretedMarkupAttribute, rawContent string) *NonInterpretedMarkupElement {
	return &NonInterpretedMarkupElement{name: name, rawContent: rawContent, attributes: attributes}
}

type MarkupNode interface {
	//ImmutableMarkupNode should return a snapshot of the markup node.
	ImmutableMarkupNode() (ImmutableMarkupNode, *sync.Pool)
}

type ImmutableMarkupNode interface {
	IsMarkupElement() bool

	MarkupTagName() (string, bool)

	MarkupAttributeCount() int

	MarkupAttributeValue(name string) (value string, present bool)

	MarkupChildNodeCount() int

	MarkupChild(childIndex int) ImmutableMarkupNode

	MarkupText() (string, bool)

	ImmutableMarkupNode_()
}
