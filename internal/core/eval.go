package core

import (
	"fmt"
	"io"
	"io/fs"
	"slices"
	"time"

	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/parse"
)

const (
	STREAM_ITERATION_WAIT_TIMEOUT = 5 * time.Millisecond
	DEFAULT_MIN_STREAM_CHUNK_SIZE = 2
	DEFAULT_MAX_STREAM_CHUNK_SIZE = 10
)

var (
	IMPLICITLY_REMOVED_ROUTINE_PERMS = []Permission{
		LThreadPermission{permkind.Create},
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
	case PruneWalk:
		return "PruneWalk"
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
	CompilationContext   *Context
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
func resolvePattern(n parse.Node, state *GlobalState) (Value, error) {
	switch node := n.(type) {
	case *parse.PatternIdentifierLiteral:
		//should we return an error if not present
		pattern := state.Ctx.ResolveNamedPattern(node.Name)
		if pattern == nil {
			return nil, ErrNonExistingNamedPattern
		}
		return pattern, nil
	case *parse.PatternNamespaceIdentifierLiteral:
		namespace := state.Ctx.ResolvePatternNamespace(node.Name)
		if namespace == nil {
			return nil, fmt.Errorf("pattern namespace %s is not defined", node.Name)
		}
		return namespace, nil
	case *parse.PatternNamespaceMemberExpression:
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
func EvalSimpleValueLiteral(n parse.SimpleValueLiteral, global *GlobalState) (Serializable, error) {
	switch node := n.(type) {
	case *parse.UnambiguousIdentifierLiteral:
		return Identifier(node.Name), nil
	case *parse.PropertyNameLiteral:
		return PropertyName(node.Name), nil
	case *parse.LongValuePathLiteral:
		var segments []ValuePathSegment
		for _, segmentNode := range node.Segments {
			segment, err := EvalSimpleValueLiteral(segmentNode, global)
			if err != nil {
				return nil, err
			}
			segments = append(segments, segment.(ValuePathSegment))
		}
		return NewLongValuePath(segments), nil
	case *parse.DoubleQuotedStringLiteral:
		return String(node.Value), nil
	case *parse.UnquotedStringLiteral:
		return String(node.Value), nil
	case *parse.MultilineStringLiteral:
		return String(node.Value), nil
	case *parse.BooleanLiteral:
		return Bool(node.Value), nil
	case *parse.AbsolutePathLiteral:
		return Path(node.Value), nil
	case *parse.RelativePathLiteral:
		return Path(node.Value), nil
	case *parse.AbsolutePathPatternLiteral:
		return PathPattern(node.Value), nil
	case *parse.RelativePathPatternLiteral:
		return PathPattern(node.Value), nil
	case *parse.URLLiteral:
		return URL(node.Value), nil
	case *parse.URLPatternLiteral:
		return URLPattern(node.Value), nil
	case *parse.SchemeLiteral:
		return Scheme(node.Name), nil
	case *parse.HostLiteral:
		return Host(node.Value), nil
	case *parse.HostPatternLiteral:
		return HostPattern(node.Value), nil
	case *parse.RuneLiteral:
		return Rune(node.Value), nil
	case *parse.IntLiteral:
		return Int(node.Value), nil
	case *parse.FloatLiteral:
		return Float(node.Value), nil
	case *parse.PortLiteral:
		scheme := Scheme(NO_SCHEME_SCHEME)
		if node.SchemeName != "" {
			scheme = Scheme(node.SchemeName)
		}
		return Port{
			Number: node.PortNumber,
			Scheme: scheme,
		}, nil
	case *parse.QuantityLiteral:
		return evalQuantity(node.Values, node.Units)
	case *parse.YearLiteral:
		return Year(node.Value), nil
	case *parse.DateLiteral:
		return Date(node.Value), nil
	case *parse.DateTimeLiteral:
		return DateTime(node.Value), nil
	case *parse.RateLiteral:
		q, err := evalQuantity(node.Values, node.Units)
		if err != nil {
			return nil, err
		}
		return evalRate(q, node.DivUnit)
	case *parse.NamedSegmentPathPatternLiteral:
		return &NamedSegmentPathPattern{node: node}, nil
	case *parse.RegularExpressionLiteral:
		return NewRegexPattern(node.Value), nil
	case *parse.PathSlice:
		return String(node.Value), nil
	case *parse.PathPatternSlice:
		return String(node.Value), nil
	case *parse.URLQueryParameterValueSlice:
		return String(node.Value), nil
	case *parse.FlagLiteral:
		return Option{Name: node.Name, Value: Bool(true)}, nil
	case *parse.ByteSliceLiteral:
		return NewMutableByteSlice(slices.Clone(node.Value), ""), nil
	case *parse.NilLiteral:
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
