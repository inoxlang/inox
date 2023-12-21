package symbolic

import "github.com/inoxlang/inox/internal/parse"

type TypeExtension struct {
	//formatted location of the extend statement that defines the extension.
	//TODO: maje sure it's a truly unique id
	Id        string
	Statement *parse.ExtendStatement

	ExtendedPattern Pattern

	PropertyExpressions []propertyExpression
}

type propertyExpression struct {
	Name string

	Expression parse.Node
	Method     *InoxFunction
}
