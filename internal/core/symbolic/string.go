package symbolic

import (
	"errors"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []GoString{
		(*String)(nil), (*Identifier)(nil), (*Path)(nil), (*PathPattern)(nil), (*Host)(nil),
		(*HostPattern)(nil), (*URLPattern)(nil),
	}

	_ = []StringLike{
		(*String)(nil), (*StringConcatenation)(nil), (*AnyStringLike)(nil),
		(*strLikeMultivalue)(nil), (*CheckedString)(nil),
	}

	ANY_STRING         = &String{}
	ANY_CHECKED_STRING = &CheckedString{}
	ANY_STR_LIKE       = &AnyStringLike{}
	ANY_STR_CONCAT     = &StringConcatenation{}
	ANY_RUNE           = &Rune{}
	ANY_RUNE_SLICE     = &RuneSlice{}

	EMPTY_STRING = NewString("")

	_ANY_STR_TYPE_PATTERN = &TypePattern{val: ANY_STRING}

	STRING_LIKE_PSEUDOPROPS  = []string{"byte-count", "rune-count", "replace", "trim_space", "has_prefix", "has_suffix"}
	RUNE_SLICE_PROPNAMES     = []string{"insert", "remove_position", "remove_position_range"}
	CHECKED_STRING_PROPNAMES = append([]string{"pattern-name", "pattern"}, STRING_LIKE_PSEUDOPROPS...)
	RUNE_PROPNAMES           = []string{"is-space", "is-printable", "is-letter"}

	RUNE_SLICE__INSERT_PARAMS      = &[]Value{NewMultivalue(ANY_RUNE, NewAnySequenceOf(ANY_RUNE))}
	RUNE_SLICE__INSERT_PARAM_NAMES = []string{"rune", "index"}
)

// A GoString represents a symbolic GoString.
type GoString interface {
	Value
	underlyingString() *String
}

// A StringLike represents a symbolic StringLike.
type StringLike interface {
	Serializable
	Sequence
	IProps
	GetOrBuildString() *String
}

// A String represents a symbolic Str.
type String struct {
	hasValue bool
	value    string

	// should not be set if value or pattern are set
	minLengthPlusOne int64
	// should not be set if value or pattern are set
	maxLength int64

	pattern StringPattern

	UnassignablePropsMixin
	SerializableMixin
	ComparableMixin
}

func NewString(v string) *String {
	return &String{
		hasValue: true,
		value:    v,
	}
}

func NewStringWithLengthRange(minLength, maxLength int64) *String {
	if minLength > maxLength {
		panic(errors.New("minLength should be <= maxLength"))
	}

	if minLength < 0 || maxLength < 0 {
		panic(errors.New("minLength and maxLength should be less or equal to zero"))
	}

	return &String{
		minLengthPlusOne: minLength + 1,
		maxLength:        maxLength,
	}
}

func NewStringMatchingPattern(p StringPattern) *String {
	return &String{
		pattern: p,
	}
}

func (s *String) minLength() int64 {
	if s.minLengthPlusOne <= 0 {
		panic(errors.New("minimum length is not set"))
	}
	return s.minLengthPlusOne - 1
}

func (s *String) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherString, ok := v.(*String)
	if !ok {
		return false
	}
	if s.pattern != nil {
		if otherString.pattern != nil {
			return otherString.pattern.Test(s.pattern, state) && s.pattern.Test(otherString.pattern, state)
		}
		return otherString.hasValue && s.pattern.TestValue(otherString, state)
	}
	if !s.hasValue {
		if s.minLengthPlusOne <= 0 {
			//s is the any string
			return true
		}
		//else s has a min & max length

		if otherString.hasValue {
			return int64(len(otherString.value)) >= s.minLength() && int64(len(otherString.value)) <= s.maxLength
		} else if otherString.minLengthPlusOne > 0 {
			return otherString.minLength() >= s.minLength() && otherString.maxLength <= s.maxLength
		} //else otherString has a pattern, we can't know the length in most cases.

		if lengthCheckingPattern, ok := otherString.pattern.(*LengthCheckingStringPattern); ok {
			return s.Test(lengthCheckingPattern.SymbolicValue(), state)
		}
		return false
	}
	//if s has a value
	return otherString.hasValue && s.value == otherString.value
}

func (s *String) IsConcretizable() bool {
	return s.hasValue
}

func (s *String) Concretize(ctx ConcreteContext) any {
	if !s.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateString(s.value)
}

func (s *String) HasValue() bool {
	return s.IsConcretizable()
}

func (s *String) Value() string {
	if !s.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return s.value
}

func (s *String) Static() Pattern {
	return _ANY_STR_TYPE_PATTERN
}

func (s *String) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if s.hasValue {
		jsonString := utils.Must(utils.MarshalJsonNoHTMLEspace(s.value))
		w.WriteBytes(jsonString)
	} else {
		w.WriteName("string")

		if s.pattern != nil {

			if seqPattern, ok := s.pattern.(*SequenceStringPattern); ok {
				w.WriteString("(")
				w.WriteString(seqPattern.stringifiedNode)
				w.WriteString(")")
			} else {
				w.WriteString("(matching ")
				s.pattern.PrettyPrint(w.ZeroIndent(), config)
				w.WriteString(")")
			}
		} else if s.minLengthPlusOne > 0 {
			w.WriteStringF("(length in %d..%d)", s.minLength(), s.maxLength)
		}
	}
}

func (s *String) HasKnownLen() bool {
	return false
}

func (s *String) KnownLen() int {
	return -1
}

func (s *String) Element() Value {
	return ANY_BYTE
}

func (*String) ElementAt(i int) Value {
	return ANY_BYTE
}

func (s *String) IteratorElementKey() Value {
	return ANY_INT
}

func (s *String) IteratorElementValue() Value {
	return ANY_BYTE
}

func (s *String) underlyingString() *String {
	return ANY_STRING
}

func (s *String) GetOrBuildString() *String {
	return s
}

func (f *String) WidestOfType() Value {
	return ANY_STRING
}

func (s *String) Reader() *Reader {
	return ANY_READER
}

func (p *String) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (s *String) Prop(name string) Value {
	fn, ok := getStringLikePseudoProp(name)
	if ok {
		return fn
	}
	panic(FormatErrPropertyDoesNotExist(name, s))
}

func (s *String) slice(start, end *Int) Sequence {
	return ANY_STRING
}

// A Rune represents a symbolic Rune.
type Rune struct {
	UnassignablePropsMixin
	SerializableMixin
	ComparableMixin

	hasValue bool
	value    rune
}

func NewRune(r rune) *Rune {
	return &Rune{
		hasValue: true,
		value:    r,
	}
}

func (r *Rune) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherRune, ok := v.(*Rune)
	if !ok {
		return false
	}
	if !r.hasValue {
		return true
	}
	return otherRune.hasValue && r.value == otherRune.value
}

func (r *Rune) IsConcretizable() bool {
	return r.hasValue
}

func (r *Rune) Concretize(ctx ConcreteContext) any {
	if !r.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateRune(r.value)
}

func (r *Rune) Static() Pattern {
	return &TypePattern{val: ANY_RUNE}
}

func (r *Rune) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if r.hasValue {
		w.WriteString(commonfmt.FmtRune(r.value))
		return
	}
	w.WriteName("rune")
}

func (r *Rune) WidestOfType() Value {
	return ANY_RUNE
}

func (r *Rune) PropertyNames() []string {
	return RUNE_PROPNAMES
}

func (r *Rune) Prop(name string) Value {
	switch name {
	case "is-space":
		return ANY_BOOL
	case "is-printable":
		return ANY_BOOL
	case "is-letter":
		return ANY_BOOL
	default:
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
}

// A CheckedString represents a symbolic CheckedString.
type CheckedString struct {
	SerializableMixin
	UnassignablePropsMixin
}

func (s *CheckedString) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*CheckedString)
	return ok
}

func (s *CheckedString) IteratorElementKey() Value {
	return ANY_STRING.IteratorElementKey()
}

func (s *CheckedString) IteratorElementValue() Value {
	return ANY_STRING.IteratorElementKey()
}

func (s *CheckedString) HasKnownLen() bool {
	return false
}

func (s *CheckedString) KnownLen() int {
	return -1
}

func (s *CheckedString) Element() Value {
	return ANY_STRING.Element()
}

func (s *CheckedString) ElementAt(i int) Value {
	return ANY_STRING.ElementAt(i)
}

func (s *CheckedString) slice(start, end *Int) Sequence {
	return ANY_STRING.slice(start, end)
}

func (s *CheckedString) GetOrBuildString() *String {
	return ANY_STRING
}

func (s *CheckedString) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("checked-string")
}

func (p *CheckedString) PropertyNames() []string {
	return CHECKED_STRING_PROPNAMES
}

func (s *CheckedString) Prop(name string) Value {
	switch name {
	case "pattern-name":
		return ANY_STRING
	case "pattern":
		return ANY_STR_PATTERN
	default:
		fn, ok := getStringLikePseudoProp(name)
		if ok {
			return fn
		}

		panic(FormatErrPropertyDoesNotExist(name, s))
	}
}

func (s *CheckedString) WidestOfType() Value {
	return ANY_CHECKED_STRING
}

type RuneSlice struct {
	SerializableMixin
	ClonableSerializableMixin
}

func NewRuneSlice() *RuneSlice {
	return &RuneSlice{}
}

func (s *RuneSlice) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*RuneSlice)
	return ok
}

func (s *RuneSlice) IsConcretizable() bool {
	return false
}

func (s *RuneSlice) Concretize(ctx ConcreteContext) any {
	panic(ErrNotConcretizable)
}

func (s *RuneSlice) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("rune-slice")
}

func (s *RuneSlice) HasKnownLen() bool {
	return false
}

func (s *RuneSlice) KnownLen() int {
	return -1
}

func (s *RuneSlice) Element() Value {
	return ANY_RUNE
}

func (*RuneSlice) ElementAt(i int) Value {
	return ANY_RUNE
}

func (s *RuneSlice) IteratorElementKey() Value {
	return ANY_INT
}

func (s *RuneSlice) IteratorElementValue() Value {
	return ANY_RUNE
}

func (b *RuneSlice) WidestOfType() Value {
	return ANY_RUNE_SLICE
}

func (s *RuneSlice) slice(start, end *Int) Sequence {
	return &RuneSlice{}
}

func (s *RuneSlice) set(ctx *Context, i *Int, v Value) {

}
func (s *RuneSlice) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (s *RuneSlice) insertElement(ctx *Context, v Value, i *Int) {
}

func (s *RuneSlice) Insert(ctx *Context, v Value, i *Int) {
	ctx.SetSymbolicGoFunctionParameters(RUNE_SLICE__INSERT_PARAMS, RUNE_SLICE__INSERT_PARAM_NAMES)
}

func (s *RuneSlice) removePosition(ctx *Context, i *Int) {

}

func (s *RuneSlice) removePositions(r *IntRange) {

}

func (s *RuneSlice) insertSequence(ctx *Context, seq Sequence, i *Int) {
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}
	if _, ok := MergeValuesWithSameStaticTypeInMultivalue(seq.Element()).(*Rune); !ok {
		ctx.AddSymbolicGoFunctionError(fmtHasElementsOfType(s, ANY_RUNE))
	}
}

func (s *RuneSlice) appendSequence(ctx *Context, seq Sequence) {
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}
	if _, ok := MergeValuesWithSameStaticTypeInMultivalue(seq.Element()).(*Rune); !ok {
		ctx.AddSymbolicGoFunctionError(fmtHasElementsOfType(s, ANY_RUNE))
	}
}

func (s *RuneSlice) TakeInMemorySnapshot() (*Snapshot, error) {
	return ANY_SNAPSHOT, nil
}

func (s *RuneSlice) PropertyNames() []string {
	return RUNE_SLICE_PROPNAMES
}

func (s *RuneSlice) Prop(name string) Value {
	switch name {
	case "insert":
		return WrapGoMethod(s.Insert)
	case "remove_position":
		return WrapGoMethod(s.removePosition)
	case "remove_position_range":
		return WrapGoMethod(s.removePositions)
	default:
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
}

func (s *RuneSlice) SetProp(state *State, node parse.Node, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (s *RuneSlice) WithExistingPropReplaced(state *State, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (s *RuneSlice) WatcherElement() Value {
	return ANY
}

// A StringConcatenation represents a symbolic StringConcatenation.
type StringConcatenation struct {
	UnassignablePropsMixin
	SerializableMixin
}

func (c *StringConcatenation) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*StringConcatenation)
	return ok
}

func (c *StringConcatenation) IsConcretizable() bool {
	return false
}

func (c *StringConcatenation) Concretize(ctx ConcreteContext) any {
	panic(ErrNotConcretizable)
}

func (c *StringConcatenation) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("string-concatenation")
}

func (c *StringConcatenation) IteratorElementKey() Value {
	return ANY_STRING.IteratorElementKey()
}

func (c *StringConcatenation) IteratorElementValue() Value {
	return ANY_STRING.IteratorElementKey()
}

func (c *StringConcatenation) HasKnownLen() bool {
	return false
}

func (c *StringConcatenation) KnownLen() int {
	return -1
}

func (c *StringConcatenation) Element() Value {
	return ANY_STRING.Element()
}

func (c *StringConcatenation) ElementAt(i int) Value {
	return ANY_STRING.ElementAt(i)
}

func (c *StringConcatenation) slice(start, end *Int) Sequence {
	return ANY_STRING.slice(start, end)
}

func (c *StringConcatenation) GetOrBuildString() *String {
	return ANY_STRING
}

func (c *StringConcatenation) WidestOfType() Value {
	return ANY_STR_CONCAT
}

func (c *StringConcatenation) Reader() *Reader {
	return ANY_READER
}

func (c *StringConcatenation) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (c *StringConcatenation) Prop(name string) Value {
	fn, ok := getStringLikePseudoProp(name)
	if ok {
		return fn
	}
	panic(FormatErrPropertyDoesNotExist(name, c))
}

func isAnyStringLike(v Value) bool {
	_, ok := v.(*AnyStringLike)
	return ok
}

// A AnyStringLike represents a symbolic StringLike we don't know the concret type.
type AnyStringLike struct {
	UnassignablePropsMixin
	Serializable
}

func (s *AnyStringLike) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(StringLike)
	return ok
}

func (s *AnyStringLike) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("string-like")
}

func (s *AnyStringLike) Element() Value {
	return ANY_BYTE
}

func (s *AnyStringLike) ElementAt(i int) Value {
	return ANY_BYTE
}

func (s *AnyStringLike) IteratorElementKey() Value {
	return ANY_INT
}

func (s *AnyStringLike) IteratorElementValue() Value {
	return ANY_BYTE
}

func (s *AnyStringLike) slice(start, end *Int) Sequence {
	return ANY_STR_LIKE
}

func (s *AnyStringLike) KnownLen() int {
	return -1
}
func (s *AnyStringLike) HasKnownLen() bool {
	return false
}

func (s *AnyStringLike) GetOrBuildString() *String {
	return ANY_STRING
}

func (s *AnyStringLike) WidestOfType() Value {
	return ANY_STRING
}

func (s *AnyStringLike) Reader() *Reader {
	return ANY_READER
}

func (p *AnyStringLike) PropertyNames() []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (s *AnyStringLike) Prop(name string) Value {
	fn, ok := getStringLikePseudoProp(name)
	if ok {
		return fn
	}
	panic(FormatErrPropertyDoesNotExist(name, s))
}

func getStringLikePseudoProp(name string) (Value, bool) {
	switch name {
	case "byte-count":
		return ANY_BYTECOUNT, true
	case "rune-count":
		return ANY_RUNECOUNT, true
	case "replace":
		return WrapGoFunction(func(ctx *Context, old, new StringLike) *String {
			return ANY_STRING
		}), true
	case "trim_space":
		return WrapGoFunction(func(ctx *Context) StringLike {
			return ANY_STR_LIKE
		}), true
	case "has_prefix":
		return WrapGoFunction(func(ctx *Context, s StringLike) *Bool {
			return ANY_BOOL
		}), true
	case "has_suffix":
		return WrapGoFunction(func(ctx *Context, s StringLike) *Bool {
			return ANY_BOOL
		}), true
	}

	return nil, false
}
