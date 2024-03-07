package codecompletion

import "github.com/inoxlang/inox/internal/parse"

func findHTMXAttributeValueSuggestions(attributeName string, strLiteral parse.SimpleValueLiteral, search completionSearch) (completions []Completion) {

	quotedStrLiteral, ok := strLiteral.(*parse.DoubleQuotedStringLiteral)
	if !ok {
		return
	}

	switch attributeName {
	case "hx-ext":

		parse.CutQuotedStringLiteral(quotedStrLiteral.Span.End)

	}

}
