package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []Sequence{(*String)(nil), (*Tuple)(nil), (*AnySequenceOf)(nil)}
	_ = []MutableLengthSequence{(*List)(nil), (*ByteSlice)(nil), (*RuneSlice)(nil)}

	ANY_SEQ_OF_ANY = NewAnySequenceOf(ANY)
)

type Sequence interface {
	Indexable
	slice(start, end *Int) Sequence
}

type MutableSequence interface {
	Sequence
	set(ctx *Context, i *Int, v SymbolicValue)
	SetSlice(ctx *Context, start, end *Int, v Sequence)
}

type MutableLengthSequence interface {
	MutableSequence
	insertElement(ctx *Context, v SymbolicValue, i *Int)
	removePosition(ctx *Context, i *Int)
	//TODO: add removePositionRange
	insertSequence(ctx *Context, seq Sequence, i *Int)
	appendSequence(ctx *Context, seq Sequence)
}

type AnySequenceOf struct {
	elem SymbolicValue
}

func NewAnySequenceOf(elem SymbolicValue) *AnySequenceOf {
	return &AnySequenceOf{elem: elem}
}

func (s *AnySequenceOf) Test(v SymbolicValue) bool {
	seq, ok := v.(Sequence)
	return ok && s.elem.Test(widenToSameStaticTypeInMultivalue(seq.element()))
}

func (*AnySequenceOf) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (s *AnySequenceOf) IteratorElementValue() SymbolicValue {
	return s.elem
}

func (*AnySequenceOf) HasKnownLen() bool {
	return false
}

func (*AnySequenceOf) KnownLen() int {
	panic(ErrUnreachable)
}

func (s *AnySequenceOf) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%sequence(")))
	s.elem.PrettyPrint(w, config, depth, 0)
	utils.PanicIfErr(w.WriteByte(')'))
}

func (s *AnySequenceOf) element() SymbolicValue {
	return s.elem
}

func (s *AnySequenceOf) elementAt(i int) SymbolicValue {
	return s.elem
}

func (s *AnySequenceOf) slice(start *Int, end *Int) Sequence {
	return s
}

func (*AnySequenceOf) WidestOfType() SymbolicValue {
	return ANY_SEQ_OF_ANY
}
