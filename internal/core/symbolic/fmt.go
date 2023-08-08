package symbolic

import (
	"bufio"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

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
	SerializableMixin
}

func (p *AnyFormat) Test(v SymbolicValue) bool {
	_, ok := v.(Format)
	return ok
}

func (p *AnyFormat) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%format")))
	return
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

func (p *AnyFormat) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (p *AnyFormat) IteratorElementKey() SymbolicValue {
	return ANY_INT
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
