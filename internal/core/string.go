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

	"github.com/inoxlang/inox/internal/parse"
)

var (
	STRING_LIKE_PSEUDOPROPS = []string{"replace", "trim_space", "has_prefix", "has_suffix"}
	RUNE_SLICE_PROPNAMES    = []string{"insert", "remove_position", "remove_position_range"}

	_ = []WrappedString{
		Str(""), Path(""), PathPattern(""), Host(""), HostPattern(""), EmailAddress(""), Identifier(""),
		URL(""), URLPattern(""), CheckedString{},
	}
	_ = []StringLike{Str(""), (*StringConcatenation)(nil)}
)

// A StringLike represents a value that wraps a Go string.
type WrappedString interface {
	Serializable

	//UnderlyingString() should instantly retrieves the wrapped string
	UnderlyingString() string
}

// A StringLike represents an abstract immutable string, it should behave exactly like a regular Str and have the same pseudo properties.
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
	HasSuffix(ctx *Context, prefix StringLike) Bool

	//TODO: EqualStringLike(ctx *Context, s StringLike)
}

// Inox string type, Str implements Value.
type Str string

func (s Str) UnderlyingString() string {
	return string(s)
}

func (s Str) GetOrBuildString() string {
	return string(s)
}

func (s Str) ByteLike() []byte {
	return []byte(s)
}

func (s Str) Len() int {
	return len(s)
}

func (s Str) At(ctx *Context, i int) Value {
	return Byte(s[i])
}

func (s Str) slice(start, end int) Sequence {
	return s[start:end]
}

func (s Str) PropertyNames(ctx *Context) []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (s Str) Prop(ctx *Context, name string) Value {
	switch name {
	case "replace":
		return ValOf(s.Replace)
	case "trim_space":
		return ValOf(s.TrimSpace)
	case "has_prefix":
		return ValOf(s.HasPrefix)
	case "has_suffix":
		return ValOf(s.HasSuffix)
	default:
		return nil
	}
}

func (Str) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (s Str) ByteLen() int {
	return len(s)
}
func (s Str) RuneCount() int {
	return utf8.RuneCountInString(string(s))
}

func (s Str) Replace(ctx *Context, old, new StringLike) StringLike {
	//TODO: make the function stoppable for large strings

	return Str(strings.Replace(string(s), old.GetOrBuildString(), new.GetOrBuildString(), -1))
}

func (s Str) TrimSpace(ctx *Context) StringLike {
	//TODO: make the function stoppable for large strings

	return Str(strings.TrimSpace(string(s)))
}

func (s Str) HasPrefix(ctx *Context, prefix StringLike) Bool {
	//TODO: make the function stoppable for large strings

	return Bool(strings.HasPrefix(string(s), prefix.GetOrBuildString()))
}

func (s Str) HasSuffix(ctx *Context, prefix StringLike) Bool {
	//TODO: make the function stoppable for large strings

	return Bool(strings.HasSuffix(string(s), prefix.GetOrBuildString()))
}

type Rune rune

func (r Rune) PropertyNames(ctx *Context) []string {
	return []string{"is_space", "is_printable", "is_letter"}
}

func (r Rune) Prop(ctx *Context, name string) Value {
	switch name {
	case "is_space":
		return Bool(unicode.IsSpace(rune(r)))
	case "is_printable":
		return Bool(unicode.IsPrint(rune(r)))
	case "is_letter":
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
	str                 string
	matchingPatternName string //if the matching pattern is in the namespace the name will contain a dot '.'
	matchingPattern     Pattern
}

func NewStringFromSlices(slices []Value, node *parse.StringTemplateLiteral, ctx *Context) (Str, error) {
	buff := bytes.NewBufferString("")

	for i, sliceValue := range slices {
		switch node.Slices[i].(type) {
		case *parse.StringTemplateSlice:
			buff.WriteString(string(sliceValue.(Str)))
		case *parse.StringTemplateInterpolation:
			switch val := sliceValue.(type) {
			case StringLike:
			case Int:
				sliceValue = Str(Stringify(val, ctx))
			default:
				panic(ErrUnreachable)
			}

			str := sliceValue.(StringLike).GetOrBuildString()
			buff.WriteString(string(str))
		default:
			return "", fmt.Errorf("runtime check error: slice value of type %T", sliceValue)
		}
	}

	str := Str(buff.String())
	return str, nil
}

// NewCheckedString creates a CheckedString in a secure way.
func NewCheckedString(slices []Value, node *parse.StringTemplateLiteral, ctx *Context) (CheckedString, error) {
	patternIdent, isPatternAnIdent := node.Pattern.(*parse.PatternIdentifierLiteral)

	var (
		namespaceName       string
		namespace           *PatternNamespace
		finalPattern        Pattern
		matchingPatternName string
	)
	if isPatternAnIdent {
		if node.HasInterpolations() {
			return CheckedString{}, errors.New("string template literals with interpolations should be preceded by a pattern which name has a prefix")
		}
		finalPattern = ctx.ResolveNamedPattern(patternIdent.Name)
		if finalPattern == nil {
			return CheckedString{}, fmt.Errorf("pattern %%%s does not exist", patternIdent.Name)
		}
		matchingPatternName = patternIdent.Name
	} else {
		namespaceMembExpr := node.Pattern.(*parse.PatternNamespaceMemberExpression)
		namespaceName = namespaceMembExpr.Namespace.Name
		namespace = ctx.ResolvePatternNamespace(namespaceName)

		if namespace == nil {
			return CheckedString{}, fmt.Errorf("cannot interpolate: pattern namespace '%s' does not exist", namespaceName)
		}

		memberName := namespaceMembExpr.MemberName.Name
		pattern, ok := namespace.Patterns[memberName]
		if !ok {
			return CheckedString{}, fmt.Errorf("cannot interpolate: member .%s of pattern namespace '%s' does not exist", memberName, namespaceName)
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
					return CheckedString{}, fmt.Errorf("pattern namespace member should be followed by .from not .%s", conversion)
				}
				shouldConvert = true
			}

			pattern, ok := namespace.Patterns[memberName]
			if !ok {
				return CheckedString{}, fmt.Errorf("cannot interpolate: member .%s of pattern namespace '%s' does not exist", memberName, namespaceName)
			}
			patternName := namespaceName + "." + memberName

			var str Str

			if shouldConvert {
				patt, ok := pattern.(ToStringConversionCapableStringPattern)
				if !ok {
					return CheckedString{}, fmt.Errorf("pattern %s is not capable of conversion", patternName)
				}
				pattern = patt
				s, err := patt.StringFrom(ctx, sliceValue)
				if err != nil {
					return CheckedString{}, fmt.Errorf("pattern %s failed to convert value", patternName)
				}
				str = Str(s)
			} else {
				str = sliceValue.(Str)
			}

			if !pattern.Test(ctx, str) {
				return CheckedString{}, fmt.Errorf("runtime check error: `%s` does not match %%%s", str, patternName)
			}

			buff.WriteString(string(str))
		default:
			return CheckedString{}, fmt.Errorf("runtime check error: slice value of type %T", sliceValue)
		}
	}

	str := Str(buff.String())
	if !finalPattern.Test(ctx, str) {
		return CheckedString{}, fmt.Errorf("runtime check error: final string `%s` does not match %%%s", str, matchingPatternName)
	}

	return CheckedString{
		str:                 buff.String(),
		matchingPatternName: matchingPatternName,
		matchingPattern:     finalPattern,
	}, nil
}

func (str CheckedString) String() string {
	return "`" + str.str + "`"
}

func (str CheckedString) UnderlyingString() string {
	return str.str
}

func (str CheckedString) PropertyNames(ctx *Context) []string {
	return []string{"pattern_name", "pattern"}
}

func (str CheckedString) Prop(ctx *Context, name string) Value {
	switch name {
	case "pattern_name":
		return Str(str.matchingPatternName)
	case "pattern":
		return str.matchingPattern
	default:
		return nil
	}
}

func (CheckedString) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
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
	mutation := NewSetSliceAtRangeMutation(ctx, NewIncludedEndIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

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

type PropertyName string

func (p PropertyName) UnderlyingString() string {
	return string(p)
}

// StringConcatenation is a lazy concatenation of values that can form a string, StringConcatenation implements StringLike and is
// therefore immutable.
type StringConcatenation struct {
	elements    []StringLike
	totalLen    int
	finalString string // empty by default
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

	return Str(c.GetOrBuildString()).slice(start, end)
}

func (c *StringConcatenation) PropertyNames(ctx *Context) []string {
	return STRING_LIKE_PSEUDOPROPS
}

func (c *StringConcatenation) Prop(ctx *Context, name string) Value {
	switch name {
	case "replace":
		return ValOf(c.Replace)
	case "trim_space":
		return ValOf(c.TrimSpace)
	case "has_prefix":
		return ValOf(c.HasPrefix)
	case "has_suffix":
		return ValOf(c.HasSuffix)
	default:
		return nil
	}
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

	return Str(c.GetOrBuildString()).Replace(ctx, old, new)
}

func (c *StringConcatenation) TrimSpace(ctx *Context) StringLike {
	//TODO: change implementation + make the function stoppable for large strings

	return Str(c.GetOrBuildString()).TrimSpace(ctx)
}

func (c *StringConcatenation) HasPrefix(ctx *Context, prefix StringLike) Bool {
	//TODO: change implementation + make the function stoppable for large strings

	return Str(c.GetOrBuildString()).HasPrefix(ctx, prefix)
}

func (c *StringConcatenation) HasSuffix(ctx *Context, prefix StringLike) Bool {
	//TODO: make the function stoppable for large strings

	return Str(c.GetOrBuildString()).HasSuffix(ctx, prefix)
}
