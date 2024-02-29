package hsparse

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/stretchr/testify/assert"
)

func TestParseHyperscriptSlow(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		res, parsingErr, criticalError := parseHyperScriptSlow(context.Background(), "on click toggle .red on me ")

		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Nil(t, parsingErr) {
			return
		}

		assert.Greater(t, len(res.Tokens), 6)
		assert.Len(t, res.TokensNoWhitespace, 6)

		assert.EqualValues(t, hscode.HyperscriptProgram, res.NodeData["type"])
	})

	t.Run("unexpected token", func(t *testing.T) {
		res, parsingErr, criticalError := parseHyperScriptSlow(context.Background(), "on click x .red on me")

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
