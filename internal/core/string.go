package core

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MIN_LAZY_STR_CONCATENATION_SIZE                 = 200 //this constant can change in the future, it's a starting point.
	MAX_SMALL_STRING_SIZE_IN_LAZY_STR_CONCATENATION = 30  //this constant can change in the future, it's a starting point.
)

var (
	RUNE_SLICE_PROPNAMES = []string{"insert", "remove_position", "remove_position_range"}

	_ = []GoString{
		String(""), Path(""), PathPattern(""), Host(""), HostPattern(""), EmailAddress(""), Identifier(""),
		URL(""), URLPattern(""),
	}
	_ = []StringLike{String(""), (*StringConcatenation)(nil), (*CheckedString)(nil)}
)

// A GoString represents a Go string value.
type GoString interface {
	Serializable

	//UnderlyingString() should instantly retrieves the wrapped string
	UnderlyingString() string
}

// A StringLike represents an abstract immutable string, it should behave exactly like a regular Str and have the same pseudo properties.
// A StringLike should never perform internal mutation if IsMutable or GetOrBuildString is called.
type StringLike interface {
	Serializable
	Sequence
	IProps
	GetOrBuildString() string
	ByteLen() int
	RuneCount() int

	Replace(ctx *Context, old, new StringLike) StringLike
	TrimSpace(ctx *Context) StringLike
	HasPrefix(ctx *Context, prefix StringLike) Bool
	HasSuffix(ctx *Context, suffix StringLike) Bool

	//TODO: EqualStringLike(ctx *Context, s StringLike)
}

// Inox string type, String implements Value.
type String string

func (s String) UnderlyingString() string {
	return string(s)
}

func (s String) GetOrBuildString() string {
	return string(s)
}

func (s String) ByteLike() []byte {
	return []byte(s)
}

func (s String) Len() int {
	return len(s)
}

func (s String) At(ctx *Context, i int) Value {
	return Byte(s[i])
}

func (s String) slice(start, end int) Sequence {
	return s[start:end]
}

func (s String) PropertyNames(ctx *Context) []string {
	return symbolic.STRING_LIKE_PSEUDOPROPS
}

func (s String) Prop(ctx *Context, name string) Value {
	res, _ := getStringLikePseudoProp(name, s)
	return res
}

func (String) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (s String) ByteLen() int {
	return len(s)
}

func (s String) RuneCount() int {
	return utf8.RuneCountInString(string(s))
}

func (s String) Replace(ctx *Context, old, new StringLike) StringLike {
	//TODO: make the function stoppable for large strings

	return String(strings.Replace(string(s), old.GetOrBuildString(), new.GetOrBuildString(), -1))
}

func (s String) TrimSpace(ctx *Context) StringLike {
	//TODO: make the function stoppable for large strings

	return String(strings.TrimSpace(string(s)))
}

func (s String) HasPrefix(ctx *Context, prefix StringLike) Bool {
	//TODO: make the function stoppable for large strings

	return Bool(strings.HasPrefix(string(s), prefix.GetOrBuildString()))
}

func (s String) HasSuffix(ctx *Context, prefix StringLike) Bool {
	//TODO: make the function stoppable for large strings

	return Bool(strings.HasSuffix(string(s), prefix.GetOrBuildString()))
}

type Rune rune

func (r Rune) PropertyNames(ctx *Context) []string {
	return symbolic.RUNE_PROPNAMES
}

func (r Rune) Prop(ctx *Context, name string) Value {
	switch name {
	case "is-space":
		return Bool(unicode.IsSpace(rune(r)))
	case "is-printable":
		return Bool(unicode.IsPrint(rune(r)))
	case "is-letter":
		return Bool(unicode.IsLetter(rune(r)))
	default:
		return nil
	}
}

func (Rune) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

// TODO: implement Iterable
type RuneRange struct {
	Start rune
	End   rune
}

func (r RuneRange) Includes(ctx *Context, i Rune) bool {
	return r.Start <= rune(i) && rune(i) <= r.End
}

func (r RuneRange) At(ctx *Context, i int) Value {
	if i >= r.Len() {
		panic(ErrIndexOutOfRange)
	}
	return Rune(rune(i) + r.Start)
}

func (r RuneRange) Len() int {
	length := r.End - r.Start + 1
	return int(length)
}

func (r RuneRange) RandomRune() rune {
	offset := rand.Intn(int(r.End - r.Start + 1))
	return r.Start + rune(offset)
}

func (r RuneRange) Random(ctx *Context) interface{} {
	return r.RandomRune()
}

type CheckedString struct {
	str                 String
	matchingPatternName string //if the matching pattern is in the namespace the name will contain a dot '.'
	matchingPattern     Pattern
}

func NewStringFromSlices(slices []Value, node *parse.StringTemplateLiteral, ctx *Context) (String, error) {
	buff := bytes.NewBufferString("")

	for i, sliceValue := range slices {
		switch node.Slices[i].(type) {
		case *parse.StringTemplateSlice:
			buff.WriteString(string(sliceValue.(String)))
		case *parse.StringTemplateInterpolation:
			switch val := sliceValue.(type) {
			case StringLike:
			case Int:
				sliceValue = String(Stringify(val, ctx))
			default:
				panic(ErrUnreachable)
			}

			str := sliceValue.(StringLike).GetOrBuildString()
			buff.WriteString(string(str))
		default:
			return "", fmt.Errorf("runtime check error: slice value of type %T", sliceValue)
		}
	}

	str := String(buff.String())
	return str, nil
}

// NewCheckedString creates a CheckedString in a secure way.
func NewCheckedString(slices []Value, node *parse.StringTemplateLiteral, ctx *Context) (*CheckedString, error) {
	patternIdent, isPatternAnIdent := node.Pattern.(*parse.PatternIdentifierLiteral)

	var (
		namespaceName       string
		namespace           *PatternNamespace
		finalPattern        Pattern
		matchingPatternName string
	)
	if isPatternAnIdent {
		if node.HasInterpolations() {
			return nil, errors.New("string template literals with interpolations should be preceded by a pattern which name has a prefix")
		}
		finalPattern = ctx.ResolveNamedPattern(patternIdent.Name)
		if finalPattern == nil {
			return nil, fmt.Errorf("pattern %%%s does not exist", patternIdent.Name)
		}
		matchingPatternName = patternIdent.Name
	} else {
		namespaceMembExpr := node.Pattern.(*parse.PatternNamespaceMemberExpression)
		namespaceName = namespaceMembExpr.Namespace.Name
		namespace = ctx.ResolvePatternNamespace(namespaceName)

		if namespace == nil {
			return nil, fmt.Errorf("cannot interpolate: pattern namespace '%s' does not exist", namespaceName)
		}

		memberName := namespaceMembExpr.MemberName.Name
		pattern, ok := namespace.Patterns[memberName]
		if !ok {
			return nil, fmt.Errorf("cannot interpolate: member .%s of pattern namespace '%s' does not exist", memberName, namespaceName)
		}

		matchingPatternName = namespaceName + "." + memberName
		finalPattern = pattern
	}

	buff := bytes.NewBufferString("")

	for i, sliceValue := range slices {
		switch s := node.Slices[i].(type) {
		case *parse.StringTemplateSlice:
			buff.WriteString(s.Raw)
		case *parse.StringTemplateInterpolation:
			memberName := s.Type
			var shouldConvert bool

			if strings.Contains(memberName, ".") {
				name, conversion, _ := strings.Cut(memberName, ".")
				memberName = name
				if conversion != "from" {
					return nil, fmt.Errorf("pattern namespace member should be followed by .from not .%s", conversion)
				}
				shouldConvert = true
			}

			pattern, ok := namespace.Patterns[memberName]
			if !ok {
				return nil, fmt.Errorf("cannot interpolate: member .%s of pattern namespace '%s' does not exist", memberName, namespaceName)
			}
			patternName := namespaceName + "." + memberName

			var str String

			if shouldConvert {
				patt, ok := pattern.(ToStringConversionCapableStringPattern)
				if !ok {
					return nil, fmt.Errorf("pattern %s is not capable of conversion", patternName)
				}
				pattern = patt
				s, err := patt.StringFrom(ctx, sliceValue)
				if err != nil {
					return nil, fmt.Errorf("pattern %s failed to convert value", patternName)
				}
				str = String(s)
			} else {
				str = sliceValue.(String)
			}

			if !pattern.Test(ctx, str) {
				return nil, fmt.Errorf("runtime check error: `%s` does not match %%%s", str, patternName)
			}

			buff.WriteString(string(str))
		default:
			return nil, fmt.Errorf("runtime check error: slice value of type %T", sliceValue)
		}
	}

	str := String(buff.String())
	if !finalPattern.Test(ctx, str) {
		return nil, fmt.Errorf("runtime check error: final string `%s` does not match %%%s", str, matchingPatternName)
	}

	return &CheckedString{
		str:                 String(buff.String()),
		matchingPatternName: matchingPatternName,
		matchingPattern:     finalPattern,
	}, nil
}

func NewCheckedStringNoCheck(
	str String,
	matchingPatternName string, //if the matching pattern is in the namespace the name will contain a dot '.'
	matchingPattern Pattern,
) *CheckedString {
	return &CheckedString{
		str:                 str,
		matchingPatternName: matchingPatternName,
		matchingPattern:     matchingPattern,
	}
}

func (str *CheckedString) GetOrBuildString() string {
	return str.str.GetOrBuildString()
}

func (str *CheckedString) PropertyNames(ctx *Context) []string {
	return symbolic.CHECKED_STRING_PROPNAMES
}

func (str *CheckedString) Prop(ctx *Context, name string) Value {
	switch name {
	case "pattern-name":
		return String(str.matchingPatternName)
	case "pattern":
		return str.matchingPattern
	default:
		res, _ := getStringLikePseudoProp(name, str)
		return res
	}
}

func (CheckedString) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (s *CheckedString) slice(start, end int) Sequence {
	return s.str.slice(start, end)
}

func (s *CheckedString) Len() int {
	return s.str.Len()
}

func (s *CheckedString) ByteLen() int {
	return s.str.ByteLen()
}

func (s *CheckedString) RuneCount() int {
	return s.str.RuneCount()
}

func (s *CheckedString) At(ctx *Context, i int) Value {
	return s.str.At(ctx, i)
}

func (s *CheckedString) Replace(ctx *Context, old, new StringLike) StringLike {
	return s.str.Replace(ctx, old, new)
}

func (s *CheckedString) TrimSpace(ctx *Context) StringLike {
	return s.str.TrimSpace(ctx)
}

func (s *CheckedString) HasPrefix(ctx *Context, prefix StringLike) Bool {
	return s.str.HasPrefix(ctx, prefix)

}

func (s *CheckedString) HasSuffix(ctx *Context, suffix StringLike) Bool {
	return s.str.HasSuffix(ctx, suffix)
}

type RuneSlice struct {
	elements     []rune
	frozen       bool
	constraintId ConstraintId

	lock              sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	watchers          *ValueWatchers
	mutationCallbacks *MutationCallbacks
}

func NewRuneSlice(runes []rune) *RuneSlice {
	return &RuneSlice{elements: runes}
}

func (slice *RuneSlice) ElementsDoNotModify() []rune {
	return slice.elements
}

func (slice *RuneSlice) set(ctx *Context, i int, v Value) {
	if slice.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	slice.elements[i] = rune(v.(Rune))

	mutation := NewSetElemAtIndexMutation(ctx, i, v.(Rune), ShallowWatching, Path("/"+strconv.Itoa(i)))

	slice.mutationCallbacks.CallMicrotasks(ctx, mutation)
	slice.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (slice *RuneSlice) SetSlice(ctx *Context, start, end int, seq Sequence) {
	if slice.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	if seq.Len() != end-start {
		panic(errors.New(FormatIndexableShouldHaveLen(end - start)))
	}

	for i := start; i < end; i++ {
		slice.elements[i] = rune(seq.At(ctx, i-start).(Rune))
	}

	path := Path("/" + strconv.Itoa(int(start)) + ".." + strconv.Itoa(int(end-1)))
	mutation := NewSetSliceAtRangeMutation(ctx, NewIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

	slice.mutationCallbacks.CallMicrotasks(ctx, mutation)
	slice.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (slice *RuneSlice) Len() int {
	return len(slice.elements)
}

func (slice *RuneSlice) At(ctx *Context, i int) Value {
	return Rune(slice.elements[i])
}

func (s *RuneSlice) slice(start, end int) Sequence {
	sliceCopy := make([]rune, end-start)
	copy(sliceCopy, s.elements[start:end])

	return &RuneSlice{
		elements: sliceCopy,
	}
}

func (s *RuneSlice) insertElement(ctx *Context, v Value, i Int) {
	if s.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	r := v.(Rune)

	s.elements = append(s.elements, 0)
	copy(s.elements[i+1:], s.elements[i:len(s.elements)-1])
	s.elements[i] = rune(r)

	mutation := NewInsertElemAtIndexMutation(ctx, int(i), r, ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *RuneSlice) removePosition(ctx *Context, i Int) {
	if s.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	if int(i) > len(s.elements) || i < 0 {
		panic(ErrIndexOutOfRange)
	}

	if int(i) == len(s.elements)-1 { // remove last position
		s.elements = s.elements[:len(s.elements)-1]
	} else {
		copy(s.elements[i:], s.elements[i+1:])
		s.elements = s.elements[:len(s.elements)-1]
	}

	mutation := NewRemovePositionMutation(ctx, int(i), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *RuneSlice) removePositionRange(ctx *Context, r IntRange) {
	if s.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	start := int(r.KnownStart())
	end := int(r.InclusiveEnd())

	if start > len(s.elements) || start < 0 || end >= len(s.elements) || end < 0 {
		panic(ErrIndexOutOfRange)
	}

	if end == len(s.elements)-1 { // remove trailing sub slice
		s.elements = s.elements[:len(s.elements)-r.Len()]
	} else {
		copy(s.elements[start:], s.elements[end+1:])
		s.elements = s.elements[:len(s.elements)-r.Len()]
	}

	path := Path("/" + strconv.Itoa(int(r.KnownStart())) + ".." + strconv.Itoa(int(r.InclusiveEnd())))
	mutation := NewRemovePositionRangeMutation(ctx, r, ShallowWatching, path)

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *RuneSlice) insertSequence(ctx *Context, seq Sequence, i Int) {
	if s.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	//TODO: lock sequence
	seqLen := seq.Len()
	if seqLen == 0 {
		return
	}

	if cap(s.elements)-len(s.elements) < seqLen {
		newSlice := make([]rune, len(s.elements)+seqLen)
		copy(newSlice, s.elements)
		s.elements = newSlice
	} else {
		s.elements = s.elements[:len(s.elements)+seqLen]
	}

	copy(s.elements[int(i)+seqLen:], s.elements[i:])
	for ind := 0; ind < seqLen; ind++ {
		s.elements[int(i)+ind] = rune(seq.At(ctx, ind).(Rune))
	}

	path := Path("/" + strconv.Itoa(int(i)))
	mutation := NewInsertSequenceAtIndexMutation(ctx, int(i), seq, ShallowWatching, path)

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *RuneSlice) appendSequence(ctx *Context, seq Sequence) {
	if s.frozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	panic(ErrNotImplementedYet)
}

func (s *RuneSlice) PropertyNames(ctx *Context) []string {
	return RUNE_SLICE_PROPNAMES
}

func (s *RuneSlice) Prop(ctx *Context, name string) Value {
	switch name {
	case "insert":
		return WrapGoMethod(s.Insert)
	case "remove_position":
		return WrapGoMethod(s.removePosition)
	case "remove_position_range":
		return WrapGoMethod(s.removePositionRange)
	default:
		return nil
	}
}

func (*RuneSlice) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (s *RuneSlice) Insert(ctx *Context, v Value, i Int) {
	if seq, ok := v.(Sequence); ok {
		s.insertSequence(ctx, seq, i)
	} else {
		s.insertElement(ctx, v, i)
	}
}

type Identifier string

func (i Identifier) UnderlyingString() string {
	return string(i)
}

// StringConcatenation is a lazy concatenation of string-like values that can form a string, it implements StringLike and is therefore
// immutable from the POV of Inox code. StringConcatenation can be considered truly immutable once the concatenation has been performed.
// This can be forced by calling the GetOrBuildString method.
type StringConcatenation struct {
	elements    []stringConcatElem
	totalLen    int
	finalString string // not set by default
}

func ConcatStringLikes(stringLikes ...StringLike) (StringLike, error) {
	if len(stringLikes) == 1 {
		return stringLikes[0], nil
	}

	totalLen := 0

	//used for concatenating consecutive short strings
	var eagerConcatenation strings.Builder

	for _, s := range stringLikes {
		length := s.Len()
		if length == 0 {
			continue
		}
		totalLen += length
	}

	//If the total length is small, concatenate now and return a String.
	if totalLen < MIN_LAZY_STR_CONCATENATION_SIZE {
		var builder strings.Builder
		for _, elem := range stringLikes {
			builder.WriteString(elem.GetOrBuildString())
		}
		return String(builder.String()), nil
	}

	var elements []stringConcatElem

	for _, s := range stringLikes {
		length := s.Len()
		if length == 0 {
			continue
		}

		switch s := s.(type) {
		case String:
			if len(s) > MAX_SMALL_STRING_SIZE_IN_LAZY_STR_CONCATENATION {
				if eagerConcatenation.Len() > 0 {
					//Add the concatenation of short consecutive strings.
					elements = append(elements, stringConcatElem{string: eagerConcatenation.String()})
					eagerConcatenation.Reset()
				}

				elements = append(elements, stringConcatElem{string: string(s)})
				continue
			}
			eagerConcatenation.WriteString(string(s))
		case *StringConcatenation:
			if eagerConcatenation.Len() > 0 {
				//Add the concatenation of short consecutive strings.
				elements = append(elements, stringConcatElem{string: eagerConcatenation.String()})
				eagerConcatenation.Reset()
			}
			elements = append(elements, stringConcatElem{concatenation: s})
		default:
			str := s.GetOrBuildString()
			elements = append(elements, stringConcatElem{string: str})
			if len(str) > MAX_SMALL_STRING_SIZE_IN_LAZY_STR_CONCATENATION {
				if eagerConcatenation.Len() > 0 {
					//Add the concatenation of short consecutive strings.
					elements = append(elements, stringConcatElem{string: eagerConcatenation.String()})
					eagerConcatenation.Reset()
				}

				elements = append(elements, stringConcatElem{string: str})
				continue
			}
			eagerConcatenation.WriteString(str)
		}
	}

	if eagerConcatenation.Len() > 0 {
		//Add the concatenation of short consecutive strings.
		elements = append(elements, stringConcatElem{string: eagerConcatenation.String()})
	}

	return &StringConcatenation{
		elements: elements,
		totalLen: totalLen,
	}, nil
}

func NewStringConcatenation(elements ...StringLike) *StringConcatenation {
	if len(elements) < 2 {
		panic(errors.New("not enough elements"))
	}

	concatenation := &StringConcatenation{
		elements: utils.MapSlice(elements, func(e StringLike) stringConcatElem {
			return stringConcatElem{string: e.GetOrBuildString()}
		}),
	}

	for _, e := range elements {
		concatenation.totalLen += e.Len()
	}

	return concatenation
}

func (c *StringConcatenation) GetOrBuildString() string {
	if c.Len() > 0 && c.finalString == "" {
		slice := make([]byte, c.totalLen)
		pos := 0
		for _, elem := range c.elements {
			copy(slice[pos:pos+elem.Len()], elem.GetOrBuildString())
			pos += elem.Len()
		}
		c.finalString = string(slice)
		//get rid of elements to allow garbage collection ?
	}
	return c.finalString
}

func (c *StringConcatenation) Len() int {
	return c.totalLen
}

func (c *StringConcatenation) At(ctx *Context, i int) Value {
	elementIndex := 0
	pos := 0
	for pos <= i {
		element := c.elements[elementIndex]
		if pos+element.Len() > i {
			return element.At(ctx, i-pos)
		}
		elementIndex++
		pos += element.Len()
	}

	panic(ErrIndexOutOfRange)
}

func (c *StringConcatenation) slice(start, end int) Sequence {
	//TODO: change implementation + make the function stoppable for large strings

	return String(c.GetOrBuildString()).slice(start, end)
}

func (c *StringConcatenation) PropertyNames(ctx *Context) []string {
	return symbolic.STRING_LIKE_PSEUDOPROPS
}

func (c *StringConcatenation) Prop(ctx *Context, name string) Value {
	res, _ := getStringLikePseudoProp(name, c)
	return res
}

func (*StringConcatenation) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (c *StringConcatenation) ByteLen() int {
	total := 0
	for _, e := range c.elements {
		total += e.ByteLen()
	}
	return total
}

func (c *StringConcatenation) RuneCount() int {
	total := 0
	for _, e := range c.elements {
		total += e.RuneCount()
	}
	return total
}

func (c *StringConcatenation) Replace(ctx *Context, old, new StringLike) StringLike {
	//TODO: change implementation + make the function stoppable for large strings

	return String(c.GetOrBuildString()).Replace(ctx, old, new)
}

func (c *StringConcatenation) TrimSpace(ctx *Context) StringLike {
	//TODO: change implementation + make the function stoppable for large strings

	return String(c.GetOrBuildString()).TrimSpace(ctx)
}

func (c *StringConcatenation) HasPrefix(ctx *Context, prefix StringLike) Bool {
	//TODO: change implementation + make the function stoppable for large strings

	return String(c.GetOrBuildString()).HasPrefix(ctx, prefix)
}

func (c *StringConcatenation) HasSuffix(ctx *Context, prefix StringLike) Bool {
	//TODO: make the function stoppable for large strings

	return String(c.GetOrBuildString()).HasSuffix(ctx, prefix)
}

type stringConcatElem struct {
	concatenation *StringConcatenation
	string        string //not set if .concatenation is not nil
}

func (e stringConcatElem) Len() int {
	if e.concatenation != nil {
		return e.concatenation.Len()
	}
	return len(e.string)
}

func (e stringConcatElem) GetOrBuildString() string {
	if e.concatenation != nil {
		return e.concatenation.GetOrBuildString()
	}
	return e.string
}

func (e stringConcatElem) Equal(ctx *Context, s String, depth int) bool {
	if e.concatenation != nil {
		return e.concatenation.Equal(ctx, s, nil, depth)
	}
	return e.string == string(s)
}

func (e stringConcatElem) At(ctx *Context, i int) Value {
	if e.concatenation != nil {
		return e.concatenation.At(ctx, i)
	}
	return Byte(e.string[i])
}

func (e stringConcatElem) ByteLen() int {
	if e.concatenation != nil {
		return e.concatenation.ByteLen()
	}
	return len(e.string)
}

func (e stringConcatElem) RuneCount() int {
	if e.concatenation != nil {
		return e.concatenation.ByteLen()
	}
	return String(e.string).RuneCount()
}

func isSubstrOf(ctx *Context, a, b Value) bool {

	switch a := a.(type) {
	case StringLike:
		byteLenA := a.ByteLen()
		if byteLenA == 0 {
			return true
		}
		switch b := b.(type) {
		case StringLike:
			byteLenB := b.Len()
			if byteLenA > byteLenB {
				return false
			}
			if b.HasPrefix(ctx, a) || b.HasSuffix(ctx, a) { //Avoid creating the right string.
				return true
			}
			return strings.Contains(b.GetOrBuildString(), a.GetOrBuildString())
		case BytesLike:
			byteLenB := b.Len()
			if byteLenA > byteLenB {
				return false
			}
			stringB := utils.BytesAsString(b.GetOrBuildBytes().bytes)
			return strings.Contains(stringB, a.GetOrBuildString())
		}
	case BytesLike:
		byteLenA := a.Len()
		if byteLenA == 0 {
			return true
		}
		switch b := b.(type) {
		case StringLike:
			byteLenB := b.Len()
			if byteLenA > byteLenB {
				return false
			}
			stringA := utils.BytesAsString(a.GetOrBuildBytes().bytes)
			stringB := b.GetOrBuildString()

			return strings.Contains(stringB, stringA)
		case BytesLike:
			byteLenB := b.Len()
			if byteLenA > byteLenB {
				return false
			}

			stringA := utils.BytesAsString(a.GetOrBuildBytes().bytes)
			stringB := utils.BytesAsString(b.GetOrBuildBytes().bytes)
			return strings.Contains(stringB, stringA)
		}
	}
	panic(ErrUnreachable)
}

func getStringLikePseudoProp[S StringLike](name string, strLike S) (Value, bool) {
	switch name {
	case "byte-count":
		return ByteCount(strLike.ByteLen()), true
	case "rune-count":
		return RuneCount(strLike.RuneCount()), true
	case "replace":
		return ValOf(strLike.Replace), true
	case "trim_space":
		return ValOf(strLike.TrimSpace), true
	case "has_prefix":
		return ValOf(strLike.HasPrefix), true
	case "has_suffix":
		return ValOf(strLike.HasSuffix), true
	}

	return nil, false
}
