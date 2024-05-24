package gen

import (
	"go/ast"
	"go/token"
	"strconv"
)

var (
	Nil       = ast.NewIdent("nil")
	MainIdent = ast.NewIdent("main")
)

func Ret(expr ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{
		Results: []ast.Expr{expr},
	}
}

func IntLit(i int64) *ast.BasicLit {
	return &ast.BasicLit{
		Kind:  token.INT,
		Value: strconv.FormatInt(i, 10),
	}
}
