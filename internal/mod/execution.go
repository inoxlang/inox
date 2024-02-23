package mod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	DEFAULT_MAX_ALLOWED_WARNINGS = 10
)

var (
	ErrExecutionAbortedTooManyWarnings = errors.New("execution was aborted because there are too many warnings")
	ErrUserRefusedExecution            = errors.New("user refused execution")
	ErrNoProvidedConfirmExecPrompt     = errors.New("risk score too high and no provided way to show confirm prompt")
)

type RunLocalModuleArgs struct {
	Fpath                     string
	PassedCLIArgs             []string
	PassedArgs                *core.ModuleArgs
	ParsingCompilationContext *core.Context

	ParentContext         *core.Context
	ParentContextRequired bool              //make .ParentContext required
	StdlibCtx             context.Context   //should not be set if ParentContext is set
	AdditionalPermissions []core.Permission //should not be set if ParentContext is set

	//used during the preinit
	PreinitFilesystem afs.Filesystem

	//If nil the parent context's filesystem is used.
	//If there is no parent context the OS filesystem is used.
	ScriptContextFileSystem afs.Filesystem

	FullAccessToDatabases bool
	Project               *project.Project
	MemberAuthToken       string

	UseBytecode      bool
	OptimizeBytecode bool
	ShowBytecode     bool

	AllowMissingEnvVars bool
	IgnoreHighRiskScore bool

	EnableTesting bool
	TestFilters   core.TestFilters

	//if not nil AND UseBytecode is false the script is executed in debug mode with this debugger.
	//Debugger.AttachAndStart is called before starting the evaluation.
	//if nil the parent state's debugger is used if present.
	Debugger *core.Debugger

	//output for execution, if nil os.Stdout is used
	Out io.Writer

	//ignored if .Logger is set
	LogOut    io.Writer
	Logger    zerolog.Logger
	LogLevels *core.LogLevels

	//PreparedChan signals when the script is prepared (nil error) or failed to be prepared (non-nil error),
	//the channel should be buffered.
	PreparedChan chan error

	OnPrepared func(state *core.GlobalState) error
}

// RunLocalModule runs a module located in the filesystem.
func RunLocalModule(args RunLocalModuleArgs) (
	scriptResult core.Value, scriptState *core.GlobalState, scriptModule *core.Module,
	preparationSuccess bool, _err error,
) {

	if args.ParentContextRequired && args.ParentContext == nil {
		return nil, nil, nil, false, errors.New(".ParentContextRequired is set to true but passed .ParentContext is nil")
	}

	state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                     args.Fpath,
		CliArgs:                   args.PassedCLIArgs,
		Args:                      args.PassedArgs,
		ParsingCompilationContext: args.ParsingCompilationContext,
		ParentContext:             args.ParentContext,
		ParentContextRequired:     args.ParentContextRequired,
		StdlibCtx:                 args.StdlibCtx,
		DefaultLimits:             core.GetDefaultScriptLimits(),
		AdditionalPermissions:     args.AdditionalPermissions,
		ScriptContextFileSystem:   args.ScriptContextFileSystem,

		Out:       args.Out,
		LogOut:    args.LogOut,
		Logger:    args.Logger,
		LogLevels: args.LogLevels,

		AllowMissingEnvVars:   args.AllowMissingEnvVars,
		PreinitFilesystem:     args.PreinitFilesystem,
		FullAccessToDatabases: args.FullAccessToDatabases,
		Project:               args.Project,
		MemberAuthToken:       args.MemberAuthToken,

		EnableTesting: args.EnableTesting,
		TestFilters:   args.TestFilters,
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

	if args.OnPrepared != nil {
		err := args.OnPrepared(state)
		if err != nil {
			return nil, state, mod, false, err
		}
	}

	return RunPreparedModule(RunPreparedModuleArgs{
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

type RunPreparedModuleArgs struct {
	State                     *core.GlobalState
	ParsingCompilationContext *core.Context
	ParentContext             *core.Context

	//do not show the confirmation prompt to the user if the risk score is high.
	IgnoreHighRiskScore bool

	UseBytecode             bool
	OptimizeBytecode        bool
	ShowBytecode            bool
	DoNotCancelWhenFinished bool

	Debugger *core.Debugger
}

// RunPreparedModule runs a prepared module with the configuration specified in RunPreparedModuleArgs (e.g. .UseBytecode).
func RunPreparedModule(args RunPreparedModuleArgs) (
	moduleResult core.Value, moduleState *core.GlobalState, module *core.Module,
	preparationSuccess bool, _err error,
) {

	defer func() {
		if e := recover(); e != nil {
			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())
			moduleResult = nil
			moduleState = nil
			module = nil
			preparationSuccess = true
			_err = err
		}
	}()

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

	// if len(warnings) > DEFAULT_MAX_ALLOWED_WARNINGS { //TODO: make the max configurable
	// 	return nil, nil, nil, true, ErrExecutionAbortedTooManyWarnings
	// }

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

	if !args.DoNotCancelWhenFinished {
		defer state.Ctx.CancelGracefully()
	}

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
