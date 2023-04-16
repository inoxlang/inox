package internal

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// An InoxFunction represents a symbolic InoxFunction.
// TODO: keep in sync with concrete InoxFunction
type InoxFunction struct {
	node           parse.Node //if nil, any function is matched
	parameters     []SymbolicValue
	parameterNames []string
	returnType     SymbolicValue
	capturedLocals map[string]SymbolicValue
	originState    *State
}

func (fn *InoxFunction) FuncExpr() *parse.FunctionExpression {
	switch node := fn.node.(type) {
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

func (fn *InoxFunction) Test(v SymbolicValue) bool {
	other, ok := v.(*InoxFunction)
	if !ok {
		return false
	}
	if fn.node == nil {
		return true
	}

	if other.node == nil {
		return false
	}

	return utils.SamePointer(fn.node, other.node)
}

func (fn *InoxFunction) IsSharable() (bool, string) {
	//TODO: reconciliate with concrete version
	return true, ""
}

func (fn *InoxFunction) Share(originState *State) PotentiallySharable {
	if fn.originState != nil {
		return fn
	}

	copy := *fn
	copy.originState = originState
	return &copy
}

func (fn *InoxFunction) IsShared() bool {
	return fn.originState != nil
}

func (fn *InoxFunction) Widen() (SymbolicValue, bool) {
	if fn.node == nil {
		return nil, false
	}
	return &InoxFunction{}, true
}

func (fn *InoxFunction) IsWidenable() bool {
	return fn.node != nil
}

func (fn *InoxFunction) String() string {
	if fn.node == nil {
		return "%fn"
	}

	buff := bytes.NewBufferString("%fn(")

	for i, param := range fn.parameters {
		if i != 0 {
			buff.WriteString(", ")
		}
		buff.WriteString(fn.parameterNames[i])
		buff.WriteByte(' ')
		buff.WriteString(param.String())
	}

	buff.WriteString(") ")
	buff.WriteString(fn.returnType.String())

	return buff.String()
}

func (fn *InoxFunction) WidestOfType() SymbolicValue {
	return &InoxFunction{}
}

// A GoFunction represents a symbolic GoFunction.
type GoFunction struct {
	fn          any //if nil, any function is matched
	kind        GoFunctionKind
	originState *State
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

func (fn *GoFunction) Test(v SymbolicValue) bool {
	other, ok := v.(*GoFunction)
	if !ok {
		return false
	}
	if fn.fn == nil {
		return true
	}

	if other.fn == nil {
		return false
	}

	return utils.SamePointer(fn.fn, other.fn)
}

func (fn *GoFunction) IsSharable() (bool, string) {
	// TODO: consider allowing methods & closures
	if fn.kind == GoFunc {
		return true, ""
	}
	return false, "Go function is not sharable because it's a Go method or Go closure"
}

func (fn *GoFunction) Share(originState *State) PotentiallySharable {
	if fn.originState != nil {
		return fn
	}

	copy := *fn
	copy.originState = originState
	return &copy
}

func (fn *GoFunction) IsShared() bool {
	return fn.originState != nil
}

func (fn *GoFunction) Widen() (SymbolicValue, bool) {
	if fn.fn == nil {
		return nil, false
	}
	return &GoFunction{}, true
}

func (fn *GoFunction) IsWidenable() bool {
	return fn.fn != nil
}

func (fn *GoFunction) String() string {
	if fn.fn == nil {
		return "%fn"
	}

	fnValType := reflect.TypeOf(fn.fn)

	isfirstArgCtx := fnValType.NumIn() > 0 && CTX_PTR_TYPE.AssignableTo(fnValType.In(0))
	isVariadic := fnValType.IsVariadic()

	start := 0
	if isfirstArgCtx {
		start++
	}

	buf := bytes.NewBufferString("%fn(")
	for i := start; i < fnValType.NumIn(); i++ {
		if i != start {
			buf.WriteString(", ")
		}

		reflectParamType := fnValType.In(i)

		if i == fnValType.NumIn()-1 && isVariadic {
			buf.WriteString("...%[]")

			param, err := converTypeToSymbolicValue(reflectParamType.Elem())
			if err != nil {
				buf.WriteString("???" + err.Error())
			} else {
				buf.WriteString(param.String())
			}

		} else {
			param, err := converTypeToSymbolicValue(reflectParamType)
			if err != nil {
				buf.WriteString("???" + err.Error())
			} else {
				buf.WriteString(param.String())
			}
		}

	}

	buf.WriteString(") ")

	if fnValType.NumOut() > 1 {
		buf.WriteString("[")
	}

	for i := 0; i < fnValType.NumOut(); i++ {
		if i != 0 {
			buf.WriteString(", ")
		}

		reflectReturnType := fnValType.Out(i)

		ret, err := converTypeToSymbolicValue(reflectReturnType)
		if err != nil {
			buf.WriteString("???" + err.Error())
		} else {
			buf.WriteString(ret.String())
		}
	}

	if fnValType.NumOut() > 1 {
		buf.WriteString("]")
	}

	return buf.String()
}

func (fn *GoFunction) WidestOfType() SymbolicValue {
	return &GoFunction{}
}

type goFunctionCallInput struct {
	symbolicArgs      []SymbolicValue
	nonSpreadArgCount int
	hasSpreadArg      bool
	state, extState   *State
	isExt, must       bool
	callNode          *parse.CallExpression
}

func (goFunc *GoFunction) Call(input goFunctionCallInput) (SymbolicValue, error) {
	if goFunc.fn == nil {
		input.state.addError(makeSymbolicEvalError(input.callNode, input.state, CANNOT_CALL_GO_FUNC_NO_CONCRETE_VALUE))
		return ANY, nil
	}

	fnVal := reflect.ValueOf(goFunc.fn)
	fnValType := fnVal.Type()

	if fnVal.Kind() != reflect.Func {
		return nil, fmt.Errorf("cannot call Go value of kind %s: %#v (%T)", fnVal.Kind(), goFunc.fn, goFunc.fn)
	}

	symbolicArgs := input.symbolicArgs
	nonSpreadArgCount := input.nonSpreadArgCount
	hasSpreadArg := input.hasSpreadArg
	state := input.state
	extState := input.extState
	isExt := input.isExt
	must := input.must
	callNode := input.callNode

	var ctx *Context = state.ctx
	if isExt {
		ctx = extState.ctx
	}

	args := make([]any, len(symbolicArgs))
	for i, e := range symbolicArgs {
		args[i] = e
	}

	isfirstArgCtx := false

	if fnValType.NumIn() == 0 || !CTX_PTR_TYPE.AssignableTo(fnValType.In(0)) {
		//ok
	} else {
		isfirstArgCtx = true
	}

	nonVariadicParamCount := fnValType.NumIn()
	if fnValType.IsVariadic() {
		nonVariadicParamCount -= 1
	}
	if isfirstArgCtx {
		nonVariadicParamCount -= 1
	}

	if fnValType.IsVariadic() {
		if nonSpreadArgCount < nonVariadicParamCount {
			state.addError(makeSymbolicEvalError(callNode, state, fmtInvalidNumberOfNonSpreadArgs(nonSpreadArgCount, nonVariadicParamCount)))
		}
	} else if hasSpreadArg {
		state.addError(makeSymbolicEvalError(callNode, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS))
	} else if len(args) != nonVariadicParamCount {
		state.addError(makeSymbolicEvalError(callNode, state, fmtInvalidNumberOfArgs(nonSpreadArgCount, nonVariadicParamCount)))
		// remove additional arguments

		if len(args) > nonVariadicParamCount {
			args = args[:nonVariadicParamCount]
		}
	}

	if isfirstArgCtx {
		args = append([]any{ctx}, args...)
		nonVariadicParamCount += 1
	}

	//check arguments
	for paramIndex := 0; paramIndex < nonVariadicParamCount; paramIndex++ {
		if paramIndex == 0 && isfirstArgCtx {
			continue
		}

		reflectParamType := fnValType.In(paramIndex)
		param, err := converTypeToSymbolicValue(reflectParamType)
		if err != nil {
			s := fmt.Sprintf("cannot convert one of a Go function parameter (type %s.%s) (function name: %s): %s",
				reflectParamType.PkgPath(), reflectParamType.Name(),
				runtime.FuncForPC(fnVal.Pointer()).Name(),
				err.Error(),
			)
			err = makeSymbolicEvalError(callNode, state, s)
			return nil, err
		}

		var arg SymbolicValue
		if paramIndex < len(args) {
			arg = args[paramIndex].(SymbolicValue)

			// if extVal, ok := arg.(*SharedValue); ok {
			// 	arg = extVal.value
			// }

			widenedArg := arg
			for !isAny(widenedArg) && !param.Test(widenedArg) {
				widenedArg = widenOrAny(widenedArg)
			}

			if !param.Test(widenedArg) {
				position := paramIndex

				if isfirstArgCtx {
					position -= 1
				}

				argNode := callNode.Arguments[position]

				state.addError(makeSymbolicEvalError(argNode, state, FmtInvalidArg(position, arg, param)))
				args[paramIndex] = param //if argument does not match we use the symbolic parameter value as argument
			} else {
				args[paramIndex] = widenedArg
			}
		} else { //if not enough arguments
			args = append(args, param)
		}
	}

	if fnValType.IsVariadic() && len(args) > nonVariadicParamCount {
		variadicArgs := args[nonVariadicParamCount:]
		goVariadicElemType := fnValType.In(fnValType.NumIn() - 1).Elem()
		variadicElemType, err := converTypeToSymbolicValue(goVariadicElemType)
		if err != nil {
			s := fmt.Sprintf("cannot convert a Go function variadic parameter type %s.%s (function name: %s): %s",
				goVariadicElemType.PkgPath(), goVariadicElemType.Name(),
				runtime.FuncForPC(fnVal.Pointer()).Name(),
				err.Error(),
			)
			err = makeSymbolicEvalError(callNode, state, s)
			return nil, err
		}

		for i, arg := range variadicArgs {
			widenedArg := arg.(SymbolicValue)
			for !isAny(widenedArg) && !variadicElemType.Test(widenedArg) {
				widenedArg = widenOrAny(widenedArg)
			}

			if !variadicElemType.Test(widenedArg) {
				position := i + nonVariadicParamCount
				if isfirstArgCtx {
					position -= 1
				}
				state.addError(makeSymbolicEvalError(callNode, state, FmtInvalidArg(position, arg.(SymbolicValue), variadicElemType)))
				variadicArgs[i] = variadicElemType
			} else {
				variadicArgs[i] = widenedArg
			}
		}
	}

	// wrap each argument in a reflect Value
	argValues := make([]reflect.Value, len(args))

	for i, arg := range args {
		//?
		// if extVal, ok := arg.(*SharedValue); ok {
		// 	arg = extVal.value
		// }
		argValue := reflect.ValueOf(arg)
		argValues[i] = argValue
	}

	resultValues := fnVal.Call(argValues)
	resultCount := fnValType.NumOut()

	symbolicResultValues := make([]SymbolicValue, resultCount)

	for i := 0; i < fnValType.NumOut(); i++ {
		var err error

		reflectVal := resultValues[i]

		if reflectVal.IsZero() {
			goResultType := fnValType.Out(i)
			symbolicResultValues[i], err = converTypeToSymbolicValue(goResultType)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot convert one of a Go function result %s.%s (function name: %s): %s",
					goResultType.PkgPath(), goResultType.Name(),
					runtime.FuncForPC(fnVal.Pointer()).Name(),
					err.Error())
			}
		} else {
			symbolicResultValues[i], err = converReflectValToSymbolicValue(reflectVal)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot convert one of a Go function result %s.%s (function name: %s): %s",
					reflectVal.Type().PkgPath(), reflectVal.Type().Name(),
					runtime.FuncForPC(fnVal.Pointer()).Name(),
					err.Error())
			}
		}

	}

	if must && resultCount >= 1 &&
		fnValType.Out(resultCount-1) == ERROR_TYPE {
		//for now we always assume that 'must' calls never panic
		symbolicResultValues = symbolicResultValues[:len(symbolicResultValues)-1]
	}

	switch len(symbolicResultValues) {
	case 0:
		return Nil, nil
	case 1:
		if isExt {
			shared, err := ShareOrClone(symbolicResultValues[0], extState)
			if err != nil {
				state.addError(makeSymbolicEvalError(callNode, state, err.Error()))
				shared = ANY
			}
			return shared, nil
		}

		return symbolicResultValues[0], nil
	}

	var results []SymbolicValue

	if isExt {
		for _, resultValue := range symbolicResultValues {
			shared, err := ShareOrClone(resultValue, extState)
			if err != nil {
				state.addError(makeSymbolicEvalError(callNode, state, err.Error()))
				shared = ANY
			}

			results = append(results, shared)
		}
	} else {
		for _, resultValue := range symbolicResultValues {
			results = append(results, resultValue)
		}
	}

	return NewList(results...), nil
}
