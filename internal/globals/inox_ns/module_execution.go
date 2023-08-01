package inox_ns

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"

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
	Args    *core.Struct
	//if set the result of the function is used instead of .Args
	GetArguments func(*core.Manifest) (*core.Struct, error)

	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	ParentContextRequired     bool
	UseParentStateAsMainState bool
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

// PrepareLocalScript parses & checks a script located in the filesystem and initializes its state.
func PrepareLocalScript(args ScriptPreparationArgs) (state *core.GlobalState, mod *core.Module, manif *core.Manifest, finalErr error) {
	// parse module

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
	}

	absPath, err := args.ParsingCompilationContext.GetFileSystem().Absolute(args.Fpath)
	if err != nil {
		finalErr = fmt.Errorf("failed to get absolute path of module: %w", err)
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

	var (
		parentState              *core.GlobalState
		manifest                 *core.Manifest
		preinitState             *core.TreeWalkState
		preinitErr               error
		preinitStaticCheckErrors []*core.StaticCheckError
	)

	if parentContext != nil {
		parentState = parentContext.GetClosestState()
	}

	if mod != nil {
		manifest, preinitState, preinitStaticCheckErrors, preinitErr = mod.PreInit(core.PreinitArgs{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			ParentState:           parentState,
			PreinitStatement:      mod.MainChunk.Node.Preinit,
			PreinitFilesystem:     args.PreinitFilesystem,
			DefaultLimitations:    default_state.GetDefaultScriptLimitations(),
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.DevMode,
			IgnoreConstDeclErrors: args.DevMode,
		})

		if !args.DevMode && preinitErr != nil {
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
		if config.Provided != nil {
			dbs[config.Name] = config.Provided
			continue
		}

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
		dbs[config.Name] = core.WrapDatabase(ctx, db)
	}

	// create the script's state

	globalState, err := default_state.NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
		AbsoluteModulePath: absPath,

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
	state.Manifest = manifest
	state.PrenitStaticCheckErrors = preinitStaticCheckErrors
	state.MainPreinitError = preinitErr
	state.FirstDatabaseOpeningError = dbOpeningError
	state.Databases = dbs
	if args.UseParentStateAsMainState {
		if parentState == nil {
			panic(core.ErrUnreachable)
		}
		state.MainState = parentState
	} else {
		state.MainState = state
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

	// CLI arguments | arguments of imported module
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
		if args.DevMode || manifest.Parameters.NoParameters() {
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

type IncludableChunkfilePreparationArgs struct {
	Fpath string //path of the file in the .ParsingCompilationContext's filesystem.

	ParsingContext *core.Context

	Out    io.Writer //defaults to os.Stdout
	LogOut io.Writer //defaults to Out

	//used to create the context
	IncludedChunkContextFileSystem afs.Filesystem
}

// PrepareDevModeIncludableChunkfile parses & checks an includable-chunk file located in the filesystem and initializes its state.
func PrepareDevModeIncludableChunkfile(args IncludableChunkfilePreparationArgs) (state *core.GlobalState, _ *core.Module, _ *core.IncludedChunk, finalErr error) {
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
		Permissions:     nil,
		Limitations:     nil,
		HostResolutions: nil,
		Filesystem:      args.IncludedChunkContextFileSystem,
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

	symbolicCtx, err_ := state.Ctx.ToSymbolicValue()
	if err_ != nil {
		finalErr = parsingErr
		return
	}

	symbolicData, err_ := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
		Node:    mod.MainChunk.Node,
		Module:  state.Module.ToSymbolic(),
		Globals: globals,
		Context: symbolicCtx,
	})

	if symbolicData != nil {
		state.SymbolicData.AddData(symbolicData)
	}

	if parsingErr != nil { //priority to parsing error
		finalErr = parsingErr
	} else if finalErr == nil {
		switch {
		case err_ != nil:
			finalErr = err_
		case staticCheckErr != nil:
			finalErr = staticCheckErr
		}
	}

	return state, mod, includedChunk, finalErr
}

type RunScriptArgs struct {
	Fpath                     string
	PassedCLIArgs             []string
	PassedArgs                *core.Struct
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

	//if not nil AND UseBytecode is false the script is executed in debug mode with this debugger.
	//Debugger.AttachAndStart is called before starting the evaluation.
	//if nil the parent state's debugger is used if present.
	Debugger *core.Debugger

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

	state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
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

	return RunPreparedScript(RunPreparedScriptArgs{
		State:                     state,
		ParsingCompilationContext: args.ParsingCompilationContext,
		ParentContext:             args.ParentContext,
		IgnoreHighRiskScore:       args.IgnoreHighRiskScore,

		UseBytecode:      args.UseBytecode,
		OptimizeBytecode: args.OptimizeBytecode,
		ShowBytecode:     args.ShowBytecode,

		Debugger: args.Debugger,
	})
}

type RunPreparedScriptArgs struct {
	State                     *core.GlobalState
	ParsingCompilationContext *core.Context
	ParentContext             *core.Context

	IgnoreHighRiskScore bool

	UseBytecode      bool
	OptimizeBytecode bool
	ShowBytecode     bool

	Debugger *core.Debugger
}

// RunPreparedScript runs a script located in the filesystem.
func RunPreparedScript(args RunPreparedScriptArgs) (
	scriptResult core.Value, scriptState *core.GlobalState, scriptModule *core.Module,
	preparationSuccess bool, _err error,
) {

	state := args.State
	out := state.Out
	mod := state.Module
	if mod == nil {
		return nil, nil, nil, true, errors.New("no module found")
	}
	manifest := state.Manifest

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
	debugger := args.Debugger
	if debugger == nil && args.ParentContext != nil {
		closestState := args.ParentContext.GetClosestState()
		parentDebugger, _ := closestState.Debugger.Load().(*core.Debugger)
		if parentDebugger != nil {
			debugger = parentDebugger.NewChild()
		}
	}
	if debugger != nil {
		debugger.AttachAndStart(treeWalkState)
		defer func() {
			go func() {
				debugger.ControlChan() <- core.DebugCommandCloseDebugger{}
			}()
		}()
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
