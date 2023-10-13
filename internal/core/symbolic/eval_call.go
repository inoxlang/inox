package symbolic

import (
	"fmt"

	"github.com/inoxlang/inox/internal/globalnames"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

func callSymbolicFunc(callNode *parse.CallExpression, calleeNode parse.Node, state *State, argNodes []parse.Node, must bool, cmdLineSyntax bool) (SymbolicValue, error) {
	var (
		callee          SymbolicValue
		err             error
		self            SymbolicValue
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
			self, _ = state.symbolicData.GetMostSpecificNodeValue(_c.Left)
			selfPartialNode = _c.Left
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
	var nonGoParameters []SymbolicValue
	var argMismatches []bool

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
	} else if function, ok := callee.(*Function); ok {
		nonGoParameters = function.parameters
	} else {
		state.addError(makeSymbolicEvalError(calleeNode, state, fmtCannotCall(callee)))
		return ANY, nil
	}

	//evaluation of arguments

	args := make([]SymbolicValue, 0)
	nonSpreadArgCount := 0
	hasSpreadArg := false
	var spreadArgNode parse.Node

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
				var elements []SymbolicValue

				indexable, ok := v.(Indexable)
				if ok && indexable.HasKnownLen() {
					for i := 0; i < indexable.KnownLen(); i++ {
						elements = append(elements, indexable.elementAt(i))
					}
				} else { //add single element
					elements = append(elements, iterable.IteratorElementValue())
				}

				for _, e := range elements {
					//same logic for non spread arguments
					if isSharedFunction {
						shared, err := ShareOrClone(e, state)
						if err != nil {
							state.addError(makeSymbolicEvalError(argNode, state, err.Error()))
							shared = ANY
						}
						e = shared.(Serializable)
					}
					args = append(args, e)
				}
			} else {
				state.addError(makeSymbolicEvalError(argNode, state, fmtSpreadArgumentShouldBeIterable(v)))
			}

		} else {
			nonSpreadArgCount++

			if ident, ok := argNode.(*parse.IdentifierLiteral); ok && cmdLineSyntax {
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
						isGlobal := state.hasGlobal(ident.Name)
						state.addWarning(makeSymbolicEvalWarning(argNode, state, fmtDidYouMeanDollarName(ident.Name, isGlobal)))
					}
				}

			} else {
				options := evalOptions{}
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
						state.addError(makeSymbolicEvalError(argNode, state, err.Error()))
						shared = ANY
					}
					arg = shared
				}
				args = append(args, arg)
			}
		}

	}

	//execution

	var fn *parse.FunctionExpression
	var capturedLocals map[string]SymbolicValue

	switch f := callee.(type) {
	case *InoxFunction:
		if f.node == nil {
			state.addError(makeSymbolicEvalError(callNode, state, CALLEE_HAS_NODE_BUT_NOT_DEFINED))
			return ANY, nil
		} else {

			capturedLocals = f.capturedLocals

			switch function := f.node.(type) {
			case *parse.FunctionExpression:
				fn = function
			case *parse.FunctionDeclaration:
				fn = function.Function
			default:
				state.addError(makeSymbolicEvalError(callNode, state, fmtCannotCallNode(f.node)))
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

		state.consumeSymbolicGoFunctionErrors(func(msg string) {
			state.addError(makeSymbolicEvalError(callNode, state, msg))
		})
		state.consumeSymbolicGoFunctionWarnings(func(msg string) {
			state.addWarning(makeSymbolicEvalWarning(callNode, state, msg))
		})

		updatedSelf, ok := state.consumeUpdatedSelf()
		if ok && self != nil {
			static, _ := state.getStaticOfNode(selfPartialNode)

			if (static != nil && !static.TestValue(updatedSelf, RecTestCallState{})) ||
				(static == nil && !self.Test(updatedSelf, RecTestCallState{})) {
				state.addError(makeSymbolicEvalError(callNode, state, INVALID_MUTATION))
			} else { //ok
				narrowPath(selfPartialNode, setExactValue, updatedSelf, state, 0)
				checkNotClonedObjectPropMutation(selfPartialNode, state, false)
				state.symbolicData.SetLocalScopeData(callNode, state.currentLocalScopeData())
				state.symbolicData.SetGlobalScopeData(callNode, state.currentGlobalScopeData())
			}
		}

		if f.fn != nil {
			utils.PanicIfErr(f.LoadSignatureData())
			params, paramNames, hasMoreSpecificParams := state.consumeSymbolicGoFunctionParameters()
			if !hasMoreSpecificParams {
				params = f.ParametersExceptCtx()
			}

			function := &Function{
				parameters:     params,
				parameterNames: paramNames,
				variadic:       f.isVariadic,
			}

			if list, ok := result.(*List); ok && multipleResults {
				function.results = SerializablesToValues(list.elements)
			} else {
				function.results = []SymbolicValue{result}
			}

			state.symbolicData.PushNodeValue(calleeNode, function)
			switch c := calleeNode.(type) {
			case *parse.IdentifierMemberExpression:
				state.symbolicData.PushNodeValue(c.PropertyNames[len(c.PropertyNames)-1], function)
			case *parse.MemberExpression:
				state.symbolicData.PushNodeValue(c.PropertyName, function)
			}

			setAllowedNonPresentProperties(argNodes, nonSpreadArgCount, params, state)

			if !hasMoreSpecificParams || !enoughArgs {
				goto go_func_result
			}

			//recheck arguments but with most specific function

			paramTypes := function.parameters
			currentArgs := args
			if !f.isVariadic {
				currentArgs = args[:len(params)]
			}

			for i, arg := range currentArgs {

				var argNode parse.Node
				if i < nonSpreadArgCount {
					argNode = argNodes[i]
				}

				paramType := paramTypes[i]

				// for !IsAnyOrAnySerializable(widenedArg) && !paramType.Test(widenedArg) {
				// 	widenedArg = widenOrAny(widenedArg)
				// }

				if !paramType.Test(arg, RecTestCallState{}) {
					if argNode != nil {
						//if the argument node is a runtime check expression we store
						//the pattern that will be used at runtime to perform the check
						if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
							args[i] = paramType
							pattern, ok := extData.SymbolicToPattern(paramType)
							if ok {
								state.symbolicData.SetRuntimeTypecheckPattern(argNode, pattern)
							} else {
								state.addError(makeSymbolicEvalError(argNode, state, UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK))
							}
						} else {
							deeperMismatch := false
							_symbolicEval(argNode, state, evalOptions{
								reEval:              true,
								expectedValue:       paramType,
								actualValueMismatch: &deeperMismatch,
							})

							if !deeperMismatch {
								state.addError(makeSymbolicEvalError(argNode, state, FmtInvalidArg(i, arg, paramType)))
							}
						}
					} else {
						//TODO: support runtime typecheck for spread arg
						node := spreadArgNode
						if node == nil {
							node = callNode
						}
						state.addError(makeSymbolicEvalError(node, state, FmtInvalidArg(i, arg, paramType)))
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

		}

	go_func_result:
		return result, err
	}

	//inox function | unknown type function
	var (
		nonVariadicParamCount int
		parameterNodes        []*parse.FunctionParameter
		variadicParamNode     *parse.FunctionParameter
		returnType            parse.Node
		isBodyExpression      bool
		isVariadic            bool
	)

	if _, ok := callee.(*InoxFunction); ok {
		nonVariadicParamCount, parameterNodes, variadicParamNode, returnType, isBodyExpression =
			fn.SignatureInformation()
	} else {
		nonVariadicParamCount, parameterNodes, variadicParamNode, returnType, isBodyExpression =
			callee.(*Function).pattern.node.SignatureInformation()
	}

	isVariadic = variadicParamNode != nil

	if isVariadic {
		if nonSpreadArgCount < nonVariadicParamCount {
			state.addError(makeSymbolicEvalError(callNode, state, fmtInvalidNumberOfNonSpreadArgs(nonSpreadArgCount, nonVariadicParamCount)))
			//if they are not enough arguments we use the parameter types to set their value

			for i := len(args); i < nonVariadicParamCount; i++ {
				var paramType SymbolicValue

				paramTypeNode := parameterNodes[i].Type
				if paramTypeNode == nil {
					paramType = ANY
				} else {
					pattern, err := symbolicallyEvalPatternNode(paramTypeNode, state)
					if err != nil {
						return nil, err
					}
					paramType = pattern.SymbolicValue()
				}

				args = append(args, paramType)
			}
		}
	} else if hasSpreadArg || len(args) != len(parameterNodes) {

		if hasSpreadArg {
			state.addError(makeSymbolicEvalError(callNode, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS))
		} else {
			state.addError(makeSymbolicEvalError(callNode, state, fmtInvalidNumberOfArgs(len(args), len(parameterNodes))))
		}

		if len(args) > len(parameterNodes) {
			//if they are too many arguments we just ignore them
			args = args[:len(parameterNodes)]
		} else {
			//if they are not enough arguments we use the parameter types to set their value
			for i := len(args); i < len(parameterNodes); i++ {
				var paramType SymbolicValue

				paramTypeNode := parameterNodes[i].Type
				if paramTypeNode == nil {
					paramType = ANY
				} else {
					pattern, err := symbolicallyEvalPatternNode(paramTypeNode, state)
					if err != nil {
						return nil, err
					}
					paramType = pattern.SymbolicValue()
				}

				args = append(args, paramType)
			}
		}
	}

	//check arguments

	var params []SymbolicValue

	for i, arg := range args {
		var paramTypeNode parse.Node

		if i >= nonVariadicParamCount {
			paramTypeNode = variadicParamNode.Type
		} else {
			paramTypeNode = parameterNodes[i].Type
		}

		if paramTypeNode == nil {
			continue
		}

		paramType := nonGoParameters[i]
		params = append(params, paramType)

		var argNode parse.Node
		if i < nonSpreadArgCount {
			argNode = argNodes[i]
		}

		// for !IsAnyOrAnySerializable(widenedArg) && !paramType.Test(widenedArg) {
		// 	widenedArg = widenOrAny(widenedArg)
		// }

		if !paramType.Test(arg, RecTestCallState{}) {
			if argNode != nil {
				if _, ok := argNode.(*parse.RuntimeTypeCheckExpression); ok {
					args[i] = paramType
					pattern, ok := extData.SymbolicToPattern(paramType)
					if ok {
						state.symbolicData.SetRuntimeTypecheckPattern(argNode, pattern)
					} else {
						state.addError(makeSymbolicEvalError(argNode, state, UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK))
					}
				} else {
					deeperMismatch := false
					_symbolicEval(argNode, state, evalOptions{
						reEval:              true,
						expectedValue:       paramType,
						actualValueMismatch: &deeperMismatch,
					})

					if !deeperMismatch {
						state.addError(makeSymbolicEvalError(argNode, state, FmtInvalidArg(i, arg, paramType)))
					}
				}
			} else {
				//TODO: support runtime typecheck for spread arg
				node := spreadArgNode
				if node == nil {
					node = callNode
				}
				state.addError(makeSymbolicEvalError(node, state, FmtInvalidArg(i, arg, paramType)))
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

	if fn == nil { // *Function
		patt, err := symbolicEval(returnType, state)
		if err != nil {
			return nil, err
		}
		return patt.(Pattern).SymbolicValue(), nil
	} //else Inox function

	inoxFn := callee.(*InoxFunction)

	//declare parameters

	state.pushScope()
	defer state.popScope()

	if self != nil {
		state.setSelf(self)
	}

	for i, p := range parameterNodes[:nonVariadicParamCount] {
		name := p.Var.Name
		state.setLocal(name, args[i], &TypePattern{val: inoxFn.parameters[i]})
	}

	for name, val := range capturedLocals {
		state.setLocal(name, val, nil)
	}

	if isVariadic {
		variadicArgs := NewArray(args[nonVariadicParamCount:]...)
		name := variadicParamNode.Var.Name
		state.setLocal(name, variadicArgs, nil)
	}

	//---------
	if returnType != nil { //if a return type is specified we return the value representing the return type
		pattern, err := symbolicallyEvalPatternNode(returnType, state)
		if err != nil {
			return nil, err
		}
		typ := pattern.SymbolicValue()
		return typ, nil
	} else { //if return type is not specified we "execute" the function

		if !state.pushCallee(callNode, fn) {
			return ANY, nil
		}

		defer state.popCallee()

		var ret SymbolicValue

		if isBodyExpression {
			ret, err = symbolicEval(fn.Body, state)
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

			_, err = symbolicEval(fn.Body, state)
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

		if must {
			if array, isArray := ret.(*Array); isArray && array.HasKnownLen() && array.KnownLen() != 0 {
				lastElem := array.elements[len(array.elements)-1]

				if _, ok := lastElem.(*Error); ok {
					panic("symbolic evaluation of 'must' calls not fully implemented")
				}
			}
		}

		if isSharedFunction {
			shared, err := ShareOrClone(ret, state)
			if err != nil {
				state.addError(makeSymbolicEvalError(callNode, state, err.Error()))
				shared = ANY
			}
			ret = shared
		}
		return ret, nil
	}

}

func setAllowedNonPresentProperties(argNodes []parse.Node, nonSpreadArgCount int, params []SymbolicValue, state *State) {
	//ignore spread arg
	argNodes = argNodes[:min(len(argNodes), nonSpreadArgCount)]
	//ignore additional arguments
	argNodes = argNodes[:min(len(argNodes), len(params))]

	removePropertiesAlreadyPresent := func(allowedNonPresentProperties []string, propNodes []*parse.ObjectProperty) []string {
		//remove properties already present
		for _, propNode := range propNodes {
			if propNode.HasImplicitKey() {
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
			state.symbolicData.SetAllowedNonPresentProperties(objLit, allowedNonPresentProperties)
		case *Record:
			allowedNonPresentProperties := GetAllPropertyNames(p)

			recordLit, ok := arg.(*parse.RecordLiteral)
			if !ok {
				continue
			}

			allowedNonPresentProperties = removePropertiesAlreadyPresent(allowedNonPresentProperties, recordLit.Properties)
			state.symbolicData.SetAllowedNonPresentProperties(recordLit, allowedNonPresentProperties)
		default:
			continue
		}
	}
}
