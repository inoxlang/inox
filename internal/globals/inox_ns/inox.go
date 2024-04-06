package inox_ns

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"

	mod "github.com/inoxlang/inox/internal/mod"
)

const (
	NAMESPACE_NAME = "inox"
)

var (
	SYMB_PREPARATION_ERRORS_RECORD = symbolic.NewInexactRecord(map[string]symbolic.Serializable{
		"parsing-errors":        symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"static-check-errors":   symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"symbolic-check-errors": symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"permission-error":      symbolic.AsSerializableChecked(symbolic.NewMultivalue(symbolic.ANY_ERR, symbolic.Nil)),
	}, nil)
	SYMB_RUN_ERRORS_RECORD = symbolic.NewInexactRecord(map[string]symbolic.Serializable{
		"parsing-errors":        symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"static-check-errors":   symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"symbolic-check-errors": symbolic.NewTupleOf(symbolic.NewError(symbolic.SOURCE_POSITION_RECORD)),
		"permission-error":      symbolic.AsSerializableChecked(symbolic.NewMultivalue(symbolic.ANY_ERR, symbolic.Nil)),
		"runtime_error":         symbolic.AsSerializableChecked(symbolic.NewMultivalue(symbolic.ANY_ERR, symbolic.Nil)),
	}, nil)
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
		_print_source, func(ctx *symbolic.Context, n *symbolic.AstNode) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
		_prepare_local_script, func(ctx *symbolic.Context, p *symbolic.Path) (*symbolic.Module, *symbolic.GlobalState, *symbolic.Record, *symbolic.Error) {
			return symbolic.ANY_MODULE, symbolic.ANY_GLOBAL_STATE, SYMB_PREPARATION_ERRORS_RECORD, nil
		},
		_run_local_script, func(ctx *symbolic.Context, p *symbolic.Path, config *symbolic.Object) (symbolic.Value, *symbolic.GlobalState, *symbolic.Record, *symbolic.Error) {
			return symbolic.ANY, symbolic.ANY_GLOBAL_STATE, SYMB_PREPARATION_ERRORS_RECORD, nil
		},
	})
}

func NewInoxNamespace() *core.Namespace {
	return core.NewNamespace(NAMESPACE_NAME, map[string]core.Value{
		"parse_chunk":            core.WrapGoFunction(_parse_chunk),
		"parse_expr":             core.WrapGoFunction(_parse_expr),
		"parse_local_script":     core.WrapGoFunction(_parse_local_script),
		"print_source":           core.WrapGoFunction(_print_source),
		"parse_in_memory_module": core.WrapGoFunction(_parse_in_memory_module),
		"prepare_local_script":   core.WrapGoFunction(_prepare_local_script),
		"run_local_script":       core.WrapGoFunction(_run_local_script),
	})
}

func _parse_chunk(ctx *core.Context, s core.String) (node core.AstNode, e error) {
	defer func() {
		err, ok := recover().(error)
		if ok {
			e = err
		}
	}()

	chunk, err := parse.ParseChunk(string(s), "")
	return core.AstNode{Node: chunk}, err
}

func _parse_expr(ctx *core.Context, s core.String) (n core.AstNode, err error) {
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

	mod, err := core.ParseLocalModule(absPath, core.ModuleParsingConfig{
		Context: ctx,
	})

	return mod, err
}

func _parse_in_memory_module(ctx *core.Context, name core.String, code core.String) (*core.Module, error) {
	mod, err := core.ParseInMemoryModule(code.UnderlyingString(), core.InMemoryModuleParsingConfig{
		Name:    string(name),
		Context: ctx,
	})

	return mod, err
}

func _prepare_local_script(ctx *core.Context, src core.Path) (*core.Module, *core.GlobalState, *core.Record, error) {
	state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                     string(src),
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		ParentContextRequired:     true,

		DefaultLimits: core.GetDefaultScriptLimits(),

		Out: ctx.MustGetClosestState().Out,
	})

	errorRecord := core.ValMap{
		"parsing-errors":        core.NewTuple(nil),
		"static-check-errors":   core.NewTuple(nil),
		"symbolic-check-errors": core.NewTuple(nil),
		"permission-error":      core.Nil,
	}

	if err != nil && state == nil && mod == nil {
		return nil, nil, core.NewRecordFromMap(errorRecord), err
	}

	errorRecord["parsing-errors"] = mod.ParsingErrorTuple()

	var permissionError *core.NotAllowedError

	if state != nil {
		if state.StaticCheckData != nil {
			errorRecord["static-check-errors"] = state.StaticCheckData.ErrorTuple()
			errorRecord["symbolic-check-errors"] = state.SymbolicData.ErrorTuple()
		}
	} else if errors.As(err, &permissionError) {
		errorRecord["permission-error"] = core.NewError(permissionError, core.Nil)
	}

	return mod, state, core.NewRecordFromMap(errorRecord), err
}

func _run_local_script(ctx *core.Context, src core.Path, config *core.Object) (core.Value, *core.GlobalState, *core.Record, error) {
	var out io.Writer = io.Discard

	if err := config.ForEachEntry(func(k string, v core.Serializable) error {

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

	runResult, state, mod, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
		Fpath:                     string(src),
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		ParentContextRequired:     true,

		Transpile: false, // ctx.MustGetClosestState().Bytecode != nil,

		Out: out,
	})

	var errorRecord = core.ValMap{
		"parsing-errors":        core.NewTuple(nil),
		"static-check-errors":   core.NewTuple(nil),
		"symbolic-check-errors": core.NewTuple(nil),
		"permission-error":      core.Nil,
		"runtime_error":         core.Nil,
	}

	if err != nil && state == nil && mod == nil {
		return nil, nil, core.NewRecordFromMap(errorRecord), err
	}

	errorRecord["parsing-errors"] = mod.ParsingErrorTuple()

	var permissionError *core.NotAllowedError

	if state != nil {
		if state.StaticCheckData != nil {
			errorRecord["static-check-errors"] = state.StaticCheckData.ErrorTuple()
			errorRecord["symbolic-check-errors"] = state.SymbolicData.ErrorTuple()

			if runResult == nil && state.StaticCheckData.ErrorTuple().Len() == 0 && len(state.SymbolicData.Errors()) == 0 {
				errorRecord["runtime_error"] = core.NewError(err, core.Nil)
			}
		}
	} else if errors.As(err, &permissionError) {
		errorRecord["permission-error"] = core.NewError(permissionError, core.Nil)
	}

	return runResult, state, core.NewRecordFromMap(errorRecord), err
}

// GetCheckData returns a map that can be safely marshaled to JSON, the data has the following structure:
//
//	{
//		parsingErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//		staticCheckErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//		symbolicCheckErrors: [ ..., {text: <string>, location: <parse.SourcePosition>}, ... ]
//	}
func GetCheckData(fpath string, compilationCtx *core.Context, out io.Writer) map[string]any {
	state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                     fpath,
		Args:                      nil,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		DefaultLimits: []core.Limit{
			core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
		},

		Out: out,
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

		fmt.Fprintln(os.Stderr, len(mod.Errors), len(mod.Errors))
		data["parsingErrors"] = utils.MapSlice(mod.Errors, func(err inoxmod.Error) any {
			i++
			return map[string]any{
				"text":     err.BaseError.Error(),
				"location": err.Position.String(),
			}
		})
	}

	if state != nil && state.StaticCheckData != nil {
		i := -1
		data["staticCheckErrors"] = utils.MapSlice(state.StaticCheckData.Errors(), func(err *staticcheck.Error) any {
			i++
			return map[string]any{
				"text":     err.Message,
				"location": err.Location[0].String(),
			}
		})
		i = -1

		data["symbolicCheckErrors"] = utils.MapSlice(state.SymbolicData.Errors(), func(err symbolic.SymbolicEvaluationError) any {
			i++
			return map[string]any{
				"text":     err.Message,
				"location": err.Location[0].String(),
			}
		})
	}

	return data
}

func _print_source(ctx *core.Context, arg core.AstNode) core.String {
	buf := bytes.Buffer{}
	_, err := parse.Print(arg.Node, arg.Chunk().Node, &buf, parse.PrintConfig{KeepTrailingSpace: true})
	if err != nil {
		panic(err)
	}
	return core.String(buf.String())
}
