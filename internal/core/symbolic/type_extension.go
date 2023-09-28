package symbolic

import parse "github.com/inoxlang/inox/internal/parse"

type TypeExtension struct {
	extendedPattern Pattern

	propertyExpressions []propertyExpression
}

type propertyExpression struct {
	name string

	expression parse.Node
	method     *InoxFunction
}
