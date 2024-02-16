package core

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
)

type TreeWalkCall struct {
	callee             Value
	callNode           *parse.CallExpression //nil if the function is called in isolation.
	self               Value
	state              *TreeWalkState
	arguments          any
	must               bool
	cmdLineSyntax      bool
	disabledArgSharing []bool
}

// TreeWalkCallFunc calls calleeNode, whatever its kind (Inox function or Go function).
// If must is true and the second result of a Go function is a non-nil error, TreeWalkCallFunc will panic.
func TreeWalkCallFunc(call TreeWalkCall) (Value, error) {
	callee := call.callee
	self := call.self
	state := call.state
	arguments := call.arguments
	must := call.must
	cmdLineSyntax := call.cmdLineSyntax

	switch f := callee.(type) {
	case *InoxFunction:
		if f.compiledFunction != nil {
			return nil, ErrCannotEvaluateCompiledFunctionInTreeWalkEval
		}
	}

	var err error

	if len(state.LocalScopeStack) > MAX_FRAMES {
		return nil, ErrStackOverflow
	}

	var extState *GlobalState
	isSharedFunction := false
	var capturedGlobals []capturedGlobal

	if inoxFn, ok := callee.(*InoxFunction); ok {
		isSharedFunction = inoxFn.IsShared()
		capturedGlobals = inoxFn.capturedGlobals

		if isSharedFunction {
			extState = inoxFn.originState
		}

	} else {
		goFn := callee.(*GoFunction)
		isSharedFunction = goFn.IsShared()
		if isSharedFunction {
			extState = goFn.originState
		}
	}

	//EVALUATION OF ARGUMENTS

	args := []interface{}{}
	nonVariadicArgCount := 0
	hasSpreadArg := false

	if l, ok := arguments.([]Value); ok {
		for _, e := range l {
			args = append(args, e)
		}
	} else {
		for argIndex, argn := range arguments.([]parse.Node) {

			if spreadArg, ok := argn.(*parse.SpreadArgument); ok {
				hasSpreadArg = true

				array, err := TreeWalkEval(spreadArg.Expr, state)
				if err != nil {
					return nil, err
				}

				a := array.(Iterable)
				it := a.Iterator(state.Global.Ctx, IteratorConfiguration{KeysNeverRead: true})

				for it.Next(state.Global.Ctx) {
					e := it.Value(state.Global.Ctx)

					//same logic for non spread arguments
					if isSharedFunction {
						shared, err := ShareOrClone(e, state.Global)
						if err != nil {
							return nil, err
						}
						e = shared.(Serializable)
					}
					args = append(args, e)
				}
			} else {
				nonVariadicArgCount++

				if ident, ok := argn.(*parse.IdentifierLiteral); ok && cmdLineSyntax {
					args = append(args, Identifier(ident.Name))
				} else {
					arg, err := TreeWalkEval(argn, state)
					if err != nil {
						return nil, err
					}
					if isSharedFunction && (len(call.disabledArgSharing) <= argIndex || !call.disabledArgSharing[argIndex]) {
						shared, err := ShareOrClone(arg, state.Global)
						if err != nil {
							return nil, err
						}
						arg = shared
					}
					args = append(args, arg)
				}
			}

		}
	}

	//EXECUTION

	var (
		fn             *parse.FunctionExpression
		capturedLocals map[string]Value
		functionName   string
	)
	switch f := callee.(type) {
	case *InoxFunction:
		capturedLocals = f.treeWalkCapturedLocals

		switch node := f.Node.(type) {
		case *parse.FunctionExpression:
			fn = node
		case *parse.FunctionDeclaration:
			fn = node.Function
			functionName = node.Name.Name
		default:
			panic(fmt.Errorf("cannot call node of type %T", node))
		}
	case *GoFunction:
		return f.Call(args, state.Global, extState, isSharedFunction, must)
	default:
		panic(fmt.Errorf("cannot call node value of type %T", callee))
	}

	//INOX FUNCTION

	nonVariadicParamCount := fn.NonVariadicParamCount()
	if fn.IsVariadic {
		if nonVariadicArgCount < fn.NonVariadicParamCount() {
			return nil, fmt.Errorf("invalid number of non-variadic arguments : %v, at least %v were expected", nonVariadicArgCount, fn.NonVariadicParamCount())
		}
	} else if len(args) != len(fn.Parameters) {
		return nil, fmt.Errorf("invalid number of arguments : %v, %v was expected", len(args), len(fn.Parameters))
	} else if hasSpreadArg {
		return nil, errors.New("cannot call non-variadic function with a spread argument")
	}

	inoxFn := callee.(*InoxFunction)

	if call.callNode != nil { //if the function is not called in isolation
		state.pushChunkOfCall(inoxFn.Chunk, call.callNode)
		defer state.popChunkOfCall()

		state.pushChunkOfCall(inoxFn.Chunk, inoxFn.Node)
		defer state.popChunkOfCall()
	}

	state.PushScope()
	prevSelf := state.self
	state.self = self

	if capturedGlobals != nil && isSharedFunction {
		state.Global.Globals.PushCapturedGlobals(capturedGlobals)
		defer state.Global.Globals.PopCapturedGlobals()
	}

	defer func() {
		state.PopScope()
		state.self = prevSelf
	}()

	if state.debug != nil {
		chunk := state.currentChunk()
		line, col := chunk.GetLineColumn(fn)

		frameName := functionName
		if frameName == "" {
			frameName = chunk.GetFormattedNodeLocation(fn)
		}

		frameName = FUNCTION_FRAME_PREFIX + frameName

		state.frameInfo = append(state.frameInfo, StackFrameInfo{
			Node:        fn,
			Name:        frameName,
			Chunk:       chunk,
			StartLine:   line,
			StartColumn: col,
			Id:          state.debug.shared.getNextStackFrameId(),
		})

		defer func() {
			state.frameInfo = state.frameInfo[:len(state.frameInfo)-1]
		}()
	}

	currentScope := state.CurrentLocalScope()

	//CAPTURED LOCALS
	for name, val := range capturedLocals {
		currentScope[name] = val
	}

	//ARGUMENTS
	for i, p := range fn.Parameters[:nonVariadicParamCount] {
		name := p.Var.Name
		currentScope[name] = args[i].(Value)
	}

	if fn.IsVariadic {
		_variadicArgs := args[nonVariadicParamCount:]
		variadicArgs := make([]Value, 0, len(_variadicArgs))
		for _, e := range _variadicArgs {
			variadicArgs = append(variadicArgs, e.(Value))
		}
		name := fn.Parameters[len(fn.Parameters)-1].Var.Name
		currentScope[name] = NewArrayFrom(variadicArgs...)
	}

	bodyResult, err := TreeWalkEval(fn.Body, state)
	if err != nil {
		return nil, err
	}

	var ret Value

	if fn.IsBodyExpression {
		ret = bodyResult
	} else {
		//we retrieve and post process the return value
		retValuePtr := state.returnValue
		if retValuePtr == nil {
			return Nil, nil
		}

		defer func() {
			state.returnValue = nil
		}()

		ret = state.returnValue
	}

	if must {
		if transformed, err := checkTransformInoxMustCallResult(ret); err == nil {
			ret = transformed
		} else {
			panic(err)
		}
	}

	if isSharedFunction {
		shared, err := ShareOrClone(ret, extState)
		if err != nil {
			return nil, fmt.Errorf("failed to share a return value: %w", err)
		}
		ret = shared
	}
	return ret, nil
}
