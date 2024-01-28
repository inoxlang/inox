package core

import (
	"fmt"
	"time"
)

var (
	_ = []IPseudoAdd{Duration(0), DateTime{}}
	_ = []IPseudoSub{Duration(0), DateTime{}}
)

type IPseudoAdd interface {
	Value
	Add(right Value) (Value, error)
}

type IPseudoSub interface {
	Value
	Sub(right Value) (Value, error)
}

func (left Duration) Add(right Value) (Value, error) {
	if err := left.Validate(); err != nil {
		return nil, err
	}

	switch right := right.(type) {
	case Duration:
		if err := right.Validate(); err != nil {
			return nil, err
		}

		return int64QuantityAdd(left, right, true)
	case DateTime:
		result := right.AsGoTime().Add(time.Duration(left))
		return DateTime(result), nil
	default:
		return nil, fmt.Errorf("unexpected right operand type: %T", right)
	}
}

func (left Duration) Sub(right Value) (Value, error) {
	if err := left.Validate(); err != nil {
		return nil, err
	}

	switch right := right.(type) {
	case Duration:
		if err := right.Validate(); err != nil {
			return nil, err
		}

		return int64QuantitySub(left, right, true)
	case DateTime:
		result := right.AsGoTime().Add(-time.Duration(left))
		return DateTime(result), nil
	default:
		return nil, fmt.Errorf("unexpected right operand type: %T", right)
	}
}

func (left DateTime) Add(right Value) (Value, error) {
	switch right := right.(type) {
	case Duration:
		if err := right.Validate(); err != nil {
			return nil, err
		}

		result := left.AsGoTime().Add(time.Duration(right))
		return DateTime(result), nil
	default:
		return nil, fmt.Errorf("unexpected right operand type: %T", right)
	}
}

func (left DateTime) Sub(right Value) (Value, error) {
	switch right := right.(type) {
	case Duration:
		if err := right.Validate(); err != nil {
			return nil, err
		}

		result := left.AsGoTime().Add(-time.Duration(right))
		return DateTime(result), nil
	default:
		return nil, fmt.Errorf("unexpected right operand type: %T", right)
	}
}

//Date.Sub is not implemented because Duration cannot be negative.
