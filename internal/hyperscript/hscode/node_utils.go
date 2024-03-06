package hscode

func GetTokenAtCursor(cursorIndex int32, tokens []Token) (Token, bool) {
	for _, t := range tokens {
		if cursorIndex >= t.Start && cursorIndex <= t.End {
			return t, true
		}
	}
	return Token{}, false
}

// GetClosestTokenOnCursorLeftSide returns the closest token on the left side of the cursor.
// If the cursor is 'inside' a token, this token is returned.
func GetClosestTokenOnCursorLeftSide(cursorIndex int32, tokens []Token) (Token, bool) {

	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]

		if t.End <= cursorIndex || (t.Start < cursorIndex && cursorIndex <= t.End) {
			return t, true
		}
	}
	return Token{}, false
}

// type TraversalAction int
// type TraversalOrder int

// const (
// 	ContinueTraversal TraversalAction = iota
// 	Prune
// 	StopTraversal
// )

// type NodeHandler = func(node Node, parent Node, ancestorChain []Node, after bool) (TraversalAction, error)

// // This functions performs a pre-order traversal on an AST (depth first).
// // postHandle is called on a node after all its descendants have been visited.
// func Walk(node Node, handle, postHandle NodeHandler) (err error) {
// 	defer func() {
// 		v := recover()

// 		switch val := v.(type) {
// 		case error:
// 			err = fmt.Errorf("%s:%w", debug.Stack(), val)
// 		case nil:
// 		case TraversalAction:
// 		default:
// 			panic(v)
// 		}
// 	}()

// 	ancestorChain := make([]Node, 0)
// 	walk(node, Node{}, &ancestorChain, handle, postHandle)
// 	return
// }

// func walkIfNotNil(node *Node, parent Node, ancestorChain *[]Node, fn, afterFn NodeHandler) {
// 	if node != nil {
// 		walk(*node, parent, ancestorChain, fn, afterFn)
// 	}
// }

// func walk(node, parent Node, ancestorChain *[]Node, fn, afterFn NodeHandler) {

// 	if reflect.ValueOf(node).IsZero() {
// 		return
// 	}

// 	if ancestorChain != nil && !reflect.ValueOf(node).IsZero() {
// 		*ancestorChain = append((*ancestorChain), parent)
// 		defer func() {
// 			*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
// 		}()
// 	}

// 	if fn != nil {
// 		action, err := fn(node, parent, *ancestorChain, false)

// 		if err != nil {
// 			panic(err)
// 		}

// 		switch action {
// 		case StopTraversal:
// 			panic(StopTraversal)
// 		case Prune:
// 			return
// 		}
// 	}

// 	walkIfNotNil(node.Root, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Expression, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Expr, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Attribute, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.From, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.To, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.FirstIndex, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.SecondIndex, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Lhs, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Rhs, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Value, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.Target, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.AttributeRef, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.InElt, node, ancestorChain, fn, afterFn)
// 	walkIfNotNil(node.WithinElt, node, ancestorChain, fn, afterFn)

// 	for _, n := range node.Children {
// 		walk(n, node, ancestorChain, fn, afterFn)
// 	}
// 	for _, n := range node.Args {
// 		walk(n, node, ancestorChain, fn, afterFn)
// 	}
// 	for _, n := range node.ArgExpressions {
// 		walk(n, node, ancestorChain, fn, afterFn)
// 	}
// 	for _, n := range node.Values {
// 		walk(n, node, ancestorChain, fn, afterFn)
// 	}
// 	for _, n := range node.Features {
// 		walk(n, node, ancestorChain, fn, afterFn)
// 	}

// 	if afterFn != nil {
// 		action, err := afterFn(node, parent, *ancestorChain, true)

// 		if err != nil {
// 			panic(err)
// 		}

// 		switch action {
// 		case StopTraversal:
// 			panic(StopTraversal)
// 		}
// 	}
// }

// func GetNodeAtCursor(cursorIndex int32, n Node) (nodeAtCursor, _parent Node, ancestors []Node) {
// 	Walk(n, func(node, parent Node, ancestorChain []Node, _ bool) (TraversalAction, error) {

// 		if node.IsZero() {
// 			return ContinueTraversal, nil
// 		}

// 		//if the cursor is not in the node's span we don't check the descendants of the node
// 		if node.StartPos() > cursorIndex || node.EndPos() < cursorIndex {
// 			return Prune, nil
// 		}

// 		if nodeAtCursor.IsZero() || node.IncludedIn(nodeAtCursor) {
// 			nodeAtCursor = node
// 		}

// 		return ContinueTraversal, nil
// 	}, nil)

// 	return
// }
