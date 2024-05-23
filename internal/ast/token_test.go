package ast_test

import (
	"fmt"
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	"github.com/stretchr/testify/assert"
)

type NodeSpan = sourcecode.NodeSpan

func TestGetTokenAtPosition(t *testing.T) {

	testCases := []struct {
		input         string
		position      int
		expectedToken ast.Token
	}{
		{"1;2", 0, ast.Token{Type: ast.INT_LITERAL, Span: NodeSpan{Start: 0, End: 1}, Raw: "1"}},
		{"1;2", 1, ast.Token{Type: ast.SEMICOLON, Span: NodeSpan{1, 2}, Raw: ";"}},
		{"1;2", 2, ast.Token{Type: ast.INT_LITERAL, Span: NodeSpan{2, 3}, Raw: "2"}},

		{"12;3", 0, ast.Token{Type: ast.INT_LITERAL, Span: NodeSpan{0, 2}, Raw: "12"}},
		{"12;3", 1, ast.Token{Type: ast.INT_LITERAL, Span: NodeSpan{0, 2}, Raw: "12"}},
		{"12;3", 2, ast.Token{Type: ast.SEMICOLON, Span: NodeSpan{Start: 2, End: 3}, Raw: ";"}},

		{"[1 2]", 0, ast.Token{Type: ast.OPENING_BRACKET, Span: NodeSpan{0, 1}, Raw: "["}},
		{"[1 2]", 1, ast.Token{Type: ast.INT_LITERAL, Span: NodeSpan{Start: 1, End: 2}, Raw: "1"}},

		{":{./a: 1}", 2, ast.Token{Type: ast.RELATIVE_PATH_LITERAL, Span: NodeSpan{2, 5}, Raw: "./a"}},

		{"-a=1", 2, ast.Token{Type: ast.EQUAL, SubType: ast.FLAG_EQUAL, Span: NodeSpan{2, 3}, Raw: "="}},
		{"%-a=1", 3, ast.Token{Type: ast.EQUAL, SubType: ast.FLAG_EQUAL, Span: NodeSpan{3, 4}, Raw: "="}},

		{"a=1", 1, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{1, 2}, Raw: "="}},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%#v", testCase), func(t *testing.T) {
			chunk := parse.MustParseChunk(testCase.input)
			token, ok := ast.GetTokenAtPosition(testCase.position, chunk, chunk)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, testCase.expectedToken, token)
		})
	}

}
