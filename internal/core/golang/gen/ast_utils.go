package gen

import "go/ast"

var (
	Nil       = ast.NewIdent("nil")
	MainIdent = ast.NewIdent("main")
)

func Ret(expr ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{
		Results: []ast.Expr{expr},
	}
}
