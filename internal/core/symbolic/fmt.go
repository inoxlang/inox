package symbolic

import (
	"errors"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_FORMAT = &AnyFormat{}
	_          = []Format{ANY_FORMAT}

	ErrInvalidFormattingArgument = errors.New("invalid formatting argument")
)

type Format interface {
	Pattern
	Format(v Value) error
}

// An AnyFormat represents a symbolic Pattern we do not know the concrete type.
type AnyFormat struct {
	NotCallablePatternMixin
	SerializableMixin
}

func (p *AnyFormat) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Format)
	return ok
}

func (p *AnyFormat) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("format")
}

func (p *AnyFormat) HasUnderlyingPattern() bool {
	return false
}

func (p *AnyFormat) TestValue(Value, RecTestCallState) bool {
	return true
}

func (p *AnyFormat) SymbolicValue() Value {
	return ANY
}

func (p *AnyFormat) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnyFormat) IteratorElementKey() Value {
	return ANY_INT
}

func (p *AnyFormat) IteratorElementValue() Value {
	return ANY
}

func (p *AnyFormat) WidestOfType() Value {
	return ANY_FORMAT
}

func (p *AnyFormat) Format(v Value) error {
	return ErrInvalidFormattingArgument
}
