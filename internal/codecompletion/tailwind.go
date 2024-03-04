package codecompletion

import (
	"strings"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/tailwind"
)

func findTailwindClassNameSuggestions(attrValueNode *parse.QuotedStringLiteral, search completionSearch) (completions []Completion) {

	cut, ok := parse.CutQuotedStringLiteral(search.cursorIndex, attrValueNode)
	if !ok {
		return nil
	}

	//Do not suggest anything if the cursor is in the middle of a class name.
	if !cut.IsIndexAtEnd && !cut.HasSpaceAfterIndex {
		return nil
	}

	classNamePrefix := ""
	if index := strings.LastIndex(cut.BeforeIndex, " "); index >= 0 {
		classNamePrefix = cut.BeforeIndex[index+1:] //Not an issue if empty.
	} else {
		classNamePrefix = cut.BeforeIndex //Not an issue if empty.
	}

	if classNamePrefix == "" {
		return nil
	}

	rulesets := tailwind.GetRulesetsFromSubset("." + classNamePrefix)

	for _, set := range rulesets {
		className := strings.TrimPrefix(set.Name, ".")
		replacedRange := search.chunk.GetSourcePosition(parse.NodeSpan{
			Start: search.cursorIndex - int32(len(classNamePrefix)),
			End:   search.cursorIndex,
		})

		completions = append(completions, Completion{
			ShownString:           className,
			Value:                 className,
			Kind:                  defines.CompletionItemKindConstant,
			ReplacedRange:         replacedRange,
			LabelDetail:           "Tailwind",
			MarkdownDocumentation: "```css\n" + set.Node.String() + "\n```",
		})
	}

	return
}
