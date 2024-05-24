package core

import (
	"errors"

	"github.com/inoxlang/inox/internal/parse"
)

var (
	ErrEmptyPropertyName             = errors.New("empty property name")
	ErrUnexpectedCharsInPropertyName = errors.New("unexpected char(s) in property name")
	ErrEmptyLongValuePath            = errors.New("empty long value-path")
	ErrSingleSegmentLongValuePath    = errors.New("single-segment value-path")

	_ = []ValuePath{PropertyName("x"), &LongValuePath{}}
	_ = []ValuePathSegment{PropertyName("x")}
)

// A ValuePath represents a path to a value in a structure.
type ValuePath interface {
	Serializable
	GetFrom(ctx *Context, v Value) Value
}

// A ValuePathSegment represents a segment of path to a value in a structure.
type ValuePathSegment interface {
	Serializable
	SegmentGetFrom(ctx *Context, v Value) Value
}

// Property name literals (e.g. `.age`) evaluate to a PropertyName.
// PropertyName implements Value and ValuePath.
type PropertyName string

func (n PropertyName) UnderlyingString() string {
	return string(n)
}

func (n PropertyName) Validate() error {
	if n == "" {
		return ErrEmptyPropertyName
	}

	for _, r := range n {
		if !parse.IsIdentChar(r) {
			return ErrUnexpectedCharsInPropertyName
		}
	}
	return nil
}

func (n PropertyName) GetFrom(ctx *Context, v Value) Value {
	return v.(IProps).Prop(ctx, string(n))
}

func (n PropertyName) SegmentGetFrom(ctx *Context, v Value) Value {
	return n.GetFrom(ctx, v)
}

// A LongValuePath represents a path (>= 2 segments) to a value in a structure, LongValuePath implements Value.
type LongValuePath []ValuePathSegment

func NewLongValuePath(segments []ValuePathSegment) *LongValuePath {
	if len(segments) < 2 {
		panic(errors.New("at least 2 segments should be provided"))
	}
	p := LongValuePath(segments)
	return &p
}

func (p *LongValuePath) Validate() error {
	switch len(*p) {
	case 0:
		return ErrEmptyLongValuePath
	case 1:
		return ErrSingleSegmentLongValuePath
	}
	return nil
}

func (p *LongValuePath) GetFrom(ctx *Context, v Value) Value {
	for _, segment := range *p {
		v = segment.SegmentGetFrom(ctx, v)
	}
	return v
}
