package project_server

import (
	"slices"
	"sort"
	"strings"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
)

func format(chunk *parse.ParsedChunk, options defines.FormattingOptions) (result string) {
	code := slices.Clone(chunk.Runes())

	defer func() {
		if recover() != nil {
			result = string(code)
			return
		}
	}()

	indentationUnit := "\t"
	if options.InsertSpaces {
		indentationUnit = strings.Repeat(" ", int(options.TabSize))
	}

	depth := 0

	type replacement struct {
		span    parse.NodeSpan
		newText string
	}

	var replacements []replacement

	replaceIfLeadingSpace := func(span parse.NodeSpan) {
		replacementEnd := span.Start
		replacementStart := replacementEnd

		lineStartfound := false

		for i := replacementEnd - 1; i >= 0; i-- {
			if code[i] != ' ' && code[i] != '\t' {
				if code[i] == '\n' {
					lineStartfound = true
					replacementStart = i + 1
				}
				break
			}
		}

		if lineStartfound {
			replacements = append(replacements, replacement{
				span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
				newText: strings.Repeat(indentationUnit, depth),
			})
		}
	}

	replaceSurroundingSpaces := func(span parse.NodeSpan, expectedBefore, expectedAfter int) {
		//before
		{
			replacementEnd := span.Start
			replacementStart := replacementEnd

			for i := replacementEnd - 1; i >= 0; i-- {
				if code[i] == ' ' || code[i] == '\t' {
					replacementStart = i
				} else {
					break
				}
			}

			replacements = append(replacements, replacement{
				span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
				newText: strings.Repeat(" ", expectedBefore),
			})
		}
		// after
		{
			replacementStart := span.End
			replacementEnd := replacementStart

			for i := replacementStart; i < int32(len(code)); i++ {
				if code[i] != ' ' && code[i] != '\t' {
					replacementEnd = i
					break
				}
			}

			replacements = append(replacements, replacement{
				span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
				newText: strings.Repeat(" ", expectedAfter),
			})
		}

	}

	//compute replacements
	var seenTokens = map[parse.Token]struct{}{}

	parse.Walk(chunk.Node, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		if needsIndentation(node, parent, ancestorChain) {
			replaceIfLeadingSpace(node.Base().Span)
		} else {
			if doesNodeIncreaseDepth(node) {
				depth++
			}

			//remove leading space of top-level statements

			if _, ok := parent.(*parse.Chunk); ok {
				replacementEnd := node.Base().Span.Start
				replacementStart := replacementEnd

				for i := replacementEnd - 1; i >= 0; i-- {
					if code[i] == ' ' || code[i] == '\t' {
						replacementStart = i
					} else {
						break
					}
				}

				replacements = append(replacements, replacement{
					span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
					newText: "",
				})
			}

		}
		return parse.ContinueTraversal, nil
	}, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		if doesNodeIncreaseDepth(node) {
			depth--
		}

		tokens := parse.GetTokens(node, chunk.Node, false)
		for _, token := range tokens {

			_, ok := seenTokens[token]
			if ok {
				continue
			}
			seenTokens[token] = struct{}{}

			switch token.Type {
			case parse.CLOSING_BRACKET, parse.CLOSING_PARENTHESIS, parse.CLOSING_CURLY_BRACKET:
				replaceIfLeadingSpace(token.Span)
			case parse.COLON:
				switch node.(type) {
				case *parse.ObjectProperty, *parse.ObjectPatternProperty:
					replaceSurroundingSpaces(token.Span, 0, 1)
				}
			case parse.ARROW:
				replaceSurroundingSpaces(token.Span, 1, 1)
			case parse.EQUAL:
				if token.SubType == parse.ASSIGN_EQUAL {
					replaceSurroundingSpaces(token.Span, 1, 1)
				}
			}
		}
		return parse.ContinueTraversal, nil
	})

	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].span.Start < replacements[j].span.Start
	})

	index := 0

	var formatted []rune

	for _, replacement := range replacements {
		formatted = append(formatted, code[index:replacement.span.Start]...)
		formatted = append(formatted, []rune(replacement.newText)...)
		index = int(replacement.span.End)
	}

	formatted = append(formatted, code[index:]...)

	return string(formatted)
}

func needsIndentation(n parse.Node, parent parse.Node, ancestors []parse.Node) bool {
	switch n.(type) {
	case *parse.ObjectMetaProperty, *parse.ObjectProperty, *parse.ObjectPatternProperty,
		*parse.DictionaryEntry,
		*parse.StaticMappingEntry, *parse.DynamicMappingEntry,
		*parse.SwitchCase, *parse.MatchCase, *parse.DefaultCase:
		return true
	}

	switch parent.(type) {
	case *parse.Block, *parse.EmbeddedModule:
		return true
	}

	return false
}

func doesNodeIncreaseDepth(node parse.Node) bool {
	switch node.(type) {
	case *parse.ObjectLiteral, *parse.ObjectPatternLiteral, *parse.RecordLiteral,
		*parse.ListLiteral, *parse.MappingExpression, *parse.DictionaryLiteral, *parse.EmbeddedModule, *parse.Block,
		*parse.SwitchStatement, *parse.MatchStatement:
		return true
	}
	return false
}
