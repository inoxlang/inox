package parse

import "github.com/inoxlang/inox/internal/ast"

// EstimateIndentationUnit estimates the indentation unit in $code (e.g. 4 space characters, 2 tabs).
func EstimateIndentationUnit(code []rune, chunk *ast.Chunk) string {
	var indents = map[string]int{}

	update := func(index int32) {
		currentIndent := ""
		for i := index - 1; i >= 0; i-- {
			// reset the currentIndent on newline or change in indentation
			if code[i] == ' ' || code[i] == '\t' {
				currentIndent += string(code[i])
			} else {
				indents[currentIndent]++
				break
			}
		}
	}

	ast.Walk(chunk, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		switch n := node.(type) {
		case *ast.ObjectLiteral:
			if _, ok := parent.(*ast.Manifest); !ok {
				break
			}
			for _, prop := range n.Properties {
				update(ast.GetFirstToken(prop, chunk).Span.Start)
			}
		case *ast.Block:
			for _, stmt := range n.Statements {
				update(ast.GetFirstToken(stmt, chunk).Span.Start)
			}
		case *ast.GlobalConstantDeclaration, *ast.SwitchStatementCase, *ast.MatchStatementCase:
			update(node.Base().Span.Start)
		case *ast.Chunk, *ast.GlobalConstantDeclarations, *ast.Manifest, *ast.FunctionDeclaration, *ast.FunctionExpression,
			*ast.ForStatement, *ast.WalkStatement, *ast.SwitchStatement, *ast.MatchStatement:
			return ast.ContinueTraversal, nil
		}
		return ast.Prune, nil
	}, nil)

	mostCommonIndent := ""
	maxCount := 0
	for indent, count := range indents {
		if count > maxCount {
			maxCount = count
			mostCommonIndent = indent
		}
	}

	if mostCommonIndent == "" {
		return ""
	}
	return mostCommonIndent
}
