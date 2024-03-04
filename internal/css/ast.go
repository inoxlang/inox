package css

import "strings"

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
	Function
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

func (n Node) String() string {
	w := &strings.Builder{}
	n.string(w, 0)

	return w.String()
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
		w.WriteString("@media")

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
	default:
		w.WriteString(n.Data)
	}
}
