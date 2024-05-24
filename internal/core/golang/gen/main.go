package gen

import (
	"go/ast"
	"go/token"
)

func NewMainFile() *ast.File {
	return &ast.File{
		Package: token.Pos(0),
		Name:    MainIdent,
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Name: MainIdent,
				Body: &ast.BlockStmt{
					Lbrace: token.Pos(10),
					List:   []ast.Stmt{},
					Rbrace: token.Pos(100),
				},
			},
		},
	}
}
