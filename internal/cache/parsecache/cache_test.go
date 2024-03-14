package parsecache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCache(t *testing.T) {

	cache := New[string, string]()

	sourceCodeA := "A"
	sourceCodeB := "B"

	parsedA := "parsed-A"
	parsedB := "parsed-B"

	//Add and retrieve an entry.

	cache.Put("/a", sourceCodeA, &parsedA, "error-A")
	res, ok := cache.GetResult(sourceCodeA)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, &parsedA, res)

	res, data, ok := cache.GetResultAndDataByPathSourcePair("/a", sourceCodeA)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, &parsedA, res)
	assert.Equal(t, "error-A", data)

	//Add and retrieve another entry.

	cache.Put("/b", sourceCodeB, &parsedB, "error-B")
	res, ok = cache.GetResult(sourceCodeB)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, &parsedB, res)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/b", sourceCodeB)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, &parsedB, res)
	assert.Equal(t, "error-B", data)

	//Invalidate the cache.

	cache.InvalidateAllEntries()

	//Check that no entries is present.

	_, ok = cache.GetResult(sourceCodeA)
	assert.False(t, ok)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/a", sourceCodeA)
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Zero(t, data)

	_, ok = cache.GetResult(sourceCodeB)
	assert.False(t, ok)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/b", sourceCodeA)
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Zero(t, data)

	//Add, remove using DeleteEntriesByParsingResult, and try to retrieve an entry.

	cache.Put("/a", sourceCodeA, &parsedA, "error-A")
	cache.DeleteEntriesByParsingResult(&parsedA)

	_, ok = cache.GetResult(sourceCodeA)
	assert.False(t, ok)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/a", sourceCodeA)
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Zero(t, data)

	assert.Empty(t, cache.additionalDataEntries) //internal check

	//Add, remove using KeepEntriesByValue and try to retrieve an entry.

	cache.Put("/a", sourceCodeA, &parsedA, "error-A")
	cache.KeepEntriesByParsingResult( /* nothing is kept*/ )

	_, ok = cache.GetResult(sourceCodeA)
	assert.False(t, ok)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/a", sourceCodeA)
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Zero(t, data)

	assert.Empty(t, cache.additionalDataEntries) //internal check

	//Add two entries with the same result and only keep one of them using KeepEntriesByPath.

	cache.Put("/a", sourceCodeA, &parsedA, "error-A")
	cache.Put("/b", sourceCodeA, &parsedA, "error-B")
	cache.KeepEntriesByPath("/b")
	assert.Len(t, cache.additionalDataEntries, 1) //internal check
	assert.Len(t, cache.resultEntries, 1)         //internal check

	//Check that /b is present but not /a.
	result, ok := cache.GetResult(sourceCodeA)
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, &parsedA, result)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/a", sourceCodeA)
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Zero(t, data)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/b", sourceCodeA)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, &parsedA, res)
	assert.Equal(t, "error-B", data)

	cache.InvalidateAllEntries()

	//Put two different parsing results for the same path.

	cache.Put("/a", sourceCodeA, &parsedA, "error-A")
	cache.Put("/a", sourceCodeB, &parsedB, "error-A")

	//Check that the previous version of the entry is no long present.

	_, ok = cache.GetResult(sourceCodeA)
	assert.False(t, ok)

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/a", sourceCodeA)
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Zero(t, data)

	//Check that the new version of the entry is present.

	res, data, ok = cache.GetResultAndDataByPathSourcePair("/a", sourceCodeB)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, &parsedB, res)
	assert.Equal(t, "error-A", data)
}
