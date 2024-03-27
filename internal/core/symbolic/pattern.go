package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"regexp/syntax"
	"slices"
	"sort"
	"strconv"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/regexutils"
	"golang.org/x/exp/maps"
)

const (
	REGEX_SYNTAX                       = syntax.Perl
	MAX_UNION_PATTERN_FLATTENING_DEPTH = 5
)

var (
	_ = []Pattern{
		(*PathPattern)(nil), (*URLPattern)(nil), (*UnionPattern)(nil), (*AnyStringPattern)(nil), (*SequenceStringPattern)(nil),
		(*HostPattern)(nil), (*ListPattern)(nil), (*ObjectPattern)(nil), (*TuplePattern)(nil), (*RecordPattern)(nil),
		(*OptionPattern)(nil), (*RegexPattern)(nil), (*TypePattern)(nil), (*AnyPattern)(nil), (*FunctionPattern)(nil),
		(*ExactValuePattern)(nil), (*ExactStringPattern)(nil), (*ParserBasedPattern)(nil),
		(*IntRangePattern)(nil), (*FloatRangePattern)(nil), (*EventPattern)(nil), (*MutationPattern)(nil), (*OptionalPattern)(nil),
		(*FunctionPattern)(nil),
		(*DifferencePattern)(nil),
		(*IntersectionPattern)(nil),
	}
	_ = []GroupPattern{
		(*NamedSegmentPathPattern)(nil),
	}

	_ = []IPropsPattern{
		(*ObjectPattern)(nil),
	}

	ANY_TYPE_PATTERN         = &TypePattern{}
	ANY_EXACT_VALUE_PATTERN  = &ExactValuePattern{value: ANY_SERIALIZABLE}
	ANY_PATTERN              = &AnyPattern{}
	ANY_SERIALIZABLE_PATTERN = &AnySerializablePattern{}
	ANY_PATH_PATTERN         = &PathPattern{
		dirConstraint: UnspecifiedDirOrFilePath,
		absoluteness:  UnspecifiedPathAbsoluteness,
	}
	ANY_NAMED_SEGMENT_PATH_PATTERN = &NamedSegmentPathPattern{}
	ANY_URL_PATTERN                = &URLPattern{}
	ANY_HOST_PATTERN               = &HostPattern{}
	ANY_STR_PATTERN                = &AnyStringPattern{}
	ANY_LIST_PATTERN               = &ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN}
	ANY_TUPLE_PATTERN              = &TuplePattern{generalElement: ANY_SERIALIZABLE_PATTERN}

	ANY_OBJECT_PATTERN = &ObjectPattern{}
	ANY_RECORD_PATTERN = &RecordPattern{}
	ANY_OPTION_PATTERN = &OptionPattern{name: "", pattern: ANY_PATTERN}

	WIDEST_LIST_PATTERN  = NewListOf(ANY_SERIALIZABLE)
	WIDEST_TUPLE_PATTERN = NewTupleOf(ANY_SERIALIZABLE)

	ANY_DIR_PATH_PATTERN = &PathPattern{
		dirConstraint: DirPath,
	}
	ANY_NON_DIR_PATH_PATTERN = &PathPattern{
		dirConstraint: NonDirPath,
	}
	ANY_ABS_PATH_PATTERN = &PathPattern{
		absoluteness: AbsolutePath,
	}
	ANY_REL_PATH_PATTERN = &PathPattern{
		absoluteness: RelativePath,
	}
	ANY_ABS_DIR_PATH_PATTERN = &PathPattern{
		absoluteness:  AbsolutePath,
		dirConstraint: DirPath,
	}
	ANY_ABS_NON_DIR_PATH_PATTERN = &PathPattern{
		absoluteness:  AbsolutePath,
		dirConstraint: NonDirPath,
	}
	ANY_REL_DIR_PATH_PATTERN = &PathPattern{
		absoluteness:  RelativePath,
		dirConstraint: DirPath,
	}
	ANY_REL_NON_DIR_PATH_PATTERN = &PathPattern{
		absoluteness:  RelativePath,
		dirConstraint: NonDirPath,
	}

	ANY_HTTP_HOST_PATTERN  = &HostPattern{scheme: HTTP_SCHEME}
	ANY_HTTPS_HOST_PATTERN = &HostPattern{scheme: HTTPS_SCHEME}
	ANY_WS_HOST_PATTERN    = &HostPattern{scheme: WS_SCHEME}
	ANY_WSS_HOST_PATTERN   = &HostPattern{scheme: WSS_SCHEME}

	ANY_REGEX_PATTERN       = &RegexPattern{}
	ANY_INT_RANGE_PATTERN   = NewIntRangePattern(ANY_INT_RANGE)
	ANY_FLOAT_RANGE_PATTERN = NewFloatRangePattern(ANY_FLOAT_RANGE)
	ANY_EVENT_PATTERN       = &EventPattern{ValuePattern: ANY_PATTERN}
	ANY_MUTATION_PATTERN    = &MutationPattern{}

	ANY_FUNCTION_PATTERN = &FunctionPattern{}

	ANY_PATTERN_NAMESPACE = &PatternNamespace{}

	ErrPatternNotCallable                        = errors.New("pattern is not callable")
	ErrValueAlreadyInitialized                   = errors.New("value already initialized")
	ErrValueInExactPatternValueShouldBeImmutable = errors.New("the value in an exact value pattern should be immutable")

	HOST_PATTERN_PROPNAMES = []string{"scheme"}
)

// A Pattern represents a symbolic Pattern.
type Pattern interface {
	Serializable
	Iterable

	HasUnderlyingPattern() bool

	//equivalent of Test() for concrete patterns
	TestValue(v Value, state RecTestCallState) bool

	Call(ctx *Context, values []Value) (Pattern, error)

	//returns a symbolic value that represent all concrete values that match against this pattern
	SymbolicValue() Value

	StringPattern() (StringPattern, bool)
}

type NotCallablePatternMixin struct {
}

func (NotCallablePatternMixin) Call(ctx *Context, values []Value) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

// A GroupPattern represents a symbolic GroupPattern.
type GroupPattern interface {
	Pattern
	MatchGroups(Value) (yes bool, possible bool, groups map[string]Serializable)
}

func isAnyPattern(val Value) bool {
	_, ok := val.(*AnyPattern)
	return ok
}

type IPropsPattern interface {
	Value
	//ValuePropPattern should return the pattern of the property (name).
	ValuePropPattern(name string) (propPattern Pattern, isOptional bool, ok bool)

	//ValuePropertyNames should return the list of all property names (optional or not) of values matching the pattern.
	ValuePropertyNames() []string
}

// An AnyPattern represents a symbolic Pattern we do not know the concrete type.
type AnyPattern struct {
	NotCallablePatternMixin
	SerializableMixin
}

func (p *AnyPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Pattern)
	return ok
}

func (p *AnyPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("pattern")
}

func (p *AnyPattern) HasUnderlyingPattern() bool {
	return false
}

func (p *AnyPattern) TestValue(Value, RecTestCallState) bool {
	return true
}

func (p *AnyPattern) SymbolicValue() Value {
	return ANY
}

func (p *AnyPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnyPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *AnyPattern) IteratorElementValue() Value {
	return ANY
}

func (p *AnyPattern) WidestOfType() Value {
	return ANY_PATTERN
}

// An AnySerializablePattern represents a symbolic Pattern we do not know the concrete type that represents patterns
// of serializable values.
type AnySerializablePattern struct {
	NotCallablePatternMixin
	SerializableMixin
}

func (p *AnySerializablePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	patt, ok := v.(Pattern)
	if ok {
		return false
	}

	_, ok = AsSerializable(patt.SymbolicValue()).(Serializable)
	return ok
}

func (p *AnySerializablePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("pattern")
}

func (p *AnySerializablePattern) HasUnderlyingPattern() bool {
	return false
}

func (p *AnySerializablePattern) TestValue(Value, RecTestCallState) bool {
	return true
}

func (p *AnySerializablePattern) SymbolicValue() Value {
	return ANY_SERIALIZABLE
}

func (p *AnySerializablePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnySerializablePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *AnySerializablePattern) IteratorElementValue() Value {
	return ANY_SERIALIZABLE
}

func (p *AnySerializablePattern) WidestOfType() Value {
	return ANY_SERIALIZABLE_PATTERN
}

// A PathPattern represents a symbolic PathPattern.
type PathPattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin

	//at most one of the following field group should be set
	absoluteness  PathAbsoluteness
	dirConstraint DirPathConstraint

	hasValue bool
	value    string

	node            parse.Node
	stringifiedNode string
}

func NewPathPattern(v string) *PathPattern {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}

	return &PathPattern{
		hasValue: true,
		value:    v,
	}
}

func NewPathPatternFromNode(n parse.Node, chunk *parse.Chunk) *PathPattern {
	printConfig := parse.PrintConfig{}

	return &PathPattern{
		node:            n,
		stringifiedNode: parse.SPrint(n, chunk, printConfig),
	}
}

func (p *PathPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*PathPattern)

	if !ok {
		return false
	}

	if p.node != nil {
		return otherPattern.node == p.node
	}

	if p.hasValue {
		return otherPattern.hasValue && otherPattern.value == p.value
	}

	if p.absoluteness != UnspecifiedPathAbsoluteness && otherPattern.absoluteness != p.absoluteness {
		return false
	}

	if p.dirConstraint != UnspecifiedDirOrFilePath && otherPattern.dirConstraint != p.dirConstraint {
		return false
	}

	return true
}

func (p *PathPattern) IsConcretizable() bool {
	return p.hasValue
}

func (p *PathPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return extData.ConcreteValueFactories.CreatePathPattern(p.value)
}

func (p *PathPattern) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *PathPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	path, ok := v.(*Path)
	if !ok {
		return false
	}

	if path.pattern == p {
		return true
	}

	if p.node != nil {
		//false is returned because it's difficult to know if the path matches
		return false
	}

	if path.hasValue {
		if p.hasValue {
			return extData.PathMatch(path.value, p.value)
		}

		if p.absoluteness != UnspecifiedPathAbsoluteness && (p.absoluteness == AbsolutePath) != (path.value[0] == '/') {
			return false
		}

		if p.dirConstraint != UnspecifiedDirOrFilePath && (p.dirConstraint == DirPath) != (path.value[len(path.value)-1] == '/') {
			return false
		}

		return true
	}

	if p.hasValue {
		return false
	}

	return p.absoluteness == UnspecifiedPathAbsoluteness && p.dirConstraint == UnspecifiedDirOrFilePath
}

func (p *PathPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {

	if p.hasValue {
		w.WriteString("%" + p.value)
		return
	}

	s := "%path-pattern"

	if p.node != nil {
		w.WriteString(s)
		w.WriteString("(")
		w.WriteString(p.stringifiedNode)
		w.WriteString(")")
		return
	}

	switch p.absoluteness {
	case AbsolutePath:
		s += "(#abs"
	case RelativePath:
		s += "(#rel"
	}

	if p.absoluteness != UnspecifiedPathAbsoluteness && p.dirConstraint != UnspecifiedDirOrFilePath {
		s += ","
	} else if p.dirConstraint != UnspecifiedDirOrFilePath {
		s += "("
	}

	switch p.dirConstraint {
	case DirPath:
		s += "#dir"
	case NonDirPath:
		s += "#non-dir"
	}

	if p.absoluteness != UnspecifiedPathAbsoluteness || p.dirConstraint != UnspecifiedDirOrFilePath {
		s += ")"
	}

	w.WriteString(s)
}

func (p *PathPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *PathPattern) SymbolicValue() Value {
	return NewPathMatchingPattern(p)
}

func (p *PathPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *PathPattern) PropertyNames() []string {
	return nil
}

func (*PathPattern) Prop(name string) Value {
	switch name {
	default:
		return nil
	}
}

func (p *PathPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *PathPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *PathPattern) underlyingString() *String {
	return ANY_STRING
}

func (p *PathPattern) WidestOfType() Value {
	return ANY_PATH_PATTERN
}

// A URLPattern represents a symbolic URLPattern.
type URLPattern struct {
	hasValue bool
	value    string

	node            parse.Node
	stringifiedNode string

	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin
}

func NewUrlPattern(v string) *URLPattern {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}
	return &URLPattern{
		hasValue: true,
		value:    v,
	}
}

func NewUrlPatternFromNode(n parse.Node, chunk *parse.Chunk) *URLPattern {
	printConfig := parse.PrintConfig{KeepLeadingSpace: true, KeepTrailingSpace: true}
	return &URLPattern{
		node:            n,
		stringifiedNode: parse.SPrint(n, chunk, printConfig),
	}
}

func (p *URLPattern) WithAdditionalPathSegment(segment string) *URLPattern {
	if p.hasValue {
		return NewUrlPattern(extData.AppendPathSegmentToURLPattern(p.value, segment))
	}

	return ANY_URL_PATTERN
}

func (p *URLPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*URLPattern)

	if !ok {
		return false
	}

	if p.node != nil {
		return otherPattern.node == p.node
	}

	if p.hasValue {
		return otherPattern.hasValue && otherPattern.value == p.value
	}

	return true
}

func (p *URLPattern) IsConcretizable() bool {
	return p.hasValue
}

func (p *URLPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return extData.ConcreteValueFactories.CreateURLPattern(p.value)
}

func (p *URLPattern) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *URLPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.hasValue {
		w.WriteString("%" + p.value)
		return
	}

	s := "%url-pattern"

	if p.node != nil {
		w.WriteString(s)
		w.WriteString("(")
		w.WriteString(p.stringifiedNode)
		w.WriteString(")")
		return
	}
}

func (p *URLPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *URLPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	u, ok := v.(*URL)
	if !ok {
		return false
	}

	if u.pattern != nil && (u.pattern == p || p.Test(u.pattern, state)) {
		return true
	}

	if p.node != nil {
		//false is returned because it's difficult to know if the url matches
		return false
	}

	if u.hasValue {
		if p.hasValue {
			return extData.URLMatch(u.value, p.value)
		}
	}

	return !p.hasValue
}

func (p *URLPattern) SymbolicValue() Value {
	return NewUrlMatchingPattern(p)
}

func (p *URLPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *URLPattern) PropertyNames() []string {
	return nil
}

func (*URLPattern) Prop(name string) Value {
	switch name {
	default:
		return nil
	}
}

func (p *URLPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *URLPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *URLPattern) underlyingString() *String {
	return ANY_STRING
}

func (p *URLPattern) WidestOfType() Value {
	return ANY_URL_PATTERN
}

// A HostPattern represents a symbolic HostPattern.
type HostPattern struct {
	scheme *Scheme //optional, not set if .hasValue is true

	hasValue bool
	value    string // ://**.com, https://**.com, ....

	node            parse.Node
	stringifiedNode string

	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin
}

func NewHostPattern(v string) *HostPattern {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}
	return &HostPattern{
		hasValue: true,
		value:    v,
	}
}

func NewHostPatternFromNode(n parse.Node, chunk *parse.Chunk) *HostPattern {
	printConfig := parse.PrintConfig{}
	return &HostPattern{
		node:            n,
		stringifiedNode: parse.SPrint(n, chunk, printConfig),
	}
}

func (p *HostPattern) Scheme() (*Scheme, bool) {
	if p.hasValue {
		if p.value[0] == ':' { //scheme-less host
			return nil, false
		}
		u := utils.Must(url.Parse(p.value))
		return GetOrNewScheme(u.Scheme), true
	}

	return p.scheme, p.scheme != nil
}

func (p *HostPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*HostPattern)

	if !ok {
		return false
	}

	if p.node != nil {
		return otherPattern.node == p.node
	}

	if p.hasValue {
		return otherPattern.hasValue && otherPattern.value == p.value
	}

	if p.scheme == nil || p.scheme.Test(ANY_SCHEME, state) {
		return true
	} //else we know that p has a concrete scheme.

	otherPatternScheme, ok := otherPattern.Scheme()
	return ok && p.scheme.Test(otherPatternScheme, state)
}

func (p *HostPattern) IsConcretizable() bool {
	return p.hasValue
}

func (p *HostPattern) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *HostPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.hasValue {
		w.WriteString("%" + p.value)
		return
	}

	s := "%host-pattern"
	w.WriteString(s)

	if p.node != nil {
		w.WriteString("(")
		w.WriteString(p.stringifiedNode)
		w.WriteString(")")
	} else if p.scheme != nil {
		w.WriteString("(")
		if p.scheme.hasValue {
			w.WriteString(p.scheme.value)
			w.WriteString(")")
		} else {
			w.WriteString("?)")
		}
	}
}

func (p *HostPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *HostPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	h, ok := v.(*Host)
	if !ok {
		return false
	}

	if h.pattern == p {
		return true
	}

	if p.node != nil {
		//false is returned because it's difficult to know if the host matches
		return false
	}

	if h.hasValue {
		if p.hasValue {
			return extData.HostMatch(h.value, p.value)
		}
	}

	patternScheme, ok1 := p.Scheme()
	if ok1 {
		//check scheme
		hostScheme, ok2 := h.Scheme()

		if ok1 != ok2 || (ok1 && !patternScheme.Test(hostScheme, state)) {
			return false
		}
	}

	return !p.hasValue
}

func (p *HostPattern) SymbolicValue() Value {
	return NewHostMatchingPattern(p)
}

func (p *HostPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *HostPattern) PropertyNames() []string {
	return HOST_PATTERN_PROPNAMES
}

func (p *HostPattern) Prop(name string) Value {
	switch name {
	case "scheme":
		scheme, ok := p.Scheme()
		if ok {
			return scheme
		}
		return NewMultivalue(ANY_SCHEME, Nil)
	default:
		return nil
	}
}

func (p *HostPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *HostPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *HostPattern) underlyingString() *String {
	return ANY_STRING
}

func (p *HostPattern) WidestOfType() Value {
	return ANY_HOST_PATTERN
}

// A NamedSegmentPathPattern represents a symbolic NamedSegmentPathPattern.
type NamedSegmentPathPattern struct {
	node *parse.NamedSegmentPathPatternLiteral //if nil, any node is matched

	UnassignablePropsMixin
	NotCallablePatternMixin
	SerializableMixin
}

func NewNamedSegmentPathPattern(node *parse.NamedSegmentPathPatternLiteral) *NamedSegmentPathPattern {
	return &NamedSegmentPathPattern{node: node}
}

func (p *NamedSegmentPathPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*NamedSegmentPathPattern)
	if !ok {
		return false
	}

	return p.node == nil || p.node == otherPattern.node
}

func (p *NamedSegmentPathPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.node == nil {
		w.WriteName("named-segment-path-pattern")
		return
	}
	w.WriteNameF("named-segment-path-pattern(%p)", p.node)
}

func (p NamedSegmentPathPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *NamedSegmentPathPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Path)
	return ok
}

func (p *NamedSegmentPathPattern) SymbolicValue() Value {
	return &Path{}
}

func (p *NamedSegmentPathPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *NamedSegmentPathPattern) MatchGroups(v Value) (yes, possible bool, groups map[string]Serializable) {

	_, ok := v.(*Path)
	if !ok {
		return
	}
	possible = true
	//TODO:

	groups = map[string]Serializable{}
	if p.node != nil {
		for _, s := range p.node.Slices {
			segment, ok := s.(*parse.NamedPathSegment)
			if ok {
				groups[segment.Name] = ANY_STRING
			}
		}
	}

	return
}

func (p *NamedSegmentPathPattern) PropertyNames() []string {
	return nil
}

func (*NamedSegmentPathPattern) Prop(name string) Value {
	switch name {
	default:
		return nil
	}
}

func (p *NamedSegmentPathPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *NamedSegmentPathPattern) IteratorElementValue() Value {
	return &Path{}
}

func (p *NamedSegmentPathPattern) WidestOfType() Value {
	return ANY_NAMED_SEGMENT_PATH_PATTERN
}

// An ExactValuePattern represents a symbolic ExactValuePattern.
type ExactValuePattern struct {
	value Serializable //immutable in most cases

	NotCallablePatternMixin
	SerializableMixin
}

func NewExactValuePattern(v Serializable) (*ExactValuePattern, error) {
	if rv, ok := v.(IRunTimeValue); ok {
		super := rv.OriginalRunTimeValue().super
		if !IsAnySerializable(super) && v.IsMutable() {
			return nil, ErrValueInExactPatternValueShouldBeImmutable
		}
	} else if !IsAnySerializable(v) && v.IsMutable() {
		return nil, ErrValueInExactPatternValueShouldBeImmutable
	}
	return &ExactValuePattern{value: v}, nil
}

func NewUncheckedExactValuePattern(v Serializable) (*ExactValuePattern, error) {
	return &ExactValuePattern{value: v}, nil
}

func NewMostAdaptedExactPattern(value Serializable) (Pattern, error) {
	if !IsAnySerializable(value) && value.IsMutable() {
		return nil, ErrValueInExactPatternValueShouldBeImmutable
	}

	if s, ok := AsStringLike(value).(StringLike); ok {
		str := s.GetOrBuildString()

		if !str.IsConcretizable() {
			rv := NewRunTimeValue(s).as(STRLIKE_INTERFACE_TYPE).(*strLikeRunTimeValue)
			return NewExactStringPatternWithRunTimeValue(rv), nil
		}

		return NewExactStringPatternWithConcreteValue(str), nil
	}

	if !IsConcretizable(value) {
		rv := NewRunTimeValue(value).as(SERIALIZABLE_INTERFACE_TYPE).(*serializableRunTimeValue)
		return NewExactValuePattern(rv)
	}

	return NewExactValuePattern(value)
}

func (p *ExactValuePattern) SetVal(v Serializable) {
	if p.value != nil {
		panic(errors.New("value already set"))
	}
	p.value = v
}

// result should not be modified
func (p *ExactValuePattern) GetVal() Value {
	return p.value
}

func (p *ExactValuePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*ExactValuePattern)
	if !ok {
		return false
	}

	return p.value.Test(other.value, state)
}

func (p *ExactValuePattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return extData.ConcreteValueFactories.CreateExactValuePattern(utils.Must(Concretize(p.value, ctx)))
}

func (p *ExactValuePattern) IsConcretizable() bool {
	return IsConcretizable(p.value)
}

func (p *ExactValuePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("exact-value-pattern(\n")
	innerIndentCount := w.ParentIndentCount + 2
	innerIndent := bytes.Repeat(config.Indent, innerIndentCount)
	parentIndent := innerIndent[:len(innerIndent)-2*len(config.Indent)]

	w.WriteBytes(innerIndent)
	p.value.PrettyPrint(w.WithDepthIndent(w.Depth+2, innerIndentCount), config)

	w.WriteByte('\n')
	w.WriteBytes(parentIndent)
	w.WriteByte(')')

}

func (p *ExactValuePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ExactValuePattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return p.value.Test(v, state) && v.Test(p.value, state)
}

func (p *ExactValuePattern) SymbolicValue() Value {
	return p.value
}

func (p *ExactValuePattern) MigrationInitialValue() (Serializable, bool) {
	return p.value, true
}

func (p *ExactValuePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ExactValuePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *ExactValuePattern) IteratorElementValue() Value {
	return p.value
}

func (p *ExactValuePattern) WidestOfType() Value {
	return ANY_EXACT_VALUE_PATTERN
}

// A RegexPattern represents a symbolic RegexPattern.
type RegexPattern struct {
	regex  *regexp.Regexp //if nil any regex pattern is matched
	syntax *syntax.Regexp
	SerializableMixin
	NotCallablePatternMixin
}

func NewRegexPattern(s string) *RegexPattern {
	regexp := regexp.MustCompile(s) //compiles with syntax.Perl flag
	syntaxRegexp := utils.Must(syntax.Parse(s, REGEX_SYNTAX))
	syntaxRegexp = regexutils.TurnCapturingGroupsIntoNonCapturing(syntaxRegexp)

	return &RegexPattern{
		regex:  regexp,
		syntax: syntaxRegexp,
	}
}

func (p *RegexPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPatt, ok := v.(*RegexPattern)
	if !ok {
		return false
	}
	return p.regex == nil || (otherPatt.regex != nil && p.syntax.Equal(otherPatt.syntax))
}

func (p *RegexPattern) IsConcretizable() bool {
	return p.regex != nil
}

func (p *RegexPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateRegexPattern(p.regex.String())
}

func (p *RegexPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.regex != nil {
		w.WriteString("%`" + p.regex.String() + "`")
		return
	}
	w.WriteName("regex-pattern")
}

func (p *RegexPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *RegexPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	s, ok := v.(StringLike)
	if !ok {
		return false
	}
	if p.regex == nil {
		return true
	}

	str := s.GetOrBuildString()
	return str.hasValue && p.regex.MatchString(str.value)
}

func (p *RegexPattern) HasRegex() bool {
	return true
}

func (p *RegexPattern) SymbolicValue() Value {
	return NewStringMatchingPattern(p)
}

func (p *RegexPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *RegexPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *RegexPattern) IteratorElementValue() Value {
	return ANY_STRING
}

func (p *RegexPattern) WidestOfType() Value {
	return ANY_REGEX_PATTERN
}

// An ObjectPattern represents a symbolic ObjectPattern.
type ObjectPattern struct {
	entries                    map[string]Pattern //if nil any object is matched
	optionalEntries            map[string]struct{}
	dependencies               map[string]propertyDependencies
	inexact                    bool
	readonly                   bool //should not be true if some property patterns are not readonly patterns
	complexPropertyConstraints []*ComplexPropertyConstraint

	NotCallablePatternMixin
	SerializableMixin
}

// dependencies for a property
type propertyDependencies struct {
	requiredKeys []string
	pattern      Pattern //pattern that the object should match if the property is present
}

func NewAnyObjectPattern() *ObjectPattern {
	return &ObjectPattern{}
}

func NewUnitializedObjectPattern() *ObjectPattern {
	return &ObjectPattern{}
}

func NewObjectPattern(exact bool, entries map[string]Pattern, optionalEntries map[string]struct{}) *ObjectPattern {
	return &ObjectPattern{
		inexact:         !exact,
		entries:         entries,
		optionalEntries: optionalEntries,
	}
}

func NewExactObjectPattern(entries map[string]Pattern, optionalEntries map[string]struct{}) *ObjectPattern {
	return NewObjectPattern(true, entries, optionalEntries)
}

func NewInexactObjectPattern(entries map[string]Pattern, optionalEntries map[string]struct{}) *ObjectPattern {
	return NewObjectPattern(false, entries, optionalEntries)
}

func InitializeObjectPattern(patt *ObjectPattern, entries map[string]Pattern, optionalEntries map[string]struct{}, inexact bool) {
	if patt.entries != nil || patt.complexPropertyConstraints != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.entries = entries
	patt.optionalEntries = optionalEntries
	patt.inexact = inexact
}

func (p *ObjectPattern) ToRecordPattern() *RecordPattern {
	if p.entries == nil {
		return NewAnyRecordPattern()
	}
	patt := NewUnitializedRecordPattern()
	//TODO: check that SymbolicValue() of entry patterns are immutable
	InitializeRecordPattern(patt, p.entries, p.optionalEntries, p.inexact)
	return patt
}

func (p *ObjectPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*ObjectPattern)

	if !ok || len(p.complexPropertyConstraints) > 0 || p.readonly != other.readonly {
		return false
	}

	if p.entries == nil {
		return true
	}

	if (!p.inexact && other.inexact) || other.entries == nil || (!p.inexact && len(p.entries) != len(other.entries)) {
		return false
	}

	//check other has stricter version of the dependencies
	for propName, deps := range p.dependencies {
		counterPartDeps, ok := other.dependencies[propName]
		if !ok {
			return false
		}
		for _, dep := range deps.requiredKeys {
			if !slices.Contains(counterPartDeps.requiredKeys, dep) {
				return false
			}
		}
		if deps.pattern != nil && (counterPartDeps.pattern == nil || !deps.pattern.Test(counterPartDeps.pattern, state)) {
			return false
		}
	}

	for k, v := range p.entries {
		otherV, ok := other.entries[k]
		if !ok || !v.Test(otherV, state) {
			return false
		}
	}

	return true
}

func (p *ObjectPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	obj, ok := v.(*Object)
	if !ok || p.readonly != obj.readonly {
		return false
	}

	if p.entries == nil {
		return true
	}

	if !p.inexact && obj.IsInexact() {
		return false
	}

	if p.inexact {
		if obj.entries == nil {
			return false
		}
	} else if obj.entries == nil || (len(p.optionalEntries) == 0 && len(p.entries) != len(obj.entries)) {
		return false
	}

	//check dependencies
	for propName, deps := range p.dependencies {
		counterPartDeps, ok := obj.dependencies[propName]
		if ok {
			for _, dep := range deps.requiredKeys {
				if !slices.Contains(counterPartDeps.requiredKeys, dep) {
					return false
				}
			}
			if deps.pattern != nil && (counterPartDeps.pattern == nil || !deps.pattern.Test(counterPartDeps.pattern, state)) {
				return false
			}
		} else if !obj.hasRequiredProperty(propName) {
			//if the property does not exist or is optional in obj it's impossible
			//to known if the dependency constraint is fulfilled.
			return false
		}
	}

	for key, valuePattern := range p.entries {
		_, isOptional := p.optionalEntries[key]
		_, isOptionalInObject := obj.optionalEntries[key]
		value, _, ok := obj.GetProperty(key)

		if !isOptional && isOptionalInObject {
			return false
		}

		if !ok {
			if !isOptional || (p.hasDeps(key) && !obj.hasDeps(key)) {
				return false
			}
		} else {
			if !valuePattern.TestValue(value, state) {
				return false
			}
			if !isOptional || !isOptionalInObject {
				//check dependencies
				deps := p.dependencies[key]
				for _, requiredKey := range deps.requiredKeys {
					if !obj.hasRequiredProperty(requiredKey) {
						return false
					}
				}
				if deps.pattern != nil && !deps.pattern.TestValue(obj, state) {
					return false
				}
			}
		}
	}

	// if pattern is exact check that there are no additional properties
	if !p.inexact {
		for _, propName := range obj.PropertyNames() {
			if _, ok := p.entries[propName]; !ok {
				return false
			}
		}
	}

	return true
}

func (p *ObjectPattern) hasDeps(name string) bool {
	_, ok := p.dependencies[name]
	return ok
}

func (p *ObjectPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concretePropertyPatterns := make(map[string]any, len(p.entries))

	for k, v := range p.entries {
		concretePropPattern := utils.Must(Concretize(v, ctx))
		concretePropertyPatterns[k] = concretePropPattern
	}

	return extData.ConcreteValueFactories.CreateObjectPattern(p.inexact, concretePropertyPatterns, maps.Clone(p.optionalEntries))
}

func (patt *ObjectPattern) IsConcretizable() bool {
	if patt.entries == nil {
		return false
	}

	for _, v := range patt.entries {
		if potentiallyConcretizable, ok := v.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (o *ObjectPattern) IsReadonlyPattern() bool {
	return o.readonly
}

func (o *ObjectPattern) ToReadonlyPattern() (PotentiallyReadonlyPattern, error) {
	if o.entries == nil {
		return nil, ErrNotConvertibleToReadonly
	}

	if o.readonly {
		return o, nil
	}

	properties := make(map[string]Pattern, len(o.entries))

	for k, v := range o.entries {
		if !v.SymbolicValue().IsMutable() {
			properties[k] = v
			continue
		}
		potentiallyReadonlyPattern, ok := v.(PotentiallyReadonlyPattern)
		if !ok {
			return nil, FmtPropertyPatternError(k, ErrNotConvertibleToReadonly)
		}
		readonly, err := potentiallyReadonlyPattern.ToReadonlyPattern()
		if err != nil {
			return nil, FmtPropertyPatternError(k, err)
		}
		properties[k] = readonly.(Pattern)
	}

	obj := NewObjectPattern(!o.inexact, properties, o.optionalEntries)
	obj.readonly = true
	return obj, nil
}

func (patt *ObjectPattern) ForEachEntry(fn func(propName string, propPattern Pattern, isOptional bool) error) error {
	for propName, propPattern := range patt.entries {
		_, isOptional := patt.optionalEntries[propName]
		if err := fn(propName, propPattern, isOptional); err != nil {
			return err
		}
	}
	return nil
}

func (p *ObjectPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.readonly {
		w.WriteName("readonly ")
	}
	if p.entries != nil {
		if w.Depth > config.MaxDepth && len(p.entries) > 0 {
			w.WriteString("%{(...)}")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteBytes([]byte{'%', '{'})

		var keys []string
		for k := range p.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteAnsiReset()
			}

			if _, ok := p.optionalEntries[k]; ok {
				w.WriteByte('?')
			}

			//colon
			w.WriteColonSpace()

			//value
			v := p.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry /* /*p.inexact*/ {
				w.WriteCommaSpace()
			}
		}

		// if p.inexact {
		// 	if !config.Compact {
		// 		w.WriteLFCR()
		// 		w.WriteBytes(indent)
		// 	}

		// 	w.WriteBytes(THREE_DOTS)
		// }

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}

		w.WriteBytes(bytes.Repeat(config.Indent, w.Depth))
		w.WriteByte('}')
		return
	}

	w.WriteName("object-pattern")
}

func (p *ObjectPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ObjectPattern) SymbolicValue() Value {
	if p.entries == nil {
		if p.readonly {
			return ANY_READONLY_OBJ
		}
		return ANY_OBJ
	}
	entries := map[string]Serializable{}
	static := map[string]Pattern{}

	if p.entries != nil {
		for key, valuePattern := range p.entries {
			entries[key] = AsSerializableChecked(valuePattern.SymbolicValue())
			static[key] = valuePattern
		}
	}

	obj := NewObject(!p.inexact, entries, p.optionalEntries, static)
	obj.dependencies = p.dependencies
	if p.readonly {
		obj.readonly = true
	}
	return obj
}

func (p *ObjectPattern) MigrationInitialValue() (Serializable, bool) {
	if p.entries == nil {
		return ANY_OBJ, true
	}
	entries := map[string]Serializable{}
	static := map[string]Pattern{}

	for key, propPattern := range p.entries {
		capable, ok := propPattern.(MigrationInitialValueCapablePattern)
		if !ok {
			return nil, false
		}
		propInitialValue, ok := capable.MigrationInitialValue()
		if !ok {
			return nil, false
		}
		entries[key] = AsSerializableChecked(propInitialValue)
		static[key] = propPattern
	}

	if p.inexact {
		return NewInexactObject(entries, p.optionalEntries, static), true
	}
	return NewExactObject(entries, p.optionalEntries, static), true
}

func (p *ObjectPattern) ValuePropPattern(name string) (propPattern Pattern, isOptional bool, ok bool) {
	if p.entries == nil {
		return nil, false, false
	}
	propPattern, ok = p.entries[name]
	_, isOptional = p.optionalEntries[name]
	return
}

func (p *ObjectPattern) ValuePropertyNames() []string {
	return maps.Keys(p.entries)
}

func (p *ObjectPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ObjectPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *ObjectPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *ObjectPattern) WidestOfType() Value {
	return ANY_OBJECT_PATTERN
}

// An RecordPattern represents a symbolic RecordPattern.
type RecordPattern struct {
	entries                    map[string]Pattern //if nil any record is matched
	optionalEntries            map[string]struct{}
	inexact                    bool
	complexPropertyConstraints []*ComplexPropertyConstraint

	NotCallablePatternMixin
	SerializableMixin
}

func NewAnyRecordPattern() *RecordPattern {
	return &RecordPattern{}
}

func NewUnitializedRecordPattern() *RecordPattern {
	return &RecordPattern{}
}

func InitializeRecordPattern(patt *RecordPattern, entries map[string]Pattern, optionalEntries map[string]struct{}, inexact bool) {
	if patt.entries != nil || patt.complexPropertyConstraints != nil {
		panic(ErrValueAlreadyInitialized)
	}
	patt.entries = entries
	patt.optionalEntries = optionalEntries
	patt.inexact = inexact
}

func NewExactRecordPattern(entries map[string]Pattern, optionalEntries map[string]struct{}) *RecordPattern {
	return &RecordPattern{
		inexact:         false,
		entries:         entries,
		optionalEntries: optionalEntries,
	}
}

func NewInexactRecordPattern(entries map[string]Pattern, optionalEntries map[string]struct{}) *RecordPattern {
	return &RecordPattern{
		inexact:         true,
		entries:         entries,
		optionalEntries: optionalEntries,
	}
}

func (p *RecordPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*RecordPattern)

	if !ok || len(p.complexPropertyConstraints) > 0 {
		return false
	}

	if p.entries == nil {
		return true
	}

	if p.inexact != other.inexact || other.entries == nil || len(p.entries) != len(other.entries) {
		return false
	}

	for k, v := range p.entries {
		otherV, ok := other.entries[k]
		if !ok || !v.Test(otherV, state) {
			return false
		}
	}

	return true
}

func (p *RecordPattern) IsConcretizable() bool {
	if p.entries == nil {
		return false
	}

	for _, v := range p.entries {
		if potentiallyConcretizable, ok := v.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (p *RecordPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concretePropertyPatterns := make(map[string]any, len(p.entries))

	for k, v := range p.entries {
		concretePropPattern := utils.Must(Concretize(v, ctx))
		concretePropertyPatterns[k] = concretePropPattern
	}

	return extData.ConcreteValueFactories.CreateRecordPattern(p.inexact, concretePropertyPatterns, maps.Clone(p.optionalEntries))
}

func (p *RecordPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.entries != nil {
		if w.Depth > config.MaxDepth && len(p.entries) > 0 {
			w.WriteName("record(%{(...)})")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteName("record(%{")

		var keys []string
		for k := range p.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteAnsiReset()
			}

			if _, ok := p.optionalEntries[k]; ok {
				w.WriteByte('?')
			}

			//colon
			w.WriteColonSpace()

			//value
			v := p.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry || p.inexact {
				w.WriteCommaSpace()

			}
		}

		// if p.inexact {
		// 	if !config.Compact {
		// 		w.WriteLFCR()
		// 		w.WriteBytes(indent)
		// 	}

		// 	w.WriteBytes(THREE_DOTS)
		// }

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}

		w.WriteBytes(bytes.Repeat(config.Indent, w.Depth))
		w.WriteClosingBracketClosingParen()
		return
	}
	w.WriteName("record-pattern")
}

func (p *RecordPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *RecordPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	rec, ok := v.(*Record)
	if !ok {
		return false
	}

	if p.entries == nil {
		return true
	}

	if p.inexact {
		if rec.entries == nil {
			return false
		}
	} else if rec.entries == nil || (len(p.optionalEntries) == 0 && len(p.entries) != len(rec.entries)) {
		return false
	}

	for key, valuePattern := range p.entries {
		_, isOptional := p.optionalEntries[key]
		value, ok := rec.entries[key]

		if ok {
			if !valuePattern.TestValue(value, state) {
				return false
			}
		} else if !isOptional {
			return false
		}
	}

	// if pattern is exact check that there are no additional properties
	if !p.inexact {
		for _, propName := range rec.PropertyNames() {
			if _, ok := p.entries[propName]; !ok {
				return false
			}
		}
	}

	return true
}

func (p *RecordPattern) SymbolicValue() Value {
	if p.entries == nil {
		return ANY_REC
	}

	rec := &Record{
		exact:           !p.inexact,
		entries:         map[string]Serializable{},
		optionalEntries: p.optionalEntries,
	}

	if p.entries != nil {
		for key, valuePattern := range p.entries {
			rec.entries[key] = AsSerializableChecked(valuePattern.SymbolicValue())
		}
	}

	return rec
}

func (p *RecordPattern) MigrationInitialValue() (Serializable, bool) {
	if p.entries == nil {
		return ANY_REC, true
	}
	entries := map[string]Serializable{}
	static := map[string]Pattern{}

	for key, propPattern := range p.entries {
		capable, ok := propPattern.(MigrationInitialValueCapablePattern)
		if !ok {
			return nil, false
		}
		propInitialValue, ok := capable.MigrationInitialValue()
		if !ok {
			return nil, false
		}
		entries[key] = AsSerializableChecked(propInitialValue)
		static[key] = propPattern
	}

	if p.inexact {
		return NewInexactRecord(entries, p.optionalEntries), true
	}
	return NewExactRecord(entries, p.optionalEntries), true
}

func (p *RecordPattern) ValuePropPattern(name string) (propPattern Pattern, isOptional bool, ok bool) {
	if p.entries == nil {
		return nil, false, false
	}
	propPattern, ok = p.entries[name]
	_, isOptional = p.optionalEntries[name]
	return
}

func (p *RecordPattern) ValuePropertyNames() []string {
	return maps.Keys(p.entries)
}

func (p *RecordPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *RecordPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *RecordPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *RecordPattern) WidestOfType() Value {
	return ANY_RECORD_PATTERN
}

type ComplexPropertyConstraint struct {
	NotCallablePatternMixin
	Properties []string
	Expr       parse.Node
}

// A ListPattern represents a symbolic ListPattern.
// .elements and .generalElement can never be both nil (nor both not nil).
type ListPattern struct {
	elements       []Pattern
	generalElement Pattern
	readonly       bool

	NotCallablePatternMixin
	SerializableMixin
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

func (p *ListPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*ListPattern)

	if !ok || p.readonly != other.readonly {
		return false
	}

	if p.elements != nil {
		if other.elements == nil || len(p.elements) != len(other.elements) {
			return false
		}

		for i, e := range p.elements {
			if !e.Test(other.elements[i], RecTestCallState{}) {
				return false
			}
		}

		return true
	} else {
		if other.elements == nil {
			return p.generalElement.Test(other.generalElement, RecTestCallState{})
		}

		for _, elem := range other.elements {
			if !p.generalElement.Test(elem, RecTestCallState{}) {
				return false
			}
		}
		return true
	}
}

func (p *ListPattern) IsConcretizable() bool {
	if p.generalElement != nil {
		potentiallyConcretizable, ok := p.generalElement.(PotentiallyConcretizable)
		return ok && potentiallyConcretizable.IsConcretizable()
	}

	for _, v := range p.elements {
		if potentiallyConcretizable, ok := v.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (p *ListPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	if p.generalElement != nil {
		concreteGeneralElement := utils.Must(Concretize(p.generalElement, ctx))
		return extData.ConcreteValueFactories.CreateListPattern(concreteGeneralElement, nil)
	}

	concreteElementPatterns := make([]any, len(p.elements))

	for i, e := range p.elements {
		concreteElemPattern := utils.Must(Concretize(e, ctx))
		concreteElementPatterns[i] = concreteElemPattern
	}

	return extData.ConcreteValueFactories.CreateListPattern(nil, concreteElementPatterns)
}

func (p *ListPattern) IsReadonlyPattern() bool {
	return p.readonly
}

func (p *ListPattern) ToReadonlyPattern() (PotentiallyReadonlyPattern, error) {
	if p.readonly {
		return p, nil
	}

	if p.generalElement != nil {
		var readonlyGeneralElementPattern Pattern
		if !p.generalElement.SymbolicValue().IsMutable() {
			readonlyGeneralElementPattern = p.generalElement
		} else {
			potentiallyReadonly, ok := p.generalElement.(PotentiallyReadonlyPattern)
			if !ok {
				return nil, FmtGeneralElementError(ErrNotConvertibleToReadonly)
			}
			readonly, err := potentiallyReadonly.ToReadonlyPattern()
			if err != nil {
				return nil, FmtGeneralElementError(err)
			}
			readonlyGeneralElementPattern = readonly
		}

		readonly := NewListPatternOf(readonlyGeneralElementPattern)
		readonly.readonly = true
		return readonly, nil
	}

	var elements []Pattern
	if len(p.elements) > 0 {
		elements = make([]Pattern, len(p.elements))
	}

	for i, e := range p.elements {
		if !e.SymbolicValue().IsMutable() {
			elements[i] = e
			continue
		}
		potentiallyReadonly, ok := e.(PotentiallyReadonlyPattern)
		if !ok {
			return nil, FmtElementError(i, ErrNotConvertibleToReadonly)
		}
		readonly, err := potentiallyReadonly.ToReadonlyPattern()
		if err != nil {
			return nil, FmtElementError(i, err)
		}
		elements[i] = readonly
	}

	readonly := NewListPattern(elements)
	readonly.readonly = true
	return readonly, nil
}

func prettyPrintListPattern(
	w pprint.PrettyPrintWriter, tuplePattern bool,
	generalElementPattern Pattern, elementPatterns []Pattern,
	config *pprint.PrettyPrintConfig,

) {

	indentCount := w.ParentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	if generalElementPattern != nil {
		b := utils.StringAsBytes("%[]")

		if tuplePattern {
			b = utils.StringAsBytes("%#[]")
		}

		w.WriteBytes(b)

		generalElementPattern.PrettyPrint(w, config)

		if tuplePattern {
			w.WriteString(")")
		}
	}

	if w.Depth > config.MaxDepth && len(elementPatterns) > 0 {
		b := utils.StringAsBytes("%[(...)]")
		if tuplePattern {
			b = utils.StringAsBytes("%#(...)")
		}

		w.WriteBytes(b)
		return
	}

	start := utils.StringAsBytes("%[")
	if tuplePattern {
		start = utils.StringAsBytes("%#[")
	}
	w.WriteBytes(start)

	printIndices := !config.Compact && len(elementPatterns) > 10

	for i, v := range elementPatterns {

		if !config.Compact {
			w.WriteLFCR()
			w.WriteBytes(indent)

			//index
			if printIndices {
				if config.Colorize {
					w.WriteBytes(config.Colors.DiscreteColor)
				}
				if i < 10 {
					w.WriteByte(' ')
				}
				w.WriteString(strconv.FormatInt(int64(i), 10))
				w.WriteBytes(config.Colors.DiscreteColor)
				w.WriteColonSpace()

				if config.Colorize {
					w.WriteAnsiReset()
				}
			}
		}

		//element
		v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

		//comma & indent
		isLastEntry := i == len(elementPatterns)-1

		if !isLastEntry {
			w.WriteCommaSpace()
		}

	}

	if !config.Compact && len(elementPatterns) > 0 {
		w.WriteLFCR()
		w.WriteBytes(bytes.Repeat(config.Indent, w.Depth))
	}

	if tuplePattern {
		w.WriteClosingbracketClosingParen()
	} else {
		w.WriteByte(']')
	}
}

func (p *ListPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.readonly {
		w.WriteName("readonly ")
	}
	if p.elements != nil {
		prettyPrintListPattern(w, false, p.generalElement, p.elements, config)
		return
	}
	w.WriteString("%[]")
	p.generalElement.PrettyPrint(w, config)
}

func (p *ListPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ListPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	list, ok := v.(*List)
	if !ok || p.readonly != list.readonly {
		return false
	}

	if p.elements != nil {
		if !list.HasKnownLen() || list.KnownLen() != len(p.elements) {
			return false
		}
		for i, e := range p.elements {
			if !e.TestValue(list.elements[i], state) {
				return false
			}
		}
		return true
	} else {
		if list.HasKnownLen() {
			for _, e := range list.elements {
				if !p.generalElement.TestValue(e, state) {
					return false
				}
			}

			return true
		} else if p.generalElement.TestValue(list.generalElement, state) {
			return true
		}

		return false
	}

}

func (p *ListPattern) SymbolicValue() Value {
	list := &List{readonly: p.readonly}

	if p.elements != nil {
		list.elements = make([]Serializable, 0)
		for _, e := range p.elements {
			element := AsSerializableChecked(e.SymbolicValue())
			list.elements = append(list.elements, element)
		}
	} else {
		list.generalElement = AsSerializableChecked(p.generalElement.SymbolicValue())
	}
	return list
}

func (p *ListPattern) MigrationInitialValue() (Serializable, bool) {
	list := &List{}

	if p.elements != nil {
		list.elements = make([]Serializable, 0)

		for _, e := range p.elements {
			capable, ok := e.(MigrationInitialValueCapablePattern)
			if !ok {
				return nil, false
			}
			elemInitialValue, ok := capable.MigrationInitialValue()
			if !ok {
				return nil, false
			}

			list.elements = append(list.elements, elemInitialValue)
		}
	} else {
		capable, ok := p.generalElement.(MigrationInitialValueCapablePattern)
		if !ok {
			return nil, false
		}
		elemInitialValue, ok := capable.MigrationInitialValue()
		if !ok {
			return nil, false
		}
		list.generalElement = elemInitialValue
	}
	return list, true
}

func (p *ListPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ListPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *ListPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *ListPattern) WidestOfType() Value {
	return &ListPattern{}
}

// A TuplePattern represents a symbolic TuplePattern.
// .elements and .generalElement can never be both nil (nor both not nil).
type TuplePattern struct {
	elements       []Pattern
	generalElement Pattern

	NotCallablePatternMixin
	SerializableMixin
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

func (p *TuplePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*TuplePattern)

	if !ok {
		return false
	}

	if p.elements != nil {
		if other.elements == nil || len(p.elements) != len(other.elements) {
			return false
		}

		for i, e := range p.elements {
			if !e.Test(other.elements[i], state) {
				return false
			}
		}

		return true
	} else {
		if other.elements == nil {
			return p.generalElement.Test(other.generalElement, state)
		}

		for _, elem := range other.elements {
			if !p.generalElement.Test(elem, state) {
				return false
			}
		}
		return true
	}
}

func (p *TuplePattern) IsConcretizable() bool {
	if p.generalElement != nil {
		potentiallyConcretizable, ok := p.generalElement.(PotentiallyConcretizable)
		return ok && potentiallyConcretizable.IsConcretizable()
	}

	for _, v := range p.elements {
		if potentiallyConcretizable, ok := v.(PotentiallyConcretizable); !ok || !potentiallyConcretizable.IsConcretizable() {
			return false
		}
	}

	return true
}

func (p *TuplePattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	if p.generalElement != nil {
		concreteGeneralElement := utils.Must(Concretize(p.generalElement, ctx))
		return extData.ConcreteValueFactories.CreateListPattern(concreteGeneralElement, nil)
	}

	concreteElementPatterns := make([]any, len(p.elements))

	for i, e := range p.elements {
		concreteElemPattern := utils.Must(Concretize(e, ctx))
		concreteElementPatterns[i] = concreteElemPattern
	}

	return extData.ConcreteValueFactories.CreateTuplePattern(nil, concreteElementPatterns)
}

func (p *TuplePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.elements != nil {
		prettyPrintListPattern(w, true, p.generalElement, p.elements, config)
		return
	}
	w.WriteString("%#[]")
	p.generalElement.PrettyPrint(w.ZeroDepth(), config)
}

func (p *TuplePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *TuplePattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	tuple, ok := v.(*Tuple)
	if !ok {
		return false
	}

	if p.elements != nil {
		if !tuple.HasKnownLen() || tuple.KnownLen() != len(p.elements) {
			return false
		}
		for i, e := range p.elements {
			if !e.TestValue(tuple.elements[i], state) {
				return false
			}
		}
		return true
	} else {
		if tuple.HasKnownLen() {
			for _, e := range tuple.elements {
				if !p.generalElement.TestValue(e, state) {
					return false
				}
			}

			return true
		} else if p.generalElement.TestValue(tuple.generalElement, state) {
			return true
		}

		return false
	}

}

func (p *TuplePattern) SymbolicValue() Value {
	tuple := &Tuple{}

	if p.elements != nil {
		tuple.elements = make([]Serializable, 0)
		for _, e := range p.elements {
			element := AsSerializableChecked(e.SymbolicValue())
			tuple.elements = append(tuple.elements, element)
		}
	} else {
		tuple.generalElement = AsSerializableChecked(p.generalElement.SymbolicValue())
	}
	return tuple
}

func (p *TuplePattern) MigrationInitialValue() (Serializable, bool) {
	tuple := &Tuple{}

	if p.elements != nil {
		tuple.elements = make([]Serializable, 0)

		for _, e := range p.elements {
			capable, ok := e.(MigrationInitialValueCapablePattern)
			if !ok {
				return nil, false
			}
			elemInitialValue, ok := capable.MigrationInitialValue()
			if !ok {
				return nil, false
			}

			tuple.elements = append(tuple.elements, elemInitialValue)
		}
	} else {
		capable, ok := p.generalElement.(MigrationInitialValueCapablePattern)
		if !ok {
			return nil, false
		}
		elemInitialValue, ok := capable.MigrationInitialValue()
		if !ok {
			return nil, false
		}
		tuple.generalElement = elemInitialValue
	}
	return tuple, true
}

func (p *TuplePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *TuplePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *TuplePattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *TuplePattern) WidestOfType() Value {
	return ANY_TUPLE_PATTERN
}

// A UnionPattern represents a symbolic UnionPattern.
type UnionPattern struct {
	cases    []Pattern //if nil, any union pattern is matched
	disjoint bool

	NotCallablePatternMixin
	SerializableMixin
}

func NewUnionPattern(cases []Pattern, disjoint bool) (*UnionPattern, error) {
	if disjoint {
		for i, case1 := range cases {
			for j, case2 := range cases {
				if i != j && (case1.Test(case2, RecTestCallState{}) || case2.Test(case1, RecTestCallState{})) {
					return nil, errors.New("impossible to create symbolic disjoint union pattern: some cases intersect")
				}
			}
		}
	}

	cases, err := flattenUnionPatternCases(cases, disjoint, 0)
	if err != nil {
		return nil, err
	}

	return &UnionPattern{cases: cases, disjoint: disjoint}, nil
}

func NewDisjointStringUnionPattern(cases ...string) (*UnionPattern, error) {
	patterns := make([]Pattern, len(cases))
	for i, v := range cases {
		for j, other := range cases {
			if i != j && v == other {
				return nil, fmt.Errorf("duplicate case %q", v)
			}
		}

		patterns[i] = NewExactStringPatternWithConcreteValue(NewString(v))
	}

	return NewUnionPattern(patterns, true)
}

func flattenUnionPatternCases(cases []Pattern, disjoint bool, depth int) (results []Pattern, _ error) {
	if depth > MAX_UNION_PATTERN_FLATTENING_DEPTH {
		return nil, errors.New("maximum flattening depth exceeded")
	}

	if len(cases) == 0 {
		panic(errors.New("cases should have at least one element"))
	}

	changes := false
	results = cases

	for i, case_ := range cases {
		if union, ok := case_.(*UnionPattern); ok && union.disjoint == disjoint {
			if !changes {
				results = slices.Clone(cases[:i])
			}
			changes = true
			flattened, err := flattenUnionPatternCases(union.cases, disjoint, depth+1)
			if err != nil {
				return nil, err
			}
			results = append(results, flattened...)
		} else if changes {
			results = append(results, case_)
		}
	}

	return
}

func (p *UnionPattern) Cases() []Pattern {
	return p.cases
}

func (p *UnionPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*UnionPattern)

	if !ok || p.disjoint != other.disjoint {
		return false
	}

	if p.cases == nil {
		return true
	}

	if len(p.cases) != len(other.cases) {
		return false
	}

	for i, case_ := range p.cases {
		if !case_.Test(other.cases[i], state) {
			return false
		}
	}

	return true
}

func (p *UnionPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("(%| ")

	if p.disjoint {
		w.WriteString("(disjoint) ")
	}

	indentCount := w.ParentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	for i, case_ := range p.cases {
		if i > 0 {
			w.WriteByte('\n')
			w.WriteBytes(indent)
			w.WriteString("| ")
		}
		case_.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)
	}
	w.WriteString(")")
}

func (p *UnionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *UnionPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	var values []Value
	if multi, ok := v.(IMultivalue); ok {
		values = multi.OriginalMultivalue().values
	} else {
		values = []Value{v}
	}

	if p.disjoint {
		for _, val := range values {
			matchingCases := 0
			for _, case_ := range p.cases {
				if case_.TestValue(val, state) {
					matchingCases++
					if matchingCases > 1 {
						return false
					}
				}
			}
			if matchingCases == 0 {
				return false
			}
		}
	} else {
		for _, val := range values {
			ok := false
			for _, case_ := range p.cases {
				if case_.TestValue(val, state) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
	}

	return true
}

func (p *UnionPattern) SymbolicValue() Value {
	values := make([]Value, len(p.cases))

	for i, case_ := range p.cases {
		values[i] = case_.SymbolicValue()
	}

	return joinValues(values)
}

func (p *UnionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *UnionPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *UnionPattern) IteratorElementValue() Value {
	return ANY
}

func (p *UnionPattern) WidestOfType() Value {
	return &UnionPattern{}
}

// An IntersectionPattern represents a symbolic IntersectionPattern.
type IntersectionPattern struct {
	NotCallablePatternMixin
	cases []Pattern //if nil, any union pattern is matched
	value Value

	SerializableMixin
}

func NewIntersectionPattern(cases []Pattern) (*IntersectionPattern, error) {

	var values []Value
	for _, c := range cases {
		values = append(values, c.SymbolicValue())
	}

	value, err := getIntersection(0, values...)
	if err != nil {
		return nil, err
	}

	return &IntersectionPattern{
		cases: cases,
		value: value,
	}, nil
}

func (p *IntersectionPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*IntersectionPattern)

	if !ok {
		return false
	}

	if p.cases == nil {
		return true
	}

	if len(p.cases) > len(other.cases) {
		return false
	}

	//check that at each case matches at least one case in the other intersection pattern
	for _, case_ := range p.cases {
		ok := false
		for _, otherCase := range other.cases {
			if case_.Test(otherCase, state) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}

	return true
}

func (p *IntersectionPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	for _, case_ := range p.cases {
		if mv, ok := v.(IMultivalue); ok {
			for _, value := range mv.OriginalMultivalue().values {
				if !case_.TestValue(value, state) {
					return false
				}
			}
		} else if !case_.TestValue(v, state) {
			return false
		}
	}
	return true
}

func (p *IntersectionPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("(%& ")
	indentCount := w.ParentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	for i, case_ := range p.cases {
		if i > 0 {
			w.WriteByte('\n')
			w.WriteBytes(indent)
			w.WriteString("& ")
		}
		case_.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)
	}
	w.WriteString(")")
}

func (p *IntersectionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *IntersectionPattern) SymbolicValue() Value {
	return p.value
}

func (p *IntersectionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *IntersectionPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *IntersectionPattern) IteratorElementValue() Value {
	return ANY
}

func (p *IntersectionPattern) WidestOfType() Value {
	return &IntersectionPattern{}
}

// A OptionPattern represents a symbolic OptionPattern.
type OptionPattern struct {
	name    string
	pattern Pattern

	NotCallablePatternMixin
	SerializableMixin
}

func NewOptionPattern(name string, pattern Pattern) *OptionPattern {
	if name == "" {
		panic(errors.New("name should not be empty"))
	}
	return &OptionPattern{name: name, pattern: pattern}
}

func (p *OptionPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*OptionPattern)
	if !ok || (p.name != "" && other.name != p.name) {
		return false
	}
	return p.pattern.Test(other.pattern, state)
}

func (p *OptionPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("option-pattern(")
	if p.name != "" {
		NewString(p.name).PrettyPrint(w.ZeroIndent(), config)
		w.WriteString(", ")
	}
	p.pattern.PrettyPrint(w.ZeroIndent(), config)
	w.WriteByte(')')
}

func (p *OptionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *OptionPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	opt, ok := v.(*Option)
	if !ok || (p.name != "" && opt.name != p.name) {
		return false
	}
	return p.pattern.TestValue(opt.value, state)
}

func (p *OptionPattern) SymbolicValue() Value {
	if p.name == "" {
		return NewAnyNameOption(p.pattern.SymbolicValue())
	}
	return NewOption(p.name, p.pattern.SymbolicValue())
}

func (p *OptionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *OptionPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *OptionPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *OptionPattern) WidestOfType() Value {
	return ANY_OPTION_PATTERN
}

func evalPatternNode(n parse.Node, state *State) (Pattern, error) {
	switch node := n.(type) {
	case *parse.ObjectPatternLiteral,
		*parse.RecordPatternLiteral,
		*parse.ListPatternLiteral,
		*parse.TuplePatternLiteral,
		*parse.OptionPatternLiteral,
		*parse.RegularExpressionLiteral,

		*parse.PatternUnion,
		*parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
		*parse.FunctionPatternExpression,
		*parse.PatternCallExpression,
		*parse.OptionalPatternExpression,

		*parse.AbsolutePathPatternLiteral, *parse.RelativePathPatternLiteral,
		*parse.NamedSegmentPathPatternLiteral,
		*parse.URLPatternLiteral, *parse.HostPatternLiteral,
		*parse.PathPatternExpression,
		*parse.ReadonlyPatternExpression:
		pattern, err := symbolicEval(node, state)
		if err != nil {
			return nil, err
		}

		return pattern.(Pattern), nil
	case *parse.ComplexStringPatternPiece:
		return NewSequenceStringPattern(node, state.currentChunk().Node), nil
	default:
		v, err := symbolicEval(n, state)
		if err != nil {
			return nil, err
		}

		if patt, ok := v.(Pattern); ok {
			return patt, nil
		}

		var exactValue Serializable

		if v.IsMutable() {
			state.addError(makeSymbolicEvalError(n, state, ONLY_SERIALIZABLE_IMMUT_VALS_ALLOWED_IN_EXACT_VAL_PATTERN))
			exactValue = ANY_SERIALIZABLE
		} else if serializable, ok := AsSerializable(v).(Serializable); ok {
			exactValue = serializable
		} else {
			exactValue = ANY_SERIALIZABLE
			state.addError(makeSymbolicEvalError(n, state, ONLY_SERIALIZABLE_IMMUT_VALS_ALLOWED_IN_EXACT_VAL_PATTERN))
		}

		pattern, err := NewMostAdaptedExactPattern(exactValue)
		if err != nil {
			state.addError(makeSymbolicEvalError(n, state, err.Error()))
			return ANY_PATTERN, nil
		}
		return pattern, nil
	}
}

type TypePattern struct {
	val                 Value //symbolic value that represents concrete values matching, if nil any TypePattern is matched.
	call                func(ctx *Context, values []Value) (Pattern, error)
	stringPattern       func() (StringPattern, bool)
	concreteTypePattern any //we play safe

	SerializableMixin
}

func NewTypePattern(
	value Value, call func(ctx *Context, values []Value) (Pattern, error),
	stringPattern func() (StringPattern, bool), concrete any,
) *TypePattern {
	return &TypePattern{
		val:                 value,
		call:                call,
		stringPattern:       stringPattern,
		concreteTypePattern: concrete,
	}
}

func (p *TypePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*TypePattern)
	if !ok {
		return false
	}
	if p.val == nil {
		return true
	}
	return ok && p.val.Test(other.val, state)
}

func (patt *TypePattern) IsConcretizable() bool {
	return patt.concreteTypePattern != nil
}

func (patt *TypePattern) Concretize(ctx ConcreteContext) any {
	if !patt.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return patt.concreteTypePattern
}

func (p *TypePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.val == nil {
		w.WriteName("type-pattern")
		return
	}
	w.WriteName("type-pattern(")
	p.val.PrettyPrint(w.IncrDepth(), config)
	w.WriteString(")")
}

func (p *TypePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *TypePattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	if p.val == nil {
		return true
	}

	if mv, ok := v.(IMultivalue); ok {
		for _, val := range mv.OriginalMultivalue().getValues() {
			if !p.val.Test(val, state) {
				return false
			}
		}
		return true
	}

	return p.val.Test(v, state)
}

func (p *TypePattern) Call(ctx *Context, values []Value) (Pattern, error) {
	if p.call == nil {
		return nil, ErrPatternNotCallable
	}
	return p.call(ctx, values)
}

func (p *TypePattern) SymbolicValue() Value {
	if p.val == nil {
		return ANY
	}
	return p.val
}

func (p *TypePattern) MigrationInitialValue() (Serializable, bool) {
	if p.val == nil {
		return nil, false
	}

	if serializable, ok := p.val.(Serializable); ok && (IsSimpleSymbolicInoxVal(serializable) || serializable == ANY_STR_LIKE) {
		return serializable, true
	}
	return nil, false
}

func (p *TypePattern) StringPattern() (StringPattern, bool) {
	if p.val == nil {
		return nil, false
	}

	if p.stringPattern == nil {
		return nil, false
	}
	return p.stringPattern()
}

func (p *TypePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *TypePattern) IteratorElementValue() Value {
	return NEVER
}

func (p *TypePattern) WidestOfType() Value {
	return ANY_TYPE_PATTERN
}

type DifferencePattern struct {
	Base    Pattern
	Removed Pattern
	NotCallablePatternMixin
	SerializableMixin
}

func (p *DifferencePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*DifferencePattern)
	return ok && p.Base.Test(other.Base, state) && other.Removed.Test(other.Removed, state)
}

func (p *DifferencePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("(")
	p.Base.PrettyPrint(w.IncrDepth(), config)
	w.WriteString(" \\ ")
	p.Removed.PrettyPrint(w.IncrDepth(), config)
	w.WriteString(")")
}

func (p *DifferencePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *DifferencePattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return p.Base.Test(v, state) && !p.Removed.TestValue(v, state)
}

func (p *DifferencePattern) SymbolicValue() Value {
	//TODO
	panic(errors.New("SymbolicValue() not implement for DifferencePattern"))
}

func (p *DifferencePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *DifferencePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *DifferencePattern) IteratorElementValue() Value {
	//TODO
	return ANY
}

func (p *DifferencePattern) WidestOfType() Value {
	return &DifferencePattern{}
}

type OptionalPattern struct {
	pattern Pattern

	NotCallablePatternMixin
	SerializableMixin
}

func NewOptionalPattern(p Pattern) *OptionalPattern {
	return &OptionalPattern{pattern: p}
}

func (p *OptionalPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*OptionalPattern)
	return ok && p.pattern.Test(other.pattern, state)
}

func (p *OptionalPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	p.pattern.PrettyPrint(w, config)
	w.WriteByte('?')
}

func (p *OptionalPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *OptionalPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	if _, ok := v.(*NilT); ok {
		return true
	}
	return p.pattern.TestValue(v, state)
}

func (p *OptionalPattern) SymbolicValue() Value {
	//TODO
	return NewMultivalue(p.pattern.SymbolicValue(), Nil)
}

func (p *OptionalPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *OptionalPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *OptionalPattern) IteratorElementValue() Value {
	//TODO
	return ANY
}

func (p *OptionalPattern) WidestOfType() Value {
	return &OptionalPattern{}
}

type FunctionPattern struct {
	function *Function //if nil any function is matched

	NotCallablePatternMixin
	SerializableMixin
}

func (fn *FunctionPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*FunctionPattern)
	if !ok {
		return false
	}

	if fn.function == nil {
		return true
	}

	if other.function == nil {
		return false
	}

	return fn.function.Test(other.function, state)
}

func (pattern *FunctionPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch fn := v.(type) {
	case *Function:
		if pattern.function == nil {
			return true
		}

		return pattern.function.Test(fn, state)
	case *GoFunction:
		if pattern.function == nil {
			return true
		}

		if fn.fn == nil {
			return false
		}

		panic(errors.New("testing a go function against a function pattern is not supported yet"))
	case *InoxFunction:
		if pattern.function == nil {
			return true
		}

		return pattern.function.Test(fn, state)
	default:
		return false
	}
}

func (fn *FunctionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *FunctionPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (fn *FunctionPattern) IteratorElementValue() Value {
	//TODO
	return fn.function
}

func (fn *FunctionPattern) SymbolicValue() Value {
	return fn.function
}

func (p *FunctionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (fn *FunctionPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if fn.function == nil {
		w.WriteName("function-pattern")
		return
	}
	w.WriteName("function-pattern(")
	fn.function.PrettyPrint(w.WithDepthIndent(w.Depth+1, 0), config)
	w.WriteString(")")
}

func (fn *FunctionPattern) WidestOfType() Value {
	return ANY_FUNCTION_PATTERN
}

// A IntRangePattern represents a symbolic IntRangePattern.
// This symbolic Value does not support the multipleOf constraint, therefore the symbolic version
// of concrete IntRangePattern(s) with such a constraint should be ANY_INT_RANGE_PATTERN.
type IntRangePattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin

	intRange *IntRange
}

func NewIntRangePattern(intRange *IntRange) *IntRangePattern {
	return &IntRangePattern{intRange: intRange}
}

func (p *IntRangePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*IntRangePattern)
	if !ok {
		return false
	}
	return p.intRange.Test(otherPattern.intRange, state)
}

func (p *IntRangePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("int-range-pattern")
	if !p.intRange.hasValue {
		return
	}
	w.WriteByte('(')
	p.intRange.PrettyPrint(w.WithDepthIndent(w.Depth+1, 0), config)
	w.WriteByte(')')
}

func (p *IntRangePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *IntRangePattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	int, ok := v.(*Int)
	if !ok {
		return false
	}
	yes, _ := p.intRange.Contains(int)
	return yes
}

func (p *IntRangePattern) SymbolicValue() Value {
	return &Int{
		hasValue:        false,
		matchingPattern: p,
	}
}

func (p *IntRangePattern) StringPattern() (StringPattern, bool) {
	return NewIntRangeStringPattern(p), true
}

func (p *IntRangePattern) PropertyNames() []string {
	return nil
}

func (*IntRangePattern) Prop(name string) Value {
	switch name {
	default:
		return nil
	}
}

func (p *IntRangePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *IntRangePattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *IntRangePattern) WidestOfType() Value {
	return ANY_INT_RANGE_PATTERN
}

// A FloatRangePattern represents a symbolic FloatRangePattern.
type FloatRangePattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin

	floatRange *FloatRange
}

func NewFloatRangePattern(floatRange *FloatRange) *FloatRangePattern {
	return &FloatRangePattern{floatRange: floatRange}
}

func (p *FloatRangePattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*FloatRangePattern)
	if !ok {
		return false
	}
	return p.floatRange.Test(otherPattern.floatRange, state)
}

func (p *FloatRangePattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("float-range-pattern")
	if !p.floatRange.hasValue {
		return
	}
	w.WriteByte('(')
	p.floatRange.PrettyPrint(w.WithDepthIndent(w.Depth+1, 0), config)
	w.WriteByte(')')
}

func (p *FloatRangePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *FloatRangePattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	float, ok := v.(*Float)
	if !ok {
		return false
	}
	yes, _ := p.floatRange.Contains(float)
	return yes
}

func (p *FloatRangePattern) SymbolicValue() Value {
	return &Float{
		hasValue:        false,
		matchingPattern: p,
	}
}

func (p *FloatRangePattern) StringPattern() (StringPattern, bool) {
	return NewFloatRangeStringPattern(p), true
}

func (p *FloatRangePattern) PropertyNames() []string {
	return nil
}

func (*FloatRangePattern) Prop(name string) Value {
	switch name {
	default:
		return nil
	}
}

func (p *FloatRangePattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *FloatRangePattern) IteratorElementValue() Value {
	return ANY_FLOAT
}

func (p *FloatRangePattern) WidestOfType() Value {
	return ANY_INT_RANGE_PATTERN
}

// An EventPattern represents a symbolic EventPattern.
type EventPattern struct {
	ValuePattern Pattern

	NotCallablePatternMixin
	SerializableMixin
}

func NewEventPattern(valuePattern Pattern) (*EventPattern, error) {
	if !isAnyPattern(valuePattern) && valuePattern.SymbolicValue().IsMutable() {
		return nil, fmt.Errorf("failed to create event pattern: value should be immutable: %T", valuePattern.SymbolicValue())
	}

	return &EventPattern{
		ValuePattern: valuePattern,
	}, nil
}

func (p *EventPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*EventPattern)
	return ok && p.ValuePattern.Test(other.ValuePattern, state)
}

func (p *EventPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("event-pattern(")
	p.ValuePattern.PrettyPrint(w.ZeroIndent(), config)
	w.WriteByte(')')
}

func (p *EventPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *EventPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	event, ok := v.(*Event)
	if !ok {
		return false
	}
	return p.ValuePattern.TestValue(event, state)
}

func (p *EventPattern) SymbolicValue() Value {
	return utils.Must(NewEvent(p.ValuePattern.SymbolicValue()))
}

func (p *EventPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *EventPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *EventPattern) IteratorElementValue() Value {
	//TODO
	return ANY
}

func (p *EventPattern) WidestOfType() Value {
	return ANY_EVENT_PATTERN
}

// A MutationPattern represents a symbolic MutationPattern.
// (work in progress)
type MutationPattern struct {
	kind         *Int //if nil any mutation pattern is matched.
	data0Pattern Pattern

	NotCallablePatternMixin
	SerializableMixin
}

func NewMutationPattern(kind *Int, data0Pattern Pattern) *MutationPattern {
	return &MutationPattern{
		kind:         kind,
		data0Pattern: data0Pattern,
	}
}

func (p *MutationPattern) matchAnyMutationPattern() bool {
	return p.kind == nil
}

func (p *MutationPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*MutationPattern)
	if !ok {
		return false
	}
	if p.matchAnyMutationPattern() {
		return true
	}
	return ok && p.kind.Test(other.kind, state) && p.data0Pattern.Test(other.data0Pattern, state)
}

func (p *MutationPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.matchAnyMutationPattern() {
		w.WriteName("mutation-pattern")
		return
	}
	w.WriteName("mutation-pattern(?, ")
	p.data0Pattern.PrettyPrint(w.ZeroIndent(), config)
	w.WriteByte(')')
}

func (p *MutationPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *MutationPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	event, ok := v.(*Event)
	if !ok {
		return false
	}
	if p.matchAnyMutationPattern() {
		return true
	}
	//TODO: check kind
	return p.data0Pattern.TestValue(event, state)
}

func (p *MutationPattern) SymbolicValue() Value {
	//TODO
	return &Mutation{}
}

func (p *MutationPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *MutationPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *MutationPattern) IteratorElementValue() Value {
	//TODO
	return &Mutation{}
}

func (p *MutationPattern) WidestOfType() Value {
	return ANY_MUTATION_PATTERN
}

// A PatternNamespace represents a symbolic PatternNamespace.
type PatternNamespace struct {
	entries map[string]Pattern //if nil, matches any pattern namespace
}

func NewPatternNamespace(patterns map[string]Pattern) *PatternNamespace {
	return &PatternNamespace{
		entries: maps.Clone(patterns),
	}
}

func (ns *PatternNamespace) ForEachPattern(fn func(name string, patt Pattern) error) error {
	for k, v := range ns.entries {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (ns *PatternNamespace) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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
		if !e.Test(otherNS.entries[i], state) {
			return false
		}
	}
	return true
}

func (ns *PatternNamespace) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if ns.entries != nil {
		if w.Depth > config.MaxDepth && len(ns.entries) > 0 {
			w.WriteString("(..pattern-namespace..)")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteName("pattern-namespace{")

		keys := maps.Keys(ns.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteAnsiReset()
			}

			//colon
			w.WriteColonSpace()

			//value
			v := ns.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteCommaSpace()
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}

		w.WriteManyBytes(bytes.Repeat(config.Indent, w.Depth), []byte{'}'})
		return
	}
	w.WriteName("pattern-namespace")
}

func (ns *PatternNamespace) WidestOfType() Value {
	return ANY_PATTERN_NAMESPACE
}
