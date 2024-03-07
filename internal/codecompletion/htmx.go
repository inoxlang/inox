package codecompletion

import (
	"strings"

	"github.com/inoxlang/inox/internal/htmx"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func findHTMXAttributeValueSuggestions(attributeName string, strLiteral parse.SimpleValueLiteral, search completionSearch) (completions []Completion) {

	cut, ok := parse.CutQuotedStringLiteral(search.cursorIndex, strLiteral)
	if !ok {
		return
	}

	literalSpan := strLiteral.Base().Span

	switch attributeName {
	case "hx-ext":
		//Retrieve the extension name at the cursor.

		leftPart := ""
		var absoluteStartIndex int32

		if index := strings.LastIndex(cut.BeforeIndex, ","); index >= 0 {
			leftPart = cut.BeforeIndex[index+1:]
			absoluteStartIndex = literalSpan.Start + 1 + int32(index+1)
		} else {
			leftPart = cut.BeforeIndex
			absoluteStartIndex = literalSpan.Start + 1
		}

		rightPart := ""
		var absoluteEndIndex int32

		if index := strings.LastIndex(cut.AfterIndex, ","); index >= 0 {
			rightPart = cut.AfterIndex[index+1:]
			absoluteEndIndex = literalSpan.Start + 1 + int32(index)
		} else {
			rightPart = cut.AfterIndex
			absoluteEndIndex = literalSpan.End - 1
		}

		part := leftPart + rightPart
		leadingSpaceCount := 0

		for i := 0; i < len(part); i++ {
			if part[i] == ' ' {
				leadingSpaceCount++
			} else {
				break
			}
		}

		trailingSpaceCount := 0
		for i := len(part) - 1; i >= /*max left most index*/ leadingSpaceCount; i-- {
			if part[i] == ' ' {
				trailingSpaceCount++
			} else {
				break
			}
		}

		partTrimmedSpace := ""
		isOnlySpace := len(part) == leadingSpaceCount

		if !isOnlySpace {
			partTrimmedSpace = part[leadingSpaceCount : len(part)-trailingSpaceCount]
		}

		leadingSpace := strings.Repeat(" ", leadingSpaceCount)

		replacedRange := search.chunk.GetSourcePosition(parse.NodeSpan{Start: absoluteStartIndex, End: absoluteEndIndex})

		for _, ext := range htmx.GetExtensionInfoByPrefix(partTrimmedSpace) {
			completions = append(completions, Completion{
				ShownString:           ext.Name,
				Value:                 leadingSpace + ext.Name,
				ReplacedRange:         replacedRange,
				Kind:                  defines.CompletionItemKindConstant,
				LabelDetail:           ext.ShortDescription,
				MarkdownDocumentation: ext.Documentation,
			})
		}
	}

	return
}
