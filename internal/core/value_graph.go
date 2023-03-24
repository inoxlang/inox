package internal

import (
	"fmt"
	"reflect"

	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
)

type TraversalConfiguration struct {
	MaxDepth int
}

// Traverse traverses a graph of values starting from v.
// Only objects, records, dictionaries, lists, tuples and udata are considered source nodes, the other ones are sinks (leaves).
// A list of encountered source nodes is used to prevent cycling.
func Traverse(v Value, fn traverseVisitFn, config TraversalConfiguration) (terror error) {
	depth := 0
	return traverse(v, fn, config, map[uintptr]uintptr{}, depth)
}

type traverseVisitFn func(Value) (parse.TraversalAction, error)

func traverse(v Value, fn traverseVisitFn, config TraversalConfiguration, visited map[uintptr]uintptr, depth int) (terror error) {

	if depth > config.MaxDepth {
		panic(parse.StopTraversal)
	}

	if v == nil {
		return nil
	}

	defer func() {
		if depth == 0 {
			val := recover()
			if val == parse.StopTraversal {
				terror = nil
			} else if val != nil {
				panic(val)
			}
		}
	}()

	switch eV := v.(type) {
	case *Object, *Record:
		ptr := reflect.ValueOf(eV).Pointer()
		if _, ok := visited[ptr]; ok {
			return nil
		}

		visited[ptr] = 0
	case *List, *Tuple:
		ptr := reflect.ValueOf(eV).Pointer()

		if _, ok := visited[ptr]; ok {
			return nil
		}

		visited[ptr] = 0
	case *Dictionary:
		ptr := reflect.ValueOf(eV).Pointer()
		if _, ok := visited[ptr]; ok {
			return nil
		}

		visited[ptr] = 0
	case *UData:
		ptr := reflect.ValueOf(eV).Pointer()
		if _, ok := visited[ptr]; ok {
			return nil
		}
		visited[ptr] = 0
	}

	action, err := fn(v)
	if err != nil {
		return err
	}

	switch action {
	case parse.Continue:
		break
	case parse.Prune:
		return nil
	case parse.StopTraversal:
		panic(parse.StopTraversal)
	default:
		return fmt.Errorf("invalid traversal action: %v", action)
	}

	switch val := v.(type) {
	case *Object:
		for _, propV := range val.values {
			if err := traverse(propV, fn, config, visited, depth+1); err != nil {
				return err
			}
		}
	case *Record:
		for _, propV := range val.values {
			if err := traverse(propV, fn, config, visited, depth+1); err != nil {
				return err
			}
		}
	case *List:
		it := val.Iterator(nil, IteratorConfiguration{})
		for it.Next(nil) {
			elem := it.Value(nil)
			if err := traverse(elem, fn, config, visited, depth+1); err != nil {
				return err
			}
		}
	case *Tuple:
		for _, elem := range val.elements {
			if err := traverse(elem, fn, config, visited, depth+1); err != nil {
				return err
			}
		}
	case *Dictionary:
		for _, elem := range val.Entries {
			if err := traverse(elem, fn, config, visited, depth+1); err != nil {
				return err
			}
		}
		for _, key := range val.Keys {
			if err := traverse(key, fn, config, visited, depth+1); err != nil {
				return err
			}
		}
	case *UData:
		const (
			stackShrinkDivider       = 4
			minShrinkableStackLength = 10 * stackShrinkDivider
		)

		if err := traverse(val.Root, fn, config, visited, depth+1); err != nil {
			return err
		}

		stack := utils.ReversedSlice(val.HiearchyEntries)
		for len(stack) > 0 {
			entry := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			//if the stack is too small compared to its capacity we shrink the stack
			if len(stack) >= minShrinkableStackLength && len(stack) <= cap(stack)/stackShrinkDivider {
				newStack := make([]UDataHiearchyEntry, len(stack))
				copy(newStack, stack)
				stack = newStack
			}

			if err := traverse(entry.Value, fn, config, visited, depth+1); err != nil {
				return err
			}
			stack = append(stack, utils.ReversedSlice(entry.Children)...)
		}
	}

	return nil
}
