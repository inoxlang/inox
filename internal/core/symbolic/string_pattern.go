package symbolic

import (
	"bufio"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_EXACT_STR_PATTERN = NewExactStringPattern(nil) //this pattern does not match any string

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

func (p *AnyStringPattern) Test(v SymbolicValue) bool {
	_, ok := v.(StringPattern)
	return ok
}

func (p *AnyStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%string-pattern")))
	return
}

func (p *AnyStringPattern) HasUnderylingPattern() bool {
	return true
}

func (p *AnyStringPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (p *AnyStringPattern) MatchGroups(v SymbolicValue) (bool, map[string]SymbolicValue) {
	//TODO
	return false, nil
}

func (p *AnyStringPattern) SymbolicValue() SymbolicValue {
	return ANY_STR
}

func (p *AnyStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnyStringPattern) HasRegex() bool {
	//TODO
	return false
}

func (p *AnyStringPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *AnyStringPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *AnyStringPattern) WidestOfType() SymbolicValue {
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

func (p *ExactStringPattern) Test(v SymbolicValue) bool {
	otherPattern, ok := v.(*ExactStringPattern)
	return ok && (p.value == nil || (otherPattern.value != nil && p.value.value == otherPattern.value.value))
}

func (p *ExactStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%exact-string-pattern")))

	if p.value != nil {
		utils.Must(w.Write(utils.StringAsBytes("(")))
		p.value.PrettyPrint(w, config, depth+1, parentIndentCount)
		utils.Must(w.Write(utils.StringAsBytes(")")))
	}
}

func (p *ExactStringPattern) HasUnderylingPattern() bool {
	return true
}

func (p *ExactStringPattern) TestValue(v SymbolicValue) bool {
	stringLike, ok := v.(StringLike)
	if !ok || !stringLike.GetOrBuildString().hasValue || p.value == nil {
		return false
	}

	return p.value.value == stringLike.GetOrBuildString().value
}

func (p *ExactStringPattern) SymbolicValue() SymbolicValue {
	return p.value
}

func (p *ExactStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ExactStringPattern) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (p *ExactStringPattern) IteratorElementValue() SymbolicValue {
	return p.value
}

func (p *ExactStringPattern) WidestOfType() SymbolicValue {
	return ANY_EXACT_STR_PATTERN
}

func (p *ExactStringPattern) HasRegex() bool {
	//TODO
	return true
}

// An SequenceStringPattern represents a symbolic SequenceStringPattern
type SequenceStringPattern struct {
	SerializableMixin
}

func (p *SequenceStringPattern) Test(v SymbolicValue) bool {
	_, ok := v.(StringPattern)
	return ok
}

func (p *SequenceStringPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%sequence-string-pattern")))
	return
}

func (p *SequenceStringPattern) HasUnderylingPattern() bool {
	return true
}

func (p *SequenceStringPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (p *SequenceStringPattern) MatchGroups(v SymbolicValue) (bool, map[string]SymbolicValue) {
	//TODO
	return false, nil
}

func (p *SequenceStringPattern) SymbolicValue() SymbolicValue {
	return ANY_STR
}

func (p *SequenceStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *SequenceStringPattern) HasRegex() bool {
	//TODO
	return false
}

func (p *SequenceStringPattern) Call(ctx *Context, values []SymbolicValue) (Pattern, error) {
	return &SequenceStringPattern{}, nil
}

func (p *SequenceStringPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *SequenceStringPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *SequenceStringPattern) WidestOfType() SymbolicValue {
	return &AnyStringPattern{}
}

// An ParserBasedPattern represents a symbolic ParserBasedPattern
type ParserBasedPattern struct {
	SerializableMixin
}

func NewParserBasedPattern() *ParserBasedPattern {
	return &ParserBasedPattern{}
}

func (p *ParserBasedPattern) Test(v SymbolicValue) bool {
	_, ok := v.(StringPattern)
	return ok
}

func (p *ParserBasedPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%parser-based-pattern")))
	return
}

func (p *ParserBasedPattern) HasUnderylingPattern() bool {
	return true
}

func (p *ParserBasedPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (p *ParserBasedPattern) SymbolicValue() SymbolicValue {
	return ANY_STR
}

func (p *ParserBasedPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *ParserBasedPattern) HasRegex() bool {
	//TODO
	return false
}

func (p *ParserBasedPattern) Call(ctx *Context, values []SymbolicValue) (Pattern, error) {
	return &ParserBasedPattern{}, nil
}

func (p *ParserBasedPattern) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *ParserBasedPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *ParserBasedPattern) WidestOfType() SymbolicValue {
	return &AnyStringPattern{}
}

//
