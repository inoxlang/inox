package css

import (
	"bufio"
	"bytes"
	"errors"
	"io"
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
	MediaFeature
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
	buf := &bytes.Buffer{}
	n.writeTo(buf, 0)

	return buf.String()
}

func (n Node) WriteTo(w io.Writer) (err error) {

	writer, ok := w.(astStringificatioWriter)
	if !ok {
		bufferedWriter := bufio.NewWriter(w)
		writer = bufferedWriter
		defer func() {
			err = bufferedWriter.Flush()
		}()
	}

	n.writeTo(writer, 0)
	return
}

func (n Node) IsZero() bool {
	return n.Children == nil && n.Type == 0 && n.Data == "" && !n.Error
}

func (n Node) Equal(other Node) bool {
	if n.Type != other.Type || n.Data != other.Data || n.Error != other.Error || len(n.Children) != len(other.Children) {
		return false
	}
	for i, child := range n.Children {
		if !child.Equal(other.Children[i]) {
			return false
		}
	}
	return true
}

func (n Node) IsImport() bool {
	return n.Type == AtRule && n.Data == "@import"
}

func (n Node) IsRegularAtRule() bool {
	if n.Type != AtRule {
		return false
	}
	switch n.Data {
	//https://developer.mozilla.org/en-US/docs/Web/CSS/At-rule#regular
	case "@charset", "@import", "@namespace":
		return true
	}
	return false
}

func (n Node) writeTo(w astStringificatioWriter, indent int) {

	for i := 0; i < indent; i++ {
		w.WriteByte(' ')
	}

	switch n.Type {
	case Stylesheet:
		for i, child := range n.Children {
			if i != 0 {
				w.WriteByte('\n')
			}
			child.writeTo(w, indent)
		}
	case AtRule:
		w.WriteString(n.Data)

		if n.IsRegularAtRule() { //regular: https://developer.mozilla.org/en-US/docs/Web/CSS/At-rule#regular

			for _, child := range n.Children {
				w.WriteByte(' ')
				child.writeTo(w, 0)
			}

			w.WriteByte(';')
		} else { //nested: https://developer.mozilla.org/en-US/docs/Web/CSS/At-rule#nested
			//Head
			n.Children[0].writeTo(w, 0)

			w.WriteString(" { ")

			//Rules
			if len(n.Children) > 1 {
				for _, child := range n.Children[1:] {
					w.WriteByte('\n')
					child.writeTo(w, indent+2)
				}
				w.WriteByte('\n')
				for i := 0; i < indent; i++ {
					w.WriteByte(' ')
				}
			}

			w.WriteString("}")
		}

	case Ruleset:
		//Selector
		n.Children[0].writeTo(w, 0)

		w.WriteString(" {")

		//Declarations
		if len(n.Children) > 1 {
			for _, child := range n.Children[1:] {
				w.WriteByte('\n')
				child.writeTo(w, indent+2)
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
			child.writeTo(w, 0)
		}

		w.WriteByte(';')
	case Selector:
		for i, child := range n.Children {
			if i != 0 {
				w.WriteByte(' ')
			}
			child.writeTo(w, 0)
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
			child.writeTo(w, 0)
		}

		w.WriteByte(')')
	case ParenthesizedExpr:
		w.WriteByte('(')

		for _, child := range n.Children {
			child.writeTo(w, 0)
		}

		w.WriteByte(')')
	case AttributeSelector:
		w.WriteByte('[')
		for _, child := range n.Children {
			child.writeTo(w, 0)
		}

		w.WriteByte(']')
	default:
		w.WriteString(n.Data)
	}
}

type astStringificatioWriter interface {
	io.Writer
	WriteByte(byte) error
	WriteString(string) (int, error)
}
