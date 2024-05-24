package core

import "github.com/inoxlang/inox/internal/ast"

func makeInvalidBinaryOperator(operator ast.BinaryOperator) string {
	return "invalid binary operator " + operator.String()
}
