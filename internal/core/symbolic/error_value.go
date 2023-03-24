package internal

var (
	ERR_PROPNAMES = []string{"text", "data"}
	ANY_ERR       = &Error{data: ANY}
)

type Error struct {
	UnassignablePropsMixin
	data SymbolicValue
}

func NewError(data SymbolicValue) *Error {
	return &Error{data: data}
}

func (e *Error) Test(v SymbolicValue) bool {
	otherError, ok := v.(*Error)

	return ok && e.data.Test(otherError.data)
}

func (e *Error) Widen() (SymbolicValue, bool) {
	if !e.data.IsWidenable() {
		return nil, false
	}
	widenedData, _ := e.data.Widen()
	return &Error{data: widenedData}, true
}

func (e *Error) IsWidenable() bool {
	return e.data.IsWidenable()
}

func (e *Error) String() string {
	return "error"
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
