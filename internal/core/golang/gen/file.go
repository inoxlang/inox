package gen

import "go/ast"

type File struct {
	f *ast.File
}

func NewFileHelper(pkgName string) *File {
	helper := &File{
		f: &ast.File{
			Scope: ast.NewScope(nil),
		},
	}

	if pkgName == "main" {
		helper.f.Name = MainPackageIdent
	} else {
		helper.f.Name = ast.NewIdent(pkgName)
	}

	return helper
}

func (f *File) AddDecl(decl ast.Decl) {
	f.f.Decls = append(f.f.Decls, decl)
}
