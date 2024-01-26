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
	if needsIndentation(node, parent, ancestorChain) {
		f.updateIndentation(node.Base().Span)
		return parse.ContinueTraversal, nil
	}

	if doesNodeIncreaseDepth(node, ancestorChain) {
		f.depth++
	}

	//remove leading space of top-level statements

	if _, ok := parent.(*parse.Chunk); ok {
		replacementEnd := node.Base().Span.Start
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
			newText: "",
		})
	}

	return parse.ContinueTraversal, nil
}

func (f *formatter) postVisitNode(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

	if doesNodeIncreaseDepth(node, ancestorChain) {
		f.depth--
	}

	tokens := parse.GetTokens(node, f.source.Node, false)
	for _, token := range tokens {

		_, ok := f.seenTokens[token.Span]
		if ok {
			continue
		}
		f.seenTokens[token.Span] = struct{}{}

		switch token.Type {
		case parse.CLOSING_BRACKET, parse.CLOSING_PARENTHESIS, parse.CLOSING_CURLY_BRACKET:
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

func (f *formatter) updateIndentation(span parse.NodeSpan) {
	replacementEnd := span.Start
	replacementStart := span.Start

	lineStartFound := false

	for i := span.Start - 1; i >= 0; i-- {
		if f.code[i] == '\n' {
			lineStartFound = true
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
		*parse.SwitchCase, *parse.MatchCase, *parse.DefaultCase:
		return true
	}

	switch parent.(type) {
	case *parse.Block, *parse.EmbeddedModule:
		return true
	}

	return false
}

func doesNodeIncreaseDepth(node parse.Node, ancestors []parse.Node) bool {
	switch node.(type) {
	case *parse.ObjectLiteral, *parse.ObjectPatternLiteral, *parse.RecordLiteral,
		*parse.ListLiteral, *parse.MappingExpression, *parse.DictionaryLiteral, *parse.EmbeddedModule,
		*parse.SwitchStatement, *parse.MatchStatement,
		*parse.XMLElement:
		return true
	case *parse.Block:
		return true
	}

	return false
}
