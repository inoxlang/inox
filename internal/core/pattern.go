package core

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrPatternNotCallable = errors.New("pattern is not callable")
	ErrNoDefaultValue     = errors.New("no default value")

	_ = []GroupPattern{&NamedSegmentPathPattern{}}
)

func RegisterDefaultPattern(s string, m Pattern) {
	if _, ok := DEFAULT_NAMED_PATTERNS[s]; ok {
		panic(fmt.Errorf("pattern '%s' is already registered", s))
	}
	DEFAULT_NAMED_PATTERNS[s] = m
}

func RegisterDefaultPatternNamespace(s string, ns *PatternNamespace) {
	if _, ok := DEFAULT_PATTERN_NAMESPACES[s]; ok {
		panic(fmt.Errorf("pattern namespace '%s' is already registered", s))
	}
	DEFAULT_PATTERN_NAMESPACES[s] = ns
}

type Pattern interface {
	Serializable
	Iterable

	//Test returns true if the argument matches the pattern.
	Test(*Context, Value) bool

	Random(ctx *Context, options ...Option) Value

	Call(values []Serializable) (Pattern, error)

	StringPattern() (StringPattern, bool)
}

type GroupMatchesFindConfigKind int

const (
	FindFirstGroupMatches GroupMatchesFindConfigKind = iota
	FindAllGroupMatches
)

type GroupMatchesFindConfig struct {
	Kind GroupMatchesFindConfigKind
}

type GroupPattern interface {
	Pattern
	MatchGroups(*Context, Serializable) (groups map[string]Serializable, ok bool, err error)
	FindGroupMatches(*Context, Serializable, GroupMatchesFindConfig) (groups []*Object, err error)
}

// DefaultValuePattern is implemented by patterns that can provide
// a default value that matches them in most cases. ErrNoDefaultValue should be returned if it's not possible.
type DefaultValuePattern interface {
	Pattern
	DefaultValue(ctx *Context) (Value, error)
}

type PatternNamespace struct {
	Patterns map[string]Pattern
}

type NotCallablePatternMixin struct {
}

func (NotCallablePatternMixin) Call(values []Serializable) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

// ExactValuePattern matches values equal to .value: .value.Equal(...) returns true.
type ExactValuePattern struct {
	value Serializable //immutable in most cases
	CallBasedPatternReprMixin

	NotCallablePatternMixin
}

func NewExactValuePattern(value Serializable) *ExactValuePattern {
	if value.IsMutable() {
		panic(ErrValueInExactPatternValueShouldBeImmutable)
	}
	return newExactValuePatternNoCheck(value)
}

func newExactValuePatternNoCheck(value Serializable) *ExactValuePattern {
	return &ExactValuePattern{
		value: value,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: getDefaultNamedPattern("__val"),
			Params: []Serializable{utils.Must(value.Clone(map[uintptr]map[int]Value{}, 0)).(Serializable)},
		},
	}
}

func NewMostAdaptedExactPattern(value Serializable) Pattern {
	if value.IsMutable() {
		panic(ErrValueInExactPatternValueShouldBeImmutable)
	}
	if s, ok := value.(StringLike); ok {
		return NewExactStringPattern(Str(s.GetOrBuildString()))
	}
	return NewExactValuePattern(value)
}

func (pattern *ExactValuePattern) Test(ctx *Context, v Value) bool {
	return pattern.value.Equal(ctx, v, map[uintptr]uintptr{}, 0)
}

func (patt *ExactValuePattern) StringPattern() (StringPattern, bool) {
	if str, ok := patt.value.(StringLike); ok {
		stringPattern := NewExactStringPattern(Str(str.GetOrBuildString()))
		return stringPattern, true
	}
	return nil, false
}

// TypePattern matches values implementing .Type (if .Type is an interface) or having their type equal to .Type
type TypePattern struct {
	Type          reflect.Type
	Name          string
	SymbolicValue symbolic.SymbolicValue
	RandomImpl    func(options ...Option) Value

	CallImpl         func(pattern *TypePattern, values []Serializable) (Pattern, error)
	SymbolicCallImpl func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error)

	stringPattern         func() (StringPattern, bool)
	symbolicStringPattern func() (symbolic.StringPattern, bool)
}

func (pattern *TypePattern) Test(ctx *Context, v Value) bool {
	if pattern.Type.Kind() == reflect.Interface {
		return reflect.TypeOf(v).Implements(pattern.Type)
	}
	return pattern.Type == reflect.TypeOf(v)
}

func (patt *TypePattern) Call(values []Serializable) (Pattern, error) {
	if patt.CallImpl == nil {
		return nil, ErrPatternNotCallable
	}
	return patt.CallImpl(patt, values)
}

func (patt *TypePattern) StringPattern() (StringPattern, bool) {
	if patt.stringPattern == nil {
		return nil, false
	}

	return patt.stringPattern()
}

type UnionPattern struct {
	NotCallablePatternMixin
	node  parse.Node
	cases []Pattern
}

func NewUnionPattern(cases []Pattern, node parse.Node) *UnionPattern {
	return &UnionPattern{node: node, cases: cases}
}

func (patt *UnionPattern) Test(ctx *Context, v Value) bool {
	for _, case_ := range patt.cases {
		if case_.Test(ctx, v) {
			return true
		}
	}
	return false
}

func (patt *UnionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type IntersectionPattern struct {
	NotCallablePatternMixin

	node  parse.Node
	cases []Pattern
}

func NewIntersectionPattern(cases []Pattern, node parse.Node) *IntersectionPattern {
	return &IntersectionPattern{node: node, cases: cases}
}

func (patt *IntersectionPattern) Test(ctx *Context, v Value) bool {
	for _, case_ := range patt.cases {
		if !case_.Test(ctx, v) {
			return false
		}
	}
	return true
}

func (patt *IntersectionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type ObjectPattern struct {
	NotCallablePatternMixin
	entryPatterns           map[string]Pattern
	optionalEntries         map[string]struct{}
	inexact                 bool //if true the matched object can have additional properties
	complexPropertyPatterns []*ComplexPropertyConstraint
}

func NewExactObjectPattern(entries map[string]Pattern) *ObjectPattern {
	return &ObjectPattern{entryPatterns: entries}
}

func NewInexactObjectPattern(entries map[string]Pattern) *ObjectPattern {
	return &ObjectPattern{entryPatterns: entries, inexact: true}
}

func NewInexactObjectPatternWithOptionalProps(entries map[string]Pattern, optionalProperties map[string]struct{}) *ObjectPattern {
	return &ObjectPattern{entryPatterns: entries, optionalEntries: optionalProperties, inexact: true}
}

func (patt *ObjectPattern) Test(ctx *Context, v Value) bool {
	obj, ok := v.(*Object)
	if !ok {
		return false
	}
	if !patt.inexact && len(patt.optionalEntries) == 0 && len(obj.keys) != len(patt.entryPatterns) {
		return false
	}

	for key, valuePattern := range patt.entryPatterns {
		if !obj.HasProp(ctx, key) {
			if _, ok := patt.optionalEntries[key]; ok {
				continue
			}
			return false
		}
		value := obj.Prop(ctx, key)
		if !valuePattern.Test(ctx, value) {
			return false
		}
	}

	// if pattern is exact check that there are no additional properties
	if !patt.inexact {
		for _, propName := range obj.PropertyNames(ctx) {
			if _, ok := patt.entryPatterns[propName]; !ok {
				return false
			}
		}
	}

	state := NewTreeWalkState(NewContext(ContextConfig{}))
	state.self = obj

	for _, constraint := range patt.complexPropertyPatterns {
		res, err := TreeWalkEval(constraint.Expr, state)
		if err != nil {
			if ctx != nil {
				ctx.Logger().Print("error when checking a complex property pattern: " + err.Error())
			}
			//TODO: log error some where
			return false
		}
		if b, ok := res.(Bool); !ok {
			ctx.Logger().Print("error when checking a multiproperty pattern")
		} else if !b {
			return false
		}
	}
	return true
}

func (patt *ObjectPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt *ObjectPattern) ForEachEntry(fn func(propName string, propPattern Pattern, isOptional bool) error) error {
	for propName, propPattern := range patt.entryPatterns {
		_, isOptional := patt.optionalEntries[propName]
		if err := fn(propName, propPattern, isOptional); err != nil {
			return err
		}
	}
	return nil
}

func (patt *ObjectPattern) EntryCount() int {
	return len(patt.entryPatterns)
}

type RecordPattern struct {
	NotCallablePatternMixin
	entryPatterns   map[string]Pattern
	optionalEntries map[string]struct{}
	inexact         bool //if true the matched object can have additional properties
}

func NewInexactRecordPattern(entries map[string]Pattern) *RecordPattern {
	return &RecordPattern{
		entryPatterns: utils.CopyMap(entries),
		inexact:       true,
	}
}

func (patt *RecordPattern) Test(ctx *Context, v Value) bool {
	rec, ok := v.(*Record)
	if !ok {
		return false
	}
	if !patt.inexact && len(patt.optionalEntries) == 0 && len(rec.keys) != len(patt.entryPatterns) {
		return false
	}

	for key, valuePattern := range patt.entryPatterns {
		if !rec.HasProp(ctx, key) {
			if _, ok := patt.optionalEntries[key]; ok {
				continue
			}
			return false
		}
		value := rec.Prop(ctx, key)
		if !valuePattern.Test(ctx, value) {
			return false
		}
	}

	// if pattern is exact check that there are no additional properties
	if !patt.inexact {
		for _, propName := range rec.PropertyNames(ctx) {
			if _, ok := patt.entryPatterns[propName]; !ok {
				return false
			}
		}
	}
	return true
}

func (patt *RecordPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type ComplexPropertyConstraint struct {
	NotCallablePatternMixin
	Properties []string
	Expr       parse.Node
}

type ListPattern struct {
	NotCallablePatternMixin
	elementPatterns       []Pattern
	generalElementPattern Pattern
}

func NewListPatternOf(generalElementPattern Pattern) *ListPattern {
	return &ListPattern{generalElementPattern: generalElementPattern}
}

func NewListPattern(elementPatterns []Pattern) *ListPattern {
	if elementPatterns == nil {
		elementPatterns = []Pattern{}
	}
	return &ListPattern{elementPatterns: elementPatterns}
}

func NewListPatternVariadic(elementPatterns ...Pattern) *ListPattern {
	if elementPatterns == nil {
		elementPatterns = []Pattern{}
	}
	return &ListPattern{elementPatterns: elementPatterns}
}

func (patt ListPattern) Test(ctx *Context, v Value) bool {
	list, ok := v.(*List)
	if !ok {
		return false
	}
	if patt.generalElementPattern != nil {
		length := list.Len()
		for i := 0; i < length; i++ {
			e := list.At(ctx, i)
			if !patt.generalElementPattern.Test(ctx, e) {
				return false
			}
		}
		return true
	}
	if list.Len() != len(patt.elementPatterns) {
		return false
	}
	for i, elementPattern := range patt.elementPatterns {
		if !ok || !elementPattern.Test(ctx, list.At(ctx, i)) {
			return false
		}
	}
	return true
}

func (patt *ListPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type TuplePattern struct {
	NotCallablePatternMixin
	elementPatterns       []Pattern
	generalElementPattern Pattern
}

func NewTuplePatternOf(generalElementPattern Pattern) *TuplePattern {
	return &TuplePattern{generalElementPattern: generalElementPattern}
}

func NewTuplePattern(elementPatterns []Pattern) *TuplePattern {
	if elementPatterns == nil {
		elementPatterns = []Pattern{}
	}
	return &TuplePattern{elementPatterns: elementPatterns}
}

func (patt *TuplePattern) Test(ctx *Context, v Value) bool {
	tuple, ok := v.(*Tuple)
	if !ok {
		return false
	}
	if patt.generalElementPattern != nil {
		length := tuple.Len()
		for i := 0; i < length; i++ {
			e := tuple.At(ctx, i)
			if !patt.generalElementPattern.Test(ctx, e) {
				return false
			}
		}
		return true
	}
	if tuple.Len() != len(patt.elementPatterns) {
		return false
	}
	for i, elementPattern := range patt.elementPatterns {
		if !ok || !elementPattern.Test(ctx, tuple.At(ctx, i)) {
			return false
		}
	}
	return true
}

func (patt *TuplePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type OptionPattern struct {
	NotCallablePatternMixin
	Name  string
	Value Pattern
}

func (patt OptionPattern) Test(ctx *Context, v Value) bool {
	opt, ok := v.(Option)
	return ok && opt.Name == patt.Name && patt.Value.Test(ctx, opt.Value)
}

func (patt *OptionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type DifferencePattern struct {
	NotCallablePatternMixin
	base    Pattern
	removed Pattern
}

func (patt *DifferencePattern) Test(ctx *Context, v Value) bool {
	return patt.base.Test(ctx, v) && !patt.removed.Test(ctx, v)
}

func (patt *DifferencePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type OptionalPattern struct {
	NotCallablePatternMixin
	Pattern Pattern
}

func NewOptionalPattern(ctx *Context, pattern Pattern) (*OptionalPattern, error) {
	if pattern.Test(ctx, Nil) {
		return nil, errors.New("cannot create optional pattern with pattern that already matches nil")
	}
	return &OptionalPattern{
		Pattern: pattern,
	}, nil
}

func (patt *OptionalPattern) Test(ctx *Context, v Value) bool {
	if _, ok := v.(NilT); ok {
		return true
	}
	return patt.Pattern.Test(ctx, v)
}

func (patt *OptionalPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type FunctionPattern struct {
	node          *parse.FunctionPatternExpression //if nil, matches any function
	symbolicValue *symbolic.FunctionPattern        //used for checking functions

	NotCallablePatternMixin
}

func (patt *FunctionPattern) Test(ctx *Context, v Value) bool {
	switch fn := v.(type) {
	case *GoFunction:
		if patt.node == nil {
			return true
		}

		if fn.fn == nil {
			return false
		}

		panic(errors.New("testing a go function against a function pattern is not supported yet"))

	case *InoxFunction:

		//TO KEEP IN SYNC WITH CONCRETE FUNCTION PATTERN
		if patt.node == nil {
			return true
		}

		fnExpr := fn.FuncExpr()
		if fnExpr == nil {
			return false
		}

		if len(fnExpr.Parameters) != len(patt.node.Parameters) || fnExpr.NonVariadicParamCount() != patt.node.NonVariadicParamCount() {
			return false
		}

		for i, param := range patt.node.Parameters {
			actualParam := fnExpr.Parameters[i]

			if (param.Type == nil) != (actualParam.Type == nil) {
				return false
			}

			if param.Type != nil && parse.SPrint(param.Type, parse.PrintConfig{TrimStart: true}) != parse.SPrint(actualParam.Type, parse.PrintConfig{TrimStart: true}) {
				return false
			}
		}

		symbolicFn := fn.symbolicValue
		if symbolicFn == nil {
			panic(errors.New("cannot Test() function against function pattern, Inox function has nil .SymbolicValue"))
		}

		return patt.symbolicValue.TestValue(symbolicFn)
	default:
		return false
	}
}

func (patt *FunctionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type IntRangePattern struct {
	intRange IntRange
	CallBasedPatternReprMixin
	NotCallablePatternMixin
}

func NewIncludedEndIntRangePattern(start, end int64) *IntRangePattern {
	if end < start {
		panic(fmt.Errorf("failed to create int range pattern, end < start"))
	}
	range_ := NewIncludedEndIntRange(start, end)
	return &IntRangePattern{
		intRange: range_,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: INT_PATTERN,
			Params: []Serializable{range_},
		},
	}
}

func NewSingleElementIntRangePattern(n int64) *IntRangePattern {
	range_ := IntRange{inclusiveEnd: true, Start: n, End: n, Step: 1}
	return &IntRangePattern{
		intRange: range_,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: INT_PATTERN,
			Params: []Serializable{range_},
		},
	}
}

func (patt *IntRangePattern) Test(ctx *Context, v Value) bool {
	n, ok := v.(Int)
	if !ok {
		return false
	}

	return n >= Int(patt.intRange.Start) && n <= Int(patt.intRange.InclusiveEnd())
}

func (patt *IntRangePattern) StringPattern() (StringPattern, bool) {
	return NewIntRangeStringPattern(patt.intRange.Start, patt.intRange.InclusiveEnd(), nil), true
}

type EventPattern struct {
	ValuePattern Pattern
	CallBasedPatternReprMixin

	NotClonableMixin
	NotCallablePatternMixin
}

func NewEventPattern(valuePattern Pattern) *EventPattern {
	return &EventPattern{
		ValuePattern: valuePattern,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: getDefaultNamedPattern("event"),
			Params: []Serializable{valuePattern},
		},
	}
}

func (patt *EventPattern) Test(ctx *Context, v Value) bool {
	e, ok := v.(*Event)
	if !ok {
		return false
	}

	if patt.ValuePattern == nil {
		return true
	}
	return patt.ValuePattern.Test(ctx, e.value)
}

func (patt *EventPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type MutationPattern struct {
	kind  MutationKind
	data0 Pattern
	CallBasedPatternReprMixin

	NotClonableMixin
	NotCallablePatternMixin
}

func NewMutationPattern(kind MutationKind, data0Pattern Pattern) *MutationPattern {
	return &MutationPattern{
		kind:  kind,
		data0: data0Pattern,
		CallBasedPatternReprMixin: CallBasedPatternReprMixin{
			Callee: getDefaultNamedPattern("mutation"),
			Params: []Serializable{Identifier(kind.String()), data0Pattern},
		},
	}
}

func (patt *MutationPattern) Test(ctx *Context, v Value) bool {
	_, ok := v.(Mutation)
	if !ok {
		return false
	}

	panic(ErrNotImplementedYet)
	//return patt.kind == m.Kind && patt.data0.Test(ctx, m.Data0)
}

func (patt *MutationPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}
