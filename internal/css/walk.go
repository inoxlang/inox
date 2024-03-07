package css

import (
	"fmt"
	"runtime/debug"
)

type AstTraversalAction int
type AstTraversalOrder int

const (
	ContinueAstTraversal AstTraversalAction = iota
	PruneAstTraversal
	StopAstTraversal
)

type NodeHandler = func(node Node, parent Node, ancestorChain []Node, after bool) (AstTraversalAction, error)

// This functions performs a pre-order traversal on an AST (depth first).
// postHandle is called on a node after all its descendants have been visited.
func WalkAST(node Node, handle, postHandle NodeHandler) (err error) {
	defer func() {
		v := recover()

		switch val := v.(type) {
		case error:
			err = fmt.Errorf("%s:%w", debug.Stack(), val)
		case nil:
		case AstTraversalAction:
		default:
			panic(v)
		}
	}()

	ancestorChain := make([]Node, 0)
	walkAST(node, Node{}, &ancestorChain, handle, postHandle)
	return
}

func walkAST(node, parent Node, ancestorChain *[]Node, fn, afterFn NodeHandler) {

	if node.IsZero() {
		return
	}

	if ancestorChain != nil {
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
		case StopAstTraversal:
			panic(StopAstTraversal)
		case PruneAstTraversal:
			return
		}
	}

	for _, child := range node.Children {
		walkAST(child, node, ancestorChain, fn, afterFn)
	}

	if afterFn != nil {
		action, err := afterFn(node, parent, *ancestorChain, true)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopAstTraversal:
			panic(StopAstTraversal)
		}
	}
}
