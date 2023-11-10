package symbolic

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	PRETTY_PRINT_OPTIONAL_PARAM_PREFIX = "optional "
)

var (
	ANY_INOX_FUNC = &InoxFunction{}
)

// An InoxFunction represents a symbolic InoxFunction.
// TODO: keep in sync with concrete InoxFunction
type InoxFunction struct {
	node           parse.Node //optional but required for call evaluation
	nodeChunk      *parse.Chunk
	parameters     []Value
	parameterNames []string
	noNodeVariadic bool
	result         Value //if nil any function is matched
	capturedLocals map[string]Value
	originState    *State

	//optional, should not be present if node is not present
	visitCheckNode    func(visit visitArgs, globalsAtCreation map[string]Value) (parse.TraversalAction, bool, error)
	globalsAtCreation map[string]Value

	SerializableMixin
}

type visitArgs struct {
	node, parent, scopeNode parse.Node
	ancestorChain           []parse.Node
	after                   bool
}

func NewInoxFunction(parameters map[string]Value, capturedLocals map[string]Value, result Value) *InoxFunction {
	fn := &InoxFunction{
		capturedLocals: capturedLocals,
		result:         result,
	}

	for name, val := range parameters {
		fn.parameterNames = append(fn.parameterNames, name)
		fn.parameters = append(fn.parameters, val)
	}

	return fn
}

func (fn *InoxFunction) IsVariadic() bool {
	if fn.node == nil {
		return fn.noNodeVariadic
	}
	return fn.FuncExpr().IsVariadic
}

func (fn *InoxFunction) Parameters() []Value {
	return fn.parameters
}

func (fn *InoxFunction) Result() Value {
	return fn.result
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

func (fn *InoxFunction) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*InoxFunction)
	if !ok {
		return false
	}

	if fn.result == nil {
		return true
	}

	if fn.visitCheckNode != nil {
		if other.node == nil {
			//impossible to check
			return false
		}
		atLeastOneNodeNotAllowed := false

		body := other.FuncExpr().Body

		parse.Walk(
			body,
			func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if _, isBody := node.(*parse.Block); isBody && node == body {
					return parse.ContinueTraversal, nil
				}

				action, allowed, err := fn.visitCheckNode(visitArgs{node, parent, scopeNode, ancestorChain, after}, fn.capturedLocals)
				if err != nil {
					return parse.StopTraversal, err
				}
				if !allowed {
					atLeastOneNodeNotAllowed = true
					return parse.StopTraversal, nil
				}
				return action, nil
			},
			nil,
		)

		if atLeastOneNodeNotAllowed {
			return false
		}
	}

	if (fn.node != nil && other.node == nil) ||
		(fn.node != nil && !utils.SamePointer(fn.node, other.node)) ||
		other.result == nil ||
		(len(fn.parameters) != len(other.parameters)) ||
		(len(fn.capturedLocals) != len(other.capturedLocals)) ||
		fn.originState != other.originState {
		return false
	}

	for i, paramVal := range fn.parameters {
		otherParamVal := other.parameters[i]
		if !deeplyEqual(paramVal, otherParamVal) {
			return false
		}
	}

	for name, val := range fn.capturedLocals {
		otherVal, found := other.capturedLocals[name]
		if !found || !deeplyEqual(val, otherVal) {
			return false
		}
	}

	return fn.result.Test(other.result, state)
}

func (fn *InoxFunction) IsConcretizable() bool {
	return false
}

func (fn *InoxFunction) Concretize(ctx ConcreteContext) any {
	panic(ErrNotConcretizable)
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

func (fn *InoxFunction) WatcherElement() Value {
	return ANY
}

func (fn *InoxFunction) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if fn.visitCheckNode != nil {
		w.WriteString("[restricted stmts] ")
	}
	if fn.result == nil {
		w.WriteString("fn")
		return
	}

	w.WriteString("fn(")

	for i, param := range fn.parameters {
		if i != 0 {
			w.WriteString(", ")
		}
		w.WriteString(fn.parameterNames[i])
		w.WriteString(" ")

		if fn.IsVariadic() && i == len(fn.parameters)-1 {
			w.WriteString("...")
		}
		param.PrettyPrint(w.ZeroDepthIndent(), config)
	}

	w.WriteString(") ")
	fn.result.PrettyPrint(w.ZeroDepthIndent(), config)
}

func (fn *InoxFunction) WidestOfType() Value {
	return ANY_INOX_FUNC
}

// A GoFunction represents a symbolic GoFunction.
type GoFunction struct {
	fn          any //if nil, any function is matched
	kind        GoFunctionKind
	originState *State

	//signature fields:

	signatureDataLoaded bool
	isVariadic          bool
	isfirstArgCtx       bool

	//if >= 0 the next parameter is either optional or variadic,
	//the ctx param is taken into account.
	lastMandatoryParamIndex int

	//true if the function has at least one OptionalParam[T] in its parameters.
	hasOptionalParams bool
	optionalParams    []optionalParam

	nonVariadicParameters []Value
	parameters            []Value

	variadicElem Value
	results      []Value
	resultList   *Array
	result       Value
}

// the result should not be modified
func (fn *GoFunction) NonVariadicParametersExceptCtx() []Value {
	utils.PanicIfErr(fn.LoadSignatureData())
	if fn.isfirstArgCtx {
		return fn.nonVariadicParameters[1:]
	}
	return fn.nonVariadicParameters
}

// the result should not be modified
func (fn *GoFunction) ParametersExceptCtx() []Value {
	utils.PanicIfErr(fn.LoadSignatureData())
	if fn.isfirstArgCtx {
		return fn.parameters[1:]
	}
	return fn.parameters
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

func (fn *GoFunction) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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

func (fn *GoFunction) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if fn.fn == nil {
		w.WriteName("fn")
	}

	fnValType := reflect.TypeOf(fn.fn)

	isfirstArgCtx := fnValType.NumIn() > 0 && CTX_PTR_TYPE.AssignableTo(fnValType.In(0))
	isVariadic := fnValType.IsVariadic()

	start := 0
	if isfirstArgCtx {
		start++
	}

	w.WriteName("fn(")

	for i := start; i < fnValType.NumIn(); i++ {
		if i != start {
			w.WriteString(", ")
		}

		reflectParamType := fnValType.In(i)

		if i == fnValType.NumIn()-1 && isVariadic {
			w.WriteString("...%[]")

			param, _, err := converTypeToSymbolicValue(reflectParamType.Elem(), false)
			if err != nil {
				w.WriteString("???" + err.Error())
			} else {
				param.PrettyPrint(w.ZeroDepthIndent(), config)
			}

		} else {
			allowOptionalParams := i > fn.lastMandatoryParamIndex

			param, isOptionalParam, err := converTypeToSymbolicValue(reflectParamType, allowOptionalParams)
			if err != nil {
				w.WriteString("???" + err.Error())
			} else {
				if isOptionalParam {
					w.WriteString(PRETTY_PRINT_OPTIONAL_PARAM_PREFIX)
				}
				param.PrettyPrint(w.ZeroDepthIndent(), config)
			}
		}

	}

	w.WriteString(") ")

	if fnValType.NumOut() > 1 {
		w.WriteString("[")
	}

	for i := 0; i < fnValType.NumOut(); i++ {
		if i != 0 {
			w.WriteString(", ")
		}

		reflectReturnType := fnValType.Out(i)

		ret, _, err := converTypeToSymbolicValue(reflectReturnType, false)
		if err != nil {
			w.WriteString("???" + err.Error())
		} else {
			ret.PrettyPrint(w.ZeroDepthIndent(), config)
		}
	}

	if fnValType.NumOut() > 1 {
		w.WriteString("]")
	}

}

func (fn *GoFunction) WidestOfType() Value {
	return &GoFunction{}
}

func (goFunc *GoFunction) Result() Value {
	return goFunc.result
}

// LoadSignatureData populates the signature fields if they are not already set.
func (goFunc *GoFunction) LoadSignatureData() (finalErr error) {
	if goFunc.signatureDataLoaded {
		return nil
	}

	if goFunc.fn == nil {
		panic(errors.New("function is nil"))
	}

	defer func() {
		if finalErr == nil {
			goFunc.signatureDataLoaded = true
		}
	}()

	fnVal := reflect.ValueOf(goFunc.fn)
	fnValType := fnVal.Type()
	goFunc.isVariadic = fnValType.IsVariadic()

	if fnVal.Kind() != reflect.Func {
		return fmt.Errorf("cannot call Go value of kind %s: %#v (%T)", fnVal.Kind(), goFunc.fn, goFunc.fn)
	}

	if fnValType.NumIn() == 0 || !CTX_PTR_TYPE.AssignableTo(fnValType.In(0)) {
		//ok
	} else {
		goFunc.isfirstArgCtx = true
	}

	nonVariadicParamCount := fnValType.NumIn()
	if goFunc.isVariadic {
		nonVariadicParamCount -= 1
	}

	goFunc.nonVariadicParameters = make([]Value, nonVariadicParamCount)
	for paramIndex := 0; paramIndex < nonVariadicParamCount; paramIndex++ {
		if paramIndex == 0 && goFunc.isfirstArgCtx {
			continue
		}

		reflectParamType := fnValType.In(paramIndex)
		param, isOptionalParam, err := converTypeToSymbolicValue(reflectParamType, true)
		if err != nil {
			s := fmt.Sprintf("cannot convert one of a Go function parameter (type %s.%s) (function name: %s): %s",
				reflectParamType.PkgPath(), reflectParamType.Name(),
				runtime.FuncForPC(fnVal.Pointer()).Name(),
				err.Error(),
			)
			return errors.New(s)
		}

		if isOptionalParam {
			//the ctx param is required because otherwise lastMandatoryParamIndex
			// would have a value of -1 if the param after the ctx was optional.
			if !goFunc.isfirstArgCtx {
				return errors.New("symbolic Go function with at least one optional parameter must have *Context as the first parameter")
			}

			if !goFunc.hasOptionalParams {
				goFunc.lastMandatoryParamIndex = paramIndex - 1
				goFunc.hasOptionalParams = true
			}
			//reflectParamType should be an *OptionalParam[T] type.
			param := reflect.New(reflectParamType.Elem()).Interface()
			goFunc.optionalParams = append(goFunc.optionalParams, param.(optionalParam))
		} else if goFunc.hasOptionalParams {
			return fmt.Errorf("go function has an unexpected non optional parameter after an optional parameter, index (%d)", paramIndex)
		}

		goFunc.nonVariadicParameters[paramIndex] = param
	}

	if fnValType.IsVariadic() {
		goVariadicElemType := fnValType.In(fnValType.NumIn() - 1).Elem()
		variadicElemType, _, err := converTypeToSymbolicValue(goVariadicElemType, false)
		if err != nil {
			s := fmt.Sprintf("cannot convert a Go function variadic parameter type %s.%s (function name: %s): %s",
				goVariadicElemType.PkgPath(), goVariadicElemType.Name(),
				runtime.FuncForPC(fnVal.Pointer()).Name(),
				err.Error(),
			)
			return errors.New(s)
		}
		goFunc.variadicElem = variadicElemType
		goFunc.parameters = append(goFunc.nonVariadicParameters, NewArray(goFunc.variadicElem))
	} else {
		goFunc.parameters = goFunc.nonVariadicParameters
	}

	for i := 0; i < fnValType.NumOut(); i++ {
		goResultType := fnValType.Out(i)
		symbolicResultValue, _, err := converTypeToSymbolicValue(goResultType, false)
		if err != nil {
			return fmt.Errorf(
				"cannot convert one of a Go function result %s.%s (function name: %s): %s",
				goResultType.PkgPath(), goResultType.Name(),
				runtime.FuncForPC(fnVal.Pointer()).Name(),
				err.Error())
		}

		if _, isErr := symbolicResultValue.(*Error); isErr {
			symbolicResultValue = NewMultivalue(symbolicResultValue, Nil)
		}

		goFunc.results = append(goFunc.results, symbolicResultValue)
	}
	goFunc.resultList = NewArray(goFunc.results...)

	switch len(goFunc.resultList.elements) {
	case 0:
		goFunc.result = Nil
	case 1:
		goFunc.result = goFunc.resultList.elementAt(0)
		//TODO: handle shared ?
	default:
		goFunc.result = goFunc.resultList
	}

	return nil
}

type goFunctionCallInput struct {
	symbolicArgs      []Value
	nonSpreadArgCount int
	hasSpreadArg      bool
	state, extState   *State
	isExt, must       bool
	callLikeNode      parse.Node
}

func (goFunc *GoFunction) Call(input goFunctionCallInput) (finalResult Value, multipleResults bool, enoughArgs bool, finalErr error) {
	if goFunc.fn == nil {
		input.state.addError(makeSymbolicEvalError(input.callLikeNode, input.state, CANNOT_CALL_GO_FUNC_NO_CONCRETE_VALUE))
		return ANY, false, false, nil
	}

	symbolicArgs := input.symbolicArgs
	nonSpreadArgCount := input.nonSpreadArgCount
	hasSpreadArg := input.hasSpreadArg
	state := input.state
	extState := input.extState
	isExt := input.isExt
	must := input.must
	callLikeNode := input.callLikeNode
	enoughArgs = true

	if err := goFunc.LoadSignatureData(); err != nil {
		err = makeSymbolicEvalError(callLikeNode, state, err.Error())
		return nil, false, false, err
	}

	var ctx *Context = state.ctx
	if isExt {
		ctx = extState.ctx
	}

	args := make([]any, len(symbolicArgs))
	for i, e := range symbolicArgs {
		args[i] = e
	}

	nonVariadicParamCount := len(goFunc.nonVariadicParameters)
	inoxLandNonVariadicParamCount := nonVariadicParamCount

	if goFunc.isfirstArgCtx {
		inoxLandNonVariadicParamCount -= 1
	}

	//only check if goFunc.hasOptionalParams
	inoxLandMandatoryParamCount := inoxLandNonVariadicParamCount - len(goFunc.optionalParams)

	if goFunc.isVariadic {
		var errMsg string
		if nonSpreadArgCount < inoxLandNonVariadicParamCount && !goFunc.hasOptionalParams {
			errMsg = fmtInvalidNumberOfNonSpreadArgs(nonSpreadArgCount, inoxLandNonVariadicParamCount)
		} else if goFunc.hasOptionalParams && nonSpreadArgCount < inoxLandMandatoryParamCount {
			errMsg = fmtInvalidNumberOfNonArgsAtLeastMandatoryMax(nonSpreadArgCount, inoxLandMandatoryParamCount, inoxLandNonVariadicParamCount)
		}

		if errMsg != "" {
			state.addError(makeSymbolicEvalError(callLikeNode, state, errMsg))
		}

	} else if hasSpreadArg {
		state.addError(makeSymbolicEvalError(callLikeNode, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS))

	} else if len(args) > inoxLandNonVariadicParamCount { //too many arguments
		state.addError(makeSymbolicEvalError(callLikeNode, state, fmtTooManyArgs(nonSpreadArgCount, inoxLandNonVariadicParamCount)))
		// remove additional arguments
		args = args[:inoxLandNonVariadicParamCount]

	} else if !goFunc.hasOptionalParams && len(args) < inoxLandNonVariadicParamCount { //not enough arguments
		errMsg := fmtNotEnoughArgs(nonSpreadArgCount, inoxLandNonVariadicParamCount)
		state.addError(makeSymbolicEvalError(callLikeNode, state, errMsg))

	} else if goFunc.hasOptionalParams && len(args) < inoxLandMandatoryParamCount { //not enough arguments

		errMsg := fmtNotEnoughArgsAtLeastMandatoryMax(nonSpreadArgCount, inoxLandMandatoryParamCount, inoxLandNonVariadicParamCount)
		state.addError(makeSymbolicEvalError(callLikeNode, state, errMsg))
	}

	if goFunc.isfirstArgCtx {
		args = append([]any{ctx}, args...)
	}

	//check arguments
	for paramIndex := 0; paramIndex < nonVariadicParamCount; paramIndex++ {
		if paramIndex == 0 && goFunc.isfirstArgCtx {
			continue
		}

		param := goFunc.nonVariadicParameters[paramIndex]

		// check argument against the parameter's type
		var argumentNodes []parse.Node

		switch c := callLikeNode.(type) {
		case *parse.CallExpression:
			argumentNodes = c.Arguments
		case *parse.XMLExpression:
			argumentNodes = []parse.Node{c.Element}
		}

		var arg Value
		if paramIndex >= len(args) {
			if !goFunc.hasOptionalParams || paramIndex <= goFunc.lastMandatoryParamIndex {
				enoughArgs = false
				args = append(args, param)
			} else { //optional parameter
				//wrap the argument in its corresponding OptionalParam[T].
				index := paramIndex - goFunc.lastMandatoryParamIndex - 1
				optionalParam := goFunc.optionalParams[index].new()
				args = append(args, optionalParam)
			}
		} else {
			position := paramIndex
			if goFunc.isfirstArgCtx {
				position -= 1
			}

			arg = args[paramIndex].(Value)
			argNode := argumentNodes[position]
			setOptionalParamValue := false

			// if extVal, ok := arg.(*SharedValue); ok {
			// 	arg = extVal.value
			// }

			if !param.Test(arg, RecTestCallState{}) {
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					args[paramIndex] = param
					pattern, ok := extData.SymbolicToPattern(param)
					if ok {
						state.symbolicData.SetRuntimeTypecheckPattern(argNode, pattern)
					} else {
						state.addError(makeSymbolicEvalError(argNode, state, UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK))
					}
				} else {
					// if the parameter is optional and the value is nil
					// } else if goFunc.hasOptionalParams && paramIndex > goFunc.lastMandatoryParamIndex && Nil.Test(arg, RecTestCallState{}) {
					//}

					state.addError(makeSymbolicEvalError(argNode, state, FmtInvalidArg(position, arg, param)))
					args[paramIndex] = param //if argument does not match we use the symbolic parameter value as argument
				}
			} else {
				//disable runtime type check
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					state.symbolicData.SetRuntimeTypecheckPattern(argNode, nil)
				}
				args[paramIndex] = arg
				setOptionalParamValue = true
			}

			//if the parameter is optional wrap it in its corresponding OptionalParam[T].
			if goFunc.hasOptionalParams && paramIndex > goFunc.lastMandatoryParamIndex {
				index := paramIndex - goFunc.lastMandatoryParamIndex - 1
				optionalParam := goFunc.optionalParams[index].new()

				if setOptionalParamValue {
					optionalParam.setValue(args[paramIndex].(Value))
				}

				args[paramIndex] = optionalParam
			}
		}
	}

	if goFunc.isVariadic && len(args) > nonVariadicParamCount {
		variadicArgs := args[nonVariadicParamCount:]

		for i, arg := range variadicArgs {
			if !goFunc.variadicElem.Test(arg.(Value), RecTestCallState{}) {
				position := i + nonVariadicParamCount
				if goFunc.isfirstArgCtx {
					position -= 1
				}
				state.addError(makeSymbolicEvalError(callLikeNode, state, FmtInvalidArg(position, arg.(Value), goFunc.variadicElem)))
				variadicArgs[i] = goFunc.variadicElem
			} else {
				variadicArgs[i] = arg
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

	fnVal := reflect.ValueOf(goFunc.fn)
	fnValType := reflect.TypeOf(goFunc.fn)

	resultValues := fnVal.Call(argValues)
	resultCount := fnValType.NumOut()

	symbolicResultValues := make([]Value, resultCount)

	for i := 0; i < fnValType.NumOut(); i++ {
		reflectVal := resultValues[i]

		if reflectVal.IsZero() {
			symbolicResultValues[i] = goFunc.results[i]
		} else {
			symbolicVal, ok := reflectVal.Interface().(Value)
			if !ok {
				return nil, false, enoughArgs, fmt.Errorf(
					"cannot convert one of a Go function result %s.%s (function name: %s): "+
						"cannot convert value of following type to symbolic value : %T",
					reflectVal.Type().PkgPath(), reflectVal.Type().Name(),
					runtime.FuncForPC(fnVal.Pointer()).Name(),
					reflectVal.Interface())
			}

			symbolicResultValues[i] = symbolicVal
		}

	}

	if must && resultCount >= 1 &&
		fnValType.Out(resultCount-1) == ERROR_TYPE {
		//for now we always assume that 'must' calls never panic
		symbolicResultValues = symbolicResultValues[:len(symbolicResultValues)-1]
	}

	switch len(symbolicResultValues) {
	case 0:
		return Nil, false, enoughArgs, nil
	case 1:
		if isExt {
			shared, err := ShareOrClone(symbolicResultValues[0], extState)
			if err != nil {
				state.addError(makeSymbolicEvalError(callLikeNode, state, err.Error()))
				shared = ANY
			}
			return shared, false, enoughArgs, nil
		}

		return symbolicResultValues[0], false, enoughArgs, nil
	}

	var results []Value

	if isExt {
		for _, resultValue := range symbolicResultValues {
			shared, err := ShareOrClone(resultValue, extState)
			if err != nil {
				state.addError(makeSymbolicEvalError(callLikeNode, state, err.Error()))
				shared = ANY
			}

			results = append(results, shared)
		}
	} else {
		results = append(results, symbolicResultValues...)
	}

	return NewArray(results...), true, enoughArgs, nil
}

// An Function represents a symbolic function we do not know the concrete type.
type Function struct {
	//if pattern is nil this function matches any function with the following parameters & results
	parameters              []Value
	firstOptionalParamIndex int //-1 if no optional parameters
	parameterNames          []string
	results                 []Value
	variadic                bool

	pattern *FunctionPattern
}

func NewFunction(
	params []Value,
	paramNames []string,
	//should have a value of -1 if there are no optional parameters
	firstOptionalParamIndex int,
	variadic bool,
	results []Value,
) *Function {
	//TODO: check that variadic parameter is a list

	if firstOptionalParamIndex < 0 {
		firstOptionalParamIndex = -1
	}

	fn := &Function{
		parameters:              params,
		firstOptionalParamIndex: firstOptionalParamIndex,
		parameterNames:          paramNames,
		results:                 results,
		variadic:                variadic,
	}

	return fn
}

// returned slice should not be modified.
func (fn *Function) NonVariadicParameters() []Value {
	if fn.variadic {
		return fn.parameters[:len(fn.parameters)-1]
	}
	return fn.parameters
}

func (fn *Function) IsVariadic() bool {
	return fn.variadic
}

func (fn *Function) HasOptionalParams() bool {
	return fn.firstOptionalParamIndex >= 0
}

func (fn *Function) VariadicParamElem() Value {
	if !fn.variadic {
		panic(errors.New("function is not variadic"))
	}
	param := fn.parameters[len(fn.parameters)-1]
	return param.(*List).IteratorElementValue()
}

func (f *Function) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	if f.pattern != nil {
		switch v.(type) {
		case *Function, *GoFunction, *InoxFunction:
			return f.pattern.TestValue(v, state)
		default:
			return false
		}
	}

	switch fn := v.(type) {
	case *Function:
		if fn.pattern != nil || len(f.parameters) != len(fn.parameters) || f.variadic != fn.variadic {
			return false
		}

		for i, param := range f.parameters {
			if !param.Test(fn.parameters[i], state) || !fn.parameters[i].Test(param, state) {
				return false
			}
		}

		for i, result := range f.results {
			if !result.Test(fn.results[i], state) || !fn.results[i].Test(result, state) {
				return false
			}
		}

		return true
	case *GoFunction:
		goFunc := fn
		fnNonVariadicParams := fn.NonVariadicParametersExceptCtx()

		if f.variadic != goFunc.isVariadic || len(fnNonVariadicParams) != len(f.NonVariadicParameters()) ||
			len(f.results) != len(goFunc.results) {
			return false
		}

		for i, param := range f.NonVariadicParameters() {
			if !deeplyEqual(param, fnNonVariadicParams[i]) {
				return false
			}
		}

		variadicParamElem := f.VariadicParamElem()

		if !deeplyEqual(variadicParamElem, goFunc.variadicElem) {
			return false
		}

		for i, result := range f.results {
			if !deeplyEqual(result, goFunc.results[i]) {
				return false
			}
		}

		return true
	case *InoxFunction:
		inoxFn := fn
		if inoxFn.result == nil || f.variadic != inoxFn.IsVariadic() || len(f.parameters) != len(inoxFn.parameters) {
			return false
		}

		for i, param := range f.parameters {
			if !deeplyEqual(param, inoxFn.parameters[i]) {
				return false
			}
		}

		var result Value
		switch len(f.results) {
		case 0:
			_, ok := inoxFn.result.(*NilT)
			return ok
		case 1:
			result = f.results[0]
		default:
			result = NewArray(f.results...)
		}
		return deeplyEqual(result, inoxFn.result)
	default:
		return false
	}
}

func (f *Function) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if f.pattern != nil {
		w.WriteName("function(???)")
		return
	}

	w.WriteString("fn(")

	for i, param := range f.parameters {
		if i != 0 {
			w.WriteString(", ")
		}

		isVariadicParam := f.variadic && i == len(f.parameters)-1
		if isVariadicParam {
			w.WriteString("...")
		}

		if len(f.parameterNames) > i {
			w.WriteString(f.parameterNames[i])
			w.WriteByte(' ')
		}

		if !isVariadicParam && f.HasOptionalParams() && i >= f.firstOptionalParamIndex {
			w.WriteString(PRETTY_PRINT_OPTIONAL_PARAM_PREFIX)
		}

		param.PrettyPrint(w.ZeroDepthIndent(), config)
	}

	w.WriteString(") ")
	switch len(f.results) {
	case 0:
	case 1:
		f.results[0].PrettyPrint(w.ZeroDepthIndent(), config)
	default:
		NewArray(f.results...).PrettyPrint(w.ZeroDepthIndent(), config)
	}
}

func (f *Function) WidestOfType() Value {
	return &Function{
		pattern: (&FunctionPattern{}).WidestOfType().(*FunctionPattern),
	}
}
