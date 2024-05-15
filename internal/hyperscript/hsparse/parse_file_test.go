package hsparse

import (
	"context"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/sourcecode"
	"github.com/stretchr/testify/assert"
)

func TestParseHyperscriptFile(t *testing.T) {

	ParseHyperScriptProgram(context.Background(), "on click toggle .red on me ") //create a VM

	t.Run("valid", func(t *testing.T) {
		chunkSource := sourcecode.File{
			NameString:  "/a._hs",
			Resource:    "/a._hs",
			ResourceDir: "/",
			CodeString:  "on click toggle .red on me",
		}
		file, err := ParseFile(context.Background(), chunkSource, nil, nil)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, file.Result) {
			return
		}

		if !assert.Nil(t, file.Error) {
			return
		}

		assert.Greater(t, len(file.Result.Tokens), 6)
		assert.Len(t, file.Result.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, file.Result.NodeData["type"])
	})

	t.Run("unexpected token", func(t *testing.T) {
		chunkSource := sourcecode.File{
			NameString:  "/a._hs",
			Resource:    "/a._hs",
			ResourceDir: "/",
			CodeString:  "on click x .red on me",
		}
		file, err := ParseFile(context.Background(), chunkSource, nil, nil)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Nil(t, file.Result) {
			return
		}

		if !assert.NotNil(t, file.Error) {
			return
		}

		assert.Greater(t, len(file.Error.Tokens), 6)
		assert.Len(t, file.Error.TokensNoWhitespace, 6)

		assert.Contains(t, file.Error.Message, "Unexpected Token")
		assert.Equal(t, hscode.Token{
			Type:   "IDENTIFIER",
			Value:  "x",
			Start:  9,
			End:    10,
			Line:   1,
			Column: 10,
		}, file.Error.Token)

	})

	t.Run("parse cache", func(t *testing.T) {
		chunkSource := sourcecode.File{
			NameString:  "/a._hs",
			Resource:    "/a._hs",
			ResourceDir: "/",
			CodeString:  "on click toggle .red on me",
		}

		cache := hscode.NewParseCache()

		file, err := ParseFile(context.Background(), chunkSource, cache, nil)

		//Check the result.

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, file.Result) {
			return
		}

		if !assert.Nil(t, file.Error) {
			return
		}

		assert.Greater(t, len(file.Result.Tokens), 6)
		assert.Len(t, file.Result.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, file.Result.NodeData["type"])

		//Check the cache.

		cachedResult, err, cacheHit := cache.GetResultAndDataByPathSourcePair(chunkSource.NameString, chunkSource.CodeString)

		if !assert.True(t, cacheHit) {
			return
		}

		if !assert.Same(t, file.Result, cachedResult) {
			return
		}

		assert.Nil(t, err)

		//Call ParseHyperscriptFile again.

		_file, _err := ParseFile(context.Background(), chunkSource, cache, nil)

		if !assert.NoError(t, _err) {
			return
		}

		//Check that the parsing results are the same.

		assert.Same(t, file.Result, _file.Result)

		//Modify the code source.

		chunkSource.CodeString = strings.ReplaceAll(chunkSource.CodeString, ".red", ".blue")

		_file, _err = ParseFile(context.Background(), chunkSource, cache, nil)

		//Check the result.

		if !assert.NoError(t, _err) {
			return
		}

		if !assert.NotNil(t, _file.Result) {
			return
		}

		if !assert.Nil(t, _file.Error) {
			return
		}

		if !assert.NotSame(t, _file.Result, cachedResult) {
			return
		}

		assert.Greater(t, len(_file.Result.Tokens), 6)
		assert.Len(t, _file.Result.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, _file.Result.NodeData["type"])

		//Check the cache.

		cachedResult, err, cacheHit = cache.GetResultAndDataByPathSourcePair(chunkSource.NameString, chunkSource.CodeString)

		if !assert.True(t, cacheHit) {
			return
		}

		if !assert.Same(t, _file.Result, cachedResult) {
			return
		}

		assert.Nil(t, err)
	})

	t.Run("file cache", func(t *testing.T) {
		chunkSource := sourcecode.File{
			NameString:  "/a._hs",
			Resource:    "/a._hs",
			ResourceDir: "/",
			CodeString:  "on click toggle .red on me",
		}

		fileCache := hscode.NewParsedFileCache()

		file, err := ParseFile(context.Background(), chunkSource, nil, fileCache)

		//Check the result.

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, file.Result) {
			return
		}

		if !assert.Nil(t, file.Error) {
			return
		}

		assert.Greater(t, len(file.Result.Tokens), 6)
		assert.Len(t, file.Result.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, file.Result.NodeData["type"])

		//Check the cache.

		cachedFile, err, cacheHit := fileCache.GetResultAndDataByPathSourcePair(chunkSource.NameString, chunkSource.CodeString)

		if !assert.True(t, cacheHit) {
			return
		}

		if !assert.Same(t, file, cachedFile) {
			return
		}

		assert.Nil(t, err)

		//Call ParseHyperscriptFile again.

		_file, _err := ParseFile(context.Background(), chunkSource, nil, fileCache)

		if !assert.NoError(t, _err) {
			return
		}

		//Check that the parsing results are the same.

		assert.Same(t, file.Result, _file.Result)

		//Modify the code source.

		chunkSource.CodeString = strings.ReplaceAll(chunkSource.CodeString, ".red", ".blue")

		_file, _err = ParseFile(context.Background(), chunkSource, nil, fileCache)

		//Check the result.

		if !assert.NoError(t, _err) {
			return
		}

		if !assert.NotNil(t, _file.Result) {
			return
		}

		if !assert.Nil(t, _file.Error) {
			return
		}

		if !assert.NotSame(t, _file, cachedFile) {
			return
		}

		assert.Greater(t, len(_file.Result.Tokens), 6)
		assert.Len(t, _file.Result.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, _file.Result.NodeData["type"])

		//Check the cache.

		cachedFile, err, cacheHit = fileCache.GetResultAndDataByPathSourcePair(chunkSource.NameString, chunkSource.CodeString)

		if !assert.True(t, cacheHit) {
			return
		}

		if !assert.Same(t, _file, cachedFile) {
			return
		}

		assert.Nil(t, err)
	})
}
