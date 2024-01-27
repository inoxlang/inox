package http_ns

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
)

var (
	ErrOutOfBoundsStatusCode = errors.New("out of bounds status code")
	_                        = []core.Serializable{Status{}, StatusCode(100)}
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

func MakeStatus(code StatusCode) (Status, error) {
	if !code.inBounds() {
		return Status{}, ErrOutOfBoundsStatusCode
	}
	return Status{code: code, reasonPhrase: http.StatusText(int(code))}, nil
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
		return core.String(s.FullText())
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

func (Status) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig, depth int) error {
	return core.ErrNotImplementedYet
}

func (s Status) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	w.WriteString(s.FullText())
	return nil
}

type StatusCode uint16

func MakeStatusCode(_ *core.Context, n core.Int) (StatusCode, error) {
	if n < 0 || n > math.MaxUint16 || !StatusCode(int(n)).inBounds() {
		return 0, fmt.Errorf("%w: %d", ErrOutOfBoundsStatusCode, n)
	}
	return StatusCode(int(n)), nil
}

func (c StatusCode) assertInBounds() {
	if !c.inBounds() {
		panic(ErrOutOfBoundsStatusCode)
	}
}

func (c StatusCode) inBounds() bool {
	return c >= 100 && c <= 599
}

func (StatusCode) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig, depth int) error {
	return core.ErrNotImplementedYet
}

func (c StatusCode) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	w.WriteUint(uint(c))
	return nil
}
