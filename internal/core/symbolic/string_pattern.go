package symbolic

import (
	"errors"
	"math"
	"strings"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_EXACT_STR_PATTERN = &ExactStringPattern{} //this pattern does not match any string

	ANY_SEQ_STRING_PATTERN             = &SequenceStringPattern{}
	ANY_LENGTH_CHECKING_STRING_PATTERN = &LengthCheckingStringPattern{minLength: -1}
	ANY_INT_RANGE_STRING_PATTERN       = &IntRangeStringPattern{}
	ANY_FLOAT_RANGE_STRING_PATTERN     = &FloatRangeStringPattern{}
	ANY_PARSED_BASED_STRING_PATTERN    = &ParserBasedPattern{}
)

// A StringPattern represents a symbolic StringPattern.
type StringPattern interface {
	Pattern
	HasRegex() bool
}

// An AnyStringPattern represents a symbolic StringPatternElement we dont know the concrete type.
type AnyStringPattern struct {
	NotCallablePatternMixin
	SerializableMixin
}

func (p *AnyStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(StringPattern)
	return ok
}

func (p *AnyStringPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("string-pattern")
}

func (p *AnyStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *AnyStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	_, ok := v.(StringLike)
	return ok
}

func (p *AnyStringPattern) MatchGroups(v Value) (bool, map[string]Value) {
	//TODO
	return false, nil
}

func (p *AnyStringPattern) SymbolicValue() Value {
	return ANY_STRING
}

func (p *AnyStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnyStringPattern) HasRegex() bool {
	//TODO
	return false
}

func (p *AnyStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *AnyStringPattern) IteratorElementValue() Value {
	return ANY
}

func (p *AnyStringPattern) WidestOfType() Value {
	return ANY_STR_PATTERN
}

// An ExactStringPattern represents a symbolic ExactStringPattern.
type ExactStringPattern struct {
	//any ExactStringPattern is matched if both fields are nil.
	concretizable *String
	runTimeValue  *strLikeRunTimeValue

	NotCallablePatternMixin
	SerializableMixin
}

func NewExactStringPatternWithConcreteValue(value *String) *ExactStringPattern {
	if !value.IsConcretizable() {
		panic(errors.New("string should have a value"))
	}
	return &ExactStringPattern{concretizable: value}
}

func NewExactStringPatternWithRunTimeValue(rv *strLikeRunTimeValue) *ExactStringPattern {
	return &ExactStringPattern{runTimeValue: rv}
}

func (p *ExactStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*ExactStringPattern)
	if !ok {
		return false
	}
	if p.concretizable == nil && p.runTimeValue == nil {
		return true
	}
	if (p.concretizable == nil) != (otherPattern.concretizable == nil) || (p.runTimeValue == nil) != (otherPattern.runTimeValue == nil) {
		return false
	}

	if p.concretizable != nil {
		return p.concretizable.value == otherPattern.concretizable.value
	}

	return p.runTimeValue.Test(otherPattern.runTimeValue, state)
}

func (p *ExactStringPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return extData.ConcreteValueFactories.CreateExactStringPattern(utils.Must(Concretize(p.concretizable, ctx)))
}

func (p *ExactStringPattern) IsConcretizable() bool {
	return p.concretizable != nil
}

func (p *ExactStringPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("exact-string-pattern")

	if p.concretizable != nil {
		w.WriteString("(")
		p.concretizable.PrettyPrint(w.IncrDepth(), config)
		w.WriteString(")")
	}

	if p.runTimeValue != nil {
		w.WriteString("(")
		p.runTimeValue.PrettyPrint(w.IncrDepth(), config)
		w.WriteString(")")
	}

}

func (p *ExactStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ExactStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	stringLike, ok := v.(StringLike)

	if !ok {
		return false
	}

	if p.runTimeValue != nil {
		return p.runTimeValue.Test(v, state)
	}

	if p.concretizable == nil {
		return false
	}

	str := stringLike.GetOrBuildString()
	return p.concretizable.Test(str, state)
}

func (p *ExactStringPattern) SymbolicValue() Value {
	if p.concretizable != nil {
		return p.concretizable
	}
	if p.runTimeValue == nil {
		return NEVER
	}
	return p.runTimeValue
}

func (p *ExactStringPattern) MigrationInitialValue() (Serializable, bool) {
	if p.concretizable != nil {
		return p.concretizable, true
	}
	return nil, false
}

func (p *ExactStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ExactStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *ExactStringPattern) IteratorElementValue() Value {
	return p.concretizable
}

func (p *ExactStringPattern) WidestOfType() Value {
	return ANY_EXACT_STR_PATTERN
}

func (p *ExactStringPattern) HasRegex() bool {
	//TODO
	return true
}

// An LengthCheckingStringPattern represents a symbolic LengthCheckingStringPattern
type LengthCheckingStringPattern struct {
	SerializableMixin
	NotCallablePatternMixin

	//if -1 any length checking string pattern is matched
	minLength int64
	maxLength int64
}

func NewLengthCheckingStringPattern(minLength, maxLength int64) *LengthCheckingStringPattern {
	if minLength > maxLength {
		panic(errors.New("minLength should be <= maxLength"))
	}

	if minLength < 0 || maxLength < 0 {
		panic(errors.New("minLength and maxLength should be less or equal to zero"))
	}

	return &LengthCheckingStringPattern{
		minLength: minLength,
		maxLength: maxLength,
	}
}

func (p *LengthCheckingStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*LengthCheckingStringPattern)
	if !ok {
		return false
	}

	return p.minLength == -1 || (otherPattern.minLength >= p.minLength && otherPattern.maxLength <= p.maxLength)
}

func (p *LengthCheckingStringPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.minLength == -1 {
		w.WriteName("length-checking-string-pattern")
	} else {
		w.WriteNameF("length-checking-string-pattern(%d..%d)", p.minLength, p.maxLength)
	}
}

func (p *LengthCheckingStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *LengthCheckingStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	strLike, ok := v.(StringLike)
	if !ok {
		return false
	}

	if p.minLength == -1 {
		return true
	}

	s := strLike.GetOrBuildString()

	if pattern, ok := s.pattern.(*LengthCheckingStringPattern); ok && *pattern == *p {
		return true
	}

	if s.hasValue {
		return int64(len(s.value)) >= p.minLength && int64(len(s.value)) <= p.maxLength
	}

	if s.minLengthPlusOne != 0 {
		return s.minLength() >= p.minLength && s.maxLength <= p.maxLength
	}

	return s.maxLength == math.MaxInt64
}

func (p *LengthCheckingStringPattern) MatchGroups(v Value) (bool, map[string]Value) {
	return false, nil
}

func (p *LengthCheckingStringPattern) SymbolicValue() Value {
	if p.minLength == -1 {
		return ANY_STRING
	}
	return NewStringWithLengthRange(p.minLength, p.maxLength)
}

func (p *LengthCheckingStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *LengthCheckingStringPattern) HasRegex() bool {
	return true
}

func (p *LengthCheckingStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *LengthCheckingStringPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *LengthCheckingStringPattern) WidestOfType() Value {
	return ANY_SEQ_STRING_PATTERN
}

// An SequenceStringPattern represents a symbolic SequenceStringPattern
type SequenceStringPattern struct {
	SerializableMixin
	NotCallablePatternMixin
	node            *parse.ComplexStringPatternPiece //if nil any sequence string pattern is matched
	stringifiedNode string                           //empty if node is nil
}

func NewSequenceStringPattern(node *parse.ComplexStringPatternPiece, chunk *parse.Chunk) *SequenceStringPattern {
	var stringifiedNode string
	if node != nil {
		var elements []string
		for _, e := range node.Elements {
			elements = append(elements, parse.SPrint(e, chunk, parse.PrintConfig{}))
		}
		stringifiedNode = strings.Join(elements, " ")
	}

	return &SequenceStringPattern{
		node:            node,
		stringifiedNode: stringifiedNode,
	}
}

func (p *SequenceStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPatt, ok := v.(*SequenceStringPattern)
	if !ok {
		return false
	}
	if p.node == nil {
		return true
	}
	return p.node == otherPatt.node
}

func (p *SequenceStringPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("sequence-string-pattern")
	if p.node != nil {
		w.WriteString("(")
		w.WriteString(p.stringifiedNode)
		w.WriteString(")")
	}
}

func (p *SequenceStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *SequenceStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	strLike, ok := v.(StringLike)
	return ok && strLike.GetOrBuildString().pattern == p
}

func (p *SequenceStringPattern) MatchGroups(v Value) (bool, map[string]Value) {
	//it's not possible to know if a string matches the sequence pattern.
	return false, nil
}

func (p *SequenceStringPattern) SymbolicValue() Value {
	//it's not possible to know if a string matches the sequence pattern.
	return NewStringMatchingPattern(p)
}

func (p *SequenceStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *SequenceStringPattern) HasRegex() bool {
	//TODO
	return false
}

func (p *SequenceStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *SequenceStringPattern) IteratorElementValue() Value {
	return ANY_STRING
}

func (p *SequenceStringPattern) WidestOfType() Value {
	return ANY_SEQ_STRING_PATTERN
}

// An ParserBasedPattern represents a symbolic ParserBasedPattern
type ParserBasedPattern struct {
	SerializableMixin
	NotCallablePatternMixin
}

func NewParserBasedPattern() *ParserBasedPattern {
	return &ParserBasedPattern{}
}

func (p *ParserBasedPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(StringPattern)
	return ok
}

func (p *ParserBasedPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("parser-based-pattern")
}

func (p *ParserBasedPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ParserBasedPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	_, ok := v.(StringLike)
	return ok
}

func (p *ParserBasedPattern) SymbolicValue() Value {
	return ANY_STRING
}

func (p *ParserBasedPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ParserBasedPattern) HasRegex() bool {
	//TODO
	return false
}

func (p *ParserBasedPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *ParserBasedPattern) IteratorElementValue() Value {
	return ANY_STRING
}

func (p *ParserBasedPattern) WidestOfType() Value {
	return ANY_PARSED_BASED_STRING_PATTERN
}

// An IntRangeStringPattern represents a symbolic IntRangeStringPattern.
type IntRangeStringPattern struct {
	NotCallablePatternMixin
	SerializableMixin

	pattern *IntRangePattern //if nil any int range string pattern is matched
}

func NewIntRangeStringPattern(p *IntRangePattern) *IntRangeStringPattern {
	return &IntRangeStringPattern{pattern: p}
}

func (p *IntRangeStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*IntRangeStringPattern)
	if !ok {
		return false
	}
	if p.pattern == nil {
		return true
	}
	if otherPattern.pattern == nil {
		return false
	}
	return p.pattern.Test(otherPattern.pattern, state)
}

func (p *IntRangeStringPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("int-range-string-pattern")

	if p.pattern != nil {
		w.WriteString("(")
		p.pattern.PrettyPrint(w.IncrDepth(), config)
		w.WriteString(")")
	}
}

func (p *IntRangeStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *IntRangeStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	stringLike, ok := v.(StringLike)
	if !ok {
		return false
	}
	str := stringLike.GetOrBuildString()
	if str.pattern == nil {
		return false
	}

	return p.Test(str.pattern, state)
}

func (p *IntRangeStringPattern) SymbolicValue() Value {
	return NewStringMatchingPattern(p)
}

func (p *IntRangeStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *IntRangeStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *IntRangeStringPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *IntRangeStringPattern) WidestOfType() Value {
	return ANY_INT_RANGE_STRING_PATTERN
}

func (p *IntRangeStringPattern) HasRegex() bool {
	//TODO
	return true
}

// An FloatRangeStringPattern represents a symbolic FloatRangeStringPattern.
type FloatRangeStringPattern struct {
	NotCallablePatternMixin
	SerializableMixin

	pattern *FloatRangePattern //if nil any float range string pattern is matched
}

func NewFloatRangeStringPattern(p *FloatRangePattern) *FloatRangeStringPattern {
	return &FloatRangeStringPattern{pattern: p}
}

func (p *FloatRangeStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*FloatRangeStringPattern)
	if !ok {
		return false
	}
	if p.pattern == nil {
		return true
	}
	if otherPattern.pattern == nil {
		return false
	}
	return p.pattern.Test(otherPattern.pattern, state)
}

func (p *FloatRangeStringPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("float-range-string-pattern")

	if p.pattern != nil {
		w.WriteString("(")
		p.pattern.PrettyPrint(w.IncrDepth(), config)
		w.WriteString(")")
	}
}

func (p *FloatRangeStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *FloatRangeStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	stringLike, ok := v.(StringLike)
	if !ok {
		return false
	}
	str := stringLike.GetOrBuildString()
	if str.pattern == nil {
		return false
	}

	return p.Test(str.pattern, state)
}

func (p *FloatRangeStringPattern) SymbolicValue() Value {
	return NewStringMatchingPattern(p)
}

func (p *FloatRangeStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *FloatRangeStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *FloatRangeStringPattern) IteratorElementValue() Value {
	return p.SymbolicValue()
}

func (p *FloatRangeStringPattern) WidestOfType() Value {
	return ANY_FLOAT_RANGE_STRING_PATTERN
}

func (p *FloatRangeStringPattern) HasRegex() bool {
	//TODO
	return true
}
