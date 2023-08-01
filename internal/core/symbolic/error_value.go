package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ERR_PROPNAMES = []string{"text", "data"}
	ANY_ERR       = &Error{data: ANY}
)

type Error struct {
	data SymbolicValue
	UnassignablePropsMixin
	SerializableMixin
}

func NewError(data SymbolicValue) *Error {
	return &Error{data: data}
}

func (e *Error) Test(v SymbolicValue) bool {
	otherError, ok := v.(*Error)

	return ok && e.data.Test(otherError.data)
}

func (e *Error) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%error")))
	return
}

func (e *Error) WidestOfType() SymbolicValue {
	return ANY_ERR
}

func (e *Error) Prop(name string) SymbolicValue {
	switch name {
	case "text":
		return ANY_STR_LIKE
	case "data":
		return e.data
	}
	panic(FormatErrPropertyDoesNotExist(name, e))
}

func (*Error) PropertyNames() []string {
	return ERR_PROPNAMES
}
