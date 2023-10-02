package mod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/default_state"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrDatabaseOpenFunctionNotFound = errors.New("function to open database not found")
)

type ScriptPreparationArgs struct {
	Fpath string //path of the script in the .ParsingCompilationContext's filesystem.

	// enable data extraction mode, this mode allows some errors.
	// this mode is intended to be used by the LSP server.
	DataExtractionMode bool

	CliArgs []string
	Args    *core.Struct
	//if set the result of the function is used instead of .Args
	GetArguments func(*core.Manifest) (*core.Struct, error)

	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	ParentContextRequired     bool
	UseParentStateAsMainState bool
	StdlibCtx                 context.Context //should not be set if ParentContext is set

	AllowMissingEnvVars   bool
	FullAccessToDatabases bool

	Project *project.Project //should only be set if the module is a main module

	Out    io.Writer //defaults to os.Stdout
	LogOut io.Writer //defaults to Out

	//used during the preinit
	PreinitFilesystem afs.Filesystem

	//used to create the context
	ScriptContextFileSystem afs.Filesystem

	AdditionalGlobalsTestOnly map[string]core.Value
}

// PrepareLocalScript parses & checks a script located in the filesystem and initializes its state.
func PrepareLocalScript(args ScriptPreparationArgs) (state *core.GlobalState, mod *core.Module, manif *core.Manifest, finalErr error) {
	// parse module

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
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

	module, parsingErr := core.ParseLocalModule(args.Fpath, core.ModuleParsingConfig{
		Context:                             args.ParsingCompilationContext,
		RecoverFromNonExistingIncludedFiles: args.DataExtractionMode,
	})

	mod = module

	if parsingErr != nil && mod == nil {
		finalErr = parsingErr
		return
	}

	//create context and state

	var ctx *core.Context

	parentContext := args.ParentContext

	var (
		parentState              *core.GlobalState
		manifest                 *core.Manifest
		preinitState             *core.TreeWalkState
		preinitErr               error
		preinitStaticCheckErrors []*core.StaticCheckError
		project                  core.Project = args.Project
	)

	if parentContext != nil {
		parentState = parentContext.GetClosestState()
	}

	if (project == nil || reflect.ValueOf(project).IsNil()) && args.UseParentStateAsMainState && parentState != nil {
		project = parentState.Project
	}

	if mod != nil {
		manifest, preinitState, preinitStaticCheckErrors, preinitErr = mod.PreInit(core.PreinitArgs{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			ParentState:           parentState,
			PreinitStatement:      mod.MainChunk.Node.Preinit,
			PreinitFilesystem:     args.PreinitFilesystem,
			DefaultLimits:         default_state.GetDefaultScriptLimits(),
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.DataExtractionMode,
			IgnoreConstDeclErrors: args.DataExtractionMode,

			AdditionalGlobalsTestOnly: args.AdditionalGlobalsTestOnly,
			Project:                   args.Project,
		})

		if (!args.DataExtractionMode && preinitErr != nil) || errors.Is(preinitErr, core.ErrParsingErrorInManifestOrPreinit) {
			finalErr = preinitErr
			return
		}

		if manifest == nil {
			manifest = core.NewEmptyManifest()
		}

	} else {
		manifest = core.NewEmptyManifest()
	}

	//create the script's context
	var ctxErr error

	ctx, ctxErr = default_state.NewDefaultContext(default_state.DefaultContextConfig{
		Permissions:         manifest.RequiredPermissions,
		Limits:              manifest.Limits,
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
		if finalErr != nil {
			ctx.CancelGracefully()
		}
	}()

	out := args.Out
	if out == nil {
		out = os.Stdout
	}

	// create the script's state

	globalState, err := default_state.NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
		AbsoluteModulePath: absPath,

		EnvPattern:          manifest.EnvPattern,
		PreinitFiles:        manifest.PreinitFiles,
		AllowMissingEnvVars: args.AllowMissingEnvVars,
		Out:                 out,
		LogOut:              args.LogOut,
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
	if args.UseParentStateAsMainState {
		if parentState == nil {
			panic(core.ErrUnreachable)
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
	dbs := map[string]*core.DatabaseIL{}
	ownedDatabases := map[string]struct{}{}

	for _, config := range manifest.Databases {
		if config.Provided != nil {
			dbs[config.Name] = config.Provided
			continue
		}
		if !config.Owned {
			panic(core.ErrUnreachable)
		}

		if host, ok := config.Resource.(core.Host); ok {
			ctx.AddHostResolutionData(host, config.ResolutionData)
		}

		openDB, ok := core.GetOpenDbFn(config.Resource.Scheme())
		if !ok {
			ctx.CancelGracefully()
			return nil, nil, nil, ErrDatabaseOpenFunctionNotFound
		}

		db, err := openDB(ctx, core.DbOpenConfiguration{
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
			db = core.NewFailedToOpenDatabase(config.Resource)
		}

		wrapped, err := core.WrapDatabase(ctx, core.DatabaseWrappingArgs{
			Inner:                        db,
			ExpectedSchemaUpdate:         config.ExpectedSchemaUpdate,
			ForceLoadBeforeOwnerStateSet: false,
			Name:                         config.Name,
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
	dbsNamespaceEntries := make(map[string]core.Value)
	for dbName, db := range dbs {
		dbsNamespaceEntries[dbName] = db
	}
	state.Globals.Set(default_state.DATABASES_GLOBAL_NAME, core.NewNamespace("dbs", dbsNamespaceEntries))

	for dbName, db := range dbs {
		if _, ok := ownedDatabases[dbName]; ok {
			if err := db.SetOwnerStateOnceAndLoadIfNecessary(ctx, state); err != nil {
				err = fmt.Errorf("failed to load data of the '%s' database: %w", dbName, err)
				if !args.DataExtractionMode {
					ctx.CancelGracefully()
					return nil, nil, nil, err
				}
				dbOpeningError = err
			}
		}
	}

	//add project-secrets global
	if args.Project != nil {
		secrets, err := args.Project.ListSecrets2(ctx)
		if err != nil {
			finalErr = fmt.Errorf("failed to create default global state: %w", err)
			return
		}

		secretNames := make([]string, len(secrets))
		secretValues := make([]core.Serializable, len(secrets))

		for i, secret := range secrets {
			secretNames[i] = secret.Name
			secretValues[i] = secret.Value
		}

		record := core.NewRecordFromKeyValLists(secretNames, secretValues)
		state.Globals.Set(default_state.PROJECT_SECRETS_GLOBAL_NAME, record)
	}

	//pass patterns & host aliases of the preinit state to the state
	if preinitState != nil {
		for name, patt := range preinitState.Global.Ctx.GetNamedPatterns() {
			if _, ok := core.DEFAULT_NAMED_PATTERNS[name]; ok {
				continue
			}
			state.Ctx.AddNamedPattern(name, patt)
		}
		for name, ns := range preinitState.Global.Ctx.GetPatternNamespaces() {
			if _, ok := core.DEFAULT_PATTERN_NAMESPACES[name]; ok {
				continue
			}
			state.Ctx.AddPatternNamespace(name, ns)
		}
		for name, val := range preinitState.Global.Ctx.GetHostAliases() {
			state.Ctx.AddHostAlias(name, val)
		}
	}

	// CLI arguments | arguments of imported/invoked module
	var modArgs *core.Struct
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
			modArgs = core.NewEmptyStruct()
		} else {
			modArgsError = errors.New("module arguments not provided")
		}
	}

	if modArgsError == nil {
		state.Globals.Set(core.MOD_ARGS_VARNAME, modArgs)
	}

	// static check

	staticCheckData, staticCheckErr := core.StaticCheck(core.StaticCheckInput{
		State:   state,
		Module:  mod,
		Node:    mod.MainChunk.Node,
		Chunk:   mod.MainChunk,
		Globals: state.Globals,
		AdditionalGlobalConsts: func() []string {
			if modArgsError != nil {
				return []string{core.MOD_ARGS_VARNAME}
			}
			return nil
		}(),
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
			(len(mod.OriginalErrors) == 1 && !utils.SliceContains(symbolic.SUPPORTED_PARSING_ERRORS, mod.OriginalErrors[0].Kind())) {
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
	state.Globals.Foreach(func(k string, v core.Value, isConst bool) error {
		globals[k] = symbolic.ConcreteGlobalValue{
			Value:      v,
			IsConstant: isConst,
		}
		return nil
	})

	delete(globals, core.MOD_ARGS_VARNAME)
	additionalSymbolicGlobals := map[string]symbolic.SymbolicValue{
		core.MOD_ARGS_VARNAME: manifest.Parameters.GetSymbolicArguments(ctx),
	}

	symbolicCtx, symbolicCheckError := state.Ctx.ToSymbolicValue()
	if symbolicCheckError != nil {
		finalErr = parsingErr
		return
	}

	basePatterns, basePatternNamespaces := state.GetBasePatternsForImportedModule()
	symbolicBasePatterns := map[string]symbolic.Pattern{}
	symbolicBasePatternNamespaces := map[string]*symbolic.PatternNamespace{}

	encountered := map[uintptr]symbolic.SymbolicValue{}
	for k, v := range basePatterns {
		symbolicBasePatterns[k] = utils.Must(v.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern)
	}
	for k, v := range basePatternNamespaces {
		symbolicBasePatternNamespaces[k] = utils.Must(v.ToSymbolicValue(ctx, encountered)).(*symbolic.PatternNamespace)
	}

	symbolicData, symbolicCheckError := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
		Node:                           mod.MainChunk.Node,
		Module:                         state.Module.ToSymbolic(),
		Globals:                        globals,
		AdditionalSymbolicGlobalConsts: additionalSymbolicGlobals,

		SymbolicBaseGlobals:           state.SymbolicBaseGlobalsForImportedModule,
		SymbolicBasePatterns:          symbolicBasePatterns,
		SymbolicBasePatternNamespaces: symbolicBasePatternNamespaces,

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

	ParsingContext *core.Context
	StdlibCtx      context.Context //used as default_state.DefaultContextConfig.ParentStdLibContext

	Out    io.Writer //defaults to os.Stdout
	LogOut io.Writer //defaults to Out

	//used to create the context
	IncludedChunkContextFileSystem afs.Filesystem
}

// PrepareExtractionModeIncludableChunkfile parses & checks an includable-chunk file located in the filesystem and initializes its state.
func PrepareExtractionModeIncludableChunkfile(args IncludableChunkfilePreparationArgs) (state *core.GlobalState, _ *core.Module, _ *core.IncludedChunk, finalErr error) {
	// parse module

	absPath, err := args.ParsingContext.GetFileSystem().Absolute(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of includable chunk: %w", err)
		return
	}
	args.Fpath = absPath

	includedChunkBaseName := filepath.Base(absPath)
	includedChunkDir := filepath.Dir(absPath)

	fakeModPath := filepath.Join(includedChunkDir, strconv.FormatInt(rand.Int63(), 16)+"-mod.ix")

	modSource := parse.SourceFile{
		NameString:  fakeModPath,
		CodeString:  `import ./` + includedChunkBaseName,
		Resource:    fakeModPath,
		ResourceDir: includedChunkDir,
	}

	mod := &core.Module{
		MainChunk:             utils.Must(parse.ParseChunkSource(modSource)),
		InclusionStatementMap: make(map[*parse.InclusionImportStatement]*core.IncludedChunk),
		IncludedChunkMap:      map[string]*core.IncludedChunk{},
	}

	criticalParsingError := core.ParseLocalIncludedFiles(mod, args.ParsingContext, args.IncludedChunkContextFileSystem, true)
	if criticalParsingError != nil {
		finalErr = criticalParsingError
		return
	}

	includedChunk := mod.IncludedChunkMap[absPath]

	var parsingErr error
	if len(mod.ParsingErrors) > 0 {
		parsingErr = core.CombineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
	}

	//create context and state

	ctx, ctxErr := default_state.NewDefaultContext(default_state.DefaultContextConfig{
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

	globalState, err := default_state.NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
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
	state.Manifest = core.NewEmptyManifest()
	state.MainState = state

	// static check

	staticCheckData, staticCheckErr := core.StaticCheck(core.StaticCheckInput{
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
			(len(mod.OriginalErrors) == 1 && !utils.SliceContains(symbolic.SUPPORTED_PARSING_ERRORS, mod.OriginalErrors[0].Kind())) {
			finalErr = parsingErr
			return state, mod, includedChunk, finalErr
		}
		//we continue if there is a single error AND the error is supported by the symbolic evaluation
	}

	// symbolic check

	globals := map[string]symbolic.ConcreteGlobalValue{}
	state.Globals.Foreach(func(k string, v core.Value, isConst bool) error {
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

	symbolicData, symbolicCheckError := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
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
