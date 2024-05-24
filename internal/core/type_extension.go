package core

import (
	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

// A type extension represents a set of methods & computed properties for values
// matching a given pattern.
type TypeExtension struct {
	extendedPattern     Pattern
	propertyExpressions []propertyExpression

	symbolicExtension *symbolic.TypeExtension
}

func (e TypeExtension) Id() string {
	return e.symbolicExtension.Id
}

func (e TypeExtension) Symbolic() *symbolic.TypeExtension {
	return e.symbolicExtension
}

type propertyExpression struct {
	name string

	expression ast.Node
	method     *InoxFunction
}
