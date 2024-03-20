package gen

import "go/ast"

type Pkg struct {
	pkg *ast.Package
}

func NewPkg(pkgName string) *Pkg {
	helper := &Pkg{
		pkg: &ast.Package{
			Name:  pkgName,
			Scope: ast.NewScope(nil),
		},
	}

	return helper
}

func (p *Pkg) Name() string {
	return p.pkg.Name
}
