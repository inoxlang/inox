package symbolic

import (
	"bufio"
	"errors"
	"reflect"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []asInterface{
		(*Multivalue)(nil), (*indexableMultivalue)(nil), (*iterableMultivalue)(nil), (*ipropsMultivalue)(nil),
		(*strLikeMultivalue)(nil),
	}

	_ = []IMultivalue{
		(*indexableMultivalue)(nil), (*iterableMultivalue)(nil), (*ipropsMultivalue)(nil),
		(*strLikeMultivalue)(nil),
	}

	enableMultivalueCaching = true
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
	case SERIALIZABLE_INTERFACE_TYPE:
		serializable := true
		for _, val := range mv.values {
			if _, ok := val.(Serializable); !ok {
				serializable = false
				break
			}
		}
		if serializable {
			result = &serializableMultivalue{Multivalue: mv}
		}
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
	case WATCHABLE_INTERFACE_TYPE:
		watchable := true
		for _, val := range mv.values {
			if _, ok := val.(Watchable); !ok {
				watchable = false
				break
			}
		}
		if watchable {
			result = &watchableMultivalue{mv}
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
	case STRLIKE_INTERFACE_TYPE:
		strLike := true
		for _, val := range mv.values {
			if _, ok := val.(StringLike); !ok {
				strLike = false
				break
			}
		}
		if strLike {
			result = &strLikeMultivalue{Multivalue: mv}
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

	if enableMultivalueCaching {
		if mv.cache == nil {
			mv.cache = make(map[reflect.Type]SymbolicValue)
		}

		mv.cache[itf] = result
	}

	return result
}

func (mv *Multivalue) getValues() []SymbolicValue {
	return mv.values
}

func (mv *Multivalue) AllValues(callbackFn func(v SymbolicValue) bool) bool {
	for _, val := range mv.values {
		if !callbackFn(val) {
			return false
		}
	}
	return true
}

func (mv *Multivalue) WidenSimpleValues() SymbolicValue {
	first := mv.values[0]
	if IsSimpleSymbolicInoxVal(first) {
		widened := first.WidestOfType()

		for _, other := range mv.values[1:] {
			if !widened.Test(other) {
				return mv
			}
		}

		return widened
	}
	return mv
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

func (m *Multivalue) OriginalMultivalue() *Multivalue {
	return m
}

type IMultivalue interface {
	OriginalMultivalue() *Multivalue
}

type serializableMultivalue struct {
	*Multivalue
	SerializableMixin
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

func (mv *indexableMultivalue) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (mv *indexableMultivalue) IteratorElementValue() SymbolicValue {
	return mv.element()
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
	return mv.Multivalue.as(itf)
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
	return mv.Multivalue.as(itf)
}

type watchableMultivalue struct {
	*Multivalue
}

func (mv *watchableMultivalue) WatcherElement() SymbolicValue {
	return ANY
}

func (mv *watchableMultivalue) as(itf reflect.Type) SymbolicValue {
	return mv.Multivalue.as(itf)
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
	return mv.Multivalue.as(itf)
}

type strLikeMultivalue struct {
	*Multivalue
	SerializableMixin
}

func (c *strLikeMultivalue) IteratorElementKey() SymbolicValue {
	return ANY_STR.IteratorElementKey()
}

func (c *strLikeMultivalue) IteratorElementValue() SymbolicValue {
	return ANY_STR.IteratorElementKey()
}

func (c *strLikeMultivalue) HasKnownLen() bool {
	return false
}

func (c *strLikeMultivalue) KnownLen() int {
	return -1
}

func (c *strLikeMultivalue) element() SymbolicValue {
	return ANY_STR.element()
}

func (c *strLikeMultivalue) elementAt(i int) SymbolicValue {
	return ANY_STR.elementAt(i)
}

func (c *strLikeMultivalue) slice(start, end *Int) Sequence {
	return ANY_STR.slice(start, end)
}

func (c *strLikeMultivalue) GetOrBuildString() *String {
	return ANY_STR
}

func (c *strLikeMultivalue) WidestOfType() SymbolicValue {
	return joinValues(c.values)
}

func (c *strLikeMultivalue) Reader() *Reader {
	return ANY_READER
}

func (c *strLikeMultivalue) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (c *strLikeMultivalue) Prop(name string) SymbolicValue {
	switch name {
	case "replace":
		return &GoFunction{
			fn: func(ctx *Context, old, new *AnyStringLike) *String {
				return ANY_STR
			},
		}
	case "trim_space":
		return &GoFunction{
			fn: func(ctx *Context) *AnyStringLike {
				return ANY_STR_LIKE
			},
		}
	case "has_prefix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return ANY_BOOL
			},
		}
	case "has_suffix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return ANY_BOOL
			},
		}
	default:
		panic(FormatErrPropertyDoesNotExist(name, c))
	}
}

func (mv *strLikeMultivalue) WithExistingPropReplaced(name string, value SymbolicValue) (StringLike, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(mv))
}

func (mv *strLikeMultivalue) as(itf reflect.Type) SymbolicValue {
	return mv.Multivalue.as(itf)
}
