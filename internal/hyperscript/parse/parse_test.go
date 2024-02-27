package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHyperscript(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		res, parsingErr, criticalError := parseHyperscript("on click toggle .red on me")
		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.NoError(t, parsingErr) {
			return
		}
		_ = res
	})

	t.Run("unexpected token", func(t *testing.T) {
		res, parsingErr, criticalError := parseHyperscript("on click x .red on me")
		if !assert.NoError(t, criticalError) {
			return
		}

		if !assert.Error(t, parsingErr) {
			return
		}

		err := parsingErr.(*ParsingError)
		assert.Contains(t, err.Message, "Unexpected Token")
		assert.Equal(t, Token{
			Type:   "IDENTIFIER",
			Value:  "x",
			Start:  9,
			End:    10,
			Line:   1,
			Column: 10,
		}, err.Token)

		_ = res
	})
}
