package internal

var (
	ANY_ITERABLE = &AnyIterable{}
)

// An Iterable represents a symbolic Iterable.
type Iterable interface {
	SymbolicValue
	IteratorElementKey() SymbolicValue
	IteratorElementValue() SymbolicValue
}

// An AnyIterable represents a symbolic Iterable we do not know the concrete type.
type AnyIterable struct {
	_ int
}

func (r *AnyIterable) Test(v SymbolicValue) bool {
	_, ok := v.(Iterable)

	return ok
}

func (r *AnyIterable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyIterable) IsWidenable() bool {
	return false
}

func (r *AnyIterable) String() string {
	return "%iterable"
}

func (r *AnyIterable) WidestOfType() SymbolicValue {
	return &AnyIterable{}
}

func (r *AnyIterable) IteratorElementKey() SymbolicValue {
	return ANY
}

func (r *AnyIterable) IteratorElementValue() SymbolicValue {
	return ANY
}

// An Iterator represents a symbolic Iterator.
type Iterator struct {
	ElementValue SymbolicValue //if nil matches any
	_            int
}

func (r *Iterator) Test(v SymbolicValue) bool {
	it, ok := v.(*Iterator)
	if !ok {
		return false
	}
	if r.ElementValue == nil {
		return true
	}
	return r.ElementValue.Test(it.ElementValue)
}

func (r *Iterator) Widen() (SymbolicValue, bool) {
	if !r.IsWidenable() {
		return nil, false
	}
	return &Iterator{}, true
}

func (r *Iterator) IsWidenable() bool {
	return r.ElementValue != nil
}

func (r *Iterator) String() string {
	return "%iterator"
}

func (r *Iterator) IteratorElementKey() SymbolicValue {
	return ANY
}

func (r *Iterator) IteratorElementValue() SymbolicValue {
	if r.ElementValue == nil {
		return ANY
	}
	return r.ElementValue
}

func (r *Iterator) WidestOfType() SymbolicValue {
	return &Iterator{}
}
