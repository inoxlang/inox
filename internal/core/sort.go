package core

import (
	"fmt"
	"slices"
	"sort"

	"github.com/inoxlang/inox/internal/utils"
)

func (l *List) Sorted(ctx *Context, order Identifier) *List {
	const ERR_PREFIX = "sorted:"

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
		case "lex":
			sort.Strings(strings)
		case "revlex":
			sort.Strings(strings)
			slices.Reverse(strings)
		default:
			panic(fmt.Errorf("%s unsupported order for strings: '%s'", ERR_PREFIX, order))
		}

		return NewWrappedStringListFrom(utils.MapSlice(strings, func(s string) StringLike {
			return Str(s)
		}))
	case Int:
		ints := make([]Int, l.Len())
		for i, e := range elements {
			integer, ok := e.(Int)
			if !ok {
				panic(fmt.Errorf("%s sort first element is an integer but at least one another element is not", ERR_PREFIX))
			}
			ints[i] = integer
		}

		switch order {
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
			panic(fmt.Errorf("%s unsupported order for integers: '%s'", ERR_PREFIX, order))
		}

		return NewWrappedIntListFrom(ints)
	default:
		panic(fmt.Errorf("%s not sortable: first element is a(n) %T", ERR_PREFIX, firstElem))
	}

}
