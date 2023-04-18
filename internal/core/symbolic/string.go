package internal

import (
	"bufio"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []WrappedString{
		&String{}, &Identifier{}, &Path{}, &PathPattern{}, &Host{},
		&HostPattern{}, &URLPattern{}, &CheckedString{},
	}

	_ = []StringLike{
		&String{}, &StringConcatenation{},
	}

	ANY_STR      = &String{}
	ANY_STR_LIKE = &AnyStringLike{}

	STRING_LIKE_PSEUDOPROPS = []string{"replace", "trim_space", "has_prefix", "has_suffix"}
	RUNE_SLICE_PROPNAMES    = []string{"insert", "remove_position", "remove_position_range"}
)

// An WrappedString represents a symbolic WrappedString.
type WrappedString interface {
	underylingString() *String
}

// A StringLike represents a symbolic StringLike.
type StringLike interface {
	SymbolicValue
	PseudoPropsValue
	GetOrBuildString() *String
}

// A String represents a symbolic Str.
type String struct {
	UnassignablePropsMixin
	_ int
}

func (s *String) Test(v SymbolicValue) bool {
	_, ok := v.(*String)
	return ok
}

func (s *String) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *String) IsWidenable() bool {
	return false
}

func (s *String) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%string")))
	return
}

func (s *String) HasKnownLen() bool {
	return false
}

func (s *String) knownLen() int {
	return -1
}

func (s *String) element() SymbolicValue {
	return &Byte{}
}

func (*String) elementAt(i int) SymbolicValue {
	return &Byte{}
}

func (s *String) underylingString() *String {
	return &String{}
}

func (s *String) GetOrBuildString() *String {
	return &String{}
}

func (f *String) WidestOfType() SymbolicValue {
	return &String{}
}

func (s *String) Reader() *Reader {
	return &Reader{}
}

func (p *String) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (*String) Prop(name string) SymbolicValue {
	switch name {
	case "replace":
		return &GoFunction{
			fn: func(ctx *Context, old, new *AnyStringLike) *AnyStringLike {
				return &AnyStringLike{}
			},
		}
	case "trim_space":
		return &GoFunction{
			fn: func(ctx *Context) *AnyStringLike {
				return &AnyStringLike{}
			},
		}
	case "has_prefix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return &Bool{}
			},
		}
	case "has_suffix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return &Bool{}
			},
		}
	default:
		return nil
	}
}

func (s *String) slice(start, end *Int) Sequence {
	return &String{}
}

// A Rune represents a symbolic Rune.
type Rune struct {
	UnassignablePropsMixin
	_ int
}

func (r *Rune) Test(v SymbolicValue) bool {
	_, ok := v.(*Rune)

	return ok
}

func (r *Rune) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Rune) IsWidenable() bool {
	return false
}

func (r *Rune) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%rune")))
	return
}

func (r *Rune) WidestOfType() SymbolicValue {
	return &Rune{}
}

func (r *Rune) PropertyNames() []string {
	return []string{"is_space", "is_printable", "is_letter"}
}

func (*Rune) Prop(name string) SymbolicValue {
	switch name {
	case "is_space":
		return &Bool{}
	case "is_printable":
		return &Bool{}
	case "is_letter":
		return &Bool{}
	default:
		return nil
	}
}

// A CheckedString represents a symbolic CheckedString.
type CheckedString struct {
	_ int
}

func (s *CheckedString) Test(v SymbolicValue) bool {
	_, ok := v.(*CheckedString)
	return ok
}

func (s *CheckedString) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *CheckedString) IsWidenable() bool {
	return false
}

func (s *CheckedString) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%checked-string")))
	return
}

func (p *CheckedString) PropertyNames() []string {
	return []string{"pattern_name", "pattern"}
}

func (*CheckedString) Prop(name string) SymbolicValue {
	switch name {
	case "pattern_name":
		return &String{}
	case "pattern":
		return &AnyPattern{}
	default:
		return nil
	}
}

func (s *CheckedString) underylingString() *String {
	return &String{}
}

func (s *CheckedString) WidestOfType() SymbolicValue {
	return &CheckedString{}
}

type RuneSlice struct {
	_ int
}

func (s *RuneSlice) Test(v SymbolicValue) bool {
	_, ok := v.(*RuneSlice)
	return ok
}

func (s *RuneSlice) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *RuneSlice) IsWidenable() bool {
	return false
}

func (s *RuneSlice) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%rune-slice")))
	return
}

func (s *RuneSlice) HasKnownLen() bool {
	return false
}

func (s *RuneSlice) knownLen() int {
	return -1
}

func (s *RuneSlice) element() SymbolicValue {
	return &Rune{}
}

func (*RuneSlice) elementAt(i int) SymbolicValue {
	return &Rune{}
}

func (b *RuneSlice) WidestOfType() SymbolicValue {
	return &RuneSlice{}
}

func (s *RuneSlice) slice(start, end *Int) Sequence {
	return &RuneSlice{}
}

func (s *RuneSlice) set(i *Int, v SymbolicValue) {

}
func (s *RuneSlice) setSlice(start, end *Int, v SymbolicValue) {

}

func (s *RuneSlice) insertElement(v SymbolicValue, i *Int) *Error {
	return nil
}

func (s *RuneSlice) removePosition(i *Int) *Error {
	return nil
}

func (s *RuneSlice) removePositions(r *IntRange) *Error {
	return nil
}

func (s *RuneSlice) insertSequence(seq Sequence, i *Int) *Error {
	return nil
}

func (s *RuneSlice) appendSequence(seq Sequence) *Error {
	return nil
}

func (s *RuneSlice) TakeInMemorySnapshot() (*Snapshot, error) {
	return ANY_SNAPSHOT, nil
}

func (s *RuneSlice) PropertyNames() []string {
	return RUNE_SLICE_PROPNAMES
}

func (s *RuneSlice) Prop(name string) SymbolicValue {
	switch name {
	case "insert":
		return WrapGoMethod(s.insertElement)
	case "remove_position":
		return WrapGoMethod(s.removePosition)
	case "remove_position_range":
		return WrapGoMethod(s.removePositions)
	default:
		return nil
	}
}

func (s *RuneSlice) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (s *RuneSlice) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (s *RuneSlice) WatcherElement() SymbolicValue {
	return ANY
}

// A StringConcatenation represents a symbolic StringConcatenation.
type StringConcatenation struct {
	UnassignablePropsMixin
	_ int
}

func (c *StringConcatenation) Test(v SymbolicValue) bool {
	_, ok := v.(*String)
	return ok
}

func (c *StringConcatenation) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (c *StringConcatenation) IsWidenable() bool {
	return false
}

func (c *StringConcatenation) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%string-concatenation")))
	return
}

func (c *StringConcatenation) HasKnownLen() bool {
	return false
}

func (c *StringConcatenation) knownLen() int {
	return -1
}

func (c *StringConcatenation) element() SymbolicValue {
	return &Rune{}
}

func (c *StringConcatenation) GetOrBuildString() *String {
	return &String{}
}

func (c *StringConcatenation) WidestOfType() SymbolicValue {
	return &String{}
}

func (c *StringConcatenation) Reader() *Reader {
	return &Reader{}
}

func (p *StringConcatenation) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (*StringConcatenation) Prop(name string) SymbolicValue {
	switch name {
	case "replace":
		return &GoFunction{
			fn: func(ctx *Context, old, new *AnyStringLike) *String {
				return &String{}
			},
		}
	case "trim_space":
		return &GoFunction{
			fn: func(ctx *Context) *AnyStringLike {
				return &AnyStringLike{}
			},
		}
	case "has_prefix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return &Bool{}
			},
		}
	case "has_suffix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return &Bool{}
			},
		}
	default:
		return nil
	}
}

func isAnyStringLike(v SymbolicValue) bool {
	_, ok := v.(*AnyStringLike)
	return ok
}

// A AnyStringLike represents a symbolic StringLike we don't know the concret type.
type AnyStringLike struct {
	UnassignablePropsMixin
	_ int
}

func (s *AnyStringLike) Test(v SymbolicValue) bool {
	_, ok := v.(StringLike)
	return ok
}

func (s *AnyStringLike) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *AnyStringLike) IsWidenable() bool {
	return false
}

func (s *AnyStringLike) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%string-like")))
	return
}

func (s *AnyStringLike) element() SymbolicValue {
	return &Byte{}
}

func (s *AnyStringLike) elementAt(i int) SymbolicValue {
	return &Byte{}
}

func (s *AnyStringLike) knownLen() int {
	return -1
}
func (s *AnyStringLike) HasKnownLen() bool {
	return false
}

func (s *AnyStringLike) GetOrBuildString() *String {
	return &String{}
}

func (s *AnyStringLike) WidestOfType() SymbolicValue {
	return &String{}
}

func (s *AnyStringLike) Reader() *Reader {
	return &Reader{}
}

func (p *AnyStringLike) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (*AnyStringLike) Prop(name string) SymbolicValue {
	switch name {
	case "replace":
		return &GoFunction{
			fn: func(ctx *Context, old, new *AnyStringLike) *AnyStringLike {
				return ANY_STR_LIKE
			},
		}
	case "trim_space":
		return &GoFunction{
			fn: func(ctx *Context) *AnyStringLike {
				return ANY_STR_LIKE
			},
		}
	case "has_prefix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return &Bool{}
			},
		}
	case "has_suffix":
		return &GoFunction{
			fn: func(ctx *Context, s *AnyStringLike) *Bool {
				return &Bool{}
			},
		}
	default:
		return nil
	}
}
