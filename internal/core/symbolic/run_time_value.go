package symbolic

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	_ = []asInterface{
		(*RunTimeValue)(nil), (*indexableRunTimeValue)(nil), (*iterableRunTimeValue)(nil), (*ipropsRunTimeValue)(nil),
		(*strLikeRunTimeValue)(nil),
	}

	_ = []IRunTimeValue{
		(*indexableRunTimeValue)(nil), (*iterableRunTimeValue)(nil), (*ipropsRunTimeValue)(nil),
		(*strLikeRunTimeValue)(nil), (*serializableRunTimeValue)(nil),
	}
)

// A RunTimeValue represents a value that is not known.
type RunTimeValue struct {
	super Value
}

func NewRunTimeValue(value Value) *RunTimeValue {
	if IsConcretizable(value) {
		panic(fmt.Errorf("unexpected concretizable value provided to create a symbolic run-time value"))
	}
	return &RunTimeValue{super: value}
}

func (rv *RunTimeValue) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherRV, ok := v.(IRunTimeValue)
	return ok && otherRV.OriginalRunTimeValue() == rv
}

func (rv *RunTimeValue) Static() Pattern {
	return getStatic(rv.super)
}

func (rv *RunTimeValue) as(itf reflect.Type) (result Value) {

top_switch:
	switch itf {
	case SERIALIZABLE_INTERFACE_TYPE:
		serializable := true
		if _, ok := rv.super.(Serializable); !ok {
			serializable = false
			break
		}
		if serializable {
			result = &serializableRunTimeValue{RunTimeValue: rv}
		}
	case INDEXABLE_INTERFACE_TYPE:
		indexable := true
		if _, ok := rv.super.(Indexable); !ok {
			indexable = false
			break
		}
		if indexable {
			result = &indexableRunTimeValue{rv}
		}
	case ITERABLE_INTERFACE_TYPE:
		iterable := true
		if _, ok := rv.super.(Iterable); !ok {
			iterable = false
			break
		}
		if iterable {
			result = &iterableRunTimeValue{rv}
		}
	case WATCHABLE_INTERFACE_TYPE:
		watchable := true
		if _, ok := rv.super.(Watchable); !ok {
			watchable = false
			break
		}
		if watchable {
			result = &watchableRunTimeValue{rv}
		}
	case IPROPS_INTERFACE_TYPE:
		iprops := true
		if _, ok := rv.super.(IProps); !ok {
			iprops = false
			break
		}
		if iprops {
			result = &ipropsRunTimeValue{rv}
		}
	case STRLIKE_INTERFACE_TYPE:
		strLike := true
		if _, ok := rv.super.(StringLike); !ok {
			strLike = false
			break
		}
		if strLike {
			result = &strLikeRunTimeValue{RunTimeValue: rv}
		}
	default:
		if !reflect.ValueOf(rv.super).Type().Implements(itf) {
			break top_switch
		}
		val, _, err := converTypeToSymbolicValue(itf, false)
		if err == nil {
			return val
		}
	}

	if result == nil {
		return rv
	}

	return
}

func (rv *RunTimeValue) asStrLike() *strLikeRunTimeValue {
	return rv.as(STRLIKE_INTERFACE_TYPE).(*strLikeRunTimeValue)
}

func (rv *RunTimeValue) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("run-time-value(")
	rv.super.PrettyPrint(w.ZeroDepthIndent(), config)
	w.WriteByte(')')
}

func (rv *RunTimeValue) WidestOfType() Value {
	return rv.super.WidestOfType()
}

func (m *RunTimeValue) OriginalRunTimeValue() *RunTimeValue {
	return m
}

type IRunTimeValue interface {
	OriginalRunTimeValue() *RunTimeValue
}

type serializableRunTimeValue struct {
	*RunTimeValue
	SerializableMixin
}

type indexableRunTimeValue struct {
	*RunTimeValue
}

func (rv *indexableRunTimeValue) Element() Value {
	return rv.super.(Indexable).Element()
}

func (rv *indexableRunTimeValue) ElementAt(i int) Value {
	return rv.super.(Indexable).ElementAt(i)
}

func (rv *indexableRunTimeValue) IteratorElementKey() Value {
	return ANY_INT
}

func (rv *indexableRunTimeValue) IteratorElementValue() Value {
	return rv.Element()
}

func (rv *indexableRunTimeValue) KnownLen() int {
	return rv.super.(Indexable).KnownLen()
}

func (rv *indexableRunTimeValue) HasKnownLen() bool {
	return rv.super.(Indexable).HasKnownLen()
}

func (rv *indexableRunTimeValue) as(itf reflect.Type) Value {
	return rv.RunTimeValue.as(itf)
}

type iterableRunTimeValue struct {
	*RunTimeValue
}

func (rv *iterableRunTimeValue) IteratorElementKey() Value {
	return rv.super.(Indexable).IteratorElementKey()
}

func (rv *iterableRunTimeValue) IteratorElementValue() Value {
	return rv.super.(Indexable).IteratorElementValue()
}

func (rv *iterableRunTimeValue) as(itf reflect.Type) Value {
	return rv.RunTimeValue.as(itf)
}

type watchableRunTimeValue struct {
	*RunTimeValue
}

func (rv *watchableRunTimeValue) WatcherElement() Value {
	return ANY
}

func (rv *watchableRunTimeValue) as(itf reflect.Type) Value {
	return rv.RunTimeValue.as(itf)
}

type ipropsRunTimeValue struct {
	*RunTimeValue
}

func (rv *ipropsRunTimeValue) Prop(name string) Value {
	return rv.super.(IProps).Prop(name)
}

func (rv *ipropsRunTimeValue) SetProp(state *State, node parse.Node, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(rv))
}

func (rv *ipropsRunTimeValue) WithExistingPropReplaced(state *State, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(rv))
}

func (rv *ipropsRunTimeValue) PropertyNames() []string {
	return nil
}

func (rv *ipropsRunTimeValue) as(itf reflect.Type) Value {
	return rv.RunTimeValue.as(itf)
}

type strLikeRunTimeValue struct {
	*RunTimeValue
	SerializableMixin
}

func (*strLikeRunTimeValue) IteratorElementKey() Value {
	return ANY_STRING.IteratorElementKey()
}

func (*strLikeRunTimeValue) IteratorElementValue() Value {
	return ANY_STRING.IteratorElementKey()
}

func (*strLikeRunTimeValue) HasKnownLen() bool {
	return false
}

func (*strLikeRunTimeValue) KnownLen() int {
	return -1
}

func (*strLikeRunTimeValue) Element() Value {
	return ANY_STRING.Element()
}

func (*strLikeRunTimeValue) ElementAt(i int) Value {
	return ANY_STRING.ElementAt(i)
}

func (*strLikeRunTimeValue) slice(start, end *Int) Sequence {
	return ANY_STRING.slice(start, end)
}

func (*strLikeRunTimeValue) GetOrBuildString() *String {
	return ANY_STRING
}

func (rv *strLikeRunTimeValue) WidestOfType() Value {
	return rv.super.WidestOfType()
}

func (*strLikeRunTimeValue) Reader() *Reader {
	return ANY_READER
}

func (*strLikeRunTimeValue) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (rv *strLikeRunTimeValue) Prop(name string) Value {
	switch name {
	case "replace":
		return &GoFunction{
			fn: func(ctx *Context, old, new StringLike) *String {
				return ANY_STRING
			},
		}
	case "trim_space":
		return &GoFunction{
			fn: func(ctx *Context) StringLike {
				return ANY_STR_LIKE
			},
		}
	case "has_prefix":
		return &GoFunction{
			fn: func(ctx *Context, s StringLike) *Bool {
				return ANY_BOOL
			},
		}
	case "has_suffix":
		return &GoFunction{
			fn: func(ctx *Context, s StringLike) *Bool {
				return ANY_BOOL
			},
		}
	default:
		panic(FormatErrPropertyDoesNotExist(name, rv))
	}
}

func (rv *strLikeRunTimeValue) WithExistingPropReplaced(state *State, name string, value Value) (StringLike, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(rv))
}

func (rv *strLikeRunTimeValue) as(itf reflect.Type) Value {
	return rv.RunTimeValue.as(itf)
}
