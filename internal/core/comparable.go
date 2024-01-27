package core

import (
	"errors"
	"math"
	"time"

	"github.com/maruel/natural"
	"golang.org/x/exp/constraints"
)

var (
	ErrNotComparable = errors.New("not comparable")
)

type Comparable interface {
	Value
	//Compare should return (0, false) if the values are not comparable. Otherwise it sould return true and one of the following:
	// (-1) a < b
	// (0) a == b
	// (1) a > b
	// The Equal method of the implementations should be consistent with Compare.
	Compare(b Value) (result int, comparable bool)
}

func equalComparable(a Comparable, b Value) bool {
	result, comparable := a.Compare(b)
	return comparable && result == 0
}

func (i Int) Compare(other Value) (result int, comparable bool) {
	return intCompare(i, other)
}

func (f Float) Compare(other Value) (result int, comparable bool) {
	return float64Compare(f, other)
}

func (b Byte) Compare(other Value) (result int, comparable bool) {
	return intCompare(b, other)
}

func (r Rune) Compare(other Value) (result int, comparable bool) {
	return intCompare(r, other)
}

func (c ByteCount) Compare(other Value) (result int, comparable bool) {
	return intCompare(c, other)
}

func (c LineCount) Compare(other Value) (result int, comparable bool) {
	return intCompare(c, other)
}

func (c RuneCount) Compare(other Value) (result int, comparable bool) {
	return intCompare(c, other)
}

func (f Frequency) Compare(other Value) (result int, comparable bool) {
	return float64Compare(f, other)
}

func (r ByteRate) Compare(other Value) (result int, comparable bool) {
	return intCompare(r, other)
}

func (d Duration) Compare(other Value) (result int, comparable bool) {
	return intCompare(d, other)
}

func (p Port) Compare(other Value) (result int, comparable bool) {
	otherPort, ok := other.(Port)
	if !ok {
		//not comparable
		return
	}
	comparable = true
	if p.Number < otherPort.Number {
		result = -1
		return
	}
	if p.Number == otherPort.Number {
		return //0
	}
	result = 1
	return
}

func (s String) Compare(other Value) (result int, comparable bool) {
	return stringCompareNaturalSortOrder(s, other)
}

func (y Year) Compare(other Value) (result int, comparable bool) {
	otherYear, ok := other.(Year)
	if !ok {
		//not comparable
		return
	}
	comparable = true

	if err := y.Validate(); err != nil {
		panic(err)
	}
	if err := otherYear.Validate(); err != nil {
		panic(err)
	}

	goTime := time.Time(y)
	otherGoTime := time.Time(otherYear)
	result = goTimeCompare(goTime, otherGoTime)
	return
}

func (d Date) Compare(other Value) (result int, comparable bool) {
	otherDate, ok := other.(Date)
	if !ok {
		//not comparable
		return
	}
	comparable = true

	if err := d.Validate(); err != nil {
		panic(err)
	}
	if err := otherDate.Validate(); err != nil {
		panic(err)
	}

	goTime := time.Time(d)
	otherGoTime := time.Time(otherDate)
	result = goTimeCompare(goTime, otherGoTime)
	return
}

func (dt DateTime) Compare(other Value) (result int, comparable bool) {
	otherDatetime, ok := other.(DateTime)
	if !ok {
		//not comparable
		return
	}
	comparable = true

	goTime := time.Time(dt)
	otherGoTime := time.Time(otherDatetime)
	result = goTimeCompare(goTime, otherGoTime)
	return
}

func (id ULID) Compare(other Value) (result int, comparable bool) {
	otherULID, ok := other.(ULID)
	if !ok {
		//not comparable
		return
	}
	comparable = true
	result = id.libValue().Compare(otherULID.libValue())
	return
}

func intCompare[I constraints.Integer](i I, other Value) (result int, comparable bool) {
	otherInt, ok := other.(I)
	if !ok {
		//not comparable
		return
	}
	comparable = true
	result = _intCompare(i, otherInt)
	return
}

func _intCompare[I constraints.Integer](i I, other I) int {
	if i < other {
		return -1
	}
	if i == other {
		return 0
	}
	return 1
}

func _negatedIntCompare[I constraints.Integer](i I, other I) int {
	if i < other {
		return 1
	}
	if i == other {
		return 0
	}
	return -1
}

func float64Compare[F ~float64](f F, other Value) (result int, comparable bool) {
	otherFloat, ok := other.(F)
	if !ok ||
		math.IsNaN(float64(f)) ||
		math.IsNaN(float64(otherFloat)) ||
		math.IsInf(float64(f), 0) ||
		math.IsInf(float64(otherFloat), 0) {
		//not comparable
		return
	}
	comparable = true
	if f < otherFloat {
		result = -1
		return
	}
	if f == otherFloat {
		return //0
	}
	result = 1
	return
}

func negatedFloat64Compare[F ~float64](f F, other Value) (result int, comparable bool) {
	otherFloat, ok := other.(F)
	if !ok ||
		math.IsNaN(float64(f)) ||
		math.IsNaN(float64(otherFloat)) ||
		math.IsInf(float64(f), 0) ||
		math.IsInf(float64(otherFloat), 0) {
		//not comparable
		return
	}
	comparable = true
	if f < otherFloat {
		result = 1
		return
	}
	if f == otherFloat {
		return //0
	}
	result = -1
	return
}

// https://en.wikipedia.org/wiki/Natural_sort_order
func stringCompareNaturalSortOrder[S ~string](s S, other Value) (result int, comparable bool) {
	otherString, ok := other.(S)
	if !ok {
		//not comparable
		return
	}
	comparable = true

	if s == otherString {
		return //0
	}

	if natural.Less(string(s), string(otherString)) {
		result = -1
		return
	}

	result = 1
	return
}

func goTimeCompare(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}
