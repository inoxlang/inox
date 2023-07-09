package core

import (
	"bytes"
	"errors"
	"io"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DATE_FORMAT_PATTERN_NAMESPACE = "date-format"
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
	*ParserBasedPseudoPattern
	layout string

	NamespaceMemberPatternReprMixin
}

func NewDateFormat(layout, namespaceMemberName string) *DateFormat {
	fmt := &DateFormat{
		layout:                   layout,
		ParserBasedPseudoPattern: NewParserBasePattern(&dateLayoutParser{layout: layout}),
		NamespaceMemberPatternReprMixin: NamespaceMemberPatternReprMixin{
			NamespaceName: DATE_FORMAT_PATTERN_NAMESPACE,
			MemberName:    namespaceMemberName,
		},
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

func (p *dateLayoutParser) Parse(ctx *Context, s string) (Serializable, error) {
	t, err := time.Parse(p.layout, s)
	if err != nil {
		return nil, err
	}
	return Date(t), nil
}
