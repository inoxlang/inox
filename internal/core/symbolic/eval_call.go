package symbolic

import (
	"fmt"
	"sort"

	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

type inoxCallInfo struct {
	calleeFnExpr       *parse.FunctionExpression
	callNode           *parse.CallExpression //nil if is initial check call
	isInitialCheckCall bool
}

func callSymbolicFunc(callNode *parse.CallExpression, calleeNode parse.Node, state *State, argNodes []parse.Node, must bool, cmdLineSyntax bool) (Value, error) {
	var (
		callee          Value
		err             error
		self            Value
		selfPartialNode parse.Node //not necessarily a node from the evaluated code
	)

	//we first get the callee
	switch c := calleeNode.(type) {
	case *parse.IdentifierLiteral, *parse.IdentifierMemberExpression,
		*parse.Variable, *parse.MemberExpression, *parse.DoubleColonExpression:
		callee, err = _symbolicEval(callNode.Callee, state, evalOptions{
			doubleColonExprAncestorChain: []parse.Node{callNode},
		})
		if err != nil {
			return nil, err
		}
		switch _c := calleeNode.(type) {
		case *parse.IdentifierMemberExpression:
			switch len(_c.PropertyNames) {
			case 0:
				self = ANY
			case 1:
				self, _ = state.symbolicData.GetMostSpecificNodeValue(_c.Left)
				selfPartialNode = _c.Left
			default:
				self, _ = state.symbolicData.GetMostSpecificNodeValue(_c.PropertyNames[len(_c.PropertyNames)-2])
				selfPartialNode = &parse.IdentifierMemberExpression{
					Left:          _c.Left,
					PropertyNames: _c.PropertyNames[:len(_c.PropertyNames)-1],
				}
			}
		case *parse.MemberExpression:
			self, _ = state.symbolicData.GetMostSpecificNodeValue(_c.Left)
			selfPartialNode = _c.Left
		case *parse.DoubleColonExpression:
			_, ok := state.symbolicData.GetURLReferencedEntity(_c)
			if ok {
				state.addError(MakeSymbolicEvalError(_c.Element, state, DIRECTLY_CALLING_METHOD_OF_URL_REF_ENTITY_NOT_ALLOWED))
			} else {
				self, _ = state.symbolicData.GetMostSpecificNodeValue(_c.Left)
				selfPartialNode = _c.Left
			}
		default:
			selfPartialNode = c
		}
	case *parse.FunctionDeclaration, *parse.FunctionExpression:
		callee = &AstNode{Node: c}
	default:
		return nil, fmt.Errorf("(symbolic) cannot call a(n) %T", c)
	}

	var extState *State
	isSharedFunction := false
	var nonGoParameters []Value
	var argMismatches []bool
	isGoFunc := false

	inoxFunctionToBeDeclared, ok := callee.(*inoxFunctionToBeDeclared)
	if ok {
		//Properly declare the function
		_, err := evalFunctionDeclaration(inoxFunctionToBeDeclared.decl, state, evalOptions{})
		if err != nil {
			return ANY, fmt.Errorf("error while evaluating the function declaration: %w", err)
		}

		varInfo, ok := state.getGlobal(inoxFunctionToBeDeclared.decl.Name.(*parse.IdentifierLiteral).Name)
		if !ok {
			return ANY, fmt.Errorf("error while evaluating the function declaration: %w", err)
		}
		callee = varInfo.value.(*InoxFunction)
	}

	if inoxFn, ok := callee.(*InoxFunction); ok {
		isSharedFunction = inoxFn.IsShared()
		if isSharedFunction {
			extState = inoxFn.originState
		}
		nonGoParameters = inoxFn.parameters
	} else if goFn, ok := callee.(*GoFunction); ok {
		isSharedFunction = goFn.IsShared()
		if isSharedFunction {
			extState = goFn.originState
		}
		isGoFunc = true
	} else if function, ok := callee.(*Function); ok {
		nonGoParameters = function.parameters
	} else {
		state.addError(MakeSymbolicEvalError(calleeNode, state, fmtCannotCall(callee)))
		return ANY, nil
	}

	//Evaluate arguments.
	//Expected values are passed to _symbolicEval for non-spread argument nodes.

	args := make([]Value, 0)
	nonSpreadArgCount := 0
	hasSpreadArg := false
	var spreadArgNode parse.Node

	errCountBeforeEvaluationOfArguments := len(state.errors())

	for argIndex, argNode := range argNodes {

		if spreadArg, ok := argNode.(*parse.SpreadArgument); ok {
			hasSpreadArg = true
			spreadArgNode = argNode
			v, err := symbolicEval(spreadArg.Expr, state)
			if err != nil {
				return nil, err
			}

			iterable, ok := v.(Iterable)

			if ok {
				var elements []Value

				indexable, ok := v.(Indexable)
				if ok && indexable.HasKnownLen() {
					for i := 0; i < indexable.KnownLen(); i++ {
						elements = append(elements, indexable.ElementAt(i))
					}
				} else { //add single element
					elements = append(elements, iterable.IteratorElementValue())
				}

				for _, e := range elements {
					//same logic for non spread arguments
					if isSharedFunction {
						shared, err := ShareOrClone(e, state)
						if err != nil {
							state.addError(MakeSymbolicEvalError(argNode, state, err.Error()))
							shared = ANY
						}
						e = shared.(Serializable)
					}
					args = append(args, e)
				}
			} else {
				state.addError(MakeSymbolicEvalError(argNode, state, fmtSpreadArgumentShouldBeIterable(v)))
			}

		} else { //Regular argument.
			nonSpreadArgCount++

			if ident, ok := argNode.(*parse.IdentifierLiteral); ok && cmdLineSyntax { //Identifier literal interpreted as an identifier value.
				args = append(args, &Identifier{name: ident.Name})

				//add warning if the identifier has the same name as a variable
				if state.hasLocal(ident.Name) || state.hasGlobal(ident.Name) {
					addWarning := false

					if calleeIdent, ok := calleeNode.(*parse.IdentifierLiteral); !ok {
						addWarning = true
					} else if calleeIdent.Name != globalnames.EXEC_FN &&
						calleeIdent.Name != globalnames.HELP_FN &&
						!slices.Contains(state.shellTrustedCommands, calleeIdent.Name) {
						addWarning = true
					}

					if addWarning {
						state.addWarning(makeSymbolicEvalWarning(argNode, state, fmtDidYouMeanDollarNameInCLI(ident.Name)))
					}
				}

			} else { //Argument interpreted in the regular way.
				//we assume that Go functions don't modify their arguments so
				//we are (almost) certain that the object will not get additional properties.
				//TODO: track Go functions that mutate their arguments.
				//TODO: for Inox function calls set forceExactObjectLiteral to true if the expected argument is a readonly object.
				options := evalOptions{neverModifiedArgument: isGoFunc}

				if len(nonGoParameters) > 0 && argIndex < len(nonGoParameters) {
					options.expectedValue = nonGoParameters[argIndex]

					argMismatches = append(argMismatches, false)
					if len(argMismatches) != argIndex+1 {
						panic(parse.ErrUnreachable)
					}
					options.actualValueMismatch = &argMismatches[argIndex]
				}

				arg, err := _symbolicEval(argNode, state, options)
				if err != nil {
					return nil, err
				}
				if isSharedFunction {
					shared, err := ShareOrClone(arg, state)
					if err != nil {
						state.addError(MakeSymbolicEvalError(argNode, state, err.Error()))
						shared = ANY
					}
					arg = shared
				}
				args = append(args, arg)
			}
		}

	}

	errorsInArguments := len(state.errors()) - errCountBeforeEvaluationOfArguments

	//Execution

	var fnExpr *parse.FunctionExpression
	var capturedLocals map[string]Value

	switch f := callee.(type) {
	case *InoxFunction:
		//For Inox functions we do not care about errors in arguments since incorrect argument values
		//are replaced by correct ones, and additional errors are reported at the corresponding positions
		//(argument nodes).

		if f.node == nil {
			state.addError(MakeSymbolicEvalError(callNode, state, CALLEE_HAS_NODE_BUT_NOT_DEFINED))
			return ANY, nil
		} else {

			capturedLocals = f.capturedLocals

			switch function := f.node.(type) {
			case *parse.FunctionExpression:
				fnExpr = function
			case *parse.FunctionDeclaration:
				fnExpr = function.Function
			default:
				state.addError(MakeSymbolicEvalError(callNode, state, fmtCannotCallNode(f.node)))
				return ANY, nil
			}
		}
		//evaluation of the Inox function is performed further in the code
	case *GoFunction:

		result, multipleResults, enoughArgs, err := f.Call(goFunctionCallInput{
			symbolicArgs:      args,
			nonSpreadArgCount: nonSpreadArgCount,
			hasSpreadArg:      hasSpreadArg,
			state:             state,
			extState:          extState,
			isExt:             isSharedFunction,
			must:              must,
			callLikeNode:      callNode,
		})

		if errorsInArguments != 0 && state.SymbolicGoFunctionErrorsCount() != 0 {
			//If there are errors at argument nodes, there is a high chance that the
			//errors and updated self reported by the Go function call are irrelevant.
			//Hence we just get information about more specific parameters and return early.

			params, _, _, hasMoreSpecificParams := state.consumeSymbolicGoFunctionParameters()
			if hasMoreSpecificParams {
				setAllowedNonPresentProperties(argNodes, nonSpreadArgCount, params, state)
			}

			state.resetGoFunctionRelatedFields()
			return result, err
		}

		state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
			var location parse.Node = callNode
			if optionalLocation != nil {
				location = optionalLocation
			}

			state.addError(MakeSymbolicEvalError(location, state, msg))
		})
		state.consumeSymbolicGoFunctionWarnings(func(msg string) {
			state.addWarning(makeSymbolicEvalWarning(callNode, state, msg))
		})

		updatedSelf, ok := state.consumeUpdatedSelf()
		if ok && self != nil {
			static, _ := state.getStaticOfNode(selfPartialNode)

			if (static != nil && !static.TestValue(updatedSelf, RecTestCallState{})) ||
				(static == nil && !self.Test(updatedSelf, RecTestCallState{})) {

				state.addErrorIf(errorsInArguments == 0, MakeSymbolicEvalError(callNode, state, INVALID_MUTATION))
			} else { //ok
				narrowChain(selfPartialNode, setExactValue, updatedSelf, state, 0)
				checkNotClonedObjectPropMutation(selfPartialNode, state, false)
				state.symbolicData.SetLocalScopeData(callNode, state.currentLocalScopeData())
				state.symbolicData.SetGlobalScopeData(callNode, state.currentGlobalScopeData())
			}
		}

		if f.fn == nil {
			return result, err
		}

		//create a more specific *Function with the result and the provided parameters.
		utils.PanicIfErr(f.LoadSignatureData())
		params, paramNames, isSpecificFuncVariadic, hasMoreSpecificParams := state.consumeSymbolicGoFunctionParameters()
		if !hasMoreSpecificParams {
			params = f.ParametersExceptCtx()
			for i := 0; i < len(params); i++ {
				paramNames = append(paramNames, "_")
			}
			isSpecificFuncVariadic = f.isVariadic
		}

		var results []Value

		if list, ok := result.(*List); ok && multipleResults {
			results = SerializablesToValues(list.elements)
		} else {
			results = []Value{result}
		}

		firstOptionalParamIndex := -1
		if f.hasOptionalParams && f.lastMandatoryParamIndex >= 0 {
			firstOptionalParamIndex = f.lastMandatoryParamIndex + 1
			if f.isfirstArgCtx {
				firstOptionalParamIndex--
			}
		}

		function := NewFunction(params, paramNames, firstOptionalParamIndex, isSpecificFuncVariadic, results)
		function.originGoFunction = f

		//update the symbolic data of the callee with the *Function.
		state.symbolicData.PushNodeValue(calleeNode, function)
		switch c := calleeNode.(type) {
		case *parse.IdentifierMemberExpression:
			state.symbolicData.PushNodeValue(c.PropertyNames[len(c.PropertyNames)-1], function)
		case *parse.MemberExpression:
			state.symbolicData.PushNodeValue(c.PropertyName, function)
		}

		setAllowedNonPresentProperties(argNodes, nonSpreadArgCount, params, state)

		if !hasMoreSpecificParams || !enoughArgs {
			return result, err
		}

		//recheck arguments but with most specific function

		paramTypes := function.parameters
		currentArgs := args
		if !f.isVariadic {
			currentArgs = args[:min(len(params), len(args))]
		}

		for i, arg := range currentArgs {

			var argNode parse.Node
			if i < nonSpreadArgCount {
				argNode = argNodes[i]
			}

			paramTypeIndex := i
			if f.isVariadic && paramTypeIndex >= len(paramTypes) {
				paramTypeIndex = len(paramNames) - 1
			}

			paramType := paramTypes[paramTypeIndex]

			// for !IsAnyOrAnySerializable(widenedArg) && !paramType.Test(widenedArg) {
			// 	widenedArg = widenOrAny(widenedArg)
			// }

			if !paramType.Test(arg, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
				if argNode != nil {
					//if the argument node is a runtime check expression we store
					//the pattern that will be used at runtime to perform the check
					if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
						args[i] = paramType
						concreteCtx := state.ctx.startingConcreteContext
						pattern, ok := extData.GetConcretePatternMatchingSymbolicValue(concreteCtx, paramType)
						if ok {
							state.symbolicData.SetRuntimeTypecheckPattern(argNode, pattern)
						} else {
							state.addError(MakeSymbolicEvalError(argNode, state, UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK))
						}
					} else {
						deeperMismatch := false
						_symbolicEval(argNode, state, evalOptions{
							reEval:              true,
							expectedValue:       paramType,
							actualValueMismatch: &deeperMismatch,
						})

						if !deeperMismatch {
							msg, regions := FmtInvalidArg(state.fmtHelper, i, arg, paramType, state.testCallMessageBuffer)
							state.addError(MakeSymbolicEvalError(argNode, state, msg, regions...))
						}
					}
				} else {
					//TODO: support runtime typecheck for spread arg
					node := spreadArgNode
					if node == nil {
						node = callNode
					}
					msg, regions := FmtInvalidArg(state.fmtHelper, i, arg, paramType, state.testCallMessageBuffer)
					state.addError(MakeSymbolicEvalError(argNode, state, msg, regions...))
				}

				args[i] = paramType
			} else {
				//disable runtime type check
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					state.symbolicData.SetRuntimeTypecheckPattern(argNode, nil)
				} else {
					_symbolicEval(argNode, state, evalOptions{
						reEval:        true,
						expectedValue: paramType,
					})
				}
				args[i] = arg
			}
		}

		return result, err
	}

	//inox function | unknown type function
	var (
		nonVariadicParamCount int
		returnType            Value
		isVariadic            bool

		//only used for Inox functions.
		isBodyExpression        bool
		hasReturnTypeAnnotation bool
		parameterNodes          []*parse.FunctionParameter
		variadicParamNode       *parse.FunctionParameter
	)

	if inoxFn, ok := callee.(*InoxFunction); ok {
		funcExpr := inoxFn.FuncExpr()
		parameterNodes = funcExpr.Parameters
		nonVariadicParamCount = funcExpr.NonVariadicParamCount()
		isVariadic = inoxFn.IsVariadic()
		if isVariadic {
			variadicParamNode = funcExpr.VariadicParameter()
		}

		hasReturnTypeAnnotation = funcExpr.ReturnType != nil
		isBodyExpression = funcExpr.IsBodyExpression

		returnType = inoxFn.Result()
	} else {
		function := callee.(*Function)
		nonVariadicParamCount = len(function.NonVariadicParameters())
		isVariadic = function.IsVariadic()

		returnType = function.results[0]
	}

	if isVariadic {
		if nonSpreadArgCount < nonVariadicParamCount {
			state.addError(MakeSymbolicEvalError(callNode, state, fmtInvalidNumberOfNonSpreadArgs(nonSpreadArgCount, nonVariadicParamCount)))
			//if they are not enough arguments we use the parameter types to set their value

			for i := len(args); i < nonVariadicParamCount; i++ {
				args = append(args, nonGoParameters[i])
			}
		}
	} else if hasSpreadArg || len(args) != len(nonGoParameters) {
		if hasSpreadArg {
			state.addError(MakeSymbolicEvalError(callNode, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS))
		} else {
			state.addError(MakeSymbolicEvalError(callNode, state, fmtInvalidNumberOfArgs(len(args), len(nonGoParameters))))
		}

		if len(args) > len(nonGoParameters) {
			//if they are too many arguments we just ignore them
			args = args[:len(nonGoParameters)]
		} else {
			//if they are not enough arguments we use the parameter types to set their value
			for i := len(args); i < len(nonGoParameters); i++ {
				args = append(args, nonGoParameters[i])
			}
		}
	}

	//check arguments

	var params []Value

	for i, arg := range args {
		checkAgainstVariadicParam := i >= nonVariadicParamCount

		var paramType Value
		if checkAgainstVariadicParam {
			variadicParamType := nonGoParameters[len(nonGoParameters)-1].(*Array)
			paramType = variadicParamType.Element()
		} else {
			paramType = nonGoParameters[i]
		}

		params = append(params, paramType)

		var argNode parse.Node
		if i < nonSpreadArgCount {
			argNode = argNodes[i]
		}

		// for !IsAnyOrAnySerializable(widenedArg) && !paramType.Test(widenedArg) {
		// 	widenedArg = widenOrAny(widenedArg)
		// }

		if !paramType.Test(arg, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
			if argNode != nil {
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					args[i] = paramType
					concreteCtx := state.ctx.startingConcreteContext
					pattern, ok := extData.GetConcretePatternMatchingSymbolicValue(concreteCtx, paramType)
					if ok {
						state.symbolicData.SetRuntimeTypecheckPattern(argNode, pattern)
					} else {
						state.addError(MakeSymbolicEvalError(argNode, state, UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK))
					}
				} else {
					deeperMismatch := false
					_symbolicEval(argNode, state, evalOptions{
						reEval:              true,
						expectedValue:       paramType,
						actualValueMismatch: &deeperMismatch,
					})

					if !deeperMismatch {
						msg, regions := FmtInvalidArg(state.fmtHelper, i, arg, paramType, state.testCallMessageBuffer)
						state.addError(MakeSymbolicEvalError(argNode, state, msg, regions...))
					}
				}
			} else {
				//TODO: support runtime typecheck for spread arg
				node := spreadArgNode
				if node == nil {
					node = callNode
				}
				msg, regions := FmtInvalidArg(state.fmtHelper, i, arg, paramType, state.testCallMessageBuffer)
				state.addError(MakeSymbolicEvalError(argNode, state, msg, regions...))
			}

			args[i] = paramType
		} else {
			//disable runtime type check
			if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
				state.symbolicData.SetRuntimeTypecheckPattern(argNode, nil)
			}
			args[i] = arg
		}
	}

	setAllowedNonPresentProperties(argNodes, nonSpreadArgCount, params, state)

	if fnExpr == nil { // *Function
		ret := returnType

		if must {
			ret = checkTransformMustCallReturnValue(ret, callNode, state)
		}
		return ret, nil
	} //else Inox function

	inoxFn := callee.(*InoxFunction)

	//declare parameters

	state.pushScope()
	defer state.popScope()

	if self != nil {
		state.setSelf(self)
	}

	for i, p := range parameterNodes[:nonVariadicParamCount] {
		name := p.Var.(*parse.IdentifierLiteral).Name
		state.setLocal(name, args[i], &TypePattern{val: inoxFn.parameters[i]})
	}

	for name, val := range capturedLocals {
		state.setLocal(name, val, nil)
	}

	if isVariadic {
		variadicArgs := NewArray(args[nonVariadicParamCount:]...)
		name := variadicParamNode.Var.(*parse.IdentifierLiteral).Name
		state.setLocal(name, variadicArgs, nil)
	}

	//---------
	if hasReturnTypeAnnotation { //if a return type is specified we return the value representing the return type
		return returnType, nil
	} else { //if return type is not specified we "execute" the function

		if !state.pushInoxCall(inoxCallInfo{
			callNode:     callNode,
			calleeFnExpr: fnExpr,
		}) {
			return ANY, nil
		}

		defer state.popCall()

		var ret Value

		if isBodyExpression {
			ret, err = symbolicEval(fnExpr.Body, state)
			if err != nil {
				return nil, err
			}
		} else { // block
			conditionalReturn := state.conditionalReturn
			defer func() {
				//restore
				state.conditionalReturn = conditionalReturn
				//TODO: restore return value & return type ?
			}()

			// we do this to prevent invalid return statements to add an error
			state.returnType = ANY

			//execute body

			_, err = symbolicEval(fnExpr.Body, state)
			if err != nil {
				return nil, err
			}

			//we retrieve and post process the return value

			retValue := state.returnValue
			defer func() {
				state.returnValue = nil
				state.returnType = nil
			}()

			if retValue == nil {
				return Nil, nil
			}

			ret = state.returnValue
		}

		//'must' call: check and update return type
		if must {
			ret = checkTransformMustCallReturnValue(ret, callNode, state)
		}

		if isSharedFunction {
			shared, err := ShareOrClone(ret, state)
			if err != nil {
				state.addError(MakeSymbolicEvalError(callNode, state, err.Error()))
				shared = ANY
			}
			ret = shared
		}
		return ret, nil
	}

}

func setAllowedNonPresentProperties(argNodes []parse.Node, nonSpreadArgCount int, params []Value, state *State) {
	//ignore spread arg
	argNodes = argNodes[:min(len(argNodes), nonSpreadArgCount)]
	//ignore additional arguments
	argNodes = argNodes[:min(len(argNodes), len(params))]

	removePropertiesAlreadyPresent := func(allowedNonPresentProperties []string, propNodes []*parse.ObjectProperty) []string {
		//remove properties already present
		for _, propNode := range propNodes {
			if propNode.HasNoKey() {
				continue
			}
			propName := propNode.Name()
			for index, name := range allowedNonPresentProperties {
				if name == propName {
					allowedNonPresentProperties = utils.RemoveIndexOfSlice(allowedNonPresentProperties, index)
					break
				}
			}
		}

		return allowedNonPresentProperties
	}

	for i, arg := range argNodes {
		param := params[i]

		switch p := param.(type) {
		case *Object:
			allowedNonPresentProperties := GetAllPropertyNames(p)

			objLit, ok := arg.(*parse.ObjectLiteral)
			if !ok {
				continue
			}

			allowedNonPresentProperties = removePropertiesAlreadyPresent(allowedNonPresentProperties, objLit.Properties)
			sort.Strings(allowedNonPresentProperties)

			state.symbolicData.SetAllowedNonPresentProperties(objLit, allowedNonPresentProperties)
		case *Record:
			allowedNonPresentProperties := GetAllPropertyNames(p)

			recordLit, ok := arg.(*parse.RecordLiteral)
			if !ok {
				continue
			}

			allowedNonPresentProperties = removePropertiesAlreadyPresent(allowedNonPresentProperties, recordLit.Properties)
			sort.Strings(allowedNonPresentProperties)

			state.symbolicData.SetAllowedNonPresentProperties(recordLit, allowedNonPresentProperties)
		default:
			continue
		}
	}
}

func checkTransformMustCallReturnValue(ret Value, callNode *parse.CallExpression, state *State) Value {
	INVALID_RETURN_TYPE_MSG := INVALID_MUST_CALL_OF_AN_INOX_FN_RETURN_TYPE_MUST_BE_XXX

outer:
	switch r := ret.(type) {
	case *Array:
		array := r
		if !array.HasKnownLen() || array.KnownLen() == 0 {
			break
		}

		lastElem := array.elements[len(array.elements)-1]
		ok := false

		switch lastElem := lastElem.(type) {
		case *Error:
			ok = true
		case IMultivalue:
			onlyErrorsAndNil := lastElem.OriginalMultivalue().AllValues(func(v Value) bool {
				return utils.Implements[*Error](v) || utils.Implements[*NilT](v)
			})
			ok = onlyErrorsAndNil
		case *NilT:
			ok = true
		}

		if ok {
			switch array.KnownLen() {
			case 1:
				break outer
			case 2:
				return array.ElementAt(0)
			default:
				return NewArray(array.elements[:len(array.elements)-1]...)
			}
		}
	case IMultivalue:
		mv := r
		if len(mv.OriginalMultivalue().getValues()) == 2 {
			onlyErrorsAndNil := mv.OriginalMultivalue().AllValues(func(v Value) bool {
				return utils.Implements[*Error](v) || utils.Implements[*NilT](v)
			})
			if onlyErrorsAndNil {
				return Nil
			}
		}
	case *Error:
		state.addWarning(makeSymbolicEvalWarning(callNode, state, ERROR_IS_ALWAYS_RETURNED_THIS_WILL_CAUSE_A_PANIC))
		return Nil
	case *NilT:
		state.addWarning(makeSymbolicEvalWarning(callNode, state, NO_ERROR_IS_RETURNED))
		return Nil
	}

	state.addError(MakeSymbolicEvalError(callNode.Callee, state, INVALID_RETURN_TYPE_MSG))
	return ret
}

func isReturnValueWithPossibleError(ret Value) bool {
	switch r := ret.(type) {
	case *Array:
		array := r
		if !array.HasKnownLen() || array.KnownLen() == 0 {
			return false
		}

		lastElem := array.elements[len(array.elements)-1]

		switch lastElem := lastElem.(type) {
		case *Error:
			return true
		case IMultivalue:
			mv := lastElem.OriginalMultivalue()
			onlyErrorsAndNil := mv.AllValues(func(v Value) bool {
				return utils.Implements[*Error](v) || utils.Implements[*NilT](v)
			})
			onlyNil := mv.AllValues(func(v Value) bool {
				return utils.Implements[*NilT](v)
			})
			return onlyErrorsAndNil && !onlyNil
		case *NilT:
			return false
		}
	case IMultivalue:
		mv := r.OriginalMultivalue()
		if len(mv.getValues()) == 2 {
			onlyErrorsAndNil := mv.AllValues(func(v Value) bool {
				return utils.Implements[*Error](v) || utils.Implements[*NilT](v)
			})
			onlyNil := mv.AllValues(func(v Value) bool {
				return utils.Implements[*NilT](v)
			})
			return onlyErrorsAndNil && !onlyNil
		}
	case *Error:
		return true
	}

	return false
}

func checkCallExprWithUnhandledError(node parse.Node, ret Value, state *State) {
	callExpr, ok := node.(*parse.CallExpression)
	if !ok {
		return
	}

	//Ignore check for the Error factory function.
	if ident, ok := callExpr.Callee.(*parse.IdentifierLiteral); ok && ident.Name == globalnames.ERROR_FN {
		return
	}

	if isReturnValueWithPossibleError(ret) {
		state.addWarning(makeSymbolicEvalWarning(node, state, CALL_MAY_RETURN_ERROR_NOT_HANDLED_EITHER_HANDLE_IT_OR_TURN_THE_CALL_IN_A_MUST_CALL))
	}
}
