package core

import (
	"fmt"
	"io"
	"io/fs"
	"reflect"
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
	EMPTY_INTERFACE_TYPE = reflect.TypeOf((*interface{})(nil)).Elem()
	CTX_PTR_TYPE         = reflect.TypeOf((*Context)(nil))

	VALUE_TYPE        = reflect.TypeOf((*Value)(nil)).Elem()
	SERIALIZABLE_TYPE = reflect.TypeOf((*Serializable)(nil)).Elem()

	ERROR_INTERFACE_TYPE                 = reflect.TypeOf((*error)(nil)).Elem()
	READABLE_INTERFACE_TYPE              = reflect.TypeOf((*Readable)(nil)).Elem()
	RESOURCE_NAME_INTERFACE_TYPE         = reflect.TypeOf((*ResourceName)(nil)).Elem()
	PATTERN_INTERFACE_TYPE               = reflect.TypeOf((*Pattern)(nil)).Elem()
	ITERABLE_INTERFACE_TYPE              = reflect.TypeOf((*Iterable)(nil)).Elem()
	SERIALIZABLE_ITERABLE_INTERFACE_TYPE = reflect.TypeOf((*SerializableIterable)(nil)).Elem()
	INDEXABLE_INTERFACE_TYPE             = reflect.TypeOf((*Indexable)(nil)).Elem()
	VALUE_RECEIVER_INTERFACE_TYPE        = reflect.TypeOf((*MessageReceiver)(nil)).Elem()
	EVENT_SOURCE_INTERFACE_TYPE          = reflect.TypeOf((*EventSource)(nil)).Elem()

	NIL_TYPE                   = reflect.TypeOf(Nil)
	BYTE_SLICE_TYPE            = reflect.TypeOf((*ByteSlice)(nil))
	RUNE_SLICE_TYPE            = reflect.TypeOf((*RuneSlice)(nil))
	FILE_INFO_TYPE             = reflect.TypeOf(FileInfo{})
	RUNE_TYPE                  = reflect.TypeOf(Rune('a'))
	BYTE_TYPE                  = reflect.TypeOf(Byte('a'))
	REGULAR_STR_TYPE           = reflect.TypeOf("")
	STRING_TYPE                = reflect.TypeOf(String(""))
	STR_LIKE_INTERFACE_TYPE    = reflect.TypeOf((*StringLike)(nil)).Elem()
	CHECKED_STR_TYPE           = reflect.TypeOf(CheckedString{})
	BOOL_TYPE                  = reflect.TypeOf(Bool(true))
	INT_TYPE                   = reflect.TypeOf(Int(1))
	PORT_TYPE                  = reflect.TypeOf(Port{})
	BYTERATE_TYPE              = reflect.TypeOf(ByteRate(1))
	FREQUENCY_TYPE             = reflect.TypeOf(Frequency(1))
	LINECOUNT_TYPE             = reflect.TypeOf(LineCount(1))
	RUNECOUNT_TYPE             = reflect.TypeOf(RuneCount(1))
	BYTECOUNT_TYPE             = reflect.TypeOf(ByteCount(1))
	DURATION_TYPE              = reflect.TypeOf(Duration(1))
	FLOAT64_TYPE               = reflect.TypeOf(Float(0))
	OBJECT_TYPE                = reflect.TypeOf((*Object)(nil))
	RECORD_TYPE                = reflect.TypeOf((*Record)(nil))
	TUPLE_TYPE                 = reflect.TypeOf((*Tuple)(nil))
	LIST_PTR_TYPE              = reflect.TypeOf((*List)(nil))
	DICT_TYPE                  = reflect.TypeOf((*Dictionary)(nil))
	KEYLIST_TYPE               = reflect.TypeOf(KeyList{})
	NODE_TYPE                  = reflect.TypeOf(AstNode{})
	MODULE_TYPE                = reflect.TypeOf((*Module)(nil))
	OPTION_TYPE                = reflect.TypeOf(Option{})
	IDENTIFIER_TYPE            = reflect.TypeOf(Identifier("a"))
	PROPNAME_TYPE              = reflect.TypeOf(PropertyName("a"))
	LONG_VALUE_PATH_TYPE       = reflect.TypeOf((*LongValuePath)(nil))
	VALUE_PATH_INTERFACE__TYPE = reflect.TypeOf((*ValuePath)(nil)).Elem()
	PATH_TYPE                  = reflect.TypeOf(Path("/"))
	PATH_PATT_TYPE             = reflect.TypeOf(PathPattern("/"))
	URL_TYPE                   = reflect.TypeOf(URL(""))
	SCHEME_TYPE                = reflect.TypeOf(Scheme(""))
	HOST_TYPE                  = reflect.TypeOf(Host(""))
	HOST_PATT_TYPE             = reflect.TypeOf(HostPattern(""))
	EMAIL_ADDR_TYPE            = reflect.TypeOf(EmailAddress(""))
	URL_PATT_TYPE              = reflect.TypeOf(URLPattern(""))
	FILE_MODE_TYPE             = reflect.TypeOf(FileMode(0))

	YEAR_TYPE     = reflect.TypeOf(Year{})
	DATE_TYPE     = reflect.TypeOf(Date{})
	DATETIME_TYPE = reflect.TypeOf(DateTime{})

	EVENT_TYPE                      = reflect.TypeOf((*Event)(nil))
	MUTATION_TYPE                   = reflect.TypeOf(Mutation{})
	MSG_TYPE                        = reflect.TypeOf(Message{})
	ERROR_TYPE                      = reflect.TypeOf(Error{})
	INT_RANGE_TYPE                  = reflect.TypeOf(IntRange{})
	FLOAT_RANGE_TYPE                = reflect.TypeOf(FloatRange{})
	RUNE_RANGE_TYPE                 = reflect.TypeOf(RuneRange{})
	VALUE_HISTORY_TYPE              = reflect.TypeOf((*ValueHistory)(nil))
	SYSGRAPH_TYPE                   = reflect.TypeOf((*SystemGraph)(nil))
	SYSGRAPH_NODE_TYPE              = reflect.TypeOf((*SystemGraphNode)(nil))
	SYSGRAPH_EDGE_TYPE              = reflect.TypeOf(SystemGraphEdge{})
	SECRET_TYPE                     = reflect.TypeOf((*Secret)(nil))
	READER_INTERFACE_TYPE           = reflect.TypeOf((*Reader)(nil))
	OBJECT_PATTERN_TYPE             = reflect.TypeOf((*ObjectPattern)(nil))
	LIST_PATTERN_TYPE               = reflect.TypeOf((*ListPattern)(nil))
	RECORD_PATTERN_TYPE             = reflect.TypeOf((*RecordPattern)(nil))
	TUPLE_PATTERN_TYPE              = reflect.TypeOf((*TuplePattern)(nil))
	ULID_TYPE                       = reflect.TypeOf(ULID{})
	UUIDv4_TYPE                     = reflect.TypeOf(UUIDv4{})
	ORDERED_PAIR_TYPE               = reflect.TypeOf((*OrderedPair)(nil))
	NAMED_SEGMENT_PATH_PATTERN_TYPE = reflect.TypeOf((*NamedSegmentPathPattern)(nil))
	TYPE_PATTERN_TYPE               = reflect.TypeOf((*TypePattern)(nil))
	EXACT_VALUE_PATTERN_TYPE        = reflect.TypeOf((*ExactValuePattern)(nil))
	EXACT_STRING_PATTERN_TYPE       = reflect.TypeOf((*ExactStringPattern)(nil))
	INT_RANGE_PATTERN_TYPE          = reflect.TypeOf(&IntRangePattern{})
	FLOAT_RANGE_PATTERN_TYPE        = reflect.TypeOf(&FloatRangePattern{})
	INT_RANGE_STRING_PATTERN_TYPE   = reflect.TypeOf(&IntRangeStringPattern{})
	FLOAT_RANGE_STRING_PATTERN_TYPE = reflect.TypeOf(&FloatRangeStringPattern{})
	SECRET_PATTERN_TYPE             = reflect.TypeOf(&SecretPattern{})
	REGEX_PATTERN_TYPE              = reflect.TypeOf(&RegexPattern{})
	EVENT_PATTERN_TYPE              = reflect.TypeOf((*EventPattern)(nil))
	MUTATION_PATTERN_TYPE           = reflect.TypeOf((*MutationPattern)(nil))

	DEV_API_TYPE =  reflect.TypeOf((*DevAPI)(nil)).Elem()
)

var IMPLICITLY_REMOVED_ROUTINE_PERMS = []Permission{
	LThreadPermission{permkind.Create},
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

	bytecode, err := Compile(CompilationInput{
		Mod:                    mod,
		Globals:                state.Globals.permanent,
		SymbolicData:           state.SymbolicData.Data,
		StaticCheckData:        state.StaticCheckData,
		TraceWriter:            compilationTracer,
		Context:                config.CompilationContext,
		IsTestingEnabled:       state.TestingState.IsTestingEnabled,
		IsImportTestingEnabled: state.TestingState.IsImportTestingEnabled,
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
