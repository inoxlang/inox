package staticcheck

import (
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// A Data is the immutable data produced by statically checking a module.
type Data struct {
	combinedErrors error
	errors         []*Error
	warnings       []*StaticCheckWarning
	fnData         map[*parse.FunctionExpression]*FunctionData
	mappingData    map[*parse.MappingExpression]*MappingData

	//key: *parse.Chunk|*parse.EmbeddedModule
	firstForbiddenPosForGlobalElementDecls map[parse.Node]int32
	functionsToDeclareEarly                map[parse.Node]*[]*parse.FunctionDeclaration
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

func (data *Data) addFnCapturedGlobal(fnExpr *parse.FunctionExpression, name string, optionalInfo *GlobalVarInfo) {
	fnData := data.fnData[fnExpr]
	if fnData == nil {
		fnData = &FunctionData{}
		data.fnData[fnExpr] = fnData
	}

	if !utils.SliceContains(fnData.capturedGlobals, name) {
		fnData.capturedGlobals = append(fnData.capturedGlobals, name)
	}

	if optionalInfo != nil && optionalInfo.FnExpr != nil {
		capturedGlobalFnData := data.GetFnData(optionalInfo.FnExpr)
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

func (data *Data) addMappingCapturedGlobal(expr *parse.MappingExpression, name string) {
	mappingData := data.mappingData[expr]
	if mappingData == nil {
		mappingData = &MappingData{}
		data.mappingData[expr] = mappingData
	}

	if !utils.SliceContains(mappingData.referencedGlobals, name) {
		mappingData.referencedGlobals = append(mappingData.referencedGlobals, name)
	}
}

func (data *Data) GetFnData(fnExpr *parse.FunctionExpression) *FunctionData {
	return data.fnData[fnExpr]
}

func (data *Data) GetMappingData(expr *parse.MappingExpression) *MappingData {
	return data.mappingData[expr]
}

func (data *Data) GetEarlyFunctionDeclarationsPosition(module parse.Node) (int32, bool) {
	switch module.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		panic(fmt.Errorf("node is a not a module, type is: %T", module))
	}

	pos, ok := data.firstForbiddenPosForGlobalElementDecls[module]
	return pos, ok
}

func (data *Data) GetFunctionsToDeclareEarly(module parse.Node) []*parse.FunctionDeclaration {
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
