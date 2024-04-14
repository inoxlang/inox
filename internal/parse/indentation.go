package parse

// EstimateIndentationUnit estimates the indentation unit in $code (e.g. 4 space characters, 2 tabs).
func EstimateIndentationUnit(code []rune, chunk *Chunk) string {
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

	Walk(chunk, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		switch n := node.(type) {
		case *ObjectLiteral:
			if _, ok := parent.(*Manifest); !ok {
				break
			}
			for _, prop := range n.Properties {
				update(GetFirstToken(prop, chunk).Span.Start)
			}
		case *Block:
			for _, stmt := range n.Statements {
				update(GetFirstToken(stmt, chunk).Span.Start)
			}
		case *GlobalConstantDeclaration, *SwitchStatementCase, *MatchStatementCase:
			update(node.Base().Span.Start)
		case *Chunk, *GlobalConstantDeclarations, *Manifest, *FunctionDeclaration, *FunctionExpression,
			*ForStatement, *WalkStatement, *SwitchStatement, *MatchStatement:
			return ContinueTraversal, nil
		}
		return Prune, nil
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
