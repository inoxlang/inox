package parse

func EstimateIndentationUnit(code []rune, node Node) string {
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

	Walk(node, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		switch n := node.(type) {
		case *ObjectLiteral:
			if _, ok := parent.(*Manifest); !ok {
				break
			}
			for _, prop := range n.Properties {
				update(GetFirstToken(prop).Span.Start)
			}
		case *Block:
			for _, stmt := range n.Statements {
				update(GetFirstToken(stmt).Span.Start)
			}
		case *GlobalConstantDeclaration:
			update(n.Span.Start)
		case *Chunk, *GlobalConstantDeclarations, *Manifest, *FunctionDeclaration, *FunctionExpression:
			return Continue, nil

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
