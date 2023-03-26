package internal

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

type ScriptPreparationArgs struct {
	Fpath                     string
	PassedArgs                []string
	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	UseContextAsParent        bool

	Out io.Writer
}

// PrepareLocalScript parses & checks a script located in the filesystem and initialize its state.
func PrepareLocalScript(args ScriptPreparationArgs) (state *core.GlobalState, mod *core.Module, finalErr error) {
	passCLIArguments := false

	handleCustomPermType := func(kind core.PermissionKind, name string, value core.Value) ([]core.Permission, bool, error) {
		if kind != core.ReadPerm || name != "cli-args" {
			return nil, false, nil
		}
		boolean, ok := value.(core.Bool)
		if !ok {
			return nil, true, errors.New("cli-args should have a boolean value")
		}

		passCLIArguments = bool(boolean)
		return nil, true, nil //okay to not give a permission ???
	}

	// parse module

	absPath, err := filepath.Abs(args.Fpath)
	if err != nil {
		return nil, nil, err
	}

	args.Fpath = absPath

	module, parsingErr := core.ParseLocalModule(core.LocalModuleParsingConfig{
		ModuleFilepath: args.Fpath,
		Context:        args.ParsingCompilationContext,
	})

	mod = module

	if parsingErr != nil && mod == nil {
		finalErr = parsingErr
		return
	}

	//create context and state

	var ctx *core.Context

	var parentContext *core.Context
	if args.UseContextAsParent {
		parentContext = args.ParentContext
	}

	var manifest *core.Manifest
	var manifestErr error

	if parsingErr == nil {
		manifest, manifestErr = mod.EvalManifest(core.ManifestEvaluationConfig{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			DefaultLimitations:    DEFAULT_LIMITATIONS,
			HandleCustomType:      handleCustomPermType,
			AddDefaultPermissions: true,
		})

		if manifest == nil {
			manifest = core.NewEmptyManifest()
		}

	} else {
		manifest = core.NewEmptyManifest()
	}

	var ctxErr error
	ctx, ctxErr = NewDefaultContext(DefaultContextConfig{
		Permissions:     manifest.RequiredPermissions,
		Limitations:     manifest.Limitations,
		HostResolutions: manifest.HostResolutions,
		ParentContext:   parentContext,
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

	state = NewDefaultGlobalState(ctx, out)
	state.Module = mod

	if passCLIArguments {
		cliArgs := []core.Value{}
		for _, arg := range args.PassedArgs {
			cliArgs = append(cliArgs, core.Str(arg))
		}
		state.Globals.Set("args", core.NewWrappedValueList(cliArgs...))
	}

	// static check

	staticCheckData, staticCheckErr := core.StaticCheck(core.StaticCheckInput{
		Module:            mod,
		Node:              mod.MainChunk.Node,
		Chunk:             mod.MainChunk,
		GlobalConsts:      state.Globals,
		Patterns:          state.Ctx.GetNamedPatterns(),
		PatternNamespaces: state.Ctx.GetPatternNamespaces(),
	})

	state.StaticCheckData = staticCheckData

	if staticCheckErr != nil && staticCheckData == nil {
		finalErr = err
		return
	}

	if parsingErr != nil {
		finalErr = parsingErr
		return
	}

	// symbolic check

	if parsingErr == nil {
		globals := map[string]any{}
		state.Globals.Foreach(func(k string, v core.Value) {
			globals[k] = v
		})

		symbolicCtx, err_ := state.Ctx.ToSymbolicValue()
		if err_ != nil {
			return nil, nil, err
		}

		symbolicData, err_ := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
			Node:         mod.MainChunk.Node,
			Module:       state.Module.ToSymbolic(),
			GlobalConsts: globals,
			Context:      symbolicCtx,
		})

		if symbolicData != nil {
			state.SymbolicData.AddData(symbolicData)
		}

		err = err_
	}

	if err == nil && manifestErr != nil {
		err = manifestErr
	}

	if err == nil && staticCheckErr != nil {
		err = staticCheckErr
	}

	return state, mod, err
}

type RunScriptArgs struct {
	Fpath                     string
	PassedArgs                []string
	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	UseContextAsParent        bool
	UseBytecode               bool
	OptimizeBytecode          bool
	ShowBytecode              bool

	//output for execution, if nil os.Stdout is used
	Out io.Writer
}

// RunLocalScript runs a script located in the filesystem.
func RunLocalScript(args RunScriptArgs) (core.Value, *core.GlobalState, *core.Module, error) {

	if args.UseContextAsParent && args.ParentContext == nil {
		return nil, nil, nil, errors.New(".UseContextAsParent is set to true but passed .Context is nil")
	}

	state, mod, err := PrepareLocalScript(ScriptPreparationArgs{
		Fpath:                     args.Fpath,
		PassedArgs:                args.PassedArgs,
		ParsingCompilationContext: args.ParsingCompilationContext,
		ParentContext:             args.ParentContext,
		UseContextAsParent:        args.UseContextAsParent,
		Out:                       args.Out,
	})

	if err != nil {
		return nil, state, mod, err
	}

	out := state.Out
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

		return res, state, mod, err
	}

	res, err := core.TreeWalkEval(state.Module.MainChunk.Node, core.NewTreeWalkStateWithGlobal(state))
	return res, state, mod, err
}
