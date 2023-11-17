package core

import (
	"fmt"
	"slices"
	"sort"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

func init() {
	RegisterSymbolicGoFunctions([]any{
		Sort, func(ctx *symbolic.Context, list *symbolic.List, order *symbolic.Identifier) *symbolic.List {
			if list.HasKnownLen() && list.KnownLen() == 0 {
				return list
			}

			orderOk := true
			if !order.HasConcreteName() {
				orderOk = false
				ctx.AddSymbolicGoFunctionError("invalid order identifier")
			}

			switch list.IteratorElementValue().(type) {
			case *symbolic.Int:

				if orderOk {
					switch order.Name() {
					case "asc", "desc":
					default:
						ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for integers, use #asc or #desc", order.Name())
					}
				}

			case symbolic.StringLike:

				if orderOk {
					switch order.Name() {
					case "lex", "revlex":
					default:
						ctx.AddFormattedSymbolicGoFunctionError("invalid order '%s' for strings, use #lex or #revlex", order.Name())
					}
				}

			default:
				ctx.AddSymbolicGoFunctionError("list should contains only integers or only strings")
			}

			return list
		},
	})

}

// TODO: support any iterable
func Sort(ctx *Context, list *List, order Identifier) *List {
	const ERR_PREFIX = "sort:"

	if list.Len() <= 1 {
		return NewWrappedValueListFrom(list.GetOrBuildElements(ctx))
	}

	elements := list.GetOrBuildElements(ctx)

	switch firstElem := elements[0].(type) {
	case StringLike:
		strings := make([]string, list.Len())
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
		ints := make([]Int, list.Len())
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
