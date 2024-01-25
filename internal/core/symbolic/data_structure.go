package symbolic

import pprint "github.com/inoxlang/inox/internal/prettyprint"

var (
	DICTIONARY_PROPNAMES = []string{"get", "set"}
	LIST_PROPNAMES       = []string{"append", "dequeue", "pop", "sorted", "sort_by", "len"}

	ANY_INDEXABLE    = &AnyIndexable{}
	ANY_ARRAY        = NewArrayOf(ANY)
	ANY_TUPLE        = NewTupleOf(ANY_SERIALIZABLE)
	ANY_ORDERED_PAIR = NewOrderedPair(ANY_SERIALIZABLE, ANY_SERIALIZABLE)
	ANY_OBJ          = &Object{}
	ANY_READONLY_OBJ = &Object{readonly: true}
	ANY_REC          = &Record{}
	ANY_DICT         = NewAnyDictionary()
	ANY_KEYLIST      = NewAnyKeyList()

	EMPTY_OBJECT          = NewEmptyObject()
	EMPTY_READONLY_OBJECT = NewEmptyReadonlyObject()
	EMPTY_LIST            = NewList()
	EMPTY_READONLY_LIST   = NewReadonlyList()
	EMPTY_TUPLE           = NewTuple()

	STRLIKE_LIST = NewListOf(ANY_STR_LIKE)

	_ = []Indexable{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Object)(nil), (*Record)(nil),
		(*IntRange)(nil), (*RuneRange)(nil), (*AnyStringLike)(nil), (*AnyIndexable)(nil), (*OrderedPair)(nil),

		(*indexableMultivalue)(nil),
	}

	_ = []Iterable{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Object)(nil), (*IntRange)(nil),
		(*AnyStringLike)(nil), (*AnyIndexable)(nil), (*OrderedPair)(nil),
	}

	_ = []Sequence{
		(*String)(nil), (*Array)(nil), (*List)(nil), (*Tuple)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil),
	}

	_ = []IProps{(*Object)(nil), (*Record)(nil), (*Namespace)(nil), (*Dictionary)(nil), (*List)(nil)}
	_ = []InexactCapable{(*Object)(nil), (*Record)(nil)}
)

type InexactCapable interface {
	Value

	//TestExact should behave like Test() at the only difference that inexactness should be ignored.
	//For example an inexact object should not match an another object that has additional properties.
	TestExact(v Value) bool
}

// An AnyIndexable represents a symbolic Indesable we do not know the concrete type.
type AnyIndexable struct {
	_ int
}

func (r *AnyIndexable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Indexable)

	return ok
}

func (i *AnyIndexable) IteratorElementKey() Value {
	return ANY
}

func (i *AnyIndexable) IteratorElementValue() Value {
	return ANY
}

func (i *AnyIndexable) Element() Value {
	return ANY
}

func (i *AnyIndexable) ElementAt(index int) Value {
	return ANY
}

func (i *AnyIndexable) KnownLen() int {
	return -1
}

func (i *AnyIndexable) HasKnownLen() bool {
	return false
}

func (r *AnyIndexable) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("indexable")
}

func (r *AnyIndexable) WidestOfType() Value {
	return ANY_INDEXABLE
}
