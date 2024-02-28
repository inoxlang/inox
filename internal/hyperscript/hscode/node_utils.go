package hscode

import (
	"fmt"
	"reflect"
	"runtime/debug"
)

type TraversalAction int
type TraversalOrder int

const (
	ContinueTraversal TraversalAction = iota
	Prune
	StopTraversal
)

type NodeHandler = func(node Node, parent Node, ancestorChain []Node, after bool) (TraversalAction, error)

// This functions performs a pre-order traversal on an AST (depth first).
// postHandle is called on a node after all its descendants have been visited.
func Walk(node Node, handle, postHandle NodeHandler) (err error) {
	defer func() {
		v := recover()

		switch val := v.(type) {
		case error:
			err = fmt.Errorf("%s:%w", debug.Stack(), val)
		case nil:
		case TraversalAction:
		default:
			panic(v)
		}
	}()

	ancestorChain := make([]Node, 0)
	walk(node, Node{}, &ancestorChain, handle, postHandle)
	return
}

func walk(node, parent Node, ancestorChain *[]Node, fn, afterFn NodeHandler) {

	if reflect.ValueOf(node).IsZero() {
		return
	}

	if ancestorChain != nil && !reflect.ValueOf(node).IsZero() {
		*ancestorChain = append((*ancestorChain), parent)
		defer func() {
			*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
		}()
	}

	if fn != nil {
		action, err := fn(node, parent, *ancestorChain, false)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopTraversal:
			panic(StopTraversal)
		case Prune:
			return
		}
	}

	walk(*node.Root, node, ancestorChain, fn, afterFn)
	walk(*node.Expression, node, ancestorChain, fn, afterFn)
	walk(*node.Expr, node, ancestorChain, fn, afterFn)
	walk(*node.Attribute, node, ancestorChain, fn, afterFn)
	walk(*node.From, node, ancestorChain, fn, afterFn)
	walk(*node.To, node, ancestorChain, fn, afterFn)
	walk(*node.FirstIndex, node, ancestorChain, fn, afterFn)
	walk(*node.SecondIndex, node, ancestorChain, fn, afterFn)
	walk(*node.Lhs, node, ancestorChain, fn, afterFn)
	walk(*node.Rhs, node, ancestorChain, fn, afterFn)
	walk(*node.Value, node, ancestorChain, fn, afterFn)
	walk(*node.Target, node, ancestorChain, fn, afterFn)
	walk(*node.AttributeRef, node, ancestorChain, fn, afterFn)
	walk(*node.InElt, node, ancestorChain, fn, afterFn)
	walk(*node.WithinElt, node, ancestorChain, fn, afterFn)

	for _, n := range node.Children {
		walk(n, node, ancestorChain, fn, afterFn)
	}
	for _, n := range node.Args {
		walk(n, node, ancestorChain, fn, afterFn)
	}
	for _, n := range node.ArgExressions {
		walk(n, node, ancestorChain, fn, afterFn)
	}
	for _, n := range node.ArgExpressions {
		walk(n, node, ancestorChain, fn, afterFn)
	}
	for _, n := range node.Values {
		walk(n, node, ancestorChain, fn, afterFn)
	}
	for _, n := range node.Features {
		walk(n, node, ancestorChain, fn, afterFn)
	}

	if afterFn != nil {
		action, err := afterFn(node, parent, *ancestorChain, true)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopTraversal:
			panic(StopTraversal)
		}
	}
}

// func getNodeAtCursor(cursorIndex int32, chunk Node) (nodeAtCursor, _parent Node, ancestors []Node) {
// 	//search node at cursor
// 	Walk(chunk, func(node, parent Node, ancestorChain []Node, _ bool) (TraversalAction, error) {

// 		//if the cursor is not in the node's span we don't check the descendants of the node
// 		if span.Start > cursorIndex || span.End < cursorIndex {
// 			return Prune, nil
// 		}

// 		if nodeAtCursor == (Node{}) || node.Base().IncludedIn(nodeAtCursor) {
// 			nodeAtCursor = node
// 		}

// 		return ContinueTraversal, nil
// 	}, nil)

// 	return
// }
