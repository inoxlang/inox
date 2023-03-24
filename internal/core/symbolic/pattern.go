package internal

import (
	"bytes"
	"errors"
	"fmt"

	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
)

var (
	_ = []Pattern{
		&PathPattern{}, &URLPattern{}, &UnionPattern{}, &AnyStringPatternElement{}, &SequenceStringPattern{},
		&HostPattern{}, &ListPattern{}, &ObjectPattern{}, &OptionPattern{},
		&RegexPattern{}, &TypePattern{}, &AnyPattern{}, &FunctionPattern{},
	}
	_ = []GroupPattern{
		&NamedSegmentPathPattern{},
	}

	ANY_PATTERN          = &AnyPattern{}
	ANY_STR_PATTERN_ELEM = &AnyStringPatternElement{}
	ANY_LIST_PATTERN     = &ListPattern{generalElement: ANY_PATTERN}
	ANY_TUPLE_PATTERN    = &TuplePattern{generalElement: ANY_PATTERN}

	ErrPatternNotCallable      = errors.New("pattern is not callable")
	ErrValueAlreadyInitialized = errors.New("value already initialized")
)

// A Pattern represents a symbolic Pattern.
type Pattern interface {
	SymbolicValue
	Iterable

	HasUnderylingPattern() bool

	//equivalent of Test() for concrete patterns
	TestValue(v SymbolicValue) bool

	Call(ctx *Context, values []SymbolicValue) (Pattern, error)

	//returns a symbolic value that represent all concrete values that match against this pattern
	SymbolicValue() SymbolicValue

	StringPattern() (StringPatternElement, bool)
}

type NotCallablePatternMixin struct {
}

func (NotCallablePatternMixin) Call(ctx *Context, values []SymbolicValue) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

// A GroupPattern represents a symbolic GroupPattern.
type GroupPattern interface {
	Pattern
	MatchGroups(SymbolicValue) (ok bool, groups map[string]SymbolicValue)
}

func isAnyPattern(val SymbolicValue) bool {
	_, ok := val.(*AnyPattern)
	return ok
}

// An AnyPattern represents a symbolic Pattern we do not know the concrete type.
type AnyPattern struct {
	NotCallablePatternMixin
	_ int
}

func (p *AnyPattern) Test(v SymbolicValue) bool {
	_, ok := v.(Pattern)
	return ok
}

func (p *AnyPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *AnyPattern) IsWidenable() bool {
	return false
}

func (p *AnyPattern) String() string {
	return "pattern"
}

func (p *AnyPattern) HasUnderylingPattern() bool {
	return false
}

func (p *AnyPattern) TestValue(SymbolicValue) bool {
	return true
}

func (p *AnyPattern) SymbolicValue() SymbolicValue {
	return ANY
}

func (p *AnyPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *AnyPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *AnyPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *AnyPattern) WidestOfType() SymbolicValue {
	return ANY_PATTERN
}

// A PathPattern represents a symbolic PathPattern.
type PathPattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	_ int
}

func (p *PathPattern) Test(v SymbolicValue) bool {
	_, ok := v.(*PathPattern)
	return ok
}

func (p *PathPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *PathPattern) IsWidenable() bool {
	return false
}

func (p *PathPattern) String() string {
	return "path-pattern"
}

func (p *PathPattern) HasUnderylingPattern() bool {
	return true
}

func (p *PathPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(*Path)
	return ok
}

func (p *PathPattern) SymbolicValue() SymbolicValue {
	return &Path{}
}

func (p *PathPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *PathPattern) PropertyNames() []string {
	return nil
}

func (*PathPattern) Prop(name string) SymbolicValue {
	switch name {
	default:
		return nil
	}
}

func (p *PathPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *PathPattern) IteratorElementValue() SymbolicValue {
	return &Path{}
}

func (p *PathPattern) underylingString() *String {
	return &String{}
}

func (p *PathPattern) WidestOfType() SymbolicValue {
	return &PathPattern{}
}

// A URLPattern represents a symbolic URLPattern.
type URLPattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	_ int
}

func (p *URLPattern) Test(v SymbolicValue) bool {
	_, ok := v.(*URLPattern)
	return ok
}

func (p *URLPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *URLPattern) IsWidenable() bool {
	return false
}

func (p *URLPattern) String() string {
	return "url-pattern"
}

func (p *URLPattern) HasUnderylingPattern() bool {
	return true
}

func (p *URLPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(*URL)
	return ok
}

func (p *URLPattern) SymbolicValue() SymbolicValue {
	return &URL{}
}

func (p *URLPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *URLPattern) PropertyNames() []string {
	return nil
}

func (*URLPattern) Prop(name string) SymbolicValue {
	switch name {
	default:
		return nil
	}
}

func (p *URLPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *URLPattern) IteratorElementValue() SymbolicValue {
	return &URL{}
}

func (p *URLPattern) underylingString() *String {
	return &String{}
}

func (p *URLPattern) WidestOfType() SymbolicValue {
	return &URLPattern{}
}

// A HostPattern represents a symbolic HostPattern.
type HostPattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	_ int
}

func (p *HostPattern) Test(v SymbolicValue) bool {
	_, ok := v.(*HostPattern)
	return ok
}

func (p *HostPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *HostPattern) IsWidenable() bool {
	return false
}

func (p *HostPattern) String() string {
	return "host-pattern"
}

func (p *HostPattern) HasUnderylingPattern() bool {
	return true
}

func (p *HostPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(*Host)
	return ok
}

func (p *HostPattern) SymbolicValue() SymbolicValue {
	return &Host{}
}

func (p *HostPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *HostPattern) PropertyNames() []string {
	return nil
}

func (*HostPattern) Prop(name string) SymbolicValue {
	switch name {
	default:
		return nil
	}
}

func (p *HostPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *HostPattern) IteratorElementValue() SymbolicValue {
	return &Host{}
}

func (p *HostPattern) underylingString() *String {
	return &String{}
}

func (p *HostPattern) WidestOfType() SymbolicValue {
	return &HostPattern{}
}

// A NamedSegmentPathPattern represents a symbolic NamedSegmentPathPattern.
type NamedSegmentPathPattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	node *parse.NamedSegmentPathPatternLiteral //if nil, any node is matched
}

func NewNamedSegmentPathPattern(node *parse.NamedSegmentPathPatternLiteral) *NamedSegmentPathPattern {
	return &NamedSegmentPathPattern{node: node}
}

func (p *NamedSegmentPathPattern) Test(v SymbolicValue) bool {
	otherPattern, ok := v.(*NamedSegmentPathPattern)
	if !ok {
		return false
	}

	return p.node == nil || p.node == otherPattern.node
}

func (p *NamedSegmentPathPattern) String() string {
	if p.node == nil {
		return "named-segment-path-pattern"
	}
	return fmt.Sprintf("named-segment-path-pattern(%p)", p.node)
}

func (p *NamedSegmentPathPattern) Widen() (SymbolicValue, bool) {
	if p.IsWidenable() {
		return &NamedSegmentPathPattern{node: nil}, true
	}
	return nil, false
}

func (p *NamedSegmentPathPattern) IsWidenable() bool {
	return p.node != nil
}

func (p NamedSegmentPathPattern) HasUnderylingPattern() bool {
	return true
}

func (p *NamedSegmentPathPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(*Path)
	return ok
}

func (p *NamedSegmentPathPattern) SymbolicValue() SymbolicValue {
	return &Path{}
}

func (p *NamedSegmentPathPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *NamedSegmentPathPattern) MatchGroups(v SymbolicValue) (bool, map[string]SymbolicValue) {
	//TODO
	return false, nil
}

func (p *NamedSegmentPathPattern) PropertyNames() []string {
	return nil
}

func (*NamedSegmentPathPattern) Prop(name string) SymbolicValue {
	switch name {
	default:
		return nil
	}
}

func (p *NamedSegmentPathPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *NamedSegmentPathPattern) IteratorElementValue() SymbolicValue {
	return &Path{}
}

func (p *NamedSegmentPathPattern) WidestOfType() SymbolicValue {
	return &NamedSegmentPathPattern{}
}

// An ExactValuePattern represents a symbolic ExactValuePattern.
type ExactValuePattern struct {
	NotCallablePatternMixin
	value SymbolicValue
}

func NewExactValuePattern(v SymbolicValue) *ExactValuePattern {
	return &ExactValuePattern{value: v}
}

func (p *ExactValuePattern) SetVal(v SymbolicValue) {
	if p.value != nil {
		panic(errors.New("value already set"))
	}
	p.value = v
}

// result should not be modified
func (p *ExactValuePattern) GetVal() SymbolicValue {
	return p.value
}

func (p *ExactValuePattern) Test(v SymbolicValue) bool {
	other, ok := v.(*ExactValuePattern)
	if !ok {
		return false
	}

	return p.value.Test(other.value)
}

func (p *ExactValuePattern) Widen() (SymbolicValue, bool) {
	if _, ok := p.value.(*Any); ok {
		return nil, false
	}
	return &ExactValuePattern{value: widenOrAny(p.value)}, true
}

func (p *ExactValuePattern) IsWidenable() bool {
	_, ok := p.value.(*Any)
	return !ok
}

func (p *ExactValuePattern) String() string {
	return fmt.Sprintf("exact-value-pattern(%s)", p.value.String())
}

func (p *ExactValuePattern) HasUnderylingPattern() bool {
	return true
}

func (p *ExactValuePattern) TestValue(v SymbolicValue) bool {
	return p.value.Test(v)
}

func (p *ExactValuePattern) SymbolicValue() SymbolicValue {
	return p.value
}

func (p *ExactValuePattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *ExactValuePattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *ExactValuePattern) IteratorElementValue() SymbolicValue {
	return p.value
}

func (p *ExactValuePattern) WidestOfType() SymbolicValue {
	return &ExactValuePattern{value: ANY}
}

// A RegexPattern represents a symbolic RegexPattern.
type RegexPattern struct {
	_ int
}

func (p *RegexPattern) Test(v SymbolicValue) bool {
	_, ok := v.(*RegexPattern)
	return ok
}

func (p *RegexPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *RegexPattern) IsWidenable() bool {
	return false
}

func (p *RegexPattern) String() string {
	return "regex-pattern"
}

func (p *RegexPattern) HasUnderylingPattern() bool {
	return true
}

func (p *RegexPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (p *RegexPattern) HasRegex() bool {
	return true
}

func (p *RegexPattern) Call(ctx *Context, values []SymbolicValue) (Pattern, error) {
	return &RegexPattern{}, nil
}

func (p *RegexPattern) SymbolicValue() SymbolicValue {
	return &RegexPattern{}
}

func (p *RegexPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *RegexPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *RegexPattern) IteratorElementValue() SymbolicValue {
	return &String{}
}

func (p *RegexPattern) WidestOfType() SymbolicValue {
	return &RegexPattern{}
}

// An ObjectPattern represents a symbolic ObjectPattern.
type ObjectPattern struct {
	NotCallablePatternMixin
	Entries                    map[string]Pattern
	Inexact                    bool
	ComplexPropertyConstraints []*ComplexPropertyConstraint
}

func NewAnyObjectPattern() *ObjectPattern {
	return &ObjectPattern{}
}

func newExactObjectPattern(entries map[string]Pattern) *ObjectPattern {
	return &ObjectPattern{Entries: entries}
}

func NewUnitializedObjectPattern() *ObjectPattern {
	return &ObjectPattern{}
}

func InitializeObjectPattern(patt *ObjectPattern, entries map[string]Pattern, inexact bool) {
	if patt.Entries != nil || patt.ComplexPropertyConstraints != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.Entries = entries
	patt.Inexact = inexact
}

func (p *ObjectPattern) ToRecordPattern() *RecordPattern {
	if p.Entries == nil {
		return NewAnyRecordPattern()
	}
	patt := NewUnitializedRecordPattern()
	//TODO: check that SymbolicValue() of entry patterns are immutable
	InitializeRecordPattern(patt, p.Entries, p.Inexact)
	return patt
}

func (p *ObjectPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*ObjectPattern)

	if !ok || p.Inexact != other.Inexact {
		return false
	}

	if p.Entries == nil {
		return true
	}

	if other.Entries == nil || len(p.Entries) != len(other.Entries) {
		return false
	}

	for k, v := range p.Entries {
		otherV, ok := other.Entries[k]
		if !ok || !v.Test(otherV) {
			return false
		}
	}

	return true
}

func (p *ObjectPattern) Widen() (SymbolicValue, bool) {
	if p.Entries == nil {
		return nil, false
	}

	if len(p.Entries) == 0 {
		return &ObjectPattern{}, true
	}

	widenedEntries := make(map[string]Pattern)
	allAlreadyWidened := true

	for k, v := range p.Entries {
		if val, ok := v.Widen(); ok {
			allAlreadyWidened = false
			widenedEntries[k] = val.(Pattern)
		}
	}

	if allAlreadyWidened {
		if !p.Inexact {
			entries := make(map[string]Pattern)

			for k, v := range p.Entries {
				entries[k] = v
			}
			return &ObjectPattern{Entries: entries, Inexact: true}, true
		}

		return &ObjectPattern{}, true
	}

	return &ObjectPattern{Entries: widenedEntries, Inexact: p.Inexact}, true
}

func (p *ObjectPattern) IsWidenable() bool {
	return p.Entries != nil
}

func (p *ObjectPattern) String() string {
	if p.Entries != nil {
		buff := bytes.NewBufferString("%{")
		i := 0
		for k, pattern := range p.Entries {
			if i > 0 {
				buff.WriteRune(',')
			}
			buff.WriteString(k)
			buff.WriteRune(':')
			buff.WriteString(pattern.String())
			i++
		}
		if p.Inexact {
			buff.WriteString(", ...")
		}
		buff.WriteRune('}')
		return buff.String()
	}
	return "object-pattern"
}

func (p *ObjectPattern) HasUnderylingPattern() bool {
	return true
}

func (p *ObjectPattern) TestValue(v SymbolicValue) bool {
	obj, ok := v.(*Object)
	if !ok {
		return false
	}

	if p.Entries == nil {
		return true
	}

	if p.Inexact {
		if obj.entries == nil {
			return false
		}
	} else if obj.entries == nil || len(p.Entries) != len(obj.entries) {
		return false
	}

	for key, valuePattern := range p.Entries {
		value, _, ok := obj.GetProperty(key)
		if !ok || !valuePattern.TestValue(value) {
			return false
		}
	}

	return true
}

func (p *ObjectPattern) SymbolicValue() SymbolicValue {
	entries := map[string]SymbolicValue{}
	static := map[string]Pattern{}

	if p.Entries != nil {
		for key, valuePattern := range p.Entries {
			entries[key] = valuePattern.SymbolicValue()
			static[key] = valuePattern
		}
	}

	return NewObject(entries, static)
}

func (p *ObjectPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *ObjectPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *ObjectPattern) IteratorElementValue() SymbolicValue {
	return &Object{}
}

func (p *ObjectPattern) WidestOfType() SymbolicValue {
	return &ObjectPattern{}
}

// An RecordPattern represents a symbolic RecordPattern.
type RecordPattern struct {
	NotCallablePatternMixin
	Entries                    map[string]Pattern
	Inexact                    bool
	ComplexPropertyConstraints []*ComplexPropertyConstraint
}

func NewAnyRecordPattern() *RecordPattern {
	return &RecordPattern{}
}

func NewUnitializedRecordPattern() *RecordPattern {
	return &RecordPattern{}
}

func InitializeRecordPattern(patt *RecordPattern, entries map[string]Pattern, inexact bool) {
	if patt.Entries != nil || patt.ComplexPropertyConstraints != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.Entries = entries
	patt.Inexact = inexact
}

func (p *RecordPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*RecordPattern)

	if !ok || p.Inexact != other.Inexact {
		return false
	}

	if p.Entries == nil {
		return true
	}

	if other.Entries == nil || len(p.Entries) != len(other.Entries) {
		return false
	}

	for k, v := range p.Entries {
		otherV, ok := other.Entries[k]
		if !ok || !v.Test(otherV) {
			return false
		}
	}

	return true
}

func (p *RecordPattern) Widen() (SymbolicValue, bool) {
	if p.Entries == nil {
		return nil, false
	}

	if len(p.Entries) == 0 {
		return &RecordPattern{}, true
	}

	widenedEntries := make(map[string]Pattern)
	allAlreadyWidened := true

	for k, v := range p.Entries {
		if val, ok := v.Widen(); ok {
			allAlreadyWidened = false
			widenedEntries[k] = val.(Pattern)
		}
	}

	if allAlreadyWidened {
		if !p.Inexact {
			entries := make(map[string]Pattern)

			for k, v := range p.Entries {
				entries[k] = v
			}
			return &RecordPattern{Entries: entries, Inexact: true}, true
		}

		return &RecordPattern{}, true
	}

	return &RecordPattern{Entries: widenedEntries, Inexact: p.Inexact}, true
}

func (p *RecordPattern) IsWidenable() bool {
	return p.Entries != nil
}

func (p *RecordPattern) String() string {
	if p.Entries != nil {
		buff := bytes.NewBufferString("%record(%{")
		i := 0
		for k, pattern := range p.Entries {
			if i > 0 {
				buff.WriteRune(',')
			}
			buff.WriteString(k)
			buff.WriteRune(':')
			buff.WriteString(pattern.String())
			i++
		}
		if p.Inexact {
			buff.WriteString(", ...")
		}
		buff.WriteString("})")
		return buff.String()
	}
	return "record-pattern"
}

func (p *RecordPattern) HasUnderylingPattern() bool {
	return true
}

func (p *RecordPattern) TestValue(v SymbolicValue) bool {
	rec, ok := v.(*Record)
	if !ok {
		return false
	}

	if p.Entries == nil {
		return true
	}

	if p.Inexact {
		if rec.entries == nil {
			return false
		}
	} else if rec.entries == nil || len(p.Entries) != len(rec.entries) {
		return false
	}

	for key, valuePattern := range p.Entries {
		value, ok := rec.entries[key]
		if !ok || !valuePattern.TestValue(value) {
			return false
		}
	}

	return true
}

func (p *RecordPattern) SymbolicValue() SymbolicValue {
	rec := &Record{
		entries: map[string]SymbolicValue{},
	}
	if p.Entries != nil {
		for key, valuePattern := range p.Entries {
			rec.entries[key] = valuePattern.SymbolicValue()
		}
	}

	return rec
}

func (p *RecordPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *RecordPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *RecordPattern) IteratorElementValue() SymbolicValue {
	return &Object{}
}

func (p *RecordPattern) WidestOfType() SymbolicValue {
	return &RecordPattern{}
}

type ComplexPropertyConstraint struct {
	NotCallablePatternMixin
	Properties []string
	Expr       parse.Node
}

// A ListPattern represents a symbolic ListPattern.
// .elements and .generalElement can never be both nil (nor both not nil).
type ListPattern struct {
	NotCallablePatternMixin
	elements       []Pattern
	generalElement Pattern
}

func NewListPattern(elements []Pattern) *ListPattern {
	return &ListPattern{elements: elements}
}

func NewListPatternOf(generalElement Pattern) *ListPattern {
	return &ListPattern{generalElement: generalElement}
}

func InitializeListPatternElements(patt *ListPattern, elements []Pattern) {
	if patt.elements != nil || patt.generalElement != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.elements = elements
}

func InitializeListPatternGeneralElement(patt *ListPattern, element Pattern) {
	if patt.elements != nil || patt.generalElement != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.generalElement = element
}

func (p *ListPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*ListPattern)

	if !ok {
		return false
	}

	if p.elements != nil {
		if other.elements == nil || len(p.elements) != len(other.elements) {
			return false
		}

		for i, e := range p.elements {
			if !e.Test(other.elements[i]) {
				return false
			}
		}

		return true
	} else {
		if other.elements == nil {
			return p.generalElement.Test(other.generalElement)
		}

		for _, elem := range other.elements {
			if !p.generalElement.Test(elem) {
				return false
			}
		}
		return true
	}
}

func (p *ListPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *ListPattern) IsWidenable() bool {
	return false
}

func (p *ListPattern) String() string {
	if p.elements != nil {
		buff := bytes.NewBufferString("%[")
		for i, elem := range p.elements {
			if i > 0 {
				buff.WriteRune(',')
			}
			buff.WriteString(elem.SymbolicValue().String())
		}
		buff.WriteRune(']')
		return buff.String()
	}
	return "%[]" + p.generalElement.SymbolicValue().String()
}

func (p *ListPattern) HasUnderylingPattern() bool {
	return true
}

func (p *ListPattern) TestValue(v SymbolicValue) bool {
	list, ok := v.(*List)
	if !ok {
		return false
	}

	if p.elements != nil {
		if !list.HasKnownLen() || list.knownLen() != len(p.elements) {
			return false
		}
		for i, e := range p.elements {
			if !e.TestValue(list.elements[i]) {
				return false
			}
		}
		return true
	} else {
		if list.HasKnownLen() {
			for _, e := range list.elements {
				if !p.generalElement.TestValue(e) {
					return false
				}
			}

			return true
		} else if p.generalElement.TestValue(list.generalElement) {
			return true
		}

		return false
	}

}

func (p *ListPattern) SymbolicValue() SymbolicValue {
	list := &List{}

	if p.elements != nil {
		list.elements = make([]SymbolicValue, 0)
		for _, e := range p.elements {
			list.elements = append(list.elements, e.SymbolicValue())
		}
	} else {
		list.generalElement = p.generalElement.SymbolicValue()
	}
	return list
}

func (p *ListPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *ListPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *ListPattern) IteratorElementValue() SymbolicValue {
	return &List{}
}

func (p *ListPattern) WidestOfType() SymbolicValue {
	return &ListPattern{}
}

// A TuplePattern represents a symbolic TuplePattern.
// .elements and .generalElement can never be both nil (nor both not nil).
type TuplePattern struct {
	NotCallablePatternMixin
	elements       []Pattern
	generalElement Pattern
}

func NewTuplePattern(elements []Pattern) *TuplePattern {
	return &TuplePattern{elements: elements}
}

func NewTuplePatternOf(generalElement Pattern) *TuplePattern {
	return &TuplePattern{generalElement: generalElement}
}

func InitializeTuplePatternElements(patt *TuplePattern, elements []Pattern) {
	if patt.elements != nil || patt.generalElement != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.elements = elements
}

func InitializeTuplePatternGeneralElement(patt *TuplePattern, element Pattern) {
	if patt.elements != nil || patt.generalElement != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.generalElement = element
}

func (p *TuplePattern) Test(v SymbolicValue) bool {
	other, ok := v.(*TuplePattern)

	if !ok {
		return false
	}

	if p.elements != nil {
		if other.elements == nil || len(p.elements) != len(other.elements) {
			return false
		}

		for i, e := range p.elements {
			if !e.Test(other.elements[i]) {
				return false
			}
		}

		return true
	} else {
		if other.elements == nil {
			return p.generalElement.Test(other.generalElement)
		}

		for _, elem := range other.elements {
			if !p.generalElement.Test(elem) {
				return false
			}
		}
		return true
	}
}

func (p *TuplePattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *TuplePattern) IsWidenable() bool {
	return false
}

func (p *TuplePattern) String() string {
	if p.elements != nil {
		buff := bytes.NewBufferString("%tuple([")
		for i, elem := range p.elements {
			if i > 0 {
				buff.WriteRune(',')
			}
			buff.WriteString(elem.SymbolicValue().String())
		}
		buff.WriteString("])")
		return buff.String()
	}
	return "%tuple(" + p.generalElement.SymbolicValue().String() + ")"
}

func (p *TuplePattern) HasUnderylingPattern() bool {
	return true
}

func (p *TuplePattern) TestValue(v SymbolicValue) bool {
	tuple, ok := v.(*Tuple)
	if !ok {
		return false
	}

	if p.elements != nil {
		if !tuple.HasKnownLen() || tuple.knownLen() != len(p.elements) {
			return false
		}
		for i, e := range p.elements {
			if !e.TestValue(tuple.elements[i]) {
				return false
			}
		}
		return true
	} else {
		if tuple.HasKnownLen() {
			for _, e := range tuple.elements {
				if !p.generalElement.TestValue(e) {
					return false
				}
			}

			return true
		} else if p.generalElement.TestValue(tuple.generalElement) {
			return true
		}

		return false
	}

}

func (p *TuplePattern) SymbolicValue() SymbolicValue {
	tuple := &Tuple{}

	if p.elements != nil {
		tuple.elements = make([]SymbolicValue, 0)
		for _, e := range p.elements {
			tuple.elements = append(tuple.elements, e.SymbolicValue())
		}
	} else {
		tuple.generalElement = p.generalElement.SymbolicValue()
	}
	return tuple
}

func (p *TuplePattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *TuplePattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *TuplePattern) IteratorElementValue() SymbolicValue {
	return &List{}
}

func (p *TuplePattern) WidestOfType() SymbolicValue {
	return &TuplePattern{}
}

// A UnionPattern represents a symbolic UnionPattern.
type UnionPattern struct {
	NotCallablePatternMixin
	Cases []Pattern //if nil, any union pattern is matched
}

func (p *UnionPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*UnionPattern)

	if !ok {
		return false
	}

	if p.Cases == nil {
		return true
	}

	if len(p.Cases) != len(other.Cases) {
		return false
	}

	for i, case_ := range p.Cases {
		if !case_.Test(other.Cases[i]) {
			return false
		}
	}

	return true
}

func (p *UnionPattern) Widen() (SymbolicValue, bool) {
	if p.IsWidenable() {
		return &UnionPattern{Cases: nil}, true
	}
	return nil, false
}

func (p *UnionPattern) IsWidenable() bool {
	return p.Cases != nil
}

func (p *UnionPattern) String() string {
	buff := bytes.NewBufferString("(%| ")
	for i, case_ := range p.Cases {
		if i > 0 {
			buff.WriteString(" | ")
		}
		buff.WriteString(case_.String())
	}
	buff.WriteRune(')')
	return buff.String()
}

func (p *UnionPattern) HasUnderylingPattern() bool {
	return true
}

func (p *UnionPattern) TestValue(v SymbolicValue) bool {
	for _, case_ := range p.Cases {
		if case_.TestValue(v) {
			return true
		}
	}

	return false

}

func (p *UnionPattern) SymbolicValue() SymbolicValue {
	values := make([]SymbolicValue, len(p.Cases))

	for i, case_ := range p.Cases {
		values[i] = case_.SymbolicValue()
	}

	return joinValues(values)
}

func (p *UnionPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *UnionPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *UnionPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *UnionPattern) WidestOfType() SymbolicValue {
	return &UnionPattern{}
}

// An IntersectionPattern represents a symbolic IntersectionPattern.
type IntersectionPattern struct {
	NotCallablePatternMixin
	Cases []Pattern //if nil, any union pattern is matched
}

func (p *IntersectionPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*IntersectionPattern)

	if !ok {
		return false
	}

	if p.Cases == nil {
		return true
	}

	if len(p.Cases) != len(other.Cases) {
		return false
	}

	for i, case_ := range p.Cases {
		if !case_.Test(other.Cases[i]) {
			return false
		}
	}

	return true
}

func (p *IntersectionPattern) Widen() (SymbolicValue, bool) {
	if p.IsWidenable() {
		return &IntersectionPattern{Cases: nil}, true
	}
	return nil, false
}

func (p *IntersectionPattern) IsWidenable() bool {
	return p.Cases != nil
}

func (p *IntersectionPattern) String() string {
	buff := bytes.NewBufferString("(%& ")
	for i, case_ := range p.Cases {
		if i > 0 {
			buff.WriteString(" & ")
		}
		buff.WriteString(case_.String())
	}
	buff.WriteRune(')')
	return buff.String()
}

func (p *IntersectionPattern) HasUnderylingPattern() bool {
	return true
}

func (p *IntersectionPattern) TestValue(v SymbolicValue) bool {
	for _, case_ := range p.Cases {
		if !case_.TestValue(v) {
			return false
		}
	}
	return true

}

func (p *IntersectionPattern) SymbolicValue() SymbolicValue {
	//TODO: implement
	return ANY
}

func (p *IntersectionPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *IntersectionPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *IntersectionPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *IntersectionPattern) WidestOfType() SymbolicValue {
	return &IntersectionPattern{}
}

// A OptionPattern represents a symbolic OptionPattern.
type OptionPattern struct {
	NotCallablePatternMixin
	_ int
}

func (p *OptionPattern) Test(v SymbolicValue) bool {
	_, ok := v.(*OptionPattern)
	return ok
}

func (p *OptionPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *OptionPattern) IsWidenable() bool {
	return false
}

func (p *OptionPattern) String() string {
	return "option-pattern"
}

func (p *OptionPattern) HasUnderylingPattern() bool {
	return true
}

func (p *OptionPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(*Option)
	return ok
}

func (p *OptionPattern) SymbolicValue() SymbolicValue {
	return &Option{}
}

func (p *OptionPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *OptionPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *OptionPattern) IteratorElementValue() SymbolicValue {
	return &Option{}
}

func (p *OptionPattern) WidestOfType() SymbolicValue {
	return &OptionPattern{}
}

func symbolicallyEvalPatternNode(n parse.Node, state *State) (Pattern, error) {
	switch node := n.(type) {
	case *parse.ObjectPatternLiteral,
		*parse.ListPatternLiteral,
		*parse.OptionPatternLiteral,
		*parse.RegularExpressionLiteral,

		*parse.PatternUnion,
		*parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
		*parse.FunctionPatternExpression,
		*parse.PatternCallExpression,
		*parse.OptionalPatternExpression:
		pattern, err := symbolicEval(node, state)
		if err != nil {
			return nil, err
		}

		return pattern.(Pattern), nil
	case *parse.ComplexStringPatternPiece:
		return &SequenceStringPattern{}, nil
	default:
		v, err := symbolicEval(n, state)
		if err != nil {
			return nil, err
		}

		return &ExactValuePattern{value: v}, nil
	}
}

type TypePattern struct {
	val           SymbolicValue //symbolic value that represents concrete values matching against the function
	call          func(ctx *Context, values []SymbolicValue) (Pattern, error)
	stringPattern func() (StringPatternElement, bool)
}

func NewTypePattern(
	value SymbolicValue, call func(ctx *Context, values []SymbolicValue) (Pattern, error),
	stringPattern func() (StringPatternElement, bool),
) *TypePattern {
	return &TypePattern{
		val:           value,
		call:          call,
		stringPattern: stringPattern,
	}
}

func (p *TypePattern) Test(v SymbolicValue) bool {
	other, ok := v.(*TypePattern)
	return ok && p.val.Test(other.val)
}

func (p *TypePattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *TypePattern) IsWidenable() bool {
	return false
}

func (p *TypePattern) String() string {
	return fmt.Sprintf("type-pattern(%s)", p.val.String())
}

func (p *TypePattern) HasUnderylingPattern() bool {
	return true
}

func (p *TypePattern) TestValue(v SymbolicValue) bool {
	return p.val.Test(v)
}

func (p *TypePattern) Call(ctx *Context, values []SymbolicValue) (Pattern, error) {
	if p.call == nil {
		return nil, ErrPatternNotCallable
	}
	return p.call(ctx, values)
}

func (p *TypePattern) SymbolicValue() SymbolicValue {
	return p.val
}

func (p *TypePattern) StringPattern() (StringPatternElement, bool) {
	if p.stringPattern == nil {
		return nil, false
	}
	return p.stringPattern()
}

func (p *TypePattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *TypePattern) IteratorElementValue() SymbolicValue {
	return nil
}

func (p *TypePattern) WidestOfType() SymbolicValue {
	return &TypePattern{}
}

type DifferencePattern struct {
	NotCallablePatternMixin
	Base    Pattern
	Removed Pattern
}

func (p *DifferencePattern) Test(v SymbolicValue) bool {
	other, ok := v.(*DifferencePattern)
	return ok && p.Base.Test(other.Base) && other.Removed.Test(other.Removed)
}

func (p *DifferencePattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *DifferencePattern) IsWidenable() bool {
	return false
}

func (p *DifferencePattern) String() string {
	return fmt.Sprintf("difference-pattern(%s \\ %s)", p.Base.String(), p.Removed.String())
}

func (p *DifferencePattern) HasUnderylingPattern() bool {
	return true
}

func (p *DifferencePattern) TestValue(v SymbolicValue) bool {
	return p.Base.Test(v) && !p.Removed.TestValue(v)
}

func (p *DifferencePattern) SymbolicValue() SymbolicValue {
	//TODO
	panic(errors.New("SymbolicValue() not implement for DifferencePattern"))
}

func (p *DifferencePattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *DifferencePattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *DifferencePattern) IteratorElementValue() SymbolicValue {
	//TODO
	return ANY
}

func (p *DifferencePattern) WidestOfType() SymbolicValue {
	return &DifferencePattern{}
}

type OptionalPattern struct {
	NotCallablePatternMixin
	pattern Pattern
}

func NewOptionalPattern(p Pattern) *OptionalPattern {
	return &OptionalPattern{pattern: p}
}

func (p *OptionalPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*OptionalPattern)
	return ok && p.pattern.Test(other.pattern)
}

func (p *OptionalPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *OptionalPattern) IsWidenable() bool {
	return false
}

func (p *OptionalPattern) String() string {
	return fmt.Sprintf("%s?", p.pattern)
}

func (p *OptionalPattern) HasUnderylingPattern() bool {
	return true
}

func (p *OptionalPattern) TestValue(v SymbolicValue) bool {
	if _, ok := v.(*NilT); ok {
		return true
	}
	return p.pattern.TestValue(v)
}

func (p *OptionalPattern) SymbolicValue() SymbolicValue {
	//TODO
	return NewMultivalue(p.pattern.SymbolicValue(), Nil)
}

func (p *OptionalPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *OptionalPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *OptionalPattern) IteratorElementValue() SymbolicValue {
	//TODO
	return ANY
}

func (p *OptionalPattern) WidestOfType() SymbolicValue {
	return &OptionalPattern{}
}

type FunctionPattern struct {
	NotCallablePatternMixin
	Node       *parse.FunctionPatternExpression //if nil, any function is matched
	ReturnType SymbolicValue
}

func (fn *FunctionPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*FunctionPattern)
	if !ok {
		return false
	}
	if fn.Node == nil {
		return true
	}

	if other.Node == nil {
		return false
	}

	return utils.SamePointer(fn.Node, other.Node)
}

func (pattern *FunctionPattern) TestValue(v SymbolicValue) bool {
	switch fn := v.(type) {
	case *Function:
		if pattern.Node == nil {
			return true
		}
		return pattern.Test(fn.pattern)
	case *GoFunction:
		if pattern.Node == nil {
			return true
		}

		if fn.fn == nil {
			return false
		}

		panic(errors.New("testing a go function against a function pattern is not supported yet"))

	case *InoxFunction:
		if pattern.Node == nil {
			return true
		}

		fnExpr := fn.FuncExpr()
		if fnExpr == nil {
			return false
		}

		if len(fnExpr.Parameters) != len(pattern.Node.Parameters) || fnExpr.NonVariadicParamCount() != pattern.Node.NonVariadicParamCount() {
			return false
		}

		for i, param := range pattern.Node.Parameters {
			actualParam := fnExpr.Parameters[i]

			if (param.Type == nil) != (actualParam.Type == nil) {
				return false
			}

			if parse.SPrint(param.Type, parse.PrintConfig{TrimStart: true}) != parse.SPrint(actualParam.Type, parse.PrintConfig{TrimStart: true}) {
				return false
			}
		}

		return pattern.ReturnType.Test(fn.returnType)
	default:
		return false
	}
}

func (fn *FunctionPattern) HasUnderylingPattern() bool {
	return true
}

func (fn *FunctionPattern) Widen() (SymbolicValue, bool) {
	if fn.Node == nil {
		return nil, false
	}
	return &FunctionPattern{}, true
}

func (fn *FunctionPattern) IsWidenable() bool {
	return fn.Node != nil
}

func (p *FunctionPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (fn *FunctionPattern) IteratorElementValue() SymbolicValue {
	//TODO
	return &Function{pattern: fn}
}

func (fn *FunctionPattern) SymbolicValue() SymbolicValue {
	return &Function{pattern: fn}
}

func (p *FunctionPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (fn *FunctionPattern) String() string {
	if fn.Node == nil {
		return "function-pattern"
	}
	return fmt.Sprintf("function-pattern(%v)", fn.Node)
}

func (fn *FunctionPattern) WidestOfType() SymbolicValue {
	return &FunctionPattern{}
}

// A IntRangePattern represents a symbolic IntRangePattern.
type IntRangePattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	_ int
}

func (p *IntRangePattern) Test(v SymbolicValue) bool {
	_, ok := v.(*IntRangePattern)
	return ok
}

func (p *IntRangePattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *IntRangePattern) IsWidenable() bool {
	return false
}

func (p *IntRangePattern) String() string {
	return "int-range-pattern"
}

func (p *IntRangePattern) HasUnderylingPattern() bool {
	return true
}

func (p *IntRangePattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(*URL)
	return ok
}

func (p *IntRangePattern) SymbolicValue() SymbolicValue {
	return &URL{}
}

func (p *IntRangePattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *IntRangePattern) PropertyNames() []string {
	return nil
}

func (*IntRangePattern) Prop(name string) SymbolicValue {
	switch name {
	default:
		return nil
	}
}

func (p *IntRangePattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *IntRangePattern) IteratorElementValue() SymbolicValue {
	return &URL{}
}

func (p *IntRangePattern) underylingString() *String {
	return &String{}
}

func (p *IntRangePattern) WidestOfType() SymbolicValue {
	return &IntRangePattern{}
}

// An EventPattern represents a symbolic EventPattern.
type EventPattern struct {
	NotCallablePatternMixin
	ValuePattern Pattern
}

func NewEventPattern(valuePattern Pattern) (*EventPattern, error) {
	if !isAnyPattern(valuePattern) && valuePattern.SymbolicValue().IsMutable() {
		return nil, fmt.Errorf("failed to create event pattern: value should be immutable: %T", valuePattern.SymbolicValue())
	}

	return &EventPattern{
		ValuePattern: valuePattern,
	}, nil
}

func (p *EventPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*EventPattern)
	return ok && p.ValuePattern.Test(other.ValuePattern)
}

func (p *EventPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *EventPattern) IsWidenable() bool {
	return false
}

func (p *EventPattern) String() string {
	return fmt.Sprintf("%%event(%s)", p.ValuePattern)
}

func (p *EventPattern) HasUnderylingPattern() bool {
	return true
}

func (p *EventPattern) TestValue(v SymbolicValue) bool {
	event, ok := v.(*Event)
	if !ok {
		return false
	}
	return p.ValuePattern.TestValue(event)
}

func (p *EventPattern) SymbolicValue() SymbolicValue {
	return utils.Must(NewEvent(p.ValuePattern.SymbolicValue()))
}

func (p *EventPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *EventPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *EventPattern) IteratorElementValue() SymbolicValue {
	//TODO
	return ANY
}

func (p *EventPattern) WidestOfType() SymbolicValue {
	return &EventPattern{ValuePattern: ANY_PATTERN}
}

// An Event
type MutationPattern struct {
	NotCallablePatternMixin
	kind         *Int
	data0Pattern Pattern
}

func NewMutationPattern(kind *Int, data0Pattern Pattern) *MutationPattern {
	return &MutationPattern{
		kind:         kind,
		data0Pattern: data0Pattern,
	}
}

func (p *MutationPattern) Test(v SymbolicValue) bool {
	other, ok := v.(*MutationPattern)
	return ok && p.kind.Test(other.kind) && p.data0Pattern.Test(other.data0Pattern)
}

func (p *MutationPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *MutationPattern) IsWidenable() bool {
	return false
}

func (p *MutationPattern) String() string {
	return fmt.Sprintf("%%mutation(%d, %s)", p.kind, p.data0Pattern)
}

func (p *MutationPattern) HasUnderylingPattern() bool {
	return true
}

func (p *MutationPattern) TestValue(v SymbolicValue) bool {
	event, ok := v.(*Event)
	if !ok {
		return false
	}
	return p.data0Pattern.TestValue(event)
}

func (p *MutationPattern) SymbolicValue() SymbolicValue {
	//TODO
	return &Mutation{}
}

func (p *MutationPattern) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *MutationPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *MutationPattern) IteratorElementValue() SymbolicValue {
	//TODO
	return &Mutation{}
}

func (p *MutationPattern) WidestOfType() SymbolicValue {
	return &MutationPattern{}
}

// A PatternNamespace represents a symbolic PatternNamespace.
type PatternNamespace struct {
	entries map[string]Pattern //if nil, matches any pattern namespace
}

func NewPatternNamespace(patterns map[string]Pattern) *PatternNamespace {
	return &PatternNamespace{
		entries: utils.CopyMap(patterns),
	}
}

func (ns *PatternNamespace) Test(v SymbolicValue) bool {
	otherNS, ok := v.(*PatternNamespace)
	if !ok {
		return false
	}

	if ns.entries == nil {
		return true
	}

	if len(ns.entries) != len(otherNS.entries) || otherNS.entries == nil {
		return false
	}

	for i, e := range ns.entries {
		if !e.Test(otherNS.entries[i]) {
			return false
		}
	}
	return true
}

func (ns *PatternNamespace) Widen() (SymbolicValue, bool) {
	if ns.entries == nil {
		return nil, false
	}

	widenedPatterns := map[string]Pattern{}
	allAlreadyWidened := true

	for k, v := range ns.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened.(Pattern)
		}
		widenedPatterns[k] = v
	}

	if allAlreadyWidened {
		return &PatternNamespace{}, true
	}

	return &PatternNamespace{entries: widenedPatterns}, true
}

func (ns *PatternNamespace) IsWidenable() bool {
	_, ok := ns.Widen()
	return ok
}

func (ns *PatternNamespace) String() string {
	if ns.entries != nil {
		buff := bytes.NewBufferString("pattern-namespace{")
		i := 0
		for k, pattern := range ns.entries {
			if i > 0 {
				buff.WriteRune(',')
			}
			buff.WriteString(k)
			buff.WriteRune(':')
			buff.WriteString(pattern.String())
			i++
		}
		buff.WriteRune('}')
		return buff.String()
	}
	return "pattern-namespace"
}

func (ns *PatternNamespace) WidestOfType() SymbolicValue {
	return &PatternNamespace{}
}
