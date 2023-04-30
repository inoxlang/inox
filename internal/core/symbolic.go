package internal

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync/atomic"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

// this file contains the implementation of Value.ToSymbolicValue for core types and does some initialization.

var (
	symbolicGoFunctionMap = map[uintptr]*symbolic.GoFunction{}

	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}
)

func init() {

	symbolic.SetExternalData(symbolic.ExternalData{
		ToSymbolicValue: func(v any, wide bool) (symbolic.SymbolicValue, error) {
			return ToSymbolicValue(v.(Value), wide)
		},
		SymbolicToPattern: func(v symbolic.SymbolicValue) (any, bool) {
			return symbolicToPattern(v)
		},
		GetQuantity: func(values []float64, units []string) (any, error) {
			return evalQuantity(values, units)
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
				symbolicNamespace, err := ns.ToSymbolicValue(false, map[uintptr]symbolic.SymbolicValue{})
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
	ptr := reflect.ValueOf(fn).Pointer()
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

type SymbolicData struct {
	*symbolic.SymbolicData

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple

	NoReprMixin
	NotClonableMixin
}

func (d *SymbolicData) ErrorTuple() *Tuple {
	if d.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Value, len(d.SymbolicData.Errors()))
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

func ToSymbolicValue(v Value, wide bool) (symbolic.SymbolicValue, error) {
	return _toSymbolicValue(v, wide, make(map[uintptr]symbolic.SymbolicValue))
}

func (n NilT) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.Nil, nil
}

func (i Int) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Int{}, nil
}

func (b Bool) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Bool{}, nil
}

func (b Float) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Float{}, nil
}

func (r Rune) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Rune{}, nil
}

func (s Str) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.String{}, nil
}

func (s CheckedString) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.CheckedString{}, nil
}

func (s *RuneSlice) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RuneSlice{}, nil
}

func (e Error) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	data, err := e.data.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewError(data), nil
}

func (i Identifier) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Identifier{}, nil
}

func (p PropertyName) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.PropertyName{}, nil
}

func (p Path) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Path{}, nil
}

func (p PathPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.PathPattern{}, nil
}

func (u URL) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.URL{}, nil
}

func (u URLPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.URLPattern{}, nil
}

func (p HostPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.HostPattern{}, nil
}

func (o Option) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewOption(o.Name), nil
}

func (l *List) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return l.underylingList.ToSymbolicValue(wide, encountered)
}

func (l *ValueList) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY), nil
}

func (l *IntList) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY_INT), nil
}

func (l *BoolList) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewListOf(symbolic.ANY_BOOL), nil
}

func (l KeyList) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	var keys = make([]string, len(l))
	copy(keys, l)
	return &symbolic.KeyList{Keys: keys}, nil
}

func (t Tuple) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	//TODO
	return symbolic.NewTupleOf(&symbolic.Any{}), nil
}

func (obj *Object) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.NewAnyObject(), nil
	}

	ptr := reflect.ValueOf(obj).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbolicObj := symbolic.NewUnitializedObject()
	encountered[ptr] = symbolicObj

	entries := map[string]symbolic.SymbolicValue{}

	obj.Lock(nil)
	defer obj.Unlock(nil)
	for i, v := range obj.values {
		k := obj.keys[i]
		symbolicVal, err := _toSymbolicValue(v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = symbolicVal
	}

	symbolic.InitializeObject(symbolicObj, entries, nil)
	return symbolicObj, nil
}

func (rec *Record) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.NewAnyrecord(), nil
	}

	ptr := reflect.ValueOf(rec).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	entries := make(map[string]symbolic.SymbolicValue)
	symbolicRec := symbolic.NewBoundEntriesRecord(entries)
	encountered[ptr] = symbolicRec

	for i, v := range rec.values {
		k := rec.keys[i]

		symbolicVal, err := _toSymbolicValue(v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = symbolicVal
	}

	return symbolicRec, nil
}

func (dict *Dictionary) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return &symbolic.Dictionary{Entries: nil, Keys: nil}, nil
	}

	ptr := reflect.ValueOf(dict).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbolicDict := &symbolic.Dictionary{
		Entries: make(map[string]symbolic.SymbolicValue),
		Keys:    make(map[string]symbolic.SymbolicValue),
	}
	encountered[ptr] = symbolicDict

	for keyRepresentation, v := range dict.Entries {
		symbolicVal, err := _toSymbolicValue(v, false, encountered)
		if err != nil {
			return nil, err
		}

		key := dict.Keys[keyRepresentation]
		symbolicKey, err := _toSymbolicValue(key, false, encountered)
		if err != nil {
			return nil, err
		}
		symbolicDict.Entries[keyRepresentation] = symbolicVal
		symbolicDict.Keys[keyRepresentation] = symbolicKey
	}

	return symbolicDict, nil
}

func (p *UnionPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		panic(symbolic.ErrWideSymbolicValue)
	}
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	unionPattern := &symbolic.UnionPattern{}
	encountered[ptr] = unionPattern

	for _, case_ := range p.cases {
		symbolicVal, err := _toSymbolicValue(case_, false, encountered)
		if err != nil {
			return nil, err
		}
		unionPattern.Cases = append(unionPattern.Cases, symbolicVal.(symbolic.Pattern))
	}

	return unionPattern, nil
}

func (p *IntersectionPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		panic(symbolic.ErrWideSymbolicValue)
	}
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	intersectionPattern := &symbolic.IntersectionPattern{}
	encountered[ptr] = intersectionPattern

	for _, case_ := range p.cases {
		symbolicVal, err := _toSymbolicValue(case_, false, encountered)
		if err != nil {
			return nil, err
		}
		intersectionPattern.Cases = append(intersectionPattern.Cases, symbolicVal.(symbolic.Pattern))
	}

	return intersectionPattern, nil
}

func (p *RegexPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return &symbolic.RegexPattern{}, nil
}

func (p *RuneRangeStringPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPatternElement{}, nil
}

func (p *IntRangePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.IntRangePattern{}, nil
}

func (p *UnionStringPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPatternElement{}, nil
}

func (p *RepeatedPatternElement) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPatternElement{}, nil
}

func (p *SequenceStringPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.SequenceStringPattern{}, nil
}

func (p *ExactValuePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.NewExactValuePattern(&symbolic.Any{}), nil
	}

	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	exactValPattern := &symbolic.ExactValuePattern{}
	encountered[ptr] = exactValPattern

	symbolicVal, err := _toSymbolicValue(p.value, false, encountered)
	if err != nil {
		return nil, err
	}
	exactValPattern.SetVal(symbolicVal)
	return exactValPattern, nil
}

func (p *ListPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.ANY_LIST_PATTERN.WidestOfType(), nil
	}

	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	listPattern := &symbolic.ListPattern{}
	encountered[ptr] = listPattern

	if p.generalElementPattern != nil {
		generalElement, err := _toSymbolicValue(p.generalElementPattern, false, encountered)
		if err != nil {
			return nil, err
		}
		symbolic.InitializeListPatternGeneralElement(listPattern, generalElement.(symbolic.Pattern))
	} else {
		elements := make([]symbolic.Pattern, 0)
		for _, e := range p.elementPatterns {
			element, err := _toSymbolicValue(e, false, encountered)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element.(symbolic.Pattern))
		}
		symbolic.InitializeListPatternElements(listPattern, elements)
	}
	return listPattern, nil
}

func (p *TuplePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.ANY_TUPLE_PATTERN.WidestOfType(), nil
	}

	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	tuplePattern := &symbolic.TuplePattern{}
	encountered[ptr] = tuplePattern

	if p.generalElementPattern != nil {
		generalElement, err := _toSymbolicValue(p.generalElementPattern, false, encountered)
		if err != nil {
			return nil, err
		}
		symbolic.InitializeTuplePatternGeneralElement(tuplePattern, generalElement.(symbolic.Pattern))
	} else {
		elements := make([]symbolic.Pattern, 0)
		for _, e := range p.elementPatterns {
			element, err := _toSymbolicValue(e, false, encountered)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element.(symbolic.Pattern))
		}
		symbolic.InitializeTuplePatternElements(tuplePattern, elements)
	}
	return tuplePattern, nil
}

func (p *ObjectPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.NewAnyObjectPattern(), nil
	}

	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	objPattern := symbolic.NewUnitializedObjectPattern()
	encountered[ptr] = objPattern

	entries := map[string]symbolic.Pattern{}

	for k, v := range p.entryPatterns {
		val, err := _toSymbolicValue(v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = val.(symbolic.Pattern)
	}

	//TODO: initialize constraints

	symbolic.InitializeObjectPattern(objPattern, entries, p.inexact)
	return objPattern, nil
}

func (p *RecordPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if wide {
		return symbolic.NewAnyRecordPattern(), nil
	}

	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	recPattern := symbolic.NewUnitializedRecordPattern()
	encountered[ptr] = recPattern

	entries := map[string]symbolic.Pattern{}

	for k, v := range p.entryPatterns {
		val, err := _toSymbolicValue(v, false, encountered)
		if err != nil {
			return nil, err
		}
		entries[k] = val.(symbolic.Pattern)
	}

	//TODO: initialize constraints

	symbolic.InitializeRecordPattern(recPattern, entries, p.inexact)
	return recPattern, nil
}

func (p *OptionPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return &symbolic.OptionPattern{}, nil
}

func (p *TypePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (p NamedSegmentPathPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewNamedSegmentPathPattern(p.node), nil
}

func (p *DynamicStringPatternElement) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AnyStringPatternElement{}, nil
}

func (p *DifferencePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	base, err := p.base.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	removed, err := p.removed.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return &symbolic.DifferencePattern{
		Base:    base.(symbolic.Pattern),
		Removed: removed.(symbolic.Pattern),
	}, nil
}

func (p *OptionalPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}

	symbPatt, err := p.Pattern.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewOptionalPattern(symbPatt.(symbolic.Pattern)), nil
}

func (p *FunctionPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return p.symbolicValue, nil
}

func (p *EventPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	symValuePattern, err := p.ValuePattern.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewEventPattern(symValuePattern.(symbolic.Pattern))
}

func (p *MutationPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	data0Pattern, err := p.data0.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewMutationPattern(&symbolic.Int{}, data0Pattern.(symbolic.Pattern)), nil
}

func (p *ParserBasedPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	ptr := reflect.ValueOf(p).Pointer()
	if r, ok := encountered[ptr]; ok {
		return r, nil
	}
	return symbolic.NewParserBasedPattern(), nil
}

func (p *IntRangeStringPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_STR_PATTERN_ELEM, nil
}

func (p *PathStringPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_STR_PATTERN_ELEM, nil
}

func (f *GoFunction) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	goFunc := f.fn
	ptr := reflect.ValueOf(goFunc).Pointer()
	symbolicGoFunc, ok := symbolicGoFunctionMap[ptr]
	if !ok {
		return nil, fmt.Errorf("missing symbolic equivalent of Go function: %#v %s", goFunc, runtime.FuncForPC(ptr).Name())
	}
	return symbolicGoFunc, nil
}

func (d Date) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Date{}, nil
}

func (d Duration) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Duration{}, nil
}

func (b Byte) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Byte{}, nil
}

func (s *ByteSlice) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.ByteSlice{}, nil
}

func (s Scheme) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Scheme{}, nil
}

func (h Host) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Host{}, nil
}

func (hddr EmailAddress) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.EmailAddress{}, nil
}

func (n AstNode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.AstNode{Node: n.Node}, nil
}

func (t Token) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_TOKEN, nil
}

func (m FileMode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.FileMode{}, nil
}

func (r QuantityRange) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.QuantityRange{}, nil
}

func (r IntRange) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.IntRange{}, nil
}

func (r RuneRange) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RuneRange{}, nil
}

func (c ByteCount) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.ByteCount{}, nil
}

func (c LineCount) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.LineCount{}, nil
}

func (c RuneCount) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RuneCount{}, nil
}

func (r ByteRate) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.ByteRate{}, nil
}

func (r SimpleRate) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.SimpleRate{}, nil
}

func (r *Reader) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Reader{}, nil
}

func (writer *Writer) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Writer{}, nil
}

func (it *KeyFilteredIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *ValueFilteredIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *KeyValueFilteredIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *indexableIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *immutableSliceIterator[T]) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it IntRangeIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it RuneRangeIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it PatternIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it indexedEntryIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *IpropsIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *EventSourceIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *DirWalker) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *ValueListIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *IntListIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *BitSetIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (it *TupleIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

//

func (r *Routine) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Routine{}, nil
}

func (g *RoutineGroup) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RoutineGroup{}, nil
}

func (i FileInfo) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.FileInfo{}, nil
}

func (t Mimetype) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Mimetype{}, nil
}

func (fn *InoxFunction) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	if fn.symbolicValue == nil {
		return nil, errors.New("cannot convert Inox function to symbolic value, .SymbolicValue is nil")
	}
	return fn.symbolicValue, nil
}

func (b *Bytecode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Bytecode{Bytecode: b}, nil
}

func (t Type) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Type{Type: t}, nil
}

func (tx *Transaction) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Transaction{}, nil
}

func (r *RandomnessSource) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RandomnessSource{}, nil
}

func (m *Mapping) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Mapping{}, nil
}

func (ns *PatternNamespace) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {

	symbPatterns := make(map[string]symbolic.Pattern)
	for name, pattern := range ns.Patterns {
		symbPattern, err := pattern.ToSymbolicValue(wide, encountered)
		if err != nil {
			return nil, fmt.Errorf("failed to convert member pattern %%%s to symbolic value: %w", name, err)
		}
		symbPatterns[name] = symbPattern.(symbolic.Pattern)
	}
	return symbolic.NewPatternNamespace(symbPatterns), nil
}

func (port Port) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Port{}, nil
}

func (u *UData) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.UData{}, nil
}

func (e UDataHiearchyEntry) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.UDataHiearchyEntry{}, nil
}

func (c *StringConcatenation) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.StringConcatenation{}, nil
}

func (c *BytesConcatenation) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.BytesConcatenation{}, nil
}

func (s *TestSuite) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.TestSuite{}, nil
}

func (c *TestCase) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.TestCase{}, nil
}

func (d *DynamicValue) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	symbVal, err := d.Resolve(nil).ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewDynamicValue(symbVal), nil
}

func (e *Event) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	symbVal, err := e.value.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewEvent(symbVal)
}

func (s *ExecutedStep) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.ExecutedStep{}, nil
}

func (j *LifetimeJob) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	symbPattern, err := j.subjectPattern.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewLifetimeJob(symbPattern.(symbolic.Pattern)), nil
}

func _toSymbolicValue(v Value, wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

	e, err := v.ToSymbolicValue(wide, encountered)
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
		symbolicPattern, err := pattern.ToSymbolicValue(false, encountered)
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

func (w *GenericWatcher) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (w *PeriodicWatcher) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (m Mutation) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Mutation{}, nil
}

func (w *joinedWatchers) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (w stoppedWatcher) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	filter, err := w.config.Filter.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return symbolic.NewWatcher(filter.(symbolic.Pattern)), nil
}

func (s *wrappedWatcherStream) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	element, err := s.watcher.Config().Filter.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewReadableStream(element), nil
}

func (s *ElementsStream) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	element, err := s.filter.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return symbolic.NewReadableStream(element), nil
}

func (s *ReadableByteStream) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewReadableStream(&symbolic.Byte{}), nil
}

func (s *WritableByteStream) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewWritableStream(&symbolic.Byte{}), nil
}

func (s *ConfluenceStream) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewWritableStream(&symbolic.Any{}), nil
}

func (c Color) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Color{}, nil
}

func (r *RingBuffer) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.RingBuffer{}, nil
}

func (c *DataChunk) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	data, err := c.data.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data of chunk to symbolic value: %w", err)
	}
	return symbolic.NewChunk(data), nil
}

func (d *StaticCheckData) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.StaticCheckData{}, nil
}

func (d *SymbolicData) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return d.SymbolicData, nil
}

func (m *Module) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return m.ToSymbolic(), nil
}

func (s *GlobalState) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.GlobalState{}, nil
}

func (f *DateFormat) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_FORMAT, nil
}

func (m Message) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_MSG, nil
}

func (s *Subscription) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewSubscription(), nil
}

func (p *Publication) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewPublication(), nil
}

func (h *ValueHistory) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewValueHistory(), nil
}

func (h *SynchronousMessageHandler) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewMessageHandler(), nil
}

func (g *SystemGraph) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH, nil
}

func (n *SystemGraphNodes) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_NODES, nil
}

func (n *SystemGraphNode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_NODE, nil
}

func (e SystemGraphEvent) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_EVENT, nil
}

func (e SystemGraphEdge) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY_SYSTEM_GRAPH_EDGE, nil
}

func (s *Secret) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewSecret(
		utils.Must(s.value.ToSymbolicValue(wide, encountered)),
		utils.Must(s.pattern.ToSymbolicValue(wide, encountered)).(*symbolic.SecretPattern),
	)
}

func (p *SecretPattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	stringPattern := utils.Must(p.stringPattern.ToSymbolicValue(wide, encountered))

	return symbolic.NewSecretPattern(stringPattern.(symbolic.StringPatternElement)), nil
}
