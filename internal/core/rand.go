package core

import (
	"bytes"
	"crypto/rand"

	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp/syntax"
	"strconv"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_RAND_LEN   = 10
	DEFAULT_MAX_OCCURRENCE = 20
)

var (
	MAX_INT64  = big.NewInt(math.MaxInt64)
	MAX_UINT64 = big.NewInt(0)

	CryptoRandSource  = &RandomnessSource{source: cryptoRandomnessSource{}}
	DefaultRandSource = CryptoRandSource

	_ = []io.Reader{DefaultRandSource}
)

func init() {
	MAX_UINT64 = MAX_UINT64.Mul(MAX_UINT64, big.NewInt(2))
	MAX_UINT64 = MAX_UINT64.Add(MAX_UINT64, big.NewInt(1))
}

type underlyingRandomnessSource interface {
	Read([]byte) (int, error)
}

type cryptoRandomnessSource struct{}

func (s cryptoRandomnessSource) Read(bytes []byte) (int, error) {
	return rand.Read(bytes[:])
}

type RandomnessSource struct {
	source underlyingRandomnessSource
}

func (s *RandomnessSource) Read(bytes []byte) (int, error) {
	return s.source.Read(bytes)
}

func (s *RandomnessSource) ReadNBytesAsHex(n int) string {
	bytes := make([]byte, n)
	return hex.EncodeToString(bytes)
}

func (r *RandomnessSource) Uint64() uint64 {
	var bytes [8]byte

	_, err := r.source.Read(bytes[:])
	if err != nil {
		panic(err)
	}

	return binary.LittleEndian.Uint64(bytes[:])
}

func (r *RandomnessSource) Int64() int64 {
	return int64(r.Uint64())
}

// RandUint64Range returns a random uint64 in the interval [start, end].
func (r *RandomnessSource) RandUint64Range(start, end uint64) uint64 {
	var val uint64

	if start > end {
		panic(errors.New("random uint64 generation: range's start must be less or equal range's end"))
	}

	if end == math.MaxUint64 && start == 0 {
		return r.Uint64()
	}

	// get uniformly distributed numbers in the range 0 to rangeSize
	rangeSize := end - start + 1
	min := (math.MaxUint64 - rangeSize) % rangeSize

	for {
		val = r.Uint64()

		if val > min {
			break
		}
	}

	val = val % rangeSize

	// shift to correct range
	return val + start
}

// RandInt64Range returns a random int64 in the interval [start, end].
func (r *RandomnessSource) RandInt64Range(start, end int64) int64 {
	if start > end {
		panic(errors.New("random int64 generation: range's start must be less or equal range's end"))
	}

	rangeSize := big.NewInt(0)
	rangeSize.Sub(big.NewInt(end), big.NewInt(start))

	u := int64(r.RandUint64Range(0, rangeSize.Uint64()))

	// shift to correct range
	return u + start
}

func (r *RandomnessSource) RandBit() bool {
	var b = []byte{0}
	_, err := r.source.Read(b)
	if err != nil {
		panic(err)
	}
	return b[0]%2 == 1
}

func RandInt(options ...Option) Value {
	var source = getRandomnessSource(options...)

	return Int(source.RandInt64Range(math.MinInt64, math.MaxInt64))
}

func RandFloat(options ...Option) Value {
	var source = getRandomnessSource(options...)

	float := utils.RandFloat(-math.MaxFloat64, math.MaxFloat64, source.Uint64())
	return Float(float)
}

func GetRandomnessSource(default_ *RandomnessSource, options ...Option) *RandomnessSource {
	for _, opt := range options {
		if opt.Name == "source" {
			return opt.Value.(*RandomnessSource)
		}
	}
	return default_
}

func getRandomnessSource(options ...Option) *RandomnessSource {
	return GetRandomnessSource(DefaultRandSource, options...)
}

func RandBool(options ...Option) Value {
	source := getRandomnessSource(options...)
	return Bool(source.RandBit())
}

// ------------ range ------------

func (r IntRange) Random(ctx *Context) Value {
	if r.unknownStart {
		panic("Random() not supported for int ranges with no start")
	}
	start := r.start
	end := r.end

	if !r.inclusiveEnd {
		end = r.end - 1
	}

	return Int(DefaultRandSource.RandInt64Range(int64(start), int64(end)))
}

func (r FloatRange) Random(ctx *Context) Value {
	if r.unknownStart {
		panic("Random() not supported for float ranges with no start")
	}

	var source = getRandomnessSource()

	float := utils.RandFloat(r.start, r.end, source.Uint64())
	return Float(float)
}

// ------------ patterns ------------

func (pattern ExactValuePattern) Random(ctx *Context, options ...Option) Value {
	return pattern.value
}

func (pattern ExactStringPattern) Random(ctx *Context, options ...Option) Value {
	return pattern.value
}

func (pattern TypePattern) Random(ctx *Context, options ...Option) Value {
	return pattern.RandomImpl(options...)
}

func (patt DynamicStringPatternElement) Random(ctx *Context, options ...Option) Value {
	return patt.resolve().Random(ctx)
}

func (patt *ObjectPattern) Random(ctx *Context, options ...Option) Value {
	obj := newUnitializedObjectWithPropCount(len(patt.entryPatterns))

	i := 0
	for k, v := range patt.entryPatterns {
		obj.keys[i] = k
		obj.values[i] = v.Random(ctx, options...).(Serializable)
		i++
	}
	obj.sortProps()

	//TODO: add random properties if inexact ?

	return obj
}

func (patt *RecordPattern) Random(ctx *Context, options ...Option) Value {
	rec := &Record{
		keys:   make([]string, len(patt.entryPatterns)),
		values: make([]Serializable, len(patt.entryPatterns)),
	}

	i := 0
	for k, v := range patt.entryPatterns {
		rec.keys[i] = k
		rec.values[i] = v.Random(ctx, options...).(Serializable)
		i++
	}
	rec.sortProps()

	//TODO: add random properties if inexact ?

	return rec
}

func randListPattern(ctx *Context, tuple bool, generalElementPattern Pattern, elementPatterns []Pattern, options ...Option) Value {
	source := getRandomnessSource(options...)

	if generalElementPattern != nil {
		randLen := source.RandInt64Range(0, DEFAULT_MAX_RAND_LEN)
		elements := make([]Serializable, randLen)

		for i := 0; i < int(randLen); i++ {
			elements[i] = generalElementPattern.Random(ctx, options...).(Serializable)
		}

		if tuple {
			return NewTuple(elements)
		} else {
			return WrapUnderlyingList(&ValueList{elements: elements})
		}
	} else {
		elements := make([]Serializable, len(elementPatterns))

		for i, e := range elementPatterns {
			elements[i] = e.Random(ctx, options...).(Serializable)
		}

		if tuple {
			return NewTuple(elements)
		} else {
			return WrapUnderlyingList(&ValueList{elements: elements})
		}
	}
}

func (patt ListPattern) Random(ctx *Context, options ...Option) Value {
	return randListPattern(ctx, false, patt.generalElementPattern, patt.elementPatterns, options...)
}

func (patt TuplePattern) Random(ctx *Context, options ...Option) Value {
	return randListPattern(ctx, true, patt.generalElementPattern, patt.elementPatterns, options...)
}

func (patt *DifferencePattern) Random(ctx *Context, options ...Option) Value {
	for {
		v := patt.base.Random(ctx, options...)
		if !patt.removed.Test(ctx, v) {
			return v
		}
	}
}

func (patt *OptionalPattern) Random(ctx *Context, options ...Option) Value {
	source := getRandomnessSource(options...)
	if source.RandBit() {
		return Nil
	}
	return patt.Pattern.Random(ctx, options...)
}

func (patt *FunctionPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplemented)
}

func (patt OptionPattern) Random(ctx *Context, options ...Option) Value {
	return Option{Name: patt.name, Value: patt.value.Random(ctx, options...)}
}

func (patt *RepeatedPatternElement) Random(ctx *Context, options ...Option) Value {
	source := getRandomnessSource(options...)
	buff := bytes.NewBufferString("")

	minCount, maxCount := patt.MinMaxCounts(DEFAULT_MAX_RAND_LEN)

	count := minCount
	if maxCount != minCount {
		count = minCount + int(source.RandUint64Range(uint64(minCount), uint64(maxCount)))
	}

	for i := 0; i < count; i++ {
		buff.WriteString(patt.element.Random(ctx, options...).(Str).UnderlyingString())
	}

	return Str(buff.String())
}

func (patt LengthCheckingStringPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (patt SequenceStringPattern) Random(ctx *Context, options ...Option) Value {
	s := bytes.NewBufferString("")
	for _, e := range patt.elements {
		s.WriteString(e.Random(ctx, options...).(Str).UnderlyingString())
	}

	return Str(s.String())
}

func (patt *UnionPattern) Random(ctx *Context, options ...Option) Value {
	source := getRandomnessSource(options...)
	sourceOption := Option{Name: "source", Value: source}

	if len(patt.cases) == 1 {
		return patt.cases[0].Random(ctx, sourceOption)
	}

	randCaseIndex := int(source.RandInt64Range(0, int64(len(patt.cases)-1)))
	return patt.cases[randCaseIndex].Random(ctx, sourceOption)
}

func (patt *IntersectionPattern) Random(ctx *Context, options ...Option) Value {
	source := getRandomnessSource(options...)
	sourceOption := Option{Name: "source", Value: source}

	if len(patt.cases) == 1 {
		return patt.cases[0].Random(ctx, sourceOption)
	}

	rand := patt.cases[0].Random(ctx, sourceOption)

loop:
	for {
		for _, otherCases := range patt.cases[1:] {
			if !otherCases.Test(ctx, rand) {
				rand = patt.cases[0].Random(ctx, sourceOption)
				continue loop
			}
		}
		break
	}

	return rand
}

func (patt UnionStringPattern) Random(ctx *Context, options ...Option) Value {
	source := getRandomnessSource(options...)

	if len(patt.cases) == 1 {
		return patt.cases[0].Random(ctx)
	}

	randCaseIndex := int(source.RandInt64Range(0, int64(len(patt.cases)-1)))
	return patt.cases[randCaseIndex].Random(ctx)
}

func (patt *RuneRangeStringPattern) Random(ctx *Context, options ...Option) Value {
	return Str(patt.runes.Random(ctx).(rune))
}

func (patt *IntRangePattern) Random(ctx *Context, options ...Option) Value {
	return patt.intRange.Random(ctx).(Int)
}

func (patt *FloatRangePattern) Random(ctx *Context, options ...Option) Value {
	return patt.floatRange.Random(ctx).(Float)
}

func (patt *EventPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (patt *MutationPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (pattern *NamedSegmentPathPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (pattern PathPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (pattern URLPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (pattern HostPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (pattern RegexPattern) Random(ctx *Context, options ...Option) Value {
	source := getRandomnessSource(options...)

	r := pattern.syntaxRegep
	buff := bytes.NewBuffer(nil)

	err := writeRandForRegexElement(r, buff, source)
	if err != nil {
		panic(err)
	}

	return Str(buff.String())
}

func writeRandForRegexElement(r *syntax.Regexp, buff *bytes.Buffer, source *RandomnessSource) error {
	var err error

	switch r.Op {
	case syntax.OpConcat:
		for _, sub := range r.Sub {
			if err := writeRandForRegexElement(sub, buff, source); err != nil {
				return err
			}
		}

	case syntax.OpLiteral:
		_, err = buff.WriteString(string(r.Rune))

	case syntax.OpCharClass:
		randIndex := source.RandUint64Range(0, uint64(len(r.Rune)-1))
		writeRandRune(r.Rune[randIndex], r.Rune[randIndex], buff, source)

	case syntax.OpQuest:
		if source.RandBit() {
			if err := writeRandForRegexElement(r.Sub[0], buff, source); err != nil {
				return err
			}
		}
	case syntax.OpPlus:
		writeRandRegexElementRandTimes(r.Sub[0], 1, DEFAULT_MAX_RAND_LEN, buff, source)

	case syntax.OpStar:
		writeRandRegexElementRandTimes(r.Sub[0], 0, DEFAULT_MAX_RAND_LEN, buff, source)

	case syntax.OpRepeat:
		writeRandRegexElementRandTimes(r.Sub[0], r.Min, r.Max, buff, source)

	case syntax.OpCapture:
		err = writeRandForRegexElement(r.Sub[0], buff, source)

	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		buff.WriteRune('?')

	case syntax.OpAlternate:
		randIndex := int(source.RandUint64Range(0, uint64(len(r.Sub)-1)))
		err = writeRandForRegexElement(r.Sub[randIndex], buff, source)

	case syntax.OpEmptyMatch:

	default:
		err = fmt.Errorf("unknown/unsupported syntax operator %s", r.Op.String())
	}

	return err
}

func writeRandRune(min, max rune, buff *bytes.Buffer, source *RandomnessSource) {
	if min == max {
		buff.WriteRune(min)
		return
	}
	r := rune(source.RandUint64Range(uint64(min), uint64(max)))

	buff.WriteRune(r)
}

func writeRandRegexElementRandTimes(r *syntax.Regexp, min, max int, buff *bytes.Buffer, source *RandomnessSource) error {
	count := min
	if min != max {
		count = int(source.RandUint64Range(uint64(min), uint64(max)))
	}

	for i := 0; i < count; i++ {
		if err := writeRandForRegexElement(r, buff, source); err != nil {
			return err
		}
	}

	return nil
}

func (pattern *ParserBasedPseudoPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplemented)
}

func (pattern *IntRangeStringPattern) Random(ctx *Context, options ...Option) Value {
	n := int64(pattern.intRange.Random(ctx).(Int))
	return Str(strconv.FormatInt(n, 10))
}

func (pattern *FloatRangeStringPattern) Random(ctx *Context, options ...Option) Value {
	n := float64(pattern.floatRange.Random(ctx).(Float))
	return Str(strconv.FormatFloat(n, 'g', -1, 64))
}

func (pattern *PathStringPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplementedYet)
}

func (patt *StructPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplemented)
}
