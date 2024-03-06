package codecompletion

import (
	"slices"

	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/hyperscript/hshelp"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func findHyperscriptAttributeCompletions(n *parse.HyperscriptAttributeShorthand, search completionSearch) (completions []Completion) {
	cursorIndexInHsCode := search.cursorIndex - n.Span.Start - (1 /* '{'*/)
	hsCodeStart := n.Span.Start + 1
	hsCodeEnd := n.Span.End - 1
	if n.IsUnterminated {
		hsCodeEnd = n.Span.End
	}

	var tokensNoSpace []hscode.Token
	var tokens []hscode.Token

	if n.HyperscriptParsingResult != nil {
		tokensNoSpace = slices.Clone(n.HyperscriptParsingResult.TokensNoWhitespace)
		tokens = slices.Clone(n.HyperscriptParsingResult.Tokens)
	} else if n.HyperscriptParsingError != nil {
		tokensNoSpace = slices.Clone(n.HyperscriptParsingError.TokensNoWhitespace)
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

	return findHyperscriptCompletions(tokensNoSpace, tokens, cursorIndexInHsCode, hsCodeStart, hsCodeEnd, search)
}

func findHyperscriptScriptCompletions(n *parse.XMLElement, search completionSearch) (completions []Completion) {
	cursorIndexInHsCode := search.cursorIndex - n.RawElementContentStart

	var tokensNoSpace []hscode.Token
	var tokens []hscode.Token

	if n.RawElementParsingResult == nil {
		return
	}
	parsingResult, ok := n.RawElementParsingResult.(*hscode.ParsingResult)
	if ok {
		tokensNoSpace = slices.Clone(parsingResult.TokensNoWhitespace)
		tokens = slices.Clone(parsingResult.Tokens)
	} else if parsingErr, ok := n.RawElementParsingResult.(*hscode.ParsingError); ok {
		tokensNoSpace = slices.Clone(parsingErr.TokensNoWhitespace)
		tokens = slices.Clone(parsingErr.Tokens)
	} else {
		return
	}

	_, ok = n.Opening.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	return findHyperscriptCompletions(tokensNoSpace, tokens, cursorIndexInHsCode, n.RawElementContentStart, n.RawElementContentEnd, search)
}

func findHyperscriptCompletions(tokensNoSpace, tokens []hscode.Token, cursorIndexInHsCode, hsCodeStart, hsCodeEnd int32, search completionSearch) (completions []Completion) {
	tokensNoLinefeeds := 0
	for _, token := range tokensNoSpace {
		if token.Value != "\n" {
			tokensNoLinefeeds++
		}
	}

	if tokensNoLinefeeds <= 1 {
		completions = append(completions, getFeatureStartCompletions(hsCodeStart, hsCodeEnd, tokensNoSpace, search)...)
	}

	if tokensNoLinefeeds > 0 {
		completions = append(completions, getHyperscriptTokenCompletions(cursorIndexInHsCode, hsCodeStart, hsCodeEnd, tokensNoSpace, search)...)
	}

	if tokensNoLinefeeds > 1 {
		completions = append(completions, tryGetTrailingCommandHelp(hsCodeStart, hsCodeEnd, cursorIndexInHsCode, tokens, search)...)
	}

	return
}

func getHyperscriptTokenCompletions(cursorIndexInHsCode, hsCodeStart, hsCodeEnd int32, tokens []hscode.Token, search completionSearch) (completions []Completion) {
	token, ok := hscode.GetTokenAtCursor(cursorIndexInHsCode, tokens)
	if !ok {
		return
	}

	keywords := hshelp.GetKeywordsByPrefix(token.Value)

	//Already valid token.
	if len(keywords) == 1 && keywords[0].Name == token.Value {
		return
	}

	keywordReplacedRange := search.chunk.GetSourcePosition(parse.NodeSpan{Start: hsCodeStart + token.Start, End: hsCodeStart + token.End})

	for _, keyword := range keywords {
		completions = append(completions, Completion{
			ShownString:           keyword.Name,
			Value:                 keyword.Name,
			Kind:                  defines.CompletionItemKindKeyword,
			LabelDetail:           keyword.DocumentationLink,
			MarkdownDocumentation: keyword.DocumentationLink,
			ReplacedRange:         keywordReplacedRange,
		})
	}

	//*<property name>
	if token.Type == hscode.STYLE_REF && token.Value[0] == '*' && len(token.Value) >= 2 {

		tokenStart := hsCodeStart + token.Start
		tokenEnd := hsCodeStart + token.End

		replacedRange := search.chunk.GetSourcePosition(parse.NodeSpan{Start: tokenStart + 1 /*do not include the '*' */, End: tokenEnd})

		propertyNamePrefix := token.Value[1:]
		css.ForEachPropertyName(propertyNamePrefix, func(name string) error {

			completions = append(completions, Completion{
				ShownString:   name,
				Value:         name,
				Kind:          defines.CompletionItemKindProperty,
				ReplacedRange: replacedRange,
				LabelDetail:   "style property",
			})
			return nil
		})
	}

	return
}

// getFeatureStartCompletions returns feature examples, it assumes there is at most one significant token in the code.
func getFeatureStartCompletions(hsCodeStart, hsCodeEnd int32, tokensNoSpace []hscode.Token, search completionSearch) (completions []Completion) {

	var replacedRange parse.SourcePositionRange

	tokensNoLinefeeds := utils.FilterSlice(tokensNoSpace, func(e hscode.Token) bool { return e.Value != "\n" })

	if len(tokensNoLinefeeds) == 1 {
		token := tokensNoLinefeeds[0]
		//replace token
		replacedRange = search.chunk.GetSourcePosition(parse.NodeSpan{Start: hsCodeStart + token.Start, End: hsCodeStart + token.End})
	} else {
		cursorIndexInHsCode := search.cursorIndex - hsCodeStart
		token, ok := hscode.GetTokenAtCursor(cursorIndexInHsCode, tokensNoSpace)
		if ok {
			//replace token
			replacedRange = search.chunk.GetSourcePosition(parse.NodeSpan{Start: hsCodeStart + token.Start, End: hsCodeStart + token.End})
		} else {
			//insert at cursor
			replacedRange = search.chunk.GetSourcePosition(parse.NodeSpan{Start: search.cursorIndex, End: search.cursorIndex})
		}
	}

	for _, example := range hshelp.HELP_DATA.FeatureStartExamples {

		completions = append(completions, Completion{
			ShownString:           "[example]" + example.Code,
			Value:                 example.Code,
			Kind:                  defines.CompletionItemKindEvent,
			LabelDetail:           example.ShortExplanation,
			MarkdownDocumentation: example.MarkdownDocumentation,
			ReplacedRange:         replacedRange,
		})
	}

	return
}

func tryGetTrailingCommandHelp(hsCodeStart, hsCodeEnd, relativeIndex int32, tokens []hscode.Token, search completionSearch) (completions []Completion) {

	if len(tokens) <= 1 {
		return
	}

	token, ok := hscode.GetTokenAtCursor(relativeIndex, tokens)

	//Only show completions if the token is whitespace or an identifier.
	if !ok || (token.Type != hscode.WHITESPACE && token.Type != hscode.IDENTIFIER) || hsgen.IsBuiltinCommandName(token.Value) {
		return
	}

	//If the previous token is 'to' or a command we do not suggest commands.
	tokenBefore, _ := hscode.GetClosestTokenOnCursorLeftSide(token.Start, tokens)
	if token.Type == hscode.WHITESPACE &&
		tokenBefore.Type == hscode.IDENTIFIER &&
		(tokenBefore.Value == "to" || hsgen.IsBuiltinCommandName(tokenBefore.Value)) {
		return
	}

	var replacedRange parse.SourcePositionRange
	if token.Type == hscode.WHITESPACE {
		//empty range (insertion)
		replacedRange = search.chunk.GetSourcePosition(parse.NodeSpan{Start: hsCodeStart + token.End, End: hsCodeStart + token.End})
	} else {
		replacedRange = search.chunk.GetSourcePosition(parse.NodeSpan{Start: hsCodeStart + token.Start, End: hsCodeStart + token.End})
	}

	for _, example := range hshelp.HELP_DATA.CommandExamples {

		completions = append(completions, Completion{
			ShownString:           "[example]" + example.Code,
			Value:                 example.Code,
			Kind:                  defines.CompletionItemKindFunction,
			LabelDetail:           example.ShortExplanation,
			MarkdownDocumentation: example.MarkdownDocumentation,
			ReplacedRange:         replacedRange,
		})
	}

	return
}
