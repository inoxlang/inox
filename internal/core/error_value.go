package core

import "fmt"

var (
	ERR_PROPNAMES = []string{"text", "data"}

	_ = error(Error{})
)

// An Error represents an error with some immutable data, Error implements Value.
type Error struct {
	goError error
	data    Serializable
}

func NewError(err error, data Serializable) Error {
	if data.IsMutable() {
		panic(fmt.Errorf("failed to create error: data should be immutable: %T", data))
	}
	return Error{
		goError: err,
		data:    data,
	}
}

func (e Error) Text() string {
	return e.goError.Error()
}

func (e Error) Error() string {
	return e.Text()
}

func (e Error) Data() Value {
	return e.data
}

func (e Error) Prop(ctx *Context, name string) Value {
	switch name {
	case "text":
		return String(e.goError.Error())
	case "data":
		return e.data
	default:
		panic(FormatErrPropertyDoesNotExist(name, e))
	}
}

func (Error) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (Error) PropertyNames(ctx *Context) []string {
	return ERR_PROPNAMES
}
