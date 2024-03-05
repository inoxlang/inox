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
		tokens = slices.Clone(n.HyperscriptParsingResult.TokensNoWhitespace)
	} else if n.HyperscriptParsingError != nil {
		tokens = slices.Clone(n.HyperscriptParsingError.TokensNoWhitespace)
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

	return findHyperscriptCompletions(tokens, cursorIndexInHsCode, search)
}

func findHyperscriptScriptCompletions(n *parse.XMLElement, search completionSearch) (completions []Completion) {
	cursorIndexInHsCode := search.cursorIndex - n.RawElementContentStart

	var tokens []hscode.Token
	if n.RawElementParsingResult == nil {
		return
	}
	parsingResult, ok := n.RawElementParsingResult.(*hscode.ParsingResult)
	if ok {
		tokens = slices.Clone(parsingResult.TokensNoWhitespace)
	} else if parsingErr, ok := n.RawElementParsingResult.(*hscode.ParsingError); ok {
		tokens = slices.Clone(parsingErr.TokensNoWhitespace)
	} else {
		return
	}

	_, ok = n.Opening.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	return findHyperscriptCompletions(tokens, cursorIndexInHsCode, search)
}

func findHyperscriptCompletions(tokens []hscode.Token, cursorIndexInHsCode int32, search completionSearch) (completions []Completion) {
	tokensNoLinefeeds := 0
	for _, token := range tokens {
		if token.Value != "\n" {
			tokensNoLinefeeds++
		}
	}

	if tokensNoLinefeeds <= 1 {
		completions = append(completions, getFeatureStartCompletions()...)
	}

	if tokensNoLinefeeds > 0 {
		completions = append(completions, getHyperscriptTokenCompletions(cursorIndexInHsCode, tokens)...)
	}

	if tokensNoLinefeeds > 1 {
		completions = append(completions, tryGetTrailingCommandHelp(cursorIndexInHsCode, tokens, search)...)
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

	//TODO: use token ?

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

func getFeatureStartCompletions() (completions []Completion) {

	for _, example := range hshelp.HELP_DATA.FeatureStartExamples {

		completions = append(completions, Completion{
			ShownString:           "(example)" + example.Code,
			Value:                 example.Code,
			Kind:                  defines.CompletionItemKindEvent,
			LabelDetail:           example.ShortExplanation,
			MarkdownDocumentation: example.MarkdownDocumentation,
		})
	}

	return
}

func tryGetTrailingCommandHelp(relativeIndex int32, tokens []hscode.Token, search completionSearch) (completions []Completion) {

	if len(tokens) <= 1 {
		return
	}

	lastToken := tokens[len(tokens)-1]

	if lastToken.Type != hscode.IDENTIFIER {
		return
	}

	for _, example := range hshelp.HELP_DATA.CommandExamples {

		completions = append(completions, Completion{
			ShownString:           "(example)" + example.Code,
			Value:                 example.Code,
			Kind:                  defines.CompletionItemKindEvent,
			LabelDetail:           example.ShortExplanation,
			MarkdownDocumentation: example.MarkdownDocumentation,
		})
	}

	return
}
