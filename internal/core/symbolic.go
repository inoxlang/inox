package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// this file contains the implementation of Value.ToSymbolicValue for core types and does some initialization.

var (
	// mapping Go function address -> symbolic Go function
	// The "address" is obtained by doing reflect.ValueOf(goFunc).Pointer().
	// Note that two closures of the same function have the same "address".
	symbolicGoFunctionMap = map[uintptr]*symbolic.GoFunction{}

	// mapping Go function address -> last mandatory param index.
	functionOptionalParamInfo = map[uintptr]optionalParamInfo{}

	functionOptionalParamInfoLock sync.Mutex //only required in tests

	// mapping symbolic Go function -> reflect.Value of the concrete Go Function.
	goFunctionMap = map[*symbolic.GoFunction]reflect.Value{}

	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}
)

type optionalParamInfo struct {
	lastMandatoryParamIndex int8
	optionalParams          []optionalParam
}

func init() {

	symbolic.SetExternalData(symbolic.ExternalData{
		CONSTRAINTS_KEY:                         CONSTRAINTS_KEY,
		VISIBILITY_KEY:                          VISIBILITY_KEY,
		MANIFEST_POSITIONAL_PARAM_NAME_FIELD:    "name",
		MANIFEST_POSITIONAL_PARAM_PATTERN_FIELD: "pattern",
		MANIFEST_PARAMS_SECTION_NAME:            inoxconsts.MANIFEST_PARAMS_SECTION_NAME,

		DEFAULT_PATTERN_NAMESPACES: func() map[string]*symbolic.PatternNamespace {
			result := make(map[string]*symbolic.PatternNamespace)
			for name, ns := range DEFAULT_PATTERN_NAMESPACES {
				symbolicNamespace, err := ns.ToSymbolicValue(nil, map[uintptr]symbolic.Value{})
				if err != nil {
					panic(err)
				}
				result[name] = symbolicNamespace.(*symbolic.PatternNamespace)
			}
			return result
		}(),

		ToSymbolicValue: func(concreteCtx symbolic.ConcreteContext, v any, wide bool) (symbolic.Value, error) {
			return ToSymbolicValue(concreteCtx.(*Context), v.(Value), wide)
		},
		GetConcretePatternMatchingSymbolicValue: func(concreteCtx symbolic.ConcreteContext, v symbolic.Value) (any, bool) {
			return getConcretePatternMatchingSymbolicValue(concreteCtx.(*Context), v)
		},
		GetQuantity: func(values []float64, units []string) (any, error) {
			return evalQuantity(values, units)
		},
		GetRate: func(values []float64, units []string, divUnit string) (any, error) {
			q, err := evalQuantity(values, units)
			if err != nil {
				return nil, err
			}
			return evalRate(q, divUnit)
		},
		// TreeWalkEvalEmptyState: func(node parse.Node) (any, error) {
		// 	state := NewTreeWalkStateWithGlobal(&GlobalState{})

		// 	return TreeWalkEval(node, state)
		// },
		// GetValueRepresentation: func(v any) string {
		// 	return GetRepresentation(v.(Value))
		// },
		IsReadable: func(v any) bool {
			_, ok := v.(Readable)
			return ok
		},
		IsWritable: func(v any) bool {
			_, ok := v.(Writer)
			return ok
		},
		PathMatch: func(path, pattern string) bool {
			return PathPattern(pattern).Test(nil, Path(path))
		},
		URLMatch: func(url, pattern string) bool {
			return URLPattern(pattern).Test(nil, URL(url))
		},
		HostMatch: func(host, pattern string) bool {
			return HostPattern(pattern).Test(nil, Host(host))
		},
		AppendPathSegmentToURL: func(u, segment string) string {
			parsed, err := url.Parse(u)
			if err != nil {
				panic(err)
			}
			return parsed.JoinPath(segment).String()
		},
		AppendPathSegmentToURLPattern: func(u, segment string) string {
			return appendPathSegmentToURLPattern(u, segment)
		},
		IsIndexKey: IsIndexKey,
		CheckDatabaseSchema: func(objectPattern any) error {
			return checkDatabaseSchema(objectPattern.(*ObjectPattern))
		},

		EstimatePermissionsFromListingNode: func(n *parse.ObjectLiteral) (any, error) {
			perms, err := estimatePermissionsFromListingNode(n)
			return perms, err
		},

		CreateConcreteContext: func(permissions any) symbolic.ConcreteContext {
			perms := permissions.([]Permission)

			return NewContext(ContextConfig{
				Permissions: perms,
			})
		},

		GetTopLevelEntitiesMigrationOperations: func(concreteCtx context.Context, current, next any) ([]symbolic.MigrationOp, error) {
			ctx := concreteCtx.(*Context)
			concreteMigrationOps, err := GetTopLevelEntitiesMigrationOperations(ctx, current.(*ObjectPattern), next.(*ObjectPattern))
			if err != nil {
				return nil, err
			}
			symbolicMigrationOps := make([]symbolic.MigrationOp, len(concreteMigrationOps))
			encountered := map[uintptr]symbolic.Value{}

			for i, op := range concreteMigrationOps {
				symbolicMigrationOps[i] = op.ToSymbolicValue(ctx, encountered)
			}

			return symbolicMigrationOps, nil
		},
		ConcreteValueFactories: symbolic.ConcreteValueFactories{
			CreateObjectPattern: func(inexact bool, concretePropertyPatterns map[string]any, optionalProperties map[string]struct{}) any {
				var entries []ObjectPatternEntry

				for k, v := range concretePropertyPatterns {
					_, isOptional := optionalProperties[k]
					entries = append(entries, ObjectPatternEntry{
						Name:       k,
						Pattern:    v.(Pattern),
						IsOptional: isOptional,
					})
				}

				return NewObjectPattern(inexact, entries)
			},
			CreateRecordPattern: func(inexact bool, concretePropertyPatterns map[string]any, optionalProperties map[string]struct{}) any {
				var entries []RecordPatternEntry

				for k, v := range concretePropertyPatterns {
					_, isOptional := optionalProperties[k]
					entries = append(entries, RecordPatternEntry{
						Name:       k,
						Pattern:    v.(Pattern),
						IsOptional: isOptional,
					})
				}

				return NewRecordPattern(inexact, entries)
			},

			CreateListPattern: func(generalElementPattern any, elementPatterns []any) any {
				if generalElementPattern != nil {
					return NewListPatternOf(generalElementPattern.(Pattern))
				}
				return NewListPattern(utils.MapSlice(elementPatterns, func(e any) Pattern {
					return e.(Pattern)
				}))
			},

			CreateTuplePattern: func(generalElementPattern any, elementPatterns []any) any {
				if generalElementPattern != nil {
					return NewTuplePatternOf(generalElementPattern.(Pattern))
				}
				return NewTuplePattern(utils.MapSlice(elementPatterns, func(e any) Pattern {
					return e.(Pattern)
				}))
			},

			CreateExactValuePattern: func(value any) any {
				return NewExactValuePattern(value.(Serializable))
			},

			CreateExactStringPattern: func(value any) any {
				return NewExactStringPattern(value.(String))
			},

			CreateRegexPattern: func(regex string) any {
				return NewRegexPattern(regex)
			},

			CreateSequenceStringPattern: func(concreteCtx symbolic.ConcreteContext, n *parse.ComplexStringPatternPiece) (any, error) {
				ctx := concreteCtx.(*Context)
				state := NewTreeWalkState(ctx)
				return TreeWalkEval(n, state)
			},

			CreateNil: func() any {
				return Nil
			},
			CreateBool: func(b bool) any {
				return Bool(b)
			},

			CreateFloat: func(f float64) any {
				return Float(f)
			},
			CreateInt: func(i int64) any {
				return Int(i)
			},

			CreateByteCount: func(c int64) any {
				return ByteCount(c)
			},
			CreateLineCount: func(c int64) any {
				return LineCount(c)
			},
			CreateRuneCount: func(c int64) any {
				return RuneCount(c)
			},
			CreateFrequency: func(r float64) any {
				return Frequency(r)
			},
			CreateByteRate: func(r int64) any {
				return ByteRate(r)
			},
			CreateDuration: func(d time.Duration) any {
				return Duration(d)
			},
			CreateYear: func(t time.Time) any {
				return Year(t)
			},
			CreateDate: func(t time.Time) any {
				return Date(t)
			},
			CreateDateTime: func(t time.Time) any {
				return DateTime(t)
			},
			CreateByte: func(b byte) any {
				return Byte(b)
			},
			CreateRune: func(r rune) any {
				return Rune(r)
			},
			CreateString: func(s string) any {
				return String(s)
			},
			CreateStringConcatenation: func(elements []any) any {
				var strLikes []StringLike
				for _, e := range elements {
					s := e.(StringLike)
					strLikes = append(strLikes, s)
				}
				return NewStringConcatenation(strLikes...)
			},
			CreatePath: func(s string) any {
				return Path(s)
			},
			CreateURL: func(s string) any {
				return URL(s)
			},
			CreateHost: func(s string) any {
				return Host(s)
			},
			CreateScheme: func(s string) any {
				return Scheme(s)
			},

			CreateIdentifier: func(s string) any {
				return Identifier(s)
			},
			CreatePropertyName: func(s string) any {
				return PropertyName(s)
			},
			CreateLongValuePath: func(segments ...any) any {
				pathSegments := make([]ValuePathSegment, len(segments))
				for i, segment := range segments {
					pathSegments[i] = segment.(ValuePathSegment)
				}
				return NewLongValuePath(pathSegments)
			},

			CreateByteSlice: func(bytes []byte) any {
				return NewByteSlice(slices.Clone(bytes), true, "")
			},
			CreateRuneSlice: func(runes []rune) any {
				return NewRuneSlice(slices.Clone(runes))
			},

			CreateObject: func(concreteProperties map[string]any) any {
				properties := map[string]Serializable{}
				for k, v := range concreteProperties {
					properties[k] = v.(Serializable)
				}
				return objFrom(properties)
			},
			CreateRecord: func(concreteProperties map[string]any) any {
				properties := map[string]Serializable{}
				for k, v := range concreteProperties {
					properties[k] = v.(Serializable)
				}
				return NewRecordFromMap(properties)
			},
			CreateList: func(elements []any) any {
				return NewWrappedValueList(utils.MapSlice(elements, ToSerializableAsserted)...)
			},
			CreateTuple: func(elements []any) any {
				return NewTuple(utils.MapSlice(elements, ToSerializableAsserted))
			},
			CreateOrderedPair: func(first, second any) any {
				return NewOrderedPair(first.(Serializable), second.(Serializable))
			},
			CreateKeyList: func(names []string) any {
				return KeyList(slices.Clone(names))
			},
			CreateDictionary: func(keys, values []any, ctx symbolic.ConcreteContext) any {
				context := ctx.(*Context)

				return NewDictionaryFromKeyValueLists(
					utils.MapSlice(keys, ToSerializableAsserted),
					utils.MapSlice(values, ToSerializableAsserted),
					context,
				)
			},

			CreatePathPattern: func(s string) any {
				return PathPattern(s)
			},
			CreateURLPattern: func(s string) any {
				return URLPattern(s)
			},
			CreateHostPattern: func(s string) any {
				return HostPattern(s)
			},

			CreateOption: func(name string, value any) any {
				return &Option{
					Name:  name,
					Value: value.(Value),
				}
			},
		},
	})

}

// RegisterSymbolicGoFunction registers the symbolic equivalent of fn, fn should not be a method or a closure.
// example: RegisterSymbolicGoFunction(func(ctx *Context){ }, func(ctx *symbolic.Context))
// This function also registers information about the concrete Go function.
func RegisterSymbolicGoFunction(fn any, symbolicFn any) {
	reflectVal := reflect.ValueOf(fn)
	reflectValType := reflectVal.Type()

	ptr := reflectVal.Pointer()
	_, ok := symbolicGoFunctionMap[ptr]
	if ok {
		panic(fmt.Errorf("symbolic equivalent of function %s already registered", runtime.FuncForPC(ptr).Name()))
	}

	goFunc, ok := symbolicFn.(*symbolic.GoFunction)
	if !ok {
		goFunc = symbolic.WrapGoFunction(symbolicFn)
	}

	if reflect.TypeOf(goFunc.GoFunc()).Kind() != reflect.Func {
		panic(fmt.Errorf("symbolic equivalent for function %s should be a function", runtime.FuncForPC(ptr).Name()))
	}

	symbolicGoFunctionMap[ptr] = goFunc
	goFunctionMap[goFunc] = reflectVal

	//register the index of the last mandatory parameter if the concrete Go function has optional parameters.
	{
		numIn := reflectValType.NumIn()
		if numIn > math.MaxInt8 {
			panic(errors.New("go function has too many parameters"))
		}
		if reflectValType.IsVariadic() {
			numIn--
		}

		var optionalParamInfo = optionalParamInfo{
			lastMandatoryParamIndex: -1,
		}

		for i := 0; i < numIn; i++ {
			paramType := reflectValType.In(i)
			if paramType.Implements(OPTIONAL_PARAM_TYPE) {
				if optionalParamInfo.lastMandatoryParamIndex == -1 {
					optionalParamInfo.lastMandatoryParamIndex = int8(i - 1)
				}
				optionalParam := reflect.New(paramType.Elem()).Interface().(optionalParam)
				optionalParamInfo.optionalParams = append(optionalParamInfo.optionalParams, optionalParam)
			}
		}

		if optionalParamInfo.lastMandatoryParamIndex != -1 {
			if testing.Testing() {
				functionOptionalParamInfoLock.Lock()
				defer functionOptionalParamInfoLock.Unlock()
			}
			functionOptionalParamInfo[ptr] = optionalParamInfo
		}
	}
}

// [<fn1>, <symbolic fn1>, <fn2>, <symbolic fn2>, ...]., See RegisterSymbolicGoFunction.
func RegisterSymbolicGoFunctions(entries []any) {
	if len(entries)%2 != 0 {
		panic(errors.New("provided slice should have an even length"))
	}
	for i := 0; i < len(entries); i += 2 {
		RegisterSymbolicGoFunction(entries[i], entries[i+1])
	}
}

func IsSymbolicEquivalentOfGoFunctionRegistered(fn any) bool {
	ptr := reflect.ValueOf(fn).Pointer()
	_, ok := symbolicGoFunctionMap[ptr]
	return ok
}

func GetConcreteGoFuncFromSymbolic(fn *symbolic.GoFunction) (reflect.Value, bool) {
	concreteFn, ok := goFunctionMap[fn]
	return concreteFn, ok
}

type SymbolicData struct {
	*symbolic.Data

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

func (d *SymbolicData) ErrorTuple() *Tuple {
	if d.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Serializable, len(d.Data.Errors()))
		for i, err := range d.Data.Errors() {
			data := createRecordFromSourcePositionStack(err.Location)
			errors[i] = NewError(err, data)
		}
		d.errorsProp = NewTuple(errors)
	}
	return d.errorsProp
}

func (d *SymbolicData) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *SymbolicData) Prop(ctx *Context, name string) Value {
	switch name {
	case "errors":
		return d.ErrorTuple()
	}

	method, ok := d.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, d))
	}
	return method
}

func (*SymbolicData) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*SymbolicData) PropertyNames(ctx *Context) []string {
	return SYMBOLIC_DATA_PROP_NAMES
}

func ToSymbolicValue(ctx *Context, v Value, wide bool) (symbolic.Value, error) {
	return _toSymbolicValue(ctx, v, wide, make(map[uintptr]symbolic.Value))
}

func GetStringifiedSymbolicValue(ctx *Context, v Value, wide bool) (string, error) {
	symbolicVal, err := ToSymbolicValue(ctx, v, wide)
	if err != nil {
		return "", err
	}
	return symbolic.Stringify(symbolicVal), nil
}

func (n NilT) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.Nil, nil
}

func (i Int) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewInt(int64(i)), nil
}

func (b Bool) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if b {
		return symbolic.TRUE, nil
	}
	return symbolic.FALSE, nil
}

func (f Float) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewFloat(float64(f)), nil
}

func (r Rune) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewRune(rune(r)), nil
}

func (s String) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewString(string(s)), nil
}

func (s CheckedString) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_CHECKED_STRING, nil
}

func (s *RuneSlice) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_RUNE_SLICE, nil
}

func (e Error) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	data, err := e.data.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewError(data), nil
}

func (i Identifier) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewIdentifier(i.UnderlyingString()), nil
}

func (p PropertyName) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewPropertyName(p.UnderlyingString()), nil
}

func (p *LongValuePath) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	var segments []symbolic.ValuePathSegment
	for _, segment := range *p {
		val, err := segment.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, err
		}
		segments = append(segments, val.(symbolic.ValuePathSegment))
	}
	return symbolic.NewLongValuePath(segments...), nil
}

func (p Path) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewPath(p.UnderlyingString()), nil
}

func (p PathPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewPathPattern(p.UnderlyingString()), nil
}

func (u URL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewUrl(u.UnderlyingString()), nil
}

func (p URLPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewUrlPattern(p.UnderlyingString()), nil
}

func (p HostPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewHostPattern(p.UnderlyingString()), nil
}

func (o Option) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	value, err := o.Value.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewOption(o.Name, value), nil
}

func (*Array) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_ARRAY, nil
}

func (l *List) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return l.underlyingList.ToSymbolicValue(ctx, encountered)
}

func (l *ValueList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewListOf(symbolic.ANY_SERIALIZABLE), nil
}

func (l *NumberList[N]) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	var sym symbolic.Serializable
	var n N
	switch any(n).(type) {
	case Int:
		sym = symbolic.ANY_INT
	case Float:
		sym = symbolic.ANY_FLOAT
	default:
		panic(ErrUnreachable)
	}
	return symbolic.NewListOf(sym), nil
}

func (l *BoolList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewListOf(symbolic.ANY_BOOL), nil
}

func (l *StringList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewListOf(symbolic.ANY_STR_LIKE), nil
}

func (l KeyList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	var keys = make([]string, len(l))
	copy(keys, l)
	return &symbolic.KeyList{Keys: keys}, nil
}

func (t Tuple) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	//TODO: support constraints

	symbolicElements := make([]symbolic.Serializable, len(t.elements))

	for i, e := range t.elements {
		symbolicElement, err := e.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, err
		}
		symbolicElements[i] = symbolicElement.(symbolic.Serializable)
	}

	return symbolic.NewTuple(symbolicElements...), nil
}

func (p *OrderedPair) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbPair := symbolic.NewUnitializedOrderedPair()
	encountered[ptr] = symbPair

	first, err := p[0].ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	second, err := p[1].ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	symbolic.InitializeOrderedPair(symbPair, first.(symbolic.Serializable), second.(symbolic.Serializable))
	return symbPair, nil
}

func (obj *Object) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(obj).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbolicObj := symbolic.NewUnitializedObject()
	encountered[ptr] = symbolicObj

	entries := map[string]symbolic.Serializable{}

	if obj.IsShared() {
		if ctx == nil {
			panic(errors.New("nil context"))
		}
		state := ctx.MustGetClosestState()
		obj._lock(state)
		defer obj._unlock(state)
	}

	for i, v := range obj.values {
		k := obj.keys[i]
		symbolicVal, err := _toSymbolicValue(ctx, v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = symbolicVal.(symbolic.Serializable)
	}

	symbolic.InitializeObject(symbolicObj, entries, nil, obj.IsShared())
	return symbolicObj, nil
}

func (rec *Record) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(rec).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	entries := make(map[string]symbolic.Serializable)
	symbolicRec := symbolic.NewBoundEntriesRecord(entries)
	encountered[ptr] = symbolicRec

	for i, v := range rec.values {
		k := rec.keys[i]

		symbolicVal, err := _toSymbolicValue(ctx, v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = symbolicVal.(symbolic.Serializable)
	}

	return symbolicRec, nil
}

func (dict *Dictionary) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(dict).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbolicDict := symbolic.NewUnitializedDictionary()
	encountered[ptr] = symbolicDict

	entries := make(map[string]symbolic.Serializable)
	keys := make(map[string]symbolic.Serializable)

	for keyRepresentation, v := range dict.entries {
		symbolicVal, err := _toSymbolicValue(ctx, v, false, encountered)
		if err != nil {
			return nil, err
		}

		key := dict.keys[keyRepresentation]
		symbolicKey, err := _toSymbolicValue(ctx, key, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[keyRepresentation] = symbolicVal.(symbolic.Serializable)
		keys[keyRepresentation] = symbolicKey.(symbolic.Serializable)
	}

	symbolic.InitializeDictionary(symbolicDict, entries, keys)
	return symbolicDict, nil
}

func (args *ModuleArgs) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(args).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	moduleParamsPattern, err := args.pattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert type of struct to symbolic: %w", err)
	}

	symbolicModuleArgs := symbolic.NewModuleArgs(moduleParamsPattern.(*symbolic.ModuleParamsPattern), nil)
	encountered[ptr] = symbolicModuleArgs

	return symbolicModuleArgs, nil
}

func (p *UnionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	unionPattern := &symbolic.UnionPattern{}
	encountered[ptr] = unionPattern

	var cases []symbolic.Pattern

	for _, case_ := range p.cases {
		symbolicVal, err := _toSymbolicValue(ctx, case_, false, encountered)
		if err != nil {
			return nil, err
		}
		cases = append(cases, symbolicVal.(symbolic.Pattern))
	}

	return symbolic.NewUnionPattern(cases, p.disjoint)
}

func (p *IntersectionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	intersectionPattern := &symbolic.IntersectionPattern{}
	encountered[ptr] = intersectionPattern

	var cases []symbolic.Pattern
	for _, case_ := range p.cases {
		symbolicVal, err := _toSymbolicValue(ctx, case_, false, encountered)
		if err != nil {
			return nil, err
		}
		cases = append(cases, symbolicVal.(symbolic.Pattern))
	}

	return symbolic.NewIntersectionPattern(cases)
}

func (p *RegexPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return &symbolic.RegexPattern{}, nil
}

func (p *RuneRangeStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *IntRangePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if p.multipleOf > 0 || p.multipleOfFloat != nil {
		return symbolic.ANY_INT_RANGE_PATTERN, nil
	}
	intRange, err := p.intRange.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert int range of int range pattern to symbolic: %w", err)
	}
	return symbolic.NewIntRangePattern(intRange.(*symbolic.IntRange)), nil
}

func (p *FloatRangePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if p.multipleOf > 0 {
		return symbolic.ANY_FLOAT_RANGE_PATTERN, nil
	}
	floatRange, err := p.floatRange.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert float range of float range pattern to symbolic: %w", err)
	}
	return symbolic.NewFloatRangePattern(floatRange.(*symbolic.FloatRange)), nil
}

func (p *UnionStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *RepeatedPatternElement) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *LengthCheckingStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewLengthCheckingStringPattern(p.lengthRange.start, p.lengthRange.end), nil
}

func (p *SequenceStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if p.node == nil {
		return symbolic.NewSequenceStringPattern(nil, nil), nil
	}
	return symbolic.NewSequenceStringPattern(p.node, p.nodeChunk), nil
}

func (p *ExactValuePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	exactValPattern := &symbolic.ExactValuePattern{}
	encountered[ptr] = exactValPattern

	symbolicVal, err := _toSymbolicValue(ctx, p.value, false, encountered)
	if err != nil {
		return nil, err
	}
	exactValPattern.SetVal(symbolicVal.(symbolic.Serializable))
	return exactValPattern, nil
}

func (p *ExactStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	symbolicVal, err := _toSymbolicValue(ctx, p.value, false, encountered)
	if err != nil {
		return nil, err
	}
	exactValPattern := symbolic.NewExactStringPatternWithConcreteValue(symbolicVal.(*symbolic.String))
	encountered[ptr] = exactValPattern

	return exactValPattern, nil
}

func (p *ListPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	listPattern := &symbolic.ListPattern{}
	encountered[ptr] = listPattern

	if p.generalElementPattern != nil {
		generalElement, err := _toSymbolicValue(ctx, p.generalElementPattern, false, encountered)
		if err != nil {
			return nil, err
		}
		symbolic.InitializeListPatternGeneralElement(listPattern, generalElement.(symbolic.Pattern))
	} else {
		elements := make([]symbolic.Pattern, 0)
		for _, e := range p.elementPatterns {
			element, err := _toSymbolicValue(ctx, e, false, encountered)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element.(symbolic.Pattern))
		}
		symbolic.InitializeListPatternElements(listPattern, elements)
	}
	return listPattern, nil
}

func (p *TuplePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	tuplePattern := &symbolic.TuplePattern{}
	encountered[ptr] = tuplePattern

	if p.generalElementPattern != nil {
		generalElement, err := _toSymbolicValue(ctx, p.generalElementPattern, false, encountered)
		if err != nil {
			return nil, err
		}
		symbolic.InitializeTuplePatternGeneralElement(tuplePattern, generalElement.(symbolic.Pattern))
	} else {
		elements := make([]symbolic.Pattern, 0)
		for _, e := range p.elementPatterns {
			element, err := _toSymbolicValue(ctx, e, false, encountered)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element.(symbolic.Pattern))
		}
		symbolic.InitializeTuplePatternElements(tuplePattern, elements)
	}
	return tuplePattern, nil
}

func (p *ObjectPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	objPattern := symbolic.NewUnitializedObjectPattern()
	encountered[ptr] = objPattern

	entries := map[string]symbolic.Pattern{}
	optionalEntries := map[string]struct{}{}

	for _, entry := range p.entries {
		val, err := _toSymbolicValue(ctx, entry.Pattern, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[entry.Name] = val.(symbolic.Pattern)
		if entry.IsOptional {
			optionalEntries[entry.Name] = struct{}{}
		}
	}

	//TODO: initialize constraints

	symbolic.InitializeObjectPattern(objPattern, entries, optionalEntries, p.inexact)
	return objPattern, nil
}

func (p *RecordPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	recPattern := symbolic.NewUnitializedRecordPattern()
	encountered[ptr] = recPattern

	entries := map[string]symbolic.Pattern{}
	optionalEntries := map[string]struct{}{}

	for _, entry := range p.entries {
		val, err := _toSymbolicValue(ctx, entry.Pattern, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[entry.Name] = val.(symbolic.Pattern)
		if entry.IsOptional {
			optionalEntries[entry.Name] = struct{}{}
		}
	}

	//TODO: initialize constraints

	symbolic.InitializeRecordPattern(recPattern, entries, optionalEntries, p.inexact)
	return recPattern, nil
}

func (p *OptionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	pattern, err := p.value.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert value pattern of option pattern to symbolic: %w", err)
	}
	return symbolic.NewOptionPattern(p.name, pattern.(symbolic.Pattern)), nil
}

func (p *TypePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	return p.ToSymbolicVal(), nil
}

func (p *TypePattern) ToSymbolicVal() symbolic.Pattern {
	for _, patt := range DEFAULT_NAMED_PATTERNS {
		switch patt.(type) {
		case *TypePattern:
			if SamePointer(p, patt) {
				return symbolic.NewTypePattern(
					p.SymbolicValue,
					p.SymbolicCallImpl,
					p.symbolicStringPattern,
					p,
				)
			}
		}
	}
	for _, namespace := range DEFAULT_PATTERN_NAMESPACES {
		for _, patt := range namespace.Patterns {
			switch patt.(type) {
			case *TypePattern:
				if SamePointer(p, patt) {
					return symbolic.NewTypePattern(
						p.SymbolicValue,
						p.SymbolicCallImpl,
						p.symbolicStringPattern,
						p,
					)
				}
			}
		}
	}
	for _, patt := range NOT_ACCESSIBLE_PATTERNS {
		switch patt.(type) {
		case *TypePattern:
			if SamePointer(p, patt) {
				return symbolic.NewTypePattern(
					p.SymbolicValue,
					p.SymbolicCallImpl,
					p.symbolicStringPattern,
					p,
				)
			}
		}
	}
	return symbolic.ANY_PATTERN
}

func (p NamedSegmentPathPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewNamedSegmentPathPattern(p.node), nil
}

func (p *DynamicStringPatternElement) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *DifferencePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	base, err := p.base.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	removed, err := p.removed.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return &symbolic.DifferencePattern{
		Base:    base.(symbolic.Pattern),
		Removed: removed.(symbolic.Pattern),
	}, nil
}

func (p *OptionalPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbPatt, err := p.pattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewOptionalPattern(symbPatt.(symbolic.Pattern)), nil
}

func (p *FunctionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return p.symbolicValue, nil
}

func (p *EventPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	symValuePattern, err := p.valuePattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewEventPattern(symValuePattern.(symbolic.Pattern))
}

func (p *MutationPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	data0Pattern, err := p.data0.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewMutationPattern(&symbolic.Int{}, data0Pattern.(symbolic.Pattern)), nil
}

func (p *ParserBasedPseudoPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return symbolic.NewParserBasedPattern(), nil
}

func (p *IntRangeStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *FloatRangeStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *PathStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (f *GoFunction) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	goFunc := f.fn
	ptr := reflect.ValueOf(goFunc).Pointer()
	symbolicGoFunc, ok := symbolicGoFunctionMap[ptr]
	if !ok {
		return nil, fmt.Errorf("missing symbolic equivalent of Go function: %#v %s", goFunc, runtime.FuncForPC(ptr).Name())
	}
	return symbolicGoFunc, nil
}

func (y Year) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewYear(time.Time(y)), nil
}

func (d Date) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewDate(time.Time(d)), nil
}

func (d DateTime) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewDateTime(time.Time(d)), nil
}

func (d Duration) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewDuration(time.Duration(d)), nil
}

func (b Byte) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_BYTE, nil
}

func (s *ByteSlice) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_BYTE_SLICE, nil
}

func (s Scheme) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.GetOrNewScheme(s.UnderlyingString()), nil
}

func (h Host) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewHost(h.UnderlyingString()), nil
}

func (addr EmailAddress) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewEmailAddress(addr.UnderlyingString()), nil
}

func (n AstNode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.AstNode{Node: n.Node}, nil
}

func (t Token) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_TOKEN, nil
}

func (m FileMode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_FILEMODE, nil
}

func (r QuantityRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	elem, err := r.start.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert lower bound of quantity range to symbolic: %w", err)
	}
	return symbolic.NewQuantityRange(elem.(symbolic.Serializable)), nil
}

func (r IntRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if r.unknownStart {
		return symbolic.ANY_INT_RANGE, nil
	}

	return symbolic.NewIntRange(
		symbolic.NewInt(r.start),
		symbolic.NewInt(r.end),
		r.step != 1,
	), nil
}

func (r FloatRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if r.unknownStart {
		return symbolic.ANY_FLOAT_RANGE, nil
	}

	return symbolic.NewFloatRange(
		symbolic.NewFloat(r.start),
		symbolic.NewFloat(r.end),
		r.inclusiveEnd,
	), nil
}

func (r RuneRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.RuneRange{}, nil
}

func (c ByteCount) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewByteCount(int64(c)), nil
}

func (c LineCount) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewLineCount(int64(c)), nil
}

func (c RuneCount) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewRuneCount(int64(c)), nil
}

func (r ByteRate) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewByteRate(int64(r)), nil
}

func (f Frequency) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewFrequency(float64(f)), nil
}

func (r *Reader) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_READER, nil
}

func (writer *Writer) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Writer{}, nil
}

func (it *KeyFilteredIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *ValueFilteredIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *KeyValueFilteredIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *ArrayIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *indexableIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *immutableSliceIterator[T]) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it IntRangeIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it FloatRangeIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it RuneRangeIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it QuantityRangeIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it PatternIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it indexedEntryIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *IpropsIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *EventSourceIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *DirWalker) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *TreedataWalker) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *ValueListIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *NumberListIterator[T]) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *BitSetIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *StrListIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (it *TupleIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

//

func (r *LThread) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.LThread{}, nil
}

func (g *LThreadGroup) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.LThreadGroup{}, nil
}

func (i FileInfo) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_FILEINFO, nil
}

func (t Mimetype) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewMimetype(t.UnderlyingString()), nil
}

func (fn *InoxFunction) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if fn.symbolicValue == nil {
		return nil, errors.New("cannot convert Inox function to symbolic value, .SymbolicValue is nil")
	}
	return fn.symbolicValue, nil
}

func (t Type) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Type{Type: t}, nil
}

func (tx *Transaction) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Transaction{}, nil
}

func (r *RandomnessSource) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.RandomnessSource{}, nil
}

func (m *Mapping) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Mapping{}, nil
}

func (ns *PatternNamespace) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {

	symbPatterns := make(map[string]symbolic.Pattern)
	for name, pattern := range ns.Patterns {
		symbPattern, err := pattern.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert member pattern %%%s to symbolic value: %w", name, err)
		}
		symbPatterns[name] = symbPattern.(symbolic.Pattern)
	}
	return symbolic.NewPatternNamespace(symbPatterns), nil
}

func (port Port) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_PORT, nil
}

func (u *Treedata) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Treedata{}, nil
}

func (e TreedataHiearchyEntry) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.TreedataHiearchyEntry{}, nil
}

func (c *StringConcatenation) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_STR_CONCAT, nil
}

func (c *BytesConcatenation) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_BYTES_CONCAT, nil
}

func (s *TestSuite) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.TestSuite{}, nil
}

func (c *TestCase) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.TestCase{}, nil
}

func (r *TestCaseResult) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	//TODO
	return symbolic.ANY, nil
}

func (d *DynamicValue) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	symbVal, err := d.Resolve(ctx).ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewDynamicValue(symbVal), nil
}

func (e *Event) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	symbVal, err := e.value.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewEvent(symbVal)
}

func (s *ExecutedStep) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.ExecutedStep{}, nil
}

func (j *LifetimeJob) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	var symbPattern symbolic.Pattern

	if j.subjectPattern == nil {
		symbPattern = j.symbolicSubjectObjectPattern
	} else {
		pattern, err := j.subjectPattern.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, err
		}
		symbPattern = pattern.(symbolic.Pattern)
	}

	return symbolic.NewLifetimeJob(symbPattern), nil
}

func _toSymbolicValue(ctx *Context, v Value, wide bool, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if encountered == nil {
		encountered = map[uintptr]symbolic.Value{}
	}

	rval := reflect.ValueOf(v)
	var ptr uintptr
	if rval.Kind() == reflect.Pointer {
		ptr = reflect.ValueOf(v).Pointer()
		if e, ok := encountered[ptr]; ok {
			return e, nil
		}
	}

	//should not be necessary
	if v == nil {
		return symbolic.Nil, nil
	}

	e, err := v.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	if rval.Kind() == reflect.Pointer {
		encountered[ptr] = e
	}
	return e, nil
}

func getConcretePatternMatchingSymbolicValue(ctx *Context, v symbolic.Value) (Pattern, bool) {
	encountered := map[uintptr]symbolic.Value{}

	for _, pattern := range DEFAULT_NAMED_PATTERNS {
		symb, err := pattern.ToSymbolicValue(ctx, encountered)
		if err != nil {
			continue
		}
		symbolicPattern := symb.(symbolic.Pattern)
		if symbolicPattern.TestValue(v, symbolic.RecTestCallState{}) &&
			v.Test(symbolicPattern.SymbolicValue(), symbolic.RecTestCallState{}) {
			return pattern, true
		}
	}
	//TODO: support patterns in namespaces
	//TODO: support specific symbolic values

	return nil, false
}

func (w *GenericWatcher) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (w *PeriodicWatcher) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (m Mutation) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Mutation{}, nil
}

func (w *joinedWatchers) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (w stoppedWatcher) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (s *wrappedWatcherStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	element, err := s.watcher.Config().Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewReadableStream(element), nil
}

func (s *ElementsStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	if s.filter == nil {
		return symbolic.READABLE_STREAM_ANY_ELEM, nil
	}
	element, err := s.filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewReadableStream(element), nil
}

func (s *ReadableByteStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewReadableStream(&symbolic.Byte{}), nil
}

func (s *WritableByteStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewWritableStream(&symbolic.Byte{}), nil
}

func (s *ConfluenceStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewWritableStream(symbolic.ANY), nil
}

func (c Color) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_COLOR, nil
}

func (r *RingBuffer) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_RING_BUFFER, nil
}

func (c *DataChunk) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	data, err := c.data.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data of chunk to symbolic value: %w", err)
	}
	return symbolic.NewChunk(data), nil
}

func (d *StaticCheckData) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.StaticCheckData{}, nil
}

func (d *SymbolicData) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return d.Data, nil
}

func (m *Module) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return m.ToSymbolic(), nil
}

func (s *GlobalState) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.GlobalState{}, nil
}

func (f *DateFormat) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_FORMAT, nil
}

func (m Message) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_MSG, nil
}

func (s *Subscription) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewSubscription(), nil
}

func (p *Publication) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewPublication(), nil
}

func (h *ValueHistory) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewValueHistory(), nil
}

func (h *SynchronousMessageHandler) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewMessageHandler(), nil
}

func (g *SystemGraph) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_SYSTEM_GRAPH, nil
}

func (n *SystemGraphNodes) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_SYSTEM_GRAPH_NODES, nil
}

func (n *SystemGraphNode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_SYSTEM_GRAPH_NODE, nil
}

func (e SystemGraphEvent) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_SYSTEM_GRAPH_EVENT, nil
}

func (e SystemGraphEdge) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_SYSTEM_GRAPH_EDGE, nil
}

func (s *Secret) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewSecret(
		utils.Must(s.value.ToSymbolicValue(ctx, encountered)),
		utils.Must(s.pattern.ToSymbolicValue(ctx, encountered)).(*symbolic.SecretPattern),
	)
}

func (p *SecretPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	stringPattern := utils.Must(p.stringPattern.ToSymbolicValue(ctx, encountered))

	return symbolic.NewSecretPattern(stringPattern.(symbolic.StringPattern)), nil
}

func (p *XMLElement) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {

	attributes := make(map[string]symbolic.Value, len(p.attributes))

	for _, attr := range p.attributes {
		symbolicVal, err := attr.value.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value of attribute '%s' to symbolic: %w", attr.name, err)
		}
		attributes[attr.name] = symbolicVal
	}

	children := make([]symbolic.Value, len(p.children))
	for i, child := range p.children {
		symbolicVal, err := child.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value of a child at index %d to symbolic: %w", i, err)
		}
		children = append(children, symbolicVal)
	}

	return symbolic.NewXmlElement(p.name, attributes, children), nil
}

func (db *DatabaseIL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	var schema *ObjectPattern
	if db.expectedSchema != nil {
		schema = db.expectedSchema
	} else if db.newSchemaSet.Load() {
		schema = db.newSchema
	} else {
		schema = db.initialSchema
	}

	pattern, err := schema.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to symbolic: %w", err)
	}

	params := symbolic.DatabaseILParams{
		Schema:               pattern.(*symbolic.ObjectPattern),
		SchemaUpdateExpected: db.schemaUpdateExpected,
	}

	switch r := db.Resource().(type) {
	case Host:
		params.BaseURL = symbolic.NewUrl(r.UnderlyingString() + "/")
	case URL:
		params.BaseURL = symbolic.NewUrl(r.UnderlyingString())
	}

	return symbolic.NewDatabaseIL(params), nil
}

func (api *ApiIL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	pattern, err := api.inner.Schema().ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to symbolic: %w", err)
	}

	return symbolic.NewApiIL(pattern.(*symbolic.ObjectPattern)), nil
}

func (ns *Namespace) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(ns).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	entries := map[string]symbolic.Value{}

	for key, val := range ns.entries {
		symbolicVal, err := val.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert namespace entry .%s to symbolic: %w", key, err)
		}
		entries[key] = symbolicVal
	}

	var result *symbolic.Namespace
	if ns.mutableEntries {
		result = symbolic.NewMutableEntriesNamespace(entries)
	} else {
		result = symbolic.NewNamespace(entries)
	}

	encountered[ptr] = result
	return result, nil
}

func (s *ModuleParamsPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	ptr := reflect.ValueOf(s).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	keys := slices.Clone(s.keys)
	types := make([]symbolic.Pattern, len(keys))

	symbolicModuleParamsPattern := new(symbolic.ModuleParamsPattern)
	encountered[ptr] = symbolicModuleParamsPattern

	for i, t := range s.types {
		symbolicPattern, err := t.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert field type .%s to symbolic: %w", keys[i], err)
		}
		types[i] = symbolicPattern.(symbolic.Pattern)
	}

	*symbolicModuleParamsPattern = symbolic.CreateModuleParamsPattern(keys, types)
	return symbolicModuleParamsPattern, nil
}

func (s *FilesystemSnapshotIL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_FS_SNAPSHOT_IL, nil
}

func (t *CurrentTest) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_CURRENT_TEST, nil
}

func (p *TestedProgram) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY_TESTED_PROGRAM, nil
}

func (id ULID) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewULID(id.libValue()), nil
}

func (id UUIDv4) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.NewUUID(id.libValue()), nil
}

func (Struct) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, errors.New("a StructAddress don't have a corresponding symbolic value")
}
