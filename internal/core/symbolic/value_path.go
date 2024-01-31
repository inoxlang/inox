package symbolic

import (
	"errors"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

var (
	_ = []ValuePath{ANY_PROPNAME, &LongValuePath{}, (*AnyValuePath)(nil)}
	_ = []ValuePathSegment{ANY_PROPNAME}
)

type ValuePath interface {
	Value
	// GetFrom should return (Value, true, nil) if the value can be retrieved from v and is necessarily present.
	// GetFrom should return (Value, false, nil) if the value can be retrieved from v but it not necessarily present.
	// GetFrom should return (nil, false, nil) if the value cannot be retrieved from v.
	// GetFrom should return (nil, false, ErrNotConcretizable) if the value path is not concretizable.
	GetFrom(v Value) (result Value, alwaysPresent bool, err error)
}

type ValuePathSegment interface {
	Value
	// SegmentGetFrom should return (Value, true, nil) if the value can be retrieved from v and is necessarily present.
	// SegmentGetFrom should return (Value, false, nil) if the value can be retrieved from v but it not necessarily present.
	// SegmentGetFrom should return (nil, false, nil) if the value cannot be retrieved from v.
	// SegmentGetFrom should return (nil, false, ErrNotConcretizable) if the value path is not concretizable.
	SegmentGetFrom(v Value) (result Value, alwaysPresent bool, err error)
}

// AnyValuePath represents a ValuePath we don't know the concrete type.
type AnyValuePath struct {
}

func (p *AnyValuePath) GetFrom(v Value) (result Value, alwaysPresent bool, err error) {
	return nil, false, nil
}

func (p *AnyValuePath) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(ValuePath)
	return ok
}

func (p *AnyValuePath) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("value-path")
}

func (p *AnyValuePath) WidestOfType() Value {
	return ANY_VALUE_PATH
}

// A PropertyName represents a symbolic PropertyName.
type PropertyName struct {
	name string
	SerializableMixin
}

func NewPropertyName(name string) *PropertyName {
	return &PropertyName{name: name}
}

func (n *PropertyName) Name() string {
	return n.name
}

func (n *PropertyName) GetFrom(v Value) (Value, bool, error) {
	if !n.IsConcretizable() {
		return nil, false, ErrNotConcretizable
	}

	iprops, ok := AsIprops(v).(IProps)
	if !ok {
		return nil, false, nil
	}

	optionalIprops, ok := iprops.(OptionalIProps)
	if ok && slices.Contains(optionalIprops.OptionalPropertyNames(), n.name) {
		return optionalIprops.Prop(n.name), false, nil
	}

	if slices.Contains(iprops.PropertyNames(), n.name) {
		return iprops.Prop(n.name), true, nil
	}

	return nil, false, nil
}

func (n *PropertyName) SegmentGetFrom(v Value) (Value, bool, error) {
	return n.GetFrom(v)
}

func (n *PropertyName) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*PropertyName)
	if !ok {
		return false
	}
	return n.name == "" || n.name == other.name
}

func (n *PropertyName) IsConcretizable() bool {
	return n.name != ""
}

func (n *PropertyName) Concretize(ctx ConcreteContext) any {
	if !n.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreatePropertyName(n.name)
}

func (n *PropertyName) Static() Pattern {
	return &TypePattern{val: n.WidestOfType()}
}

func (n *PropertyName) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if n.name == "" {
		w.WriteName("property-name")
		return
	}

	w.WriteByte('.')
	w.WriteString(n.name)
}

func (n *PropertyName) underlyingString() *String {
	return &String{}
}

func (n *PropertyName) WidestOfType() Value {
	return ANY_PROPNAME
}

// A LongValuePath represents a symbolic LongValuePath.
type LongValuePath struct {
	SerializableMixin
	segments []ValuePathSegment //if empty any LongValuePath is matched.
}

func NewLongValuePath(segments ...ValuePathSegment) *LongValuePath {
	if len(segments) < 2 {
		panic(errors.New("at least two segments should be provided"))
	}
	return &LongValuePath{segments: segments}
}

func (p *LongValuePath) matchesAnyLongValuePath() bool {
	return len(p.segments) == 0
}

func (p *LongValuePath) GetFrom(v Value) (result Value, alwaysPresent bool, err error) {
	if !p.IsConcretizable() {
		return nil, false, ErrNotConcretizable
	}
	result = v

	alwaysPresent = true

	for _, segment := range p.segments {
		var _alwaysPresent bool
		result, _alwaysPresent, err = segment.SegmentGetFrom(result)
		if err != nil {
			return nil, false, err
		}
		if result == nil {
			return nil, false, nil
		}
		if !_alwaysPresent {
			alwaysPresent = false
		}
	}
	return
}

func (p *LongValuePath) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*LongValuePath)
	if !ok {
		return false
	}
	if p.matchesAnyLongValuePath() {
		return true
	}
	if other.matchesAnyLongValuePath() || len(p.segments) != len(other.segments) {
		return false
	}

	for i, segment := range p.segments {
		if !deeplyMatch(segment, other.segments[i]) {
			return false
		}
	}
	return true
}

func (p *LongValuePath) IsConcretizable() bool {
	if p.matchesAnyLongValuePath() {
		return false
	}
	for _, segment := range p.segments {
		if !IsConcretizable(segment) {
			return false
		}
	}
	return true
}

func (p *LongValuePath) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteSegments := make([]any, len(p.segments))

	for i, segment := range p.segments {
		concreteSegments[i] = utils.Must(Concretize(segment, ctx))
	}
	return extData.ConcreteValueFactories.CreateLongValuePath(concreteSegments...)
}

func (p *LongValuePath) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *LongValuePath) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("long-value-path")
	if p.matchesAnyLongValuePath() {
		return
	}
	w.WriteByte('(')

	for i, segment := range p.segments {
		if i > 0 {
			w.WriteByte(',')
		}
		segment.PrettyPrint(w, config)
	}
	w.WriteByte(')')
}

func (p *LongValuePath) WidestOfType() Value {
	return ANY_LONG_VALUE_PATH
}
