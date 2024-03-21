package gen

import "go/ast"

type visitFn func(node ast.Node) (w ast.Visitor)

func (fn visitFn) Visit(node ast.Node) (w ast.Visitor) {
	return fn(node)
}
