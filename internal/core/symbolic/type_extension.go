package symbolic

import (
	"github.com/inoxlang/inox/internal/ast"
)

type TypeExtension struct {
	//formatted location of the extend statement that defines the extension.
	//TODO: maje sure it's a truly unique id
	Id        string
	Statement *ast.ExtendStatement

	ExtendedPattern Pattern

	PropertyExpressions []propertyExpression
}

type propertyExpression struct {
	Name string

	Expression ast.Node
	Method     *InoxFunction
}
