package core

import (
	"sync/atomic"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/sourcecode"
)

const (
	UnspecifiedModuleKind = inoxmod.UnspecifiedModuleKind
	SpecModule            = inoxmod.SpecModule
	ApplicationModule     = inoxmod.ApplicationModule
	UserLThreadModule     = inoxmod.UserLThreadModule
	TestSuiteModule       = inoxmod.TestSuiteModule
	TestCaseModule        = inoxmod.TestCaseModule
)

var (
	ParseLocalIncludedFiles     = inoxmod.ParseLocalIncludedFiles
	SOURCE_POS_RECORD_PROPNAMES = []string{"source", "line", "column", "start", "end"}

	ErrInvalidModuleKind               = inoxmod.ErrInvalidModuleKind
	ErrParsingErrorInManifestOrPreinit = inoxmod.ErrParsingErrorInManifestOrPreinit
)

func init() {
	inoxmod.CreatePath = func(absolutePath string) inoxmod.ResourceName {
		return Path(absolutePath)
	}
	inoxmod.CreateURL = func(url string) inoxmod.ResourceName {
		return URL(url)
	}
	inoxmod.CreateReadFilePermission = func(absolutePath string) permbase.Permission {
		return CreateFsReadPerm(Path(absolutePath))
	}
	inoxmod.CreateHttpReadPermission = func(url string) permbase.Permission {
		return CreateHttpReadPerm(URL(url))
	}
	inoxmod.EvalResourceNameLiteral = func(svl ast.SimpleValueLiteral) (inoxmod.ResourceName, error) {
		res, err := EvalSimpleValueLiteral(svl, nil)
		if err != nil {
			return nil, err
		}
		return res.(ResourceName), nil
	}
	inoxmod.CreateBoundChildCtx = func(ctx inoxmod.Context) inoxmod.Context {
		return ctx.(*Context).BoundChild()
	}
}

type ModuleLower = inoxmod.Module
type ModuleKind = inoxmod.Kind
type IncludedChunk = inoxmod.IncludedChunk
type ModuleParsingConfig = inoxmod.ModuleParsingConfig
type InMemoryModuleParsingConfig = inoxmod.InMemoryModuleParsingConfig
type IncludedFilesParsingConfig = inoxmod.IncludedFilesParsingConfig

type Module struct {
	*inoxmod.Module

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

func WrapLowerModule(mod *inoxmod.Module) *Module {
	return &Module{
		Module: mod,
	}
}

func ParseLocalModule(fpath string, config ModuleParsingConfig) (*Module, error) {
	mod, err := inoxmod.ParseLocalModule(fpath, config)
	if err != nil && mod == nil {
		return nil, err
	}
	return WrapLowerModule(mod), err
}

func ParseInMemoryModule(codeString string, config InMemoryModuleParsingConfig) (*Module, error) {
	mod, err := inoxmod.ParseInMemoryModule(codeString, config)
	if err != nil && mod == nil {
		return nil, err
	}
	return WrapLowerModule(mod), err
}

func (mod *Module) Lower() *inoxmod.Module {
	return mod.Module
}

func (mod *Module) SourceName() ResourceName {
	return mod.Module.SourceName().(ResourceName)
}

func (mod *Module) ToSymbolic() *symbolic.Module {
	inclusionStmtMap := make(map[*ast.InclusionImportStatement]*symbolic.IncludedChunk, len(mod.IncludedChunkMap))
	importedModuleMap := make(map[*ast.ImportStatement]*symbolic.Module)

	for k, v := range mod.InclusionStatementMap {
		inclusionStmtMap[k] = &symbolic.IncludedChunk{
			ParsedChunkSource: v.ParsedChunkSource,
		}
	}

	for k, v := range mod.DirectlyImportedModulesByStatement {
		importedModuleMap[k] = (&Module{Module: v}).ToSymbolic()
	}

	return symbolic.NewModule(mod.MainChunk, inclusionStmtMap, importedModuleMap)
}

func (m *Module) ParsingErrorTuple() *Tuple {
	if m.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Serializable, len(m.Errors))
		for i, err := range m.Errors {
			errors[i] = NewError(err, String(err.AdditionalInfo))
		}
		m.errorsProp = NewTuple(errors)
	}
	return m.errorsProp
}

func (*Module) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (m *Module) Prop(ctx *Context, name string) Value {
	switch name {
	case "parsing-errors":
		return m.ParsingErrorTuple()
	case "main-chunk-node":
		return AstNode{Node: m.MainChunk.Node}
	}

	method, ok := m.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, m))
	}
	return method
}

func (*Module) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*Module) PropertyNames(ctx *Context) []string {
	return symbolic.MODULE_PROP_NAMES
}

func CreateRecordFromSourcePosition(pos sourcecode.PositionRange) *Record {
	rec := NewRecordFromKeyValLists(
		SOURCE_POS_RECORD_PROPNAMES,
		[]Serializable{String(pos.SourceName), Int(pos.StartLine), Int(pos.StartColumn), Int(pos.Span.Start), Int(pos.Span.End)},
	)
	return rec
}

func createRecordFromSourcePositionStack(posStack sourcecode.PositionStack) *Record {
	positionRecords := make([]Serializable, len(posStack))

	for i, pos := range posStack {
		positionRecords[i] = CreateRecordFromSourcePosition(pos)
	}

	return NewRecordFromKeyValLists([]string{"position-stack"}, []Serializable{NewTuple(positionRecords)})
}
