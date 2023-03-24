package internal

// A StringPatternElement represents a symbolic StringPatternElement.
type StringPatternElement interface {
	Pattern
	HasRegex() bool
}

// An AnyStringPatternElement represents a symbolic StringPatternElement we dont know the concrete type.
type AnyStringPatternElement struct {
	NotCallablePatternMixin
	_ int
}

func (p *AnyStringPatternElement) Test(v SymbolicValue) bool {
	_, ok := v.(StringPatternElement)
	return ok
}

func (p *AnyStringPatternElement) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *AnyStringPatternElement) IsWidenable() bool {
	return false
}

func (p *AnyStringPatternElement) String() string {
	return "string-pattern"
}

func (p *AnyStringPatternElement) HasUnderylingPattern() bool {
	return true
}

func (p *AnyStringPatternElement) TestValue(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (p *AnyStringPatternElement) MatchGroups(v SymbolicValue) (bool, map[string]SymbolicValue) {
	//TODO
	return false, nil
}

func (p *AnyStringPatternElement) SymbolicValue() SymbolicValue {
	return &String{}
}

func (p *AnyStringPatternElement) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *AnyStringPatternElement) HasRegex() bool {
	//TODO
	return false
}

func (p *AnyStringPatternElement) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *AnyStringPatternElement) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *AnyStringPatternElement) WidestOfType() SymbolicValue {
	return ANY_STR_PATTERN_ELEM
}

// An SequenceStringPattern represents a symbolic SequenceStringPattern
type SequenceStringPattern struct {
	_ int
}

func (p *SequenceStringPattern) Test(v SymbolicValue) bool {
	_, ok := v.(StringPatternElement)
	return ok
}

func (p *SequenceStringPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *SequenceStringPattern) IsWidenable() bool {
	return false
}

func (p *SequenceStringPattern) String() string {
	return "sequence-string-pattern"
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
	return &String{}
}

func (p *SequenceStringPattern) StringPattern() (StringPatternElement, bool) {
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
	return &AnyStringPatternElement{}
}

// An ParserBasedPattern represents a symbolic ParserBasedPattern
type ParserBasedPattern struct {
	_ int
}

func NewParserBasedPattern() *ParserBasedPattern {
	return &ParserBasedPattern{}
}

func (p *ParserBasedPattern) Test(v SymbolicValue) bool {
	_, ok := v.(StringPatternElement)
	return ok
}

func (p *ParserBasedPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *ParserBasedPattern) IsWidenable() bool {
	return false
}

func (p *ParserBasedPattern) String() string {
	return "parser-based-pattern"
}

func (p *ParserBasedPattern) HasUnderylingPattern() bool {
	return true
}

func (p *ParserBasedPattern) TestValue(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (p *ParserBasedPattern) SymbolicValue() SymbolicValue {
	return &String{}
}

func (p *ParserBasedPattern) StringPattern() (StringPatternElement, bool) {
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
	return &AnyStringPatternElement{}
}

//
