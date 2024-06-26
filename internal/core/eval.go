package core

import (
	"fmt"
	"io"
	"io/fs"
	"slices"
	"time"

	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/permbase"
)

const (
	STREAM_ITERATION_WAIT_TIMEOUT = 5 * time.Millisecond
	DEFAULT_MIN_STREAM_CHUNK_SIZE = 2
	DEFAULT_MAX_STREAM_CHUNK_SIZE = 10
)

var (
	IMPLICITLY_REMOVED_ROUTINE_PERMS = []Permission{
		LThreadPermission{permbase.Create},
	}
	DEFAULT_SWITCH_MATCH_EXPR_RESULT = Nil
)

func (change IterationChange) String() string {
	switch change {
	case NoIterationChange:
		return "NoIterationChange"
	case BreakIteration:
		return "BreakIteration"
	case ContinueIteration:
		return "ContinueIteration"
	default:
		return "InvalidIterationChange"
	}
}

type IProps interface {
	Prop(ctx *Context, name string) Value
	PropertyNames(*Context) []string
	SetProp(ctx *Context, name string, value Value) error
}

type BytecodeEvaluationConfig struct {
	Tracer               io.Writer
	ShowCompilationTrace bool
	OptimizeBytecode     bool
	CompilationContext   *Context
}

// EvalVM compiles the passed module (in module source) and evaluates the bytecode with the passed global state.
func EvalVM(mod *Module, state *GlobalState, config BytecodeEvaluationConfig) (Value, error) {
	compilationTracer := io.Writer(nil)
	if config.ShowCompilationTrace {
		compilationTracer = config.Tracer
	}

	bytecode, err := Compile(CompilationInput{
		Mod:             mod,
		Globals:         state.Globals.permanent,
		SymbolicData:    state.SymbolicData.Data,
		StaticCheckData: state.StaticCheckData,
		TraceWriter:     compilationTracer,
		Context:         config.CompilationContext,
		//TODO: IsTestingEnabled:       state.TestingState.IsTestingEnabled,
		//TODO: IsImportTestingEnabled: state.TestingState.IsImportTestingEnabled,
	})
	if err != nil {
		return nil, err
	}
	state.Bytecode = bytecode

	if config.OptimizeBytecode {
		optimizeBytecode(bytecode, compilationTracer)
	}

	config.Tracer.Write([]byte(bytecode.Format(config.CompilationContext, "")))

	vm, err := NewVM(VMConfig{
		Bytecode: bytecode,
		State:    state,
	})
	if err != nil {
		return nil, err
	}
	return vm.Run()
}

func EvalBytecode(bytecode *Bytecode, state *GlobalState, self Value) (Value, error) {
	vm, err := NewVM(VMConfig{
		Bytecode: bytecode,
		State:    state,
		Self:     self,
	})
	if err != nil {
		return nil, err
	}
	return vm.Run()
}

func CreateDirEntry(path, walkedDirPath string, addDotSlashPrefix bool, d fs.DirEntry) *Object {
	isWalkStart := path == walkedDirPath

	var pth Path

	if isWalkStart || !addDotSlashPrefix || len(path) >= 2 && path[0] == '.' && path[1] == '/' {
		pth = Path(path)
	} else {
		pth = Path("./" + path)
	}

	if d.IsDir() && pth[len(pth)-1] != '/' {
		pth += "/"
	}

	return objFrom(ValMap{
		"name":          String(d.Name()),
		"path":          pth,
		"is-dir":        Bool(d.IsDir()),
		"is-regular":    Bool(d.Type().IsRegular()),
		"is-walk-start": Bool(isWalkStart),
	})
}

func CreatePatternNamespace(ctx *Context, init Value) (*PatternNamespace, error) {
	var entries = map[string]Serializable{}
	switch r := init.(type) {
	case *Object:
		entries = r.EntryMap(ctx)
	case *Record:
		entries = r.EntryMap()
	default:
		return nil, fmt.Errorf("cannot initialized pattern namespace with value of type %T", r)
	}

	namespace := &PatternNamespace{
		Patterns: make(map[string]Pattern),
	}

	for k, v := range entries {
		if patt, ok := v.(Pattern); ok {
			namespace.Patterns[k] = patt
		} else {
			namespace.Patterns[k] = NewMostAdaptedExactPattern(v)
		}
	}

	return namespace, nil
}

// resolve pattern evaluates a pattern identifier liferal | a pattern namespace identifier | a pattern namespace's member
func resolvePattern(n ast.Node, state *GlobalState) (Value, error) {
	switch node := n.(type) {
	case *ast.PatternIdentifierLiteral:
		//should we return an error if not present
		pattern := state.Ctx.ResolveNamedPattern(node.Name)
		if pattern == nil {
			return nil, ErrNonExistingNamedPattern
		}
		return pattern, nil
	case *ast.PatternNamespaceIdentifierLiteral:
		namespace := state.Ctx.ResolvePatternNamespace(node.Name)
		if namespace == nil {
			return nil, fmt.Errorf("pattern namespace %s is not defined", node.Name)
		}
		return namespace, nil
	case *ast.PatternNamespaceMemberExpression:
		namespace, err := resolvePattern(node.Namespace, state)
		if err != nil {
			return nil, err
		}
		patt, ok := namespace.(*PatternNamespace).Patterns[node.MemberName.Name]
		if !ok {
			return nil, fmt.Errorf("pattern namespace %s has not a pattern named %s", node.Namespace.Name, node.MemberName.Name)
		}
		return patt, nil
	default:
		return nil, fmt.Errorf("invalid node of type %T", n)
	}
}

// EvalSimpleValueLiteral evalutes a SimpleValueLiteral node (except IdentifierLiteral because it is ambiguous)
func EvalSimpleValueLiteral(n ast.SimpleValueLiteral, global *GlobalState) (Serializable, error) {
	switch node := n.(type) {
	case *ast.UnambiguousIdentifierLiteral:
		return Identifier(node.Name), nil
	case *ast.PropertyNameLiteral:
		return PropertyName(node.Name), nil
	case *ast.LongValuePathLiteral:
		var segments []ValuePathSegment
		for _, segmentNode := range node.Segments {
			segment, err := EvalSimpleValueLiteral(segmentNode, global)
			if err != nil {
				return nil, err
			}
			segments = append(segments, segment.(ValuePathSegment))
		}
		return NewLongValuePath(segments), nil
	case *ast.DoubleQuotedStringLiteral:
		return String(node.Value), nil
	case *ast.UnquotedStringLiteral:
		return String(node.Value), nil
	case *ast.MultilineStringLiteral:
		return String(node.Value), nil
	case *ast.BooleanLiteral:
		return Bool(node.Value), nil
	case *ast.AbsolutePathLiteral:
		return Path(node.Value), nil
	case *ast.RelativePathLiteral:
		return Path(node.Value), nil
	case *ast.AbsolutePathPatternLiteral:
		return PathPattern(node.Value), nil
	case *ast.RelativePathPatternLiteral:
		return PathPattern(node.Value), nil
	case *ast.URLLiteral:
		return URL(node.Value), nil
	case *ast.URLPatternLiteral:
		return URLPattern(node.Value), nil
	case *ast.SchemeLiteral:
		return Scheme(node.Name), nil
	case *ast.HostLiteral:
		return Host(node.Value), nil
	case *ast.HostPatternLiteral:
		return HostPattern(node.Value), nil
	case *ast.RuneLiteral:
		return Rune(node.Value), nil
	case *ast.IntLiteral:
		return Int(node.Value), nil
	case *ast.FloatLiteral:
		return Float(node.Value), nil
	case *ast.PortLiteral:
		scheme := Scheme(NO_SCHEME_SCHEME)
		if node.SchemeName != "" {
			scheme = Scheme(node.SchemeName)
		}
		return Port{
			Number: node.PortNumber,
			Scheme: scheme,
		}, nil
	case *ast.QuantityLiteral:
		return evalQuantity(node.Values, node.Units)
	case *ast.YearLiteral:
		return Year(node.Value), nil
	case *ast.DateLiteral:
		return Date(node.Value), nil
	case *ast.DateTimeLiteral:
		return DateTime(node.Value), nil
	case *ast.RateLiteral:
		q, err := evalQuantity(node.Values, node.Units)
		if err != nil {
			return nil, err
		}
		return evalRate(q, node.DivUnit)
	case *ast.NamedSegmentPathPatternLiteral:
		return NewNamedSegmentPathPattern(node), nil
	case *ast.RegularExpressionLiteral:
		return NewRegexPattern(node.Value), nil
	case *ast.PathSlice:
		return String(node.Value), nil
	case *ast.PathPatternSlice:
		return String(node.Value), nil
	case *ast.URLQueryParameterValueSlice:
		return String(node.Value), nil
	case *ast.FlagLiteral:
		return Option{Name: node.Name, Value: Bool(true)}, nil
	case *ast.ByteSliceLiteral:
		return NewMutableByteSlice(slices.Clone(node.Value), ""), nil
	case *ast.NilLiteral:
		return Nil, nil
	default:
		panic(fmt.Errorf("node of type %T is not a simple value literal", node))
	}
}

func createBestSuitedList(ctx *Context, values []Serializable, elemType Pattern) *List {
	switch t := elemType.(type) {
	case *TypePattern:
		if t.Type == INT_TYPE {
			integers := make([]Int, len(values))
			for i, e := range values {
				integers[i] = e.(Int)
			}
			return WrapUnderlyingList(&IntList{elements: integers})
		}
		if t.Type == FLOAT64_TYPE {
			integers := make([]Float, len(values))
			for i, e := range values {
				integers[i] = e.(Float)
			}
			return WrapUnderlyingList(&FloatList{elements: integers})
		}
	}

	//TODO: set constraint

	return WrapUnderlyingList(&ValueList{elements: values})
}

func toPattern(val Value) Pattern {
	if patt, ok := val.(Pattern); ok {
		return patt
	}
	return NewMostAdaptedExactPattern(val.(Serializable))
}
