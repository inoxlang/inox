package css

import (
	"errors"
	"strings"
)

type Node struct {
	Children []Node
	Data     string
	Type     NodeType
	Error    bool
}

type NodeType uint8

const (
	Stylesheet NodeType = iota + 1
	Ruleset
	AtRule
	Selector
	MediaQuery
	Declaration
	Dimension
	Number
	Ident
	ClassName
	FunctionCall
	ParenthesizedExpr
	AttributeSelector
	Hash
	String
	URL
	Percentage
	UnicodeRange
	MatchOperator
	CustomPropertyName
	CustomPropertyValue
	Whitespace
	Comment
)

// SelectorString returns the selector if $n is a Ruleset, it panics otherwise.
func (n Node) SelectorString() string {
	if n.Type != Ruleset {
		panic(errors.New("node is not a ruleset"))
	}
	return n.Children[0].String()
}

func (n Node) String() string {
	w := &strings.Builder{}
	n.string(w, 0)

	return w.String()
}

func (n Node) IsZero() bool {
	return n.Children == nil && n.Type == 0 && n.Data == "" && !n.Error
}

func (n Node) string(w *strings.Builder, indent int) {

	for i := 0; i < indent; i++ {
		w.WriteByte(' ')
	}

	switch n.Type {
	case Stylesheet:
		for i, child := range n.Children {
			if i != 0 {
				w.WriteByte('\n')
			}
			child.string(w, indent)
		}
	case AtRule:
		w.WriteString(n.Data)

		//Query
		n.Children[0].string(w, 0)

		w.WriteString(" { ")

		//Rules
		if len(n.Children) > 1 {
			for _, child := range n.Children[1:] {
				w.WriteByte('\n')
				child.string(w, indent+2)
			}
			w.WriteByte('\n')
			for i := 0; i < indent; i++ {
				w.WriteByte(' ')
			}
		}

		w.WriteString("}")
	case Ruleset:
		//Selector
		n.Children[0].string(w, 0)

		w.WriteString(" {")

		//Declarations
		if len(n.Children) > 1 {
			for _, child := range n.Children[1:] {
				w.WriteByte('\n')
				child.string(w, indent+2)
			}
			w.WriteByte('\n')
			for i := 0; i < indent; i++ {
				w.WriteByte(' ')
			}
		}

		w.WriteString("}")
	case Declaration:
		//Name
		w.WriteString(n.Data)

		w.WriteByte(':')

		//Value
		for _, child := range n.Children {
			w.WriteByte(' ')
			child.string(w, 0)
		}

		w.WriteByte(';')
	case Selector:
		for i, child := range n.Children {
			if i != 0 {
				w.WriteByte(' ')
			}
			child.string(w, 0)
		}
	case ClassName:
		w.WriteByte('.')
		w.WriteString(n.Data)
	case FunctionCall:
		w.WriteString(n.Data)
		w.WriteByte('(')

		for i, child := range n.Children {
			if i != 0 {
				w.WriteString(", ")
			}
			child.string(w, 0)
		}

		w.WriteByte(')')
	case ParenthesizedExpr:
		w.WriteByte('(')

		for _, child := range n.Children {
			child.string(w, 0)
		}

		w.WriteByte(')')
	case AttributeSelector:
		w.WriteByte('[')
		for _, child := range n.Children {
			child.string(w, 0)
		}

		w.WriteByte(']')
	default:
		w.WriteString(n.Data)
	}
}
