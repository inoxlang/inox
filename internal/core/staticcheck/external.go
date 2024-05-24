package staticcheck

import (
	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/inoxmod"
)

var (
	CreateScheme                     func(scheme string) Scheme
	CreateHost                       func(host string) Host
	GetHostScheme                    func(host Host) Scheme
	EvalSimpleValueLiteral           func(node ast.SimpleValueLiteral) (any, error)
	CheckQuantity                    func(values []float64, units []string) error
	GetCheckImportedModuleSourceName func(sourceNode ast.Node, currentModule *inoxmod.Module, checkCtx inoxmod.Context) (string, error)
	ErrNegQuantityNotSupported       error
)
