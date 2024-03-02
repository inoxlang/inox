package codecompletion

import (
	"slices"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hshelp"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func findHyperscriptAttributeCompletions(n *parse.HyperscriptAttributeShorthand, search completionSearch) (completions []Completion) {
	cursorIndexInHsCode := search.cursorIndex - n.Span.Start - (1 /* '{'*/)

	var tokens []hscode.Token
	if n.HyperscriptParsingResult != nil {
		tokens = slices.Clone(n.HyperscriptParsingResult.Tokens)
	} else if n.HyperscriptParsingError != nil {
		tokens = slices.Clone(n.HyperscriptParsingError.Tokens)
	} else {
		return
	}

	openingElem, ok := search.parent.(*parse.XMLOpeningElement)
	if !ok {
		return
	}

	_, ok = openingElem.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	if len(tokens) == 0 {
		// completions = append(completions, Completion{
		// 	ShownString: "on click",
		// 	Value:       "on click",
		// 	Kind:        defines.CompletionItemKindText,
		// })
	} else {
		completions = append(completions, getHyperscriptTokenCompletions(cursorIndexInHsCode, tokens)...)
	}

	return
}

func findHyperscriptScriptCompletions(n *parse.XMLElement, search completionSearch) (completions []Completion) {
	cursorIndexInHsCode := search.cursorIndex - n.RawElementContentStart

	var tokens []hscode.Token
	if n.RawElementParsingResult == nil {
		return
	}
	parsingResult, ok := n.RawElementParsingResult.(*hscode.ParsingResult)
	if ok {
		tokens = parsingResult.Tokens
	} else if parsingErr, ok := n.RawElementParsingResult.(*hscode.ParsingError); ok {
		tokens = parsingErr.Tokens
	} else {
		return
	}

	_, ok = n.Opening.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	if len(tokens) == 0 {
		return
	} else {
		completions = append(completions, getHyperscriptTokenCompletions(cursorIndexInHsCode, tokens)...)
	}

	return
}

func getHyperscriptTokenCompletions(cursorIndexInHsCode int32, tokens []hscode.Token) (completions []Completion) {
	token, ok := hscode.GetTokenAtCursor(cursorIndexInHsCode, tokens)
	if !ok {
		return
	}

	keywords := hshelp.GetKeywordsByPrefix(token.Value)

	//Already valid token.
	if len(keywords) == 1 && keywords[0].Name == token.Value {
		return
	}

	for _, keyword := range keywords {
		completions = append(completions, Completion{
			ShownString:           keyword.Name,
			Value:                 keyword.Name,
			Kind:                  defines.CompletionItemKindKeyword,
			LabelDetail:           keyword.DocumentationLink,
			MarkdownDocumentation: keyword.DocumentationLink,
		})
	}

	return
}
