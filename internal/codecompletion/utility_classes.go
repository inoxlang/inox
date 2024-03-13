package codecompletion

import (
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/css/varclasses"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"golang.org/x/exp/maps"
)

func findUtilityClassSuggestions(attrValueNode parse.SimpleValueLiteral, search completionSearch) (completions []Completion) {
	quotedStrLiteral, ok := attrValueNode.(*parse.DoubleQuotedStringLiteral)
	if !ok {
		return
	}

	cut, ok := parse.CutQuotedStringLiteral(search.cursorIndex, quotedStrLiteral)
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

	if classNamePrefix == "" || strings.Count(classNamePrefix, ":") > 1 {
		return nil
	}

	if strings.HasPrefix(classNamePrefix, "--") {
		completions = append(completions, findCssVarBasedClassSuggestions(classNamePrefix, search)...)
	} else {
		completions = append(completions, findTailwindClassNameSuggestions(classNamePrefix, search)...)
	}

	return
}

func findCssVarBasedClassSuggestions(classNamePrefix string, search completionSearch) (completions []Completion) {
	if search.inputData.CodebaseAnalysis == nil {
		return
	}

	replacedRange := search.chunk.GetSourcePosition(parse.NodeSpan{
		Start: search.cursorIndex - int32(len(classNamePrefix)),
		End:   search.cursorIndex,
	})

	vars := maps.Values(search.inputData.CodebaseAnalysis.CssVariables)
	slices.SortFunc(vars, func(a, b varclasses.Variable) int {
		return strings.Compare(string(a.Name), string(b.Name))
	})

	for _, cssVar := range vars {
		if cssVar.AffectedProperty == "" {
			continue
		}
		c := Completion{
			ShownString:           string(cssVar.Name),
			Value:                 string(cssVar.Name),
			Kind:                  defines.CompletionItemKindConstant,
			ReplacedRange:         replacedRange,
			LabelDetail:           cssVar.AutoRuleset.StringifiedRules(" "),
			MarkdownDocumentation: "```css\n" + cssVar.AutoRuleset.String() + "\n```",
		}

		completions = append(completions, c)
	}

	return
}
