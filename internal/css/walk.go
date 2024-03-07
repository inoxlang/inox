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

type GraphTraversalAction int
type GraphTraversalOrder int

const (
	ContinueGraphTraversal GraphTraversalAction = iota
	PruneGraphTraversal
	StopGraphTraversal
)

type ImportNodeHandler = func(node Import, localFile *LocalFile, importerStack []*LocalFile, after bool) (GraphTraversalAction, error)

type ImportGraphWalkParams struct {
	Handle       ImportNodeHandler //pre-order
	PostHandle   ImportNodeHandler //post-order
	AllowRevisit bool              //if true the imports in a file included several other files can be visited several times.
}

// This functions performs a pre-order traversal on the import graph (depth first).
// postHandle is called on a node after all its descendants have been visited.
// Local files that contain no imports are visited with a zero import node.
func (g *ImportGraph) Walk(params ImportGraphWalkParams) (err error) {
	defer func() {
		v := recover()

		switch val := v.(type) {
		case error:
			err = fmt.Errorf("%s:%w", debug.Stack(), val)
		case nil:
		case GraphTraversalAction:
		default:
			panic(v)
		}
	}()

	importerStack := make([]*LocalFile, 0)
	var visited map[*LocalFile]struct{}
	if params.AllowRevisit {
		visited = map[*LocalFile]struct{}{}
	}

	for _, _import := range g.root.imports {
		g.walk(_import, g.root, &importerStack, visited, params.Handle, params.PostHandle)
	}

	if len(g.root.imports) == 0 {
		g.walk(Import{}, g.root, &importerStack, visited, params.Handle, params.PostHandle)
	}

	return
}

func (g *ImportGraph) walk(node Import, importer *LocalFile, ancestorChain *[]*LocalFile, visited map[*LocalFile]struct{}, fn, afterFn ImportNodeHandler) {

	if visited != nil {
		if _, ok := visited[importer]; ok {
			return
		}

		defer func() {
			//Several imports can be visited, therefore importer should be added to $visited AFTER the visits.
			visited[importer] = struct{}{}
		}()
	}

	if ancestorChain != nil {
		*ancestorChain = append((*ancestorChain), importer)
		defer func() {
			*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
		}()
	}

	if fn != nil {
		action, err := fn(node, importer, *ancestorChain, false)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopGraphTraversal:
			panic(StopGraphTraversal)
		case PruneGraphTraversal:
			return
		}
	}

	file, ok := node.LocalFile()
	if ok {
		imports := file.Imports()
		for _, _import := range imports {
			g.walk(_import, file, ancestorChain, visited, fn, afterFn)
		}

		if len(imports) == 0 {
			g.walk(Import{}, file, ancestorChain, visited, fn, afterFn)
		}
	}

	if afterFn != nil {
		action, err := afterFn(node, importer, *ancestorChain, true)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopGraphTraversal:
			panic(StopGraphTraversal)
		}
	}
}
