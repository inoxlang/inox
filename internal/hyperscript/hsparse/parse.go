package hsparse

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
)

// ParseHyperScript uses the parser implementation written in Go to parse HyperScript code.
// Only lexing is supported for now, therefore .Node is nil in the parsing result.
func ParseHyperScript(source string) (*hscode.ParsingResult, *hscode.ParsingError, error) {

	lexer := NewLexer()
	tokens, err := lexer.tokenize(source, false)
	if err != nil {
		parsingErr := &hscode.ParsingError{
			Message:        err.Error(),
			MessageAtToken: err.Error(),
			Tokens:         tokens,
		}

		if len(tokens) > 0 {
			parsingErr.Token = tokens[len(tokens)-1]
		}
		return nil, parsingErr, nil
	}

	result := &hscode.ParsingResult{
		Node:   hscode.Node{},
		Tokens: tokens,
	}

	result.TokensNoWhitespace = utils.FilterSlice(result.Tokens, isNotWhitespaceToken)

	return result, nil, nil
}
