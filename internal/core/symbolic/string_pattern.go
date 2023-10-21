package symbolic

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"strings"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_EXACT_STR_PATTERN = NewExactStringPattern(nil) //this pattern does not match any string

	ANY_SEQ_STRING_PATTERN             = &SequenceStringPattern{}
	ANY_LENGTH_CHECKING_STRING_PATTERN = &LengthCheckingStringPattern{minLength: -1}
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

func (p *AnyStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%string-pattern")))
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
	return ANY_STR
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
	value *String //if nil any string is matched
	NotCallablePatternMixin
	SerializableMixin
}

func NewExactStringPattern(value *String) *ExactStringPattern {
	if value != nil && !value.hasValue {
		panic(errors.New("string should have a value"))
	}
	return &ExactStringPattern{value: value}
}

func (p *ExactStringPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*ExactStringPattern)
	return ok && (p.value == nil || (otherPattern.value != nil && p.value.value == otherPattern.value.value))
}

func (p *ExactStringPattern) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	return extData.ConcreteValueFactories.CreateExactStringPattern(utils.Must(Concretize(p.value, ctx)))
}

func (p *ExactStringPattern) IsConcretizable() bool {
	return IsConcretizable(p.value)
}

func (p *ExactStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%exact-string-pattern")))

	if p.value != nil {
		utils.Must(w.Write(utils.StringAsBytes("(")))
		p.value.PrettyPrint(w, config, depth+1, parentIndentCount)
		utils.Must(w.Write(utils.StringAsBytes(")")))
	}
}

func (p *ExactStringPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *ExactStringPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	stringLike, ok := v.(StringLike)
	if !ok || !stringLike.GetOrBuildString().hasValue || p.value == nil {
		return false
	}

	return p.value.value == stringLike.GetOrBuildString().value
}

func (p *ExactStringPattern) SymbolicValue() Value {
	return p.value
}

func (p *ExactStringPattern) MigrationInitialValue() (Serializable, bool) {
	return p.value, true
}

func (p *ExactStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ExactStringPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *ExactStringPattern) IteratorElementValue() Value {
	return p.value
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

func (p *LengthCheckingStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.minLength == -1 {
		utils.Must(w.Write(utils.StringAsBytes("%length-checking-string-pattern")))
	} else {
		utils.Must(w.Write(utils.StringAsBytes(fmt.Sprintf("%%length-checking-string-pattern(%d..%d)", p.minLength, p.maxLength))))
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
		return ANY_STR
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
			elements = append(elements, parse.SPrint(e, chunk, parse.PrintConfig{TrimStart: true, TrimEnd: true}))
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

func (p *SequenceStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%sequence-string-pattern")))
	if p.node != nil {
		utils.Must(w.Write(utils.StringAsBytes("(")))
		utils.Must(w.Write(utils.StringAsBytes(p.stringifiedNode)))
		utils.Must(w.Write(utils.StringAsBytes(")")))
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
	return ANY_STR
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

func (p *ParserBasedPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%parser-based-pattern")))
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
	return ANY_STR
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
	return ANY_STR
}

func (p *ParserBasedPattern) WidestOfType() Value {
	return ANY_PARSED_BASED_STRING_PATTERN
}

//
