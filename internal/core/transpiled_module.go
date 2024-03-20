package core

import (
	goast "go/ast"
)

// A TranspiledModule represents an Inox module transpiled to Golang, it does not hold any state and should NOT be modified.
type TranspiledModule struct {
	sourceModule        *Module
	pkg                 *goast.Package
	transpilationConfig ModuleTranspilationConfig
}
