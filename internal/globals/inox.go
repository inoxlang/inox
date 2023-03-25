package internal

import (
	"errors"
	"io"
	"path/filepath"

	"github.com/inox-project/inox/internal/commonfmt"
	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	parse "github.com/inox-project/inox/internal/parse"
)

var (
	SYMB_PREPARATION_ERRORS_RECORD = symbolic.NewRecord(map[string]symbolic.SymbolicValue{
		"parsing_errors":        symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"static_check_errors":   symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"symbolic_check_errors": symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"permission_error":      symbolic.NewMultivalue(symbolic.ANY_ERR, symbolic.Nil),
	})
	SYMB_RUN_ERRORS_RECORD = symbolic.NewRecord(map[string]symbolic.SymbolicValue{
		"parsing_errors":        symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"static_check_errors":   symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"symbolic_check_errors": symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"permission_error":      symbolic.NewMultivalue(symbolic.ANY_ERR, symbolic.Nil),
		"runtime_error":         symbolic.NewMultivalue(symbolic.ANY_ERR, symbolic.Nil),
	})
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		_parse_chunk, func(ctx *symbolic.Context, s *symbolic.String) (*symbolic.AstNode, *symbolic.Error) {
			return &symbolic.AstNode{}, nil
		},
		_parse_expr, func(ctx *symbolic.Context, s *symbolic.String) (*symbolic.AstNode, *symbolic.Error) {
			return &symbolic.AstNode{}, nil
		},
		_parse_local_script, func(ctx *symbolic.Context, p *symbolic.Path) (*symbolic.Module, *symbolic.Error) {
			return symbolic.ANY_MODULE, nil
		},
		_parse_in_memory_module, func(ctx *symbolic.Context, name, code *symbolic.String) (*symbolic.Module, *symbolic.Error) {
			return symbolic.ANY_MODULE, nil
		},
		_prepare_local_script, func(ctx *symbolic.Context, p *symbolic.Path) (*symbolic.Module, *symbolic.GlobalState, *symbolic.Record, *symbolic.Error) {
			return symbolic.ANY_MODULE, symbolic.ANY_GLOBAL_STATE, SYMB_PREPARATION_ERRORS_RECORD, nil
		},
		_run_local_script, func(ctx *symbolic.Context, p *symbolic.Path, config *symbolic.Object) (symbolic.SymbolicValue, *symbolic.GlobalState, *symbolic.Record, *symbolic.Error) {
			return symbolic.ANY, symbolic.ANY_GLOBAL_STATE, SYMB_PREPARATION_ERRORS_RECORD, nil
		},
	})
}

func NewInoxNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"parse_chunk":            core.WrapGoFunction(_parse_chunk),
		"parse_expr":             core.WrapGoFunction(_parse_expr),
		"parse_local_script":     core.WrapGoFunction(_parse_local_script),
		"parse_in_memory_module": core.WrapGoFunction(_parse_in_memory_module),
		"prepare_local_script":   core.WrapGoFunction(_prepare_local_script),
		"run_local_script":       core.WrapGoFunction(_run_local_script),
	})
}

func _parse_chunk(ctx *core.Context, s core.Str) (node core.AstNode, e error) {
	defer func() {
		err, ok := recover().(error)
		if ok {
			e = err
		}
	}()

	chunk, err := parse.ParseChunk(string(s), "")
	return core.AstNode{Node: chunk}, err
}

func _parse_expr(ctx *core.Context, s core.Str) (n core.AstNode, err error) {
	defer func() {
		e, ok := recover().(error)
		if ok {
			err = e
		}
	}()

	node, ok := parse.ParseExpression(string(s))
	if !ok {
		return core.AstNode{}, errors.New("invalid expression")
	}
	return core.AstNode{Node: node}, nil
}

func _parse_local_script(ctx *core.Context, src core.Path) (*core.Module, error) {
	absPath, err := filepath.Abs(string(src))
	if err != nil {
		return nil, err
	}

	mod, err := core.ParseLocalModule(core.LocalModuleParsingConfig{
		ModuleFilepath: absPath,
		Context:        ctx,
	})

	return mod, err
}

func _parse_in_memory_module(ctx *core.Context, name core.Str, code core.Str) (*core.Module, error) {
	mod, err := core.ParseInMemoryModule(code, core.InMemoryModuleParsingConfig{
		Name:    string(name),
		Context: ctx,
	})

	return mod, err
}

func _prepare_local_script(ctx *core.Context, src core.Path) (*core.Module, *core.GlobalState, *core.Record, error) {
	state, mod, err := PrepareLocalScript(ScriptPreparationArgs{
		Fpath:                     string(src),
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		UseContextAsParent:        true,

		Out: ctx.GetClosestState().Out,
	})

	errorRecord := core.ValMap{
		"parsing_errors":        core.NewTuple(nil),
		"static_check_errors":   core.NewTuple(nil),
		"symbolic_check_errors": core.NewTuple(nil),
		"permission_error":      core.Nil,
	}

	if err != nil && state == nil && mod == nil {
		return nil, nil, core.NewRecordFromMap(errorRecord), err
	}

	errorRecord["parsing_errors"] = mod.ParsingErrorTuple()

	var permissionError core.NotAllowedError

	if state != nil {
		if state.StaticCheckData != nil {
			errorRecord["static_check_errors"] = state.StaticCheckData.ErrorTuple()
			errorRecord["symbolic_check_errors"] = state.SymbolicData.ErrorTuple()
		}
	} else if errors.As(err, &permissionError) {
		errorRecord["permission_error"] = core.NewError(permissionError, core.Nil)
	}

	return mod, state, core.NewRecordFromMap(errorRecord), err
}

func _run_local_script(ctx *core.Context, src core.Path, config *core.Object) (core.Value, *core.GlobalState, *core.Record, error) {
	var out io.Writer = io.Discard

	if err := config.ForEachEntry(func(k string, v core.Value) error {

		switch k {
		case "out":
			writable, ok := v.(core.Writable)
			if !ok {
				return commonfmt.FmtInvalidValueForPropXOfArgY("out", "config", "a writable is expected")
			}
			out = writable.Writer()
		}

		return nil
	}); err != nil {
		return nil, nil, nil, err
	}

	runResult, state, mod, err := RunLocalScript(RunScriptArgs{
		Fpath:                     string(src),
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		UseContextAsParent:        true,

		UseBytecode:      ctx.GetClosestState().Module.IsCompiled(),
		OptimizeBytecode: true,

		Out: out,
	})

	var errorRecord = core.ValMap{
		"parsing_errors":        core.NewTuple(nil),
		"static_check_errors":   core.NewTuple(nil),
		"symbolic_check_errors": core.NewTuple(nil),
		"permission_error":      core.Nil,
		"runtime_error":         core.Nil,
	}

	if err != nil && state == nil && mod == nil {
		return nil, nil, core.NewRecordFromMap(errorRecord), err
	}

	errorRecord["parsing_errors"] = mod.ParsingErrorTuple()

	var permissionError core.NotAllowedError

	if state != nil {
		if state.StaticCheckData != nil {
			errorRecord["static_check_errors"] = state.StaticCheckData.ErrorTuple()
			errorRecord["symbolic_check_errors"] = state.SymbolicData.ErrorTuple()

			if runResult == nil && state.StaticCheckData.ErrorTuple().Len() == 0 && len(state.SymbolicData.Errors()) == 0 {
				errorRecord["runtime_error"] = core.NewError(err, core.Nil)
			}
		}
	} else if errors.As(err, &permissionError) {
		errorRecord["permission_error"] = core.NewError(permissionError, core.Nil)
	}

	return runResult, state, core.NewRecordFromMap(errorRecord), err
}
