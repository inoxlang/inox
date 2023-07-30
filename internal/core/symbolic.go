package core

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

// this file contains the implementation of Value.ToSymbolicValue for core types and does some initialization.

var (
	symbolicGoFunctionMap = map[uintptr]*symbolic.GoFunction{}
	goFunctionMap         = map[*symbolic.GoFunction]reflect.Value{}

	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}
)

func init() {

	symbolic.SetExternalData(symbolic.ExternalData{
		ToSymbolicValue: func(v any, wide bool) (symbolic.SymbolicValue, error) {
			return ToSymbolicValue(nil, v.(Value), wide)
		},
		SymbolicToPattern: func(v symbolic.SymbolicValue) (any, bool) {
			return symbolicToPattern(v)
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
		ConvertKeyReprToValue: func(s string) any {
			return convertKeyReprToValue(s)
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
		IMPLICIT_KEY_LEN_KEY: IMPLICIT_KEY_LEN_KEY,
		CONSTRAINTS_KEY:      CONSTRAINTS_KEY,
		VISIBILITY_KEY:       VISIBILITY_KEY,

		DEFAULT_PATTERN_NAMESPACES: func() map[string]*symbolic.PatternNamespace {
			result := make(map[string]*symbolic.PatternNamespace)
			for name, ns := range DEFAULT_PATTERN_NAMESPACES {
				symbolicNamespace, err := ns.ToSymbolicValue(nil, map[uintptr]symbolic.SymbolicValue{})
				if err != nil {
					panic(err)
				}
				result[name] = symbolicNamespace.(*symbolic.PatternNamespace)
			}
			return result
		}(),
	})

}

// RegisterSymbolicGoFunction registers the symbolic equivalent of fn, fn should not be a method or a closure.
// example: RegisterSymbolicGoFunction(func(ctx *Context){ }, func(ctx *symbolic.Context))
func RegisterSymbolicGoFunction(fn any, symbolicFn any) {
	reflectVal := reflect.ValueOf(fn)
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
	*symbolic.SymbolicData

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

func (d *SymbolicData) ErrorTuple() *Tuple {
	if d.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Serializable, len(d.SymbolicData.Errors()))
		for i, err := range d.SymbolicData.Errors() {
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

func ToSymbolicValue(ctx *Context, v Value, wide bool) (symbolic.SymbolicValue, error) {
	return _toSymbolicValue(ctx, v, wide, make(map[uintptr]symbolic.SymbolicValue))
}

func GetStringifiedSymbolicValue(ctx *Context, v Value, wide bool) (string, error) {
	symbolicVal, err := ToSymbolicValue(ctx, v, wide)
	if err != nil {
		return "", err
	}
	return symbolic.Stringify(symbolicVal), nil
}

func (n NilT) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.Nil, nil
}

func (i Int) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewInt(int64(i)), nil
}

func (b Bool) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if b {
		return symbolic.TRUE, nil
	}
	return symbolic.FALSE, nil
}

func (f Float) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewFloat(float64(f)), nil
}

func (r Rune) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewRune(rune(r)), nil
}

func (s Str) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewString(string(s)), nil
}

func (s CheckedString) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_CHECKED_STR, nil
}

func (s *RuneSlice) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_RUNE_SLICE, nil
}

func (e Error) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	data, err := e.data.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewError(data), nil
}

func (i Identifier) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewIdentifier(i.UnderlyingString()), nil
}

func (p PropertyName) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewPropertyName(p.UnderlyingString()), nil
}

func (p Path) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_PATH, nil
}

func (p PathPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_PATH_PATTERN, nil
}

func (u URL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_URL, nil
}

func (u URLPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_URL_PATTERN, nil
}

func (p HostPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_HOST_PATTERN, nil
}

func (o Option) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewOption(o.Name), nil
}

func (*Array) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_ARRAY, nil
}

func (l *List) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return l.underylingList.ToSymbolicValue(ctx, encountered)
}

func (l *ValueList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY_SERIALIZABLE), nil
}

func (l *IntList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY_INT), nil
}

func (l *BoolList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY_BOOL), nil
}

func (l *StringList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY_STR_LIKE), nil
}

func (l KeyList) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	var keys = make([]string, len(l))
	copy(keys, l)
	return &symbolic.KeyList{Keys: keys}, nil
}

func (t Tuple) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	//TODO
	return symbolic.NewTupleOf(symbolic.ANY_SERIALIZABLE), nil
}

func (obj *Object) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(obj).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbolicObj := symbolic.NewUnitializedObject()
	encountered[ptr] = symbolicObj

	entries := map[string]symbolic.Serializable{}

	obj.Lock(nil)
	defer obj.Unlock(nil)
	for i, v := range obj.values {
		k := obj.keys[i]
		symbolicVal, err := _toSymbolicValue(ctx, v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = symbolicVal.(symbolic.Serializable)
	}

	symbolic.InitializeObject(symbolicObj, entries, nil)
	return symbolicObj, nil
}

func (rec *Record) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (dict *Dictionary) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (s *Struct) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(s).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	structPattern, err := s.structType.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert type of struct to symbolic: %w", err)
	}

	symbolicStruct := symbolic.NewStruct(structPattern.(*symbolic.StructPattern))
	encountered[ptr] = symbolicStruct

	return symbolicStruct, nil
}

func (p *UnionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	unionPattern := &symbolic.UnionPattern{}
	encountered[ptr] = unionPattern

	for _, case_ := range p.cases {
		symbolicVal, err := _toSymbolicValue(ctx, case_, false, encountered)
		if err != nil {
			return nil, err
		}
		unionPattern.Cases = append(unionPattern.Cases, symbolicVal.(symbolic.Pattern))
	}

	return unionPattern, nil
}

func (p *IntersectionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	intersectionPattern := &symbolic.IntersectionPattern{}
	encountered[ptr] = intersectionPattern

	for _, case_ := range p.cases {
		symbolicVal, err := _toSymbolicValue(ctx, case_, false, encountered)
		if err != nil {
			return nil, err
		}
		intersectionPattern.Cases = append(intersectionPattern.Cases, symbolicVal.(symbolic.Pattern))
	}

	return intersectionPattern, nil
}

func (p *RegexPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return &symbolic.RegexPattern{}, nil
}

func (p *RuneRangeStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPattern{}, nil
}

func (p *IntRangePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.IntRangePattern{}, nil
}

func (p *UnionStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPattern{}, nil
}

func (p *RepeatedPatternElement) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPattern{}, nil
}

func (p *SequenceStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.SequenceStringPattern{}, nil
}

func (p *ExactValuePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (p *ExactStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	exactValPattern := symbolic.NewExactStringPattern()
	encountered[ptr] = exactValPattern

	return exactValPattern, nil
}

func (p *ListPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (p *TuplePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (p *ObjectPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	objPattern := symbolic.NewUnitializedObjectPattern()
	encountered[ptr] = objPattern

	entries := map[string]symbolic.Pattern{}

	for k, v := range p.entryPatterns {
		val, err := _toSymbolicValue(ctx, v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = val.(symbolic.Pattern)
	}

	//TODO: initialize constraints

	symbolic.InitializeObjectPattern(objPattern, entries, p.optionalEntries, p.inexact)
	return objPattern, nil
}

func (p *RecordPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	recPattern := symbolic.NewUnitializedRecordPattern()
	encountered[ptr] = recPattern

	entries := map[string]symbolic.Pattern{}

	for k, v := range p.entryPatterns {
		val, err := _toSymbolicValue(ctx, v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = val.(symbolic.Pattern)
	}

	//TODO: initialize constraints

	symbolic.InitializeRecordPattern(recPattern, entries, p.optionalEntries, p.inexact)
	return recPattern, nil
}

func (p *OptionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return &symbolic.OptionPattern{}, nil
}

func (p *TypePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	for _, patt := range DEFAULT_NAMED_PATTERNS {
		switch patt.(type) {
		case *TypePattern:
			if SamePointer(p, patt) {
				return symbolic.NewTypePattern(
					p.SymbolicValue,
					p.SymbolicCallImpl,
					p.symbolicStringPattern,
				), nil
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
					), nil
				}
			}
		}
	}
	return &symbolic.AnyPattern{}, nil
}

func (p NamedSegmentPathPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewNamedSegmentPathPattern(p.node), nil
}

func (p *DynamicStringPatternElement) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPattern{}, nil
}

func (p *DifferencePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (p *OptionalPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbPatt, err := p.Pattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewOptionalPattern(symbPatt.(symbolic.Pattern)), nil
}

func (p *FunctionPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return p.symbolicValue, nil
}

func (p *EventPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	symValuePattern, err := p.ValuePattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewEventPattern(symValuePattern.(symbolic.Pattern))
}

func (p *MutationPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (p *ParserBasedPseudoPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return symbolic.NewParserBasedPattern(), nil
}

func (p *IntRangeStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (p *PathStringPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_STR_PATTERN, nil
}

func (f *GoFunction) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	goFunc := f.fn
	ptr := reflect.ValueOf(goFunc).Pointer()
	symbolicGoFunc, ok := symbolicGoFunctionMap[ptr]
	if !ok {
		return nil, fmt.Errorf("missing symbolic equivalent of Go function: %#v %s", goFunc, runtime.FuncForPC(ptr).Name())
	}
	return symbolicGoFunc, nil
}

func (d Date) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewDate(time.Time(d)), nil
}

func (d Duration) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewDuration(time.Duration(d)), nil
}

func (b Byte) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_BYTE, nil
}

func (s *ByteSlice) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_BYTE_SLICE, nil
}

func (s Scheme) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SCHEME, nil
}

func (h Host) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_HOST, nil
}

func (addr EmailAddress) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewEmailAddress(addr.UnderlyingString()), nil
}

func (n AstNode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AstNode{Node: n.Node}, nil
}

func (t Token) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_TOKEN, nil
}

func (m FileMode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_FILEMODE, nil
}

func (r QuantityRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	elem, err := r.Start.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert lower bound of quantity range to symbolic: %w", err)
	}
	return symbolic.NewQuantityRange(elem.(symbolic.Serializable)), nil
}

func (r IntRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.IntRange{}, nil
}

func (r RuneRange) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RuneRange{}, nil
}

func (c ByteCount) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewByteCount(int64(c)), nil
}

func (c LineCount) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewLineCount(int64(c)), nil
}

func (c RuneCount) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewRuneCount(int64(c)), nil
}

func (r ByteRate) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewByteRate(int64(r)), nil
}

func (r SimpleRate) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewSimpleRate(int64(r)), nil
}

func (r *Reader) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_READER, nil
}

func (writer *Writer) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Writer{}, nil
}

func (it *KeyFilteredIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *ValueFilteredIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *KeyValueFilteredIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *ArrayIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *indexableIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *immutableSliceIterator[T]) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it IntRangeIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it RuneRangeIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it PatternIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it indexedEntryIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *IpropsIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *EventSourceIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *DirWalker) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *ValueListIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *IntListIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *BitSetIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *StrListIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *TupleIterator) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

//

func (r *Routine) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Routine{}, nil
}

func (g *RoutineGroup) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RoutineGroup{}, nil
}

func (i FileInfo) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_FILEINFO, nil
}

func (t Mimetype) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewMimetype(t.UnderlyingString()), nil
}

func (fn *InoxFunction) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if fn.symbolicValue == nil {
		return nil, errors.New("cannot convert Inox function to symbolic value, .SymbolicValue is nil")
	}
	return fn.symbolicValue, nil
}

func (b *Bytecode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Bytecode{Bytecode: b}, nil
}

func (t Type) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Type{Type: t}, nil
}

func (tx *Transaction) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Transaction{}, nil
}

func (r *RandomnessSource) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RandomnessSource{}, nil
}

func (m *Mapping) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Mapping{}, nil
}

func (ns *PatternNamespace) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {

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

func (port Port) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_PORT, nil
}

func (u *UData) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.UData{}, nil
}

func (e UDataHiearchyEntry) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.UDataHiearchyEntry{}, nil
}

func (c *StringConcatenation) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.StringConcatenation{}, nil
}

func (c *BytesConcatenation) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.BytesConcatenation{}, nil
}

func (s *TestSuite) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.TestSuite{}, nil
}

func (c *TestCase) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.TestCase{}, nil
}

func (d *DynamicValue) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	symbVal, err := d.Resolve(ctx).ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewDynamicValue(symbVal), nil
}

func (e *Event) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	symbVal, err := e.value.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewEvent(symbVal)
}

func (s *ExecutedStep) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.ExecutedStep{}, nil
}

func (j *LifetimeJob) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func _toSymbolicValue(ctx *Context, v Value, wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if encountered == nil {
		encountered = map[uintptr]symbolic.SymbolicValue{}
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

func symbolicToPattern(v symbolic.SymbolicValue) (Pattern, bool) {
	encountered := map[uintptr]symbolic.SymbolicValue{}

	for _, pattern := range DEFAULT_NAMED_PATTERNS {
		symbolicPattern, err := pattern.ToSymbolicValue(nil, encountered)
		if err != nil {
			continue
		}
		matchedSymbolicVal := symbolicPattern.(symbolic.Pattern).SymbolicValue()
		if v.Test(matchedSymbolicVal) && matchedSymbolicVal.Test(v) {
			return pattern, true
		}
	}
	//TODO: support patterns in namespaces
	//TODO: support specific symbolic values

	return nil, false
}

func (w *GenericWatcher) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (w *PeriodicWatcher) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (m Mutation) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Mutation{}, nil
}

func (w *joinedWatchers) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (w stoppedWatcher) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (s *wrappedWatcherStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	element, err := s.watcher.Config().Filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewReadableStream(element), nil
}

func (s *ElementsStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	element, err := s.filter.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewReadableStream(element), nil
}

func (s *ReadableByteStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewReadableStream(&symbolic.Byte{}), nil
}

func (s *WritableByteStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewWritableStream(&symbolic.Byte{}), nil
}

func (s *ConfluenceStream) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewWritableStream(&symbolic.Any{}), nil
}

func (c Color) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Color{}, nil
}

func (r *RingBuffer) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RingBuffer{}, nil
}

func (c *DataChunk) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	data, err := c.data.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data of chunk to symbolic value: %w", err)
	}
	return symbolic.NewChunk(data), nil
}

func (d *StaticCheckData) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.StaticCheckData{}, nil
}

func (d *SymbolicData) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return d.SymbolicData, nil
}

func (m *Module) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return m.ToSymbolic(), nil
}

func (s *GlobalState) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.GlobalState{}, nil
}

func (f *DateFormat) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_FORMAT, nil
}

func (m Message) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_MSG, nil
}

func (s *Subscription) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewSubscription(), nil
}

func (p *Publication) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewPublication(), nil
}

func (h *ValueHistory) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewValueHistory(), nil
}

func (h *SynchronousMessageHandler) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewMessageHandler(), nil
}

func (g *SystemGraph) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH, nil
}

func (n *SystemGraphNodes) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_NODES, nil
}

func (n *SystemGraphNode) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_NODE, nil
}

func (e SystemGraphEvent) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_EVENT, nil
}

func (e SystemGraphEdge) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_EDGE, nil
}

func (s *Secret) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewSecret(
		utils.Must(s.value.ToSymbolicValue(ctx, encountered)),
		utils.Must(s.pattern.ToSymbolicValue(ctx, encountered)).(*symbolic.SecretPattern),
	)
}

func (p *SecretPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	stringPattern := utils.Must(p.stringPattern.ToSymbolicValue(ctx, encountered))

	return symbolic.NewSecretPattern(stringPattern.(symbolic.StringPattern)), nil
}

func (p *XMLElement) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {

	attributes := make(map[string]symbolic.SymbolicValue, len(p.attributes))

	for _, attr := range p.attributes {
		symbolicVal, err := attr.value.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value of attribute '%s' to symbolic: %w", attr.name, err)
		}
		attributes[attr.name] = symbolicVal
	}

	children := make([]symbolic.SymbolicValue, len(p.children))
	for i, child := range p.children {
		symbolicVal, err := child.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value of a child at index %d to symbolic: %w", i, err)
		}
		children = append(children, symbolicVal)
	}

	return symbolic.NewXmlElement(p.name, attributes, children), nil
}

func (db *DatabaseIL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	pattern, err := db.inner.Schema().ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to symbolic: %w", err)
	}

	return symbolic.NewDatabaseIL(pattern.(*symbolic.ObjectPattern)), nil
}

func (api *ApiIL) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	pattern, err := api.inner.Schema().ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to symbolic: %w", err)
	}

	return symbolic.NewApiIL(pattern.(*symbolic.ObjectPattern)), nil
}

func (ns *Namespace) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(ns).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	entries := map[string]symbolic.SymbolicValue{}

	for key, val := range ns.entries {
		symbolicVal, err := val.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert namespace entry .%s to symbolic: %w", key, err)
		}
		entries[key] = symbolicVal
	}

	result := symbolic.NewNamespace(entries)
	encountered[ptr] = result
	return result, nil
}

func (s *StructPattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(s).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	keys := utils.CopySlice(s.keys)
	types := make([]symbolic.Pattern, len(keys))

	symbolicStructPattern := new(symbolic.StructPattern)
	encountered[ptr] = symbolicStructPattern

	for i, t := range s.types {
		symbolicPattern, err := t.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert field type .%s to symbolic: %w", keys[i], err)
		}
		types[i] = symbolicPattern.(symbolic.Pattern)
	}

	*symbolicStructPattern = symbolic.CreateStructPattern(s.name, s.tempId, keys, types)
	return symbolicStructPattern, nil
}
