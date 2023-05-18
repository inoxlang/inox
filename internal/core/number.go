package internal

import (
	"errors"
	"fmt"
	"math"
)

var (
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

func intDiv(l, r Int) (Value, error) {
	if r == 0 {
		return nil, ErrIntDivisionByZero
	}
	if l == math.MinInt64 && r == -1 {
		return nil, ErrIntOverflow
	}
	return l / r, nil
}

type IntRange struct {
	unknownStart bool //if true .Start depends on the context (not *Context)
	inclusiveEnd bool
	Start        int64
	End          int64
	Step         int64 //only 1 supported for now
}

func NewIncludedEndIntRange(start, end int64) IntRange {
	if end < start {
		panic(fmt.Errorf("failed to create int pattern, end < start"))
	}
	return IntRange{inclusiveEnd: true, Start: start, End: end, Step: 1}
}

func (r IntRange) Includes(ctx *Context, i Int) bool {
	if r.unknownStart {
		panic(errors.New("range has unknown start"))
	}

	return r.Start <= int64(i) && int64(i) <= r.InclusiveEnd()
}

func (r IntRange) At(ctx *Context, i int) Value {
	if i >= r.Len() {
		panic(ErrIndexOutOfRange)
	}
	return Int(i + int(r.Start))
}

func (r IntRange) Len() int {
	if r.unknownStart {
		panic(errors.New("range has unknown start"))
	}

	return r.len(r.Start)
}

func (r IntRange) KnownStart() int64 {
	if r.unknownStart {
		panic(errors.New("range has unknown start"))
	}
	return r.Start
}

func (r IntRange) InclusiveEnd() int64 {
	if r.inclusiveEnd {
		return r.End
	}
	return r.End - 1
}

func (r IntRange) len(min int64) int {
	start := r.Start
	if r.unknownStart {
		start = min
	}
	length := r.End - start
	if r.inclusiveEnd {
		length++
	}

	return int(length)
}

// clampedAdd adds .Start(s) togethers & .End(s) toghethers, on overflow the corresponding field is set to math.MaxInt64.
func (r IntRange) clampedAdd(other IntRange) IntRange {

	if r.unknownStart || other.unknownStart {
		panic(errors.New("cannot clamp add int ranges with at least one unknown start"))
	}

	if r.Step != 1 || other.Step != 1 {
		panic(errors.New("cannot clamp add other range: only ranges with a step of 1 are supported"))
	}

	if r.Start < 0 || other.Start < 0 {
		panic(errors.New("cannot clamp add other range: only positive start ranges are supported"))
	}

	newRange := IntRange{
		Start:        r.Start,
		End:          r.End,
		inclusiveEnd: r.inclusiveEnd,
		Step:         1,
	}

	if other.Start >= math.MaxInt64-r.Start {
		newRange.Start = math.MaxInt64
		newRange.End = math.MaxInt64
		newRange.inclusiveEnd = true
		return newRange
	} else {
		newRange.Start = r.Start + other.Start
	}

	if other.End >= math.MaxInt64-r.End {
		newRange.End = math.MaxInt64
		newRange.inclusiveEnd = true
	} else {
		newRange.End = r.End + other.InclusiveEnd()
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

	minOverflow := r.Start != 0 && n >= math.MaxInt64/r.Start
	maxOverflow := r.End != 0 && m >= math.MaxInt64/r.End

	if !clamped && (minOverflow || maxOverflow) {
		inclusiveEnd := r.InclusiveEnd()

		if r.unknownStart {
			panic(fmt.Errorf("cannot multiply integer range ..%d by %d %d", inclusiveEnd, n, m))
		}
		panic(fmt.Errorf("cannot multiply integer range %d..%d by %d %d", r.Start, inclusiveEnd, n, m))
	}

	start := int64(0)
	end := int64(0)

	if minOverflow {
		start = math.MaxInt64
	} else {
		start = r.Start * n
	}

	if maxOverflow {
		end = math.MaxInt64
	} else {
		end = r.End * m
	}

	return IntRange{
		inclusiveEnd: r.inclusiveEnd,
		Start:        start,
		End:          end,
		Step:         r.Step,
	}
}
