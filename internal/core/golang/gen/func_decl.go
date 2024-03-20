package gen

import "go/ast"

type FuncDecl struct {
	decl *ast.FuncDecl
}

func NewFuncDeclHelper(name string) *FuncDecl {
	decl := &FuncDecl{
		decl: &ast.FuncDecl{
			Name: ast.NewIdent(name),
			Type: &ast.FuncType{
				Params: &ast.FieldList{},
			},
			Body: &ast.BlockStmt{},
		},
	}

	return decl
}

func (d *FuncDecl) AddParam(name string, typ ast.Expr) {
	params := d.params()
	params.List = append(params.List, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type:  typ,
	})
}

func (d *FuncDecl) AddStmt(stmt ast.Stmt) {
	body := d.body()
	body.List = append(body.List, stmt)
}

func (d *FuncDecl) Node() *ast.FuncDecl {
	return d.decl
}

func (d *FuncDecl) params() *ast.FieldList {
	return d.decl.Type.Params
}

func (d *FuncDecl) body() *ast.BlockStmt {
	return d.decl.Body
}
