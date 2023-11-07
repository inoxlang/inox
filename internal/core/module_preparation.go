package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	MOD_PREP_LOG_SRC = "mod-prep"
)

var (
	ErrDatabaseOpenFunctionNotFound = errors.New("function to open database not found")
	ErrNonMatchingCachedModulePath  = errors.New("the cached module's path is not the same as the absolute version of the provided path")
)

type ScriptPreparationArgs struct {
	//path of the script in the .ParsingCompilationContext's filesystem.
	Fpath string

	//if not nil the module is not parsed and this value is used.
	CachedModule *Module

	// enable data extraction mode, this mode allows some errors.
	// this mode is intended to be used by the LSP server.
	DataExtractionMode bool

	AllowMissingEnvVars     bool
	FullAccessToDatabases   bool
	ForceExpectSchemaUpdate bool
	EnableTesting           bool
	TestFilters             TestFilters

	// If set this function is called just before the context creation,
	// the preparation is aborted if an error is returned.
	// The returned limits are used instead of the manifest limits.
	BeforeContextCreation func(*Manifest) ([]Limit, error)

	CliArgs []string
	Args    *Struct
	// if set the result of the function is used instead of .Args
	GetArguments func(*Manifest) (*Struct, error)

	ParsingCompilationContext *Context
	ParentContext             *Context
	ParentContextRequired     bool
	UseParentStateAsMainState bool

	//should not be set if ParentContext is set
	StdlibCtx context.Context

	//should not be set if ParentContext is set
	AdditionalPermissions []Permission

	//should only be set if the module is a main module
	Project Project

	//defaults to os.Stdout
	Out io.Writer

	//defaults to Out, ignored if .Logger is set
	LogOut io.Writer

	Logger zerolog.Logger

	//used during the preinit
	PreinitFilesystem afs.Filesystem

	//used to create the context
	ScriptContextFileSystem afs.Filesystem

	AdditionalGlobalsTestOnly map[string]Value
}

// PrepareLocalScript parses & checks a script located in the filesystem and initializes its state.
func PrepareLocalScript(args ScriptPreparationArgs) (state *GlobalState, mod *Module, manif *Manifest, finalErr error) {

	//check arguments

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
	}

	if args.ParentContext != nil && len(args.AdditionalPermissions) != 0 {
		return nil, nil, nil, errors.New(".ParentContext is set  but passed .AdditionalPermissions is not empty")
	}

	if args.UseParentStateAsMainState && args.Project != nil {
		return nil, nil, nil, errors.New(".UseParentStateAsMainState is true but .Project was set")
	}

	absPath, err := args.ParsingCompilationContext.GetFileSystem().Absolute(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of module: %w", err)
		return
	}
	args.Fpath = absPath

	if args.CachedModule != nil && args.CachedModule.Name() != absPath {
		finalErr = fmt.Errorf("%w: (%q != %q)", ErrNonMatchingCachedModulePath, args.CachedModule.Name(), absPath)
		return
	}

	//the src field is not added to the logger because it is very likely present.
	logger := args.ParsingCompilationContext.Logger()

	// parse module or use cache

	var parsingErr error

	if args.CachedModule != nil {
		mod = args.CachedModule
	} else {
		start := time.Now()

		var module *Module
		module, parsingErr = ParseLocalModule(args.Fpath, ModuleParsingConfig{
			Context:                             args.ParsingCompilationContext,
			RecoverFromNonExistingIncludedFiles: args.DataExtractionMode,
		})
		logger.Debug().Dur("parsing", time.Since(start)).Send()

		mod = module

		if parsingErr != nil && mod == nil {
			//unrecoverable parsing error
			finalErr = parsingErr
			return
		}
	}

	//create context and state

	var ctx *Context

	parentContext := args.ParentContext

	var (
		parentState              *GlobalState
		manifest                 *Manifest
		preinitState             *TreeWalkState
		preinitErr               error
		preinitStaticCheckErrors []*StaticCheckError
		project                  Project = args.Project
	)

	if parentContext != nil {
		parentState = parentContext.GetClosestState()
	}

	if (project == nil || reflect.ValueOf(project).IsZero()) && args.UseParentStateAsMainState && parentState != nil {
		project = parentState.Project
	}

	if mod != nil {
		preinitStart := time.Now()

		manifest, preinitState, preinitStaticCheckErrors, preinitErr = mod.PreInit(PreinitArgs{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			ParentState:           parentState,
			PreinitStatement:      mod.MainChunk.Node.Preinit,
			PreinitFilesystem:     args.PreinitFilesystem,
			DefaultLimits:         GetDefaultScriptLimits(),
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.DataExtractionMode,
			IgnoreConstDeclErrors: args.DataExtractionMode,

			AdditionalGlobalsTestOnly: args.AdditionalGlobalsTestOnly,
			Project:                   args.Project,
		})
		logger.Debug().Dur("preinit-dur", time.Since(preinitStart)).Send()

		if (!args.DataExtractionMode && preinitErr != nil) || errors.Is(preinitErr, ErrParsingErrorInManifestOrPreinit) {
			finalErr = preinitErr
			return
		}

		if manifest == nil {
			manifest = NewEmptyManifest()
		}

		//if testing is enabled and the file is a spec file we add some permissions.
		if args.EnableTesting && strings.HasSuffix(args.Fpath, inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
			manifest.RequiredPermissions = append(manifest.RequiredPermissions,
				FilesystemPermission{Kind_: permkind.Read, Entity: PathPattern("/...")},
				FilesystemPermission{Kind_: permkind.Write, Entity: PathPattern("/...")},
				FilesystemPermission{Kind_: permkind.Delete, Entity: PathPattern("/...")},

				HttpPermission{Kind_: permkind.Read, AnyEntity: true},

				LThreadPermission{Kind_: permkind.Create},
			)
		}
	} else {
		manifest = NewEmptyManifest()
	}

	var limits []Limit = manifest.Limits
	if args.BeforeContextCreation != nil {
		limitList, err := args.BeforeContextCreation(manifest)
		if err != nil {
			return nil, nil, nil, err
		}
		limits = limitList
	}

	//create the script's context
	var ctxErr error

	ctx, ctxErr = NewDefaultContext(DefaultContextConfig{
		Permissions:         append(slices.Clone(manifest.RequiredPermissions), args.AdditionalPermissions...),
		Limits:              limits,
		HostResolutions:     manifest.HostResolutions,
		ParentContext:       parentContext,
		ParentStdLibContext: args.StdlibCtx,
		Filesystem:          args.ScriptContextFileSystem,
		OwnedDatabases:      manifest.OwnedDatabases(),
	})

	if ctxErr != nil {
		finalErr = ctxErr
		return
	}

	defer func() {
		if finalErr != nil && ctx != nil && !args.DataExtractionMode {
			ctx.CancelGracefully()
		}
	}()

	out := args.Out
	if out == nil {
		out = os.Stdout
	}

	// create the script's state

	globalState, err := NewDefaultGlobalState(ctx, DefaultGlobalStateConfig{
		AbsoluteModulePath: absPath,

		EnvPattern:          manifest.EnvPattern,
		PreinitFiles:        manifest.PreinitFiles,
		AllowMissingEnvVars: args.AllowMissingEnvVars,
		Out:                 out,
		LogOut:              args.LogOut,
		Logger:              args.Logger,
	})

	if err != nil {
		finalErr = fmt.Errorf("failed to create global state: %w", err)
		return
	}

	for k, v := range args.AdditionalGlobalsTestOnly {
		globalState.Globals.Set(k, v)
	}

	state = globalState
	state.Module = mod
	state.Manifest = manifest
	state.PrenitStaticCheckErrors = preinitStaticCheckErrors
	state.MainPreinitError = preinitErr
	state.IsTestingEnabled = args.EnableTesting
	state.TestFilters = args.TestFilters

	if args.UseParentStateAsMainState {
		if parentState == nil {
			panic(ErrUnreachable)
		}
		state.MainState = parentState
	} else {
		state.MainState = state
		state.Project = args.Project
	}
	state.OutputFieldsInitialized.Store(true)

	//connect to databases
	//TODO: disconnect if connection still not used after a few minutes

	var dbOpeningError error
	dbs := map[string]*DatabaseIL{}
	ownedDatabases := map[string]struct{}{}

	dbOpeningStart := time.Now()

	for _, config := range manifest.Databases {
		if config.Provided != nil {
			dbs[config.Name] = config.Provided
			continue
		}
		if !config.Owned {
			panic(ErrUnreachable)
		}

		if host, ok := config.Resource.(Host); ok {
			ctx.AddHostResolutionData(host, config.ResolutionData)
		}

		openDB, ok := GetOpenDbFn(config.Resource.Scheme())
		if !ok {
			ctx.CancelGracefully()
			return nil, nil, nil, ErrDatabaseOpenFunctionNotFound
		}

		db, err := openDB(ctx, DbOpenConfiguration{
			Resource:       config.Resource,
			ResolutionData: config.ResolutionData,
			FullAccess:     args.FullAccessToDatabases,
			Project:        project,
		})
		if err != nil {
			err = fmt.Errorf("failed to open the '%s' database: %w", config.Name, err)
			if !args.DataExtractionMode {
				ctx.CancelGracefully()
				return nil, nil, nil, err
			}
			dbOpeningError = err
			//TODO: use cached schema
			db = NewFailedToOpenDatabase(config.Resource)
		}

		wrapped, err := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                        db,
			ExpectedSchemaUpdate:         config.ExpectedSchemaUpdate || args.ForceExpectSchemaUpdate,
			ForceLoadBeforeOwnerStateSet: false,
			Name:                         config.Name,
			DevMode:                      args.DataExtractionMode,
		})
		if err != nil {
			err = fmt.Errorf("failed to wrap '%s' database: %w", config.Name, err)
			if !args.DataExtractionMode {
				ctx.CancelGracefully()
				return nil, nil, nil, err
			}
			dbOpeningError = err
		}
		dbs[config.Name] = wrapped
		ownedDatabases[config.Name] = struct{}{}
	}

	state.FirstDatabaseOpeningError = dbOpeningError
	state.Databases = dbs

	//add namespace 'dbs'
	dbsNamespaceEntries := make(map[string]Value)
	for dbName, db := range dbs {
		dbsNamespaceEntries[dbName] = db
	}
	state.Globals.Set(globalnames.DATABASES, NewMutableEntriesNamespace(globalnames.DATABASES, dbsNamespaceEntries))

	for dbName, db := range dbs {
		if _, ok := ownedDatabases[dbName]; ok {
			if err := db.SetOwnerStateOnceAndLoadIfNecessary(ctx, state); err != nil {
				err = fmt.Errorf("failed to load data of the '%s' database: %w", dbName, err)
				if !args.DataExtractionMode {
					ctx.CancelGracefully()
					return nil, nil, nil, err
				}
				dbOpeningError = err
				if state.FirstDatabaseOpeningError == nil {
					state.FirstDatabaseOpeningError = err
				}
			}
		}
	}
	logger.Debug().Dur("db-openings-dur", time.Since(dbOpeningStart)).Send()

	//add project-secrets global
	if args.Project != nil && !reflect.ValueOf(args.Project).IsNil() {
		secrets, err := args.Project.GetSecrets(ctx)
		if err != nil {
			finalErr = fmt.Errorf("failed to create default global state: %w", err)
			return
		}

		secretNames := make([]string, len(secrets))
		secretValues := make([]Serializable, len(secrets))

		for i, secret := range secrets {
			secretNames[i] = secret.Name
			secretValues[i] = secret.Value
		}

		record := NewRecordFromKeyValLists(secretNames, secretValues)
		state.Globals.Set(globalnames.PROJECT_SECRETS, record)
	}

	//pass patterns & host aliases of the preinit state to the state
	if preinitState != nil {
		for name, patt := range preinitState.Global.Ctx.GetNamedPatterns() {
			if _, ok := DEFAULT_NAMED_PATTERNS[name]; ok {
				continue
			}
			state.Ctx.AddNamedPattern(name, patt)
		}
		for name, ns := range preinitState.Global.Ctx.GetPatternNamespaces() {
			if _, ok := DEFAULT_PATTERN_NAMESPACES[name]; ok {
				continue
			}
			state.Ctx.AddPatternNamespace(name, ns)
		}
		for name, val := range preinitState.Global.Ctx.GetHostAliases() {
			state.Ctx.AddHostAlias(name, val)
		}
	}

	// CLI arguments | arguments of imported/invoked module
	var modArgs *Struct
	var modArgsError error

	if args.GetArguments != nil {
		args.Args, err = args.GetArguments(manifest)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if args.Args != nil {
		modArgs, modArgsError = manifest.Parameters.GetArgumentsFromStruct(ctx, args.Args)
	} else if args.CliArgs != nil {
		args, err := manifest.Parameters.GetArgumentsFromCliArgs(ctx, args.CliArgs)
		if err != nil {
			modArgsError = fmt.Errorf("%w\nusage: %s", err, manifest.Usage(state.Ctx))
		} else {
			modArgs = args
		}
	} else { // no arguments provided
		if args.DataExtractionMode || manifest.Parameters.NoParameters() {
			modArgs = NewEmptyStruct()
		} else {
			modArgsError = errors.New("module arguments not provided")
		}
	}

	if modArgsError == nil {
		state.Globals.Set(MOD_ARGS_VARNAME, modArgs)
	}

	// static check

	staticCheckStart := time.Now()

	staticCheckData, staticCheckErr := StaticCheck(StaticCheckInput{
		State:   state,
		Module:  mod,
		Node:    mod.MainChunk.Node,
		Chunk:   mod.MainChunk,
		Globals: state.Globals,
		AdditionalGlobalConsts: func() []string {
			if modArgsError != nil {
				return []string{MOD_ARGS_VARNAME}
			}
			return nil
		}(),
		Patterns:          state.Ctx.GetNamedPatterns(),
		PatternNamespaces: state.Ctx.GetPatternNamespaces(),
	})
	logger.Debug().Dur("static-check-dur", time.Since(staticCheckStart)).Send()

	state.StaticCheckData = staticCheckData

	if finalErr == nil && staticCheckErr != nil && staticCheckData == nil {
		finalErr = staticCheckErr
		return
	}

	if parsingErr != nil {
		if len(mod.OriginalErrors) > 1 ||
			(len(mod.OriginalErrors) == 1 && !utils.SliceContains(symbolic.SUPPORTED_PARSING_ERRORS, mod.OriginalErrors[0].Kind)) {
			finalErr = parsingErr
			return
		}
		//we continue if there is a single error AND the error is supported by the symbolic evaluation
	}

	if preinitErr != nil {
		finalErr = preinitErr
		return
	}

	// symbolic check

	globals := map[string]symbolic.ConcreteGlobalValue{}
	state.Globals.Foreach(func(k string, v Value, isConst bool) error {
		globals[k] = symbolic.ConcreteGlobalValue{
			Value:      v,
			IsConstant: isConst,
		}
		return nil
	})

	delete(globals, MOD_ARGS_VARNAME)
	additionalSymbolicGlobals := map[string]symbolic.Value{
		MOD_ARGS_VARNAME: manifest.Parameters.GetSymbolicArguments(ctx),
	}

	symbolicCtx, symbolicCheckError := state.Ctx.ToSymbolicValue()
	if symbolicCheckError != nil {
		finalErr = parsingErr
		return
	}

	basePatterns, basePatternNamespaces := state.GetBasePatternsForImportedModule()
	symbolicBasePatterns := map[string]symbolic.Pattern{}
	symbolicBasePatternNamespaces := map[string]*symbolic.PatternNamespace{}

	encountered := map[uintptr]symbolic.Value{}
	for k, v := range basePatterns {
		symbolicBasePatterns[k] = utils.Must(v.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern)
	}
	for k, v := range basePatternNamespaces {
		symbolicBasePatternNamespaces[k] = utils.Must(v.ToSymbolicValue(ctx, encountered)).(*symbolic.PatternNamespace)
	}

	symbolicCheckStart := time.Now()
	symbolicData, symbolicCheckError := symbolic.EvalCheck(symbolic.EvalCheckInput{
		Node:                           mod.MainChunk.Node,
		Module:                         state.Module.ToSymbolic(),
		Globals:                        globals,
		AdditionalSymbolicGlobalConsts: additionalSymbolicGlobals,

		SymbolicBaseGlobals:           state.SymbolicBaseGlobalsForImportedModule,
		SymbolicBasePatterns:          symbolicBasePatterns,
		SymbolicBasePatternNamespaces: symbolicBasePatternNamespaces,

		Context: symbolicCtx,
	})
	logger.Debug().Dur("symb-check-dur", time.Since(symbolicCheckStart)).Send()

	isCriticalSymbolicCheckError := symbolicCheckError != nil && symbolicData == nil

	if symbolicData != nil {
		state.SymbolicData.AddData(symbolicData)
	}

	if parsingErr != nil { //priority to parsing error
		finalErr = parsingErr
	} else if finalErr == nil {
		switch {
		case symbolicCheckError != nil:
			if isCriticalSymbolicCheckError {
				return nil, nil, nil, symbolicCheckError
			}
			finalErr = symbolicCheckError
		case preinitErr != nil:
			finalErr = preinitErr
		case staticCheckErr != nil:
			finalErr = staticCheckErr
		case modArgsError != nil:
			finalErr = modArgsError
		case dbOpeningError != nil:
			finalErr = dbOpeningError
		}
	}

	return state, mod, manifest, finalErr
}

type IncludableChunkfilePreparationArgs struct {
	Fpath string //path of the file in the .ParsingCompilationContext's filesystem.

	ParsingContext *Context
	StdlibCtx      context.Context //used as core.DefaultContextConfig.ParentStdLibContext

	Out    io.Writer //defaults to os.Stdout
	LogOut io.Writer //defaults to Out

	//used to create the context
	IncludedChunkContextFileSystem afs.Filesystem
}

// PrepareExtractionModeIncludableChunkfile parses & checks an includable-chunk file located in the filesystem and initializes its state.
func PrepareExtractionModeIncludableChunkfile(args IncludableChunkfilePreparationArgs) (state *GlobalState, _ *Module, _ *IncludedChunk, finalErr error) {
	// parse module

	absPath, err := args.ParsingContext.GetFileSystem().Absolute(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of includable chunk: %w", err)
		return
	}
	args.Fpath = absPath

	includedChunkBaseName := filepath.Base(absPath)
	includedChunkDir := filepath.Dir(absPath)

	fakeModPath := filepath.Join(includedChunkDir, strconv.FormatInt(rand.Int63(), 16)+"-mod"+inoxconsts.INOXLANG_FILE_EXTENSION)

	modSource := parse.SourceFile{
		NameString:  fakeModPath,
		CodeString:  `import ./` + includedChunkBaseName,
		Resource:    fakeModPath,
		ResourceDir: includedChunkDir,
	}

	mod := &Module{
		MainChunk:             utils.Must(parse.ParseChunkSource(modSource)),
		InclusionStatementMap: make(map[*parse.InclusionImportStatement]*IncludedChunk),
		IncludedChunkMap:      map[string]*IncludedChunk{},
	}

	criticalParsingError := ParseLocalIncludedFiles(mod, args.ParsingContext, args.IncludedChunkContextFileSystem, true)
	if criticalParsingError != nil {
		finalErr = criticalParsingError
		return
	}

	includedChunk := mod.IncludedChunkMap[absPath]

	var parsingErr error
	if len(mod.ParsingErrors) > 0 {
		parsingErr = CombineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
	}

	//create context and state

	ctx, ctxErr := NewDefaultContext(DefaultContextConfig{
		Permissions:         nil,
		Limits:              nil,
		HostResolutions:     nil,
		Filesystem:          args.IncludedChunkContextFileSystem,
		ParentStdLibContext: args.StdlibCtx,
	})

	if ctxErr != nil {
		finalErr = ctxErr
		return
	}

	defer func() {
		if finalErr != nil {
			ctx.CancelGracefully()
		}
	}()

	out := args.Out
	if out == nil {
		out = os.Stdout
	}

	// create the included chunk's state

	globalState, err := NewDefaultGlobalState(ctx, DefaultGlobalStateConfig{
		AllowMissingEnvVars: false,
		Out:                 out,
		LogOut:              args.LogOut,
	})
	if err != nil {
		finalErr = fmt.Errorf("failed to create global state: %w", err)
		return
	}
	state = globalState
	state.Module = mod
	state.Manifest = NewEmptyManifest()
	state.MainState = state

	// static check

	staticCheckData, staticCheckErr := StaticCheck(StaticCheckInput{
		State:             state,
		Module:            mod,
		Node:              mod.MainChunk.Node,
		Chunk:             mod.MainChunk,
		Globals:           state.Globals,
		Patterns:          state.Ctx.GetNamedPatterns(),
		PatternNamespaces: state.Ctx.GetPatternNamespaces(),
	})

	state.StaticCheckData = staticCheckData

	if finalErr == nil && staticCheckErr != nil && staticCheckData == nil {
		finalErr = staticCheckErr
		return
	}

	if parsingErr != nil {
		if len(mod.OriginalErrors) > 1 ||
			(len(mod.OriginalErrors) == 1 && !utils.SliceContains(symbolic.SUPPORTED_PARSING_ERRORS, mod.OriginalErrors[0].Kind)) {
			finalErr = parsingErr
			return state, mod, includedChunk, finalErr
		}
		//we continue if there is a single error AND the error is supported by the symbolic evaluation
	}

	// symbolic check

	globals := map[string]symbolic.ConcreteGlobalValue{}
	state.Globals.Foreach(func(k string, v Value, isConst bool) error {
		globals[k] = symbolic.ConcreteGlobalValue{
			Value:      v,
			IsConstant: isConst,
		}
		return nil
	})

	symbolicCtx, symbolicCheckError := state.Ctx.ToSymbolicValue()
	if symbolicCheckError != nil {
		finalErr = parsingErr
		return
	}

	symbolicData, symbolicCheckError := symbolic.EvalCheck(symbolic.EvalCheckInput{
		Node:    mod.MainChunk.Node,
		Module:  state.Module.ToSymbolic(),
		Globals: globals,
		Context: symbolicCtx,
	})

	isCriticalSymbolicCheckError := symbolicCheckError != nil && symbolicData == nil

	if symbolicData != nil {
		state.SymbolicData.AddData(symbolicData)
	}

	if parsingErr != nil { //priority to parsing error
		finalErr = parsingErr
	} else if finalErr == nil {
		switch {
		case symbolicCheckError != nil:
			if isCriticalSymbolicCheckError {
				return nil, nil, nil, symbolicCheckError
			}
			finalErr = symbolicCheckError
		case staticCheckErr != nil:
			finalErr = staticCheckErr
		}
	}

	return state, mod, includedChunk, finalErr
}
