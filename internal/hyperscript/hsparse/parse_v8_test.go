package hsparse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/stretchr/testify/assert"
)

func TestParseHyperscript(t *testing.T) {

	const MAX_PARSING_DURATION_FOR_SMALL_PIECE_OF_CODE = 3 * time.Millisecond

	ParseHyperScript(context.Background(), "on click toggle .red on me ") //create a VM

	t.Run("valid", func(t *testing.T) {
		start := time.Now()
		res, parsingErr, criticalError := ParseHyperScript(context.Background(), "on click toggle .red on me")

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		if !assert.Less(t, time.Since(start), MAX_PARSING_DURATION_FOR_SMALL_PIECE_OF_CODE) {
			return
		}

		assert.Greater(t, len(res.Tokens), 6)
		assert.Len(t, res.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, res.NodeData["type"])
	})

	t.Run("unexpected token", func(t *testing.T) {
		start := time.Now()
		res, parsingErr, criticalError := ParseHyperScript(context.Background(), "on click x .red on me")

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.NotNil(t, parsingErr) {
			return
		}

		if !assert.Less(t, time.Since(start), MAX_PARSING_DURATION_FOR_SMALL_PIECE_OF_CODE) {
			return
		}

		assert.Contains(t, parsingErr.Message, "Unexpected Token")
		assert.Equal(t, hscode.Token{
			Type:   "IDENTIFIER",
			Value:  "x",
			Start:  9,
			End:    10,
			Line:   1,
			Column: 10,
		}, parsingErr.Token)

		_ = res
	})

	t.Run("long code but shorter than MAX_SOURCE_CODE_LENGTH", func(t *testing.T) {
		start := time.Now()
		repeatCount := 100
		source := strings.Repeat("on click toggle .red on me end\n", repeatCount)

		if len(source) >= MAX_SOURCE_CODE_LENGTH {
			t.FailNow()
		}

		res, parsingErr, criticalError := ParseHyperScript(context.Background(), source)

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		if !assert.Less(t, time.Since(start), MAX_PARSING_DURATION_FOR_SMALL_PIECE_OF_CODE*time.Millisecond) {
			return
		}

		assert.Greater(t, len(res.Tokens), 7*repeatCount)
		assert.Len(t, res.TokensNoWhitespace, 7*repeatCount)

		assert.EqualValues(t, hscode.HyperscriptProgram, res.NodeData["type"])
	})

	t.Run("string with back ticks", func(t *testing.T) {
		start := time.Now()
		res, parsingErr, criticalError := ParseHyperScript(context.Background(), "init set :a to `s`")

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		if !assert.Less(t, time.Since(start), MAX_PARSING_DURATION_FOR_SMALL_PIECE_OF_CODE) {
			return
		}

		assert.Greater(t, len(res.Tokens), 6)
		assert.Len(t, res.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, res.NodeData["type"])
	})

	t.Run("string template with back ticks", func(t *testing.T) {
		start := time.Now()
		res, parsingErr, criticalError := ParseHyperScript(context.Background(), "init set :a to `{s}`")

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		if !assert.Less(t, time.Since(start), MAX_PARSING_DURATION_FOR_SMALL_PIECE_OF_CODE) {
			return
		}

		assert.Greater(t, len(res.Tokens), 6)
		assert.Len(t, res.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, res.NodeData["type"])
	})

}
