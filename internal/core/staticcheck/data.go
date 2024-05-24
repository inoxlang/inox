package staticcheck

import (
	"fmt"
	"slices"

	"github.com/inoxlang/inox/internal/ast"
)

// A Data is the immutable data produced by statically checking a module.
type Data struct {
	combinedErrors error
	errors         []*Error
	warnings       []*StaticCheckWarning
	fnData         map[*ast.FunctionExpression]*FunctionData
	mappingData    map[*ast.MappingExpression]*MappingData

	//key: *ast.Chunk|*ast.EmbeddedModule
	firstForbiddenPosForGlobalElementDecls map[ast.Node]int32
	functionsToDeclareEarly                map[ast.Node]*[]*ast.FunctionDeclaration
}

// Errors returns all errors in the code after a static check, the result should not be modified.
func (d *Data) Errors() []*Error {
	return d.errors
}

func (d *Data) CombinedErrors() error {
	return d.combinedErrors
}

// Warnings returns all warnings in the code after a static check, the result should not be modified.
func (d *Data) Warnings() []*StaticCheckWarning {
	return d.warnings
}

type FunctionData struct {
	capturedGlobals []string
}

func NewFunctionStaticData(capturedGlobals []string) *FunctionData {
	return &FunctionData{
		capturedGlobals: capturedGlobals,
	}
}

func (d FunctionData) CapturedGlobals() []string {
	return d.capturedGlobals
}

type MappingData struct {
	referencedGlobals []string
}

func NewMappingStaticData(referencedGlobals []string) *MappingData {
	return &MappingData{
		referencedGlobals: referencedGlobals,
	}
}

func (d MappingData) ReferencedGlobals() []string {
	return d.referencedGlobals
}

func (data *Data) addFnCapturedGlobal(fnExpr *ast.FunctionExpression, name string, optionalInfo *GlobalVarInfo) {
	fnData := data.fnData[fnExpr]
	if fnData == nil {
		fnData = &FunctionData{}
		data.fnData[fnExpr] = fnData
	}

	if !slices.Contains(fnData.capturedGlobals, name) {
		fnData.capturedGlobals = append(fnData.capturedGlobals, name)
	}

	if optionalInfo != nil && optionalInfo.FnExpr != nil {
		capturedGlobalFnData := data.GetFnData(optionalInfo.FnExpr)
		if capturedGlobalFnData != nil {
			for _, name := range capturedGlobalFnData.capturedGlobals {
				if slices.Contains(fnData.capturedGlobals, name) {
					continue
				}

				fnData.capturedGlobals = append(fnData.capturedGlobals, name)
			}
		}
	}
}

func (data *Data) addMappingCapturedGlobal(expr *ast.MappingExpression, name string) {
	mappingData := data.mappingData[expr]
	if mappingData == nil {
		mappingData = &MappingData{}
		data.mappingData[expr] = mappingData
	}

	if !slices.Contains(mappingData.referencedGlobals, name) {
		mappingData.referencedGlobals = append(mappingData.referencedGlobals, name)
	}
}

func (data *Data) GetFnData(fnExpr *ast.FunctionExpression) *FunctionData {
	return data.fnData[fnExpr]
}

func (data *Data) GetMappingData(expr *ast.MappingExpression) *MappingData {
	return data.mappingData[expr]
}

func (data *Data) GetEarlyFunctionDeclarationsPosition(module ast.Node) (int32, bool) {
	switch module.(type) {
	case *ast.Chunk, *ast.EmbeddedModule:
	default:
		panic(fmt.Errorf("node is a not a module, type is: %T", module))
	}

	pos, ok := data.firstForbiddenPosForGlobalElementDecls[module]
	return pos, ok
}

func (data *Data) GetFunctionsToDeclareEarly(module ast.Node) []*ast.FunctionDeclaration {
	switch module.(type) {
	case *ast.Chunk, *ast.EmbeddedModule:
	default:
		panic(fmt.Errorf("node is a not a module, type is: %T", module))
	}

	decls, ok := data.functionsToDeclareEarly[module]
	if ok {
		return *decls
	}
	return nil
}
