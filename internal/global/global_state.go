package globals

import (
	"fmt"
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/namespaces"
	"golang.org/x/exp/maps"

	"github.com/rs/zerolog"
)

const (
	DEFAULT_MODULE_LOG_LEVEL = zerolog.InfoLevel
)

func Init() {
	//set initial working directory on unix, on WASM it's done by the main package
	// targetSpecificInit()
	// registerHelp()

	// inoxsh_ns.SetNewDefaultGlobalState(func(ctx *core.Context, envPattern *core.ObjectPattern, out io.Writer) *core.GlobalState {
	// 	return utils.Must(NewDefaultGlobalState(ctx, core.DefaultGlobalStateConfig{
	// 		EnvPattern: envPattern,
	// 		Out:        out,
	// 	}))
	// })

	core.SetNewDefaultGlobalStateFn(NewDefaultGlobalState)
	core.SetNewDefaultContext(NewDefaultContext)
	core.SetDefaultScriptLimits(DEFAULT_SCRIPT_LIMITS)
}

// NewDefaultGlobalState creates a new GlobalState with the default globals.
func NewDefaultGlobalState(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {

	// //create env namespace

	// envNamespace, err := env_ns.NewEnvNamespace(ctx, conf.EnvPattern, conf.AllowMissingEnvVars)
	// if err != nil {
	// 	return nil, err
	// }

	//create value for the preinit-data global
	var preinitFilesKeys []string
	var preinitDataValues []core.Serializable
	for _, preinitFile := range conf.PreinitFiles {
		preinitFilesKeys = append(preinitFilesKeys, preinitFile.Name)
		preinitDataValues = append(preinitDataValues, preinitFile.Parsed)
	}

	preinitData :=
		core.NewRecordFromKeyValLists([]string{"files"}, []core.Serializable{core.NewRecordFromKeyValLists(preinitFilesKeys, preinitDataValues)})

	initialWorkingDir := ctx.InitialWorkingDirectory()

	constants := map[string]core.Value{
		// constants
		globalnames.INITIAL_WORKING_DIR_VARNAME:        initialWorkingDir,
		globalnames.INITIAL_WORKING_DIR_PREFIX_VARNAME: initialWorkingDir.ToPrefixPattern(),
	}

	namespaces.AddNamespacesTo(constants)
	maps.Copy(constants, GLOBAL_FUNCTIONS)

	// for k, v := range transientcontainers.NewTransientContainersNamespace() {
	// 	constants[k] = v
	// }

	if conf.AbsoluteModulePath != "" {
		constants[globalnames.MODULE_DIRPATH] = core.DirPathFrom(filepath.Dir(conf.AbsoluteModulePath))
		constants[globalnames.MODULE_FILEPATH] = core.PathFrom(conf.AbsoluteModulePath)
	}

	baseGlobals := maps.Clone(constants)
	constants[globalnames.PREINIT_DATA] = preinitData

	symbolicBaseGlobals := map[string]symbolic.Value{}
	{
		encountered := map[uintptr]symbolic.Value{}
		for k, v := range baseGlobals {
			symbolicValue, err := v.ToSymbolicValue(ctx, encountered)
			if err != nil {
				return nil, fmt.Errorf("failed to convert base global '%s' to symbolic: %w", k, err)
			}
			symbolicBaseGlobals[k] = symbolicValue
		}
	}

	state := core.NewGlobalState(ctx, constants)
	state.Out = conf.Out
	state.Logger, state.LogLevels = getLoggerAndLevels(conf)
	state.GetBaseGlobalsForImportedModule = func(ctx *core.Context, manifest *core.Manifest) (core.GlobalVariables, error) {
		importedModuleGlobals := maps.Clone(baseGlobals)
		// env, err := env_ns.NewEnvNamespace(ctx, nil, conf.AllowMissingEnvVars)
		// if err != nil {
		// 	return core.GlobalVariables{}, err
		// }

		//importedModuleGlobals["env"] = env
		baseGlobalKeys := maps.Keys(importedModuleGlobals)
		return core.GlobalVariablesFromMap(importedModuleGlobals, baseGlobalKeys), nil
	}
	state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
		return maps.Clone(core.DEFAULT_NAMED_PATTERNS), maps.Clone(core.DEFAULT_PATTERN_NAMESPACES)
	}
	state.SymbolicBaseGlobalsForImportedModule = symbolicBaseGlobals
	state.OutputFieldsInitialized.Store(true)

	return state, nil
}
