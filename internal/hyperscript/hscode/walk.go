package hscode

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

type NodeHandler = func(node Map, nodeType NodeType, parent Map, ancestorChain []Map, after bool) (AstTraversalAction, error)

// This functions performs a pre-order traversal on an AST (depth first).
// postHandle is called on a node after all its descendants have been visited.
func Walk(node Map, handle, postHandle NodeHandler) (err error) {
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

	ancestorChain := make([]Map, 0)
	walkAST(node, nil, &ancestorChain, handle, postHandle)
	return
}

func walkAST(node, parent Map, ancestorChain *[]Map, fn, afterFn NodeHandler) {

	if node == nil || !LooksLikeNode(node) {
		return
	}

	if ancestorChain != nil {
		*ancestorChain = append((*ancestorChain), parent)
		defer func() {
			*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
		}()
	}

	nodeType := NodeType(node["type"].(string))

	if fn != nil {
		action, err := fn(node, nodeType, parent, *ancestorChain, false)

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

	for _, value := range node {
		if LooksLikeNode(value) {
			child := value.(Map)
			walkAST(child, node, ancestorChain, fn, afterFn)
		}
		//TODO: are there there also node lists ?
	}

	if afterFn != nil {
		action, err := afterFn(node, nodeType, parent, *ancestorChain, true)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopAstTraversal:
			panic(StopAstTraversal)
		}
	}
}
