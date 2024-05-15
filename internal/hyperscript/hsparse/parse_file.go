package hsparse

import (
	"context"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/sourcecode"
)

// ParseFile parses an Hyperscript file. The error result is only set if a critical error happened, not a parsing error.
// $fileCache is checked first, then $cache.
func ParseFile(ctx context.Context, chunkSource sourcecode.ChunkSource, parseCache *hscode.ParseCache, fileCache *hscode.ParsedFileCache) (*hscode.ParsedFile, error) {
	var parsingResult *hscode.ParsingResult
	var parsingError *hscode.ParsingError

	chunkName := chunkSource.Name()
	sourceCode := chunkSource.Code()

	if fileCache != nil {
		file, criticalErr, ok := fileCache.GetResultAndDataByPathSourcePair(chunkName, sourceCode)
		if ok {
			return file, criticalErr
		}
	}

	if parseCache != nil {
		result, err, cacheHit := parseCache.GetResultAndDataByPathSourcePair(chunkSource.Name(), sourceCode)
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
		if parseCache != nil {
			parseCache.Put(chunkName, sourceCode, nil, criticalErr)
		}
		if fileCache != nil {
			fileCache.Put(chunkName, sourceCode, nil, criticalErr)
		}
		return nil, criticalErr
	}

	if parseCache != nil {
		parseCache.Put(chunkName, sourceCode, parsingResult, parsingError)
	}

	file := &hscode.ParsedFile{
		Result:                parsingResult,
		Error:                 parsingError,
		ParsedChunkSourceBase: sourcecode.MakeParsedChunkSourceBaseWithRunes(chunkSource, []rune(sourceCode)),
	}

	if fileCache != nil {
		fileCache.Put(chunkName, sourceCode, file, nil)
	}

	return file, nil
}
