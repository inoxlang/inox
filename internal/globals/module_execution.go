package internal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

type ScriptPreparationArgs struct {
	Fpath string

	CliArgs []string
	Args    *core.Object

	ParsingCompilationContext *core.Context
	ParentContext             *core.Context
	UseContextAsParent        bool
	IgnoreNonCriticalIssues   bool

	Out io.Writer
}

// PrepareLocalScript parses & checks a script located in the filesystem and initialize its state.
func PrepareLocalScript(args ScriptPreparationArgs) (state *core.GlobalState, mod *core.Module, finalErr error) {
	// parse module

	absPath, pathErr := filepath.Abs(args.Fpath)
	if pathErr != nil {
		return nil, nil, pathErr
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

	if mod != nil {
		manifest, manifestErr = mod.EvalManifest(core.ManifestEvaluationConfig{
			GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
			DefaultLimitations:    DEFAULT_SCRIPT_LIMITATIONS,
			AddDefaultPermissions: true,
			IgnoreUnknownSections: args.IgnoreNonCriticalIssues,
			IgnoreConstDeclErrors: args.IgnoreNonCriticalIssues,
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

	state = NewDefaultGlobalState(ctx, manifest.EnvPattern, out)
	state.Module = mod

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
		Module:       mod,
		Node:         mod.MainChunk.Node,
		Chunk:        mod.MainChunk,
		GlobalConsts: state.Globals,
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

	if staticCheckErr != nil && staticCheckData == nil {
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

	// symbolic check

	globals := map[string]any{}
	state.Globals.Foreach(func(k string, v core.Value) {
		globals[k] = v
	})

	delete(globals, core.MOD_ARGS_VARNAME)
	additionalSymbolicGlobals := map[string]symbolic.SymbolicValue{
		core.MOD_ARGS_VARNAME: manifest.Parameters.GetSymbolicArguments(),
	}

	symbolicCtx, err_ := state.Ctx.ToSymbolicValue()
	if err_ != nil {
		return nil, nil, err_
	}

	symbolicData, err_ := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
		Node:                           mod.MainChunk.Node,
		Module:                         state.Module.ToSymbolic(),
		GlobalConsts:                   globals,
		AdditionalSymbolicGlobalConsts: additionalSymbolicGlobals,
		Context:                        symbolicCtx,
	})

	if symbolicData != nil {
		state.SymbolicData.AddData(symbolicData)
	}

	if parsingErr != nil {
		finalErr = parsingErr
	}

	if finalErr == nil && manifestErr != nil {
		finalErr = manifestErr
	}

	if finalErr == nil && err_ != nil {
		finalErr = err_
	}

	if finalErr == nil && staticCheckErr != nil {
		finalErr = staticCheckErr
	}

	if finalErr == nil && modArgsError != nil {
		finalErr = modArgsError
	}

	return state, mod, finalErr
}

type RunScriptArgs struct {
	Fpath                     string
	PassedCLIArgs             []string
	PassedArgs                *core.Object
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
		CliArgs:                   args.PassedCLIArgs,
		Args:                      args.PassedArgs,
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

// GetCheckData returns a map that can be safely marshaled to JSON, the data has the following structure:
//
//	{
//		parsingErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//		staticCheckErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//		symbolicCheckErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//	}
func GetCheckData(fpath string, compilationCtx *core.Context, out io.Writer) map[string]any {
	state, mod, err := PrepareLocalScript(ScriptPreparationArgs{
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
