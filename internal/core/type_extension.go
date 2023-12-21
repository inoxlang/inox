package core

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
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

type propertyExpression struct {
	name string

	expression parse.Node
	method     *InoxFunction
}
