package staticcheck

import (
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/parse"
)

var (
	CreateScheme                     func(scheme string) Scheme
	CreateHost                       func(host string) Host
	GetHostScheme                    func(host Host) Scheme
	EvalSimpleValueLiteral           func(node parse.SimpleValueLiteral) (any, error)
	CheckQuantity                    func(values []float64, units []string) error
	GetCheckImportedModuleSourceName func(sourceNode parse.Node, currentModule *inoxmod.Module, checkCtx inoxmod.Context) (string, error)
	ErrNegQuantityNotSupported       error
)
