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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/rs/zerolog"
)

const (
	MOD_PREP_LOG_SRC = "mod-prep"
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

	DoNotRefreshCache bool

	InoxChunkCache *parse.ChunkCache

	IsUnderTest bool

	//Enable data extraction mode, this mode allows some errors.
	//this mode is intended to be used by the LSP server.
	DataExtractionMode bool

	AllowMissingEnvVars     bool
	FullAccessToDatabases   bool
	ForceExpectSchemaUpdate bool

	EnableTesting bool
	//TestFilters   TestFilters

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
	MemberAuthToken                string
	ListeningPort                  uint16 //optional, defaults to inoxconsts.DEV_PORT_0
	ForceLocalhostListeningAddress bool   //if true the application listening host is localhost

	//defaults to os.Stdout
	Out io.Writer

	//defaults to Out, ignored if .Logger is set
	LogOut    io.Writer
	Logger    zerolog.Logger
	LogLevels *slog.Levels

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

	if args.Cache != nil && args.CacheEntry != nil {
		finalErr = errors.New(".CacheEntry is set but .Cache is also set")
		return
	}

	absPath, err := filepath.Abs(args.Fpath)
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
	)

	if parentContext != nil {
		parentState = parentContext.MustGetClosestState()
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

		isCacheValid = args.ForceUseCache || cacheEntry.CheckValidity()

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
		preinitStaticCheckErrors []*staticcheck.Error
	)

	if mod != nil {
		preinitStart := time.Now()
		additionalGlobals := map[string]Value{}
		maps.Copy(additionalGlobals, args.AdditionalGlobalsTestOnly)

		manifest, preinitState, preinitStaticCheckErrors, preinitErr = mod.PreInit(PreinitArgs{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			ParentState:           parentState,
			PreinitStatement:      mod.MainChunk.Node.Preinit,
			DefaultLimits:         args.DefaultLimits,
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.DataExtractionMode,
			IgnoreConstDeclErrors: args.DataExtractionMode,

			AdditionalGlobals: additionalGlobals,
		})
		preparationLogger.Debug().Dur("preinit-dur", time.Since(preinitStart)).Send()

		if (!args.DataExtractionMode && preinitErr != nil) || errors.Is(preinitErr, inoxmod.ErrParsingErrorInManifestOrPreinit) {
			finalErr = preinitErr
			return
		}

		if manifest == nil {
			manifest = NewEmptyManifest()
		}

		//if testing is enabled and the file is a spec file we add some permissions.
		if args.EnableTesting && strings.HasSuffix(args.Fpath, inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
			manifest.RequiredPermissions = append(manifest.RequiredPermissions,
				FilesystemPermission{Kind_: permbase.Read, Entity: PathPattern("/...")},
				FilesystemPermission{Kind_: permbase.Write, Entity: PathPattern("/...")},
				FilesystemPermission{Kind_: permbase.Delete, Entity: PathPattern("/...")},

				LThreadPermission{Kind_: permbase.Create},
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
		InitialWorkingDirectory: manifest.InitialWorkingDirectory,
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
	//TODO: state.TestingState.IsTestingEnabled = args.EnableTesting
	//state.TestingState.Filters = args.TestFilters

	if args.UseParentStateAsMainState {
		if parentState == nil {
			panic(ErrUnreachable)
		}
		state.MainState = parentState
	} else {
		state.MainState = state
		state.MemberAuthToken = args.MemberAuthToken
	}
	state.OutputFieldsInitialized.Store(true)

	//Save a subset of the preparation parameters.

	state.EffectivePreparationParameters = effectiveParams

	var patternsFromPreinit map[string]struct{}
	var patternNamespacesFromPreinit map[string]struct{}

	//Pass patterns defined by the preinit statement to the state.

	if preinitState != nil {
		patternsFromPreinit = make(map[string]struct{})
		patternNamespacesFromPreinit = make(map[string]struct{})

		state.Ctx.Update(func(ctxData LockedContextData) error {
			preinitCtx := preinitState.Global.Ctx

			preinitCtx.ForEachNamedPattern(func(name string, pattern Pattern) error {
				if _, ok := DEFAULT_NAMED_PATTERNS[name]; ok {
					return nil
				}
				ctxData.NamedPatterns[name] = pattern
				patternsFromPreinit[name] = struct{}{}
				return nil
			})

			preinitCtx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace) error {
				if _, ok := DEFAULT_PATTERN_NAMESPACES[name]; ok {
					return nil
				}
				ctxData.PatternNamespaces[name] = namespace
				patternNamespacesFromPreinit[name] = struct{}{}
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
		modArgs, modArgsError = manifest.Parameters.GetArgumentsFromModArgs(ctx, args.Args)
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
		state.Globals.Set(globalnames.MOD_ARGS_VARNAME, modArgs)
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
					return []string{globalnames.MOD_ARGS_VARNAME}
				}
				return nil
			}(),
			Patterns:          state.Ctx.GetNamedPatternNames(),
			PatternNamespaces: state.Ctx.GetPatternNamespacePatternNames(),
		})
	}

	preparationLogger.Debug().Dur("static-check-dur", time.Since(staticCheckStart)).Send()

	state.StaticCheckData = staticCheckData

	if finalErr == nil && staticCheckErr != nil && staticCheckData == nil { //critical static check error.
		finalErr = staticCheckErr
		return
	}

	if parsingErr != nil {
		if len(mod.FileLevelParsingErrors) > 1 ||
			(len(mod.FileLevelParsingErrors) == 1 && !slices.Contains(symbolic.SUPPORTED_PARSING_ERRORS, mod.FileLevelParsingErrors[0].Kind)) {
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

		delete(globals, globalnames.MOD_ARGS_VARNAME)
		additionalSymbolicGlobals := map[string]symbolic.Value{
			globalnames.MOD_ARGS_VARNAME: manifest.Parameters.GetSymbolicArguments(ctx),
		}

		symbolicCtx, err := state.Ctx.ToSymbolicValue(ContextSymbolicConversionParams{
			doNotIncludePatternsFromPreinit: true,
			patternsFromPreinit:             patternsFromPreinit,
			patternNamespacesFromPreinit:    patternNamespacesFromPreinit,
		})
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
		}
	}

	//At this point we know there is no critical error.

	//Update cache.
	if !isCacheValid && !args.DoNotRefreshCache {
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

	//Timeout duration set in parse.ParserOptions.
	SingleFileParsingTimeout time.Duration
}

// PrepareExtractionModeIncludableFile parses & checks an includable file located in the filesystem and initializes the state
// of a fake module that includes it.
func PrepareExtractionModeIncludableFile(args IncludableFilePreparationArgs) (state *GlobalState, _ *Module, _ *IncludedChunk, finalErr error) {

	absPath, err := filepath.Abs(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of includable file: %w", err)
		return
	}
	args.Fpath = absPath

	//Create a fake module that imports (includes) the includable file.

	includedChunkBaseName := filepath.Base(absPath)
	includedChunkDir := filepath.Dir(absPath)

	fakeModPath := filepath.Join(includedChunkDir, strconv.FormatInt(rand.Int63(), 16)+"-mod"+inoxconsts.INOXLANG_FILE_EXTENSION)

	modSource := sourcecode.File{
		NameString:  fakeModPath,
		CodeString:  `import ./` + includedChunkBaseName,
		Resource:    fakeModPath,
		ResourceDir: includedChunkDir,
	}

	parsedChunk := utils.Must(parse.ParseChunkSource(modSource, parse.ParserOptions{
		ParsedFileCache: args.InoxChunkCache,
	}))

	mod := WrapLowerModule(&inoxmod.Module{
		MainChunk:             parsedChunk,
		TopLevelNode:          parsedChunk.Node,
		InclusionStatementMap: make(map[*ast.InclusionImportStatement]*IncludedChunk),
		IncludedChunkMap:      map[string]*IncludedChunk{},
	})

	criticalParsingError := ParseLocalIncludedFiles(args.ParsingContext, IncludedFilesParsingConfig{
		Module:                              mod.Module,
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
	if len(mod.Errors) > 0 {
		parsingErr = inoxmod.CombineErrors(mod.Errors)
	}

	//Create a context for the the fake module

	ctx, ctxErr := NewDefaultContext(DefaultContextConfig{
		Permissions:         nil,
		Limits:              nil,
		HostDefinitions:     nil,
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
		Patterns:          state.Ctx.GetNamedPatternNames(),
		PatternNamespaces: state.Ctx.GetPatternNamespacePatternNames(),
	})

	state.StaticCheckData = staticCheckData

	if finalErr == nil && staticCheckErr != nil && staticCheckData == nil {
		finalErr = staticCheckErr
		return
	}

	if parsingErr != nil {
		if len(mod.FileLevelParsingErrors) > 1 ||
			(len(mod.FileLevelParsingErrors) == 1 && !slices.Contains(symbolic.SUPPORTED_PARSING_ERRORS, mod.FileLevelParsingErrors[0].Kind)) {
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

	symbolicCtx, symbolicCheckError := state.Ctx.ToSymbolicValue(ContextSymbolicConversionParams{})
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
