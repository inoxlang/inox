package symbolic

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_INOX_FUNC = &InoxFunction{
		node: nil,
	}
)

// An InoxFunction represents a symbolic InoxFunction.
// TODO: keep in sync with concrete InoxFunction
type InoxFunction struct {
	node           parse.Node //if nil, any function is matched
	parameters     []SymbolicValue
	parameterNames []string
	result         SymbolicValue
	capturedLocals map[string]SymbolicValue
	originState    *State

	SerializableMixin
}

func (fn *InoxFunction) IsVariadic() bool {
	if fn.node == nil {
		panic(errors.New("node is nil"))
	}
	return fn.FuncExpr().IsVariadic
}

func (fn *InoxFunction) Parameters() []SymbolicValue {
	return fn.parameters
}

func (fn *InoxFunction) Result() SymbolicValue {
	if fn.node == nil {
		panic(errors.New("node is nil"))
	}
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

func (fn *InoxFunction) IsConcretizable() bool {
	return false
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

func (fn *InoxFunction) WatcherElement() SymbolicValue {
	return ANY
}

func (fn *InoxFunction) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if fn.node == nil {
		utils.Must(w.Write(utils.StringAsBytes("fn")))
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("fn(")))

	for i, param := range fn.parameters {
		if i != 0 {
			utils.Must(w.Write(utils.StringAsBytes(", ")))
		}
		utils.Must(w.Write(utils.StringAsBytes(fn.parameterNames[i])))
		utils.Must(w.Write(utils.StringAsBytes(" ")))

		if fn.IsVariadic() && i == len(fn.parameters)-1 {
			utils.Must(w.Write(utils.StringAsBytes("...")))
		}
		param.PrettyPrint(w, config, 0, 0)
	}

	utils.Must(w.Write(utils.StringAsBytes(") ")))
	fn.result.PrettyPrint(w, config, 0, 0)
}

func (fn *InoxFunction) WidestOfType() SymbolicValue {
	return &InoxFunction{}
}

// A GoFunction represents a symbolic GoFunction.
type GoFunction struct {
	fn          any //if nil, any function is matched
	kind        GoFunctionKind
	originState *State

	//signature
	signatureDataLoaded   bool
	isVariadic            bool
	isfirstArgCtx         bool
	nonVariadicParameters []SymbolicValue
	parameters            []SymbolicValue
	variadicElem          SymbolicValue
	results               []SymbolicValue
	resultList            *Array
	result                SymbolicValue
}

// the result should not be modified
func (fn *GoFunction) NonVariadicParametersExceptCtx() []SymbolicValue {
	utils.PanicIfErr(fn.LoadSignatureData())
	if fn.isfirstArgCtx {
		return fn.nonVariadicParameters[1:]
	}
	return fn.nonVariadicParameters
}

// the result should not be modified
func (fn *GoFunction) ParametersExceptCtx() []SymbolicValue {
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

func (fn *GoFunction) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if fn.fn == nil {
		utils.Must(w.Write(utils.StringAsBytes("fn")))
	}

	fnValType := reflect.TypeOf(fn.fn)

	isfirstArgCtx := fnValType.NumIn() > 0 && CTX_PTR_TYPE.AssignableTo(fnValType.In(0))
	isVariadic := fnValType.IsVariadic()

	start := 0
	if isfirstArgCtx {
		start++
	}

	utils.Must(w.Write(utils.StringAsBytes("fn(")))

	buf := bytes.NewBufferString("fn(")
	for i := start; i < fnValType.NumIn(); i++ {
		if i != start {
			utils.Must(w.Write(utils.StringAsBytes(", ")))
		}

		reflectParamType := fnValType.In(i)

		if i == fnValType.NumIn()-1 && isVariadic {
			buf.WriteString("...%[]")

			param, err := converTypeToSymbolicValue(reflectParamType.Elem())
			if err != nil {
				buf.WriteString("???" + err.Error())
			} else {
				param.PrettyPrint(w, config, 0, 0)
			}

		} else {
			param, err := converTypeToSymbolicValue(reflectParamType)
			if err != nil {
				buf.WriteString("???" + err.Error())
			} else {
				param.PrettyPrint(w, config, 0, 0)
			}
		}

	}

	utils.Must(w.Write(utils.StringAsBytes(") ")))

	if fnValType.NumOut() > 1 {
		utils.Must(w.Write(utils.StringAsBytes("[")))
	}

	for i := 0; i < fnValType.NumOut(); i++ {
		if i != 0 {
			utils.Must(w.Write(utils.StringAsBytes(", ")))
		}

		reflectReturnType := fnValType.Out(i)

		ret, err := converTypeToSymbolicValue(reflectReturnType)
		if err != nil {
			buf.WriteString("???" + err.Error())
		} else {
			ret.PrettyPrint(w, config, 0, 0)
		}
	}

	if fnValType.NumOut() > 1 {
		utils.Must(w.Write(utils.StringAsBytes("]")))
	}

}

func (fn *GoFunction) WidestOfType() SymbolicValue {
	return &GoFunction{}
}

func (goFunc *GoFunction) Result() SymbolicValue {
	return goFunc.result
}

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

	goFunc.nonVariadicParameters = make([]SymbolicValue, nonVariadicParamCount)
	for paramIndex := 0; paramIndex < nonVariadicParamCount; paramIndex++ {
		if paramIndex == 0 && goFunc.isfirstArgCtx {
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
			return errors.New(s)
		}
		goFunc.nonVariadicParameters[paramIndex] = param
	}

	if fnValType.IsVariadic() {
		goVariadicElemType := fnValType.In(fnValType.NumIn() - 1).Elem()
		variadicElemType, err := converTypeToSymbolicValue(goVariadicElemType)
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
		symbolicResultValue, err := converTypeToSymbolicValue(goResultType)
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
	symbolicArgs      []SymbolicValue
	nonSpreadArgCount int
	hasSpreadArg      bool
	state, extState   *State
	isExt, must       bool
	callLikeNode      parse.Node
}

func (goFunc *GoFunction) Call(input goFunctionCallInput) (finalResult SymbolicValue, multipleResults bool, enoughArgs bool, finalErr error) {
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

	if goFunc.isVariadic {
		if nonSpreadArgCount < inoxLandNonVariadicParamCount {
			state.addError(makeSymbolicEvalError(callLikeNode, state, fmtInvalidNumberOfNonSpreadArgs(nonSpreadArgCount, inoxLandNonVariadicParamCount)))
		}
	} else if hasSpreadArg {
		state.addError(makeSymbolicEvalError(callLikeNode, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS))
	} else if len(args) != inoxLandNonVariadicParamCount {
		state.addError(makeSymbolicEvalError(callLikeNode, state, fmtInvalidNumberOfArgs(nonSpreadArgCount, inoxLandNonVariadicParamCount)))
		// remove additional arguments

		if len(args) > inoxLandNonVariadicParamCount {
			args = args[:inoxLandNonVariadicParamCount]
		}
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

		var arg SymbolicValue
		if paramIndex < len(args) {
			position := paramIndex
			if goFunc.isfirstArgCtx {
				position -= 1
			}

			arg = args[paramIndex].(SymbolicValue)
			argNode := argumentNodes[position]

			// if extVal, ok := arg.(*SharedValue); ok {
			// 	arg = extVal.value
			// }

			if !param.Test(arg) {
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					args[paramIndex] = param
					pattern, ok := extData.SymbolicToPattern(param)
					if ok {
						state.symbolicData.SetRuntimeTypecheckPattern(argNode, pattern)
					} else {
						state.addError(makeSymbolicEvalError(argNode, state, UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK))
					}
				} else {
					state.addError(makeSymbolicEvalError(argNode, state, FmtInvalidArg(position, arg, param)))
				}

				args[paramIndex] = param //if argument does not match we use the symbolic parameter value as argument
			} else {
				//disable runtime type check
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					state.symbolicData.SetRuntimeTypecheckPattern(argNode, nil)
				}
				args[paramIndex] = arg
			}
		} else { //if not enough arguments
			enoughArgs = false
			args = append(args, param)
		}
	}

	if goFunc.isVariadic && len(args) > nonVariadicParamCount {
		variadicArgs := args[nonVariadicParamCount:]

		for i, arg := range variadicArgs {
			if !goFunc.variadicElem.Test(arg.(SymbolicValue)) {
				position := i + nonVariadicParamCount
				if goFunc.isfirstArgCtx {
					position -= 1
				}
				state.addError(makeSymbolicEvalError(callLikeNode, state, FmtInvalidArg(position, arg.(SymbolicValue), goFunc.variadicElem)))
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

	symbolicResultValues := make([]SymbolicValue, resultCount)

	for i := 0; i < fnValType.NumOut(); i++ {
		var err error

		reflectVal := resultValues[i]

		if reflectVal.IsZero() {
			symbolicResultValues[i] = goFunc.results[i]
		} else {
			symbolicResultValues[i], err = converReflectValToSymbolicValue(reflectVal)
			if err != nil {
				return nil, false, enoughArgs, fmt.Errorf(
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

	var results []SymbolicValue

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
	parameters     []SymbolicValue
	parameterNames []string
	results        []SymbolicValue
	variadic       bool

	pattern *FunctionPattern
}

func NewFunction(parameters []SymbolicValue, parameterNames []string, variadic bool, results []SymbolicValue) *Function {
	//TODO: check that variadic parameter is a list

	return &Function{
		parameters:     parameters,
		parameterNames: parameterNames,
		results:        results,
		variadic:       variadic,
	}
}

// returned slice should not be modified.
func (fn *Function) NonVariadicParameters() []SymbolicValue {
	if fn.variadic {
		return fn.parameters[:len(fn.parameters)-1]
	}
	return fn.parameters
}

func (fn *Function) VariadicParamElem() SymbolicValue {
	if !fn.variadic {
		panic(errors.New("function is not variadic"))
	}
	param := fn.parameters[len(fn.parameters)-1]
	return param.(*List).IteratorElementValue()
}

func (f *Function) Test(v SymbolicValue) bool {
	if f.pattern != nil {
		switch v.(type) {
		case *Function, *GoFunction, *InoxFunction:
			return f.pattern.TestValue(v)
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
			if !param.Test(fn.parameters[i]) || !fn.parameters[i].Test(param) {
				return false
			}
		}

		for i, result := range f.results {
			if !result.Test(fn.results[i]) || !fn.results[i].Test(result) {
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
		if inoxFn.node == nil || f.variadic != inoxFn.IsVariadic() || len(f.parameters) != len(inoxFn.parameters) {
			return false
		}

		for i, param := range f.parameters {
			if !deeplyEqual(param, inoxFn.parameters[i]) {
				return false
			}
		}

		var result SymbolicValue
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

func (f *Function) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if f.pattern != nil {
		utils.Must(w.Write(utils.StringAsBytes("%function(???)")))
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("fn(")))

	for i, param := range f.parameters {
		if i != 0 {
			utils.Must(w.Write(utils.StringAsBytes(", ")))
		}

		if f.variadic && i == len(f.parameters)-1 {
			utils.Must(w.Write(utils.StringAsBytes("...")))
		}

		if len(f.parameterNames) > i {
			utils.Must(w.Write(utils.StringAsBytes(f.parameterNames[i])))
			utils.PanicIfErr(w.WriteByte(' '))
		}

		param.PrettyPrint(w, config, 0, 0)
	}

	utils.Must(w.Write(utils.StringAsBytes(") ")))
	switch len(f.results) {
	case 0:
	case 1:
		f.results[0].PrettyPrint(w, config, 0, 0)
	default:
		NewArray(f.results...).PrettyPrint(w, config, 0, 0)
	}
}

func (f *Function) WidestOfType() SymbolicValue {
	return &Function{
		pattern: (&FunctionPattern{}).WidestOfType().(*FunctionPattern),
	}
}
