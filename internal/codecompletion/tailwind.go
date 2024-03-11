package codecompletion

import (
	"strings"

	"github.com/inoxlang/inox/internal/css/tailwind"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func findTailwindClassNameSuggestions(classNamePrefix string, search completionSearch) (completions []Completion) {

	replacedRange := search.chunk.GetSourcePosition(parse.NodeSpan{
		Start: search.cursorIndex - int32(len(classNamePrefix)),
		End:   search.cursorIndex,
	})

	modifierName, basename, ok := strings.Cut(classNamePrefix, ":")

	if ok {
		if basename == "" {
			//Do not suggest all class names because this is resource intensive.
			return
		}

		rulesets := tailwind.GetRulesetsFromSubset("." + basename)

		for _, set := range rulesets {
			set = set.WithOnlyModifier(modifierName)
			className := modifierName + ":" + strings.TrimPrefix(set.UserFriendlyBaseName, ".")

			completions = append(completions, makeTailwindCompletion(className, "Tailwind class with modifier", set.String(), replacedRange))
		}
	} else { //no modifier
		modifiers := tailwind.GetModifierInfoByPrefix(classNamePrefix)

		for _, modifier := range modifiers {
			completion := makeTailwindCompletion(modifier.Name+":", "Tailwind modifier: "+modifier.Description, "", replacedRange)
			completions = append(completions, completion)
		}

		rulesets := tailwind.GetRulesetsFromSubset("." + classNamePrefix)

		for _, set := range rulesets {
			className := strings.TrimPrefix(set.UserFriendlyBaseName, ".")

			completions = append(completions, makeTailwindCompletion(className, "Tailwind class", set.String(), replacedRange))
		}
	}

	return
}

func makeTailwindCompletion(name string, labelDetail, doc string, replacedRange parse.SourcePositionRange) Completion {
	c := Completion{
		ShownString:   name,
		Value:         name,
		Kind:          defines.CompletionItemKindConstant,
		ReplacedRange: replacedRange,
		LabelDetail:   labelDetail,
	}

	if doc != "" {
		c.MarkdownDocumentation = "```css\n" + doc + "\n```"
	}
	return c
}
