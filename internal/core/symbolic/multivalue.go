package internal

import (
	"bufio"
	"errors"
	"reflect"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []asInterface{&Multivalue{}, &indexableMultivalue{}, &iterableMultivalue{}, &ipropsMultivalue{}}
)

// A Multivalue represents a set of possible values.
type Multivalue struct {
	cache  map[reflect.Type]SymbolicValue
	values []SymbolicValue
}

func NewMultivalue(values ...SymbolicValue) *Multivalue {
	if len(values) < 2 {
		panic(errors.New("failed to create MultiValue: value slice should have at least 2 elements"))
	}

	return &Multivalue{values: values}
}

func (mv *Multivalue) Test(v SymbolicValue) bool {
	for _, val := range mv.values {
		if val.Test(v) {
			return true
		}
	}

	otherMv, ok := v.(*Multivalue)
	if !ok || len(mv.values) < len(otherMv.values) {
		return false
	}

	for _, otherVal := range otherMv.values {
		ok := false

		for _, val := range mv.values {
			if val.Test(otherVal) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}

	return true
}

func (mv *Multivalue) as(itf reflect.Type) SymbolicValue {
	result, ok := mv.cache[itf]
	if ok {
		return result
	}

top_switch:
	switch itf {
	case INDEXABLE_INTERFACE_TYPE:
		indexable := true
		for _, val := range mv.values {
			if _, ok := val.(Indexable); !ok {
				indexable = false
				break
			}
		}
		if indexable {
			result = &indexableMultivalue{mv}
		}
	case ITERABLE_INTERFACE_TYPE:
		iterable := true
		for _, val := range mv.values {
			if _, ok := val.(Iterable); !ok {
				iterable = false
				break
			}
		}
		if iterable {
			result = &iterableMultivalue{mv}
		}
	case IPROPS_INTERFACE_TYPE:
		iprops := true
		for _, val := range mv.values {
			if _, ok := val.(IProps); !ok {
				iprops = false
				break
			}
		}
		if iprops {
			result = &ipropsMultivalue{mv}
		}
	default:
		for _, val := range mv.values {
			if !reflect.ValueOf(val).Type().Implements(itf) {
				break top_switch
			}
		}
		val, err := converTypeToSymbolicValue(itf)
		if err == nil {
			return val
		}
	}

	if result == nil {
		return mv
	}

	if mv.cache == nil {
		mv.cache = make(map[reflect.Type]SymbolicValue)
	}

	mv.cache[itf] = result
	return result
}

func (mv *Multivalue) getValues() []SymbolicValue {
	return mv.values
}

func (mv *Multivalue) Widen() (SymbolicValue, bool) {
	widenedValues := make([]SymbolicValue, len(mv.values))

	for i, val := range mv.values {
		if !val.IsWidenable() {
			return nil, false
		}
		widenedValues[i], _ = val.Widen()
	}

	return joinValues(widenedValues), true
}

func (mv *Multivalue) IsWidenable() bool {
	for _, val := range mv.values {
		if !val.IsWidenable() {
			return false
		}
	}
	return true
}

func (mv *Multivalue) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.PanicIfErr(w.WriteByte('('))

	for i, val := range mv.values {
		val.PrettyPrint(w, config, 0, 0)
		if i < len(mv.values)-1 {
			utils.Must(w.Write(utils.StringAsBytes(" | ")))
		}
	}
	utils.PanicIfErr(w.WriteByte(')'))
}

func (mv *Multivalue) WidestOfType() SymbolicValue {
	return joinValues(mv.values)
}

type indexableMultivalue struct {
	*Multivalue
}

func (mv *indexableMultivalue) element() SymbolicValue {
	elements := make([]SymbolicValue, len(mv.values))
	for i, val := range mv.values {
		elements[i] = val.(Indexable).element()
	}

	return joinValues(elements)
}

func (mv *indexableMultivalue) elementAt(i int) SymbolicValue {
	elements := make([]SymbolicValue, len(mv.values))
	for i, val := range mv.values {
		indexable := val.(Indexable)
		if !indexable.HasKnownLen() || i >= indexable.KnownLen() {
			return ANY
		}
		elements[i] = val.(Indexable).elementAt(i)
	}

	return joinValues(elements)
}

func (mv *indexableMultivalue) KnownLen() int {
	return mv.values[0].(Indexable).KnownLen()
}

func (mv *indexableMultivalue) HasKnownLen() bool {
	length := 0

	for i, val := range mv.values {
		indexable := val.(Indexable)
		if !indexable.HasKnownLen() {
			return false
		}

		if i == 0 {
			length = indexable.KnownLen()
		} else {
			if indexable.KnownLen() != length {
				return false
			}
		}
	}

	return true
}

func (mv *indexableMultivalue) as(itf reflect.Type) SymbolicValue {
	return mv.as(itf)
}

type iterableMultivalue struct {
	*Multivalue
}

func (mv *iterableMultivalue) IteratorElementKey() SymbolicValue {
	elements := make([]SymbolicValue, len(mv.values))
	for i, val := range mv.values {
		elements[i] = val.(Iterable).IteratorElementKey()
	}

	return joinValues(elements)
}

func (mv *iterableMultivalue) IteratorElementValue() SymbolicValue {
	elements := make([]SymbolicValue, len(mv.values))
	for i, val := range mv.values {
		elements[i] = val.(Iterable).IteratorElementValue()
	}

	return joinValues(elements)
}

func (mv *iterableMultivalue) as(itf reflect.Type) SymbolicValue {
	return mv.as(itf)
}

type ipropsMultivalue struct {
	*Multivalue
}

func (mv *ipropsMultivalue) Prop(name string) SymbolicValue {
	props := make([]SymbolicValue, len(mv.values))
	for i, val := range mv.values {
		props[i] = val.(IProps).Prop(name)
	}

	return joinValues(props)
}

func (mv *ipropsMultivalue) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(mv))
}

func (mv *ipropsMultivalue) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(mv))
}

func (mv *ipropsMultivalue) PropertyNames() []string {
	return nil
}

func (mv *ipropsMultivalue) as(itf reflect.Type) SymbolicValue {
	return mv.as(itf)
}
