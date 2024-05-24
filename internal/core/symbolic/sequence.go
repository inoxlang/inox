package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
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
	set(ctx *Context, i *Int, v Value)
	SetSlice(ctx *Context, start, end *Int, v Sequence)
}

type MutableLengthSequence interface {
	MutableSequence
	insertElement(ctx *Context, v Value, i *Int)
	removePosition(ctx *Context, i *Int)
	//TODO: add removePositionRange
	insertSequence(ctx *Context, seq Sequence, i *Int)
	appendSequence(ctx *Context, seq Sequence)
}

type AnySequenceOf struct {
	elem Value
}

func NewAnySequenceOf(elem Value) *AnySequenceOf {
	return &AnySequenceOf{elem: elem}
}

func (s *AnySequenceOf) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	seq, ok := v.(Sequence)
	return ok && s.elem.Test(MergeValuesWithSameStaticTypeInMultivalue(seq.Element()), state)
}

func (*AnySequenceOf) IteratorElementKey() Value {
	return ANY_INT
}

func (s *AnySequenceOf) IteratorElementValue() Value {
	return s.elem
}

func (*AnySequenceOf) HasKnownLen() bool {
	return false
}

func (*AnySequenceOf) KnownLen() int {
	panic(ErrUnreachable)
}

func (s *AnySequenceOf) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("sequence(")
	s.elem.PrettyPrint(w.ZeroIndent(), config)
	w.WriteByte(')')
}

func (s *AnySequenceOf) Element() Value {
	return s.elem
}

func (s *AnySequenceOf) ElementAt(i int) Value {
	return s.elem
}

func (s *AnySequenceOf) slice(start *Int, end *Int) Sequence {
	return s
}

func (*AnySequenceOf) WidestOfType() Value {
	return ANY_SEQ_OF_ANY
}
