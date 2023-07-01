package inox_ns

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_ALLOWED_WARNINGS = 10
)

var (
	ErrExecutionAbortedTooManyWarnings = errors.New("execution was aborted because there are too many warnings")
	ErrUserRefusedExecution            = errors.New("user refused execution")
	ErrNoProvidedConfirmExecPrompt     = errors.New("risk score too high and no provided way to show confirm prompt")
	ErrDatabaseOpenFunctionNotFound    = errors.New("function to open database not found")
)

type ScriptPreparationArgs struct {
	Fpath string //path of the script in the .ParsingCompilationContext's filesystem.

	CliArgs []string
	Args    *core.Object

	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	ParentContextRequired     bool
	DevMode                   bool
	AllowMissingEnvVars       bool
	FullAccessToDatabases     bool

	Out    io.Writer //defaults to os.Stdout
	LogOut io.Writer //defaults to Out

	//used during the preinit
	PreinitFilesystem afs.Filesystem

	//used to create the context
	ScriptContextFileSystem afs.Filesystem
}

// PrepareLocalScript parses & checks a script located in the filesystem and initialize its state.
func PrepareLocalScript(args ScriptPreparationArgs) (state *core.GlobalState, mod *core.Module, manif *core.Manifest, finalErr error) {
	// parse module

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
	}

	absPath, pathErr := filepath.Abs(args.Fpath)
	if pathErr != nil {
		finalErr = fmt.Errorf("failed to get absolute path of script: %w", pathErr)
		return
	}

	args.Fpath = absPath

	module, parsingErr := core.ParseLocalModule(core.LocalModuleParsingConfig{
		ModuleFilepath:                      args.Fpath,
		Context:                             args.ParsingCompilationContext,
		RecoverFromNonExistingIncludedFiles: args.DevMode,
	})

	mod = module

	if parsingErr != nil && mod == nil {
		finalErr = parsingErr
		return
	}

	//create context and state

	var ctx *core.Context

	parentContext := args.ParentContext

	var manifest *core.Manifest
	var preinitState *core.TreeWalkState
	var preinitErr error
	var preinitStaticCheckErrors []*core.StaticCheckError

	if mod != nil {
		manifest, preinitState, preinitStaticCheckErrors, preinitErr = mod.PreInit(core.PreinitArgs{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			PreinitStatement:      mod.MainChunk.Node.Preinit,
			PreinitFilesystem:     args.PreinitFilesystem,
			DefaultLimitations:    default_state.GetDefaultScriptLimitations(),
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.DevMode,
			IgnoreConstDeclErrors: args.DevMode,
		})

		if manifest == nil {
			manifest = core.NewEmptyManifest()
		}

	} else {
		manifest = core.NewEmptyManifest()
	}

	//create the script's context
	var ctxErr error

	ctx, ctxErr = default_state.NewDefaultContext(default_state.DefaultContextConfig{
		Permissions:     manifest.RequiredPermissions,
		Limitations:     manifest.Limitations,
		HostResolutions: manifest.HostResolutions,
		ParentContext:   parentContext,
		Filesystem:      args.ScriptContextFileSystem,
	})

	if ctxErr != nil {
		finalErr = ctxErr
		return
	}

	defer func() {
		if finalErr != nil {
			ctx.Cancel()
		}
	}()

	out := args.Out
	if out == nil {
		out = os.Stdout
	}

	//connect to databases
	//TODO: disconnect if connection still not used after a few minutes

	var dbOpeningError error
	dbs := map[string]*core.DatabaseIL{}
	for _, config := range manifest.Databases {
		if host, ok := config.Resource.(core.Host); ok {
			ctx.AddHostResolutionData(host, config.ResolutionData)
		}

		openDB, ok := core.GetOpenDbFn(config.Resource.Scheme())
		if !ok {
			ctx.Cancel()
			return nil, nil, nil, ErrDatabaseOpenFunctionNotFound
		}

		//possible futures issues because there is no state in the context
		db, err := openDB(ctx, core.DbOpenConfiguration{
			Resource:       config.Resource,
			ResolutionData: config.ResolutionData,
			FullAccess:     args.FullAccessToDatabases,
		})
		if err != nil {
			err = fmt.Errorf("failed to open the '%s' database: %w", config.Name, err)
			if !args.DevMode {
				ctx.Cancel()
				return nil, nil, nil, err
			}
			dbOpeningError = err
			//TODO: use cached schema
			db = core.NewFailedToOpenDatabase()
		}
		dbs[config.Name] = core.WrapDatabase(db)
	}

	// create the script's state

	globalState, err := default_state.NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
		EnvPattern:          manifest.EnvPattern,
		PreinitFiles:        manifest.PreinitFiles,
		Databases:           dbs,
		AllowMissingEnvVars: args.AllowMissingEnvVars,
		Out:                 out,
		LogOut:              args.LogOut,
	})
	if err != nil {
		finalErr = fmt.Errorf("failed to create global state: %w", err)
		return
	}
	state = globalState
	state.Module = mod
	state.PrenitStaticCheckErrors = preinitStaticCheckErrors
	state.MainPreinitError = preinitErr
	state.FirstDatabaseOpeningError = dbOpeningError
	state.Databases = dbs

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

	// CLI arguments | arguments of imported module
	var modArgs *core.Object
	var modArgsError error

	if args.Args != nil {
		modArgs, modArgsError = manifest.Parameters.GetArguments(ctx, args.Args)
	} else if args.CliArgs != nil {
		args, err := manifest.Parameters.GetArgumentsFromCliArgs(ctx, args.CliArgs)
		if err != nil {
			modArgsError = fmt.Errorf("%w\nusage: %s", err, manifest.Usage())
		} else {
			modArgs = args
		}
	} else {
		modArgs = core.NewObject()
	}

	if modArgsError == nil {
		state.Globals.Set(core.MOD_ARGS_VARNAME, modArgs)
	}

	// static check

	staticCheckData, staticCheckErr := core.StaticCheck(core.StaticCheckInput{
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
		core.MOD_ARGS_VARNAME: manifest.Parameters.GetSymbolicArguments(),
	}

	symbolicCtx, err_ := state.Ctx.ToSymbolicValue()
	if err_ != nil {
		finalErr = parsingErr
		return
	}

	symbolicData, err_ := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
		Node:                           mod.MainChunk.Node,
		Module:                         state.Module.ToSymbolic(),
		Globals:                        globals,
		AdditionalSymbolicGlobalConsts: additionalSymbolicGlobals,
		Context:                        symbolicCtx,
	})

	if symbolicData != nil {
		state.SymbolicData.AddData(symbolicData)
	}

	if parsingErr != nil { //priority to parsing error
		finalErr = parsingErr
	} else if finalErr == nil {
		switch {
		case preinitErr != nil:
			finalErr = preinitErr
		case err_ != nil:
			finalErr = err_
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

type RunScriptArgs struct {
	Fpath                     string
	PassedCLIArgs             []string
	PassedArgs                *core.Object
	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	ParentContextRequired     bool
	//used during the preinit
	PreinitFilesystem afs.Filesystem

	FullAccessToDatabases bool

	UseBytecode      bool
	OptimizeBytecode bool
	ShowBytecode     bool

	AllowMissingEnvVars bool
	IgnoreHighRiskScore bool

	Debugger *core.Debugger //if not nil the script is executed in debug mode with this debugger

	//output for execution, if nil os.Stdout is used
	Out io.Writer

	LogOut io.Writer

	//PreparedChan signals when the script is prepared (nil error) or failed to prepared (non-nil error),
	//the channel should be buffered.
	PreparedChan chan error
}

// RunLocalScript runs a script located in the filesystem.
func RunLocalScript(args RunScriptArgs) (
	scriptResult core.Value, scriptState *core.GlobalState, scriptModule *core.Module,
	preparationSuccess bool, _err error,
) {

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, false, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
	}

	state, mod, manifest, err := PrepareLocalScript(ScriptPreparationArgs{
		Fpath:                     args.Fpath,
		CliArgs:                   args.PassedCLIArgs,
		Args:                      args.PassedArgs,
		ParsingCompilationContext: args.ParsingCompilationContext,
		ParentContext:             args.ParentContext,
		ParentContextRequired:     args.ParentContextRequired,
		Out:                       args.Out,
		LogOut:                    args.LogOut,
		AllowMissingEnvVars:       args.AllowMissingEnvVars,
		PreinitFilesystem:         args.PreinitFilesystem,
		FullAccessToDatabases:     args.FullAccessToDatabases,
	})

	if args.PreparedChan != nil {
		select {
		case args.PreparedChan <- err:
		default:
		}
	}

	if err != nil {
		return nil, state, mod, false, err
	}

	out := state.Out

	//show warnings
	warnings := state.SymbolicData.Warnings()
	for _, warning := range warnings {
		fmt.Fprintln(out, warning.LocatedMessage)
	}

	if len(warnings) > DEFAULT_MAX_ALLOWED_WARNINGS { //TODO: make the max configurable
		return nil, nil, nil, true, ErrExecutionAbortedTooManyWarnings
	}

	riskScore, requiredPerms := core.ComputeProgramRiskScore(mod, manifest)

	// if the program is risky ask the user to confirm the execution
	if !args.IgnoreHighRiskScore && riskScore > config.DEFAULT_TRUSTED_RISK_SCORE {
		waitConfirmPrompt := args.ParsingCompilationContext.GetWaitConfirmPrompt()
		if waitConfirmPrompt == nil {
			return nil, nil, nil, true, ErrNoProvidedConfirmExecPrompt
		}
		msg := bytes.NewBufferString(mod.Name())
		msg.WriteString("\nrisk score is ")
		msg.WriteString(riskScore.ValueAndLevel())
		msg.WriteString("\nthe program is asking for the following permissions:\n")

		for _, perm := range requiredPerms {
			//ignore global var permissions
			if _, ok := perm.(core.GlobalVarPermission); ok {
				continue
			}
			msg.WriteByte('\t')
			msg.WriteString(perm.String())
			msg.WriteByte('\n')
		}
		msg.WriteString("allow execution (y,yes) ? ")

		if ok, err := waitConfirmPrompt(msg.String(), []string{"y", "yes"}); err != nil {
			return nil, nil, nil, true, fmt.Errorf("failed to show confirm prompt to user: %w", err)
		} else if !ok {
			return nil, nil, nil, true, ErrUserRefusedExecution
		}
	}

	state.InitSystemGraph()

	defer state.Ctx.Cancel()

	//execute the script

	if args.UseBytecode {
		tracer := io.Discard
		if args.ShowBytecode {
			tracer = out
		}
		res, err := core.EvalVM(state.Module, state, core.BytecodeEvaluationConfig{
			Tracer:               tracer,
			ShowCompilationTrace: args.ShowBytecode,
			OptimizeBytecode:     args.OptimizeBytecode,
			CompilationContext:   args.ParsingCompilationContext,
		})

		return res, state, mod, true, err
	}

	treeWalkState := core.NewTreeWalkStateWithGlobal(state)
	if args.Debugger != nil {
		args.Debugger.AttachAndStart(treeWalkState)
	}

	res, err := core.TreeWalkEval(state.Module.MainChunk.Node, treeWalkState)
	return res, state, mod, true, err
}

// GetCheckData returns a map that can be safely marshaled to JSON, the data has the following structure:
//
//	{
//		parsingErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//		staticCheckErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//		symbolicCheckErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//	}
func GetCheckData(fpath string, compilationCtx *core.Context, out io.Writer) map[string]any {
	state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
		Fpath:                     fpath,
		Args:                      nil,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       out,
	})

	data := map[string]any{
		"parsingErrors":       []any{},
		"staticCheckErrors":   []any{},
		"symbolicCheckErrors": []any{},
	}

	if err == nil {
		return data
	}

	if err != nil && state == nil && mod == nil {
		return data
	}

	{
		i := -1

		fmt.Fprintln(os.Stderr, len(mod.ParsingErrors), len(mod.ParsingErrorPositions))
		data["parsingErrors"] = utils.MapSlice(mod.ParsingErrors, func(err core.Error) any {
			i++
			return map[string]any{
				"text":     err.Text(),
				"location": mod.ParsingErrorPositions[i],
			}
		})
	}

	if state != nil && state.StaticCheckData != nil {
		i := -1
		data["staticCheckErrors"] = utils.MapSlice(state.StaticCheckData.Errors(), func(err *core.StaticCheckError) any {
			i++
			return map[string]any{
				"text":     err.Message,
				"location": err.Location[0],
			}
		})
		i = -1

		data["symbolicCheckErrors"] = utils.MapSlice(state.SymbolicData.Errors(), func(err symbolic.SymbolicEvaluationError) any {
			i++
			return map[string]any{
				"text":     err.Message,
				"location": err.Location[0],
			}
		})
	}

	return data
}
