package gen

import "go/ast"

type File struct {
	F *ast.File
}

func NewFile(pkgName string) *File {
	helper := &File{
		F: &ast.File{
			Scope: ast.NewScope(nil),
		},
	}

	if pkgName == "main" {
		helper.F.Name = MainIdent
	} else {
		helper.F.Name = ast.NewIdent(pkgName)
	}

	return helper
}

func (f *File) AddDecl(decl ast.Decl) {
	f.F.Decls = append(f.F.Decls, decl)
}
