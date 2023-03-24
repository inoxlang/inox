package internal

import parse "github.com/inox-project/inox/internal/parse"

func makeInvalidBinaryOperator(operator parse.BinaryOperator) string {
	return "invalid binary operator " + operator.String()
}
