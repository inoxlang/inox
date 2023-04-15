package internal

import (
	"bytes"
	"errors"
	"io"
	"time"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []Format{(*DateFormat)(nil)}
	_ = []StringFormat{(*DateFormat)(nil)}

	ErrInvalidFormattingArgument = errors.New("invalid formatting argument")
)

func init() {
	RegisterSymbolicGoFunction(Fmt, func(ctx *symbolic.Context, format symbolic.Format, v symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
		return symbolic.ANY, nil
	})
}

// A Format represents a string or binary format.
type Format interface {
	Pattern
	Format(ctx *Context, arg Value, w io.Writer) (int, error)
}

func Fmt(ctx *Context, format Format, arg Value) (Value, error) {
	buf := bytes.NewBuffer(nil)
	if _, err := format.Format(ctx, arg, buf); err != nil {
		return nil, err
	}
	if _, ok := format.(StringFormat); ok {
		return Str(buf.String()), nil
	}
	panic(errors.New("only string formats are supported for now"))
}

type StringFormat interface {
	Format
	StringPattern
}

// TODO:
// type BinaryFormat interface {
// 	Format
// }

type DateFormat struct {
	*ParserBasedPattern
	layout string
}

func NewDateFormat(layout string) *DateFormat {
	fmt := &DateFormat{
		layout:             layout,
		ParserBasedPattern: NewParserBasePattern(&dateLayoutParser{layout: layout}),
	}

	return fmt
}

func (f *DateFormat) Format(ctx *Context, v Value, w io.Writer) (int, error) {
	t, ok := v.(Date)
	if !ok {
		return -1, ErrInvalidFormattingArgument
	}

	return w.Write(utils.StringAsBytes(time.Time(t).Format(f.layout)))
}

type dateLayoutParser struct {
	layout string
}

func (p *dateLayoutParser) Validate(ctx *Context, s string) bool {
	_, err := time.Parse(p.layout, s)
	return err == nil
}

func (p *dateLayoutParser) Parse(ctx *Context, s string) (Value, error) {
	t, err := time.Parse(p.layout, s)
	if err != nil {
		return nil, err
	}
	return Date(t), nil
}
