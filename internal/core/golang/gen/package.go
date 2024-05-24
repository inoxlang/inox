package gen

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/inoxlang/inox/internal/utils/pathutils"
)

type Pkg struct {
	Pkg *ast.Package
}

func NewPkg(pkgName string) *Pkg {
	helper := &Pkg{
		Pkg: &ast.Package{
			Name:  pkgName,
			Scope: ast.NewScope(nil),
			Files: map[string]*ast.File{},
		},
	}

	return helper
}

func (p *Pkg) Name() string {
	return p.Pkg.Name
}

func (p *Pkg) AddFile(basename string, file *ast.File) {

	if basename != filepath.Base(basename) {
		panic(fmt.Errorf("%s is not file basename", basename))
	}

	_, ok := p.Pkg.Files[basename]
	if ok {
		panic(fmt.Errorf("%s is already present in the package %s", basename, p.Name()))
	}

	if file.Scope != nil && file.Scope.Outer != nil {
		panic(fmt.Errorf("the outer file's scope should not be alredy set"))
	}

	p.Pkg.Files[basename] = file
}

// WriteTo writes the package in $dir in the provided filesystem, no subpackage is written.
func (p *Pkg) WriteTo(dir string) error {

	if p.Pkg.Name != "main" {
		return fmt.Errorf("only main packages can be written to a filesystem for now")
	}

	pathSegments := pathutils.GetPathSegments(dir)
	pkgStack := []*ast.Package{p.Pkg}
	unusedFset := token.NewFileSet() //we nedd this for printing Go code.

	var visit visitFn
	var finalErr error

	visit = func(node ast.Node) (w ast.Visitor) {
		if finalErr != nil {
			return nil //prune
		}

		switch n := node.(type) {
		case *ast.Package:
			if n != p.Pkg {
				pkgStack = append(pkgStack, n)
				pathSegments = append(pathSegments, n.Name)
			}

			for basename, goFile := range n.Files {
				filePath := "/" + strings.Join(pathSegments, "/") + "/" + basename

				f, err := os.Create(filePath)

				if err == nil {
					defer f.Close()
					printer.Fprint(f, unusedFset, goFile)
				}

				if err != nil {
					finalErr = err
					return nil
				}
			}
		}

		return visit
	}

	ast.Walk(visit, p.Pkg)
	return nil
}
