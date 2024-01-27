package core

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_                           = []SortableByNestedValue{(*ValueList)(nil) /*TODO: (*StringList)(nil)*/}
	ErrUnsupportedNestedValue   = errors.New("unsupported nested value")
	ErrUnsupportedOrder         = errors.New("unsupported order")
	ErrNotSortableByNestedValue = errors.New("not sortable by nested value")
	ErrInvalidOrderIdentifier   = errors.New("invalid order identifier")
)

type Order = symbolic.Order

type SortableByNestedValue interface {
	SortByNestedValue(ctx *Context, path ValuePath, order Order) error
}

func (l *List) Sorted(ctx *Context, orderIdent Identifier) *List {
	const ERR_PREFIX = "sorted:"

	order, ok := symbolic.OrderFromString(orderIdent.UnderlyingString())
	if !ok {
		panic(ErrInvalidOrderIdentifier)
	}

	if l.Len() <= 1 {
		return NewWrappedValueListFrom(l.GetOrBuildElements(ctx))
	}

	elements := l.GetOrBuildElements(ctx)

	switch firstElem := elements[0].(type) {
	case StringLike:
		strings := make([]string, l.Len())
		for i, e := range elements {
			s, ok := e.(StringLike)
			if !ok {
				panic(fmt.Errorf("%s first element is string-like but at least one another element is not", ERR_PREFIX))
			}
			strings[i] = s.GetOrBuildString()
		}

		switch order {
		case symbolic.LexicographicOrder:
			sort.Strings(strings)
		case symbolic.ReverseLexicographicOrder:
			sort.Strings(strings)
			slices.Reverse(strings)
		default:
			panic(fmt.Errorf("%s unsupported order for strings: '%s'", ERR_PREFIX, orderIdent))
		}

		return NewWrappedStringListFrom(utils.MapSlice(strings, func(s string) StringLike {
			return String(s)
		}))
	case Int:
		ints := make([]Int, l.Len())
		for i, e := range elements {
			integer, ok := e.(Int)
			if !ok {
				panic(fmt.Errorf("%s first element is an integer but at least one another element is not", ERR_PREFIX))
			}
			ints[i] = integer
		}

		switch orderIdent {
		case "asc":
			sort.Slice(ints, func(i, j int) bool {
				return ints[i] < ints[j]
			})
		case "desc":
			sort.Slice(ints, func(i, j int) bool {
				return ints[i] < ints[j]
			})
			slices.Reverse(ints)
		default:
			panic(fmt.Errorf("%s unsupported order for integers: '%s'", ERR_PREFIX, orderIdent))
		}

		return NewWrappedIntListFrom(ints)
	case Float:
		floats := make([]Float, l.Len())
		for i, e := range elements {
			float, ok := e.(Float)
			if !ok {
				panic(fmt.Errorf("%s first element is a float but at least one another element is not", ERR_PREFIX))
			}
			floats[i] = float
		}

		switch orderIdent {
		case "asc":
			sort.Slice(floats, func(i, j int) bool {
				return floats[i] < floats[j]
			})
		case "desc":
			sort.Slice(floats, func(i, j int) bool {
				return floats[i] < floats[j]
			})
			slices.Reverse(floats)
		default:
			panic(fmt.Errorf("%s unsupported order for integers: '%s'", ERR_PREFIX, orderIdent))
		}

		return NewWrappedFloatListFrom(floats)
	default:
		panic(fmt.Errorf("%s not sortable: first element is a(n) %T", ERR_PREFIX, firstElem))
	}

}

func (l *List) SortBy(ctx *Context, path ValuePath, orderIdent Identifier) {
	const ERR_PREFIX = "sort_by:"

	order, ok := symbolic.OrderFromString(orderIdent.UnderlyingString())
	if !ok {
		panic(ErrInvalidOrderIdentifier)
	}

	if l.Len() <= 1 {
		return
	}

	sortable, ok := l.underlyingList.(SortableByNestedValue)
	if !ok {
		panic(ErrNotSortableByNestedValue)
	}

	err := sortable.SortByNestedValue(ctx, path, order)
	if err != nil {
		panic(err)
	}
}

func (l *ValueList) SortByNestedValue(ctx *Context, path ValuePath, order Order) error {
	if l.Len() <= 1 {
		return nil
	}

	firstNestedValue := path.GetFrom(ctx, l.elements[0])

	switch firstNestedValue.(type) {
	case Int:
		type intItem struct {
			integer Int
			index   int
		}

		items := make([]intItem, len(l.elements))

		for i, e := range l.elements {
			items[i] = intItem{
				integer: path.GetFrom(ctx, e).(Int),
				index:   i,
			}
		}

		switch order {
		case symbolic.AscendingOrder:
			slices.SortFunc(items, func(a, b intItem) int {
				return _intCompare(a.integer, b.integer)
			})
		case symbolic.DescendingOrder:
			slices.SortFunc(items, func(a, b intItem) int {
				return _negatedIntCompare(a.integer, b.integer)
			})
		default:
			return ErrUnsupportedOrder
		}

		elements := slices.Clone(l.elements)
		for i, item := range items {
			l.elements[i] = elements[item.index]
		}
		return nil
	case Float:
		type floatItem struct {
			float Float
			index int
		}

		items := make([]floatItem, len(l.elements))

		for i, e := range l.elements {
			items[i] = floatItem{
				float: path.GetFrom(ctx, e).(Float),
				index: i,
			}
		}
		comparable := true

		switch order {
		case symbolic.AscendingOrder:
			slices.SortFunc(items, func(a, b floatItem) int {
				res, ok := float64Compare(a.float, b.float)
				if !ok {
					comparable = false
				}
				return res
			})
		case symbolic.DescendingOrder:
			slices.SortFunc(items, func(a, b floatItem) int {
				res, ok := negatedFloat64Compare(a.float, b.float)
				if !ok {
					comparable = false
				}
				return res
			})
		default:
			return ErrUnsupportedOrder
		}

		if !comparable {
			return fmt.Errorf("failed to compare some elements: %w", ErrNotComparable)
		}

		elements := slices.Clone(l.elements)
		for i, item := range items {
			l.elements[i] = elements[item.index]
		}
		return nil
	case StringLike:
		return fmt.Errorf("sorting by a nested string is not supported yet")
	default:
		return ErrUnsupportedNestedValue
	}

}
