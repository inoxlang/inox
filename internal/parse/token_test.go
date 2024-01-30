package parse

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTokenAtPosition(t *testing.T) {

	testCases := []struct {
		input         string
		position      int
		expectedToken Token
	}{
		{"1;2", 0, Token{Type: INT_LITERAL, Span: NodeSpan{0, 1}, Raw: "1"}},
		{"1;2", 1, Token{Type: SEMICOLON, Span: NodeSpan{1, 2}, Raw: ";"}},
		{"1;2", 2, Token{Type: INT_LITERAL, Span: NodeSpan{2, 3}, Raw: "2"}},

		{"12;3", 0, Token{Type: INT_LITERAL, Span: NodeSpan{0, 2}, Raw: "12"}},
		{"12;3", 1, Token{Type: INT_LITERAL, Span: NodeSpan{0, 2}, Raw: "12"}},
		{"12;3", 2, Token{Type: SEMICOLON, Span: NodeSpan{2, 3}, Raw: ";"}},

		{"[1 2]", 0, Token{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}, Raw: "["}},
		{"[1 2]", 1, Token{Type: INT_LITERAL, Span: NodeSpan{1, 2}, Raw: "1"}},

		{":{./a: 1}", 2, Token{Type: RELATIVE_PATH_LITERAL, Span: NodeSpan{2, 5}, Raw: "./a"}},

		{"-a=1", 2, Token{Type: EQUAL, SubType: FLAG_EQUAL, Span: NodeSpan{2, 3}, Raw: "="}},
		{"%-a=1", 3, Token{Type: EQUAL, SubType: FLAG_EQUAL, Span: NodeSpan{3, 4}, Raw: "="}},

		{"a=1", 1, Token{Type: EQUAL, SubType: ASSIGN_EQUAL, Span: NodeSpan{1, 2}, Raw: "="}},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%#v", testCase), func(t *testing.T) {
			chunk := MustParseChunk(testCase.input)
			token, ok := GetTokenAtPosition(testCase.position, chunk, chunk)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, testCase.expectedToken, token)
		})
	}

}
