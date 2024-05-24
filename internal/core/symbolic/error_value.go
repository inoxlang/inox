package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ERR_PROPNAMES = []string{"text", "data"}
	ANY_ERR       = &Error{data: ANY}
)

type Error struct {
	data Value
	UnassignablePropsMixin
	SerializableMixin
}

func NewError(data Value) *Error {
	return &Error{data: data}
}

func (e *Error) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherError, ok := v.(*Error)

	return ok && e.data.Test(otherError.data, state)
}

func (e *Error) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("error")
}

func (e *Error) WidestOfType() Value {
	return ANY_ERR
}

func (e *Error) Prop(name string) Value {
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
