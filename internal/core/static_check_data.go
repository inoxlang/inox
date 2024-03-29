package core

import (
	"fmt"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// A StaticCheckData is the immutable data produced by statically checking a module.
type StaticCheckData struct {
	combinedErrors error
	errors         []*StaticCheckError
	warnings       []*StaticCheckWarning
	fnData         map[*parse.FunctionExpression]*FunctionStaticData
	mappingData    map[*parse.MappingExpression]*MappingStaticData

	//key: *parse.Chunk|*parse.EmbeddedModule
	firstForbiddenPosForGlobalElementDecls map[parse.Node]int32
	functionsToDeclareEarly                map[parse.Node]*[]*parse.FunctionDeclaration

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple

	//.warnings property accessible from scripts
	warningsPropSet atomic.Bool
	warningsProp    *Tuple
}

// Errors returns all errors in the code after a static check, the result should not be modified.
func (d *StaticCheckData) Errors() []*StaticCheckError {
	return d.errors
}

func (d *StaticCheckData) CombinedErrors() error {
	return d.combinedErrors
}

func (d *StaticCheckData) ErrorTuple() *Tuple {
	if d.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Serializable, len(d.errors))
		for i, err := range d.errors {
			errors[i] = err.Err()
		}
		d.errorsProp = NewTuple(errors)
	}
	return d.errorsProp
}

// Warnings returns all warnings in the code after a static check, the result should not be modified.
func (d *StaticCheckData) Warnings() []*StaticCheckWarning {
	return d.warnings
}

func (d *StaticCheckData) WarningTuple() *Tuple {
	if d.warningsPropSet.CompareAndSwap(false, true) {
		warnings := make([]Serializable, len(d.warnings))
		for i, warning := range d.warnings {
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
	return STATIC_CHECK_DATA_PROP_NAMES
}

type FunctionStaticData struct {
	capturedGlobals []string
}

func NewFunctionStaticData(capturedGlobals []string) *FunctionStaticData {
	return &FunctionStaticData{
		capturedGlobals: capturedGlobals,
	}
}

type MappingStaticData struct {
	referencedGlobals []string
}

func NewMappingStaticData(referencedGlobals []string) *MappingStaticData {
	return &MappingStaticData{
		referencedGlobals: referencedGlobals,
	}
}

func (data *StaticCheckData) addFnCapturedGlobal(fnExpr *parse.FunctionExpression, name string, optionalInfo *globalVarInfo) {
	fnData := data.fnData[fnExpr]
	if fnData == nil {
		fnData = &FunctionStaticData{}
		data.fnData[fnExpr] = fnData
	}

	if !utils.SliceContains(fnData.capturedGlobals, name) {
		fnData.capturedGlobals = append(fnData.capturedGlobals, name)
	}

	if optionalInfo != nil && optionalInfo.fnExpr != nil {
		capturedGlobalFnData := data.GetFnData(optionalInfo.fnExpr)
		if capturedGlobalFnData != nil {
			for _, name := range capturedGlobalFnData.capturedGlobals {
				if utils.SliceContains(fnData.capturedGlobals, name) {
					continue
				}

				fnData.capturedGlobals = append(fnData.capturedGlobals, name)
			}
		}
	}
}

func (data *StaticCheckData) addMappingCapturedGlobal(expr *parse.MappingExpression, name string) {
	mappingData := data.mappingData[expr]
	if mappingData == nil {
		mappingData = &MappingStaticData{}
		data.mappingData[expr] = mappingData
	}

	if !utils.SliceContains(mappingData.referencedGlobals, name) {
		mappingData.referencedGlobals = append(mappingData.referencedGlobals, name)
	}
}

func (data *StaticCheckData) GetFnData(fnExpr *parse.FunctionExpression) *FunctionStaticData {
	return data.fnData[fnExpr]
}

func (data *StaticCheckData) GetMappingData(expr *parse.MappingExpression) *MappingStaticData {
	return data.mappingData[expr]
}

func (data *StaticCheckData) GetEarlyFunctionDeclarationsPosition(module parse.Node) (int32, bool) {
	switch module.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		panic(fmt.Errorf("node is a not a module, type is: %T", module))
	}

	pos, ok := data.firstForbiddenPosForGlobalElementDecls[module]
	return pos, ok
}

func (data *StaticCheckData) GetFunctionsToDeclareEarly(module parse.Node) []*parse.FunctionDeclaration {
	switch module.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		panic(fmt.Errorf("node is a not a module, type is: %T", module))
	}

	decls, ok := data.functionsToDeclareEarly[module]
	if ok {
		return *decls
	}
	return nil
}
