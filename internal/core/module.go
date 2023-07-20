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

	"github.com/go-git/go-billy/v5"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X = "included file path should not contain '..'"
	MOD_ARGS_VARNAME                        = "mod-args"
	MAX_PREINIT_FILE_SIZE                   = int32(100_000)
	DEFAULT_MAX_READ_FILE_SIZE              = int32(100_000_000)
)

var (
	MODULE_PROP_NAMES            = []string{"parsing_errors", "main_chunk_node"}
	SOURCE_POS_RECORD_PROPNAMES  = []string{"source", "line", "column", "start", "end"}
	ErrFileToIncludeDoesNotExist = errors.New("file to include does not exist")
	ErrFileToIncludeIsAFolder    = errors.New("file to include is a folder")
	ErrMissingManifest           = errors.New("missing manifest")
)

// A Module represents an Inox module, it does not hold any state and should NOT be modified. Module implements Value.
type Module struct {
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
	OriginalErrors             []*parse.ParsingError //len(.OriginalErrors) <= len(.ParsingErrors)

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

	for k, v := range mod.InclusionStatementMap {
		inclusionStmtMap[k] = &symbolic.IncludedChunk{
			ParsedChunk: v.ParsedChunk,
		}
	}
	return symbolic.NewModule(mod.MainChunk, inclusionStmtMap)
}

type PreinitArgs struct {
	GlobalConsts     *parse.GlobalConstantDeclarations //only used if no running state
	PreinitStatement *parse.PreinitStatement           //only used if no running state

	RunningState *TreeWalkState //optional

	//if RunningState is nil .PreinitFilesystem is used to create the temporary context.
	PreinitFilesystem afs.Filesystem

	DefaultLimitations    []Limitation
	AddDefaultPermissions bool
	HandleCustomType      CustomPermissionTypeHandler //optional
	IgnoreUnknownSections bool
	IgnoreConstDeclErrors bool
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
func (m *Module) PreInit(preinitArgs PreinitArgs) (_ *Manifest, _ *TreeWalkState, _ []*StaticCheckError, preinitErr error) {
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
		return &Manifest{}, nil, nil, nil
	}

	manifestObjLiteral, ok := m.ManifestTemplate.Object.(*parse.ObjectLiteral)
	if !ok {
		return &Manifest{}, nil, nil, nil
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

	// check object literal
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
		})
		if len(checkErrs) != 0 {
			return nil, nil, checkErrs, fmt.Errorf("%s: error while checking manifest's object literal: %w", m.Name(), combineStaticCheckErrors(checkErrs...))
		}
	}

	var state *TreeWalkState
	var envPattern *ObjectPattern
	preinitFiles := make(PreinitFiles, 0)

	//we create a temporary state to evaluate some parts of the permissions
	if preinitArgs.RunningState == nil {
		ctx := NewContext(ContextConfig{
			Permissions: []Permission{GlobalVarPermission{permkind.Read, "*"}},
			Filesystem:  preinitArgs.PreinitFilesystem,
		})
		for k, v := range DEFAULT_NAMED_PATTERNS {
			ctx.AddNamedPattern(k, v)
		}

		for k, v := range DEFAULT_PATTERN_NAMESPACES {
			ctx.AddPatternNamespace(k, v)
		}

		global := NewGlobalState(ctx, getGlobalsAccessibleFromManifest().ValueEntryMap(nil))
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
				if preinitArgs.IgnoreConstDeclErrors && decl.Left == nil || decl.Right == nil || parse.NodeIs(decl.Right, (*parse.MissingExpression)(nil)) {
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
			fls := ctx.GetFileSystem()

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
				content, err := ReadFileInFS(fls, string(file.Path), MAX_PREINIT_FILE_SIZE)
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

	manifest, err := createManifest(state.Global.Ctx, manifestObj, manifestObjectConfig{
		defaultLimitations:    preinitArgs.DefaultLimitations,
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

	return mod, combineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
}

type LocalModuleParsingConfig struct {
	ModuleFilepath                      string
	Context                             *Context //this context is used for checking permissions & getting the filesystem
	RecoverFromNonExistingIncludedFiles bool
	//DefaultLimitations          []Limitation
	//CustomPermissionTypeHandler CustomPermissionTypeHandler
}

func ParseLocalModule(config LocalModuleParsingConfig) (*Module, error) {
	fpath := config.ModuleFilepath
	ctx := config.Context
	fls := ctx.GetFileSystem()
	absPath, err := fls.Absolute(fpath)
	if err != nil {
		return nil, err
	}

	//read the script

	{
		readPerm := FilesystemPermission{Kind_: permkind.Read, Entity: Path(absPath)}
		if err := ctx.CheckHasPermission(readPerm); err != nil {
			return nil, fmt.Errorf("failed to parse local module: %w", err)
		}
	}

	file, err := ctx.fs.Open(fpath)

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
			mod.OriginalErrors = append(mod.OriginalErrors, err)
			mod.ParsingErrors[i] = NewError(err, createRecordFromSourcePosition(pos))
			mod.ParsingErrorPositions[i] = pos
		}
	}

	// add error if manifest is missing
	if code.Node.Manifest == nil {
		err := NewError(fmt.Errorf("missing manifest in module %s: the file should start with 'manifest {}'", fpath), Path(fpath))
		mod.ParsingErrors = append(mod.ParsingErrors, err)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, parse.SourcePositionRange{
			SourceName:  fpath,
			StartLine:   1,
			StartColumn: 1,
			EndLine:     1,
			EndColumn:   2,
			Span:        parse.NodeSpan{Start: 0, End: 1},
		})
	}

	// parse included files

	inclusionStmts := parse.FindNodes(code.Node, &parse.InclusionImportStatement{}, nil)

	for _, stmt := range inclusionStmts {
		relativePath := stmt.PathSource().Value
		stmtPos := mod.MainChunk.GetSourcePosition(stmt.Span)

		chunk, err := ParseLocalSecondaryChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath:                       fls.Join(src.ResourceDir, relativePath),
			Module:                              mod,
			Context:                             ctx,
			ImportPosition:                      stmtPos,
			RecoverFromNonExistingIncludedFiles: config.RecoverFromNonExistingIncludedFiles,
		})

		if err != nil && chunk == nil {
			return nil, err
		}

		mod.OriginalErrors = append(mod.OriginalErrors, chunk.OriginalErrors...)
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
		NameString:    fpath,
		Resource:      fpath,
		ResourceDir:   filepath.Dir(absPath),
		IsResourceURL: false,
	}

	var existenceError error

	file, err := ctx.fs.Open(fpath)

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

	inclusionStmts := parse.FindNodes(chunk.Node, &parse.InclusionImportStatement{}, nil)

	for _, stmt := range inclusionStmts {
		relativePath := stmt.PathSource().Value
		stmtPos := chunk.GetSourcePosition(stmt.Span)

		chunk, err := ParseLocalSecondaryChunk(LocalSecondaryChunkParsingConfig{
			ChunkFilepath:                       fls.Join(src.ResourceDir, relativePath),
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
	f, err := fls.Open(name)
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
