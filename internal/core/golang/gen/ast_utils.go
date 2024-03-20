package gen

import "go/ast"

var (
	Nil              = ast.NewIdent("nil")
	MainPackageIdent = ast.NewIdent("package")
)

func Ret(expr ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{
		Results: []ast.Expr{expr},
	}
}
