package symbolic

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"sort"
	"strconv"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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

	ANY_PATTERN              = &AnyPattern{}
	ANY_SERIALIZABLE_PATTERN = &AnySerializablePattern{}
	ANY_PATH_PATTERN         = &PathPattern{
		dirConstraint: UnspecifiedDirOrFilePath,
		absoluteness:  UnspecifiedPathAbsoluteness,
	}
	ANY_URL_PATTERN   = &URLPattern{}
	ANY_HOST_PATTERN  = &HostPattern{}
	ANY_STR_PATTERN   = &AnyStringPattern{}
	ANY_LIST_PATTERN  = &ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN}
	ANY_TUPLE_PATTERN = &TuplePattern{generalElement: ANY_SERIALIZABLE_PATTERN}

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

	ANY_REGEX_PATTERN       = &RegexPattern{}
	ANY_INT_RANGE_PATTERN   = &IntRangePattern{}
	ANY_FLOAT_RANGE_PATTERN = &FloatRangePattern{}

	ErrPatternNotCallable                        = errors.New("pattern is not callable")
	ErrValueAlreadyInitialized                   = errors.New("value already initialized")
	ErrValueInExactPatternValueShouldBeImmutable = errors.New("the value in an exact value pattern should be immutable")
)

// A Pattern represents a symbolic Pattern.
type Pattern interface {
	Serializable
	Iterable

	HasUnderlyingPattern() bool

	//equivalent of Test() for concrete patterns
	TestValue(v SymbolicValue, state RecTestCallState) bool

	Call(ctx *Context, values []SymbolicValue) (Pattern, error)

	//returns a symbolic value that represent all concrete values that match against this pattern
	SymbolicValue() SymbolicValue

	StringPattern() (StringPattern, bool)
}

type NotCallablePatternMixin struct {
}

func (NotCallablePatternMixin) Call(ctx *Context, values []SymbolicValue) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

// A GroupPattern represents a symbolic GroupPattern.
type GroupPattern interface {
	Pattern
	MatchGroups(SymbolicValue) (ok bool, groups map[string]Serializable)
}

func isAnyPattern(val SymbolicValue) bool {
	_, ok := val.(*AnyPattern)
	return ok
}

type IPropsPattern interface {
	SymbolicValue
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

func (p *AnyPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Pattern)
	return ok
}

func (p *AnyPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%pattern")))
}

func (p *AnyPattern) HasUnderlyingPattern() bool {
	return false
}

func (p *AnyPattern) TestValue(SymbolicValue, RecTestCallState) bool {
	return true
}

func (p *AnyPattern) SymbolicValue() SymbolicValue {
	return ANY
}

func (p *AnyPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnyPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *AnyPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *AnyPattern) WidestOfType() SymbolicValue {
	return ANY_PATTERN
}

// An AnySerialiablePattern represents a symbolic Pattern we do not know the concrete type that represents patterns
// of serializable values.
type AnySerializablePattern struct {
	NotCallablePatternMixin
	SerializableMixin
}

func (p *AnySerializablePattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	patt, ok := v.(Pattern)
	if ok {
		return false
	}

	_, ok = AsSerializable(patt.SymbolicValue()).(Serializable)
	return ok
}

func (p *AnySerializablePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%pattern")))
}

func (p *AnySerializablePattern) HasUnderlyingPattern() bool {
	return false
}

func (p *AnySerializablePattern) TestValue(SymbolicValue, RecTestCallState) bool {
	return true
}

func (p *AnySerializablePattern) SymbolicValue() SymbolicValue {
	return ANY_SERIALIZABLE
}

func (p *AnySerializablePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnySerializablePattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *AnySerializablePattern) IteratorElementValue() SymbolicValue {
	return ANY_SERIALIZABLE
}

func (p *AnySerializablePattern) WidestOfType() SymbolicValue {
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
	printConfig := parse.PrintConfig{TrimStart: true, TrimEnd: true}

	return &PathPattern{
		node:            n,
		stringifiedNode: parse.SPrint(n, chunk, printConfig),
	}
}

func (p *PathPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *PathPattern) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *PathPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *PathPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {

	if p.hasValue {
		utils.Must(w.Write(utils.StringAsBytes("%" + p.value)))
		return
	}

	s := "%path-pattern"

	if p.node != nil {
		utils.Must(w.Write(utils.StringAsBytes(s)))
		utils.Must(w.Write(utils.StringAsBytes("(")))
		utils.Must(w.Write(utils.StringAsBytes(p.stringifiedNode)))
		utils.Must(w.Write(utils.StringAsBytes(")")))
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

	utils.Must(w.Write(utils.StringAsBytes(s)))
}

func (p *PathPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *PathPattern) SymbolicValue() SymbolicValue {
	return NewPathMatchingPattern(p)
}

func (p *PathPattern) StringPattern() (StringPattern, bool) {
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
	return ANY_INT
}

func (p *PathPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *PathPattern) underlyingString() *String {
	return ANY_STR
}

func (p *PathPattern) WidestOfType() SymbolicValue {
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
	printConfig := parse.PrintConfig{TrimStart: true, TrimEnd: true}
	return &URLPattern{
		node:            n,
		stringifiedNode: parse.SPrint(n, chunk, printConfig),
	}
}

func (p *URLPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *URLPattern) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *URLPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.hasValue {
		utils.Must(w.Write(utils.StringAsBytes("%" + p.value)))
		return
	}

	s := "%url-pattern"

	if p.node != nil {
		utils.Must(w.Write(utils.StringAsBytes(s)))
		utils.Must(w.Write(utils.StringAsBytes("(")))
		utils.Must(w.Write(utils.StringAsBytes(p.stringifiedNode)))
		utils.Must(w.Write(utils.StringAsBytes(")")))
		return
	}
}

func (p *URLPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *URLPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	u, ok := v.(*URL)
	if !ok {
		return false
	}

	if u.pattern == p {
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

func (p *URLPattern) SymbolicValue() SymbolicValue {
	return NewUrlMatchingPattern(p)
}

func (p *URLPattern) StringPattern() (StringPattern, bool) {
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
	return ANY_INT
}

func (p *URLPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *URLPattern) underlyingString() *String {
	return ANY_STR
}

func (p *URLPattern) WidestOfType() SymbolicValue {
	return ANY_URL_PATTERN
}

// A HostPattern represents a symbolic HostPattern.
type HostPattern struct {
	hasValue bool
	value    string

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
	printConfig := parse.PrintConfig{TrimStart: true, TrimEnd: true}
	return &HostPattern{
		node:            n,
		stringifiedNode: parse.SPrint(n, chunk, printConfig),
	}
}

func (p *HostPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

	return true
}

func (p *HostPattern) IsConcretizable() bool {
	return p.hasValue
}

func (p *HostPattern) Static() Pattern {
	return &TypePattern{val: p.WidestOfType()}
}

func (p *HostPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.hasValue {
		utils.Must(w.Write(utils.StringAsBytes("%" + p.value)))
		return
	}

	s := "%host-pattern"

	if p.node != nil {
		utils.Must(w.Write(utils.StringAsBytes(s)))
		utils.Must(w.Write(utils.StringAsBytes("(")))
		utils.Must(w.Write(utils.StringAsBytes(p.stringifiedNode)))
		utils.Must(w.Write(utils.StringAsBytes(")")))
		return
	}
}

func (p *HostPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *HostPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

	return !p.hasValue
}

func (p *HostPattern) SymbolicValue() SymbolicValue {
	return NewHostMatchingPattern(p)
}

func (p *HostPattern) StringPattern() (StringPattern, bool) {
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
	return ANY_INT
}

func (p *HostPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *HostPattern) underlyingString() *String {
	return ANY_STR
}

func (p *HostPattern) WidestOfType() SymbolicValue {
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

func (p *NamedSegmentPathPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*NamedSegmentPathPattern)
	if !ok {
		return false
	}

	return p.node == nil || p.node == otherPattern.node
}

func (p *NamedSegmentPathPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.node == nil {
		utils.Must(w.Write(utils.StringAsBytes("%named-segment-path-pattern")))
		return
	}
	utils.Must(fmt.Fprintf(w, "%%named-segment-path-pattern(%p)", p.node))
}

func (p NamedSegmentPathPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *NamedSegmentPathPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Path)
	return ok
}

func (p *NamedSegmentPathPattern) SymbolicValue() SymbolicValue {
	return &Path{}
}

func (p *NamedSegmentPathPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *NamedSegmentPathPattern) MatchGroups(v SymbolicValue) (bool, map[string]Serializable) {
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
	return ANY_INT
}

func (p *NamedSegmentPathPattern) IteratorElementValue() SymbolicValue {
	return &Path{}
}

func (p *NamedSegmentPathPattern) WidestOfType() SymbolicValue {
	return &NamedSegmentPathPattern{}
}

// An ExactValuePattern represents a symbolic ExactValuePattern.
type ExactValuePattern struct {
	value Serializable //immutable in most cases

	NotCallablePatternMixin
	SerializableMixin
}

func NewExactValuePattern(v Serializable) (*ExactValuePattern, error) {
	if !IsAnySerializable(v) && v.IsMutable() {
		return nil, ErrValueInExactPatternValueShouldBeImmutable
	}
	return &ExactValuePattern{value: v}, nil
}

func NewUncheckedExactValuePattern(v Serializable) (*ExactValuePattern, error) {
	return &ExactValuePattern{value: v}, nil
}

func NewMostAdaptedExactPattern(value Serializable) (Pattern, error) {
	if !IsAny(value) && value.IsMutable() {
		return nil, ErrValueInExactPatternValueShouldBeImmutable
	}
	if s, ok := value.(StringLike); ok {
		return NewExactStringPattern(s.GetOrBuildString()), nil
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
func (p *ExactValuePattern) GetVal() SymbolicValue {
	return p.value
}

func (p *ExactValuePattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *ExactValuePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%exact-value-pattern(\n")))
	innerIndentCount := parentIndentCount + 2
	innerIndent := bytes.Repeat(config.Indent, innerIndentCount)
	parentIndent := innerIndent[:len(innerIndent)-2*len(config.Indent)]

	utils.Must(w.Write(innerIndent))
	p.value.PrettyPrint(w, config, depth+2, innerIndentCount)

	utils.PanicIfErr(w.WriteByte('\n'))
	utils.Must(w.Write(parentIndent))
	utils.PanicIfErr(w.WriteByte(')'))

}

func (p *ExactValuePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ExactValuePattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return p.value.Test(v, state) && v.Test(p.value, state)
}

func (p *ExactValuePattern) SymbolicValue() SymbolicValue {
	return p.value
}

func (p *ExactValuePattern) MigrationInitialValue() (Serializable, bool) {
	return p.value, true
}

func (p *ExactValuePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ExactValuePattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *ExactValuePattern) IteratorElementValue() SymbolicValue {
	return p.value
}

func (p *ExactValuePattern) WidestOfType() SymbolicValue {
	return &ExactValuePattern{value: ANY_SERIALIZABLE}
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
	syntaxRegexp = utils.TurnCapturingGroupsIntoNonCapturing(syntaxRegexp)

	return &RegexPattern{
		regex:  regexp,
		syntax: syntaxRegexp,
	}
}

func (p *RegexPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPatt, ok := v.(*RegexPattern)
	if !ok {
		return false
	}
	return p.regex == nil || (otherPatt.regex != nil && p.syntax.Equal(otherPatt.syntax))
}

func (p *RegexPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.regex != nil {
		utils.Must(w.Write(utils.StringAsBytes("%`" + p.regex.String() + "`")))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%regex-pattern")))
}

func (p *RegexPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *RegexPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *RegexPattern) SymbolicValue() SymbolicValue {
	return NewStringMatchingPattern(p)
}

func (p *RegexPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *RegexPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *RegexPattern) IteratorElementValue() SymbolicValue {
	return ANY_STR
}

func (p *RegexPattern) WidestOfType() SymbolicValue {
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

func (p *ObjectPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *ObjectPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *ObjectPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.readonly {
		utils.Must(w.Write(utils.StringAsBytes("%readonly ")))
	}
	if p.entries != nil {
		if depth > config.MaxDepth && len(p.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("%{(...)}")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write([]byte{'%', '{'}))

		var keys []string
		for k := range p.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			if _, ok := p.optionalEntries[k]; ok {
				utils.PanicIfErr(w.WriteByte('?'))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := p.entries[k]
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry /* /*p.inexact*/ {
				utils.Must(w.Write(COMMA_SPACE))
			}
		}

		// if p.inexact {
		// 	if !config.Compact {
		// 		utils.Must(w.Write(LF_CR))
		// 		utils.Must(w.Write(indent))
		// 	}

		// 	utils.Must(w.Write(THREE_DOTS))
		// }

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
		if err := w.WriteByte('}'); err != nil {
			panic(err)
		}
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("%object-pattern")))
}

func (p *ObjectPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ObjectPattern) SymbolicValue() SymbolicValue {
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

func (p *ObjectPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *ObjectPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *ObjectPattern) WidestOfType() SymbolicValue {
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

func (p *RecordPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *RecordPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.entries != nil {
		if depth > config.MaxDepth && len(p.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("record(%{(...)})")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes("record(%{")))

		var keys []string
		for k := range p.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			if _, ok := p.optionalEntries[k]; ok {
				utils.PanicIfErr(w.WriteByte('?'))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := p.entries[k]
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry || p.inexact {
				utils.Must(w.Write(COMMA_SPACE))

			}
		}

		// if p.inexact {
		// 	if !config.Compact {
		// 		utils.Must(w.Write(LF_CR))
		// 		utils.Must(w.Write(indent))
		// 	}

		// 	utils.Must(w.Write(THREE_DOTS))
		// }

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
		utils.Must(w.Write(CLOSING_CURLY_BRACKET_CLOSING_PAREN))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%record-pattern")))
}

func (p *RecordPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *RecordPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *RecordPattern) SymbolicValue() SymbolicValue {
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

func (p *RecordPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *RecordPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *RecordPattern) WidestOfType() SymbolicValue {
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

func (p *ListPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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
	w *bufio.Writer, tuplePattern bool,
	generalElementPattern Pattern, elementPatterns []Pattern,
	config *pprint.PrettyPrintConfig, depth int, parentIndentCount int,

) {

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	if generalElementPattern != nil {
		b := utils.StringAsBytes("%[]")

		if tuplePattern {
			b = utils.StringAsBytes("%#[]")
		}

		utils.Must(w.Write(b))

		generalElementPattern.PrettyPrint(w, config, depth, parentIndentCount)

		if tuplePattern {
			utils.Must(w.Write(utils.StringAsBytes(")")))
		}
	}

	if depth > config.MaxDepth && len(elementPatterns) > 0 {
		b := utils.StringAsBytes("%[(...)]")
		if tuplePattern {
			b = utils.StringAsBytes("%#(...)")
		}

		utils.Must(w.Write(b))
		return
	}

	start := utils.StringAsBytes("%[")
	if tuplePattern {
		start = utils.StringAsBytes("%#[")
	}
	utils.Must(w.Write(start))

	printIndices := !config.Compact && len(elementPatterns) > 10

	for i, v := range elementPatterns {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))

			//index
			if printIndices {
				if config.Colorize {
					utils.Must(w.Write(config.Colors.DiscreteColor))
				}
				if i < 10 {
					utils.PanicIfErr(w.WriteByte(' '))
				}
				utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(int64(i), 10))))
				utils.Must(w.Write(config.Colors.DiscreteColor))
				utils.Must(w.Write(COLON_SPACE))

				if config.Colorize {
					utils.Must(w.Write(ANSI_RESET_SEQUENCE))
				}
			}
		}

		//element
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(elementPatterns)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}

	}

	if !config.Compact && len(elementPatterns) > 0 {
		utils.Must(w.Write(LF_CR))
		utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	}

	if tuplePattern {
		utils.Must(w.Write(CLOSING_BRACKET_CLOSING_PAREN))
	} else {
		utils.PanicIfErr(w.WriteByte(']'))
	}
}

func (p *ListPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.readonly {
		utils.Must(w.Write(utils.StringAsBytes("%readonly ")))
	}
	if p.elements != nil {
		prettyPrintListPattern(w, false, p.generalElement, p.elements, config, depth, parentIndentCount)
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%[]")))
	p.generalElement.PrettyPrint(w, config, depth, parentIndentCount)
}

func (p *ListPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ListPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *ListPattern) SymbolicValue() SymbolicValue {
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

func (p *ListPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *ListPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *ListPattern) WidestOfType() SymbolicValue {
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

func (p *TuplePattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *TuplePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.elements != nil {
		prettyPrintListPattern(w, true, p.generalElement, p.elements, config, depth, parentIndentCount)
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%#[]")))
	p.generalElement.PrettyPrint(w, config, 0, parentIndentCount)
}

func (p *TuplePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *TuplePattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *TuplePattern) SymbolicValue() SymbolicValue {
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

func (p *TuplePattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *TuplePattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *TuplePattern) WidestOfType() SymbolicValue {
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

func (p *UnionPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *UnionPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("(%| ")))

	if p.disjoint {
		utils.Must(w.Write(utils.StringAsBytes("(disjoint) ")))
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	for i, case_ := range p.cases {
		if i > 0 {
			utils.PanicIfErr(w.WriteByte('\n'))
			utils.Must(w.Write(indent))
			utils.Must(w.Write(utils.StringAsBytes("| ")))
		}
		case_.PrettyPrint(w, config, depth+1, indentCount)
	}
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (p *UnionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *UnionPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	var values []SymbolicValue
	if multi, ok := v.(*Multivalue); ok {
		values = multi.values
	} else {
		values = []SymbolicValue{v}
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

func (p *UnionPattern) SymbolicValue() SymbolicValue {
	values := make([]SymbolicValue, len(p.cases))

	for i, case_ := range p.cases {
		values[i] = case_.SymbolicValue()
	}

	return joinValues(values)
}

func (p *UnionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *UnionPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
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
	cases []Pattern //if nil, any union pattern is matched
	value SymbolicValue

	SerializableMixin
}

func NewIntersectionPattern(cases []Pattern) (*IntersectionPattern, error) {

	var values []SymbolicValue
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

func (p *IntersectionPattern) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (p *IntersectionPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
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

func (p *IntersectionPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("(%& ")))
	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	for i, case_ := range p.cases {
		if i > 0 {
			utils.PanicIfErr(w.WriteByte('\n'))
			utils.Must(w.Write(indent))
			utils.Must(w.Write(utils.StringAsBytes("& ")))
		}
		case_.PrettyPrint(w, config, depth+1, indentCount)
	}
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (p *IntersectionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *IntersectionPattern) SymbolicValue() SymbolicValue {
	return p.value
}

func (p *IntersectionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *IntersectionPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *IntersectionPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *IntersectionPattern) WidestOfType() SymbolicValue {
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

func (p *OptionPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*OptionPattern)
	if !ok || (p.name != "" && other.name != p.name) {
		return false
	}
	return p.pattern.Test(other.pattern, state)
}

func (p *OptionPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%option-pattern(")))
	if p.name != "" {
		NewString(p.name).PrettyPrint(w, config, depth, 0)
		utils.Must(w.Write(utils.StringAsBytes(", ")))
	}
	p.pattern.PrettyPrint(w, config, depth, 0)
	utils.PanicIfErr(w.WriteByte(')'))
}

func (p *OptionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *OptionPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	opt, ok := v.(*Option)
	if !ok || (p.name != "" && opt.name != p.name) {
		return false
	}
	return p.pattern.TestValue(opt.value, state)
}

func (p *OptionPattern) SymbolicValue() SymbolicValue {
	if p.name == "" {
		return NewAnyNameOption(p.pattern.SymbolicValue())
	}
	return NewOption(p.name, p.pattern.SymbolicValue())
}

func (p *OptionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *OptionPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *OptionPattern) IteratorElementValue() SymbolicValue {
	return p.SymbolicValue()
}

func (p *OptionPattern) WidestOfType() SymbolicValue {
	return ANY_OPTION_PATTERN
}

func symbolicallyEvalPatternNode(n parse.Node, state *State) (Pattern, error) {
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

		if p, ok := v.(*ExactValuePattern); ok {
			return p, nil
		}

		return &ExactValuePattern{value: AsSerializableChecked(v)}, nil
	}
}

type TypePattern struct {
	val                 SymbolicValue //symbolic value that represents concrete values matching
	call                func(ctx *Context, values []SymbolicValue) (Pattern, error)
	stringPattern       func() (StringPattern, bool)
	concreteTypePattern any //we play safe

	SerializableMixin
}

func NewTypePattern(
	value SymbolicValue, call func(ctx *Context, values []SymbolicValue) (Pattern, error),
	stringPattern func() (StringPattern, bool), concrete any,
) *TypePattern {
	return &TypePattern{
		val:                 value,
		call:                call,
		stringPattern:       stringPattern,
		concreteTypePattern: concrete,
	}
}

func (p *TypePattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*TypePattern)
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

func (p *TypePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%type-pattern(")))
	p.val.PrettyPrint(w, config, depth+1, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (p *TypePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *TypePattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return p.val.Test(v, state)
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

func (p *TypePattern) MigrationInitialValue() (Serializable, bool) {
	if serializable, ok := p.val.(Serializable); ok && (IsSimpleSymbolicInoxVal(serializable) || serializable == ANY_STR_LIKE) {
		return serializable, true
	}
	return nil, false
}

func (p *TypePattern) StringPattern() (StringPattern, bool) {
	if p.stringPattern == nil {
		return nil, false
	}
	return p.stringPattern()
}

func (p *TypePattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *TypePattern) IteratorElementValue() SymbolicValue {
	return nil
}

func (p *TypePattern) WidestOfType() SymbolicValue {
	return &TypePattern{val: ANY}
}

type DifferencePattern struct {
	Base    Pattern
	Removed Pattern
	NotCallablePatternMixin
	SerializableMixin
}

func (p *DifferencePattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*DifferencePattern)
	return ok && p.Base.Test(other.Base, state) && other.Removed.Test(other.Removed, state)
}

func (p *DifferencePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("(")))
	p.Base.PrettyPrint(w, config, depth+1, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes(" \\ ")))
	p.Removed.PrettyPrint(w, config, depth+1, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (p *DifferencePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *DifferencePattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return p.Base.Test(v, state) && !p.Removed.TestValue(v, state)
}

func (p *DifferencePattern) SymbolicValue() SymbolicValue {
	//TODO
	panic(errors.New("SymbolicValue() not implement for DifferencePattern"))
}

func (p *DifferencePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *DifferencePattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *DifferencePattern) IteratorElementValue() SymbolicValue {
	//TODO
	return ANY
}

func (p *DifferencePattern) WidestOfType() SymbolicValue {
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

func (p *OptionalPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*OptionalPattern)
	return ok && p.pattern.Test(other.pattern, state)
}

func (p *OptionalPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	p.pattern.PrettyPrint(w, config, depth, parentIndentCount)
	utils.PanicIfErr(w.WriteByte('?'))
}

func (p *OptionalPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *OptionalPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	if _, ok := v.(*NilT); ok {
		return true
	}
	return p.pattern.TestValue(v, state)
}

func (p *OptionalPattern) SymbolicValue() SymbolicValue {
	//TODO
	return NewMultivalue(p.pattern.SymbolicValue(), Nil)
}

func (p *OptionalPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *OptionalPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *OptionalPattern) IteratorElementValue() SymbolicValue {
	//TODO
	return ANY
}

func (p *OptionalPattern) WidestOfType() SymbolicValue {
	return &OptionalPattern{}
}

type FunctionPattern struct {
	parameters              []SymbolicValue
	parameterNames          []string
	firstOptionalParamIndex int //-1 if no optional parameters
	isVariadic              bool

	node       *parse.FunctionPatternExpression //if nil, any function is matched
	nodeChunk  *parse.Chunk
	returnType SymbolicValue

	NotCallablePatternMixin
	SerializableMixin
}

func (fn *FunctionPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*FunctionPattern)
	if !ok {
		return false
	}
	if fn.node == nil {
		return true
	}

	if other.node == nil {
		return false
	}

	return utils.SamePointer(fn.node, other.node)
}

func (pattern *FunctionPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch fn := v.(type) {
	case *Function:
		if pattern.node == nil {
			return true
		}
		return pattern.Test(fn.pattern, state)
	case *GoFunction:
		if pattern.node == nil {
			return true
		}

		if fn.fn == nil {
			return false
		}

		panic(errors.New("testing a go function against a function pattern is not supported yet"))

	case *InoxFunction:
		if pattern.node == nil {
			return true
		}

		fnExpr := fn.FuncExpr()
		if fnExpr == nil {
			return false
		}

		if len(fnExpr.Parameters) != len(pattern.node.Parameters) || fnExpr.NonVariadicParamCount() != pattern.node.NonVariadicParamCount() {
			return false
		}

		for i, param := range pattern.node.Parameters {
			actualParam := fnExpr.Parameters[i]

			if (param.Type == nil) != (actualParam.Type == nil) {
				return false
			}

			printConfig := parse.PrintConfig{TrimStart: true}
			if parse.SPrint(param.Type, pattern.nodeChunk, printConfig) != parse.SPrint(actualParam.Type, fn.nodeChunk, printConfig) {
				return false
			}
		}

		return pattern.returnType.Test(fn.result, state)
	default:
		return false
	}
}

func (fn *FunctionPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *FunctionPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (fn *FunctionPattern) IteratorElementValue() SymbolicValue {
	//TODO
	return &Function{pattern: fn}
}

func (fn *FunctionPattern) SymbolicValue() SymbolicValue {
	return &Function{fn.parameters, fn.firstOptionalParamIndex, fn.parameterNames, nil, fn.isVariadic, fn}
}

func (p *FunctionPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (fn *FunctionPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if fn.node == nil {
		utils.Must(w.Write(utils.StringAsBytes("%function-pattern")))
		return
	}
	utils.Must(fmt.Fprintf(w, "%%function-pattern(%v)", fn.node))
}

func (fn *FunctionPattern) WidestOfType() SymbolicValue {
	return &FunctionPattern{firstOptionalParamIndex: -1}
}

// A IntRangePattern represents a symbolic IntRangePattern.
type IntRangePattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin
}

func (p *IntRangePattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*IntRangePattern)
	return ok
}

func (p *IntRangePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%int-range-pattern")))
}

func (p *IntRangePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *IntRangePattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Int)
	return ok
}

func (p *IntRangePattern) SymbolicValue() SymbolicValue {
	return ANY_INT
}

func (p *IntRangePattern) StringPattern() (StringPattern, bool) {
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
	return ANY_INT
}

func (p *IntRangePattern) IteratorElementValue() SymbolicValue {
	return ANY_INT
}

func (p *IntRangePattern) WidestOfType() SymbolicValue {
	return ANY_INT_RANGE_PATTERN
}

// A FloatRangePattern represents a symbolic FloatRangePattern.
type FloatRangePattern struct {
	NotCallablePatternMixin
	UnassignablePropsMixin
	SerializableMixin
}

func (p *FloatRangePattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*FloatRangePattern)
	return ok
}

func (p *FloatRangePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%float-range-pattern")))
}

func (p *FloatRangePattern) HasUnderlyingPattern() bool {
	return true
}

func (p *FloatRangePattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Float)
	return ok
}

func (p *FloatRangePattern) SymbolicValue() SymbolicValue {
	return ANY_FLOAT
}

func (p *FloatRangePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *FloatRangePattern) PropertyNames() []string {
	return nil
}

func (*FloatRangePattern) Prop(name string) SymbolicValue {
	switch name {
	default:
		return nil
	}
}

func (p *FloatRangePattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *FloatRangePattern) IteratorElementValue() SymbolicValue {
	return ANY_FLOAT
}

func (p *FloatRangePattern) WidestOfType() SymbolicValue {
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

func (p *EventPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*EventPattern)
	return ok && p.ValuePattern.Test(other.ValuePattern, state)
}

func (p *EventPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%event-pattern(")))
	p.ValuePattern.PrettyPrint(w, config, depth, 0)
	utils.PanicIfErr(w.WriteByte(')'))
}

func (p *EventPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *EventPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	event, ok := v.(*Event)
	if !ok {
		return false
	}
	return p.ValuePattern.TestValue(event, state)
}

func (p *EventPattern) SymbolicValue() SymbolicValue {
	return utils.Must(NewEvent(p.ValuePattern.SymbolicValue()))
}

func (p *EventPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *EventPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
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
	kind         *Int
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

func (p *MutationPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*MutationPattern)
	return ok && p.kind.Test(other.kind, state) && p.data0Pattern.Test(other.data0Pattern, state)
}

func (p *MutationPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%mutation(?, ")))
	p.data0Pattern.PrettyPrint(w, config, depth, 0)
	utils.PanicIfErr(w.WriteByte(')'))
}

func (p *MutationPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *MutationPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	event, ok := v.(*Event)
	if !ok {
		return false
	}
	return p.data0Pattern.TestValue(event, state)
}

func (p *MutationPattern) SymbolicValue() SymbolicValue {
	//TODO
	return &Mutation{}
}

func (p *MutationPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *MutationPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
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

func (ns *PatternNamespace) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (ns *PatternNamespace) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if ns.entries != nil {
		if depth > config.MaxDepth && len(ns.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("(..pattern-namespace..)")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes("pattern-namespace{")))

		keys := maps.Keys(ns.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := ns.entries[k]
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				utils.Must(w.Write(COMMA_SPACE))
			}
		}

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%pattern-namespace")))
}

func (ns *PatternNamespace) WidestOfType() SymbolicValue {
	return &PatternNamespace{}
}
