package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	MOD_PREP_LOG_SRC = "mod-prep"
	LDB_MAIN_HOST    = Host("ldb://main")
)

var (
	ErrDatabaseOpenFunctionNotFound           = errors.New("function to open database not found")
	ErrNonMatchingCachedEntryModulePath       = errors.New("the module path of the preparation cache entry has not the same as the absolute version of the provided path")
	ErrNonMatchingCacheEntryPreparationParams = errors.New("the preparation cache entry has not the same preparation parameters as the ones provided")
)

type ModulePreparationArgs struct {
	//Path of the module in the .ParsingCompilationContext's filesystem.
	Fpath string

	//Timeout duration set in parse.ParserOptions.
	SingleFileParsingTimeout time.Duration

	//If not nil and contains an entry corresponding to the module, some operations are not performed
	//(module parsing, static check, symbolic check), and the cached data is used instead.
	//The cache entry is retreived using a PreparationCacheKey created from the effetive preparation parameters.
	Cache *PreparationCache

	//If not nil and valid, some operations are not performed (module parsing, static check, symbolic check),
	//and the cached data is used instead. This field should not be set if .Cache is set.
	CacheEntry *PreparationCacheEntry

	//If true .CacheEntry and entries from .Cache are assumed to be valid.
	ForceUseCache bool

	InoxChunkCache *parse.ChunkCache

	IsUnderTest bool

	//Enable data extraction mode, this mode allows some errors.
	//this mode is intended to be used by the LSP server.
	DataExtractionMode bool

	AllowMissingEnvVars     bool
	FullAccessToDatabases   bool
	ForceExpectSchemaUpdate bool
	AfterDBCreations        func(state *GlobalState, dbs map[string]*DatabaseIL) error

	EnableTesting bool
	TestFilters   TestFilters

	// If set this function is called just before the context creation,
	// the preparation is aborted if an error is returned.
	// The returned limits are used instead of the manifest limits.
	BeforeContextCreation func(*Manifest) ([]Limit, error)

	CliArgs []string
	Args    *ModuleArgs
	// if set the result of the function is used instead of .Args
	GetArguments func(*Manifest) (*ModuleArgs, error)

	ParsingCompilationContext *Context //always necessary even if .CachedModule is set
	ParentContext             *Context
	ParentContextRequired     bool
	UseParentStateAsMainState bool

	//Non-Inox context. It should not be set if ParentContext is set.
	StdlibCtx context.Context

	//Limits that are not in this list nor in the prepared module's manifest will be initialized
	//with the minimum value.
	DefaultLimits []Limit

	//should not be set if ParentContext is set
	AdditionalPermissions []Permission

	//should only be set if the module is a main module
	Project                        Project
	MemberAuthToken                string
	ListeningPort                  uint16 //optional, defaults to inoxconsts.DEV_PORT_0
	ForceLocalhostListeningAddress bool   //if true the application listening host is localhost

	//defaults to os.Stdout
	Out io.Writer

	//defaults to Out, ignored if .Logger is set
	LogOut    io.Writer
	Logger    zerolog.Logger
	LogLevels *LogLevels

	//used during the preinit
	PreinitFilesystem afs.Filesystem

	//Used to create the context.
	//If nil the parent context's filesystem is used.
	ScriptContextFileSystem afs.Filesystem

	AdditionalGlobalsTestOnly map[string]Value
}

type EffectivePreparationParameters struct {
	PreparationCacheKey //subset of effective parameters
}

// PrepareLocalModule parses & checks a module located in the filesystem and initializes its state.
func PrepareLocalModule(args ModulePreparationArgs) (state *GlobalState, mod *Module, manif *Manifest, finalErr error) {

	preparationStart := time.Now()

	//Check arguments

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
	}

	if args.ParentContext != nil && len(args.AdditionalPermissions) != 0 {
		return nil, nil, nil, errors.New(".ParentContext is set  but passed .AdditionalPermissions is not empty")
	}

	if args.UseParentStateAsMainState && args.Project != nil {
		return nil, nil, nil, errors.New(".UseParentStateAsMainState is true but .Project was set")
	}

	if args.Cache != nil && args.CacheEntry != nil {
		finalErr = errors.New(".CacheEntry is set but .Cache is also set")
		return
	}

	fls := args.ParsingCompilationContext.GetFileSystem()
	absPath, err := fls.Absolute(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of module: %w", err)
		return
	}
	args.Fpath = absPath

	//Create logger

	//the src field is not added to the logger because it is very likely present.
	preparationLogger := args.ParsingCompilationContext.NewChildLoggerForInternalSource(MOD_PREP_LOG_SRC)

	//Get project

	parentContext := args.ParentContext

	var (
		parentState *GlobalState
		project     Project = args.Project
	)

	if parentContext != nil {
		parentState = parentContext.GetClosestState()
	}

	if (project == nil || reflect.ValueOf(project).IsZero()) && args.UseParentStateAsMainState && parentState != nil {
		project = parentState.Project
	} else if project != nil && reflect.ValueOf(project).IsZero() {
		project = nil
	}

	//Start

	//Determine listening address (if any)
	var applicationListeningAddr Host

	if project != nil {
		port := inoxconsts.DEV_PORT_0
		if args.ListeningPort != 0 {
			port = strconv.Itoa(int(args.ListeningPort))
		}

		applicationListeningAddr = Host("https://localhost:" + port)
		if project.Configuration().AreExposedWebServersAllowed() && !args.ForceLocalhostListeningAddress {
			applicationListeningAddr = Host("https://0.0.0.0:" + port)
		}
	}

	//Determine effective parameters

	effectiveParams := EffectivePreparationParameters{}
	effectiveParams.AbsoluteModulePath = args.Fpath
	effectiveParams.DataExtractionMode = args.DataExtractionMode
	effectiveParams.AllowMissingEnvVars = args.AllowMissingEnvVars
	effectiveParams.TestingEnabled = args.EnableTesting
	//effectiveParams.EffectiveListeningAddress = applicationListeningAddr

	isCacheValid := false

	//Some of the following variables will be set if the cache is valid.
	var (
		cache                       = args.Cache
		cacheEntry                  = args.CacheEntry
		cachedModule                *Module
		cachedStaticCheckData       *StaticCheckData
		cachedSymbolicData          *symbolic.Data
		cachedFinalSymbolicCheckErr error
	)

	if cache != nil {
		cacheEntry, _ = cache.Get(effectiveParams.PreparationCacheKey)
	}

	if cacheEntry != nil {
		//Check that the cache entry has the same preparation parameters as the current ones.

		if cacheEntry.ModuleName() != absPath {
			finalErr = fmt.Errorf("%w: (%q != %q)", ErrNonMatchingCachedEntryModulePath, cacheEntry.ModuleName(), absPath)
			return
		}

		if cacheEntry.Key() != effectiveParams.PreparationCacheKey {
			finalErr = ErrNonMatchingCacheEntryPreparationParams
			return
		}

		//Check whether the cache entry is still valid.

		isCacheValid = args.ForceUseCache || cacheEntry.CheckValidity(fls)

		if isCacheValid {
			func() {
				cacheEntry.lock.Lock()
				defer cacheEntry.lock.Unlock()
				cachedModule = cacheEntry.module
				cachedStaticCheckData = cacheEntry.staticCheckData
				cachedSymbolicData = cacheEntry.symbolicData
				cachedFinalSymbolicCheckErr = cacheEntry.finalSymbolicCheckErr
			}()
		}
	}

	// parse module or use cache

	var parsingErr error

	if isCacheValid {
		mod = cachedModule
	} else {
		start := time.Now()

		var module *Module
		module, parsingErr = ParseLocalModule(args.Fpath, ModuleParsingConfig{
			Context:                             args.ParsingCompilationContext,
			RecoverFromNonExistingIncludedFiles: args.DataExtractionMode,
			SingleFileParsingTimeout:            args.SingleFileParsingTimeout,
			ChunkCache:                          args.InoxChunkCache,
		})
		preparationLogger.Debug().Dur("parsing", time.Since(start)).Send()

		mod = module

		if parsingErr != nil && mod == nil {
			//unrecoverable parsing error
			finalErr = parsingErr
			return
		}
	}

	//create context and state

	var ctx *Context

	var (
		manifest                 *Manifest
		preinitState             *TreeWalkState
		preinitErr               error
		preinitStaticCheckErrors []*StaticCheckError
	)

	if mod != nil {
		preinitStart := time.Now()
		additionalGlobals := map[string]Value{}
		maps.Copy(additionalGlobals, args.AdditionalGlobalsTestOnly)
		if applicationListeningAddr != "" {
			additionalGlobals[globalnames.APP_LISTENING_ADDR] = applicationListeningAddr
		}

		manifest, preinitState, preinitStaticCheckErrors, preinitErr = mod.PreInit(PreinitArgs{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			ParentState:           parentState,
			PreinitStatement:      mod.MainChunk.Node.Preinit,
			PreinitFilesystem:     args.PreinitFilesystem,
			Filesystem:            args.ScriptContextFileSystem,
			DefaultLimits:         args.DefaultLimits,
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.DataExtractionMode,
			IgnoreConstDeclErrors: args.DataExtractionMode,

			AdditionalGlobals: additionalGlobals,
			Project:           args.Project,
		})
		preparationLogger.Debug().Dur("preinit-dur", time.Since(preinitStart)).Send()

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

				DatabasePermission{Kind_: permkind.Read, Entity: LDB_MAIN_HOST},
				DatabasePermission{Kind_: permkind.Write, Entity: LDB_MAIN_HOST},
				DatabasePermission{Kind_: permkind.Delete, Entity: LDB_MAIN_HOST},

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
		Permissions:             append(slices.Clone(manifest.RequiredPermissions), args.AdditionalPermissions...),
		DoNotCheckDatabasePerms: args.EnableTesting,

		Limits:                  limits,
		HostDefinitions:         manifest.HostDefinitions,
		ParentContext:           parentContext,
		ParentStdLibContext:     args.StdlibCtx,
		Filesystem:              args.ScriptContextFileSystem,
		InitialWorkingDirectory: manifest.InitialWorkingDirectory,
		OwnedDatabases:          manifest.OwnedDatabases(),
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
		AbsoluteModulePath:       absPath,
		ApplicationListeningAddr: applicationListeningAddr, //not an issue if empty

		EnvPattern:          manifest.EnvPattern,
		PreinitFiles:        manifest.PreinitFiles,
		AllowMissingEnvVars: args.AllowMissingEnvVars,
		Out:                 out,
		LogOut:              args.LogOut,
		Logger:              args.Logger,
		LogLevels:           args.LogLevels,
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
	state.TestingState.IsTestingEnabled = args.EnableTesting
	state.TestingState.Filters = args.TestFilters

	if args.UseParentStateAsMainState {
		if parentState == nil {
			panic(ErrUnreachable)
		}
		state.MainState = parentState
	} else {
		state.MainState = state
		state.Project = args.Project
		state.MemberAuthToken = args.MemberAuthToken
	}
	state.OutputFieldsInitialized.Store(true)

	//Save a subset of the preparation parameters.

	state.EffectivePreparationParameters = effectiveParams

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
			resourceName, ok := config.ResolutionData.(ResourceName)
			if ok {
				ctx.AddHostDefinition(host, resourceName)
			} else {
				//no data
				ctx.AddHostDefinition(host, host)
			}
		}

		openDB, ok := GetOpenDbFn(config.Resource.Scheme())
		if !ok {
			ctx.CancelGracefully()
			return nil, nil, nil, ErrDatabaseOpenFunctionNotFound
		}

		openingConfig := DbOpenConfiguration{
			Resource:       config.Resource,
			ResolutionData: config.ResolutionData,
			FullAccess:     args.FullAccessToDatabases,
			Project:        project,
			IsTestDatabase: args.IsUnderTest,
		}

		db, err := openDB(ctx, openingConfig)

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
			ExpectedSchema:               config.ExpectedSchema,
			DevMode:                      args.DataExtractionMode,

			OpeningFunction:      openDB,
			OpeningConfiguration: openingConfig,
		})

		if err != nil && (!args.DataExtractionMode || !errors.Is(err, ErrCurrentSchemaNotEqualToExpectedSchema)) {
			err = fmt.Errorf("failed to wrap '%s' database: %w", config.Name, err)
			if !args.DataExtractionMode {
				ctx.CancelGracefully()
				return nil, nil, nil, err
			}
			dbOpeningError = err
		}
		//note: in dev mode WrapDatabase returns the database alongside the error if the latter is ErrCurrentSchemaNotEqualToExpectedSchema.
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

	//call the .SetOwnerStateOnceAndLoadIfNecessary method of owned databases.
	for dbName, db := range dbs {
		if _, isOwned := ownedDatabases[dbName]; !isOwned {
			continue
		}
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

	if args.AfterDBCreations != nil {
		err := args.AfterDBCreations(state, dbs)
		if err != nil {
			finalErr = fmt.Errorf("post database creations handler: %w", err)
			return
		}
	}

	preparationLogger.Debug().Dur("db-openings-dur", time.Since(dbOpeningStart)).Send()

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
			secretNames[i] = string(secret.Name)
			secretValues[i] = secret.Value
		}

		record := NewRecordFromKeyValLists(secretNames, secretValues)
		state.Globals.Set(globalnames.PROJECT_SECRETS, record)
	}

	//pass patterns of the preinit state to the state
	if preinitState != nil {
		state.Ctx.Update(func(ctxData LockedContextData) error {
			preinitCtx := preinitState.Global.Ctx

			preinitCtx.ForEachNamedPattern(func(name string, pattern Pattern) error {
				if _, ok := DEFAULT_NAMED_PATTERNS[name]; ok {
					return nil
				}
				ctxData.NamedPatterns[name] = pattern
				return nil
			})

			preinitCtx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace) error {
				if _, ok := DEFAULT_PATTERN_NAMESPACES[name]; ok {
					return nil
				}
				ctxData.PatternNamespaces[name] = namespace
				return nil
			})

			return nil
		})
	}

	// CLI arguments | arguments of imported/invoked module
	var modArgs *ModuleArgs
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
			modArgs = NewEmptyModuleArgs()
		} else {
			modArgsError = errors.New("module arguments not provided")
		}
	}

	if modArgsError == nil {
		state.Globals.Set(MOD_ARGS_VARNAME, modArgs)
	}

	// Static check

	staticCheckStart := time.Now()

	var staticCheckData *StaticCheckData
	var staticCheckErr error

	if isCacheValid && cachedStaticCheckData != nil {
		staticCheckData = cachedStaticCheckData
		staticCheckErr = cachedStaticCheckData.CombinedErrors()
	} else {
		staticCheckData, staticCheckErr = StaticCheck(StaticCheckInput{
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
	}

	preparationLogger.Debug().Dur("static-check-dur", time.Since(staticCheckStart)).Send()

	state.StaticCheckData = staticCheckData

	if finalErr == nil && staticCheckErr != nil && staticCheckData == nil { //critical static check error.
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

	symbolicCheckStart := time.Now()

	var symbolicData *symbolic.Data
	var symbolicCheckError error

	if isCacheValid && cachedSymbolicData != nil {
		symbolicData = cachedSymbolicData
		symbolicCheckError = cachedFinalSymbolicCheckErr
	} else {
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

		symbolicCtx, err := state.Ctx.ToSymbolicValue()
		if err != nil {
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

		symbolicData, symbolicCheckError = symbolic.EvalCheck(symbolic.EvalCheckInput{
			Node:                           mod.MainChunk.Node,
			Module:                         state.Module.ToSymbolic(),
			Globals:                        globals,
			AdditionalSymbolicGlobalConsts: additionalSymbolicGlobals,

			SymbolicBaseGlobals:           state.SymbolicBaseGlobalsForImportedModule,
			SymbolicBasePatterns:          symbolicBasePatterns,
			SymbolicBasePatternNamespaces: symbolicBasePatternNamespaces,

			ProjectFilesystem: utils.If[billy.Filesystem](state.Project != nil, ctx.GetFileSystem(), nil),

			Context: symbolicCtx,
		})
	}

	preparationLogger.Debug().Dur("symb-check-dur", time.Since(symbolicCheckStart)).Send()

	state.FinalSymbolicCheckError = symbolicCheckError
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

	//At this point we know there is no critical error.

	//Update cache.
	update := PreparationCacheEntryUpdate{
		Module:                mod,
		Time:                  preparationStart,
		StaticCheckData:       staticCheckData,
		SymbolicData:          symbolicData,
		FinalSymbolicCheckErr: symbolicCheckError,
	}

	if cacheEntry != nil {
		cacheEntry.Refresh(update)
	} else if cache != nil {
		cache.Put(effectiveParams.PreparationCacheKey, update)
	}

	preparationLogger.Debug().Dur("total-dur", time.Since(preparationStart)).Send()

	return state, mod, manifest, finalErr
}

type IncludableFilePreparationArgs struct {
	Fpath string //path of the file in the .ParsingCompilationContext's filesystem.

	ParsingContext *Context
	StdlibCtx      context.Context //used as core.DefaultContextConfig.ParentStdLibContext
	InoxChunkCache *parse.ChunkCache

	Out    io.Writer //defaults to os.Stdout
	LogOut io.Writer //defaults to Out

	//used to create the context
	IncludedChunkContextFileSystem afs.Filesystem

	//Timeout duration set in parse.ParserOptions.
	SingleFileParsingTimeout time.Duration
}

// PrepareExtractionModeIncludableFile parses & checks an includable file located in the filesystem and initializes the state
// of a fake module that includes it.
func PrepareExtractionModeIncludableFile(args IncludableFilePreparationArgs) (state *GlobalState, _ *Module, _ *IncludedChunk, finalErr error) {

	absPath, err := args.ParsingContext.GetFileSystem().Absolute(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of includable file: %w", err)
		return
	}
	args.Fpath = absPath

	//Create a fake module that imports (includes) the includable file.

	includedChunkBaseName := filepath.Base(absPath)
	includedChunkDir := filepath.Dir(absPath)

	fakeModPath := filepath.Join(includedChunkDir, strconv.FormatInt(rand.Int63(), 16)+"-mod"+inoxconsts.INOXLANG_FILE_EXTENSION)

	modSource := parse.SourceFile{
		NameString:  fakeModPath,
		CodeString:  `import ./` + includedChunkBaseName,
		Resource:    fakeModPath,
		ResourceDir: includedChunkDir,
	}

	parsedChunk := utils.Must(parse.ParseChunkSource(modSource, parse.ParserOptions{
		ParsedFileCache: args.InoxChunkCache,
	}))

	mod := &Module{
		MainChunk:             parsedChunk,
		TopLevelNode:          parsedChunk.Node,
		InclusionStatementMap: make(map[*parse.InclusionImportStatement]*IncludedChunk),
		IncludedChunkMap:      map[string]*IncludedChunk{},
	}

	criticalParsingError := ParseLocalIncludedFiles(args.ParsingContext, IncludedFilesParsingConfig{
		Module:                              mod,
		Filesystem:                          args.IncludedChunkContextFileSystem,
		RecoverFromNonExistingIncludedFiles: true,

		SingleFileParsingTimeout: args.SingleFileParsingTimeout,
		Cache:                    args.InoxChunkCache,
	})

	if criticalParsingError != nil {
		finalErr = criticalParsingError
		return
	}

	includedChunk := mod.IncludedChunkMap[absPath]

	var parsingErr error
	if len(mod.ParsingErrors) > 0 {
		parsingErr = CombineParsingErrorValues(mod.ParsingErrors, mod.ParsingErrorPositions)
	}

	//Create a context for the the fake module

	ctx, ctxErr := NewDefaultContext(DefaultContextConfig{
		Permissions:         nil,
		Limits:              nil,
		HostDefinitions:     nil,
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

	// Create a state for the fake module

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

	// Static check

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

	// Symbolic check

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
