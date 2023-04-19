package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X = "included file path should not contain '..'"
)

var (
	MODULE_PROP_NAMES           = []string{"parsing_errors", "main_chunk_node"}
	SOURCE_POS_RECORD_PROPNAMES = []string{"source", "line", "column", "start", "end"}
)

// A Module represents an Inox module, it does not hold any state and should NOT be modified. Module implements Value.
type Module struct {
	NoReprMixin
	NotClonableMixin

	ModuleKind
	MainChunk                  *parse.ParsedChunk
	IncludedChunkForest        []*IncludedChunk
	FlattenedIncludedChunkList []*IncludedChunk
	InclusionStatementMap      map[*parse.InclusionImportStatement]*IncludedChunk
	IncludedChunkMap           map[string]*IncludedChunk
	ManifestTemplate           *parse.Manifest
	Bytecode                   *Bytecode
	ParsingErrors              []Error
	ParsingErrorPositions      []parse.SourcePositionRange

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

type ModuleKind int

const (
	UnspecifiedModuleKind ModuleKind = iota
	UserRoutineModule
	TestSuiteModule
	TestCaseModule
	LifetimeJobModule
)

func (k ModuleKind) IsEmbedded() bool {
	return k >= UserRoutineModule && k <= LifetimeJobModule
}

func (mod *Module) HasURLSource() bool {
	sourceFile, ok := mod.MainChunk.Source.(parse.SourceFile)
	return ok && sourceFile.IsResourceURL
}

func (mod *Module) HasResourceDir() bool {
	_, ok := mod.MainChunk.Source.(parse.SourceFile)
	return ok
}

func (mod *Module) ResourceDir() string {
	return mod.MainChunk.Source.(parse.SourceFile).ResourceDir
}

func (mod *Module) Name() string {
	return mod.MainChunk.Name()
}

func (mod *Module) IsCompiled() bool {
	return mod.Bytecode != nil
}

func (mod *Module) ToSymbolic() *symbolic.Module {
	inclusionStmtMap := make(map[*parse.InclusionImportStatement]*symbolic.IncludedChunk, len(mod.IncludedChunkMap))

	for k, v := range mod.InclusionStatementMap {
		inclusionStmtMap[k] = &symbolic.IncludedChunk{
			ParsedChunk: v.ParsedChunk,
		}
	}
	return symbolic.NewModule(mod.MainChunk, inclusionStmtMap)
}

type ManifestEvaluationConfig struct {
	GlobalConsts          *parse.GlobalConstantDeclarations
	RunningState          *TreeWalkState //optional
	DefaultLimitations    []Limitation
	AddDefaultPermissions bool
	HandleCustomType      CustomPermissionTypeHandler //optional
	IgnoreUnknownSections bool
}

func (m *Module) EvalManifest(config ManifestEvaluationConfig) (*Manifest, error) {
	if m.ManifestTemplate == nil {
		return &Manifest{}, nil
	}

	manifestObjLiteral, ok := m.ManifestTemplate.Object.(*parse.ObjectLiteral)
	if !ok {
		return &Manifest{}, nil
	}

	// check object literal
	{
		var checkErr []error
		checkManifestObject(manifestObjLiteral, config.IgnoreUnknownSections, func(n parse.Node, msg string) {
			checkErr = append(checkErr, errors.New(msg))
		})
		if len(checkErr) != 0 {
			return nil, fmt.Errorf("%s: failed to check manifest's object literal: %w", m.Name(), combineErrors(checkErr...))
		}
	}

	var state *TreeWalkState

	//we create a temporary state to evaluate some parts of the permissions
	if config.RunningState == nil {
		ctx := NewContext(ContextConfig{Permissions: []Permission{GlobalVarPermission{ReadPerm, "*"}}})
		for k, v := range DEFAULT_NAMED_PATTERNS {
			ctx.AddNamedPattern(k, v)
		}

		for k, v := range DEFAULT_PATTERN_NAMESPACES {
			ctx.AddPatternNamespace(k, v)
		}

		state = NewTreeWalkState(ctx, getGlobalsAccessibleFromManifest().EntryMap())

		if config.GlobalConsts != nil {
			for _, decl := range config.GlobalConsts.Declarations {
				state.SetGlobal(decl.Left.Name, utils.Must(TreeWalkEval(decl.Right, state)), GlobalConst)
			}
		}

	} else {
		state = config.RunningState
	}

	// evaluate object literal
	v, err := TreeWalkEval(m.ManifestTemplate.Object, state)
	if err != nil {
		if err != nil {
			return nil, fmt.Errorf("%s: failed to evaluate manifest object: %w", m.Name(), err)
		}
	}

	manifestObj := v.(*Object)

	manifest, err := createManifest(manifestObj, manifestObjectConfig{
		defaultLimitations:    config.DefaultLimitations,
		handleCustomType:      config.HandleCustomType,
		addDefaultPermissions: config.AddDefaultPermissions,
		//addDefaultPermissions: true,
		ignoreUnkownSections: config.IgnoreUnknownSections,
	})

	return manifest, err
}

func (m *Module) ParsingErrorTuple() *Tuple {
	if m.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Value, len(m.ParsingErrors))
		for i, err := range m.ParsingErrors {
			errors[i] = err
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
	case "parsing_errors":
		return m.ParsingErrorTuple()
	case "main_chunk_node":
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
	return MODULE_PROP_NAMES
}

type InMemoryModuleParsingConfig struct {
	Name    string
	Context *Context //this context is used to check permissions
}

func ParseInMemoryModule(codeString Str, config InMemoryModuleParsingConfig) (*Module, error) {
	src := parse.InMemorySource{
		NameString: config.Name,
		CodeString: string(codeString),
	}

	code, err := parse.ParseChunkSource(src)
	if err != nil && code == nil {
		return nil, fmt.Errorf("failed to parse in-memory module named '%s': %w", config.Name, err)
	}

	mod := &Module{
		MainChunk:        code,
		ManifestTemplate: code.Node.Manifest,
	}

	// add parsing errors to the module
	if err != nil {
		errorAggregation, ok := err.(*parse.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}

		mod.ParsingErrors = make([]Error, len(errorAggregation.Errors))
		mod.ParsingErrorPositions = make([]parse.SourcePositionRange, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			mod.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			mod.ParsingErrorPositions[i] = pos
		}
	}

	// add error if manifest is missing
	if code.Node.Manifest == nil {
		err := NewError(fmt.Errorf("missing manifest in in-memory module "+string(config.Name)), Str(config.Name))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
	}

	inclusionStmts := parse.FindNodes(code.Node, &parse.InclusionImportStatement{}, nil)

	// add error if there are inclusion statements
	if len(inclusionStmts) != 0 {
		err := NewError(fmt.Errorf("inclusion import statements found in in-memory module "+config.Name), Str(config.Name))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
	}

	return mod, combineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
}

type LocalModuleParsingConfig struct {
	ModuleFilepath string
	Context        *Context //this context is used to check permissions
	//DefaultLimitations          []Limitationr
	//CustomPermissionTypeHandler CustomPermissionTypeHandler
}

func ParseLocalModule(config LocalModuleParsingConfig) (*Module, error) {
	fpath := config.ModuleFilepath
	ctx := config.Context
	absPath, err := filepath.Abs(fpath)
	if err != nil {
		return nil, err
	}

	//read the script

	{
		readPerm := FilesystemPermission{Kind_: ReadPerm, Entity: Path(absPath)}
		if err := ctx.CheckHasPermission(readPerm); err != nil {
			return nil, fmt.Errorf("failed to parse local module: %w", err)
		}
	}

	if info, err := os.Stat(fpath); err == os.ErrNotExist || (err == nil && info.IsDir()) {
		return nil, fmt.Errorf("%s does not exist or is a folder", fpath)
	}

	b, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %s", fpath, err)
	}

	//parse

	src := parse.SourceFile{
		NameString:    fpath,
		Resource:      fpath,
		CodeString:    string(b),
		ResourceDir:   filepath.Dir(absPath),
		IsResourceURL: false,
	}

	code, err := parse.ParseChunkSource(src)
	if err != nil && code == nil {
		return nil, fmt.Errorf("failed to parse %s: %w", fpath, err)
	}

	if err != nil {
		//log.Println(parse.GetTreeView(code.Node))
	}

	mod := &Module{
		MainChunk:             code,
		ManifestTemplate:      code.Node.Manifest,
		InclusionStatementMap: make(map[*parse.InclusionImportStatement]*IncludedChunk),
		IncludedChunkMap:      map[string]*IncludedChunk{},
	}

	// add parsing errors to the module
	if err != nil {
		errorAggregation, ok := err.(*parse.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}

		mod.ParsingErrors = make([]Error, len(errorAggregation.Errors))
		mod.ParsingErrorPositions = make([]parse.SourcePositionRange, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			mod.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			mod.ParsingErrorPositions[i] = pos
		}
	}

	// add error if manifest is missing
	if code.Node.Manifest == nil {
		err := NewError(fmt.Errorf("missing manifest in module "+fpath), Path(fpath))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, parse.SourcePositionRange{
			SourceName:  fpath,
			StartLine:   1,
			StartColumn: 1,
			Span:        parse.NodeSpan{Start: 0, End: 1},
		})
	}

	// parse included files

	inclusionStmts := parse.FindNodes(code.Node, &parse.InclusionImportStatement{}, nil)

	for _, stmt := range inclusionStmts {
		relativePath := stmt.PathSource().Value

		chunk, err := ParseLocalSecondaryChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath: filepath.Join(src.ResourceDir, relativePath),
			Module:        mod,
			Context:       ctx,
		})

		if err != nil && chunk == nil {
			return nil, err
		}

		mod.ParsingErrors = append(mod.ParsingErrors, chunk.ParsingErrors...)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, chunk.ParsingErrorPositions...)
		mod.InclusionStatementMap[stmt] = chunk
		mod.IncludedChunkForest = append(mod.IncludedChunkForest, chunk)
	}

	return mod, combineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
}

// An IncludedChunk represents an Inox chunk that is included in another chunk,
// it does not hold any state and should NOT be modified.
type IncludedChunk struct {
	*parse.ParsedChunk
	IncludedChunkForest   []*IncludedChunk
	ParsingErrors         []Error
	ParsingErrorPositions []parse.SourcePositionRange
}

type LocalSecondaryChunkParsingConfig struct {
	ChunkFilepath string
	Module        *Module
	Context       *Context
}

func ParseLocalSecondaryChunk(config LocalSecondaryChunkParsingConfig) (*IncludedChunk, error) {
	fpath := config.ChunkFilepath
	mod := config.Module

	if strings.Contains(fpath, "..") {
		return nil, errors.New(INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X)
	}

	absPath, err := filepath.Abs(fpath)
	if err != nil {
		return nil, err
	}

	if _, ok := mod.IncludedChunkMap[absPath]; ok {
		return nil, fmt.Errorf("%s already included", absPath)
	}

	//read the file

	{
		readPerm := FilesystemPermission{Kind_: ReadPerm, Entity: Path(absPath)}
		if err := config.Context.CheckHasPermission(readPerm); err != nil {
			return nil, fmt.Errorf("failed to parse included chunk %s: %w", config.ChunkFilepath, err)
		}
	}

	if info, err := os.Stat(fpath); err == os.ErrNotExist || (err == nil && info.IsDir()) {
		return nil, fmt.Errorf("%s does not exist or is a folder", fpath)
	}

	b, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fpath, err)
	}

	//parse

	src := parse.SourceFile{
		NameString:    fpath,
		Resource:      fpath,
		CodeString:    string(b),
		ResourceDir:   filepath.Dir(absPath),
		IsResourceURL: false,
	}

	chunk, err := parse.ParseChunkSource(src)
	if err != nil && chunk == nil {
		return nil, fmt.Errorf("failed to parse %s: %w", fpath, err)
	}

	includedChunk := &IncludedChunk{
		ParsedChunk: chunk,
	}

	// add parsing errors to the included chunk
	if err != nil {
		errorAggregation, ok := err.(*parse.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}

		includedChunk.ParsingErrors = make([]Error, len(errorAggregation.Errors))
		includedChunk.ParsingErrorPositions = make([]parse.SourcePositionRange, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			includedChunk.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			includedChunk.ParsingErrorPositions[i] = pos
		}
	}

	// add error if a manifest is present
	if chunk.Node.Manifest != nil {
		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors,
			NewError(fmt.Errorf("included chunk files cannot contain a manifest: %s:"+fpath), Path(fpath)),
		)
	}

	mod.IncludedChunkMap[absPath] = includedChunk

	inclusionStmts := parse.FindNodes(chunk.Node, &parse.InclusionImportStatement{}, nil)

	for _, stmt := range inclusionStmts {
		relativePath := stmt.PathSource().Value

		chunk, err := ParseLocalSecondaryChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath: filepath.Join(src.ResourceDir, relativePath),
			Module:        mod,
			Context:       config.Context,
		})

		if err != nil && chunk == nil {
			return nil, err
		}

		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors, chunk.ParsingErrors...)
		mod.InclusionStatementMap[stmt] = chunk
		includedChunk.IncludedChunkForest = append(includedChunk.IncludedChunkForest, chunk)
	}

	mod.FlattenedIncludedChunkList = append(mod.FlattenedIncludedChunkList, includedChunk)

	return includedChunk, nil
}

func createRecordFromSourcePosition(pos parse.SourcePositionRange) *Record {
	rec := NewRecordFromKeyValLists(
		SOURCE_POS_RECORD_PROPNAMES,
		[]Value{Str(pos.SourceName), Int(pos.StartLine), Int(pos.StartColumn), Int(pos.Span.Start), Int(pos.Span.End)},
	)
	return rec
}

func createRecordFromSourcePositionStack(posStack parse.SourcePositionStack) *Record {
	positionRecords := make([]Value, len(posStack))

	for i, pos := range posStack {
		positionRecords[i] = createRecordFromSourcePosition(pos)
	}

	return NewRecordFromKeyValLists([]string{"position-stack"}, []Value{NewTuple(positionRecords)})
}
