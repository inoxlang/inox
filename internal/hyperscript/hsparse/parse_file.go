package hsparse

import (
	"context"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/sourcecode"
)

// ParseFile parses an Hyperscript file. The error result is only set if a critical error happened, not a parsing error.
// The cache is used to retrieve an *hscode.ParsingResult.
func ParseFile(ctx context.Context, chunkSource sourcecode.ChunkSource, cache *hscode.ParseCache) (*hscode.ParsedFile, error) {
	var parsingResult *hscode.ParsingResult
	var parsingError *hscode.ParsingError

	chunkName := chunkSource.Name()
	sourceCode := chunkSource.Code()

	if cache != nil {
		result, err, cacheHit := cache.GetResultAndDataByPathSourcePair(chunkSource.Name(), sourceCode)
		parsingError, _ = err.(*hscode.ParsingError)
		parsingResult = result

		if cacheHit {
			if parsingError == nil && parsingResult == nil { //critical error
				return nil, err
			}

			return &hscode.ParsedFile{
				Result:                parsingResult,
				Error:                 parsingError,
				ParsedChunkSourceBase: sourcecode.MakeParsedChunkSourceBaseWithRunes(chunkSource, []rune(sourceCode)),
			}, nil
		}
	}

	parsingResult, parsingError, criticalErr := ParseHyperScriptProgram(ctx, sourceCode)

	if criticalErr != nil {
		if cache != nil {
			cache.Put(chunkName, sourceCode, nil, criticalErr)
		}
		return nil, criticalErr
	}

	if cache != nil {
		cache.Put(chunkName, sourceCode, parsingResult, parsingError)
	}

	return &hscode.ParsedFile{
		Result:                parsingResult,
		Error:                 parsingError,
		ParsedChunkSourceBase: sourcecode.MakeParsedChunkSourceBaseWithRunes(chunkSource, []rune(sourceCode)),
	}, nil
}
