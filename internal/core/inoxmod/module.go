package inoxmod

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

const (
	INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X = "included file path should not contain '..'"
	MAX_PREINIT_FILE_SIZE                   = int32(100_000)
	DEFAULT_MAX_READ_FILE_SIZE              = int32(100_000_000)

	DEFAULT_MAX_MOD_GRAPH_PATH_LEN = 5
	DEFAULT_FETCH_TIMEOUT          = 10 * time.Second

	MOD_IMPORT_FETCH_TIMEOUT = 5 * time.Second
)

var (
	MODULE_KIND_NAMES = [...]string{
		UnspecifiedModuleKind: inoxconsts.UNSPECIFIED_MODULE_KIND_NAME,
		SpecModule:            inoxconsts.SPEC_MODULE_KIND_NAME,
		UserLThreadModule:     inoxconsts.LTHREAD_MODULE_KIND_NAME,
		TestSuiteModule:       inoxconsts.TESTSUITE_MODULE_KIND_NAME,
		TestCaseModule:        inoxconsts.TESTCASE_MODULE_KIND_NAME,
		ApplicationModule:     inoxconsts.APP_MODULE_KIND_NAME,
	}

	ErrFileToIncludeDoesNotExist       = errors.New("file to include does not exist")
	ErrFileToIncludeIsAFolder          = errors.New("file to include is a folder")
	ErrMissingManifest                 = errors.New("missing manifest")
	ErrParsingErrorInManifestOrPreinit = errors.New("parsing error in manifest or preinit")
	ErrInvalidModuleKind               = errors.New("invalid module kind")
	ErrNotAnIncludableFile             = errors.New("not an includable file")
	ErrFileAlreadyIncluded             = errors.New("file already included")
	ErrUnreachable                     = errors.New("unreachable")

	ErrImportCycleDetected          = errors.New("import cycle detected")
	ErrMaxModuleImportDepthExceeded = fmt.Errorf(
		"the module import depth has exceeded the maximum (%d)", DEFAULT_MAX_MOD_GRAPH_PATH_LEN)
	ErrInvalidModuleSourceURL = errors.New("invalid module source URL")
)

// A Module represents an Inox module, it does not hold any state and should NOT be modified. Module implements Value.
type Module struct {
	Kind

	//no set for modules with an in-memory sourceName
	sourceName ResourceName

	MainChunk    *parse.ParsedChunkSource
	TopLevelNode ast.Node //*ast.Chunk|*ast.EmbeddedModule

	//inclusion imports (in top level adnd preinit block)

	IncludedChunkForest        []*IncludedChunk
	FlattenedIncludedChunkList []*IncludedChunk
	InclusionStatementMap      map[*ast.InclusionImportStatement]*IncludedChunk //may include several inclusions of the same file
	IncludedChunkMap           map[string]*IncludedChunk

	//module imports

	DirectlyImportedModules            map[string]*Module
	DirectlyImportedModulesByStatement map[*ast.ImportStatement]*Module

	//manifest node

	ManifestTemplate *ast.Manifest

	//errors

	Errors                 []Error
	FileLevelParsingErrors []*sourcecode.ParsingError //len(.FileLevelParsingErrors) <= len(.Errors)

}

type Kind int

const (
	//file module kinds

	UnspecifiedModuleKind Kind = iota
	SpecModule                 //.spec.ix file
	ApplicationModule

	//embedded module kinds

	UserLThreadModule
	TestSuiteModule
	TestCaseModule
)

func ParseModuleKind(s string) (Kind, error) {
	for kind, name := range MODULE_KIND_NAMES {
		if name == s {
			return Kind(kind), nil
		}
	}

	return -1, ErrInvalidModuleKind
}

func (k Kind) IsTestModule() bool {
	return k == TestSuiteModule || k == TestCaseModule
}

func (k Kind) IsEmbedded() bool {
	return k >= UserLThreadModule
}

func (k Kind) String() string {
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

func (mod *Module) SourceName() ResourceName {
	return mod.sourceName
}

func (mod *Module) HasURLSource() bool {
	return mod.sourceName.IsURL()
}

func (mod *Module) Name() string {
	return mod.MainChunk.Name()
}

// ImportStatements returns the top-level import statements.
func (mod *Module) ImportStatements() (imports []*ast.ImportStatement) {
	for _, stmt := range mod.MainChunk.Node.Statements {
		if importStmt, ok := stmt.(*ast.ImportStatement); ok {
			imports = append(imports, importStmt)
		}
	}
	return
}

func (mod *Module) ParameterNames() (names []string) {
	if mod.ManifestTemplate == nil {
		return nil
	}
	objLit, ok := mod.ManifestTemplate.Object.(*ast.ObjectLiteral)
	if !ok {
		return nil
	}

	propValue, _ := objLit.PropValue(inoxconsts.MANIFEST_PARAMS_SECTION_NAME)
	paramsObject, ok := propValue.(*ast.ObjectLiteral)

	if !ok {
		return nil
	}

	for _, prop := range paramsObject.Properties {
		if prop.HasNoKey() {
			positionalParamDesc, ok := prop.Value.(*ast.ObjectLiteral)

			if !ok {
				continue
			}

			nameValue, _ := positionalParamDesc.PropValue("name")
			switch nameValue := nameValue.(type) {
			case *ast.UnambiguousIdentifierLiteral:
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

type InMemoryModuleParsingConfig struct {
	Name    string
	Context Context //this context is used to check permissions
}

func ParseInMemoryModule(codeString string, config InMemoryModuleParsingConfig) (*Module, error) {
	src := sourcecode.InMemorySource{
		NameString: config.Name,
		CodeString: codeString,
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
		errorAggregation, ok := err.(*sourcecode.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}

		mod.FileLevelParsingErrors = append(mod.FileLevelParsingErrors, errorAggregation.Errors...)
		mod.Errors = make([]Error, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			mod.Errors[i] = Error{
				BaseError: err,
				Position:  pos,
			}
		}
	}

	// add error if manifest is missing
	if parsedChunk.Node.Manifest == nil {
		err := Error{
			BaseError: fmt.Errorf("missing manifest in in-memory module %s: the file should start with 'manifest {}'", config.Name),
			//Position:           //TODO
			AdditionalInfo: config.Name,
		}
		mod.Errors = append(mod.Errors, err)
	}

	inclusionStmts := ast.FindNodes(parsedChunk.Node, &ast.InclusionImportStatement{}, nil)

	// add error if there are inclusion statements
	if len(inclusionStmts) != 0 {
		err := Error{
			BaseError: fmt.Errorf("inclusion import statements found in in-memory module " + config.Name),
			//Position:           //TODO
			AdditionalInfo: config.Name,
		}

		mod.Errors = append(mod.Errors, err)
	}

	return mod, CombineErrors(mod.Errors)
}

func ParseLocalModule(fpath string, config ModuleParsingConfig) (*Module, error) {

	select {
	case <-config.Context.Done():
		return nil, config.Context.Err()
	default:
	}

	ctx := config.Context

	absPath, err := filepath.Abs(fpath)
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
		readPerm := CreateReadFilePermission(absPath)
		if err := ctx.CheckHasPermission(readPerm); err != nil {
			return nil, fmt.Errorf("failed to parse local module: %w", err)
		}
	}

	file, err := os.OpenFile(fpath, os.O_RDONLY, 0)

	if os.IsNotExist(err) {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get information for file %s: %w", fpath, err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("%s is a folder", fpath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", fpath, err)
	}

	b, err := io.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fpath, err)
	}

	//parse

	src := sourcecode.File{
		NameString:             absPath,
		UserFriendlyNameString: fpath,
		Resource:               absPath,
		CodeString:             string(b),
		ResourceDir:            filepath.Dir(absPath),
		IsResourceURL:          false,
	}

	return ParseModuleFromSource(src, CreatePath(absPath), config)
}

type ModuleParsingConfig struct {
	Context                  Context //this context is used for checking permissions + getting the filesystem
	SingleFileParsingTimeout time.Duration
	ChunkCache               *parse.ChunkCache

	RecoverFromNonExistingIncludedFiles bool
	IgnoreBadlyConfiguredModuleImports  bool
	InsecureModImports                  bool
	//DefaultLimits          []Limit
	//CustomPermissionTypeHandler CustomPermissionTypeHandler

	moduleGraph *memds.DirectedGraph[string, struct{}, map[string]memds.NodeId]
}

func ParseModuleFromSource(src sourcecode.ChunkSource, resource ResourceName, config ModuleParsingConfig) (*Module, error) {

	select {
	case <-config.Context.Done():
		return nil, config.Context.Err()
	default:
	}

	//check that the resource name is a URL or an absolute path
	switch {
	case resource.IsPath():
		path := resource.ResourceName()
		if isPathRelative(path) {
			return nil, fmt.Errorf("invalid resource name: %q, path should have been made absolute by the caller", path)
		}
	case resource.IsURL():
	default:
		return nil, fmt.Errorf("invalid resource name: %q", resource.ResourceName())
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
		ParentContext:   config.Context,
		Timeout:         config.SingleFileParsingTimeout,
		ParsedFileCache: config.ChunkCache,
	})

	if err != nil && code == nil {
		return nil, fmt.Errorf("failed to parse %s: %w", resource.ResourceName(), err)
	}

	if err != nil {
		//log.Println(ast.GetTreeView(code.Node))
	}

	mod := &Module{
		MainChunk:    code,
		TopLevelNode: code.Node,
		sourceName:   resource,

		ManifestTemplate:      code.Node.Manifest,
		InclusionStatementMap: make(map[*ast.InclusionImportStatement]*IncludedChunk),
		IncludedChunkMap:      map[string]*IncludedChunk{},
	}

	//the following condition should be updated if URLs with a query are supported.
	if strings.HasSuffix(resource.ResourceName(), inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
		mod.Kind = SpecModule
	}

	// add parsing errors to the module
	if err != nil {
		errorAggregation, ok := err.(*sourcecode.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}

		mod.Errors = make([]Error, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			mod.FileLevelParsingErrors = append(mod.FileLevelParsingErrors, err)
			mod.Errors[i] = Error{
				BaseError: err,
				Position:  pos,
			}
		}
	}

	// add error if manifest is missing
	if code.Node.Manifest == nil || code.Node.Manifest.Object == nil || !utils.Implements[*ast.ObjectLiteral](code.Node.Manifest.Object) {
		err := Error{
			BaseError:      fmt.Errorf("missing manifest in module %s: the file should start with 'manifest {}'", src.Name()),
			AdditionalInfo: resource.ResourceName(),
			Position: sourcecode.PositionRange{
				SourceName:  src.Name(),
				StartLine:   1,
				StartColumn: 1,
				EndLine:     1,
				EndColumn:   2,
				Span:        sourcecode.NodeSpan{Start: 0, End: 1},
			},
		}
		mod.Errors = append(mod.Errors, err)
	} else {
		//attempt to determine the module kind, we don't report errors because the static checker will.
		objLit := code.Node.Manifest.Object.(*ast.ObjectLiteral)
		node, hasKindProp := objLit.PropValue(inoxconsts.MANIFEST_KIND_SECTION_NAME)
		if hasKindProp {
			kindName, ok := GetUncheckedModuleKindNameFromNode(node)
			if ok {
				kind, err := ParseModuleKind(kindName)
				//Update the module kind if the specified kind is valid and the kind has not been inferred from the filename.
				if err == nil && mod.Kind == UnspecifiedModuleKind {
					mod.Kind = kind
				}
			}
		}
	}

	ctx := config.Context

	unrecoverableError := ParseLocalIncludedFiles(ctx, IncludedFilesParsingConfig{
		Module:                              mod,
		RecoverFromNonExistingIncludedFiles: config.RecoverFromNonExistingIncludedFiles,
		SingleFileParsingTimeout:            config.SingleFileParsingTimeout,
		Cache:                               config.ChunkCache,
	})
	if unrecoverableError != nil {
		return nil, unrecoverableError
	}

	unrecoverableError = fetchParseImportedModules(mod, ctx, importedModulesFetchConfig{
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

	return mod, CombineErrors(mod.Errors)
}

type IncludedFilesParsingConfig struct {
	SingleFileParsingTimeout time.Duration
	Cache                    *parse.ChunkCache

	Module                              *Module
	RecoverFromNonExistingIncludedFiles bool
}

// ParseLocalIncludedFiles parses all the files included by $mod.
func ParseLocalIncludedFiles(ctx Context, config IncludedFilesParsingConfig) (unrecoverableError error) {
	mod, recoverFromNonExistingIncludedFiles := config.Module, config.RecoverFromNonExistingIncludedFiles

	src := mod.MainChunk.Source.(sourcecode.File)

	inclusionStmts := ast.FindNodes(mod.MainChunk.Node, (*ast.InclusionImportStatement)(nil), nil)

	for _, stmt := range inclusionStmts {
		//ignore import if the source has an error
		if recoverFromNonExistingIncludedFiles && (stmt.Source == nil || stmt.Source.Base().Err != nil) {
			continue
		}

		path, isAbsolute := stmt.PathSource()
		chunkFilepath := path

		if !isAbsolute {
			chunkFilepath = filepath.Join(src.ResourceDir, path)
		}

		stmtPos := mod.MainChunk.GetSourcePosition(stmt.Span)

		includedChunk, absoluteChunkPath, err := ParseIncludedChunk(LocalSecondaryChunkParsingConfig{
			Context:                  ctx,
			ChunkFilepath:            chunkFilepath,
			SingleFileParsingTimeout: config.SingleFileParsingTimeout,
			ChunkCache:               config.Cache,

			Module:                              mod,
			ImportPosition:                      stmtPos,
			TopLevelImportPosition:              stmtPos,
			RecoverFromNonExistingIncludedFiles: recoverFromNonExistingIncludedFiles,
		})

		if err != nil && includedChunk == nil { //critical error
			return err
		}

		if errors.Is(err, ErrFileAlreadyIncluded) {
			//mod.InclusionStatementMap[stmt] = includedChunk

			//Add the error at the import in the module.

			mod.Errors = append(mod.Errors, Error{
				BaseError: err,
				Position:  stmtPos,
			})
			continue
		}

		mod.FileLevelParsingErrors = append(mod.FileLevelParsingErrors, includedChunk.OriginalErrors...)

		mod.Errors = append(mod.Errors, includedChunk.Errors...)

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

func GetUncheckedModuleKindNameFromNode(n ast.Node) (name string, found bool) {
	var kindName string

	switch node := n.(type) {
	case *ast.DoubleQuotedStringLiteral:
		kindName = node.Value
	case *ast.MultilineStringLiteral:
		kindName = node.Value
	case *ast.UnquotedStringLiteral:
		kindName = node.Value
	default:
		return "", false
	}

	return kindName, true
}

func checkNoCycleOrLongPathInModuleGraph(moduleGraph *memds.DirectedGraph[string, struct{}, map[string]memds.NodeId]) error {
	longestPath, longestPathLen := moduleGraph.LongestPath()
	if longestPathLen == -1 {
		return ErrImportCycleDetected
	}
	if longestPathLen > DEFAULT_MAX_MOD_GRAPH_PATH_LEN {
		moduleNames := utils.MapSlice(longestPath, func(nodeId memds.NodeId) string {
			return utils.MustGet(moduleGraph.Node(nodeId)).Data
		})
		return fmt.Errorf("%w: path is %s", ErrMaxModuleImportDepthExceeded, strings.Join(moduleNames, " -> "))
	}
	return nil
}
