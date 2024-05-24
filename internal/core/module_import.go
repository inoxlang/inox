package core

import (
	"fmt"

	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"

	utils "github.com/inoxlang/inox/internal/utils/common"
)

const (
	DEFAULT_IMPORT_TIMEOUT = 10 * time.Second
)

// ImportWaitModule imports a module and waits for its lthread to return its result.
// ImportWaitModule also adds the test suite results to the parent state.
func ImportWaitModule(config ImportConfig) (Value, error) {
	lthread, err := ImportModule(config)
	if err != nil {
		return nil, err
	}
	//TODO: add timeout
	result, err := lthread.WaitResult(config.ParentState.Ctx)
	if err != nil {
		return nil, fmt.Errorf("import: failed: %s", err.Error())
	}

	//TODO
	// parentState := config.ParentState

	// //add test suite results to the parent state.
	// //we only try to lock to avoid blocking if already locked.
	// if parentState.TestingState.IsTestingEnabled && lthread.state.TestingState.ResultsLock.TryLock() {
	// 	func() {
	// 		defer lthread.state.TestingState.ResultsLock.Unlock()

	// 		parentState.TestingState.ResultsLock.Lock()
	// 		defer parentState.TestingState.ResultsLock.Unlock()

	// 		parentState.TestingState.SuiteResults = append(parentState.TestingState.SuiteResults, lthread.state.TestingState.SuiteResults...)
	// 	}()
	// }

	return result, nil
}

type ImportConfig struct {
	Src                ResourceName
	ValidationString   String  //hash of the imported module
	ArgObj             *Object //arguments for the evaluation of the imported module
	GrantedPermListing *Object
	ParentState        *GlobalState  //the state of the module doing the import
	Insecure           bool          //if true certificate verification is ignored when making HTTP requests
	Timeout            time.Duration //total timeout for combined fetching + evaluation of the imported module
}

func buildImportConfig(obj *Object, importSource ResourceName, parentState *GlobalState) (ImportConfig, error) {
	src, err := inoxmod.GetSourceFromImportSource(importSource, parentState.Module.Module, parentState.Ctx)
	if err != nil {
		return ImportConfig{}, err
	}

	config := ImportConfig{
		Src:         src.(ResourceName),
		ParentState: parentState,
	}

	err = obj.ForEachEntry(func(k string, v Serializable) error {
		switch k {
		case inoxconsts.IMPORT_CONFIG__VALIDATION_PROPNAME:
			config.ValidationString = v.(String)
		case inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME:
			config.ArgObj = v.(*Object)
		case inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME:
			config.GrantedPermListing = v.(*Object)
		default:
			return fmt.Errorf("invalid import configuration, unknown section '%s'", k)
		}
		return nil
	})

	if err != nil {
		return ImportConfig{}, err
	}

	return config, nil
}

// ImportModule imports a module and returned a spawned lthread running the module.
func ImportModule(config ImportConfig) (*LThread, error) {
	parentState := config.ParentState
	timeout := config.Timeout
	if timeout == 0 {
		timeout = DEFAULT_IMPORT_TIMEOUT
	}
	deadline := time.Now().Add(timeout)

	grantedPerms, err := getPermissionsFromListing(parentState.Ctx, config.GrantedPermListing, nil, nil, true)
	if err != nil {
		return nil, err
	}
	forbiddenPerms := parentState.Ctx.forbiddenPermissions

	for _, perm := range grantedPerms {
		if err := parentState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("import: cannot allow permission: %w", err)
		}
	}

	importedModLower, ok := parentState.Module.DirectlyImportedModules[config.Src.ResourceName()]
	if !ok {
		panic(ErrUnreachable)
	}

	importedMod := WrapLowerModule(importedModLower)

	manifest, preinitState, _, err := importedMod.PreInit(PreinitArgs{
		ParentState:           parentState,
		GlobalConsts:          importedMod.MainChunk.Node.GlobalConstantDeclarations,
		PreinitStatement:      importedMod.MainChunk.Node.Preinit,
		AddDefaultPermissions: true,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, fmt.Errorf("import: manifest: %s", err.Error())
	}

	if ok, missingPerms := manifest.ArePermsGranted(grantedPerms, forbiddenPerms); !ok {
		list := utils.MapSlice(missingPerms, func(p Permission) string {
			return p.String()
		})
		return nil, fmt.Errorf("import: some permissions in the imported module's manifest are not granted: %s", strings.Join(list, "\n"))
	}

	routineCtx := NewContext(ContextConfig{
		Permissions:          grantedPerms,
		ForbiddenPermissions: forbiddenPerms,
		ParentContext:        config.ParentState.Ctx,
	})

	// add base patterns
	var basePatterns map[string]Pattern
	var basePatternNamespaces map[string]*PatternNamespace
	basePatterns, basePatternNamespaces = config.ParentState.GetBasePatternsForImportedModule()

	for name, patt := range basePatterns {
		routineCtx.AddNamedPattern(name, patt)
	}
	for name, ns := range basePatternNamespaces {
		routineCtx.AddPatternNamespace(name, ns)
	}

	// add base globals
	var globals GlobalVariables
	if config.ParentState.GetBaseGlobalsForImportedModule != nil {
		baseGlobals, err := config.ParentState.GetBaseGlobalsForImportedModule(routineCtx, manifest)
		if err != nil {
			return nil, err
		}
		globals = baseGlobals
	} else {
		globals = GlobalVariablesFromMap(map[string]Value{}, nil)
	}

	// pass patterns of the preinit state to the context
	if preinitState != nil {
		for name, patt := range preinitState.Global.Ctx.GetNamedPatterns() {
			if _, ok := basePatterns[name]; ok {
				continue
			}
			routineCtx.AddNamedPattern(name, patt)
		}
		for name, ns := range preinitState.Global.Ctx.GetPatternNamespaces() {
			if _, ok := basePatternNamespaces[name]; ok {
				continue
			}
			routineCtx.AddPatternNamespace(name, ns)
		}
	}

	if config.ArgObj != nil {
		args, err := manifest.Parameters.GetArgumentsFromObject(routineCtx, config.ArgObj)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		globals.Set(globalnames.MOD_ARGS_VARNAME, args)
	} else {
		globals.Set(globalnames.MOD_ARGS_VARNAME, Nil)
	}

	logLevels := config.ParentState.LogLevels
	logger := slog.ChildLoggerForSource(config.ParentState.Logger, importedMod.Name())

	resourceName, ok := importedMod.AbsoluteSource()
	if ok {
		logger = logger.Level(logLevels.LevelFor(resourceName.(ResourceName)))
	}

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: config.ParentState,
		Globals:      globals,
		Module:       importedMod,
		Manifest:     manifest,
		LthreadCtx:   routineCtx,

		Logger: logger,

		//Bytecode: //TODO
		//AbsScriptDir: absScriptDir,
		Timeout:                      time.Until(deadline),
		IgnoreCreateLThreadPermCheck: true,

		//TODO
		//IsTestingEnabled: parentState.TestingState.IsTestingEnabled && parentState.TestingState.IsImportTestingEnabled,
		//TestFilters:      parentState.TestingState.Filters,
	})
	if err != nil {
		return nil, fmt.Errorf("import: %s", err.Error())
	}

	config.ParentState.SetDescendantState(config.Src, lthread.state)

	return lthread, nil
}
