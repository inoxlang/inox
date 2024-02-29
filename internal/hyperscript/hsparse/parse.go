package hsparse

import (
	"context"
	"errors"
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
)

const (
	DEFAULT_MAX_SOURCE_CODE_LENGTH = 2_000
	MAX_CACHE_ENTRY_COUNT          = 10_000
)

var (
	cache     = lru.New(MAX_CACHE_ENTRY_COUNT)
	cacheLock sync.Mutex
)

func ParseHyperScript(ctx context.Context, source string) (parsingResult *hscode.ParsingResult, parsingErr *hscode.ParsingError, criticalError error) {

	if len(source) > DEFAULT_MAX_SOURCE_CODE_LENGTH {
		//Don't parse if the input is too large.
		return &hscode.ParsingResult{NodeData: map[string]any{}}, nil, nil
	}

	//Check the cache.

	cacheLock.Lock()
	cached, ok := cache.Get(source)
	cacheLock.Unlock()

	if ok {
		result, ok := cached.(*hscode.ParsingResult)
		if ok {
			return result, nil, nil
		}
		return nil, cached.(*hscode.ParsingError), nil
	}

	defer func() {
		//If there is no critical error, we cache the result or the parsing error.

		if parsingResult != nil {
			cacheLock.Lock()
			defer cacheLock.Unlock()
			cache.Add(source, parsingResult)
		}

		if parsingErr != nil {
			cacheLock.Lock()
			defer cacheLock.Unlock()
			cache.Add(source, parsingErr)
		}
	}()

	result, parsingErr, err := tryParseHyperScriptWithDenoService(ctx, source)

	if !errors.Is(err, ErrDenoServiceNotAvailable) {
		return result, parsingErr, err
	}

	return parseHyperScriptSlow(ctx, source)
}

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
