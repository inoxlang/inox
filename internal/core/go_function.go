package core

import (
	"fmt"
	"log"
	"reflect"
	"sync/atomic"
	"testing"
)

var (
	OPTIONAL_PARAM_TYPE = reflect.TypeOf((*optionalParam)(nil)).Elem()
)

// A GoFunction represents a native (Go) function.
type GoFunction struct {
	fn          any //example: func add(*Context, a, b Int) Int
	kind        GoFunctionKind
	shared      atomic.Bool
	originState *GlobalState // used for methods & closures, nil otherwise
}

type GoFunctionKind int

const (
	GoFunc GoFunctionKind = iota
	GoMethod
	GoClosure
)

func WrapGoFunction(goFn any) *GoFunction {
	return &GoFunction{fn: goFn, kind: GoFunc}
}

func WrapGoClosure(goFn any) *GoFunction {
	return &GoFunction{fn: goFn, kind: GoClosure}
}

func WrapGoMethod(goFn any) *GoFunction {
	return &GoFunction{fn: goFn, kind: GoMethod}
}

func (fn *GoFunction) GoFunc() any {
	return fn.fn
}

func (fn *GoFunction) Kind() GoFunctionKind {
	return fn.kind
}

func (fn *GoFunction) IsSharable(originState *GlobalState) (bool, string) {
	// sync with symbolic
	// TODO: consider allowing methods & closures (this would probably require a lock for calls)
	if fn.kind == GoFunc {
		return true, ""
	}
	return false, "Go function is not sharable because it's a Go method or Go closure"
}

func (fn *GoFunction) Share(originState *GlobalState) {
	if fn.shared.CompareAndSwap(false, true) {
		fn.originState = originState
	}
}

func (fn *GoFunction) IsShared() bool {
	return fn.shared.Load()
}

func (fn *GoFunction) SmartLock(state *GlobalState) {
}

func (fn *GoFunction) SmartUnlock(state *GlobalState) {

}

func (fn *GoFunction) Prop(ctx *Context, name string) Value {
	method, ok := fn.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, fn))
	}
	return method
}

func (*GoFunction) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*GoFunction) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (*GoFunction) PropertyNames(ctx *Context) []string {
	return nil
}

// Call calls the underlying native function using reflection, the global state .goCallArgPrepBuf and .goCallArgsBuf
// buffers are used. The reflect package makes 2 small allocations during the call.
func (goFunc *GoFunction) Call(args []any, globalState, extState *GlobalState, isExt, must bool) (Value, error) {
	defer func() {
		clear(globalState.goCallArgPrepBuf)
		clear(globalState.goCallArgsBuf)
	}()

	fnVal := reflect.ValueOf(goFunc.fn)
	fnValType := fnVal.Type()

	if fnVal.Kind() != reflect.Func {
		log.Panicf("cannot call Go value of kind %s: %#v (%T)\n", fnVal.Kind(), goFunc.fn, goFunc.fn)
	}

	//If the function comes from an other state we need to use its context
	//in order to execute the function with its permissions.
	var ctx *Context = globalState.Ctx
	if isExt {
		ctx = extState.Ctx
	}

	numIn := fnValType.NumIn()
	isVariadic := fnValType.IsVariadic()

	if numIn == 0 || !CTX_PTR_TYPE.AssignableTo(fnValType.In(0)) {
		//ok
	} else {
		//add context as the first argument
		if len(args) >= len(globalState.goCallArgPrepBuf) {
			globalState.goCallArgPrepBuf = make([]any, len(args)+1)
			globalState.goCallArgsBuf = make([]reflect.Value, len(args)+1)
		}

		copy(globalState.goCallArgPrepBuf[1:], args)
		globalState.goCallArgPrepBuf[0] = ctx
		args = globalState.goCallArgPrepBuf[:len(args)+1]
	}

	if testing.Testing() {
		functionOptionalParamInfoLock.Lock()
	}

	optionalParamInfo, ok := functionOptionalParamInfo[fnVal.Pointer()]
	if testing.Testing() {
		functionOptionalParamInfoLock.Unlock()
	}

	if ok {
		lastMandatoryParamIndex := optionalParamInfo.lastMandatoryParamIndex
		if len(args) < int(lastMandatoryParamIndex)+1 {
			return nil, fmt.Errorf("invalid number of arguments : %v, at least %v were expected", len(args), lastMandatoryParamIndex+1)
		}
		lastOptionalParamIndex := numIn - 1
		if isVariadic {
			lastOptionalParamIndex--
		}

		optionalParamInfoIndex := 0
		for paramIndex := int(lastMandatoryParamIndex) + 1; paramIndex <= lastOptionalParamIndex; paramIndex++ {
			optionalParam := optionalParamInfo.optionalParams[optionalParamInfoIndex]

			if paramIndex < len(args) {
				optionalParam = optionalParam.new()
				optionalParam.setValue(args[paramIndex].(Value))
				args[paramIndex] = optionalParam
			} else {
				args = append(args, optionalParam.newNil())
			}
			optionalParamInfoIndex++
		}
	} else {
		if len(args) != numIn && (!isVariadic || len(args) < numIn-1) {
			return nil, fmt.Errorf("invalid number of arguments : %v, %v was expected", len(args), numIn)
		}
	}

	argValues := globalState.goCallArgsBuf[:len(args)]

	//get the reflect.Value of every argument
	for i, arg := range args {
		argValue := reflect.ValueOf(arg)
		argValues[i] = argValue
	}

	resultValues := fnVal.Call(argValues)
	resultCount := fnValType.NumOut()

	select {
	case <-ctx.Done():
		panic(ctx.Err())
	default:
	}

	if must && resultCount >= 1 &&
		fnValType.Out(resultCount-1).Implements(ERROR_INTERFACE_TYPE) {
		lastElem := resultValues[len(resultValues)-1]

		if lastElem.IsNil() {
			resultValues = resultValues[:len(resultValues)-1]
		} else {
			panic(lastElem.Interface().(error))
		}
	}

	switch len(resultValues) {
	case 0:
		return Nil, nil
	case 1:
		if isExt {
			shared, err := ShareOrClone(ConvertReturnValue(resultValues[0]), extState)
			if err != nil {
				err = fmt.Errorf("failed to share/clone the result of Go function: %T: %w", resultValues[0].Interface(), err)
			}
			return shared, err
		}
		return ConvertReturnValue(resultValues[0]), nil
	}
	results := make([]Value, 0, len(resultValues))

	if isExt {
		for _, resultValue := range resultValues {
			shared, err := ShareOrClone(ConvertReturnValue(resultValue), extState)
			if err != nil {
				return nil, fmt.Errorf("failed to share/clone one of the result of a Go function: %T: %w", resultValue.Interface(), err)
			}
			results = append(results, shared)
		}
	} else {
		for _, resultValue := range resultValues {
			results = append(results, ConvertReturnValue(resultValue))
		}
	}

	//TODO: support any result types

	return NewArrayFrom(results...), nil
}

// ConvertReturnValue converts to Value a reflect.Value returned by calling a Go funtion using reflection.
func ConvertReturnValue(rval reflect.Value) Value {
	interf := rval.Interface()

	if !rval.IsValid() {
		return Nil
	}

	if val, ok := interf.(Value); ok {
		return val
	}

	if rval.Type().Implements(ERROR_INTERFACE_TYPE) {
		if rval.Interface() == nil {
			return Nil
		}
		return NewError(rval.Interface().(error), Nil)
	}

	if rval.Kind() == reflect.Slice || rval.Kind() == reflect.Pointer && rval.Elem().Kind() == reflect.Slice {
		switch v := interf.(type) {
		case []rune:
			return &RuneSlice{elements: v}
		case []byte:
			return NewMutableByteSlice(v, "")
		}

		list := &List{underlyingList: &ValueList{}}
		for i := 0; i < rval.Len(); i++ {
			list.append(nil, ValOf(rval.Index(i).Interface()).(Serializable))
		}
		return list
	}

	if rval.Type() == VALUE_TYPE {
		return Nil
	}
	panic(fmt.Errorf("cannot convert return value of type %v, value is %#v", rval.Type(), rval))
}

// optional parameter in symbolic Go function parameters
type OptionalParam[T Value] struct {
	Value T
}

func (p *OptionalParam[T]) _optionalParamType() {
	//type assertion
	_ = optionalParam(p)
}

func (p *OptionalParam[T]) setValue(v Value) {
	p.Value = v.(T)
}

func (p *OptionalParam[T]) new() optionalParam {
	return &OptionalParam[T]{}
}

func (p *OptionalParam[T]) newNil() optionalParam {
	return (*OptionalParam[T])(nil)
}

type optionalParam interface {
	_optionalParamType()
	setValue(v Value)
	new() optionalParam
	newNil() optionalParam
}

func ToOptionalParam[T Value](v T) *OptionalParam[T] {
	return &OptionalParam[T]{
		Value: v,
	}
}

func ToValueOptionalParam(v Value) *OptionalParam[Value] {
	return &OptionalParam[Value]{
		Value: v,
	}
}
