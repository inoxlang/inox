package core

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-git/go-billy/v5"

	"github.com/inoxlang/inox/internal/afs"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X = "included file path should not contain '..'"
	MOD_ARGS_VARNAME                        = "mod-args"
	MAX_PREINIT_FILE_SIZE                   = int32(100_000)
	DEFAULT_MAX_READ_FILE_SIZE              = int32(100_000_000)

	MOD_IMPORT_FETCH_TIMEOUT = 5 * time.Second
)

var (
	SOURCE_POS_RECORD_PROPNAMES = []string{"source", "line", "column", "start", "end"}

	MODULE_KIND_NAMES = [...]string{
		UnspecifiedModuleKind: "unspecified",
		SpecModule:            "spec",
		UserLThreadModule:     "userlthread",
		TestSuiteModule:       "testsuite",
		TestCaseModule:        "testcase",
		LifetimeJobModule:     "lifetimejob",
		ApplicationModule:     "application",
	}

	ErrFileToIncludeDoesNotExist       = errors.New("file to include does not exist")
	ErrFileToIncludeIsAFolder          = errors.New("file to include is a folder")
	ErrMissingManifest                 = errors.New("missing manifest")
	ErrParsingErrorInManifestOrPreinit = errors.New("parsing error in manifest or preinit")
	ErrInvalidModuleKind               = errors.New("invalid module kind")
	ErrNotAnIncludableFile             = errors.New("not an includable file")
)

// A Module represents an Inox module, it does not hold any state and should NOT be modified. Module implements Value.
type Module struct {
	ModuleKind

	//no set for modules with an in-memory sourceName
	sourceName ResourceName

	MainChunk    *parse.ParsedChunkSource
	TopLevelNode parse.Node //*parse.Chunk|*parse.EmbeddedModule

	//inclusion imports (in top level adnd preinit block)

	IncludedChunkForest        []*IncludedChunk
	FlattenedIncludedChunkList []*IncludedChunk
	InclusionStatementMap      map[*parse.InclusionImportStatement]*IncludedChunk
	IncludedChunkMap           map[string]*IncludedChunk

	//module imports

	DirectlyImportedModules            map[string]*Module
	DirectlyImportedModulesByStatement map[*parse.ImportStatement]*Module

	//manifest node

	ManifestTemplate *parse.Manifest

	//errors

	ParsingErrors         []Error
	ParsingErrorPositions []parse.SourcePositionRange
	OriginalErrors        []*parse.ParsingError //len(.OriginalErrors) <= len(.ParsingErrors)

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

type ModuleKind int

const (
	//file module kinds

	UnspecifiedModuleKind ModuleKind = iota
	SpecModule                       //.spec.ix file
	ApplicationModule

	//embedded module kinds

	UserLThreadModule
	TestSuiteModule
	TestCaseModule
	LifetimeJobModule
)

func ParseModuleKind(s string) (ModuleKind, error) {
	for kind, name := range MODULE_KIND_NAMES {
		if name == s {
			return ModuleKind(kind), nil
		}
	}

	return -1, ErrInvalidModuleKind
}

func (k ModuleKind) IsTestModule() bool {
	return k == TestSuiteModule || k == TestCaseModule
}

func (k ModuleKind) IsEmbedded() bool {
	return k >= UserLThreadModule && k <= LifetimeJobModule
}

func (k ModuleKind) String() string {
	return MODULE_KIND_NAMES[k]
}

// AbsoluteSource returns the absolute resource name (URL or absolute path) of the module.
// If the module is embedded or has an in-memory source then (nil, false) is returned.
func (mod *Module) AbsoluteSource() (ResourceName, bool) {
	if mod.sourceName == nil {
		return nil, false
	}
	return mod.sourceName, true
}

func (mod *Module) HasURLSource() bool {
	_, ok := mod.sourceName.(URL)
	return ok
}

func (mod *Module) Name() string {
	return mod.MainChunk.Name()
}

// ImportStatements returns the top-level import statements.
func (mod *Module) ImportStatements() (imports []*parse.ImportStatement) {
	for _, stmt := range mod.MainChunk.Node.Statements {
		if importStmt, ok := stmt.(*parse.ImportStatement); ok {
			imports = append(imports, importStmt)
		}
	}
	return
}

func (mod *Module) ToSymbolic() *symbolic.Module {
	inclusionStmtMap := make(map[*parse.InclusionImportStatement]*symbolic.IncludedChunk, len(mod.IncludedChunkMap))
	importedModuleMap := make(map[*parse.ImportStatement]*symbolic.Module)

	for k, v := range mod.InclusionStatementMap {
		inclusionStmtMap[k] = &symbolic.IncludedChunk{
			ParsedChunkSource: v.ParsedChunkSource,
		}
	}

	for k, v := range mod.DirectlyImportedModulesByStatement {
		importedModuleMap[k] = v.ToSymbolic()
	}

	return symbolic.NewModule(mod.MainChunk, inclusionStmtMap, importedModuleMap)
}

func (mod *Module) ParameterNames() (names []string) {
	if mod.ManifestTemplate == nil {
		return nil
	}
	objLit, ok := mod.ManifestTemplate.Object.(*parse.ObjectLiteral)
	if !ok {
		return nil
	}

	propValue, _ := objLit.PropValue(MANIFEST_PARAMS_SECTION_NAME)
	paramsObject, ok := propValue.(*parse.ObjectLiteral)

	if !ok {
		return nil
	}

	for _, prop := range paramsObject.Properties {
		if prop.HasImplicitKey() {
			positionalParamDesc, ok := prop.Value.(*parse.ObjectLiteral)

			if !ok {
				continue
			}

			nameValue, _ := positionalParamDesc.PropValue("name")
			switch nameValue := nameValue.(type) {
			case *parse.UnambiguousIdentifierLiteral:
				names = append(names, nameValue.Name)
			default:
				//invalid
			}
		} else {
			names = append(names, prop.Name())
		}
	}

	return
}

func (m *Module) ParsingErrorTuple() *Tuple {
	if m.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Serializable, len(m.ParsingErrors))
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

type InMemoryModuleParsingConfig struct {
	Name    string
	Context *Context //this context is used to check permissions
}

func ParseInMemoryModule(codeString String, config InMemoryModuleParsingConfig) (*Module, error) {
	src := parse.InMemorySource{
		NameString: config.Name,
		CodeString: string(codeString),
	}

	parsedChunk, err := parse.ParseChunkSource(src)
	if err != nil && parsedChunk == nil {
		return nil, fmt.Errorf("failed to parse in-memory module named '%s': %w", config.Name, err)
	}

	mod := &Module{
		MainChunk:        parsedChunk,
		TopLevelNode:     parsedChunk.Node,
		ManifestTemplate: parsedChunk.Node.Manifest,
	}

	// add parsing errors to the module
	if err != nil {
		errorAggregation, ok := err.(*parse.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}

		mod.OriginalErrors = append(mod.OriginalErrors, errorAggregation.Errors...)
		mod.ParsingErrors = make([]Error, len(errorAggregation.Errors))
		mod.ParsingErrorPositions = make([]parse.SourcePositionRange, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			mod.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			mod.ParsingErrorPositions[i] = pos
		}
	}

	// add error if manifest is missing
	if parsedChunk.Node.Manifest == nil {
		err := NewError(fmt.Errorf("missing manifest in in-memory module %s: the file should start with 'manifest {}'", config.Name), String(config.Name))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
		//TODO: add position
	}

	inclusionStmts := parse.FindNodes(parsedChunk.Node, &parse.InclusionImportStatement{}, nil)

	// add error if there are inclusion statements
	if len(inclusionStmts) != 0 {
		err := NewError(fmt.Errorf("inclusion import statements found in in-memory module "+config.Name), String(config.Name))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
		//TODO: add position
	}

	return mod, CombineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
}

func ParseLocalModule(fpath string, config ModuleParsingConfig) (*Module, error) {

	select {
	case <-config.Context.Done():
		return nil, config.Context.Err()
	default:
	}

	ctx := config.Context
	fls := ctx.GetFileSystem()

	absPath, err := fls.Absolute(fpath)
	if err != nil {
		return nil, err
	}

	if config.moduleGraph == nil {
		config.moduleGraph = memds.NewDirectedGraphUniqueString[string, struct{}](memds.ThreadSafe)
	}

	if found, err := config.moduleGraph.HasNode(memds.WithData, absPath); err != nil {
		return nil, err
	} else if !found {
		config.moduleGraph.AddNode(absPath)
	}

	if err := checkNoCycleOrLongPathInModuleGraph(config.moduleGraph); err != nil {
		return nil, err
	}

	//read the script

	{
		readPerm := FilesystemPermission{Kind_: permkind.Read, Entity: Path(absPath)}
		if err := ctx.CheckHasPermission(readPerm); err != nil {
			return nil, fmt.Errorf("failed to parse local module: %w", err)
		}
	}

	file, err := ctx.fs.OpenFile(fpath, os.O_RDONLY, 0)

	if os.IsNotExist(err) {
		return nil, err
	}

	var info fs.FileInfo
	if err == nil {
		info, err = FileStat(file, fls)
		if err != nil {
			return nil, fmt.Errorf("failed to get information for file %s: %w", fpath, err)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("%s is a folder", fpath)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", fpath, err)
	}

	b, err := io.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fpath, err)
	}

	//parse

	src := parse.SourceFile{
		NameString:             absPath,
		UserFriendlyNameString: fpath,
		Resource:               absPath,
		CodeString:             string(b),
		ResourceDir:            filepath.Dir(absPath),
		IsResourceURL:          false,
	}

	return ParseModuleFromSource(src, Path(absPath), config)
}

type ModuleParsingConfig struct {
	Context *Context //this context is used for checking permissions + getting the filesystem

	RecoverFromNonExistingIncludedFiles bool
	IgnoreBadlyConfiguredModuleImports  bool
	InsecureModImports                  bool
	//DefaultLimits          []Limit
	//CustomPermissionTypeHandler CustomPermissionTypeHandler

	moduleGraph              *memds.DirectedGraph[string, struct{}, map[string]memds.NodeId]
	SingleFileParsingTimeout time.Duration
}

func ParseModuleFromSource(src parse.ChunkSource, resource ResourceName, config ModuleParsingConfig) (*Module, error) {

	select {
	case <-config.Context.Done():
		return nil, config.Context.Err()
	default:
	}

	//check that the resource name is a URL or an absolute path
	switch r := resource.(type) {
	case Path:
		if r.IsRelative() {
			return nil, fmt.Errorf("invalid resource name: %q, path should have been made absolute by the caller", r.UnderlyingString())
		}
	case URL:
	default:
		return nil, fmt.Errorf("invalid resource name: %q", r.UnderlyingString())
	}

	if config.moduleGraph == nil {
		config.moduleGraph = memds.NewDirectedGraphUniqueString[string, struct{}](memds.ThreadSafe)
	}

	//add the module to the graph if necessary
	var nodeId memds.NodeId
	node, err := config.moduleGraph.GetNode(memds.WithData, src.Name())
	if err != nil && !errors.Is(err, memds.ErrNodeNotFound) {
		return nil, fmt.Errorf("failed to check if module %q is present in the module graph: %w", src.Name(), err)
	} else if errors.Is(err, memds.ErrNodeNotFound) {
		nodeId = config.moduleGraph.AddNode(src.Name())
	} else {
		nodeId = node.Id
	}

	code, err := parse.ParseChunkSource(src, parse.ParserOptions{
		ParentContext: config.Context,
		Timeout:       config.SingleFileParsingTimeout,
	})

	if err != nil && code == nil {
		return nil, fmt.Errorf("failed to parse %s: %w", resource.ResourceName(), err)
	}

	if err != nil {
		//log.Println(parse.GetTreeView(code.Node))
	}

	mod := &Module{
		MainChunk:    code,
		TopLevelNode: code.Node,
		sourceName:   resource,

		ManifestTemplate:      code.Node.Manifest,
		InclusionStatementMap: make(map[*parse.InclusionImportStatement]*IncludedChunk),
		IncludedChunkMap:      map[string]*IncludedChunk{},
	}

	//the following condition should be updated if URLs with a query are supported.
	if strings.HasSuffix(resource.UnderlyingString(), inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
		mod.ModuleKind = SpecModule
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
			mod.OriginalErrors = append(mod.OriginalErrors, err)
			mod.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			mod.ParsingErrorPositions[i] = pos
		}
	}

	// add error if manifest is missing
	if code.Node.Manifest == nil || code.Node.Manifest.Object == nil || !utils.Implements[*parse.ObjectLiteral](code.Node.Manifest.Object) {
		err := NewError(fmt.Errorf("missing manifest in module %s: the file should start with 'manifest {}'", src.Name()), resource)
		mod.ParsingErrors = append(mod.ParsingErrors, err)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, parse.SourcePositionRange{
			SourceName:  src.Name(),
			StartLine:   1,
			StartColumn: 1,
			EndLine:     1,
			EndColumn:   2,
			Span:        parse.NodeSpan{Start: 0, End: 1},
		})
	} else {
		//attempt to determine the module kind, we don't report errors because the static checker will.
		objLit := code.Node.Manifest.Object.(*parse.ObjectLiteral)
		node, ok := objLit.PropValue(MANIFEST_KIND_SECTION_NAME)
		if ok {
			kindName, ok := getUncheckedModuleKindNameFromNode(node)
			if ok {
				kind, err := ParseModuleKind(kindName)
				if err == nil {
					mod.ModuleKind = kind
				}
			}
		}
	}

	ctx := config.Context
	fls := ctx.GetFileSystem()

	unrecoverableError := ParseLocalIncludedFiles(ctx, IncludedFilesParsingConfig{
		Module:                              mod,
		Filesystem:                          fls,
		RecoverFromNonExistingIncludedFiles: config.RecoverFromNonExistingIncludedFiles,
		SingleFileParsingTimeout:            config.SingleFileParsingTimeout,
	})
	if unrecoverableError != nil {
		return nil, unrecoverableError
	}

	unrecoverableError = fetchParseImportedModules(mod, ctx, fls, importedModulesFetchConfig{
		recoverFromNonExistingFiles:  config.RecoverFromNonExistingIncludedFiles,
		ignoreBadlyConfiguredImports: config.IgnoreBadlyConfiguredModuleImports,
		timeout:                      MOD_IMPORT_FETCH_TIMEOUT,
		insecure:                     config.InsecureModImports,
		subModuleParsing:             config,
		parentModuleId:               nodeId,
	})

	if unrecoverableError != nil {
		return nil, unrecoverableError
	}

	return mod, CombineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
}

type IncludedFilesParsingConfig struct {
	Module                              *Module
	Filesystem                          afs.Filesystem
	RecoverFromNonExistingIncludedFiles bool
	SingleFileParsingTimeout            time.Duration
}

// ParseLocalIncludedFiles parses all the files included by $mod.
func ParseLocalIncludedFiles(ctx *Context, config IncludedFilesParsingConfig) (unrecoverableError error) {
	mod, fls, recoverFromNonExistingIncludedFiles := config.Module, config.Filesystem, config.RecoverFromNonExistingIncludedFiles

	src := mod.MainChunk.Source.(parse.SourceFile)

	inclusionStmts := parse.FindNodes(mod.MainChunk.Node, (*parse.InclusionImportStatement)(nil), nil)

	for _, stmt := range inclusionStmts {
		//ignore import if the source has an error
		if recoverFromNonExistingIncludedFiles && (stmt.Source == nil || stmt.Source.Base().Err != nil) {
			continue
		}

		path, isAbsolute := stmt.PathSource()
		chunkFilepath := path

		if !isAbsolute {
			chunkFilepath = fls.Join(src.ResourceDir, path)
		}

		stmtPos := mod.MainChunk.GetSourcePosition(stmt.Span)

		includedChunk, absoluteChunkPath, err := ParseIncludedChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath:                       chunkFilepath,
			Module:                              mod,
			Context:                             ctx,
			ImportPosition:                      stmtPos,
			RecoverFromNonExistingIncludedFiles: recoverFromNonExistingIncludedFiles,
			SingleFileParsingTimeout:            config.SingleFileParsingTimeout,
		})

		if err != nil && includedChunk == nil { //critical error
			return err
		}

		mod.OriginalErrors = append(mod.OriginalErrors, includedChunk.OriginalErrors...)
		mod.ParsingErrors = append(mod.ParsingErrors, includedChunk.ParsingErrors...)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, includedChunk.ParsingErrorPositions...)

		if !errors.Is(err, ErrNotAnIncludableFile) {
			mod.InclusionStatementMap[stmt] = includedChunk
			mod.IncludedChunkMap[absoluteChunkPath] = includedChunk
			mod.IncludedChunkForest = append(mod.IncludedChunkForest, includedChunk)
			mod.FlattenedIncludedChunkList = append(mod.FlattenedIncludedChunkList, includedChunk)
			continue
		}

	}
	return nil
}

// An IncludedChunk represents an Inox chunk that is included in another chunk,
// it does not hold any state and should NOT be modified.
type IncludedChunk struct {
	*parse.ParsedChunkSource
	IncludedChunkForest   []*IncludedChunk
	OriginalErrors        []*parse.ParsingError
	ParsingErrors         []Error
	ParsingErrorPositions []parse.SourcePositionRange
}

type LocalSecondaryChunkParsingConfig struct {
	ChunkFilepath                       string
	Module                              *Module
	Context                             *Context
	ImportPosition                      parse.SourcePositionRange
	RecoverFromNonExistingIncludedFiles bool
	SingleFileParsingTimeout            time.Duration
}

func ParseIncludedChunk(config LocalSecondaryChunkParsingConfig) (_ *IncludedChunk, absolutePath string, _ error) {
	fpath := config.ChunkFilepath
	ctx := config.Context
	fls := ctx.GetFileSystem()
	mod := config.Module

	if strings.Contains(fpath, "..") {
		return nil, "", errors.New(INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X)
	}

	absPath, err := fls.Absolute(fpath)
	if err != nil {
		return nil, "", err
	}

	if _, ok := mod.IncludedChunkMap[absPath]; ok {
		return nil, "", fmt.Errorf("%s already included", absPath)
	}

	//read the file

	{
		readPerm := FilesystemPermission{Kind_: permkind.Read, Entity: Path(absPath)}
		if err := config.Context.CheckHasPermission(readPerm); err != nil {
			return nil, "", fmt.Errorf("failed to parse included chunk %s: %w", config.ChunkFilepath, err)
		}
	}

	src := parse.SourceFile{
		NameString:             absPath,
		UserFriendlyNameString: fpath, //fpath is probably equal to absPath since config.ChunkFilepath is absolute (?).
		Resource:               absPath,
		ResourceDir:            filepath.Dir(absPath),
		IsResourceURL:          false,
	}

	var existenceError error

	file, err := ctx.fs.OpenFile(fpath, os.O_RDONLY, 0)

	var info fs.FileInfo
	if err == nil {
		info, err = FileStat(file, fls)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get information for file to include %s: %w", fpath, err)
		}
	}

	if os.IsNotExist(err) {
		if !config.RecoverFromNonExistingIncludedFiles {
			return nil, "", err
		}

		existenceError = fmt.Errorf("%w: %s", ErrFileToIncludeDoesNotExist, fpath)
	} else if err == nil && info.IsDir() {
		if !config.RecoverFromNonExistingIncludedFiles {
			return nil, "", err
		}

		existenceError = fmt.Errorf("%w: %s", ErrFileToIncludeIsAFolder, fpath)
	} else {
		if err != nil {
			return nil, "", fmt.Errorf("failed to open included file %s: %s", fpath, err)
		}

		b, err := io.ReadAll(file)

		if err != nil {
			return nil, "", fmt.Errorf("failed to read included file %s: %s", fpath, err)
		}

		src.CodeString = utils.BytesAsString(b)
	}

	//parse

	chunk, err := parse.ParseChunkSource(src, parse.ParserOptions{
		ParentContext: config.Context,
		Timeout:       config.SingleFileParsingTimeout,
	})

	if err != nil && chunk == nil { //critical error
		return nil, "", fmt.Errorf("failed to parse included file %s: %w", fpath, err)
	}

	isModule := chunk != nil && chunk.Node.Manifest != nil

	includedChunk := &IncludedChunk{
		ParsedChunkSource: chunk,
	}

	if isModule {
		// Add error and return.
		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors,
			NewError(fmt.Errorf("included files should not contain a manifest: %s", fpath), Path(fpath)),
		)
		includedChunk.ParsingErrorPositions = append(includedChunk.ParsingErrorPositions, config.ImportPosition)
		return includedChunk, "", ErrNotAnIncludableFile
	}

	// add parsing errors to the included chunk
	if existenceError != nil {
		includedChunk.ParsingErrors = []Error{NewError(existenceError, Path(fpath))}
		includedChunk.ParsingErrorPositions = []parse.SourcePositionRange{config.ImportPosition}
	} else if err != nil {
		errorAggregation, ok := err.(*parse.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}
		includedChunk.OriginalErrors = append(mod.OriginalErrors, errorAggregation.Errors...)
		includedChunk.ParsingErrors = make([]Error, len(errorAggregation.Errors))
		includedChunk.ParsingErrorPositions = make([]parse.SourcePositionRange, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			includedChunk.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			includedChunk.ParsingErrorPositions[i] = pos
		}
	}

	if existenceError == nil && chunk.Node.IncludableChunkDesc == nil {
		// Add an error if the includable-file

		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors,
			NewError(fmt.Errorf("included files should start with the %s keyword: %s", parse.INCLUDABLE_CHUNK_KEYWORD_STR, fpath), Path(fpath)),
		)
		includedChunk.ParsingErrorPositions = append(includedChunk.ParsingErrorPositions, config.ImportPosition)
	}

	inclusionStmts := parse.FindNodes(chunk.Node, (*parse.InclusionImportStatement)(nil), nil)

	for _, stmt := range inclusionStmts {
		//ignore import if the source has an error
		if config.RecoverFromNonExistingIncludedFiles && (stmt.Source == nil || stmt.Source.Base().Err != nil) {
			continue
		}

		path, isAbsolute := stmt.PathSource()
		chunkFilepath := path

		if !isAbsolute {
			chunkFilepath = fls.Join(src.ResourceDir, path)
		}

		stmtPos := chunk.GetSourcePosition(stmt.Span)

		chunk, absoluteChunkPath, err := ParseIncludedChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath:                       chunkFilepath,
			Module:                              mod,
			Context:                             config.Context,
			ImportPosition:                      stmtPos,
			RecoverFromNonExistingIncludedFiles: config.RecoverFromNonExistingIncludedFiles,
			SingleFileParsingTimeout:            config.SingleFileParsingTimeout,
		})

		if err != nil && chunk == nil {
			return nil, "", err
		}

		includedChunk.OriginalErrors = append(mod.OriginalErrors, chunk.OriginalErrors...)
		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors, chunk.ParsingErrors...)

		if !errors.Is(err, ErrNotAnIncludableFile) {
			mod.InclusionStatementMap[stmt] = chunk
			mod.IncludedChunkMap[absoluteChunkPath] = chunk
			includedChunk.IncludedChunkForest = append(includedChunk.IncludedChunkForest, chunk)
			mod.FlattenedIncludedChunkList = append(mod.FlattenedIncludedChunkList, chunk)
		}
	}

	return includedChunk, absPath, nil
}

func createRecordFromSourcePosition(pos parse.SourcePositionRange) *Record {
	rec := NewRecordFromKeyValLists(
		SOURCE_POS_RECORD_PROPNAMES,
		[]Serializable{String(pos.SourceName), Int(pos.StartLine), Int(pos.StartColumn), Int(pos.Span.Start), Int(pos.Span.End)},
	)
	return rec
}

func createRecordFromSourcePositionStack(posStack parse.SourcePositionStack) *Record {
	positionRecords := make([]Serializable, len(posStack))

	for i, pos := range posStack {
		positionRecords[i] = createRecordFromSourcePosition(pos)
	}

	return NewRecordFromKeyValLists([]string{"position-stack"}, []Serializable{NewTuple(positionRecords)})
}

// FileStat tries to directly use the given file to get file information,
// if it fails and fls is not nil then fls.Stat(f) is used.
func FileStat(f billy.File, fls billy.Basic) (os.FileInfo, error) {
	interf, ok := f.(interface{ Stat() (os.FileInfo, error) })
	if !ok {
		return fls.Stat(f.Name())
	}
	return interf.Stat()
}

var (
	ErrFileSizeExceedSpecifiedLimit = errors.New("file's size exceeds the specified limit")
)

// ReadFileInFS reads up to maxSize bytes from a file in the given filesystem.
// if maxSize is <=0 the max size is set to 100MB.
func ReadFileInFS(fls billy.Basic, name string, maxSize int32) ([]byte, error) {
	f, err := fls.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	if maxSize <= 0 {
		maxSize = DEFAULT_MAX_READ_FILE_SIZE
	}

	var size32 int32
	if info, err := FileStat(f, fls); err == nil {
		size64 := info.Size()
		if size64 > int64(maxSize) || size64 >= math.MaxInt32 {
			return nil, ErrFileSizeExceedSpecifiedLimit
		}

		size32 = int32(size64)
	}

	size32++ // one byte for final read at EOF
	// If a file claims a small size, read at least 512 bytes.
	// In particular, files in Linux's /proc claim size 0 but
	// then do not work right if read in small pieces,
	// so an initial read of 1 byte would not work correctly.

	if size32 < 512 {
		size32 = 512
	}

	data := make([]byte, 0, size32)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}

		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]

		if len(data) > int(maxSize) {
			return nil, ErrFileSizeExceedSpecifiedLimit
		}

		if err != nil {
			if err == io.EOF {
				err = nil
			}

			return data, err
		}
	}
}

func getUncheckedModuleKindNameFromNode(n parse.Node) (name string, found bool) {
	var kindName string

	switch node := n.(type) {
	case *parse.DoubleQuotedStringLiteral:
		kindName = node.Value
	case *parse.MultilineStringLiteral:
		kindName = node.Value
	case *parse.UnquotedStringLiteral:
		kindName = node.Value
	default:
		return "", false
	}

	return kindName, true
}
