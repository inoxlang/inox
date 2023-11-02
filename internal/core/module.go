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
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/in_mem_ds"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
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
	MODULE_PROP_NAMES           = []string{"parsing_errors", "main_chunk_node"}
	SOURCE_POS_RECORD_PROPNAMES = []string{"source", "line", "column", "start", "end"}

	ErrFileToIncludeDoesNotExist       = errors.New("file to include does not exist")
	ErrFileToIncludeIsAFolder          = errors.New("file to include is a folder")
	ErrMissingManifest                 = errors.New("missing manifest")
	ErrParsingErrorInManifestOrPreinit = errors.New("parsing error in manifest or preinit")
)

// A Module represents an Inox module, it does not hold any state and should NOT be modified. Module implements Value.
type Module struct {
	ModuleKind

	//no set for modules with an in-memory sourceName
	sourceName ResourceName

	MainChunk                  *parse.ParsedChunk
	IncludedChunkForest        []*IncludedChunk
	FlattenedIncludedChunkList []*IncludedChunk
	InclusionStatementMap      map[*parse.InclusionImportStatement]*IncludedChunk
	IncludedChunkMap           map[string]*IncludedChunk

	DirectlyImportedModules            map[string]*Module
	DirectlyImportedModulesByStatement map[*parse.ImportStatement]*Module

	ManifestTemplate *parse.Manifest

	ParsingErrors         []Error
	ParsingErrorPositions []parse.SourcePositionRange
	OriginalErrors        []*parse.ParsingError //len(.OriginalErrors) <= len(.ParsingErrors)

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

type ModuleKind int

const (
	UnspecifiedModuleKind ModuleKind = iota
	UserLThreadModule
	TestSuiteModule
	TestCaseModule
	LifetimeJobModule
)

func (k ModuleKind) IsTestModule() bool {
	return k == TestSuiteModule || k == TestCaseModule
}

func (k ModuleKind) IsEmbedded() bool {
	return k >= UserLThreadModule && k <= LifetimeJobModule
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
			ParsedChunk: v.ParsedChunk,
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

type PreinitArgs struct {
	GlobalConsts     *parse.GlobalConstantDeclarations //only used if no running state
	PreinitStatement *parse.PreinitStatement           //only used if no running state

	RunningState *TreeWalkState //optional
	ParentState  *GlobalState   //optional

	//if RunningState is nil .PreinitFilesystem is used to create the temporary context.
	PreinitFilesystem afs.Filesystem

	DefaultLimits         []Limit
	AddDefaultPermissions bool
	HandleCustomType      CustomPermissionTypeHandler //optional
	IgnoreUnknownSections bool
	IgnoreConstDeclErrors bool

	//used if .RunningState is nil
	AdditionalGlobalsTestOnly map[string]Value

	Project Project //optional
}

// PreInit performs the pre-initialization of the module:
// 1)  the pre-init block is statically checked (if present).
// 2)  the manifest's object literal is statically checked.
// 3)  if .RunningState is not nil go to 10)
// 4)  else (.RunningState is nil) a temporary context & state are created.
// 5)  pre-evaluate the env section of the manifest.
// 6)  pre-evaluate the preinit-files section of the manifest.
// 7)  read & parse the preinit-files using the provided .PreinitFilesystem.
// 8)  evaluate & define the global constants (const ....).
// 9)  evaluate the preinit block.
// 10) evaluate the manifest's object literal.
// 11) create the manifest.
//
// If an error occurs at any step, the function returns.
func (m *Module) PreInit(preinitArgs PreinitArgs) (_ *Manifest, usedRunningState *TreeWalkState, _ []*StaticCheckError, preinitErr error) {
	defer func() {
		if preinitErr != nil && m.ManifestTemplate != nil {
			preinitErr = LocatedEvalError{
				error:    preinitErr,
				Message:  preinitErr.Error(),
				Location: parse.SourcePositionStack{m.MainChunk.GetSourcePosition(m.ManifestTemplate.Span)},
			}
		}
	}()

	if m.ManifestTemplate == nil {
		manifest := NewEmptyManifest()
		if preinitArgs.AddDefaultPermissions {
			manifest.RequiredPermissions = append(manifest.RequiredPermissions, GetDefaultGlobalVarPermissions()...)
		}
		return manifest, nil, nil, nil
	}

	manifestObjLiteral, ok := m.ManifestTemplate.Object.(*parse.ObjectLiteral)
	if !ok {
		return &Manifest{}, nil, nil, nil
	}

	if parse.HasErrorAtAnyDepth(manifestObjLiteral) ||
		(preinitArgs.PreinitStatement != nil && parse.HasErrorAtAnyDepth(preinitArgs.PreinitStatement)) {
		return nil, nil, nil, ErrParsingErrorInManifestOrPreinit
	}

	//check preinit block
	if preinitArgs.PreinitStatement != nil {
		var checkErrs []*StaticCheckError
		checkPreinitBlock(preinitArgs.PreinitStatement, func(n parse.Node, msg string) {
			location := m.MainChunk.GetSourcePosition(n.Base().Span)
			checkErr := NewStaticCheckError(msg, parse.SourcePositionStack{location})
			checkErrs = append(checkErrs, checkErr)
		})
		if len(checkErrs) != 0 {
			return nil, nil, checkErrs, fmt.Errorf("%s: error while checking preinit block: %w", m.Name(), combineStaticCheckErrors(checkErrs...))
		}
	}

	// check manifest
	{
		var checkErrs []*StaticCheckError
		checkManifestObject(manifestStaticCheckArguments{
			objLit:                manifestObjLiteral,
			ignoreUnknownSections: preinitArgs.IgnoreUnknownSections,
			onError: func(n parse.Node, msg string) {
				location := m.MainChunk.GetSourcePosition(n.Base().Span)
				checkErr := NewStaticCheckError(msg, parse.SourcePositionStack{location})
				checkErrs = append(checkErrs, checkErr)
			},
			project: preinitArgs.Project,
		})
		if len(checkErrs) != 0 {
			return nil, nil, checkErrs, fmt.Errorf("%s: error while checking manifest's object literal: %w", m.Name(), combineStaticCheckErrors(checkErrs...))
		}
	}

	var state *TreeWalkState
	var envPattern *ObjectPattern
	preinitFiles := make(PreinitFiles, 0)

	//we create a temporary state to pre-evaluate some parts of the manifest
	if preinitArgs.RunningState == nil {
		ctx := NewContext(ContextConfig{
			Permissions:               []Permission{GlobalVarPermission{permkind.Read, "*"}},
			Filesystem:                preinitArgs.PreinitFilesystem,
			DoNotSetFilesystemContext: true,
			DoNotSpawnDoneGoroutine:   true,
		})
		defer ctx.CancelGracefully()

		for k, v := range DEFAULT_NAMED_PATTERNS {
			ctx.AddNamedPattern(k, v)
		}

		for k, v := range DEFAULT_PATTERN_NAMESPACES {
			ctx.AddPatternNamespace(k, v)
		}

		global := NewGlobalState(ctx, getGlobalsAccessibleFromManifest().ValueEntryMap(nil))
		global.OutputFieldsInitialized.Store(true)
		global.Module = m
		state = NewTreeWalkStateWithGlobal(global)

		// pre evaluate the env section of the manifest
		envSection, ok := manifestObjLiteral.PropValue(MANIFEST_ENV_SECTION_NAME)
		if ok {
			v, err := TreeWalkEval(envSection, state)
			if err != nil {
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: failed to pre-evaluate the %s section: %w", m.Name(), MANIFEST_ENV_SECTION_NAME, err)
				}
			}
			envPattern = v.(*ObjectPattern)
		}

		//evaluate & declare the global constants.
		if preinitArgs.GlobalConsts != nil {
			for _, decl := range preinitArgs.GlobalConsts.Declarations {
				//ignore declaration if incomplete
				if preinitArgs.IgnoreConstDeclErrors && decl.Left == nil || decl.Right == nil || utils.Implements[*parse.MissingExpression](decl.Right) {
					continue
				}

				constVal, err := TreeWalkEval(decl.Right, state)
				if err != nil {
					if !preinitArgs.IgnoreConstDeclErrors {
						return nil, nil, nil, fmt.Errorf(
							"%s: failed to evaluate manifest object: error while evaluating constant declarations: %w", m.Name(), err)
					}
				} else {
					state.SetGlobal(decl.Ident().Name, constVal, GlobalConst)
				}
			}
		}

		//evalute preinit block
		if preinitArgs.PreinitStatement != nil {
			_, err := TreeWalkEval(preinitArgs.PreinitStatement.Block, state)
			if err != nil {
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: failed to evaluate preinit block: %w", m.Name(), err)
				}
			}
		}

		// pre evaluate the preinit-files section of the manifest
		preinitFilesSection, ok := manifestObjLiteral.PropValue(MANIFEST_PREINIT_FILES_SECTION_NAME)
		if ok {
			v, err := TreeWalkEval(preinitFilesSection, state)
			if err != nil {
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: failed to pre-evaluate the %s section: %w", m.Name(), MANIFEST_PREINIT_FILES_SECTION_NAME, err)
				}
			}

			obj := v.(*Object)

			err = obj.ForEachEntry(func(k string, v Serializable) error {
				desc := v.(*Object)
				propNames := desc.PropertyNames(ctx)

				if !utils.SliceContains(propNames, MANIFEST_PREINIT_FILE__PATH_PROP_NAME) {
					return fmt.Errorf("missing .%s property in description of preinit file %s", MANIFEST_PREINIT_FILE__PATH_PROP_NAME, k)
				}

				if !utils.SliceContains(propNames, MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME) {
					return fmt.Errorf("missing .%s property in description of preinit file %s", MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME, k)
				}

				path, ok := desc.Prop(ctx, MANIFEST_PREINIT_FILE__PATH_PROP_NAME).(Path)
				if !ok {
					return fmt.Errorf("property .%s in description of preinit file %s is not a path", MANIFEST_PREINIT_FILE__PATH_PROP_NAME, k)
				}
				pattern, ok := desc.Prop(ctx, MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME).(Pattern)
				if !ok {
					return fmt.Errorf("property .%s in description of preinit file %s is not a pattern", MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME, k)
				}

				if !path.IsAbsolute() {
					return fmt.Errorf("property .%s in description of preinit file %s should be an absolute path", MANIFEST_PREINIT_FILE__PATH_PROP_NAME, k)
				}

				switch patt := pattern.(type) {
				case StringPattern:
				case *SecretPattern:
				case *TypePattern:
					if patt != STR_PATTERN {
						return fmt.Errorf("invalid pattern type %T for preinit file '%s'", patt, k)
					}
				default:
					return fmt.Errorf("invalid pattern type %T for preinit file '%s'", patt, k)
				}

				preinitFiles = append(preinitFiles, &PreinitFile{
					Name:    k,
					Path:    path,
					Pattern: pattern,
					RequiredPermission: FilesystemPermission{
						Kind_:  permkind.Read,
						Entity: path,
					},
				})

				return nil
			})

			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s: failed to pre-evaluate the %s section: %w", m.Name(), MANIFEST_PREINIT_FILES_SECTION_NAME, err)
			}

			//read & parse preinit files
			atLeastOneReadParseError := false
			for _, file := range preinitFiles {
				content, err := ReadFileInFS(preinitArgs.PreinitFilesystem, string(file.Path), MAX_PREINIT_FILE_SIZE)
				file.Content = content
				file.ReadParseError = err

				if err != nil {
					atLeastOneReadParseError = true
					continue
				}

				switch patt := file.Pattern.(type) {
				case StringPattern:
					file.Parsed, file.ReadParseError = patt.Parse(ctx, string(content))
				case *SecretPattern:
					file.Parsed, file.ReadParseError = patt.NewSecret(ctx, string(content))
				case *TypePattern:
					if patt != STR_PATTERN {
						panic(ErrUnreachable)
					}
					file.Parsed = Str(content)
				default:
					panic(ErrUnreachable)
				}

				if file.ReadParseError != nil {
					atLeastOneReadParseError = true
				}
			}

			if atLeastOneReadParseError {
				//not very explicative on purpose.
				return nil, nil, nil, fmt.Errorf("%s: at least one error when reading & parsing preinit files", m.Name())
			}
		}

		for k, v := range preinitArgs.AdditionalGlobalsTestOnly {
			state.SetGlobal(k, v, GlobalConst)
		}
	} else {
		if preinitArgs.GlobalConsts != nil {
			return nil, nil, nil, fmt.Errorf(".GlobalConstants argument should not have been passed")
		}

		if preinitArgs.PreinitStatement != nil {
			return nil, nil, nil, fmt.Errorf(".Preinit argument should not have been passed")
		}

		state = preinitArgs.RunningState
	}

	// evaluate object literal
	v, err := TreeWalkEval(m.ManifestTemplate.Object, state)
	if err != nil {
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: failed to evaluate manifest object: %w", m.Name(), err)
		}
	}

	manifestObj := v.(*Object)

	manifest, err := m.createManifest(state.Global.Ctx, manifestObj, manifestObjectConfig{
		parentState:           preinitArgs.ParentState,
		defaultLimits:         preinitArgs.DefaultLimits,
		handleCustomType:      preinitArgs.HandleCustomType,
		addDefaultPermissions: preinitArgs.AddDefaultPermissions,
		envPattern:            envPattern,
		preinitFileConfigs:    preinitFiles,
		//addDefaultPermissions: true,
		ignoreUnkownSections: preinitArgs.IgnoreUnknownSections,
	})

	return manifest, state, nil, err
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
	if code.Node.Manifest == nil {
		err := NewError(fmt.Errorf("missing manifest in in-memory module %s: the file should start with 'manifest {}'", config.Name), Str(config.Name))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
		//TODO: add position
	}

	inclusionStmts := parse.FindNodes(code.Node, &parse.InclusionImportStatement{}, nil)

	// add error if there are inclusion statements
	if len(inclusionStmts) != 0 {
		err := NewError(fmt.Errorf("inclusion import statements found in in-memory module "+config.Name), Str(config.Name))
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
	fls := config.Filesystem

	if fls == nil {
		fls = ctx.GetFileSystem()
	}

	absPath, err := fls.Absolute(fpath)
	if err != nil {
		return nil, err
	}

	if config.moduleGraph == nil {
		config.moduleGraph = in_mem_ds.NewDirectedGraphUniqueString[string, struct{}](in_mem_ds.ThreadSafe)
	}

	if found, err := config.moduleGraph.HasNode(in_mem_ds.WithData, absPath); err != nil {
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
	Context    *Context //this context is used for checking permissions + getting the filesystem if .Filesystem is not set
	Filesystem afs.Filesystem

	RecoverFromNonExistingIncludedFiles bool
	IgnoreBadlyConfiguredModuleImports  bool
	InsecureModImports                  bool
	//DefaultLimits          []Limit
	//CustomPermissionTypeHandler CustomPermissionTypeHandler

	moduleGraph *in_mem_ds.DirectedGraph[string, struct{}, map[string]in_mem_ds.NodeId]
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
		config.moduleGraph = in_mem_ds.NewDirectedGraphUniqueString[string, struct{}](in_mem_ds.ThreadSafe)
	}

	//add the module to the graph if necessary
	var nodeId in_mem_ds.NodeId
	node, err := config.moduleGraph.GetNode(in_mem_ds.WithData, src.Name())
	if err != nil && !errors.Is(err, in_mem_ds.ErrNodeNotFound) {
		return nil, fmt.Errorf("failed to check if module %q is present in the module graph: %w", src.Name(), err)
	} else if errors.Is(err, in_mem_ds.ErrNodeNotFound) {
		nodeId = config.moduleGraph.AddNode(src.Name())
	} else {
		nodeId = node.Id
	}

	code, err := parse.ParseChunkSource(src)
	if err != nil && code == nil {
		return nil, fmt.Errorf("failed to parse %s: %w", resource.ResourceName(), err)
	}

	if err != nil {
		//log.Println(parse.GetTreeView(code.Node))
	}

	mod := &Module{
		MainChunk:  code,
		sourceName: resource,

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
			mod.OriginalErrors = append(mod.OriginalErrors, err)
			mod.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			mod.ParsingErrorPositions[i] = pos
		}
	}

	// add error if manifest is missing
	if code.Node.Manifest == nil {
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
	}

	ctx := config.Context
	fls := ctx.GetFileSystem()

	unrecoverableError := ParseLocalIncludedFiles(mod, ctx, fls, config.RecoverFromNonExistingIncludedFiles)
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

func ParseLocalIncludedFiles(mod *Module, ctx *Context, fls afs.Filesystem, recoverFromNonExistingIncludedFiles bool) (unrecoverableError error) {
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

		chunk, err := ParseLocalSecondaryChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath:                       chunkFilepath,
			Module:                              mod,
			Context:                             ctx,
			ImportPosition:                      stmtPos,
			RecoverFromNonExistingIncludedFiles: recoverFromNonExistingIncludedFiles,
		})

		if err != nil && chunk == nil {
			return err
		}

		mod.OriginalErrors = append(mod.OriginalErrors, chunk.OriginalErrors...)
		mod.ParsingErrors = append(mod.ParsingErrors, chunk.ParsingErrors...)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, chunk.ParsingErrorPositions...)
		mod.InclusionStatementMap[stmt] = chunk
		mod.IncludedChunkForest = append(mod.IncludedChunkForest, chunk)
	}
	return nil
}

// An IncludedChunk represents an Inox chunk that is included in another chunk,
// it does not hold any state and should NOT be modified.
type IncludedChunk struct {
	*parse.ParsedChunk
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
}

func ParseLocalSecondaryChunk(config LocalSecondaryChunkParsingConfig) (*IncludedChunk, error) {
	fpath := config.ChunkFilepath
	ctx := config.Context
	fls := ctx.GetFileSystem()
	mod := config.Module

	if strings.Contains(fpath, "..") {
		return nil, errors.New(INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X)
	}

	absPath, err := fls.Absolute(fpath)
	if err != nil {
		return nil, err
	}

	if _, ok := mod.IncludedChunkMap[absPath]; ok {
		return nil, fmt.Errorf("%s already included", absPath)
	}

	//read the file

	{
		readPerm := FilesystemPermission{Kind_: permkind.Read, Entity: Path(absPath)}
		if err := config.Context.CheckHasPermission(readPerm); err != nil {
			return nil, fmt.Errorf("failed to parse included chunk %s: %w", config.ChunkFilepath, err)
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
			return nil, fmt.Errorf("failed to get information for file to include %s: %w", fpath, err)
		}
	}

	if os.IsNotExist(err) {
		if !config.RecoverFromNonExistingIncludedFiles {
			return nil, err
		}

		existenceError = fmt.Errorf("%w: %s", ErrFileToIncludeDoesNotExist, fpath)
	} else if err == nil && info.IsDir() {
		if !config.RecoverFromNonExistingIncludedFiles {
			return nil, err
		}

		existenceError = fmt.Errorf("%w: %s", ErrFileToIncludeIsAFolder, fpath)
	} else {
		if err != nil {
			return nil, fmt.Errorf("failed to open included file %s: %s", fpath, err)
		}

		b, err := io.ReadAll(file)

		if err != nil {
			return nil, fmt.Errorf("failed to read included file %s: %s", fpath, err)
		}

		src.CodeString = utils.BytesAsString(b)
	}

	//parse

	chunk, err := parse.ParseChunkSource(src)
	if err != nil && chunk == nil {
		return nil, fmt.Errorf("failed to parse included file %s: %w", fpath, err)
	}

	includedChunk := &IncludedChunk{
		ParsedChunk: chunk,
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

	// add error if a manifest is present
	if chunk.Node.Manifest != nil {
		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors,
			NewError(fmt.Errorf("included chunk files should not contain a manifest: %s", fpath), Path(fpath)),
		)
		includedChunk.ParsingErrorPositions = append(includedChunk.ParsingErrorPositions, config.ImportPosition)
	} else if existenceError == nil && chunk.Node.IncludableChunkDesc == nil {
		includedChunk.ParsingErrors = append(includedChunk.ParsingErrors,
			NewError(fmt.Errorf("included chunk files should start with the %s keyword: %s", parse.INCLUDABLE_CHUNK_KEYWORD_STR, fpath), Path(fpath)),
		)
		includedChunk.ParsingErrorPositions = append(includedChunk.ParsingErrorPositions, config.ImportPosition)
	}

	mod.IncludedChunkMap[absPath] = includedChunk

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

		chunk, err := ParseLocalSecondaryChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath:                       chunkFilepath,
			Module:                              mod,
			Context:                             config.Context,
			ImportPosition:                      stmtPos,
			RecoverFromNonExistingIncludedFiles: config.RecoverFromNonExistingIncludedFiles,
		})

		if err != nil && chunk == nil {
			return nil, err
		}

		includedChunk.OriginalErrors = append(mod.OriginalErrors, chunk.OriginalErrors...)
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
		[]Serializable{Str(pos.SourceName), Int(pos.StartLine), Int(pos.StartColumn), Int(pos.Span.Start), Int(pos.Span.End)},
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
