package core

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	IMPLICIT_KEY_LEN_KEY = "_len_"

	STREAM_ITERATION_WAIT_TIMEOUT = 5 * time.Millisecond
	DEFAULT_MIN_STREAM_CHUNK_SIZE = 2
	DEFAULT_MAX_STREAM_CHUNK_SIZE = 10
)

var (
	EMPTY_INTERFACE_TYPE = reflect.TypeOf((*interface{})(nil)).Elem()
	CTX_PTR_TYPE         = reflect.TypeOf(&Context{})

	VALUE_TYPE = reflect.TypeOf((*Value)(nil)).Elem()
	SERIALIZABLE_TYPE = reflect.TypeOf((*Serializable)(nil)).Elem()


	ERROR_INTERFACE_TYPE                 = reflect.TypeOf((*error)(nil)).Elem()
	READABLE_INTERFACE_TYPE              = reflect.TypeOf((*Readable)(nil)).Elem()
	RESOURCE_NAME_INTERFACE_TYPE         = reflect.TypeOf((*ResourceName)(nil)).Elem()
	STRLIKE_INTERFACE_TYPE               = reflect.TypeOf((*StringLike)(nil)).Elem()
	PATTERN_INTERFACE_TYPE               = reflect.TypeOf((*Pattern)(nil)).Elem()
	ITERABLE_INTERFACE_TYPE              = reflect.TypeOf((*Iterable)(nil)).Elem()
	SERIALIZABLE_ITERABLE_INTERFACE_TYPE = reflect.TypeOf((*SerializableIterable)(nil)).Elem()
	INDEXABLE_INTERFACE_TYPE             = reflect.TypeOf((*Indexable)(nil)).Elem()
	VALUE_RECEIVER_INTERFACE_TYPE        = reflect.TypeOf((*MessageReceiver)(nil)).Elem()
	EVENT_SOURCE_INTERFACE_TYPE          = reflect.TypeOf((*EventSource)(nil)).Elem()

	NIL_TYPE                = reflect.TypeOf(Nil)
	BYTE_SLICE_TYPE         = reflect.TypeOf(&ByteSlice{})
	RUNE_SLICE_TYPE         = reflect.TypeOf(&RuneSlice{})
	FILE_INFO_TYPE          = reflect.TypeOf(FileInfo{})
	RUNE_TYPE               = reflect.TypeOf(Rune('a'))
	BYTE_TYPE               = reflect.TypeOf(Byte('a'))
	REGULAR_STR_TYPE        = reflect.TypeOf("")
	STR_TYPE                = reflect.TypeOf(Str(""))
	STR_LIKE_INTERFACE_TYPE = reflect.TypeOf((*StringLike)(nil)).Elem()
	CHECKED_STR_TYPE        = reflect.TypeOf(CheckedString{})
	BOOL_TYPE               = reflect.TypeOf(Bool(true))
	INT_TYPE                = reflect.TypeOf(Int(1))
	FLOAT64_TYPE            = reflect.TypeOf(Float(0))
	OBJECT_TYPE             = reflect.TypeOf(&Object{})
	RECORD_TYPE             = reflect.TypeOf(&Record{})
	TUPLE_TYPE              = reflect.TypeOf(&Tuple{})
	LIST_PTR_TYPE           = reflect.TypeOf(&List{})
	DICT_TYPE               = reflect.TypeOf(&Dictionary{})
	KEYLIST_TYPE            = reflect.TypeOf(KeyList{})
	NODE_TYPE               = reflect.TypeOf(AstNode{})
	MODULE_TYPE             = reflect.TypeOf(&Module{})
	OPTION_TYPE             = reflect.TypeOf(Option{})
	IDENTIFIER_TYPE         = reflect.TypeOf(Identifier("a"))
	PROPNAME_TYPE           = reflect.TypeOf(PropertyName("a"))
	PATH_TYPE               = reflect.TypeOf(Path("/"))
	PATH_PATT_TYPE          = reflect.TypeOf(PathPattern("/"))
	URL_TYPE                = reflect.TypeOf(URL(""))
	SCHEME_TYPE             = reflect.TypeOf(Scheme(""))
	HOST_TYPE               = reflect.TypeOf(Host(""))
	HOST_PATT_TYPE          = reflect.TypeOf(HostPattern(""))
	EMAIL_ADDR_TYPE         = reflect.TypeOf(EmailAddress(""))
	URL_PATT_TYPE           = reflect.TypeOf(URLPattern(""))
	FILE_MODE_TYPE          = reflect.TypeOf(FileMode(0))
	DATE_TYPE               = reflect.TypeOf(Date{})
	EVENT_TYPE              = reflect.TypeOf((*Event)(nil))
	MUTATION_TYPE           = reflect.TypeOf(Mutation{})
	MSG_TYPE                = reflect.TypeOf(Message{})
	ERROR_TYPE              = reflect.TypeOf(Error{})
	INT_RANGE_TYPE          = reflect.TypeOf(IntRange{})
	VALUE_HISTORY_TYPE      = reflect.TypeOf(&ValueHistory{})
	SYSGRAPH_TYPE           = reflect.TypeOf(&SystemGraph{})
	SYSGRAPH_NODE_TYPE      = reflect.TypeOf(&SystemGraphNode{})
	SYSGRAPH_EDGE_TYPE      = reflect.TypeOf(SystemGraphEdge{})
	SECRET_TYPE             = reflect.TypeOf((*Secret)(nil))
)

var IMPLICITLY_REMOVED_ROUTINE_PERMS = []Permission{
	RoutinePermission{permkind.Create},
}

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
	OptimizeBytecode     bool
	CompilationContext   *Context
}

// EvalVM compiles the passed module (in module source) and evaluates the bytecode with the passed global state.
func EvalVM(mod *Module, state *GlobalState, config BytecodeEvaluationConfig) (Value, error) {
	compilationTracer := io.Writer(nil)
	if config.ShowCompilationTrace {
		compilationTracer = config.Tracer
	}

	err := Compile(CompilationInput{
		Mod:             mod,
		Globals:         state.Globals.permanent,
		SymbolicData:    state.SymbolicData.SymbolicData,
		StaticCheckData: state.StaticCheckData,
		TraceWriter:     compilationTracer,
		Context:         config.CompilationContext,
	})
	if err != nil {
		return nil, err
	}
	bytecode := mod.Bytecode

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

func Append(ctx *Context, list *List, args ...Serializable) *List {
	list.append(ctx, args...)
	return list
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
		"name":          Str(d.Name()),
		"path":          pth,
		"is-dir":        Bool(d.IsDir()),
		"is-regular":    Bool(d.Type().IsRegular()),
		"is-walk-start": Bool(isWalkStart),
	})
}

func CreatePatternNamespace(init Value) (*PatternNamespace, error) {
	var entries = map[string]Serializable{}
	switch r := init.(type) {
	case *Object:
		entries = r.EntryMap()
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

// evalSimpleValueLiteral evalutes a SimpleValueLiteral node (except IdentifierLiteral because it is ambiguous)
func evalSimpleValueLiteral(n parse.SimpleValueLiteral, global *GlobalState) (Serializable, error) {
	switch node := n.(type) {
	case *parse.UnambiguousIdentifierLiteral:
		return Identifier(node.Name), nil
	case *parse.PropertyNameLiteral:
		return PropertyName(node.Name), nil
	case *parse.QuotedStringLiteral:
		return Str(node.Value), nil
	case *parse.UnquotedStringLiteral:
		return Str(node.Value), nil
	case *parse.MultilineStringLiteral:
		return Str(node.Value), nil
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
	case *parse.EmailAddressLiteral:
		return EmailAddress(node.Value), nil
	case *parse.AtHostLiteral:
		res := global.Ctx.ResolveHostAlias(node.Value[1:])
		if res == "" {
			return nil, fmt.Errorf("host alias '%s' is not defined", node.Value)
		}
		return res, nil
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
	case *parse.DateLiteral:
		return Date(node.Value), nil
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
		return Str(node.Value), nil
	case *parse.PathPatternSlice:
		return Str(node.Value), nil
	case *parse.URLQueryParameterValueSlice:
		return Str(node.Value), nil
	case *parse.FlagLiteral:
		return Option{Name: node.Name, Value: Bool(true)}, nil
	case *parse.ByteSliceLiteral:
		return &ByteSlice{Bytes: utils.CopySlice(node.Value), IsDataMutable: true}, nil
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
			return WrapUnderylingList(&IntList{Elements: integers})
		}
	}

	//TODO: set constraint

	return WrapUnderylingList(&ValueList{elements: values})
}

func concatValues(ctx *Context, values []Value) (Value, error) {
	if len(values) == 0 {
		return nil, errors.New("cannot create concatenation with no elements")
	}
	switch values[0].(type) {
	case BytesLike:
		//TODO: concatenate small sequences together

		elements := make([]BytesLike, len(values))
		totalLen := 0

		for i, elem := range values {
			if bytesLike, ok := elem.(BytesLike); !ok {
				return nil, fmt.Errorf("bytes concatenation: invalid element of type %T", elem)
			} else {
				if bytesLike.Mutable() {
					b := utils.CopySlice(bytesLike.GetOrBuildBytes().Bytes) // TODO: use Copy On Write
					elements[i] = NewByteSlice(b, false, "")
				} else {
					elements[i] = bytesLike
				}
				totalLen += bytesLike.Len()
			}
		}

		if len(elements) == 1 {
			return elements[0], nil
		}

		return &BytesConcatenation{
			elements: elements,
			totalLen: totalLen,
		}, nil
	case StringLike:
		//TODO: concatenate small strings together

		elements := make([]StringLike, len(values))
		totalLen := 0

		for i, elem := range values {
			if strLike, ok := elem.(StringLike); !ok {
				return nil, fmt.Errorf("string concatenation: invalid element of type %T", elem)
			} else {
				elements[i] = strLike
				totalLen += strLike.Len()
			}
		}
		if len(elements) == 1 {
			return elements[0], nil
		}

		return &StringConcatenation{
			elements: elements,
			totalLen: totalLen,
		}, nil
	case *Tuple:
		if len(values) == 1 {
			return values[0].(*Tuple), nil
		}

		var tupleElements []Serializable

		for _, elem := range values {
			if tuple, ok := elem.(*Tuple); !ok {
				return nil, fmt.Errorf("tuple concatenation: invalid element of type %T", elem)
			} else {
				tupleElements = append(tupleElements, tuple.elements...)
			}
		}

		return NewTuple(tupleElements), nil
	default:
		return nil, errors.New("only string, bytes & tuple concatenations are supported for now")
	}
}

func toPattern(val Value) Pattern {
	if patt, ok := val.(Pattern); ok {
		return patt
	}
	return NewMostAdaptedExactPattern(val.(Serializable))
}
