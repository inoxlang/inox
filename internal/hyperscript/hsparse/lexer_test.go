package hsparse

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLexer(t *testing.T) {

	t.Run("ok", func(t *testing.T) {
		const source = "on click toggle .red on me"
		res, _, _ := parseHyperScriptSlow(context.Background(), source)

		lexer := NewLexer()
		tokens, err := lexer.tokenize(source, false)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, res.Tokens, tokens)
	})

	t.Run("unknown token", func(t *testing.T) {
		t.Skip("TODO: fix")

		const source = "on click toggle . on me"
		_, parsingErr, _ := parseHyperScriptSlow(context.Background(), source)

		lexer := NewLexer()

		tokens, err := lexer.tokenize(source, false)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, parsingErr.Tokens, tokens)
	})
}
