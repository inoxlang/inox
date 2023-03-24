package internal

import "errors"

var (
	ANY_FORMAT = &AnyFormat{}
	_          = []Format{ANY_FORMAT}

	ErrInvalidFormattingArgument = errors.New("invalid formatting argument")
)

type Format interface {
	Pattern
	Format(v SymbolicValue) error
}

// An AnyFormat represents a symbolic Pattern we do not know the concrete type.
type AnyFormat struct {
	NotCallablePatternMixin
	_ int
}

func (p *AnyFormat) Test(v SymbolicValue) bool {
	_, ok := v.(Format)
	return ok
}

func (p *AnyFormat) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (p *AnyFormat) IsWidenable() bool {
	return false
}

func (p *AnyFormat) String() string {
	return "format"
}

func (p *AnyFormat) HasUnderylingPattern() bool {
	return false
}

func (p *AnyFormat) TestValue(SymbolicValue) bool {
	return true
}

func (p *AnyFormat) SymbolicValue() SymbolicValue {
	return ANY
}

func (p *AnyFormat) StringPattern() (StringPatternElement, bool) {
	return nil, false
}

func (p *AnyFormat) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (p *AnyFormat) IteratorElementValue() SymbolicValue {
	return ANY
}

func (p *AnyFormat) WidestOfType() SymbolicValue {
	return ANY_FORMAT
}

func (p *AnyFormat) Format(v SymbolicValue) error {
	return ErrInvalidFormattingArgument
}
