package hsparse

// // ParseHyperScript uses the parser implementation written in Go to parse HyperScript code.
// // Only lexing is supported for now, therefore .Node is nil in the parsing result.
// func ParseHyperScript(source string) (*hscode.ParsingResult, *hscode.ParsingError, error) {

// 	lexer := NewLexer()
// 	tokens, err := lexer.tokenize(source, false)
// 	if err != nil {
// 		parsingErr := &hscode.ParsingError{
// 			Message:        err.Error(),
// 			MessageAtToken: err.Error(),
// 			Tokens:         tokens,
// 		}

// 		if len(tokens) > 0 {
// 			parsingErr.Token = tokens[len(tokens)-1]
// 		}
// 		return nil, parsingErr, nil
// 	}

// 	result := &hscode.ParsingResult{
// 		Node:   hscode.Node{},
// 		Tokens: tokens,
// 	}
// 	result.TokensNoWhitespace = utils.FilterSlice(result.Tokens, isNotWhitespaceToken)

// 	parser := newParser()
// 	result.Node = parser.parseHyperScript(NewTokens(tokens, nil, []rune(source), source))

// 	return result, nil, nil
// }
