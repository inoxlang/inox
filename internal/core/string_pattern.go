package internal

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const REGEX_SYNTAX = syntax.Perl

var (
	ErrStrGroupMatchingOnlySupportedForPatternWithRegex = errors.New("group matching is only supported by string patterns with a regex for now")
	ErrCannotParse                                      = errors.New("cannot parse")
	ErrInvalidInputString                               = errors.New("invalid input string")
	_                                                   = []StringPattern{&ParserBasedPattern{}}
)

type StringPattern interface {
	Pattern

	Regex() string
	CompiledRegex() *regexp.Regexp
	HasRegex() bool

	LengthRange() IntRange
	EffectiveLengthRange() IntRange //length range effectively used to match strings

	validate(s string, i *int) bool
	FindMatches(*Context, Value, MatchesFindConfig) (groups []Value, err error)
	Parse(*Context, string) (Value, error)
}

type MatchesFindConfigKind int

const (
	FindFirstMatch MatchesFindConfigKind = iota
	FindAllMatches
)

type MatchesFindConfig struct {
	Kind MatchesFindConfigKind
}

// ExactStringPattern matches values equal to .value: .value.Equal(...) returns true.
type ExactStringPattern struct {
	NotCallablePatternMixin
	NoReprMixin

	value  Str
	regexp *regexp.Regexp
}

func NewExactStringPattern(value Str) *ExactStringPattern {
	regex := regexp.QuoteMeta(string(value))
	regexp := regexp.MustCompile(regex)

	return &ExactStringPattern{
		value:  value,
		regexp: regexp,
	}
}

func (pattern *ExactStringPattern) Test(ctx *Context, v Value) bool {
	return pattern.value.Equal(ctx, v, map[uintptr]uintptr{}, 0)
}

func (pattern *ExactStringPattern) Regex() string {
	return pattern.regexp.String()
}

func (patt *ExactStringPattern) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (pattern *ExactStringPattern) HasRegex() bool {
	return true
}

func (pattern *ExactStringPattern) validate(parsed string, i *int) bool {
	exactString := pattern.value

	length := len(exactString)
	index := *i
	if len(parsed)-index < length {
		return false
	}

	if parsed[index:index+length] == string(exactString) {
		*i += length
		return true
	}
	return false
}

func (patt *ExactStringPattern) Parse(ctx *Context, s string) (Value, error) {
	if s != string(patt.value) {
		return nil, errors.New("string not equal to expected string")
	}

	return Str(s), nil
}

func (pattern *ExactStringPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (matches []Value, err error) {
	return FindMatchesForStringPattern(ctx, pattern, val, config)
}

func (pattern *ExactStringPattern) LengthRange() IntRange {
	//cache ?

	length := utf8.RuneCountInString(string(pattern.value))
	return IntRange{
		Start:        int64(length),
		End:          int64(length),
		inclusiveEnd: true,
		Step:         1,
	}
}

func (pattern *ExactStringPattern) EffectiveLengthRange() IntRange {
	return pattern.LengthRange()
}

func (patt *ExactStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// SequenceStringPattern represents a string pattern with sub elements
type SequenceStringPattern struct {
	regexp             *regexp.Regexp
	entireStringRegexp *regexp.Regexp
	syntaxRegexp       *syntax.Regexp

	node       parse.Node
	elements   []StringPattern
	groupNames []string

	lengthRange             IntRange
	hasEffectiveLengthRange bool
	effectiveLengthRange    IntRange
}

func NewSequenceStringPattern(node parse.Node, subpatterns []StringPattern, groupNames KeyList) (*SequenceStringPattern, error) {
	allElemsHaveRegex := true

	if len(groupNames) != 0 && len(groupNames) != len(subpatterns) {
		return nil, errors.New("sequence string pattern: number of provided group names is not equal to the number of subpatterns")
	}

	for _, patternElement := range subpatterns {
		if repeated, ok := patternElement.(*RepeatedPatternElement); ok {
			patternElement = repeated.element
		}
		if !patternElement.HasRegex() {
			allElemsHaveRegex = false
		}
	}

	var regex *regexp.Regexp
	var entireStringRegex *regexp.Regexp

	lengthRange := IntRange{
		Start:        0,
		End:          0,
		inclusiveEnd: true,
		Step:         1,
	}

	if allElemsHaveRegex {
		regexBuff := bytes.NewBufferString("")

		for _, subpatt := range subpatterns {
			subpatternRegexBuff := bytes.NewBufferString("")

			//create regex for sub pattern
			if repeatedElement, ok := subpatt.(*RepeatedPatternElement); ok {
				subpatternRegexBuff.WriteRune('(')

				elementRegex := utils.Must(syntax.Parse(repeatedElement.element.Regex(), REGEX_SYNTAX))
				elementRegex = turnCapturingGroupsIntoNonCapturing(elementRegex)

				subpatternRegexBuff.WriteString("(?:")
				subpatternRegexBuff.WriteString(elementRegex.String())
				subpatternRegexBuff.WriteRune(')')

				switch repeatedElement.ocurrenceModifier {
				case parse.AtLeastOneOcurrence:
					subpatternRegexBuff.WriteRune('+')
				case parse.ZeroOrMoreOcurrence:
					subpatternRegexBuff.WriteRune('*')
				case parse.OptionalOcurrence:
					subpatternRegexBuff.WriteRune('?')
				case parse.ExactOcurrence:
					subpatternRegexBuff.WriteRune('{')
					subpatternRegexBuff.WriteString(strconv.Itoa(repeatedElement.exactCount))
					subpatternRegexBuff.WriteRune('}')
				}
				subpatternRegexBuff.WriteRune(')')

				repeatedElement.regexp = regexp.MustCompile(subpatternRegexBuff.String())
			} else {
				subpattRegex := utils.Must(syntax.Parse(subpatt.Regex(), REGEX_SYNTAX))
				subpattRegex = turnCapturingGroupsIntoNonCapturing(subpattRegex)

				subpatternRegexBuff.WriteRune('(')
				subpatternRegexBuff.WriteString(subpattRegex.String())
				subpatternRegexBuff.WriteRune(')')
			}

			// append the sub pattern's regex to the sequence's regex.
			regexBuff.WriteString(subpatternRegexBuff.String())

			subPattLenRange := subpatt.LengthRange()
			lengthRange = lengthRange.clampedAdd(subPattLenRange)
		}

		entireRegexExpr := "^" + regexBuff.String() + "$"
		regexExpr := entireRegexExpr[1 : len(entireRegexExpr)-1]

		regex = regexp.MustCompile(regexExpr)
		entireStringRegex = regexp.MustCompile(entireRegexExpr)
	}

	return &SequenceStringPattern{
		regexp:               regex,
		entireStringRegexp:   entireStringRegex,
		node:                 node,
		elements:             subpatterns,
		lengthRange:          lengthRange,
		effectiveLengthRange: lengthRange,
		groupNames:           utils.CopySlice(groupNames),
	}, nil
}

func (patt *SequenceStringPattern) Test(ctx *Context, v Value) bool {
	_str, ok := v.(StringLike)
	if !ok || !checkMatchedStringLen(_str, patt) {
		return false
	}

	str := _str.GetOrBuildString()
	if patt.HasRegex() {
		return patt.entireStringRegexp.MatchString(str)
	} else {
		i := 0
		return patt.validate(str, &i) && i == len(str)
	}
}

func (patt *SequenceStringPattern) validate(s string, i *int) bool {
	j := *i
	for _, el := range patt.elements {
		if !el.validate(s, &j) {
			return false
		}
	}
	*i = j
	return true
}

func (patt *SequenceStringPattern) Parse(ctx *Context, s string) (Value, error) {
	if !patt.Test(ctx, Str(s)) {
		return nil, ErrInvalidInputString
	}
	return Str(s), nil
}

func (patt *SequenceStringPattern) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	if !patt.HasRegex() {
		return nil, false, ErrStrGroupMatchingOnlySupportedForPatternWithRegex
	}

	s, ok := v.(StringLike)
	if !ok {
		return nil, false, nil
	}

	submatches := patt.regexp.FindStringSubmatch(s.GetOrBuildString())
	if submatches == nil || !patt.Test(ctx, v) {
		return nil, false, nil
	}

	obj, ok, err := patt.constructGroupMatchingResult(ctx, submatches)
	if ok {
		return obj.EntryMap(), true, nil
	}
	return nil, ok, err
}

func (patt *SequenceStringPattern) FindGroupMatches(ctx *Context, v Value, config GroupMatchesFindConfig) (groups []*Object, err error) {
	if !patt.HasRegex() {
		return nil, ErrStrGroupMatchingOnlySupportedForPatternWithRegex
	}

	s, ok := v.(StringLike)
	if !ok {
		return nil, nil
	}

	//TODO: prevent DoS

	submatchesList, err := FindGroupMatchesForRegex(ctx, patt.regexp, s.GetOrBuildString(), config)
	if err != nil {
		return nil, err
	}

	if submatchesList == nil {
		return nil, nil
	}

	results := make([]*Object, len(submatchesList))

	for i, submatches := range submatchesList {
		result, ok, err := patt.constructGroupMatchingResult(ctx, submatches)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		results[i] = result
	}

	return results, nil
}

func (patt *SequenceStringPattern) constructGroupMatchingResult(ctx *Context, submatches []string) (groups *Object, ok bool, err error) {
	if len(patt.groupNames) >= 0 {
		result := newUnitializedObjectWithPropCount(0)

		for i, submatch := range submatches[1:] { //first submatch is whole match
			groupName := patt.groupNames[i]

			if groupName == "" {
				continue
			}

			if groupPatt, ok := patt.elements[i].(GroupPattern); ok {
				subresult, ok, err := groupPatt.MatchGroups(ctx, Str(submatch))
				if err != nil {
					return nil, false, err
				}
				if !ok {
					return nil, false, nil
				}
				result.keys = append(result.keys, patt.groupNames[i])
				result.values = append(result.values, objFrom(subresult))
			} else {
				result.keys = append(result.keys, patt.groupNames[i])
				result.values = append(result.values, Str(submatch))
			}
		}
		result.keys = append(result.keys, "0")
		result.values = append(result.values, Str(submatches[0]))
		result.implicitPropCount++
		result.sortProps()
		return result, true, nil
	} else {
		return objFrom(ValMap{"0": Str(submatches[0])}), true, nil
	}

}

func (patt *SequenceStringPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *SequenceStringPattern) Regex() string {
	return patt.regexp.String()
}

func (patt *SequenceStringPattern) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (patt *SequenceStringPattern) HasRegex() bool {
	return patt.regexp != nil
}

func (patt *SequenceStringPattern) LengthRange() IntRange {
	return patt.lengthRange
}

func (patt *SequenceStringPattern) EffectiveLengthRange() IntRange {
	return patt.effectiveLengthRange
}

func (patt *SequenceStringPattern) Call(values []Value) (Pattern, error) {
	lenRange, found, err := getNewEffectiveLenRange(values, patt.LengthRange())
	if err != nil {
		return nil, err
	}

	clone, err := patt.Clone(map[uintptr]map[int]Value{})
	if err != nil {
		return nil, err
	}

	pattClone := clone.(*SequenceStringPattern)

	if found {
		pattClone.effectiveLengthRange = lenRange
		pattClone.hasEffectiveLengthRange = true
	}

	return pattClone, nil
}

func (patt *SequenceStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type UnionStringPattern struct {
	NotCallablePatternMixin
	regexp             *regexp.Regexp
	entireStringRegexp *regexp.Regexp

	node  parse.Node
	cases []StringPattern
}

func NewUnionStringPattern(node parse.Node, cases []StringPattern) (*UnionStringPattern, error) {

	allCasesHaveRegex := true
	noCaseHaveRegex := true

	for _, patternElement := range cases {
		if !patternElement.HasRegex() {
			allCasesHaveRegex = false
		} else {
			noCaseHaveRegex = false
		}
	}

	if noCaseHaveRegex {
		return nil, fmt.Errorf("failed to create a string pattern union: at least one of the case should be non-recursive")
	}

	var regex *regexp.Regexp
	var entireStringRegex *regexp.Regexp

	if allCasesHaveRegex {
		regexBuff := bytes.NewBufferString("(")

		for i, patternElement := range cases {

			if i > 0 {
				regexBuff.WriteRune('|')
			}

			if !patternElement.HasRegex() {
				return &UnionStringPattern{
					node:  node,
					cases: cases,
				}, nil
			}

			regexBuff.WriteString(patternElement.Regex())
		}

		regexBuff.WriteRune(')')
		regex = regexp.MustCompile(regexBuff.String())
		entireStringRegex = regexp.MustCompile("^" + regexBuff.String() + "$")
	}

	return &UnionStringPattern{
		regexp:             regex,
		entireStringRegexp: entireStringRegex,

		node:  node,
		cases: cases,
	}, nil
}

func (patt *UnionStringPattern) Test(ctx *Context, v Value) bool {
	_str, ok := v.(StringLike)
	if !ok {
		return false
	}
	str := _str.GetOrBuildString()

	if patt.HasRegex() {
		return patt.entireStringRegexp.MatchString(str)
	} else {
		for _, case_ := range patt.cases {
			j := 0
			if case_.validate(str, &j) && j == len(str) {
				return true
			}
		}
	}
	return false
}

func (patt *UnionStringPattern) validate(s string, i *int) bool {
	for _, case_ := range patt.cases {
		j := *i
		if case_.validate(s, &j) {
			*i = j
			return true
		}
	}
	return false
}

func (patt *UnionStringPattern) Parse(ctx *Context, s string) (Value, error) {
	return nil, ErrCannotParse
}

func (patt *UnionStringPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *UnionStringPattern) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	_, ok := v.(StringLike)
	if !ok {
		return nil, false, nil
	}

	for _, case_ := range patt.cases {

		if case_.Test(ctx, v) {
			if groupPattern, ok := case_.(GroupPattern); ok {
				result, ok, _ := groupPattern.MatchGroups(ctx, v)
				if ok {
					return result, true, nil
				}
			} else {
				return map[string]Value{"0": v}, true, nil
			}
		}

	}

	return nil, false, nil
}

func (patt *UnionStringPattern) Regex() string {
	return patt.regexp.String()
}

func (patt *UnionStringPattern) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (patt *UnionStringPattern) HasRegex() bool {
	return patt.regexp != nil
}

func (patt *UnionStringPattern) LengthRange() IntRange {
	return patt.lengthRange(false)
}

func (patt *UnionStringPattern) EffectiveLengthRange() IntRange {
	return patt.lengthRange(true)
}

func (patt *UnionStringPattern) lengthRange(effective bool) IntRange {
	minLen := int64(math.MaxInt64)
	maxLen := int64(0)

	for _, case_ := range patt.cases {

		var lenRange IntRange
		if effective {
			lenRange = case_.EffectiveLengthRange()
		} else {
			lenRange = case_.LengthRange()
		}

		minLen = utils.Min(minLen, lenRange.Start)
		maxLen = utils.Max(maxLen, lenRange.InclusiveEnd())
	}

	return IntRange{
		Start:        minLen,
		End:          maxLen,
		inclusiveEnd: true,
		Step:         1,
	}
}

func (patt *UnionStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type RuneRangeStringPattern struct {
	NotCallablePatternMixin
	regexp             *regexp.Regexp
	entireStringRegexp *regexp.Regexp

	node  parse.Node
	runes RuneRange
}

func NewRuneRangeStringPattern(lower, upper rune, node parse.Node) *RuneRangeStringPattern {
	entireRegex := fmt.Sprintf("^[%c-%c]$", lower, upper)

	return &RuneRangeStringPattern{
		regexp:             regexp.MustCompile(entireRegex[1 : len(entireRegex)-1]),
		entireStringRegexp: regexp.MustCompile(entireRegex),

		node: node,
		runes: RuneRange{
			Start: lower,
			End:   upper,
		},
	}

}
func (patt *RuneRangeStringPattern) Test(ctx *Context, v Value) bool {
	str, ok := v.(StringLike)
	if !ok {
		return false
	}
	return patt.regexp.MatchString(str.GetOrBuildString())
}

func (patt *RuneRangeStringPattern) validate(s string, i *int) bool {

	for _, r := range s[*i:] {
		if patt.runes.Start <= r && r <= patt.runes.End {
			*i += len(string(r))
			return true
		}
		return false
	}

	return false
}

func (patt *RuneRangeStringPattern) Parse(ctx *Context, s string) (Value, error) {
	if utf8.RuneCountInString(s) != 1 {
		return nil, errors.New("failed to parse rune: string has not exatly one rune")
	}
	for _, r := range s {
		if patt.runes.Start <= r && r <= patt.runes.End {
			return Rune(r), nil
		} else {
			return nil, errors.New("rune is not in range")
		}
	}
	panic(ErrUnreachable)
}

func (patt *RuneRangeStringPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *RuneRangeStringPattern) Regex() string {
	return patt.regexp.String()
}

func (patt *RuneRangeStringPattern) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (patt *RuneRangeStringPattern) HasRegex() bool {
	return true
}

func (patt *RuneRangeStringPattern) LengthRange() IntRange {
	return IntRange{
		Start:        1,
		End:          1,
		inclusiveEnd: true,
		Step:         1,
	}
}

func (patt *RuneRangeStringPattern) EffectiveLengthRange() IntRange {
	return patt.LengthRange()
}

func (patt *RuneRangeStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// An IntRangeStringPattern matches a string (or substring) representing a decimal integer number in a given range.
// Example: for the range (-99,99) the found match substrings in the following strings are surrounded  by parenthesis.
// positive:
// (12)
// (12)-
// a12
// 123
// 12_
// 12a
// a12
//
// negative:
// (-12)
// (-12)-
// -(-12)
// a(-12)
// -123
// -12_
// -12a
type IntRangeStringPattern struct {
	NotCallablePatternMixin
	NoReprMixin
	NotClonableMixin

	regexp             *regexp.Regexp
	entireStringRegexp *regexp.Regexp

	node        parse.Node
	intRange    IntRange
	lengthRange IntRange
}

func NewIntRangeStringPattern(lower, upper int64, node parse.Node) *IntRangeStringPattern {
	entireRegex := ""

	lengthRange := IntRange{
		Step:         1,
		inclusiveEnd: true,
	}

	onlyNines := func(i int64) bool {
		for _, digit := range strconv.FormatInt(utils.Abs(i), 10) {
			if digit != '9' {
				return false
			}
		}
		return true
	}

	if lower < 0 {
		if !onlyNines(lower) {
			panic(errors.New(
				"for now integer range string patterns only support lower bounds with only '9' digits",
			))
		}
		if !onlyNines(upper) {
			panic(errors.New("for now integer range string patterns only support upper bounds with only '9' digits"))
		}

		absLower := math.Abs(float64(lower))
		absUpper := math.Abs(float64(upper))
		negativeIntPart := ""
		positiveIntPart := ""
		digitsRegex := "(?:\\d{%d,%d})\\b"

		if upper < 0 {
			max := utils.Max(absLower, absUpper)
			min := utils.Min(absLower, absUpper)

			maxDigitCount := int(math.Ceil(math.Log10(max)))
			minDigitCount := int(math.Ceil(math.Log10(min)))

			negativeIntPart = fmt.Sprintf(digitsRegex, minDigitCount, maxDigitCount)

			lengthRange.Start = 1 + int64(minDigitCount)
			lengthRange.End = 1 + int64(maxDigitCount)
		} else {
			negMaxDigitCount := int(math.Ceil(math.Log10(absLower)))
			posMaxDigitCount := 1
			if upper >= 1 {
				posMaxDigitCount = int(math.Ceil(math.Log10(float64(upper))))
			}

			negativeIntPart = fmt.Sprintf(digitsRegex, 0, negMaxDigitCount)
			positiveIntPart = fmt.Sprintf(digitsRegex, 0, posMaxDigitCount)

			lengthRange.Start = 1 // zero
			lengthRange.End = int64(utils.Max(1+negMaxDigitCount, posMaxDigitCount))
		}
		entireRegex = fmt.Sprintf("^(-%s|\\b%s)$", negativeIntPart, positiveIntPart)
	} else {
		if (upper % 9) != 0 {
			panic(errors.New("for now integer range string patterns only support upper bounds with only '9' digits"))
		}

		minDigitCount := 1
		if lower >= 1 {
			minDigitCount = int(math.Ceil(math.Log10(float64(lower))))
		}

		maxDigitCount := 1

		if upper >= 1 {
			maxDigitCount = int(math.Ceil(math.Log10(float64(upper))))
		}

		entireRegex = fmt.Sprintf("^\\b\\d{%d,%d}\\b$", minDigitCount, maxDigitCount)

		lengthRange.Start = int64(minDigitCount)
		lengthRange.End = int64(maxDigitCount)
	}
	return &IntRangeStringPattern{
		regexp:             regexp.MustCompile(entireRegex[1 : len(entireRegex)-1]),
		entireStringRegexp: regexp.MustCompile(entireRegex),

		node: node,
		intRange: IntRange{
			inclusiveEnd: true,
			Start:        lower,
			End:          upper,
			Step:         1,
		},
		lengthRange: lengthRange,
	}
}

func (patt *IntRangeStringPattern) Test(ctx *Context, v Value) bool {
	str, ok := v.(StringLike)
	if !ok {
		return false
	}
	return patt.entireStringRegexp.MatchString(str.GetOrBuildString())
}

func (patt *IntRangeStringPattern) validate(s string, i *int) bool {
	panic(ErrNotImplementedYet)
}

func (patt *IntRangeStringPattern) Parse(ctx *Context, s string) (Value, error) {
	i, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		return nil, err
	}
	if patt.intRange.Includes(ctx, Int(i)) {
		return Int(i), nil
	}
	return nil, errors.New("integer is not in valid range")
}

func (patt *IntRangeStringPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *IntRangeStringPattern) Regex() string {
	return patt.regexp.String()
}

func (patt *IntRangeStringPattern) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (patt *IntRangeStringPattern) HasRegex() bool {
	return true
}

func (patt *IntRangeStringPattern) LengthRange() IntRange {
	return patt.lengthRange
}

func (patt *IntRangeStringPattern) EffectiveLengthRange() IntRange {
	return patt.LengthRange()
}

func (patt *IntRangeStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

type DynamicStringPatternElement struct {
	name string
	ctx  *Context
}

func (patt DynamicStringPatternElement) resolve() StringPattern {
	return patt.ctx.ResolveNamedPattern(patt.name).(StringPattern)
}

func (patt DynamicStringPatternElement) Test(ctx *Context, v Value) bool {
	return patt.resolve().Test(ctx, v)
}

func (patt DynamicStringPatternElement) validate(s string, i *int) bool {
	return patt.resolve().validate(s, i)
}

func (patt *DynamicStringPatternElement) Parse(ctx *Context, s string) (Value, error) {
	return patt.resolve().Parse(ctx, s)
}

func (patt DynamicStringPatternElement) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	panic("DynamicStringPatternElement cannot find matches because it cannot have a regex")
}

func (patt DynamicStringPatternElement) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	panic("DynamicStringPatternElement cannot match groups because it cannot have a regex")
}

func (patt DynamicStringPatternElement) Regex() string {
	panic("DynamicStringPatternElement cannot have a regex")
}

func (patt DynamicStringPatternElement) CompiledRegex() *regexp.Regexp {
	panic("DynamicStringPatternElement cannot have a regex")
}

func (patt DynamicStringPatternElement) Call(values []Value) (Pattern, error) {
	return patt.resolve().Call(values)
}

func (patt *DynamicStringPatternElement) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt DynamicStringPatternElement) HasRegex() bool {
	return false
}

func (patt DynamicStringPatternElement) LengthRange() IntRange {
	return patt.resolve().LengthRange()
}

func (patt DynamicStringPatternElement) EffectiveLengthRange() IntRange {
	return patt.resolve().EffectiveLengthRange()
}

type RepeatedPatternElement struct {
	NotCallablePatternMixin
	regexp            *regexp.Regexp
	ocurrenceModifier parse.OcurrenceCountModifier
	exactCount        int
	element           StringPattern
}

func (patt *RepeatedPatternElement) Test(ctx *Context, v Value) bool {
	_str, ok := v.(StringLike)
	if !ok {
		return false
	}
	str := _str.GetOrBuildString()
	if patt.HasRegex() {
		return patt.regexp.MatchString(str)
	} else {
		i := 0
		return patt.validate(str, &i)
	}
}

func (patt *RepeatedPatternElement) validate(s string, i *int) bool {
	j := *i
	ok := false

	if !patt.element.validate(s, &j) {
		ok = patt.ocurrenceModifier == parse.ZeroOrMoreOcurrence || patt.ocurrenceModifier == parse.OptionalOcurrence
	} else {
		count := 1
		for patt.element.validate(s, &j) { //TODO: fix: stop if count == exact count
			count++
		}

		switch patt.ocurrenceModifier {
		case parse.ExactlyOneOcurrence:
			ok = count == 1
		case parse.AtLeastOneOcurrence, parse.ZeroOrMoreOcurrence, parse.OptionalOcurrence:
			ok = true
		case parse.ExactOcurrence:
			ok = count == patt.exactCount
		}
	}

	if ok {
		*i = j
	}
	return ok
}

func (patt *RepeatedPatternElement) Parse(ctx *Context, s string) (Value, error) {
	return nil, ErrCannotParse
}

func (patt *RepeatedPatternElement) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *RepeatedPatternElement) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	_, ok := v.(StringLike)

	if !ok || !patt.Test(ctx, v) {
		return nil, false, nil
	}

	return map[string]Value{"0": v}, true, nil
}

func (patt *RepeatedPatternElement) Regex() string {
	return patt.regexp.String()
}

func (patt *RepeatedPatternElement) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (patt *RepeatedPatternElement) HasRegex() bool {
	return patt.regexp != nil
}

func (patt *RepeatedPatternElement) LengthRange() IntRange {
	return patt.lengthRange(false)
}

func (patt *RepeatedPatternElement) EffectiveLengthRange() IntRange {
	return patt.lengthRange(true)
}

func (patt *RepeatedPatternElement) lengthRange(effective bool) IntRange {
	var elemRange IntRange
	if effective {
		elemRange = patt.element.EffectiveLengthRange()
	} else {
		elemRange = patt.element.LengthRange()
	}

	switch patt.ocurrenceModifier {
	case parse.ExactlyOneOcurrence:
		return elemRange
	case parse.AtLeastOneOcurrence:
		return IntRange{
			Start:        elemRange.Start, //elem range should always have a known start
			End:          math.MaxInt64,
			inclusiveEnd: true,
			Step:         1,
		}
	case parse.ZeroOrMoreOcurrence:
		return IntRange{
			Start:        0,
			End:          math.MaxInt64,
			inclusiveEnd: true,
			Step:         1,
		}
	case parse.OptionalOcurrence:
		return IntRange{
			Start:        0,
			End:          elemRange.End,
			inclusiveEnd: elemRange.inclusiveEnd,
			Step:         1,
		}
	case parse.ExactOcurrence:
		return elemRange.times(int64(patt.exactCount), int64(patt.exactCount), true)

	default:
		panic("invalid ocurrence modifier")
	}
}

func (patt *RepeatedPatternElement) MinMaxCounts(maxRandOcurrence int) (int, int) {
	minCount := patt.exactCount
	maxCount := patt.exactCount

	switch patt.ocurrenceModifier {
	case parse.ExactOcurrence:
		//ok
	case parse.ExactlyOneOcurrence:
		minCount = 1
		maxCount = 1
	case parse.ZeroOrMoreOcurrence:
		minCount = 0
		maxCount = maxRandOcurrence
	case parse.AtLeastOneOcurrence:
		minCount = 1
		maxCount = maxRandOcurrence
	case parse.OptionalOcurrence:
		minCount = 0
		maxCount = 1
	}
	return minCount, maxCount
}

func (patt *RepeatedPatternElement) StringPattern() (StringPattern, bool) {
	return nil, false
}

type ParserBasedPattern struct {
	NotCallablePatternMixin
	NoReprMixin
	NotClonableMixin

	parser StatelessParser
}

func NewParserBasePattern(parser StatelessParser) *ParserBasedPattern {
	return &ParserBasedPattern{parser: parser}
}

func (patt *ParserBasedPattern) Test(ctx *Context, v Value) bool {
	s, ok := v.(StringLike)
	if !ok {
		return false
	}
	return patt.parser.Validate(ctx, s.GetOrBuildString())
}

func (pattern *ParserBasedPattern) Regex() string {
	panic(ErrNotImplemented)
}

func (patt *ParserBasedPattern) CompiledRegex() *regexp.Regexp {
	panic(ErrNotImplemented)
}

func (pattern *ParserBasedPattern) HasRegex() bool {
	return false
}

func (patt *ParserBasedPattern) validate(s string, i *int) bool {
	panic(ErrNotImplementedYet)
}

func (patt *ParserBasedPattern) Parse(ctx *Context, s string) (Value, error) {
	return patt.parser.Parse(ctx, s)
}

func (patt *ParserBasedPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return nil, ErrNotImplementedYet
}

func (patt *ParserBasedPattern) LengthRange() IntRange {
	return IntRange{
		inclusiveEnd: true,
		Start:        0,
		End:          10_000,
		Step:         1,
	}
}

func (patt *ParserBasedPattern) EffectiveLengthRange() IntRange {
	return patt.LengthRange()
}

func (patt *ParserBasedPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// A NamedSegmentPathPattern is a path pattern with named sections, NamedSegmentPathPattern implements GroupPattern.
type NamedSegmentPathPattern struct {
	NotCallablePatternMixin
	node *parse.NamedSegmentPathPatternLiteral
}

func (patt *NamedSegmentPathPattern) Test(ctx *Context, v Value) bool {
	_, ok, err := patt.MatchGroups(ctx, v)
	return ok && err == nil
}

func (patt *NamedSegmentPathPattern) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	pth, ok := v.(Path)
	if !ok {
		return nil, false, nil
	}

	str := string(pth)
	i := 0
	groups := map[string]Value{"0": v}

	for index, s := range patt.node.Slices {

		if i >= len(str) {
			return nil, false, nil
		}

		switch n := s.(type) {
		case *parse.PathPatternSlice:
			if i+len(n.Value) > len(str) {
				return nil, false, nil
			}
			if str[i:i+len(n.Value)] != n.Value {
				return nil, false, nil
			}
			i += len(n.Value)
		case *parse.NamedPathSegment:
			segmentEnd := strings.Index(str[i:], "/")
			if segmentEnd < 0 {
				if index < len(patt.node.Slices)-1 {
					return nil, false, nil
				}
				groups[n.Name] = Str(str[i:])
				return groups, true, nil
			} else if index == len(patt.node.Slices)-1 { //if $var$ is at the end of the pattern there should not be a '/'
				return nil, false, nil
			} else {
				groups[n.Name] = Str(str[i : i+segmentEnd])
				i += segmentEnd
			}
		}
	}

	if i == len(str) {
		return groups, true, nil
	}

	return nil, false, nil
}

func (patt *NamedSegmentPathPattern) FindGroupMatches(*Context, Value, GroupMatchesFindConfig) (groups []*Object, err error) {
	return nil, ErrNotImplementedYet
}

func (patt *NamedSegmentPathPattern) PropertyNames(ctx *Context) []string {
	return nil
}

func (patt *NamedSegmentPathPattern) Prop(ctx *Context, name string) Value {
	return nil
}

func (*NamedSegmentPathPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (patt *NamedSegmentPathPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// RegexPattern matches any StringLike that matches .regexp
type RegexPattern struct {
	regexp      *regexp.Regexp
	syntaxRegep *syntax.Regexp

	hasEffectiveLengthRange bool
	effectiveLengthRange    IntRange
}

// NewRegexPattern creates a RegexPattern from the given string, the unicode flag is enabled.
func NewRegexPattern(s string) *RegexPattern {
	regexp := regexp.MustCompile(s) //compiles with syntax.Perl flag
	syntaxRegexp := utils.Must(syntax.Parse(s, REGEX_SYNTAX))
	syntaxRegexp = turnCapturingGroupsIntoNonCapturing(syntaxRegexp)

	return &RegexPattern{
		regexp:      regexp,
		syntaxRegep: syntaxRegexp,
	}
}

func (pattern *RegexPattern) Test(ctx *Context, v Value) bool {
	str, ok := v.(StringLike)
	if !ok || !checkMatchedStringLen(str, pattern) {
		return false
	}
	return pattern.regexp.MatchString(str.GetOrBuildString())
}

func (pattern *RegexPattern) Regex() string {
	return pattern.regexp.String()
}

func (patt *RegexPattern) CompiledRegex() *regexp.Regexp {
	return patt.regexp
}

func (pattern *RegexPattern) HasRegex() bool {
	return true
}

func (patt *RegexPattern) validate(s string, i *int) bool {
	panic(".validate() not implemented yet for regex patterns")
}

func (patt *RegexPattern) Parse(ctx *Context, s string) (Value, error) {
	if !patt.Test(ctx, Str(s)) {
		return nil, ErrInvalidInputString
	}
	return Str(s), nil
}

func (patt *RegexPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *RegexPattern) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	_, ok := v.(StringLike)
	if !ok || !patt.Test(ctx, v) {
		return nil, false, nil
	}

	return map[string]Value{"0": v}, true, nil
}

func (patt *RegexPattern) LengthRange() IntRange {

	var computeLenRange func(r *syntax.Regexp) (lenRange IntRange)

	computeLenRange = func(r *syntax.Regexp) (lenRange IntRange) {
		lenRange = IntRange{
			Step:         1,
			inclusiveEnd: true,
		}

		switch r.Op {
		case syntax.OpConcat:
			if len(r.Sub) == 0 {
				return
			}

			lenRange = computeLenRange(r.Sub[0])

			for _, sub := range r.Sub[1:] {
				subLenRange := computeLenRange(sub)
				lenRange = lenRange.clampedAdd(subLenRange)
			}
			return

		case syntax.OpLiteral:
			n := int64(len(r.Rune))
			lenRange.Start = n
			lenRange.End = n
			return

		case syntax.OpCharClass:
			lenRange.Start = 1
			lenRange.End = 1
			return

		case syntax.OpQuest:
			subLenRange := computeLenRange(r.Sub[0])
			lenRange.Start = 0
			lenRange.End = subLenRange.End
			return

		case syntax.OpPlus:
			subLenRange := computeLenRange(r.Sub[0])
			lenRange.Start = subLenRange.Start
			lenRange.End = math.MaxInt64
			return

		case syntax.OpStar:
			lenRange.Start = 0
			lenRange.End = math.MaxInt64
			return

		case syntax.OpRepeat:
			subLenRange := computeLenRange(r.Sub[0])

			if r.Max < 0 { //no maximum (infinite)
				lenRange = subLenRange.times(int64(r.Min), math.MaxInt64, true)
				return
			}

			lenRange = subLenRange.times(int64(r.Min), int64(r.Max), true)
			return

		case syntax.OpCapture:
			return computeLenRange(r.Sub[0])

		case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
			lenRange.Start = 1
			lenRange.End = 1
			return

		case syntax.OpAlternate:
			minLen := int64(math.MaxInt64)
			maxLen := int64(0)

			for _, sub := range r.Sub {
				subLenRange := computeLenRange(sub)
				minLen = utils.Min(minLen, subLenRange.Start)
				maxLen = utils.Max(maxLen, subLenRange.End)
			}

			lenRange.Start = minLen
			lenRange.End = maxLen
			return

		case syntax.OpEmptyMatch:
			lenRange.Start = 0
			lenRange.End = 0
			return
		case syntax.OpNoWordBoundary, syntax.OpWordBoundary, syntax.OpBeginText, syntax.OpEndText:
			return
		}

		panic(fmt.Errorf("unknown/unsupported syntax operator %s", r.Op.String()))
	}

	return computeLenRange(patt.syntaxRegep)
}

func (patt *RegexPattern) EffectiveLengthRange() IntRange {
	if patt.hasEffectiveLengthRange {
		return patt.effectiveLengthRange
	}
	return patt.LengthRange()
}

func (patt *RegexPattern) Call(values []Value) (Pattern, error) {
	lenRange, found, err := getNewEffectiveLenRange(values, patt.LengthRange())
	if err != nil {
		return nil, err
	}

	clone, err := patt.Clone(map[uintptr]map[int]Value{})
	if err != nil {
		return nil, err
	}

	pattClone := clone.(*RegexPattern)

	if found {
		pattClone.effectiveLengthRange = lenRange
		pattClone.hasEffectiveLengthRange = true
	}

	return pattClone, nil
}

func (patt *RegexPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// A PathStringPattern represents a string pattern for paths
type PathStringPattern struct {
	NoReprMixin
	optionalPathPattern PathPattern

	hasEffectiveLengthRange bool
	effectiveLengthRange    IntRange
}

// NewRegexPattern creates a StringPathPattern from the given string, if path pattern is empty the pattern matches any path.
func NewStringPathPattern(pathPattern PathPattern) *PathStringPattern {
	return &PathStringPattern{optionalPathPattern: pathPattern}
}

// AddValidPathPrefix adds the ./ prefix if necessary, AddValidPathPrefix does NOT check that its argument is valid path.
func AddValidPathPrefix(s string) (string, error) {
	if s != "" && s[0] == '/' {
		return s, nil
	}

top:
	switch len(s) {
	case 0:
		return "", ErrInvalidInputString
	case 1:
		switch s {
		case ".":
			s = "./"
		case "/":
			break top
		default:
			s = "./" + s
		}
	case 2:
		switch s {
		case "..":
			s = "./.."
		case "./":
			break top
		default:
			s = "./" + s
		}
	default:
		switch s[:3] {
		case "../":
			break top
		}

		switch s[:2] {
		case "./":
			break top
		default:
			s = "./" + s
		}
	}

	return s, nil
}

func (pattern *PathStringPattern) Test(ctx *Context, v Value) bool {
	str, ok := v.(StringLike)
	if !ok || !checkMatchedStringLen(str, pattern) {
		return false
	}

	path, err := AddValidPathPrefix(str.GetOrBuildString())
	if err != nil {
		return false
	}

	if pattern.optionalPathPattern == "" {
		parsed, _ := ParseRepr(ctx, []byte(path))
		_, ok := parsed.(Path)
		return ok
	}
	return pattern.optionalPathPattern.Test(ctx, Str(path))
}

func (pattern *PathStringPattern) Regex() string {
	panic(ErrNotImplemented)
}

func (patt *PathStringPattern) CompiledRegex() *regexp.Regexp {
	panic(ErrNotImplemented)
}

func (pattern *PathStringPattern) HasRegex() bool {
	return false
}

func (patt *PathStringPattern) validate(s string, i *int) bool {
	panic(ErrNotImplementedYet)
}

func (patt *PathStringPattern) Parse(ctx *Context, s string) (Value, error) {
	path, err := AddValidPathPrefix(s)
	if err != nil {
		return nil, ErrInvalidInputString
	}

	if !patt.Test(ctx, Str(s)) {
		return nil, ErrInvalidInputString
	}
	return Path(path), nil
}

func (patt *PathStringPattern) FindMatches(ctx *Context, val Value, config MatchesFindConfig) (groups []Value, err error) {
	return FindMatchesForStringPattern(ctx, patt, val, config)
}

func (patt *PathStringPattern) MatchGroups(ctx *Context, v Value) (map[string]Value, bool, error) {
	_, ok := v.(StringLike)
	if !ok || !patt.Test(ctx, v) {
		return nil, false, nil
	}

	return map[string]Value{"0": v}, true, nil
}

func (patt *PathStringPattern) LengthRange() IntRange {
	return IntRange{
		Start:        1,
		End:          100,
		inclusiveEnd: true,
		Step:         1,
	}
}

func (patt *PathStringPattern) EffectiveLengthRange() IntRange {
	if patt.hasEffectiveLengthRange {
		return patt.effectiveLengthRange
	}
	return patt.LengthRange()
}

func (patt *PathStringPattern) Call(values []Value) (Pattern, error) {
	lenRange, found, err := getNewEffectiveLenRange(values, patt.LengthRange())
	if err != nil {
		return nil, err
	}

	newPattern, err := patt.Clone(map[uintptr]map[int]Value{})
	if err != nil {
		return nil, err
	}

	pattClone := newPattern.(*PathStringPattern)

	if found {
		pattClone.effectiveLengthRange = lenRange
		pattClone.hasEffectiveLengthRange = true
	}

	return pattClone, nil
}

func (patt *PathStringPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func getNewEffectiveLenRange(args []Value, originalRange IntRange) (intRange IntRange, found bool, err error) {
	for _, arg := range args {
		switch a := arg.(type) {
		case IntRange:
			if found {
				return IntRange{}, false, FmtErrArgumentProvidedAtLeastTwice("length range")
			}
			found = true
			intRange = a
		default:
			return IntRange{}, false, FmtErrInvalidArgument(a)
		}
	}

	if intRange.unknownStart {
		return IntRange{}, false, errors.New("provided length range should not have an unknown start")
	}

	if intRange.Start < originalRange.Start {
		return IntRange{}, false, fmt.Errorf(
			"provided length range have a minimum (%d) smaller than the minimum length of the called pattern (%d)",
			intRange.Start, originalRange.Start,
		)
	}

	if intRange.InclusiveEnd() > originalRange.InclusiveEnd() {
		return IntRange{}, false, fmt.Errorf(
			"provided length range have a maximum (%d) bigger than the maximum length of the called pattern (%d)",
			intRange.InclusiveEnd(), originalRange.InclusiveEnd(),
		)
	}

	return
}

func checkMatchedStringLen(s StringLike, patt StringPattern) bool {
	lenRange := patt.EffectiveLengthRange()

	minPossibleRuneCount, maxPossibleRuneCount := utils.MinMaxPossibleRuneCount(s.ByteLen())

	if int64(minPossibleRuneCount) > lenRange.InclusiveEnd() || int64(maxPossibleRuneCount) < lenRange.Start {
		return false
	}

	//slow check
	runeCount := int64(s.RuneCount())
	return runeCount >= lenRange.Start && runeCount <= lenRange.InclusiveEnd()
}

func turnCapturingGroupsIntoNonCapturing(regex *syntax.Regexp) *syntax.Regexp {
	newRegex := new(syntax.Regexp)

	if regex.Op != syntax.OpCapture {
		newRegex.Op = regex.Op
		newRegex.Flags = regex.Flags
	}

	switch regex.Op {
	case syntax.OpConcat:
		newRegex.Sub = make([]*syntax.Regexp, len(regex.Sub))
		for i, sub := range regex.Sub {
			newRegex.Sub[i] = turnCapturingGroupsIntoNonCapturing(sub)
		}

	case syntax.OpLiteral:
		newRegex.Rune = regex.Rune

	case syntax.OpCharClass:
		newRegex.Rune = regex.Rune

	case syntax.OpQuest, syntax.OpPlus, syntax.OpStar:
		newRegex.Sub = []*syntax.Regexp{turnCapturingGroupsIntoNonCapturing(regex.Sub[0])}

	case syntax.OpRepeat:
		newRegex.Min = regex.Min
		newRegex.Max = regex.Max
		newRegex.Sub = []*syntax.Regexp{turnCapturingGroupsIntoNonCapturing(regex.Sub[0])}

	case syntax.OpCapture:
		return turnCapturingGroupsIntoNonCapturing(regex.Sub[0])

	case syntax.OpAlternate:
		newRegex.Sub = make([]*syntax.Regexp, len(regex.Sub))
		for i, sub := range regex.Sub {
			newRegex.Sub[i] = turnCapturingGroupsIntoNonCapturing(sub)
		}

	case syntax.OpAnyChar, syntax.OpAnyCharNotNL, syntax.OpEmptyMatch, syntax.OpNoWordBoundary, syntax.OpWordBoundary:
	case syntax.OpEndText, syntax.OpBeginText:

	default:
		panic(fmt.Errorf("unknown syntax operator %s", regex.Op.String()))
	}

	return newRegex
}

func FindMatchesForStringPattern(ctx *Context, patt StringPattern, val Value, config MatchesFindConfig) (matches []Value, err error) {
	if !patt.HasRegex() {
		return nil, ErrNotImplementedYet
	}

	s, ok := val.(StringLike)
	if !ok {
		return nil, nil
	}

	matches, err = FindMatchesForRegex(ctx, patt.CompiledRegex(), s.GetOrBuildString(), config)
	if err != nil {
		return nil, err
	}

	return matches, nil
}

func FindMatchesForRegex(ctx *Context, regexp *regexp.Regexp, s string, config MatchesFindConfig) (matches []Value, err error) {
	switch config.Kind {
	case FindAllMatches:
		matches := regexp.FindAllString(string(s), -1)
		results := make([]Value, len(matches))
		for i, match := range matches {
			results[i] = Str(match)
		}
		return results, nil
	case FindFirstMatch:
		match := regexp.FindString(string(s))

		return []Value{Str(match)}, nil
	default:
		panic(fmt.Errorf("matching: invalid config"))
	}

}

func FindGroupMatchesForRegex(ctx *Context, regexp *regexp.Regexp, s string, config GroupMatchesFindConfig) (groups [][]string, err error) {
	switch config.Kind {
	case FindAllGroupMatches:
		return regexp.FindAllStringSubmatch(string(s), -1), nil
	case FindFirstGroupMatches:
		match := regexp.FindStringSubmatch(string(s))

		if match == nil {
			return nil, nil
		}

		return [][]string{match}, nil
	default:
		return nil, fmt.Errorf("matching: invalid config")
	}
}
