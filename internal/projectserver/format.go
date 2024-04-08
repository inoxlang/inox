package projectserver

import (
	"slices"
	"sort"
	"strings"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func formatInoxChunk(chunk *parse.ParsedChunkSource, options defines.FormattingOptions) (result string) {
	formatter := formatter{}
	return formatter.formatInoxChunk(chunk, options)
}

type formatter struct {
	source          *parse.ParsedChunkSource
	code            []rune
	depth           int
	indentationUnit string
	replacements    []replacement
	allTokens       []parse.Token
	seenTokens      map[parse.NodeSpan]struct{}
}

type replacement struct {
	span    parse.NodeSpan
	newText string
}

func (f *formatter) formatInoxChunk(chunk *parse.ParsedChunkSource, options defines.FormattingOptions) (result string) {
	f.code = slices.Clone(chunk.Runes())

	defer func() {
		if e := recover(); e != nil {
			_ = e
			result = string(f.code)
			return
		}
	}()

	f.source = chunk
	f.replacements = nil
	f.depth = 0
	f.allTokens = parse.GetTokens(f.source.Node, f.source.Node, false)
	f.seenTokens = map[parse.NodeSpan]struct{}{}
	f.indentationUnit = "\t"

	if options.InsertSpaces {
		f.indentationUnit = strings.Repeat(" ", int(options.TabSize))
	}

	//compute replacements

	parse.Walk(chunk.Node, f.preVisitNode, f.postVisitNode)

	sort.Slice(f.replacements, func(i, j int) bool {
		return f.replacements[i].span.Start < f.replacements[j].span.Start
	})

	index := 0

	var formatted []rune

	for _, replacement := range f.replacements {
		formatted = append(formatted, f.code[index:replacement.span.Start]...)
		formatted = append(formatted, []rune(replacement.newText)...)
		index = int(replacement.span.End)
	}

	formatted = append(formatted, f.code[index:]...)

	return string(formatted)
}

func (f *formatter) preVisitNode(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

	switch node.(type) {
	case *parse.MarkupExpression, *parse.MarkupElement:
		return parse.Prune, nil
	case *parse.MarkupInterpolation:
		return parse.Prune, nil
	}

	if needsIndentation(node, parent, ancestorChain) {
		f.updateIndentation(node.Base().Span)
	}

	if doesNodeIncreaseDepth(node, ancestorChain) {
		f.depth++
	}

	return parse.ContinueTraversal, nil
}

func (f *formatter) postVisitNode(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

	tokens, hasTokens := f.getTokensOfNode(node.Base().Span)

	//Update identation of comment tokens.
	if hasTokens {
		for _, token := range tokens {
			if token.SubType == parse.MARKUP_TAG_OPENING_BRACKET { //Temporary fix
				break
			}

			switch token.Type {
			case parse.COMMENT:
			default:
				continue
			}

			_, ok := f.seenTokens[token.Span]
			if ok {
				continue
			}
			f.seenTokens[token.Span] = struct{}{}
			f.updateIndentation(token.Span)
		}
	}

	if doesNodeIncreaseDepth(node, ancestorChain) {
		f.depth--
	}

	if !hasTokens {
		return parse.ContinueTraversal, nil
	}

	//Update identation and surrounding space of some tokens.
	for _, token := range tokens {
		if token.SubType == parse.MARKUP_TAG_OPENING_BRACKET { //Temporary fix
			return parse.ContinueTraversal, nil
		}

		_, ok := f.seenTokens[token.Span]
		if ok {
			continue
		}
		f.seenTokens[token.Span] = struct{}{}

		switch token.Type {
		case parse.OPENING_CURLY_BRACKET:
			switch token.SubType {
			case 0:
				fallthrough
			case parse.BLOCK_OPENING_BRACE, parse.OBJECT_LIKE_OPENING_BRACE:
				f.updateIndentation(token.Span)
			default:
			}
		case parse.CLOSING_CURLY_BRACKET:
			switch token.SubType {
			case 0:
				fallthrough
			case parse.BLOCK_CLOSING_BRACE, parse.OBJECT_LIKE_CLOSING_BRACE:
				f.updateIndentation(token.Span)
			default:
			}
		case parse.CLOSING_PARENTHESIS, parse.OPENING_BRACKET, parse.CLOSING_BRACKET:
			f.updateIndentation(token.Span)
		case parse.COLON:
			switch node.(type) {
			case *parse.ObjectProperty, *parse.ObjectPatternProperty:
				f.replaceSurroundingSpaces(token.Span, 0, 1)
			}
		case parse.ARROW:
			f.replaceSurroundingSpaces(token.Span, 1, 1)
		case parse.EQUAL:
			if token.SubType == parse.ASSIGN_EQUAL {
				f.replaceSurroundingSpaces(token.Span, 1, 1)
			}
		}
	}
	return parse.ContinueTraversal, nil
}

func (f *formatter) getTokensOfNode(nodeSpan parse.NodeSpan) ([]parse.Token, bool) {
	//Search the first token in the node.
	startTokenIndex, ok := slices.BinarySearchFunc(f.allTokens, nodeSpan.Start, func(token parse.Token, spanStart int32) int {
		return int(token.Span.Start) - int(spanStart)
	})

	if !ok {
		return nil, false
	}

	//Search the first token after the node.
	endTokenIndex, _ := slices.BinarySearchFunc(f.allTokens, nodeSpan.End, func(token parse.Token, spanEnd int32) int {
		return int(token.Span.Start) - int(spanEnd)
	})

	//no issue if endTokenIndex == 1 + max index

	return f.allTokens[startTokenIndex:endTokenIndex], true
}

func (f *formatter) updateIndentation(span parse.NodeSpan) {
	replacementEnd := span.Start
	replacementStart := span.Start

	lineStartFound := false
	prevSameLineStatementFound := false

	for i := span.Start - 1; i >= 0; i-- {
		if i == 0 {
			lineStartFound = true
			replacementStart = 0
			break
		}

		if f.code[i] == '\n' {
			lineStartFound = true
			replacementStart = i + 1
			break
		}

		if f.code[i] == ';' {
			prevSameLineStatementFound = true
			replacementStart = i + 1
			break
		}
		if f.code[i] != ' ' && f.code[i] != '\t' {
			//The indentation of the node is not updated if there is a non whitespace
			//character somewhere between the start of the line and the start of the node.
			return
		}
	}

	if lineStartFound {
		f.replacements = append(f.replacements, replacement{
			span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
			newText: strings.Repeat(f.indentationUnit, f.depth),
		})
	} else if prevSameLineStatementFound {
		f.replacements = append(f.replacements, replacement{
			span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
			newText: " ",
		})
	}
}

func (f *formatter) replaceSurroundingSpaces(span parse.NodeSpan, expectedBefore, expectedAfter int) {
	//before
	{
		replacementEnd := span.Start
		replacementStart := replacementEnd

		for i := replacementEnd - 1; i >= 0; i-- {
			if f.code[i] == ' ' || f.code[i] == '\t' {
				replacementStart = i
			} else {
				break
			}
		}

		f.replacements = append(f.replacements, replacement{
			span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
			newText: strings.Repeat(" ", expectedBefore),
		})
	}
	// after
	{
		replacementStart := span.End
		replacementEnd := replacementStart

		for i := replacementStart; i < int32(len(f.code)); i++ {
			if f.code[i] != ' ' && f.code[i] != '\t' {
				replacementEnd = i
				break
			}
		}

		f.replacements = append(f.replacements, replacement{
			span:    parse.NodeSpan{Start: int32(replacementStart), End: int32(replacementEnd)},
			newText: strings.Repeat(" ", expectedAfter),
		})
	}

}

func needsIndentation(n parse.Node, parent parse.Node, ancestors []parse.Node) bool {
	switch n.(type) {
	case *parse.ObjectMetaProperty, *parse.ObjectProperty, *parse.ObjectPatternProperty,
		*parse.DictionaryEntry,
		*parse.StaticMappingEntry, *parse.DynamicMappingEntry,
		*parse.SwitchStatementCase, *parse.MatchStatementCase, *parse.DefaultCaseWithBlock,
		*parse.GlobalConstantDeclaration:
		return true
	}

	switch parent.(type) {
	case *parse.Block, *parse.ListLiteral, *parse.TupleLiteral, *parse.EmbeddedModule, *parse.Chunk:
		return true
	}

	return false
}

func doesNodeIncreaseDepth(node parse.Node, ancestors []parse.Node) bool {
	switch node.(type) {
	case *parse.ObjectLiteral, *parse.ObjectPatternLiteral, *parse.RecordLiteral,
		*parse.ListLiteral, *parse.MappingExpression, *parse.DictionaryLiteral, *parse.EmbeddedModule,
		*parse.SwitchStatement, *parse.MatchStatement,
		*parse.MarkupElement,
		*parse.GlobalConstantDeclarations:
		return true
	case *parse.Block:
		return true
	}

	return false
}
