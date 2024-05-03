package hscode

import (
	"fmt"
	"runtime/debug"

	"github.com/inoxlang/inox/internal/utils"
)

type AstTraversalAction int
type AstTraversalOrder int

const (
	ContinueAstTraversal AstTraversalAction = iota
	PruneAstTraversal
	StopAstTraversal
)

type NodeHandler = func(node JSONMap, nodeType NodeType, parent JSONMap, parentType NodeType, ancestorChain []JSONMap, after bool) (AstTraversalAction, error)

// This functions performs a pre-order traversal on an AST (depth first).
// postHandle is called on a node after all its descendants have been visited.
func Walk(node JSONMap, handle, postHandle NodeHandler) (err error) {
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

	ancestorChain := make([]JSONMap, 0)
	walkAST(node, nil, "", &ancestorChain, handle, postHandle)
	return
}

func walkAST(node, parent JSONMap, parentType NodeType, ancestorChain *[]JSONMap, fn, afterFn NodeHandler) {

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
		action, err := fn(node, nodeType, parent, parentType, *ancestorChain, false)

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
			child := value.(JSONMap)
			walkAST(child, node, nodeType, ancestorChain, fn, afterFn)
		} else if slice, ok := value.([]any); ok && utils.All(slice, LooksLikeNode) {
			for _, elem := range slice {
				child := elem.(JSONMap)
				walkAST(child, node, nodeType, ancestorChain, fn, afterFn)
			}
		}
		//TODO: are there there also node lists ?
	}

	if afterFn != nil {
		action, err := afterFn(node, nodeType, parent, parentType, *ancestorChain, true)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopAstTraversal:
			panic(StopAstTraversal)
		}
	}
}
