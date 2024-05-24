package ast

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/sourcecode"
)

// NodeIsStringLiteral returns true if and only if node is of one of the following types:
// *QuotedStringLiteral, *UnquotedStringLiteral, *StringTemplateLiteral, *MultilineStringLiteral
func NodeIsStringLiteral(node Node) bool {
	switch node.(type) {
	case *DoubleQuotedStringLiteral, *UnquotedStringLiteral, *StringTemplateLiteral, *MultilineStringLiteral:
		return true
	}
	return false
}

func NodeIsSimpleValueLiteral(node Node) bool {
	_, ok := node.(SimpleValueLiteral)
	return ok
}

func NodeIsPattern(node Node) bool {
	switch node.(type) {
	case *PatternCallExpression,
		*ListPatternLiteral, *TuplePatternLiteral,
		*ObjectPatternLiteral, *RecordPatternLiteral,
		*DictionaryPatternLiteral,
		*PatternIdentifierLiteral, *PatternNamespaceMemberExpression,
		*ComplexStringPatternPiece, //not 100% correct since it can be included in another *ComplexStringPatternPiece,
		*PatternConversionExpression,
		*PatternUnion,
		*PathPatternExpression, *AbsolutePathPatternLiteral, *RelativePathPatternLiteral,
		*URLPatternLiteral, *HostPatternLiteral, *OptionalPatternExpression,
		*OptionPatternLiteral, *FunctionPatternExpression, *NamedSegmentPathPatternLiteral, *ReadonlyPatternExpression,
		*RegularExpressionLiteral:
		return true
	}
	return false
}

func NodeIs[T Node](node Node, typ T) bool {
	return reflect.TypeOf(typ) == reflect.TypeOf(node)
}

// shifts the span of all nodes in node by offset
func ShiftNodeSpans(node Node, offset int32) {
	ancestorChain := make([]Node, 0)

	walk(node, nil, &ancestorChain, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
		node.BasePtr().Span.Start += offset
		node.BasePtr().Span.End += offset

		// tokens := node.BasePtr().Tokens
		// for i, token := range tokens {
		// 	token.Span.Start += offset
		// 	token.Span.End += offset
		// 	tokens[i] = token
		// }
		return ContinueTraversal, nil
	}, nil)
}

func CountNodes(n Node) (count int) {
	Walk(n, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		count += 1
		return ContinueTraversal, nil
	}, nil)

	return
}

func FindNodeWithSpan(root Node, searchedNodeSpan sourcecode.NodeSpan) (n Node, found bool) {
	Walk(root, func(node, _, _ Node, _ []Node, _ bool) (TraversalAction, error) {

		nodeSpan := node.Base().Span
		if searchedNodeSpan.End < nodeSpan.Start || searchedNodeSpan.Start >= nodeSpan.End {
			return Prune, nil
		}

		if searchedNodeSpan == nodeSpan {
			n = node
			found = true
			return StopTraversal, nil
		}
		return ContinueTraversal, nil
	}, nil)

	return
}

func FindNodes[T Node](root Node, typ T, handle func(n T) bool) []T {
	n, _ := FindNodesAndChains(root, typ, handle)
	return n
}

func FindNodesAndChains[T Node](root Node, typ T, handle func(n T) bool) ([]T, [][]Node) {
	searchedType := reflect.TypeOf(typ)
	var found []T
	var ancestors [][]Node

	Walk(root, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if reflect.TypeOf(node) == searchedType {
			if handle == nil || handle(node.(T)) {
				found = append(found, node.(T))
				ancestors = append(ancestors, slices.Clone(ancestorChain))
			}
		}
		return ContinueTraversal, nil
	}, nil)

	return found, ancestors
}

// FindNodeAndChain walks over an AST node and returns the first node of type $typ for which $handle returns true.
// If $handle is nil only the type is checked.
func FindNode[T Node](root Node, typ T, handle func(n T, isFirstFound bool, ancestors []Node) bool) T {
	n, _ := FindNodeAndChain(root, typ, handle)
	return n
}

// FindNodeAndChain walks over an AST node and returns the first node of type $typ.
func FindFirstNode[T Node](root Node, typ T) T {
	n, _ := FindNodeAndChain(root, typ, func(n T, isUnique bool, ancestors []Node) bool {
		return isUnique
	})
	return n
}

// FindNodeAndChain walks over an AST node and returns the first node of type $typ (and its ancestors) for which $handle returns true.
// If $handle is nil only the type is checked.
func FindNodeAndChain[T Node](root Node, typ T, handle func(n T, isFirstFound bool, ancestors []Node) bool) (T, []Node) {
	searchedType := reflect.TypeOf(typ)
	isUnique := true

	var found T
	var _ancestorChain []Node

	Walk(root, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if reflect.TypeOf(node) == searchedType {
			if handle == nil || handle(node.(T), isUnique, _ancestorChain) {
				found = node.(T)
				isUnique = false
				_ancestorChain = ancestorChain
			}
		}
		return ContinueTraversal, nil
	}, nil)

	return found, _ancestorChain
}

// FindClosest searches for an ancestor node of type typ starting from the parent node (last ancestor).
func FindClosest[T Node](ancestorChain []Node, typ T) (node T, index int, ok bool) {
	return FindClosestMaxDistance[T](ancestorChain, typ, -1)
}

// FindClosestMaxDistance searches for an ancestor node of type typ starting from the parent node (last ancestor),
// maxDistance is the maximum distance from the parent node. A negative or zero maxDistance is ignored.
func FindClosestMaxDistance[T Node](ancestorChain []Node, typ T, maxDistance int) (node T, index int, ok bool) {
	searchedType := reflect.TypeOf(typ)

	lastI := 0
	if maxDistance > 0 {
		lastI = max(0, len(ancestorChain)-maxDistance-1)
	}

	for i := len(ancestorChain) - 1; i >= lastI; i-- {
		n := ancestorChain[i]
		if reflect.TypeOf(n) == searchedType {
			return n.(T), i, true
		}
	}

	return reflect.Zero(searchedType).Interface().(T), -1, false
}

// FindClosestTopLevelStatement returns the deepest top level statement among node and its ancestors.
func FindClosestTopLevelStatement(node Node, ancestorChain []Node) (Node, bool) {
	if len(ancestorChain) == 0 {
		return nil, false
	}
	parent := ancestorChain[len(ancestorChain)-1]

	if IsTheTopLevel(parent) {
		return node, true
	}

	for i := len(ancestorChain) - 1; i >= 1; i-- {
		if IsTheTopLevel(ancestorChain[i-1]) {
			return ancestorChain[i], true
		}
	}
	return nil, false
}

func FindPreviousStatement(n Node, ancestorChain []Node) (stmt Node, ok bool) {
	stmt, _, ok = FindPreviousStatementAndChain(n, ancestorChain, true)
	return
}

func FindPreviousStatementAndChain(n Node, ancestorChain []Node, climbBlocks bool) (stmt Node, chain []Node, ok bool) {
	if len(ancestorChain) == 0 || IsScopeContainerNode(n) {
		return nil, nil, false
	}

	p := ancestorChain[len(ancestorChain)-1]
	switch parent := p.(type) {
	case *Block:
		for i, stmt := range parent.Statements {
			if stmt == n {
				if i == 0 {
					if !climbBlocks {
						return nil, nil, false
					}
					return FindPreviousStatementAndChain(parent, ancestorChain[:len(ancestorChain)-1], climbBlocks)
				}
				return parent.Statements[i-1], ancestorChain, true
			}
		}
		if !climbBlocks {
			return nil, nil, false
		}
	case *Chunk:
		for i, stmt := range parent.Statements {
			if stmt == n {
				if i == 0 {
					return nil, nil, false
				}
				return parent.Statements[i-1], ancestorChain, true
			}
		}
	case *EmbeddedModule:
		for i, stmt := range parent.Statements {
			if stmt == n {
				if i == 0 {
					return nil, nil, false
				}
				return parent.Statements[i-1], ancestorChain, true
			}
		}
	}
	return FindPreviousStatementAndChain(p, ancestorChain[:len(ancestorChain)-1], climbBlocks)
}

func FindIdentWithName(root Node, name string) Node {
	return FindNode(root, (*IdentifierLiteral)(nil), func(n *IdentifierLiteral, isFirstFound bool, ancestors []Node) bool {
		return n.Name == name
	})
}

func FindLocalVarWithName(root Node, name string) Node {
	return FindNode(root, (*Variable)(nil), func(n *Variable, isFirstFound bool, ancestors []Node) bool {
		return n.Name == name
	})
}

func FindPatternIdentWithName(root Node, name string) Node {
	return FindNode(root, (*PatternIdentifierLiteral)(nil), func(n *PatternIdentifierLiteral, isFirstFound bool, ancestors []Node) bool {
		return n.Name == name
	})
}

func FindIntLiteralWithValue(root Node, value int64) *IntLiteral {
	return FindNode(root, (*IntLiteral)(nil), func(n *IntLiteral, isFirstFound bool, ancestors []Node) bool {
		return n.Value == value
	})
}

func FindObjPropWithName(root Node, name string) *ObjectProperty {
	return FindNode(root, (*ObjectProperty)(nil), func(n *ObjectProperty, isFirstFound bool, ancestors []Node) bool {
		return !n.HasNoKey() && n.Name() == name
	})
}

func FindLocalVarDeclWithName(root Node, name string) *LocalVariableDeclarator {
	return FindNode(root, (*LocalVariableDeclarator)(nil), func(n *LocalVariableDeclarator, isFirstFound bool, ancestors []Node) bool {
		ident, ok := n.Left.(*IdentifierLiteral)
		return ok && ident.Name == name
	})
}

func IsIdentLiteralWithName(n Node, name string) bool {
	ident, ok := n.(*IdentifierLiteral)
	return ok && ident.Name == name
}

func IsIdentMemberExprWithNames(n Node, name string, propNames ...string) bool {
	memberExpr, ok := n.(*IdentifierMemberExpression)
	if !ok || len(propNames) != len(memberExpr.PropertyNames) {
		return false
	}
	for i, propName := range propNames {
		if memberExpr.PropertyNames[i] == nil || memberExpr.PropertyNames[i].Name != propName {
			return false
		}
	}
	return true
}

func HasErrorAtAnyDepth(n Node) bool {
	err := false
	Walk(n, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if node.Base().Err != nil {
			err = true
			return StopTraversal, nil
		}
		return ContinueTraversal, nil
	}, nil)

	return err
}

func GetTreeView(n Node, chunk *Chunk) string {
	var buf = bytes.NewBuffer(nil)

	Walk(n, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		depth := len(ancestorChain)

		buf.Write(bytes.Repeat([]byte{' ', ' '}, depth))
		buf.WriteString(reflect.TypeOf(node).Elem().Name())

		if !NodeIsSimpleValueLiteral(node) {
			buf.WriteString("{ ")
			for _, tok := range GetTokens(node, chunk, false) {

				switch tok.Type {
				case UNEXPECTED_CHAR:
					buf.WriteString("(unexpected)`")
					if tok.Raw == "\n" {
						buf.WriteString("\\n")
					} else {
						buf.WriteString(tok.Str())
					}
				case NEWLINE:
					buf.WriteString(" `")
					buf.WriteString("\\n")
				default:
					buf.WriteString(" `")
					buf.WriteString(tok.Str())
				}
				buf.WriteString("` ")
			}
			buf.WriteByte('\n')
		} else {
			buf.WriteByte('\n')
		}

		return ContinueTraversal, nil
	}, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if !NodeIsSimpleValueLiteral(node) {
			depth := len(ancestorChain)
			buf.Write(bytes.Repeat([]byte{' ', ' '}, depth))
			buf.WriteString("}\n")
		}
		return ContinueTraversal, nil
	})

	return buf.String()
}

func GetInteriorSpan(node Node, chunk *Chunk) (interiorSpan sourcecode.NodeSpan, err error) {
	switch node.(type) {
	case *ObjectLiteral:
		return getInteriorSpan(node, chunk, OPENING_CURLY_BRACKET, CLOSING_CURLY_BRACKET)
	case *RecordLiteral:
		return getInteriorSpan(node, chunk, OPENING_RECORD_BRACKET, CLOSING_CURLY_BRACKET)
	case *DictionaryLiteral:
		return getInteriorSpan(node, chunk, OPENING_DICTIONARY_BRACKET, CLOSING_CURLY_BRACKET)
	}
	err = errors.New("not supported yet")
	return
}

// GetInteriorSpan returns the span of the "interior" of nodes such as blocks, objects or lists.
// the fist token matching the opening token is taken as the starting token (the span starts just after the token),
// the last token matching the closingToken is as taken as the ending token (the span ends just before this token).
func getInteriorSpan(node Node, chunk *Chunk, openingToken, closingToken TokenType) (interiorSpan sourcecode.NodeSpan, err error) {
	tokens := GetTokens(node, chunk, false)
	if len(tokens) == 0 {
		err = ErrMissingTokens
		return
	}

	interiorSpan = sourcecode.NodeSpan{Start: -1, End: -1}

	for _, token := range tokens {
		switch {
		case token.Type == openingToken && interiorSpan.Start < 0:
			interiorSpan.Start = token.Span.Start + 1
		case token.Type == closingToken:
			interiorSpan.End = token.Span.Start
		}
	}

	if interiorSpan.Start == -1 || interiorSpan.End == -1 {
		interiorSpan = sourcecode.NodeSpan{Start: -1, End: -1}
		err = ErrMissingTokens
		return
	}

	return
}

// DetermineActiveParameterIndex determines the index of the function parameter,
// it returns -1 if the index cannot be determined or if $ancestors does not contain
// a *Chunk. $callExprIndex should be -1 if $nodeAtSpan is $callExpr.
func DetermineActiveParameterIndex(
	cursorSpan sourcecode.NodeSpan,
	nodeAtSpan Node,
	callExpr *CallExpression,
	callExprIndex int,
	ancestors []Node,
) int {
	var argNode Node

	//Find the chunk in ancestors.

	var chunk *Chunk

	for _, ancestor := range ancestors {
		if c, ok := ancestor.(*Chunk); ok {
			chunk = c
			break
		}
	}

	if chunk == nil {
		return -1
	}

	if callExpr == ancestors[len(ancestors)-1] {
		if nodeAtSpan != callExpr {
			argNode = nodeAtSpan
		}
	} else if callExprIndex >= 0 {
		argNode = ancestors[callExprIndex+1]
	}

	if argNode != nil {
		for i, n := range callExpr.Arguments {
			if n == argNode {
				return i
			}
		}
		return -1
	} else if len(callExpr.Arguments) > 0 { //find the argument on the left of the cursor
		activeParamIndex := -1
		for i, currentArgNode := range callExpr.Arguments {
			currentArgEnd := currentArgNode.Base().Span.End

			if cursorSpan.Start >= currentArgEnd {
				activeParamIndex = i

				// increment argNodeIndex if the cursor is after a comma located after the current argument.
				for _, token := range GetTokens(callExpr, chunk, false) {
					if token.Type == COMMA && token.Span.Start >= currentArgEnd && cursorSpan.Start >= token.Span.End {
						activeParamIndex++
						break
					}
				}
			}
		}
		return activeParamIndex
	} else {
		return 0
	}
}

func IsAnyVariableIdentifier(node Node) bool {
	switch node.(type) {
	case *Variable, *IdentifierLiteral:
		return true
	default:
		return false
	}
}

func GetVariableName(node Node) string {
	switch n := node.(type) {
	case *Variable:
		return n.Name
	case *IdentifierLiteral:
		return n.Name
	default:
		panic(fmt.Errorf("cannot get variable name from node of type %T", node))
	}
}
