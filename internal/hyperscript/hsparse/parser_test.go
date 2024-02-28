package hsparse

import (
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/stretchr/testify/assert"
)

const MAX_SMALL_PROGRAM_PARSING_DURATION = time.Millisecond

func TestParseHyperscript(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		startTime := time.Now()
		res, parsingErr, criticalError := ParseHyperScript("on click toggle .red on me")

		assert.Less(t, time.Now(), startTime.Add(MAX_SMALL_PROGRAM_PARSING_DURATION))

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		assert.Greater(t, len(res.Tokens), 6)
		assert.Len(t, res.TokensNoWhitespace, 6)

		//TODO:		assert.Equal(t, hscode.HyperscriptProgram, res.Node.Type)
	})

	t.Run("unexpected token", func(t *testing.T) {
		t.Skip("TODO: implement parser in Golang")

		startTime := time.Now()
		res, parsingErr, criticalError := ParseHyperScript("on click x .red on me")

		assert.Less(t, time.Now(), startTime.Add(MAX_SMALL_PROGRAM_PARSING_DURATION))

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.NotNil(t, parsingErr) {
			return
		}

		assert.Contains(t, parsingErr.Message, "unexpected token")
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
}
