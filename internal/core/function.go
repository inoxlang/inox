package internal

import (
	"fmt"
	"log"
	"reflect"
	"sync/atomic"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	parse "github.com/inox-project/inox/internal/parse"
)

type InoxFunction struct {
	NoReprMixin
	Node                   parse.Node
	originState            *GlobalState
	shared                 atomic.Bool
	treeWalkCapturedLocals map[string]Value
	capturedGlobals        []capturedGlobal // set when shared, should not be nil in this case

	compiledFunction *CompiledFunction //can be nil
	capturedLocals   []Value           //alway empty if .CompiledFunction is nil

	symbolicValue *symbolic.InoxFunction
	staticData    *FunctionStaticData
}

type capturedGlobal struct {
	name  string
	value Value
}

func (fn *InoxFunction) FuncExpr() *parse.FunctionExpression {
	switch node := fn.Node.(type) {
	case *parse.FunctionDeclaration:
		return node.Function
	case *parse.FunctionExpression:
		return node
	default:
		if node == nil {
			return nil
		}
		panic(fmt.Errorf("InoxFunction has an invalid value for .Node: %#v", node))
	}
}

func (fn *InoxFunction) Call(globalState *GlobalState, self Value, args []Value) (Value, error) {
	if fn.compiledFunction != nil {
		vm, err := NewVM(VMConfig{
			Bytecode: fn.compiledFunction.Bytecode,
			Fn:       fn,
			State:    globalState,
			Self:     self,
			FnArgs:   args,
		})
		if err != nil {
			return nil, err
		}
		return vm.Run()
	} else {
		newState := NewTreeWalkStateWithGlobal(globalState)
		return TreeWalkCallFunc(fn, self, newState, newList(&ValueList{elements: args}), false, false)
	}
}

func (fn *InoxFunction) IsSharable(originState *GlobalState) (bool, string) {
	//TODO: only sharable if sharable captured locals ?

	if fn.staticData == nil {
		return true, ""
		//TODO: return false, "function is not sharable because static data is missing"
	}

	if fn.staticData.assignGlobal {
		return false, "function is not sharable because it assigns a global"
	}

	return true, ""
}

func (fn *InoxFunction) Share(originState *GlobalState) {
	if fn.shared.CompareAndSwap(false, true) {
		fn.originState = originState
		if fn.staticData != nil && len(fn.staticData.capturedGlobals) > 0 {
			fn.capturedGlobals = make([]capturedGlobal, len(fn.staticData.capturedGlobals))
			for i, name := range fn.staticData.capturedGlobals {

				value := originState.Globals.Get(name)
				if value == nil {
					panic(fmt.Errorf("function sharing: failed to capture global variable '%s' of origin state", name))
				}
				fn.capturedGlobals[i] = capturedGlobal{
					name:  name,
					value: value,
				}
			}
		}
	}
}

func (fn *InoxFunction) IsShared() bool {
	return fn.shared.Load()
}

func (fn *InoxFunction) Lock(state *GlobalState) {

}

func (fn *InoxFunction) Unlock(state *GlobalState) {

}

func (fn *InoxFunction) ForceLock() {
}

func (fn *InoxFunction) ForceUnlock() {

}

type GoFunction struct {
	fn          any
	kind        GoFunctionKind
	shared      atomic.Bool
	originState *GlobalState // used for methods & closures, nil otherwise
	NotClonableMixin
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

func (fn *GoFunction) Lock(state *GlobalState) {

}

func (fn *GoFunction) Unlock(state *GlobalState) {

}

func (fn *GoFunction) ForceLock() {
}

func (fn *GoFunction) ForceUnlock() {

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

func (goFunc *GoFunction) Call(args []any, globalState, extState *GlobalState, isExt, must bool) (Value, error) {
	fnVal := reflect.ValueOf(goFunc.fn)
	fnValType := fnVal.Type()

	if fnVal.Kind() != reflect.Func {
		log.Panicf("cannot call Go value of kind %s: %#v (%T)\n", fnVal.Kind(), goFunc.fn, goFunc.fn)
	}

	var ctx *Context = globalState.Ctx
	if isExt {
		ctx = extState.Ctx
	}

	if fnValType.NumIn() == 0 || !CTX_PTR_TYPE.AssignableTo(fnValType.In(0)) {
		//ok
	} else {
		args = append([]any{ctx}, args...)
	}

	if len(args) != fnValType.NumIn() && (!fnValType.IsVariadic() || len(args) < fnValType.NumIn()-1) {
		return nil, fmt.Errorf("invalid number of arguments : %v, %v was expected", len(args), fnValType.NumIn())
	}

	argValues := make([]reflect.Value, len(args))

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

	return newList(&ValueList{elements: results}), nil
}

func IsResultWithError(result Value) (bool, error) {
	if list, isList := result.(*List); isList && list.Len() != 0 {
		lastElem := reflect.ValueOf(list.Len() - 1)
		if lastElem.Type().Implements(ERROR_INTERFACE_TYPE) && !lastElem.IsNil() {
			return true, lastElem.Interface().(error)
		}
	}
	return false, nil
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
			return &ByteSlice{Bytes: v, IsDataMutable: true}
		}

		list := &List{underylingList: &ValueList{}}
		for i := 0; i < rval.Len(); i++ {
			list.append(nil, ValOf(rval.Index(i).Interface()))
		}
		return list
	}

	if rval.Type() == VALUE_TYPE {
		return Nil
	}
	panic(fmt.Errorf("cannot convert return value of type %v, value is %#v", rval.Type(), rval))
}
