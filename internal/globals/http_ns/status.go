package http_ns

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

var (
	ErrOutOfBoundsStatusCode = errors.New("out of bounds status code")
	_                        = []core.Value{Status{}, StatusCode(100)}
)

type Status struct {
	code         StatusCode //example: 200
	reasonPhrase string     //example: "OK"
	text         string     //code + reason phrase, example: "200 OK"
}

func ParseStatus(str string) (Status, error) {
	firstPart, secondPart, found := strings.Cut(str, " ")
	if !found || firstPart == "" || secondPart == "" {
		return Status{}, fmt.Errorf("invalid status: %q", str)
	}
	code, err := strconv.Atoi(firstPart)
	if err != nil || code < 100 || code > 599 {
		return Status{}, fmt.Errorf("invalid status code '%s'", str)
	}

	return Status{
		code:         StatusCode(code),
		text:         str,
		reasonPhrase: secondPart,
	}, nil
}

func (s Status) Code() StatusCode {
	s.code.assertInBounds()
	return s.code
}

func (s Status) ReasonPhrase() string {
	return s.reasonPhrase
}

func (s Status) FullText() string {
	if s.text == "" {
		s.code.assertInBounds()
		return strconv.Itoa(int(s.code)) + " " + s.reasonPhrase
	}
	return s.text
}

func (*Status) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (s *Status) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "code":
		return s.code
	case "full-text":
		return core.Str(s.FullText())
	default:
		return core.GetGoMethodOrPanic(name, s)
	}
}

func (Status) PropertyNames(*core.Context) []string {
	return http_ns_symb.STATUS_PROPNAMES
}

func (Status) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

type StatusCode uint16

func (c StatusCode) assertInBounds() {
	if c < 100 || c > 599 {
		panic(ErrOutOfBoundsStatusCode)
	}
}
