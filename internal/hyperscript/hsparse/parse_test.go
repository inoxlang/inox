package hsparse

import (
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/stretchr/testify/assert"
)

func TestParseHyperscript(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		res, parsingErr, criticalError := ParseHyperscript("on click toggle .red on me")
		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		assert.Greater(t, len(res.Tokens), 6)
		assert.Len(t, res.TokensNoWhitespace, 6)

		assert.Equal(t, hscode.HyperscriptProgram, res.Node.Type)
	})

	t.Run("unexpected token", func(t *testing.T) {
		res, parsingErr, criticalError := ParseHyperscript("on click x .red on me")
		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.NotNil(t, parsingErr) {
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
}
