package core

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrUnknownStartIntRange   = errors.New("integer range has unknown start")
	ErrUnknownStartFloatRange = errors.New("float range has unknown start")

	_ = []Integral{Int(0), Byte(0)}
)

type Integral interface {
	Value
	Int64() (n int64, signed bool)
}

// Int implements Value.
type Int int64

func (i Int) Int64() (n int64, signed bool) {
	return int64(i), true
}

// Float implements Value.
type Float float64

// A FloatRange represents a float64 range, FloatRange implements Value.
// Inox's float range literals (e.g. `1.0..2.0`) evaluate to a FloatRange.
type FloatRange struct {
	unknownStart bool //if true .Start depends on the context (not *Context)
	inclusiveEnd bool
	start        float64 //can be negative infinity
	end          float64 //can be positive infinity
}

func NewIncludedEndFloatRange(start, end float64) FloatRange {
	if end < start {
		panic(fmt.Errorf("failed to create float range, end < start"))
	}
	return FloatRange{inclusiveEnd: true, start: start, end: end}
}

func NewFloatRange(start, end float64, inclusiveEnd bool) FloatRange {
	if end < start {
		panic(fmt.Errorf("failed to create float range, end < start"))
	}
	return FloatRange{
		inclusiveEnd: inclusiveEnd,
		start:        start,
		end:          end,
	}
}

func NewUnknownStartFloatRange(end float64, inclusiveEnd bool) FloatRange {
	return FloatRange{
		unknownStart: true,
		inclusiveEnd: inclusiveEnd,
		end:          end,
	}
}

func (r FloatRange) Includes(ctx *Context, n Float) bool {
	if r.unknownStart {
		panic(ErrUnknownStartFloatRange)
	}

	return r.start <= float64(n) && float64(n) <= r.InclusiveEnd()
}

func (r FloatRange) HasKnownStart() bool {
	return !r.unknownStart
}

func (r FloatRange) KnownStart() float64 {
	if r.unknownStart {
		panic(ErrUnknownStartFloatRange)
	}
	return r.start
}

func (r FloatRange) InclusiveEnd() float64 {
	if r.inclusiveEnd || math.IsInf(r.end, 1) {
		return r.end
	}
	return math.Nextafter(r.end, math.Inf(-1))
}

// intAdd adds l an r in a safe way:
// - if there is an overflow the returned value is nil and the error is ErrIntOverflow.
// - if there is an underflow the returned value is nil and the error is ErrIntUnderflow.
func intAdd(l, r Int) (Value, error) {
	if r > 0 {
		if l > math.MaxInt64-r {
			return nil, ErrIntOverflow
		}
	} else {
		if l < math.MinInt64-r {
			return nil, ErrIntUnderflow
		}
	}
	return l + r, nil
}

// intSub substracts r from l in a safe way:
// - if there is an overflow the returned value is nil and the error is ErrIntOverflow.
// - if there is an underflow the returned value is nil and the error is ErrIntUnderflow.
func intSub(l, r Int) (Value, error) {
	if r < 0 {
		if l > math.MaxInt64+r {
			return nil, ErrIntOverflow
		}
	} else {
		if l < math.MinInt64+r {
			return nil, ErrIntUnderflow
		}
	}
	return l - r, nil
}

// intMul multiplies l and r in a safe way:
// - if there is an overflow the returned value is nil and the error is ErrIntOverflow.
// - if there is an underflow the returned value is nil and the error is ErrIntUnderflow.
func intMul(l, r Int) (Value, error) {
	if r > 0 {
		if l > math.MaxInt64/r || l < math.MinInt64/r {
			return nil, ErrIntOverflow
		}
	} else if r < 0 {
		if r == -1 {
			if l == math.MinInt64 {
				return nil, ErrIntOverflow
			}
		} else if l < math.MaxInt64/r || l > math.MinInt64/r {
			return nil, ErrIntUnderflow
		}
	}
	return l * r, nil
}

// intDiv multiplies l by r in a safe way:
// - if there is an overflow the returned value is nil and the error is ErrIntOverflow.
// - if r is equal to zero the returned value is nil and the error is ErrIntDivisionByZero.
func intDiv(l, r Int) (Value, error) {
	if r == 0 {
		return nil, ErrIntDivisionByZero
	}
	if l == math.MinInt64 && r == -1 {
		return nil, ErrIntOverflow
	}
	return l / r, nil
}

// An IntRange represents an int64 range, IntRange implements Value.
// Inox's integer range literals (e.g. `1..2`) evaluate to an IntRange.
type IntRange struct {
	unknownStart bool //if true .Start depends on the context (not *Context)
	start        int64
	end          int64 //inclusive
	step         int64 //only 1 supported for now
}

func NewIntRange(start, inclusiveEnd int64) IntRange {
	if inclusiveEnd < start {
		panic(fmt.Errorf("failed to create int range, end < start"))
	}
	return IntRange{
		start: start,
		end:   inclusiveEnd,
		step:  1,
	}
}

func NewUnknownStartIntRange(end int64) IntRange {
	return IntRange{
		unknownStart: true,
		end:          end,
		step:         1,
	}
}

func (r IntRange) Includes(ctx *Context, i Int) bool {
	if r.unknownStart {
		panic(ErrUnknownStartIntRange)
	}

	return r.start <= int64(i) && int64(i) <= r.InclusiveEnd()
}

func (r IntRange) At(ctx *Context, i int) Value {
	if i >= r.Len() {
		panic(ErrIndexOutOfRange)
	}
	return Int(i + int(r.start))
}

// Len returns the number of integers in the range if the start (lower bound) is known.
// Len panics otherwise.
func (r IntRange) Len() int {
	if r.unknownStart {
		panic(ErrUnknownStartIntRange)
	}

	return r.len(r.start)
}

func (r IntRange) HasKnownStart() bool {
	return !r.unknownStart
}

func (r IntRange) KnownStart() int64 {
	if r.unknownStart {
		panic(ErrUnknownStartIntRange)
	}
	return r.start
}

func (r IntRange) InclusiveEnd() int64 {
	return r.end
}

func (r IntRange) len(min int64) int {
	start := r.start
	if r.unknownStart {
		start = min
	}
	length := r.end - start + 1
	return int(length)
}

// clampedAdd adds .Start(s) togethers & .End(s) toghethers, on overflow the corresponding field is set to math.MaxInt64.
func (r IntRange) clampedAdd(other IntRange) IntRange {

	if r.unknownStart || other.unknownStart {
		panic(errors.New("cannot clamp add int ranges with at least one unknown start"))
	}

	if r.step != 1 || other.step != 1 {
		panic(errors.New("cannot clamp add other range: only ranges with a step of 1 are supported"))
	}

	if r.start < 0 || other.start < 0 {
		panic(errors.New("cannot clamp add other range: only positive start ranges are supported"))
	}

	newRange := IntRange{
		start: r.start,
		end:   r.end,
		step:  1,
	}

	if other.start >= math.MaxInt64-r.start {
		newRange.start = math.MaxInt64
		newRange.end = math.MaxInt64
		return newRange
	} else {
		newRange.start = r.start + other.start
	}

	if other.end >= math.MaxInt64-r.end {
		newRange.end = math.MaxInt64
	} else {
		newRange.end = r.end + other.InclusiveEnd()
	}

	return newRange
}

func (r IntRange) times(n, m int64, clamped bool) IntRange {

	if n < 0 {
		panic(fmt.Errorf("cannot multiply the lower bound of an integer range by a negative number: %d", n))
	}

	if m <= 0 {
		panic(fmt.Errorf("cannot multiply the upper bound of an integer range by a negative number or zero: %d", m))
	}

	if r.unknownStart {
		panic(errors.New("cannot multiply int range with an unknown start"))
	}

	minOverflow := r.start != 0 && n >= math.MaxInt64/r.start
	maxOverflow := r.end != 0 && m >= math.MaxInt64/r.end

	if !clamped && (minOverflow || maxOverflow) {
		inclusiveEnd := r.InclusiveEnd()

		if r.unknownStart {
			panic(fmt.Errorf("cannot multiply integer range ..%d by %d %d", inclusiveEnd, n, m))
		}
		panic(fmt.Errorf("cannot multiply integer range %d..%d by %d %d", r.start, inclusiveEnd, n, m))
	}

	start := int64(0)
	end := int64(0)

	if minOverflow {
		start = math.MaxInt64
	} else {
		start = r.start * n
	}

	if maxOverflow {
		end = math.MaxInt64
	} else {
		end = r.end * m
	}

	return IntRange{
		start: start,
		end:   end,
		step:  r.step,
	}
}
