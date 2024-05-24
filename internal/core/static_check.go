package core

import (
	"sync/atomic"

	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/maps"
)

func init() {
	staticcheck.CreateScheme = func(scheme string) staticcheck.Scheme {
		return Scheme(scheme)
	}
	staticcheck.CreateHost = func(host string) staticcheck.Host {
		return Host(host)
	}
	staticcheck.GetHostScheme = func(host staticcheck.Host) staticcheck.Scheme {
		return host.(Host).Scheme()
	}
	staticcheck.EvalSimpleValueLiteral = func(node ast.SimpleValueLiteral) (any, error) {
		return EvalSimpleValueLiteral(node, nil)
	}
	staticcheck.CheckQuantity = func(values []float64, units []string) error {
		_, err := evalQuantity(values, units)
		return err
	}
	staticcheck.GetCheckImportedModuleSourceName = func(sourceNode ast.Node, currentModule *inoxmod.Module, ctx inoxmod.Context) (string, error) {
		switch sourceNode.(type) {
		case *ast.URLLiteral, *ast.AbsolutePathLiteral, *ast.RelativePathLiteral:
			value, err := EvalSimpleValueLiteral(sourceNode.(ast.SimpleValueLiteral), nil)
			if err != nil {
				panic(ErrUnreachable)
			}
			src, err := inoxmod.GetSourceFromImportSource(value.(ResourceName), currentModule, ctx)
			if err != nil {
				return "", err
			}
			return src.ResourceName(), nil
		default:
			panic(ErrUnreachable)
		}
	}
	staticcheck.ErrNegQuantityNotSupported = ErrNegQuantityNotSupported
}

type StaticCheckInput struct {
	State  *GlobalState //mainly used when checking imported modules
	Node   ast.Node
	Module *Module
	Chunk  *parse.ParsedChunkSource

	Globals           GlobalVariables
	Patterns          map[string]struct{}
	PatternNamespaces map[string][]string

	AdditionalGlobalConsts []string
	ShellLocalVars         []string
}

func StaticCheck(input StaticCheckInput) (*StaticCheckData, error) {
	globals := map[string]staticcheck.GlobalVarInfo{}

	input.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
		globals[name] = staticcheck.GlobalVarInfo{IsConst: true, IsStartConstant: true}
		return nil
	})

	//Base globals and patterns for imported modules.

	basePatterns, basePatternNamespaces := input.State.GetBasePatternsForImportedModule()

	baseGlobals := map[string]staticcheck.GlobalVarInfo{}
	for name := range input.State.SymbolicBaseGlobalsForImportedModule {
		baseGlobals[name] = staticcheck.GlobalVarInfo{IsConst: true, IsStartConstant: true}
	}

	basePatternNamespacePatterns := map[string][]string{}

	for name, namespace := range basePatternNamespaces {
		basePatternNamespacePatterns[name] = maps.Keys(namespace.Patterns)
	}

	//Module

	var mod *inoxmod.Module
	if input.Module != nil {
		mod = input.Module.Lower()
	}

	data, err := staticcheck.Check(staticcheck.Input{
		Node:   input.Node,
		Chunk:  input.Chunk,
		Module: mod,

		GlobalsInfo:       globals,
		CheckContext:      input.State.Ctx,
		Patterns:          input.Patterns,
		PatternNamespaces: input.PatternNamespaces,

		BasePatternsForImportedModule:          utils.KeySet(basePatterns),
		BasePatternNamespacesForImportedModule: basePatternNamespacePatterns,
		BaseGlobalsForImportedModule:           baseGlobals,

		ShellLocalVars:         input.ShellLocalVars,
		AdditionalGlobalConsts: input.AdditionalGlobalConsts,
	})

	if data == nil {
		return nil, err
	}

	return &StaticCheckData{
		Data: data,
	}, err
}

type StaticallyCheckHostDefinitionFn func(node ast.Node) (errorMsg string)

func RegisterStaticallyCheckHostDefinitionFn(scheme Scheme, fn StaticallyCheckHostDefinitionFn) {
	staticcheck.RegisterStaticallyCheckHostDefinitionFn(scheme, func(node ast.Node) (errorMsg string) {
		return fn(node)
	})
}

type StaticCheckData struct {
	*staticcheck.Data

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple

	//.warnings property accessible from scripts
	warningsPropSet atomic.Bool
	warningsProp    *Tuple
}

func (d *StaticCheckData) ErrorTuple() *Tuple {
	if d.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Serializable, len(d.Errors()))
		for i, err := range d.Errors() {
			errors[i] = NewError(err, createRecordFromSourcePositionStack(err.Location))
		}
		d.errorsProp = NewTuple(errors)
	}
	return d.errorsProp
}

func (d *StaticCheckData) WarningTuple() *Tuple {
	if d.warningsPropSet.CompareAndSwap(false, true) {
		warnings := make([]Serializable, len(d.Warnings()))
		for i, warning := range d.Warnings() {
			warnings[i] = String(warning.LocatedMessage)
		}
		d.warningsProp = NewTuple(warnings)
	}
	return d.warningsProp
}

func (d *StaticCheckData) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *StaticCheckData) Prop(ctx *Context, name string) Value {
	switch name {
	case "errors":
		return d.ErrorTuple()
	case "warnings":
		return d.WarningTuple()
	}

	method, ok := d.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, d))
	}
	return method
}

func (*StaticCheckData) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*StaticCheckData) PropertyNames(ctx *Context) []string {
	return symbolic.STATIC_CHECK_DATA_PROP_NAMES
}
