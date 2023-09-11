package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"slices"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"golang.org/x/exp/maps"
	"gonum.org/v1/gonum/graph/simple"
)

type IterationChange int

const (
	NoIterationChange IterationChange = iota
	BreakIteration
	ContinueIteration
	PruneWalk
)

type GlobalConstness = int

const (
	GlobalVar GlobalConstness = iota
	GlobalConst
)

const (
	MAX_STRING_SUGGESTION_DIFF = 3
)

var (
	CTX_PTR_TYPE                         = reflect.TypeOf((*Context)(nil))
	ERROR_TYPE                           = reflect.TypeOf((*Error)(nil))
	SYMBOLIC_VALUE_INTERFACE_TYPE        = reflect.TypeOf((*SymbolicValue)(nil)).Elem()
	SERIALIZABLE_INTERFACE_TYPE          = reflect.TypeOf((*Serializable)(nil)).Elem()
	ITERABLE_INTERFACE_TYPE              = reflect.TypeOf((*Iterable)(nil)).Elem()
	SERIALIZABLE_ITERABLE_INTERFACE_TYPE = reflect.TypeOf((*SerializableIterable)(nil)).Elem()
	INDEXABLE_INTERFACE_TYPE             = reflect.TypeOf((*Indexable)(nil)).Elem()
	SEQUENCE_INTERFACE_TYPE              = reflect.TypeOf((*Sequence)(nil)).Elem()
	MUTABLE_SEQUENCE_INTERFACE_TYPE      = reflect.TypeOf((*MutableSequence)(nil)).Elem()
	INTEGRAL_INTERFACE_TYPE              = reflect.TypeOf((*Integral)(nil)).Elem()
	WRITABLE_INTERFACE_TYPE              = reflect.TypeOf((*Writable)(nil)).Elem()
	STRLIKE_INTERFACE_TYPE               = reflect.TypeOf((*StringLike)(nil)).Elem()
	BYTESLIKE_INTERFACE_TYPE             = reflect.TypeOf((*BytesLike)(nil)).Elem()

	IPROPS_INTERFACE_TYPE              = reflect.TypeOf((*IProps)(nil)).Elem()
	PROTOCOL_CLIENT_INTERFACE_TYPE     = reflect.TypeOf((*ProtocolClient)(nil)).Elem()
	READABLE_INTERFACE_TYPE            = reflect.TypeOf((*Readable)(nil)).Elem()
	PATTERN_INTERFACE_TYPE             = reflect.TypeOf((*Pattern)(nil)).Elem()
	RESOURCE_NAME_INTERFACE_TYPE       = reflect.TypeOf((*ResourceName)(nil)).Elem()
	VALUE_RECEIVER_INTERFACE_TYPE      = reflect.TypeOf((*MessageReceiver)(nil)).Elem()
	STREAMABLE_INTERFACE_TYPE          = reflect.TypeOf((*StreamSource)(nil)).Elem()
	WATCHABLE_INTERFACE_TYPE           = reflect.TypeOf((*Watchable)(nil)).Elem()
	STR_PATTERN_ELEMENT_INTERFACE_TYPE = reflect.TypeOf((*StringPattern)(nil)).Elem()
	FORMAT_INTERFACE_TYPE              = reflect.TypeOf((*Format)(nil)).Elem()
	IN_MEM_SNAPSHOTABLE                = reflect.TypeOf((*InMemorySnapshotable)(nil)).Elem()

	ANY_READABLE = &AnyReadable{}
	ANY_READER   = &Reader{}

	SUPPORTED_PARSING_ERRORS = []parse.ParsingErrorKind{
		parse.UnterminatedMemberExpr, parse.MissingBlock, parse.MissingFnBody,
		parse.MissingEqualsSignInDeclaration,
	}
)

type ConcreteGlobalValue struct {
	Value      any
	IsConstant bool
}

func (v ConcreteGlobalValue) Constness() GlobalConstness {
	if v.IsConstant {
		return GlobalConst
	}
	return GlobalVar
}

type SymbolicEvalCheckInput struct {
	Node   *parse.Chunk
	Module *Module

	//should not be set if UseBaseGlobals is true
	Globals                        map[string]ConcreteGlobalValue
	AdditionalSymbolicGlobalConsts map[string]SymbolicValue

	UseBaseGlobals                bool
	SymbolicBaseGlobals           map[string]SymbolicValue
	SymbolicBasePatterns          map[string]Pattern
	SymbolicBasePatternNamespaces map[string]*PatternNamespace

	IsShellChunk   bool
	ShellLocalVars map[string]any
	Context        *Context

	importPositions     []parse.SourcePositionRange
	initialSymbolicData *SymbolicData
}

// SymbolicEvalCheck performs various checks on an AST, most checks are type checks.
// If the returned data is not nil the error is nil or is the combination of checking errors, the list of checking errors
// is stored in the symbolic data.
// If the returned data is nil the error is an unexpected one (it is not about bad code).
// StaticCheck() should be runned before this function.
func SymbolicEvalCheck(input SymbolicEvalCheckInput) (*SymbolicData, error) {

	state := newSymbolicState(input.Context, input.Module.mainChunk)
	state.Module = input.Module
	state.baseGlobals = input.SymbolicBaseGlobals
	state.basePatterns = input.SymbolicBasePatterns
	state.basePatternNamespaces = input.SymbolicBasePatternNamespaces
	state.importPositions = utils.CopySlice(input.importPositions)

	if input.UseBaseGlobals {
		if input.Globals != nil {
			return nil, errors.New(".Globals should not be set")
		}
		if input.AdditionalSymbolicGlobalConsts != nil {
			return nil, errors.New(".AdditionalSymbolicGlobalConsts should not be set")
		}
		for k, v := range input.SymbolicBaseGlobals {
			state.setGlobal(k, v, GlobalConst)
		}
	} else {
		for k, concreteGlobal := range input.Globals {
			symbolicVal, err := extData.ToSymbolicValue(concreteGlobal.Value, false)
			if err != nil {
				return nil, fmt.Errorf("cannot convert global %s: %s", k, err)
			}
			state.setGlobal(k, symbolicVal, concreteGlobal.Constness())
		}

		for k, v := range input.AdditionalSymbolicGlobalConsts {
			state.setGlobal(k, v, GlobalConst)
		}
	}

	if input.IsShellChunk {
		if input.ShellLocalVars != nil {
			state.pushScope()
			defer state.popScope()
		}

		for k, v := range input.ShellLocalVars {
			symbolicVal, err := extData.ToSymbolicValue(v, false)
			if err != nil {
				return nil, fmt.Errorf("cannot convert global %s: %s", k, err)
			}
			state.setLocal(k, symbolicVal, &AnyPattern{})
		}
	}

	if input.initialSymbolicData != nil {
		state.symbolicData = input.initialSymbolicData
	} else {
		state.symbolicData = NewSymbolicData()
	}

	_, err := symbolicEval(input.Node, state)

	finalErrBuff := bytes.NewBuffer(nil)
	if err != nil { //unexpected error
		return nil, err
	}

	if len(state.errors()) == 0 { //no error in checked code
		return state.symbolicData, nil
	}

	for _, err := range state.errors() {
		finalErrBuff.WriteString(err.Error())
		finalErrBuff.WriteRune('\n')
	}

	return state.symbolicData, errors.New(finalErrBuff.String())
}

func SymbolicEval(node parse.Node, state *State) (result SymbolicValue, finalErr error) {
	return symbolicEval(node, state)
}

func symbolicEval(node parse.Node, state *State) (result SymbolicValue, finalErr error) {
	return _symbolicEval(node, state, evalOptions{})
}

type evalOptions struct {
	ignoreNodeValue     bool
	expectedValue       SymbolicValue
	actualValueMismatch *bool
	reEval              bool

	//used for checking that double-colon expressions are not misplaced
	doubleColonExprAncestorChain []parse.Node
}

func (opts evalOptions) setActualValueMismatchIfNotNil() {
	if opts.actualValueMismatch != nil {
		*opts.actualValueMismatch = true
	}
}

func _symbolicEval(node parse.Node, state *State, options evalOptions) (result SymbolicValue, finalErr error) {
	defer func() {

		e := recover()
		if e != nil {
			location := state.getErrorMesssageLocation(node)
			stack := string(debug.Stack())
			switch val := e.(type) {
			case error:
				finalErr = fmt.Errorf("%s %w\n%s", location, val, stack)
			default:
				finalErr = fmt.Errorf("panic: %s %#v\n%s", location, val, stack)
			}
			result = ANY
			return
		}

		if !options.ignoreNodeValue && !options.reEval && finalErr == nil && result != nil && state.symbolicData != nil {
			state.symbolicData.SetMostSpecificNodeValue(node, result)
		}
	}()

	if state.ctx.noCheckFuel == 0 && state.ctx.startingConcreteContext != nil {
		select {
		case <-state.ctx.startingConcreteContext.Done():
			return nil, fmt.Errorf("stopped symbolic evaluation because context is done: %w", state.ctx.startingConcreteContext.Err())
		default:
			state.ctx.noCheckFuel = INITIAL_NO_CHECK_FUEL
		}
	} else {
		state.ctx.noCheckFuel--
	}

	if options.reEval {
		//note: re-evaluation should aways be side-effect free, its main purpose
		//is having better error locations & better completions.
		switch node.(type) {
		case *parse.ObjectLiteral, *parse.RecordLiteral, *parse.DictionaryLiteral,
			*parse.ListLiteral, *parse.TupleLiteral:
		default:
			nodeValue, ok := state.symbolicData.GetMostSpecificNodeValue(node)
			if !ok {
				return nil, fmt.Errorf("no value for node of type %T", nodeValue)
			}
			return nodeValue, nil
		}
	}

	switch n := node.(type) {
	case *parse.BooleanLiteral:
		return NewBool(n.Value), nil
	case *parse.IntLiteral:
		return &Int{value: n.Value, hasValue: true}, nil
	case *parse.FloatLiteral:
		return NewFloat(n.Value), nil
	case *parse.PortLiteral:
		return &Port{}, nil
	case *parse.QuantityLiteral:
		v, err := extData.GetQuantity(n.Values, n.Units)
		if err != nil {
			return nil, err
		}
		return extData.ToSymbolicValue(v, false)
	case *parse.DateLiteral:
		return NewDate(n.Value), nil
	case *parse.RateLiteral:
		v, err := extData.GetRate(n.Values, n.Units, n.DivUnit)
		if err != nil {
			return nil, err
		}
		return extData.ToSymbolicValue(v, false)
	case *parse.QuotedStringLiteral:
		return NewString(n.Value), nil
	case *parse.UnquotedStringLiteral:
		return NewString(n.Value), nil
	case *parse.MultilineStringLiteral:
		return NewString(n.Value), nil
	case *parse.RuneLiteral:
		return NewRune(n.Value), nil
	case *parse.IdentifierLiteral:
		info, ok := state.get(n.Name)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtVarIsNotDeclared(n.Name)))
			return ANY, nil
		}
		return info.value, nil
	case *parse.UnambiguousIdentifierLiteral:
		return &Identifier{name: n.Name}, nil
	case *parse.PropertyNameLiteral:
		return &PropertyName{name: n.Name}, nil
	case *parse.AbsolutePathLiteral:
		return NewPath(n.Value), nil
	case *parse.RelativePathLiteral:
		return NewPath(n.Value), nil
	case *parse.AbsolutePathPatternLiteral:
		return NewPathPattern(n.Value), nil
	case *parse.RelativePathPatternLiteral:
		return NewPathPattern(n.Value), nil
	case *parse.NamedSegmentPathPatternLiteral:
		return &NamedSegmentPathPattern{node: n}, nil
	case *parse.RegularExpressionLiteral:
		return NewRegexPattern(n.Value), nil
	case *parse.PathSlice, *parse.PathPatternSlice:
		return ANY_STR, nil
	case *parse.URLQueryParameterValueSlice:
		return ANY_STR, nil
	case *parse.FlagLiteral:
		return NewOption(n.Name, TRUE), nil
	case *parse.OptionExpression:
		v, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		return NewOption(n.Name, v), nil
	case *parse.AbsolutePathExpression, *parse.RelativePathExpression:
		var slices []parse.Node

		switch pexpr := n.(type) {
		case *parse.AbsolutePathExpression:
			slices = pexpr.Slices
		case *parse.RelativePathExpression:
			slices = pexpr.Slices
		}

		for _, node := range slices {
			_, isStaticPathSlice := node.(*parse.PathSlice)
			_, err := _symbolicEval(node, state, evalOptions{ignoreNodeValue: isStaticPathSlice})
			if err != nil {
				return nil, err
			}

			if isStaticPathSlice {
				state.symbolicData.SetMostSpecificNodeValue(node, ANY_PATH)
			}
		}

		return ANY_PATH, nil
	case *parse.PathPatternExpression:
		return NewPathPatternFromNode(n), nil
	case *parse.URLLiteral:
		return NewUrl(n.Value), nil
	case *parse.SchemeLiteral:
		return NewScheme(n.ValueString()), nil
	case *parse.HostLiteral:
		return NewHost(n.Value), nil
	case *parse.AtHostLiteral:
		return ANY, nil
	case *parse.EmailAddressLiteral:
		return NewEmailAddress(n.Value), nil
	case *parse.HostPatternLiteral:
		return NewHostPattern(n.Value), nil
	case *parse.URLPatternLiteral:
		return NewUrlPattern(n.Value), nil
	case *parse.URLExpression:
		_, err := _symbolicEval(n.HostPart, state, evalOptions{ignoreNodeValue: true})
		if err != nil {
			return nil, err
		}

		state.symbolicData.SetMostSpecificNodeValue(n.HostPart, ANY_URL)

		//path evaluation

		for _, node := range n.Path {
			_, isStaticPathSlice := node.(*parse.PathSlice)
			_, err := _symbolicEval(node, state, evalOptions{ignoreNodeValue: isStaticPathSlice})
			if err != nil {
				return nil, err
			}

			if isStaticPathSlice {
				state.symbolicData.SetMostSpecificNodeValue(node, ANY_URL)
			}
		}

		//query evaluation

		for _, p := range n.QueryParams {
			param := p.(*parse.URLQueryParameter)

			state.symbolicData.SetMostSpecificNodeValue(param, ANY_URL)

			for _, slice := range param.Value {
				val, err := symbolicEval(slice, state)
				if err != nil {
					return nil, err
				}
				switch val.(type) {
				case StringLike, *Int, *Bool:
				default:
					state.addError(makeSymbolicEvalError(p, state, fmtValueNotStringifiableToQueryParamValue(val)))
				}
			}
		}

		return ANY_URL, nil
	case *parse.NilLiteral:
		return &NilT{}, nil
	case *parse.SelfExpression:
		v, ok := state.getSelf()
		if !ok {
			return nil, errors.New("no self")
		}
		return v, nil
	case *parse.SupersysExpression:
		return ANY, nil
	case *parse.Variable:
		info, ok := state.getLocal(n.Name)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtLocalVarIsNotDeclared(n.Name)))
			return ANY, nil
		}
		return info.value, nil
	case *parse.GlobalVariable:
		info, ok := state.getGlobal(n.Name)

		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtGlobalVarIsNotDeclared(n.Name)))
			return ANY, nil
		}
		return info.value, nil
	case *parse.ReturnStatement:
		if n.Expr == nil {
			return nil, nil
		}

		var deeperMismatch *bool
		if state.returnType != nil {
			deeperMismatch = new(bool)
		}

		value, err := _symbolicEval(n.Expr, state, evalOptions{
			expectedValue:       state.returnType,
			actualValueMismatch: deeperMismatch,
		})

		if err != nil {
			return nil, err
		}
		v := value

		if state.returnType != nil && !state.returnType.Test(v) {
			if !*deeperMismatch {
				state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(v, state.returnType)))
			}
			state.returnValue = state.returnType
		}

		if state.returnValue != nil {
			state.returnValue = joinValues([]SymbolicValue{state.returnValue, v})
		} else {
			state.returnValue = v
		}

		state.conditionalReturn = false

		return nil, nil
	case *parse.YieldStatement:
		if n.Expr == nil {
			return nil, nil
		}

		_, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return nil, nil
	case *parse.BreakStatement:
		return nil, nil
	case *parse.ContinueStatement:
		return nil, nil
	case *parse.PruneStatement:
		return nil, nil
	case *parse.CallExpression:
		return callSymbolicFunc(n, n.Callee, state, n.Arguments, n.Must, n.CommandLikeSyntax)
	case *parse.PatternCallExpression:
		callee, err := symbolicEval(n.Callee, state)
		if err != nil {
			return nil, err
		}

		args := make([]SymbolicValue, len(n.Arguments))

		errCount := len(state.errors())

		for i, argNode := range n.Arguments {
			arg, err := symbolicEval(argNode, state)
			if err != nil {
				return nil, err
			}
			args[i] = arg
		}

		if len(state.errors()) == errCount {
			patt, err := callee.(Pattern).Call(state.ctx, args)
			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				state.addError(makeSymbolicEvalError(n, state, msg))
			})

			if err != nil {
				state.addError(makeSymbolicEvalError(n, state, err.Error()))
				patt = ANY_PATTERN
			}
			return patt, nil
		}
		return ANY_PATTERN, nil
	case *parse.PipelineStatement, *parse.PipelineExpression:
		var stages []*parse.PipelineStage

		switch e := n.(type) {
		case *parse.PipelineStatement:
			stages = e.Stages
		case *parse.PipelineExpression:
			stages = e.Stages
		}

		defer func() {
			state.removeLocal("")
		}()

		var res SymbolicValue
		var err error

		for _, stage := range stages {
			res, err = symbolicEval(stage.Expr, state)
			if err != nil {
				return nil, err
			}
			state.overrideLocal("", res)
		}

		return res, nil
	case *parse.LocalVariableDeclarations:
		for _, decl := range n.Declarations {
			name := decl.Left.(*parse.IdentifierLiteral).Name

			var static Pattern
			var staticMatching SymbolicValue

			if decl.Type != nil {
				type_, err := symbolicEval(decl.Type, state)
				if err != nil {
					return nil, err
				}
				static = type_.(Pattern)
				staticMatching = static.SymbolicValue()
			}

			var (
				right SymbolicValue
				err   error
			)

			if decl.Right != nil {
				deeperMismatch := false
				right, err = _symbolicEval(decl.Right, state, evalOptions{expectedValue: staticMatching, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}

				if static != nil {
					if !static.TestValue(right) {
						if !deeperMismatch {
							state.addError(makeSymbolicEvalError(decl.Right, state, fmtNotAssignableToVarOftype(right, static)))
						}
						right = ANY
					} else {
						if holder, ok := right.(StaticDataHolder); ok {
							holder.AddStatic(static) //TODO: use path narowing, values should never be modified directly
						}
					}
				}
			} else {
				if static == nil {
					right = ANY
				} else {
					right = static.SymbolicValue()
				}
			}

			state.setLocal(name, right, static, decl.Left)
			state.symbolicData.SetMostSpecificNodeValue(decl.Left, right)
		}
		state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		return nil, nil
	case *parse.Assignment:
		badIntOperationRHS := false
		var __rhs SymbolicValue

		getRHS := func(expected SymbolicValue) (value SymbolicValue, deeperMismatch bool, _ error) {
			if __rhs != nil {
				panic(errors.New("right node already evaluated"))
			}

			var result SymbolicValue
			var err error
			if expected == nil {
				result, err = symbolicEval(n.Right, state)
			} else {
				result, err = _symbolicEval(n.Right, state, evalOptions{
					expectedValue:       expected,
					actualValueMismatch: &deeperMismatch,
				})
			}

			if err != nil {
				return nil, false, err
			}

			if n.Operator.Int() {
				// if the operation requires integer operands we check that RHS is an integer
				if _, ok := result.(*Int); !ok {
					badIntOperationRHS = true
					state.addError(makeSymbolicEvalError(n.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT))
				}
				result = ANY_INT
			}

			value = result
			__rhs = value
			return
		}

		defer func() {
			if finalErr == nil {
				//if n.Right was not evaluated we dot it now
				if __rhs == nil {
					_, _, finalErr = getRHS(nil)
					if finalErr != nil {
						return
					}
				}

				state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
				state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
			}
		}()

		switch lhs := n.Left.(type) {
		case *parse.Variable:
			name := lhs.Name

			if state.hasLocal(name) {
				if n.Operator.Int() {
					info, _ := state.getLocal(name)
					rhs, _, err := getRHS(nil)
					if err != nil {
						return nil, err
					}

					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateLocal(name, rhs, node)
					}
				} else {
					if _, err := state.updateLocal2(name, node, getRHS, false); err != nil {
						return nil, err
					}
				}

			} else {
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}
				state.setLocal(name, rhs, nil, n.Left)
			}

			checkNotClonedObjectPropMutation(lhs, state)

			//TODO: set to previous value instead ?
			state.symbolicData.SetMostSpecificNodeValue(lhs, __rhs)
			state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		case *parse.IdentifierLiteral:
			name := lhs.Name

			if state.hasLocal(name) {
				if n.Operator.Int() {
					info, _ := state.getLocal(name)
					rhs, _, err := getRHS(nil)
					if err != nil {
						return nil, err
					}

					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateLocal(name, rhs, node)
					}
				} else {
					if _, err := state.updateLocal2(name, node, getRHS, false); err != nil {
						return nil, err
					}
				}

			} else if state.hasGlobal(name) {
				if n.Operator.Int() {
					info, _ := state.getGlobal(name)
					rhs, _, err := getRHS(nil)
					if err != nil {
						return nil, err
					}

					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateGlobal(name, rhs, node)
					}
				} else {
					if _, err := state.updateGlobal2(name, node, getRHS, false); err != nil {
						return nil, err
					}
				}
			} else {
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}
				state.setLocal(name, rhs, nil, n.Left)
			}

			checkNotClonedObjectPropMutation(lhs, state)

			//TODO: set to previous value instead ?
			state.symbolicData.SetMostSpecificNodeValue(lhs, __rhs)
			state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		case *parse.GlobalVariable:
			name := lhs.Name

			info, alreadyDefined := state.getGlobal(name)
			if alreadyDefined && info.isConstant {
				state.addError(makeSymbolicEvalError(node, state, fmtAttempToAssignConstantGlobal(name)))
			}

			if state.hasGlobal(name) {
				if n.Operator.Int() {
					info, _ := state.getGlobal(name)
					rhs, _, err := getRHS(nil)
					if err != nil {
						return nil, err
					}

					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateGlobal(name, rhs, node)
					}
				} else {
					if _, err := state.updateGlobal2(name, node, getRHS, false); err != nil {
						return nil, err
					}
				}

			} else {
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}
				state.setGlobal(name, rhs, GlobalVar, n.Left)
			}

			checkNotClonedObjectPropMutation(lhs, state)

			//TODO: set to previous value instead ?
			state.symbolicData.SetMostSpecificNodeValue(lhs, __rhs)
			state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
		case *parse.MemberExpression:
			object, err := _symbolicEval(lhs.Left, state, evalOptions{
				doubleColonExprAncestorChain: []parse.Node{node},
			})
			if err != nil {
				return nil, err
			}

			if n.Err != nil {
				return nil, nil
			}

			var iprops IProps
			isAnySerializable := false
			{
				value := object
				// if sharedVal, isSharedVal := object.(*SharedValue); isSharedVal {
				// 	value = sharedVal.value
				// }
				switch val := value.(type) {
				case IProps:
					iprops = val
				case *Any:
					return nil, nil //no check
				case *AnySerializable:
					isAnySerializable = true
					//checked later

					//no check for watchable ?
				case nil:
					return nil, errors.New("nil value")
				default:
					state.addError(makeSymbolicEvalError(node, state, FmtCannotAssignPropertyOf(val)))
					iprops = &Object{}
				}
			}

			var expectedValue SymbolicValue
			static, ok := iprops.(IToStatic)
			if ok {
				expectedIprops, ok := AsIprops(static.Static().SymbolicValue()).(IProps)
				if ok && HasRequiredOrOptionalProperty(expectedIprops, lhs.PropertyName.Name) {
					expectedValue = expectedIprops.Prop(lhs.PropertyName.Name)
				}
			}

			rhs, deeperMismatch, err := getRHS(expectedValue)
			if err != nil {
				return nil, err
			}

			if _, ok := rhs.(Serializable); !ok && isAnySerializable {
				state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
				return nil, nil
			}

			propName := lhs.PropertyName.Name
			hasPrevValue := utils.SliceContains(iprops.PropertyNames(), propName)

			if hasPrevValue {
				prevValue := iprops.Prop(propName)
				state.symbolicData.SetMostSpecificNodeValue(lhs.PropertyName, prevValue)

				checkNotClonedObjectPropMutation(lhs, state)

				if _, ok := iprops.(Serializable); ok {
					if _, ok := rhs.(Serializable); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
						return nil, nil
					}
				}

				if _, ok := asWatchable(iprops).(Watchable); ok {
					if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
					}
				}

				if n.Operator.Int() {
					if _, ok := prevValue.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
					}
				} else if badIntOperationRHS {

				} else {
					if newIprops, err := iprops.SetProp(propName, rhs); err != nil {
						if !deeperMismatch {
							state.addError(makeSymbolicEvalError(node, state, err.Error()))
						}
					} else {
						narrowPath(lhs.Left, setExactValue, newIprops, state, 0)
					}
				}

			} else {
				checkNotClonedObjectPropMutation(lhs, state)

				nonSerializableErr := false
				if _, ok := iprops.(Serializable); ok {
					if _, ok := rhs.(Serializable); !ok {
						nonSerializableErr = true
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
						rhs = ANY_SERIALIZABLE
					}
				}

				if _, ok := asWatchable(iprops).(Watchable); ok && !nonSerializableErr {
					if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
					}
				}

				if newIprops, err := iprops.SetProp(propName, rhs); err != nil {
					if !deeperMismatch {
						state.addError(makeSymbolicEvalError(node, state, err.Error()))
					}
				} else {
					narrowPath(lhs.Left, setExactValue, newIprops, state, 0)
				}
			}

		case *parse.IdentifierMemberExpression:
			v, err := _symbolicEval(lhs.Left, state, evalOptions{
				doubleColonExprAncestorChain: []parse.Node{node},
			})
			if err != nil {
				return nil, err
			}

			for _, ident := range lhs.PropertyNames[:len(lhs.PropertyNames)-1] {
				v = symbolicMemb(v, ident.Name, false, lhs, state)
				state.symbolicData.SetMostSpecificNodeValue(ident, v)
			}

			var iprops IProps
			isAnySerializable := true
			{
				value := v
				// if sharedVal, isSharedVal := v.(*SharedValue); isSharedVal {
				// 	value = sharedVal.value
				// }
				switch val := value.(type) {
				case IProps:
					iprops = val
				case *Any:
					return nil, nil //no check
				case *AnySerializable:
					isAnySerializable = true
					//checked later

					//no check for watchable ?
				case nil:
					return nil, errors.New("nil value")
				default:
					state.addError(makeSymbolicEvalError(node, state, FmtCannotAssignPropertyOf(val)))
					iprops = &Object{}
				}
			}

			lastPropNameNode := lhs.PropertyNames[len(lhs.PropertyNames)-1]
			lastPropName := lastPropNameNode.Name
			hasPrevValue := utils.SliceContains(iprops.PropertyNames(), lastPropName)

			var expectedValue SymbolicValue
			static, ok := iprops.(IToStatic)
			if ok {
				expectedIprops, ok := AsIprops(static.Static().SymbolicValue()).(IProps)
				if ok && HasRequiredOrOptionalProperty(expectedIprops, lastPropName) {
					expectedValue = expectedIprops.Prop(lastPropName)
				}
			}

			rhs, deeperMismatch, err := getRHS(expectedValue)
			if err != nil {
				return nil, err
			}

			if _, ok := rhs.(Serializable); !ok && isAnySerializable {
				state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
				return nil, nil
			}

			if hasPrevValue {
				prevValue := iprops.Prop(lastPropName)
				state.symbolicData.SetMostSpecificNodeValue(lastPropNameNode, prevValue)

				checkNotClonedObjectPropMutation(lhs, state)

				if _, ok := iprops.(Serializable); ok {

					if _, ok := rhs.(Serializable); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
						return nil, nil
					}
				}

				if _, ok := asWatchable(iprops).(Watchable); ok {
					if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
					}
				}

				if _, ok := prevValue.(*Int); !ok && n.Operator.Int() {
					state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
				} else {
					if newIprops, err := iprops.SetProp(lastPropName, rhs); err != nil {
						if !deeperMismatch {
							state.addError(makeSymbolicEvalError(node, state, err.Error()))
						}
					} else {
						narrowPath(lhs, setExactValue, newIprops, state, 1)
					}
				}
			} else {
				checkNotClonedObjectPropMutation(lhs, state)

				nonSerializableErr := false
				if _, ok := iprops.(Serializable); ok {
					if _, ok := rhs.(Serializable); !ok {
						nonSerializableErr = true
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
						rhs = ANY_SERIALIZABLE
					}
				}

				if _, ok := asWatchable(iprops).(Watchable); ok && !nonSerializableErr {
					if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
						state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
					}
				}

				if newIprops, err := iprops.SetProp(lastPropName, rhs); err != nil {
					if !deeperMismatch {
						state.addError(makeSymbolicEvalError(node, state, err.Error()))
					}
				} else {
					narrowPath(lhs, setExactValue, newIprops, state, 1)
				}
			}
		case *parse.IndexExpression:
			index, err := symbolicEval(lhs.Index, state)
			if err != nil {
				return nil, err
			}

			intIndex, ok := index.(*Int)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtIndexIsNotAnIntButA(index)))
			}

			slice, err := _symbolicEval(lhs.Indexed, state, evalOptions{
				doubleColonExprAncestorChain: []parse.Node{node, lhs},
			})
			if err != nil {
				return nil, err
			}

			checkNotClonedObjectPropMutation(lhs, state)

			seq, isMutableSeq := asIndexable(slice).(MutableSequence)
			if isMutableSeq && (!seq.HasKnownLen() ||
				intIndex == nil ||
				!intIndex.hasValue ||
				(intIndex.value >= 0 && intIndex.value < int64(seq.KnownLen()))) {

				var seqElementAtIndex Serializable
				if intIndex != nil && intIndex.hasValue {
					seqElementAtIndex = seq.elementAt(int(intIndex.value)).(Serializable)
				}

				if IsReadonly(seq) {
					state.addError(makeSymbolicEvalError(n.Left, state, ErrReadonlyValueCannotBeMutated.Error()))
					break
				}

				//evaluate right
				var deeperMismatch bool
				{
					var expectedValue SymbolicValue = seqElementAtIndex
					if expectedValue == nil {
						expectedValue = seq.element()
					}
					_, deeperMismatch, err = getRHS(expectedValue)
					if err != nil {
						return nil, err
					}
				}

				//-----------------------------------------
				if _, ok := slice.(Serializable); ok {
					if _, ok := __rhs.(Serializable); !ok {
						state.addError(makeSymbolicEvalError(node, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
						break
					}
				}

				if _, ok := asWatchable(slice).(Watchable); ok {
					if _, ok := asWatchable(__rhs).(Watchable); !ok && __rhs.IsMutable() {
						state.addError(makeSymbolicEvalError(node, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
						break
					}
				}

				ignoreNextAssignabilityError := false

				if n.Operator.Int() {
					if seqElementAtIndex != nil {
						if !ANY_INT.Test(seqElementAtIndex) {
							state.addError(makeSymbolicEvalError(lhs, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
							ignoreNextAssignabilityError = true
						}
						//note: the element is widened in order to support multivalues such as (1 | 2)
					} else if !ANY_INT.Test(widenToSameStaticTypeInMultivalue(seq.element())) {

						state.addError(makeSymbolicEvalError(lhs, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
						ignoreNextAssignabilityError = true
					}
				}

				if seqElementAtIndex == nil || !seqElementAtIndex.Test(__rhs) {
					assignable := false
					var staticSeq MutableLengthSequence
					var staticSeqElement SymbolicValue

					//get static
					static, ofConstant, ok := state.getInfoOfNode(lhs.Indexed)

					if ok && !ofConstant {
						staticSeq, ok = static.SymbolicValue().(MutableLengthSequence)

						if !ok {
							goto add_assignability_error
						}

						if staticSeq.HasKnownLen() {
							if intIndex != nil && intIndex.hasValue && staticSeq.KnownLen() > int(intIndex.value) {
								//known index
								staticSeqElement = staticSeq.elementAt(int(intIndex.value)).(Serializable)
								if staticSeqElement.Test(__rhs) {
									assignable = true
									narrowPath(lhs.Indexed, setExactValue, staticSeq, state, 0)
								}
							} else {
								state.addError(makeSymbolicEvalError(n.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENT))
								ignoreNextAssignabilityError = true
								staticSeqElement = staticSeq.element()
							}
						} else {
							staticSeqElement = staticSeq.element()
							if staticSeqElement.Test(widenToSameStaticTypeInMultivalue(__rhs)) {
								assignable = true
								narrowPath(lhs.Indexed, setExactValue, staticSeq, state, 0)
							}
						}
					}

				add_assignability_error:
					if !assignable && !ignoreNextAssignabilityError && !deeperMismatch {
						if staticSeq != nil {
							state.addError(makeSymbolicEvalError(n.Right, state, fmtNotAssignableToElementOfValue(__rhs, staticSeqElement)))
						} else {
							state.addError(makeSymbolicEvalError(n.Right, state, fmtNotAssignableToElementOfValue(__rhs, seq.element())))
						}
					}
				}
			} else if isMutableSeq && intIndex != nil && intIndex.hasValue && seq.HasKnownLen() {
				state.addError(makeSymbolicEvalError(lhs.Index, state, INDEX_IS_OUT_OF_BOUNDS))
			} else {
				state.addError(makeSymbolicEvalError(lhs.Indexed, state, fmtXisNotAMutableSequence(slice)))
				slice = NewListOf(ANY_SERIALIZABLE)
			}

			return nil, nil
		case *parse.SliceExpression:
			startIndex, err := symbolicEval(lhs.StartIndex, state)
			if err != nil {
				return nil, err
			}

			endIndex, err := symbolicEval(lhs.EndIndex, state)
			if err != nil {
				return nil, err
			}

			startIntIndex, ok := startIndex.(*Int)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtStartIndexIsNotAnIntButA(startIndex)))
			}

			endIntIndex, ok := endIndex.(*Int)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtEndIndexIsNotAnIntButA(endIndex)))
			}

			if startIntIndex != nil && endIntIndex != nil && startIntIndex.hasValue && endIntIndex.hasValue &&
				endIntIndex.value < startIntIndex.value {
				state.addError(makeSymbolicEvalError(lhs.EndIndex, state, END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX))
			}

			slice, err := _symbolicEval(lhs.Indexed, state, evalOptions{
				doubleColonExprAncestorChain: []parse.Node{node, lhs},
			})
			if err != nil {
				return nil, err
			}

			checkNotClonedObjectPropMutation(lhs, state)

			seq, isMutableSeq := slice.(MutableSequence)
			if isMutableSeq && (!seq.HasKnownLen() ||
				startIntIndex == nil ||
				!startIntIndex.hasValue ||
				(startIntIndex.value >= 0 && startIntIndex.value < int64(seq.KnownLen()))) {

				if IsReadonly(seq) {
					state.addError(makeSymbolicEvalError(n.Left, state, ErrReadonlyValueCannotBeMutated.Error()))
					break
				}

				//in order to simplify the validation logic the assignment is considered valid
				//if and only if the static type of LHS is a sequence of unknown length whose .element() matches the
				//elements of RHS.

				// get static
				var lhsInfo *varSymbolicInfo

				switch indexed := lhs.Indexed.(type) {
				case *parse.Variable:
					info, ok := state.getLocal(indexed.Name)
					if !ok {
						break
					}
					lhsInfo = &info
				case *parse.GlobalVariable:
					info, ok := state.getGlobal(indexed.Name)
					if !ok {
						break
					}
					lhsInfo = &info
				case *parse.IdentifierLiteral:
					info, ok := state.get(indexed.Name)
					if !ok {
						break
					}
					lhsInfo = &info
				}

				assignable := false
				ignoreNextAssignabilityError := false
				invalidRHSLength := false
				var staticSeq MutableSequence
				var rightSeqElement SymbolicValue
				var deeperMismatch bool

				//get static
				if lhsInfo != nil {
					info := *lhsInfo
					staticSeq, ok = info.static.SymbolicValue().(MutableSequence)
					if !ok {
						goto add_slice_assignability_error
					}
				}

				if staticSeq == nil {
					staticSeq = seq
				}

				{
					//evaluate right
					var expectedValue SymbolicValue = NewAnySequenceOf(staticSeq.element())
					_, deeperMismatch, err = getRHS(expectedValue)
					if err != nil {
						return nil, err
					}

					rightSeq, ok := __rhs.(Sequence)
					if !ok {
						state.addError(makeSymbolicEvalError(n.Right, state, fmtSequenceExpectedButIs(__rhs)))
						break
					}

					//---------------------------
					if _, ok := slice.(Serializable); ok {
						if _, ok := __rhs.(Serializable); !ok {
							state.addError(makeSymbolicEvalError(node, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
							break
						}
					}

					if _, ok := asWatchable(slice).(Watchable); ok {
						if _, ok := asWatchable(__rhs).(Watchable); !ok && __rhs.IsMutable() {
							state.addError(makeSymbolicEvalError(node, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
							break
						}
					}

					if rightSeq.HasKnownLen() && startIntIndex != nil && endIntIndex != nil &&
						startIntIndex.hasValue && endIntIndex.hasValue && startIntIndex.value >= 0 &&
						endIntIndex.value >= startIntIndex.value && endIntIndex.value-startIntIndex.value != int64(rightSeq.KnownLen()) {
						expectedLength := endIntIndex.value - startIntIndex.value
						invalidRHSLength = true
						state.addError(makeSymbolicEvalError(n.Right, state, fmtRHSSequenceShouldHaveLenOf(int(expectedLength))))
					}

					rightSeqElement = rightSeq.element()
				}

				if staticSeq.HasKnownLen() {
					if !invalidRHSLength && endIntIndex != nil && endIntIndex.hasValue && startIntIndex != nil && startIntIndex.hasValue &&
						staticSeq.KnownLen() > int(startIntIndex.value) {
						//conservatively assume not assignable
					} else {
						state.addError(makeSymbolicEvalError(n.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENTS))
						ignoreNextAssignabilityError = true
					}
				} else {
					staticSeqElement := staticSeq.element()
					if staticSeqElement.Test(widenToSameStaticTypeInMultivalue(rightSeqElement)) {
						assignable = true
						narrowPath(lhs.Indexed, setExactValue, staticSeq, state, 0)
					}
				}

			add_slice_assignability_error:
				if !assignable && !ignoreNextAssignabilityError && !deeperMismatch {
					if staticSeq != nil {
						state.addError(makeSymbolicEvalError(n.Right, state, fmtSeqOfXNotAssignableToSliceOfTheValue(rightSeqElement, staticSeq)))
					} else {
						state.addError(makeSymbolicEvalError(n.Right, state, fmtSeqOfXNotAssignableToSliceOfTheValue(rightSeqElement, seq)))
					}
				}

			} else if isMutableSeq && startIntIndex != nil && startIntIndex.hasValue && seq.HasKnownLen() {
				state.addError(makeSymbolicEvalError(lhs.StartIndex, state, START_INDEX_IS_OUT_OF_BOUNDS))
			} else {
				state.addError(makeSymbolicEvalError(lhs.Indexed, state, fmtMutableSequenceExpectedButIs(slice)))
				slice = NewListOf(ANY_SERIALIZABLE)
			}

			return nil, nil
		default:
			return nil, fmt.Errorf("invalid assignment: left hand side is a(n) %T", n.Left)
		}

		return nil, nil
	case *parse.MultiAssignment:
		isNillable := n.Nillable
		right, err := symbolicEval(n.Right, state)
		startRight := right

		if err != nil {
			return nil, err
		}

		seq, ok := right.(Sequence)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtSeqExpectedButIs(startRight)))
			right = &List{generalElement: ANY_SERIALIZABLE}

			for _, var_ := range n.Variables {
				name := var_.(*parse.IdentifierLiteral).Name

				if !state.hasLocal(name) {
					state.setLocal(name, ANY, nil, var_)
				}
				state.symbolicData.SetMostSpecificNodeValue(var_, ANY)
			}
		} else {
			if seq.HasKnownLen() && seq.KnownLen() < len(n.Variables) && !isNillable {
				state.addError(makeSymbolicEvalError(node, state, fmtListShouldHaveLengthGreaterOrEqualTo(len(n.Variables))))
			}

			for i, var_ := range n.Variables {
				name := var_.(*parse.IdentifierLiteral).Name

				val := seq.elementAt(i)
				if isNillable && (!seq.HasKnownLen() || i >= seq.KnownLen() && isNillable) {
					val = joinValues([]SymbolicValue{val, Nil})
				}

				if state.hasLocal(name) {
					state.updateLocal(name, val, n)
				} else {
					state.setLocal(name, val, nil, var_)
				}
				state.symbolicData.SetMostSpecificNodeValue(var_, val)
			}
		}

		state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		return nil, nil
	case *parse.HostAliasDefinition:
		name := n.Left.Value[1:]
		value, err := symbolicEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		if host, ok := value.(*Host); ok {
			state.ctx.AddHostAlias(name, host, state.inPreinit)
			state.symbolicData.SetMostSpecificNodeValue(n.Left, host)
		} else {
			state.addError(makeSymbolicEvalError(node, state, fmtCannotCreateHostAliasWithA(value)))
			state.ctx.AddHostAlias(name, &Host{}, state.inPreinit)
		}

		return nil, nil
	case *parse.Chunk:
		manageLocalScope := !n.IsShellChunk && len(state.chunkStack) <= 1

		if manageLocalScope {
			state.scopeStack = state.scopeStack[:1] //we only keep the global scope
			state.pushScope()
		}

		state.returnValue = nil
		defer func() {
			state.returnValue = nil
			state.iterationChange = NoIterationChange
			if manageLocalScope {
				state.popScope()
			}
		}()

		if self := state.topLevelSelf; self != nil {
			state.setSelf(self)
			defer state.unsetSelf()
		}

		//evaluation of constants
		if n.GlobalConstantDeclarations != nil {
			for _, decl := range n.GlobalConstantDeclarations.Declarations {
				constVal, err := symbolicEval(decl.Right, state)
				if err != nil {
					return nil, err
				}
				state.symbolicData.SetMostSpecificNodeValue(decl.Left, constVal)
				if !state.setGlobal(decl.Ident().Name, constVal, GlobalConst, decl.Left) {
					return nil, fmt.Errorf("failed to set global '%s'", decl.Ident().Name)
				}
			}
		}

		//evaluation of preinit block
		if n.Preinit != nil {
			state.inPreinit = true
			for _, stmt := range n.Preinit.Block.Statements {
				_, err := symbolicEval(stmt, state)

				if err != nil {
					return nil, err
				}
				if state.returnValue != nil {
					return nil, fmt.Errorf("preinit block should not return")
				}
			}
			state.inPreinit = false
		}

		// evaluation of manifest, this is performed only to get symbolic data
		if n.Manifest != nil {
			manifestObject, err := symbolicEval(n.Manifest.Object, state)
			if err != nil {
				return nil, err
			}

			//if the manifest object has the correct AND the module arguments variable is not already defined
			//we read the type & name of parameters and we set the module arguments variable.
			if object, ok := manifestObject.(*Object); ok && !state.hasGlobal(extData.MOD_ARGS_VARNAME) {
				parameters := getModuleParameters(object, n.Manifest.Object.(*parse.ObjectLiteral))
				args := make(map[string]SymbolicValue)

				var paramNames []string
				var paramPatterns []Pattern

				for _, param := range parameters {
					paramNames = append(paramNames, param.name)
					paramPatterns = append(paramPatterns, param.pattern)
					args[param.name] = param.pattern.SymbolicValue()
				}

				structType := NewStructPattern("", ulid.Make(), paramNames, paramPatterns)

				if !state.setGlobal(extData.MOD_ARGS_VARNAME, NewStruct(structType, args), GlobalConst) {
					panic(ErrUnreachable)
				}
			}
		}

		state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
		state.symbolicData.SetContextData(n, state.ctx.currentData())

		//evaluation of statements
		if len(n.Statements) == 1 {
			res, err := symbolicEval(n.Statements[0], state)
			if err != nil {
				return nil, err
			}
			if state.returnValue != nil && !state.conditionalReturn {
				return state.returnValue, nil
			}

			if res == nil && state.returnValue != nil {
				return joinValues([]SymbolicValue{state.returnValue, Nil}), nil
			}
			return res, nil
		}

		var returnValue SymbolicValue
		for _, stmt := range n.Statements {
			_, err := symbolicEval(stmt, state)

			if err != nil {
				return nil, err
			}
			if state.returnValue != nil {
				if state.conditionalReturn {
					returnValue = state.returnValue
					continue
				}
				return state.returnValue, nil
			}
		}

		return returnValue, nil
	case *parse.EmbeddedModule:
		return &AstNode{Node: n.ToChunk()}, nil
	case *parse.Block:
		for _, stmt := range n.Statements {
			_, err := symbolicEval(stmt, state)
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	case *parse.SynchronizedBlockStatement:
		for _, valNode := range n.SynchronizedValues {
			val, err := symbolicEval(valNode, state)
			if err != nil {
				return nil, err
			}

			if !val.IsMutable() {
				continue
			}

			if potentiallySharable, ok := val.(PotentiallySharable); !ok || !utils.Ret0(potentiallySharable.IsSharable()) {
				state.addError(makeSymbolicEvalError(node, state, fmtSynchronizedValueShouldBeASharableValueOrImmutableNot(val)))
			}
		}

		if n.Block == nil {
			return nil, nil
		}

		_, err := symbolicEval(n.Block, state)
		if err != nil {
			return nil, err
		}

		return nil, nil
	case *parse.PermissionDroppingStatement:
		return nil, nil
	case *parse.InclusionImportStatement:
		if state.Module == nil {
			panic(fmt.Errorf("cannot evaluate inclusion import statement: global state's module is nil"))
		}
		chunk, ok := state.Module.inclusionStatementMap[n]
		if !ok { //included file does not exist or is a folder
			return nil, nil
		}
		state.pushChunk(chunk.ParsedChunk, n)
		defer state.popChunk()

		_, err := symbolicEval(chunk.Node, state)
		state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
		state.symbolicData.SetContextData(n, state.ctx.currentData())
		return nil, err
	case *parse.ImportStatement:
		value := ANY
		state.setGlobal(n.Identifier.Name, value, GlobalConst)

		state.symbolicData.SetMostSpecificNodeValue(n.Identifier, value)
		state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())

		var pathOrURL string

		switch src := n.Source.(type) {
		case *parse.RelativePathLiteral:
			pathOrURL = src.Value
		case *parse.AbsolutePathLiteral:
			pathOrURL = src.Value
		case *parse.URLLiteral:
			pathOrURL = src.Value
		default:
			panic(ErrUnreachable)
		}

		if !strings.HasSuffix(pathOrURL, ".ix") {
			state.addError(makeSymbolicEvalError(n.Source, state, IMPORTED_MOD_PATH_MUST_END_WITH_IX))
			return nil, nil
		}

		importedModule, ok := state.Module.directlyImportedModules[n]
		if !ok {
			return nil, nil
		}

		//TODO: use concrete context with permissions of imported module
		importedModuleContext := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx)
		for name, basePattern := range state.basePatterns {
			importedModuleContext.AddNamedPattern(name, basePattern, false)
		}

		for name, basePatternNamespace := range state.basePatternNamespaces {
			importedModuleContext.AddPatternNamespace(name, basePatternNamespace, false)
		}

		importPositions := append(utils.CopySlice(state.importPositions), state.getErrorMesssageLocation(n)...)

		data, err := SymbolicEvalCheck(SymbolicEvalCheckInput{
			Node:   importedModule.mainChunk.Node,
			Module: importedModule,

			UseBaseGlobals:                true,
			SymbolicBaseGlobals:           state.baseGlobals,
			SymbolicBasePatterns:          state.basePatterns,
			SymbolicBasePatternNamespaces: state.basePatternNamespaces,
			Context:                       importedModuleContext,

			initialSymbolicData: state.symbolicData,
			importPositions:     importPositions,
		})

		if data == nil && err != nil {
			return nil, err
		}

		return nil, nil
	case *parse.SpawnExpression:
		var actualGlobals = map[string]SymbolicValue{}
		var embeddedModule *parse.Chunk

		var meta map[string]SymbolicValue
		var globals any

		if !state.ctx.HasAPermissionWithKindAndType(permkind.Create, permkind.ROUTINE_PERM_TYPENAME) {
			state.addWarning(makeSymbolicEvalWarning(n, state, POSSIBLE_MISSING_PERM_TO_CREATE_A_COROUTINE))
		}

		if n.Meta != nil {
			meta = map[string]SymbolicValue{}
			if objLit, ok := n.Meta.(*parse.ObjectLiteral); ok {

				for _, property := range objLit.Properties {
					propertyName := property.Name() //okay since implicit-key properties are not allowed

					if propertyName == "globals" {
						globalsObjectLit, ok := property.Value.(*parse.ObjectLiteral)
						//handle description separately if it's an object literal because non-serializable value are not accepted.
						if ok {
							globalMap := map[string]SymbolicValue{}
							globals = globalMap

							for _, prop := range globalsObjectLit.Properties {
								globalName := prop.Name() //okay since implicit-key properties are not allowed
								globalVal, err := symbolicEval(prop.Value, state)
								if err != nil {
									return nil, err
								}
								globalMap[globalName] = globalVal
							}
							continue
						}
					}

					propertyVal, err := symbolicEval(property.Value, state)
					if err != nil {
						return nil, err
					}
					meta[propertyName] = propertyVal
				}
			}
		}

		// add constant globals from parent
		state.forEachGlobal(func(name string, info varSymbolicInfo) {
			if info.isConstant {
				actualGlobals[name] = info.value
			}
		})

		// add globals defined in the 'globals' section

		for k, v := range meta {
			switch k {
			case "globals":
				globals = v
			case "group":
				_, ok := v.(*RoutineGroup)
				if !ok {
					state.addError(makeSymbolicEvalError(n.Meta, state, fmtGroupPropertyNotRoutineGroup(v)))
				}
			case "allow":
			default:
				state.addWarning(makeSymbolicEvalWarning(n.Meta, state, fmtUnknownSectionInCoroutineMetadata(k)))
			}
		}

		switch g := globals.(type) {
		case map[string]SymbolicValue:
			for k, v := range g {
				symVal, err := ShareOrClone(v, state)
				if err != nil {
					state.addError(makeSymbolicEvalError(n.Meta, state, err.Error()))
					symVal = ANY
				}
				actualGlobals[k] = symVal
			}
		case *KeyList:
			for _, name := range g.Keys {
				info, ok := state.getGlobal(name)
				if ok {
					actualGlobals[name] = info.value
				} else {
					actualGlobals[name] = ANY
				}
			}
		case nil, *NilT:
			break
		default:
			return nil, fmt.Errorf("spawn expression: globals: only objects and keylists are supported, not %T", g)
		}

		v, err := symbolicEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		if symbolicNode, ok := v.(*AstNode); ok {
			if embeddedMod, ok := symbolicNode.Node.(*parse.Chunk); ok {
				embeddedModule = embeddedMod
			} else {
				varname := parse.GetVariableName(n.Module)
				state.addError(makeSymbolicEvalError(node, state, fmtValueOfVarShouldBeAModuleNode(varname)))
			}
		} else {
			varname := parse.GetVariableName(n.Module)
			state.addError(makeSymbolicEvalError(node, state, fmtValueOfVarShouldBeAModuleNode(varname)))
		}

		//TODO: check the allow section to know the permissions
		modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx)
		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		modState.Module = state.Module
		modState.symbolicData = state.symbolicData

		for k, v := range actualGlobals {
			modState.setGlobal(k, v, GlobalConst)
		}

		if n.Module.SingleCallExpr {
			calleeName := n.Module.Statements[0].(*parse.CallExpression).Callee.(*parse.IdentifierLiteral).Name
			info, ok := state.get(calleeName)
			if ok {
				modState.setGlobal(calleeName, info.value, GlobalConst)
			}
		}

		_, err = symbolicEval(embeddedModule, modState)
		if err != nil {
			return nil, err
		}

		for _, err := range modState.errors() {
			state.addError(err)
		}

		return &Routine{}, nil
	case *parse.MappingExpression:
		mapping := &Mapping{}

		for _, entry := range n.Entries {
			fork := state.fork()
			fork.pushScope()

			switch e := entry.(type) {
			case *parse.StaticMappingEntry:
				_, err := symbolicEval(e.Value, fork)
				if err != nil {
					return nil, err
				}
			case *parse.DynamicMappingEntry:
				key, err := symbolicEval(e.Key, fork)
				if err != nil {
					return nil, err
				}

				keyVarname := e.KeyVar.(*parse.IdentifierLiteral).Name
				keyVal := key
				if patt, ok := key.(Pattern); ok {
					keyVal = patt.SymbolicValue()
				}
				fork.setLocal(keyVarname, keyVal, nil, e.KeyVar)
				state.symbolicData.SetMostSpecificNodeValue(e.KeyVar, keyVal)

				if e.GroupMatchingVariable != nil {
					matchingVarName := e.GroupMatchingVariable.(*parse.IdentifierLiteral).Name
					anyObj := NewAnyObject()
					fork.setLocal(matchingVarName, anyObj, nil, e.GroupMatchingVariable)
					state.symbolicData.SetMostSpecificNodeValue(e.GroupMatchingVariable, anyObj)
				}

				_, err = symbolicEval(e.ValueComputation, fork)
				if err != nil {
					return nil, err
				}
			}
		}

		return mapping, nil
	case *parse.UDataLiteral:
		return &UData{}, nil
	case *parse.ComputeExpression:
		fork := state.fork()

		v, err := symbolicEval(n.Arg, fork)
		if err != nil {
			return nil, err
		}

		if !IsSimpleSymbolicInoxVal(v) {
			state.addError(makeSymbolicEvalError(n.Arg, state, INVALID_KEY_IN_COMPUTE_EXPRESSION_ONLY_SIMPLE_VALUE_ARE_SUPPORTED))
		}

		return ANY, nil
	case *parse.ObjectLiteral:
		entries := map[string]Serializable{}
		indexKey := 0

		var keys []string
		keyToProp := map[string]*parse.ObjectProperty{}
		keyIds := map[string]int{}
		idToKey := map[int64]string{}

		graph := simple.NewDirectedGraph()
		var selfDependent []string

		//first iteration of the properties: we get all keys
		for i, p := range n.Properties {
			var key string

			//add the key
			switch n := p.Key.(type) {
			case *parse.QuotedStringLiteral:
				key = n.Value
				_, err := strconv.ParseUint(key, 10, 32)
				if err == nil {
					//see Check function
					indexKey++
				}
			case *parse.IdentifierLiteral:
				key = n.Name
			case nil:
				key = strconv.Itoa(indexKey)
				indexKey++
			default:
				return nil, fmt.Errorf("invalid key type %T", n)
			}
			keys = append(keys, key)
			keyIds[key] = i
			keyToProp[key] = p
			idToKey[int64(i)] = key
			//

		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := symbolicEval(el.Expr, state)
			if err != nil {
				return nil, err
			}

			object := evaluatedElement.(*Object)

			for _, key := range el.Expr.(*parse.ExtractionExpression).Keys.Keys {
				name := key.(*parse.IdentifierLiteral).Name
				v, _, ok := object.GetProperty(name)
				if !ok {
					panic(fmt.Errorf("missing property %s", name))
				}

				serializable, ok := v.(Serializable)
				if !ok {
					state.addError(makeSymbolicEvalError(el, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
					serializable = ANY_SERIALIZABLE
				} else if _, ok := asWatchable(v).(Watchable); !ok && v.IsMutable() {
					state.addError(makeSymbolicEvalError(el, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
				}

				entries[name] = serializable
			}
		}

		if indexKey != 0 {
			// TODO: implicit prop count
		}

		//second iteration of the properties: we build a graph of dependencies
		for i, p := range n.Properties {
			propKey := keys[i]
			propKeyId := int64(i)

			if _, ok := p.Value.(*parse.FunctionExpression); !ok {
				continue
			}

			// find method's dependencies
			propNode, new := graph.NodeWithID(propKeyId)
			if new {
				graph.AddNode(propNode)
			}

			parse.Walk(p.Value, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

				if parse.IsScopeContainerNode(node) && node != p.Value {
					return parse.Prune, nil
				}

				switch node.(type) {
				case *parse.SelfExpression:
					propName := ""

					switch p := parent.(type) {
					case *parse.MemberExpression:
						propName = p.PropertyName.Name
					case *parse.DynamicMemberExpression:
						propName = p.PropertyName.Name
					}

					if propName != "" {

						keyId, ok := keyIds[propName]
						if !ok {
							//?
							return parse.Continue, nil
						}

						otherNode, new := graph.NodeWithID(int64(keyId))

						if new {
							graph.AddNode(otherNode)
						}

						if keyId == int(propKeyId) {
							selfDependent = append(selfDependent, propKey)
						} else if !graph.HasEdgeFromTo(propKeyId, otherNode.ID()) {
							// otherNode -<- propNode (propNode depends on otherNode)
							edge := graph.NewEdge(propNode, otherNode)
							graph.SetEdge(edge)
						}
					}
				}
				return parse.Continue, nil
			}, nil)
		}

		// we sort the keys based on the dependency graph

		var dependencyChainCountsCache = make(map[int64]int, len(keys))
		var getDependencyChainDepth func(int64, []int64) int
		var cycles [][]string

		getDependencyChainDepth = func(nodeId int64, chain []int64) int {
			for _, id := range chain {
				if nodeId == id && len(chain) >= 1 {
					cycle := make([]string, 0, len(chain))

					for _, id := range chain {
						cycle = append(cycle, "."+keys[id])
					}
					cycles = append(cycles, cycle)
					return 0
				}
			}

			chain = append(chain, nodeId)

			if v, ok := dependencyChainCountsCache[nodeId]; ok {
				return v
			}

			depth_ := 0
			directDependencies := graph.From(nodeId)

			for directDependencies.Next() {
				dep := directDependencies.Node()
				count := 1 + getDependencyChainDepth(dep.(simple.Node).ID(), chain)
				if count > depth_ {
					depth_ = count
				}
			}

			dependencyChainCountsCache[nodeId] = depth_
			return depth_
		}

		expectedObj, ok := findInMultivalue[*Object](options.expectedValue)
		if !ok {
			expectedObj = &Object{}
		}

		sort.Slice(keys, func(i, j int) bool {
			keyA := keys[i]
			keyB := keys[j]
			idA := int64(keyIds[keyA])
			idB := int64(keyIds[keyB])

			// we move all implicit lifetime jobs at the end
			p1 := keyToProp[keyA]
			if _, ok := p1.Value.(*parse.LifetimejobExpression); ok && p1.HasImplicitKey() {
				return false
			}
			p2 := keyToProp[keyB]
			if _, ok := p2.Value.(*parse.LifetimejobExpression); ok && p2.HasImplicitKey() {
				return true
			}

			return getDependencyChainDepth(idA, nil) < getDependencyChainDepth(idB, nil)
		})

		obj := NewInexactObject(entries, nil, nil)
		if expectedObj.readonly {
			obj.readonly = true
		}

		if len(cycles) > 0 {
			state.addError(makeSymbolicEvalError(node, state, fmtMethodCyclesDetected(cycles)))
			return ANY_OBJ, nil
		}

		prevNextSelf, restoreNextSelf := state.getNextSelf()
		if restoreNextSelf {
			state.unsetNextSelf()
		}
		state.setNextSelf(obj)

		//add allowed missing properties
		{
			var properties []string
			expectedObj.ForEachEntry(func(propName string, propValue SymbolicValue) error {
				if slices.Contains(keys, propName) {
					return nil
				}
				properties = append(properties, propName)
				return nil
			})
			state.symbolicData.SetAllowedNonPresentProperties(n, properties)
		}

		//evaluate properties
		for _, key := range keys {
			p := keyToProp[key]

			var static Pattern

			expectedPropVal := expectedObj.entries[key]
			deeperMismatch := false

			if p.Key != nil && expectedObj.exact && expectedPropVal == nil {
				closest, _, ok := utils.FindClosestString(state.ctx.startingConcreteContext, maps.Keys(expectedObj.entries), p.Name(), 2)
				options.setActualValueMismatchIfNotNil()

				msg := ""
				if ok {
					msg = fmtUnexpectedPropertyDidYouMeanElse(key, closest)
				} else {
					msg = fmtUnexpectedProperty(key)
				}

				state.addError(makeSymbolicEvalError(p.Key, state, msg))
			}

			propVal, err := _symbolicEval(p.Value, state, evalOptions{expectedValue: expectedPropVal, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}

			if p.Type != nil {
				_propType, err := symbolicEval(p.Type, state)
				if err != nil {
					return nil, err
				}
				static = _propType.(Pattern)
				if !static.TestValue(propVal) {
					state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfType(propVal, static)))
					propVal = static.SymbolicValue()
				}
			} else if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(propVal) {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfExpectedValue(propVal, expectedPropVal)))
			}

			serializable, ok := propVal.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				serializable = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(propVal).(Watchable); !ok && propVal.IsMutable() {
				state.addError(makeSymbolicEvalError(p, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
			}

			//additional checks if expected object is readonly
			if expectedObj.readonly {
				if _, ok := propVal.(*LifetimeJob); ok {
					state.addError(makeSymbolicEvalError(p, state, LIFETIME_JOBS_NOT_ALLOWED_IN_READONLY_OBJECTS))
				} else if !IsReadonlyOrImmutable(propVal) {
					state.addError(makeSymbolicEvalError(p.Key, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE))
				}
			}

			obj.initNewProp(key, serializable, static)
			state.symbolicData.SetMostSpecificNodeValue(p.Key, propVal)
		}
		state.unsetNextSelf()
		if restoreNextSelf {
			state.setNextSelf(prevNextSelf)
		}

		// evaluate meta properties

		for _, p := range n.MetaProperties {
			switch p.Name() {
			case extData.CONSTRAINTS_KEY:
				if err := handleConstraints(obj, p.Initialization, state); err != nil {
					return nil, err
				}
			case extData.VISIBILITY_KEY:
				//
			default:
				state.addError(makeSymbolicEvalError(p, state, fmtCannotInitializedMetaProp(p.Name())))
			}
		}

		return obj, nil
	case *parse.RecordLiteral:
		entries := map[string]Serializable{}
		rec := NewBoundEntriesRecord(entries)

		var keys []string

		//get keys

		indexKey := 0
		for _, p := range n.Properties {
			var key string

			switch n := p.Key.(type) {
			case *parse.QuotedStringLiteral:
				key = n.Value
				_, err := strconv.ParseUint(key, 10, 32)
				if err == nil {
					//see Check function
					indexKey++
				}
			case *parse.IdentifierLiteral:
				key = n.Name
			case nil:
				key = strconv.Itoa(indexKey)
				indexKey++
			default:
				return nil, fmt.Errorf("invalid key type %T", n)
			}

			keys = append(keys, key)
		}

		expectedRecord, ok := findInMultivalue[*Record](options.expectedValue)
		if ok && expectedRecord.entries != nil {
			var properties []string
			expectedRecord.ForEachEntry(func(propName string, _ SymbolicValue) error {
				if slices.Contains(keys, propName) {
					return nil
				}
				properties = append(properties, propName)
				return nil
			})

			state.symbolicData.SetAllowedNonPresentProperties(n, properties)
		} else {
			expectedRecord = &Record{}
		}

		//evaluate values
		for i, p := range n.Properties {
			key := keys[i]

			expectedPropVal := expectedRecord.entries[key]
			deeperMismatch := false

			v, err := _symbolicEval(p.Value, state, evalOptions{expectedValue: expectedPropVal, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}

			if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(v) {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfExpectedValue(v, expectedPropVal)))
			}

			if v.IsMutable() {
				state.addError(makeSymbolicEvalError(p.Value, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable(key)))
				entries[key] = ANY_SERIALIZABLE
			} else {
				entries[key] = v.(Serializable)
			}
		}

		for _, el := range n.SpreadElements {
			state.addError(makeSymbolicEvalError(el, state, PROP_SPREAD_IN_REC_NOT_SUPP_YET))
			break
			// evaluatedElement, err := symbolicEval(el.Expr, state)
			// if err != nil {
			// 	return nil, err
			// }

			// object := evaluatedElement.(*SymbolicObject)

			// for _, key := range el.Expr.(*parse.ExtractionExpression).Keys.Keys {
			// 	name := key.(*parse.IdentifierLiteral).Name
			// 	v, ok := object.getProperty(name)
			// 	if !ok {
			// 		panic(fmt.Errorf("missing property %s", name))
			// 	}
			// 	rec.updateProperty(name, v)
			// }
		}

		return rec, nil
	case *parse.ListLiteral:
		elements := make([]Serializable, 0)
		expectedList, _ := findInMultivalue[*List](options.expectedValue)

		if n.TypeAnnotation != nil {
			generalElemPattern, err := symbolicEval(n.TypeAnnotation, state)
			if err != nil {
				return nil, err
			}

			generalElem := generalElemPattern.(Pattern).SymbolicValue().(Serializable)
			resultList := NewListOf(generalElem)
			deeperMismatch := false

			for _, elemNode := range n.Elements {
				var e SymbolicValue

				spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
				if ok {
					val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: resultList, actualValueMismatch: &deeperMismatch})
					if err != nil {
						return nil, err
					}

					list, isList := val.(*List)
					if isList {
						e = list.element()
					} else {
						state.addError(makeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST))
						e = generalElem
					}
				} else {
					e, err = _symbolicEval(elemNode, state, evalOptions{expectedValue: generalElem, actualValueMismatch: &deeperMismatch})
					if err != nil {
						return nil, err
					}
				}

				if !generalElem.Test(e) && !deeperMismatch {
					state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListAnnotated(e, generalElemPattern.(Pattern))))
				}

				e = AsSerializable(e)
				_, ok = e.(Serializable)
				if !ok {
					state.addError(makeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
					e = ANY_SERIALIZABLE
				} else if _, ok := asWatchable(e).(Watchable); !ok && e.IsMutable() {
					state.addError(makeSymbolicEvalError(elemNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
				}
			}

			return resultList, nil
		}

		var expectedElement SymbolicValue = nil
		if expectedList != nil {
			expectedElement = expectedList.element()
		} else {
			//we do not search for a Sequence because we could find a sequence that is not a list
			expectedSeq, ok := findInMultivalue[*AnySequenceOf](options.expectedValue)
			if ok {
				expectedElement = expectedSeq.element()
			}
		}

		if len(n.Elements) == 0 {
			if expectedList != nil && expectedList.readonly {
				return EMPTY_READONLY_LIST, nil
			}
			return EMPTY_LIST, nil
		}

		for _, elemNode := range n.Elements {
			var e SymbolicValue
			deeperMismatch := false

			spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
			if ok {
				val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: expectedElement, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}

				list, isList := val.(*List)
				if isList {
					e = list.element()
				} else {
					state.addError(makeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST))
					if expectedElement != nil {
						e = expectedElement
					} else {
						continue
					}
				}
			} else {
				var err error
				e, err = _symbolicEval(elemNode, state, evalOptions{expectedValue: expectedElement, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}
			}

			if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedElement != nil && !expectedElement.Test(e) && !deeperMismatch {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListofValues(e, expectedElement)))
			}

			e = AsSerializable(e)
			_, ok = e.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
				e = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(e).(Watchable); !ok && e.IsMutable() {
				state.addError(makeSymbolicEvalError(elemNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
			}

			elements = append(elements, AsSerializable(e).(Serializable))
		}

		resultList := NewList(elements...)
		if expectedList != nil && expectedList.readonly {
			resultList.readonly = true
		}
		return resultList, nil
	case *parse.TupleLiteral:
		elements := make([]Serializable, 0)

		if n.TypeAnnotation != nil {
			generalElemPattern, err := symbolicEval(n.TypeAnnotation, state)
			if err != nil {
				return nil, err
			}

			generalElem := generalElemPattern.(Pattern).SymbolicValue().(Serializable)
			deeperMismatch := false

			for _, elemNode := range n.Elements {
				spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
				var e SymbolicValue
				if ok {
					val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: result, actualValueMismatch: &deeperMismatch})
					if err != nil {
						return nil, err
					}

					tuple, isTuple := val.(*Tuple)
					if isTuple {
						e = tuple.element()
					} else {
						state.addError(makeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_TUPLE))
						e = generalElem
					}
				} else {
					e, err = _symbolicEval(elemNode, state, evalOptions{expectedValue: generalElem, actualValueMismatch: &deeperMismatch})
					if err != nil {
						return nil, err
					}
				}

				if e.IsMutable() {
					state.addError(makeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE))
					e = ANY_SERIALIZABLE
				}

				e = AsSerializable(e)
				_, ok = e.(Serializable)
				if !ok {
					state.addError(makeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
					e = ANY_SERIALIZABLE
				}

				if !generalElem.Test(e) {
					state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(e, generalElemPattern.(Pattern))))
				}
			}

			return NewTupleOf(generalElem), nil
		}

		expectedTuple, ok := findInMultivalue[*Tuple](options.expectedValue)
		var expectedElement SymbolicValue = nil
		if ok {
			expectedElement = expectedTuple.element()
		} else {
			//we do not search for a Sequence because we could find a sequence that is not a tuple
			expectedSeq, ok := findInMultivalue[*AnySequenceOf](options.expectedValue)
			if ok {
				expectedElement = expectedSeq.element()
			}
		}

		if len(n.Elements) == 0 {
			return EMPTY_TUPLE, nil
		}

		for _, elemNode := range n.Elements {
			var e SymbolicValue
			deeperMismatch := false

			spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
			if ok {
				val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: expectedElement, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}

				list, isList := val.(*List)
				if isList {
					e = list.element()
				} else {
					state.addError(makeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST))
					if expectedElement != nil {
						e = expectedElement
					} else {
						continue
					}
				}
			} else {
				var err error
				e, err = _symbolicEval(elemNode, state, evalOptions{expectedValue: expectedElement, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}
			}

			if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedElement != nil && !expectedElement.Test(e) && !deeperMismatch {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListofValues(e, expectedElement)))
			}

			if e.IsMutable() {
				state.addError(makeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE))
				e = ANY_SERIALIZABLE
			}

			e = AsSerializable(e)
			_, ok = e.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
				e = ANY_SERIALIZABLE
			}

			elements = append(elements, AsSerializable(e).(Serializable))
		}
		return NewTuple(elements...), nil
	case *parse.DictionaryLiteral:
		entries := make(map[string]Serializable)
		keys := make(map[string]Serializable)

		expectedDictionary, ok := findInMultivalue[*Dictionary](options.expectedValue)
		if ok && expectedDictionary.entries != nil {
			var keys []string
			expectedDictionary.ForEachEntry(func(keyRepr string, _ SymbolicValue) error {
				if slices.Contains(keys, keyRepr) {
					return nil
				}
				keys = append(keys, keyRepr)
				return nil
			})

			state.symbolicData.SetAllowedNonPresentKeys(n, keys)
		} else {
			expectedDictionary = &Dictionary{}
		}

		for _, entry := range n.Entries {
			keyRepr := parse.SPrint(entry.Key, parse.PrintConfig{TrimStart: true})

			expectedEntryValue, _ := expectedDictionary.get(keyRepr)
			deeperMismatch := false

			v, err := _symbolicEval(entry.Value, state, evalOptions{expectedValue: expectedEntryValue, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}
			if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedEntryValue != nil && !deeperMismatch && !expectedEntryValue.Test(v) {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(entry.Value, state, fmtNotAssignableToEntryOfExpectedValue(v, expectedEntryValue)))
			}
			_, ok := v.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(entry.Value, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
				v = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(v).(Watchable); !ok && v.IsMutable() {
				state.addError(makeSymbolicEvalError(entry.Value, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
			}

			//TODO: refactor
			key, err := symbolicEval(entry.Key, state)
			_ = err

			_, ok = key.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(entry.Value, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
				key = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(key).(Watchable); !ok && key.IsMutable() {
				state.addError(makeSymbolicEvalError(entry.Value, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
			}

			entries[keyRepr] = v.(Serializable)
			keys[keyRepr] = key.(Serializable)
			state.symbolicData.SetMostSpecificNodeValue(entry.Key, key)
		}

		return NewDictionary(entries, keys), nil
	case *parse.IfStatement:
		test, err := symbolicEval(n.Test, state)
		if err != nil {
			return nil, err
		}

		if _, ok := test.(*Bool); !ok {
			state.addError(makeSymbolicEvalError(n.Test, state, fmtIfStmtTestNotBoolBut(test)))
		}

		if n.Consequent != nil {
			//consequent
			var consequentStateFork *State
			{
				consequentStateFork = state.fork()
				narrow(true, n.Test, state, consequentStateFork)
				state.symbolicData.SetLocalScopeData(n.Consequent, consequentStateFork.currentLocalScopeData())
				state.symbolicData.SetGlobalScopeData(n.Consequent, consequentStateFork.currentGlobalScopeData())

				_, err = symbolicEval(n.Consequent, consequentStateFork)
				if err != nil {
					return nil, err
				}
			}

			var alternateStateFork *State
			if n.Alternate != nil {
				alternateStateFork = state.fork()
				narrow(false, n.Test, state, alternateStateFork)
				state.symbolicData.SetLocalScopeData(n.Alternate, alternateStateFork.currentLocalScopeData())
				state.symbolicData.SetGlobalScopeData(n.Alternate, alternateStateFork.currentGlobalScopeData())

				_, err = symbolicEval(n.Alternate, alternateStateFork)
				if err != nil {
					return nil, err
				}
			}

			if alternateStateFork != nil {
				state.join(consequentStateFork, alternateStateFork)
			} else {
				state.join(consequentStateFork)
			}
		}

		return nil, nil
	case *parse.IfExpression:
		test, err := symbolicEval(n.Test, state)
		if err != nil {
			return nil, err
		}

		var consequentValue SymbolicValue
		var atlernateValue SymbolicValue

		if _, ok := test.(*Bool); ok {
			if n.Consequent != nil {
				consequentStateFork := state.fork()
				narrow(true, n.Test, state, consequentStateFork)
				state.symbolicData.SetLocalScopeData(n.Consequent, consequentStateFork.currentLocalScopeData())
				state.symbolicData.SetGlobalScopeData(n.Consequent, consequentStateFork.currentGlobalScopeData())

				consequentValue, err = symbolicEval(n.Consequent, consequentStateFork)
				if err != nil {
					return nil, err
				}

				var alternateStateFork *State
				if n.Alternate != nil {
					alternateStateFork := state.fork()
					narrow(false, n.Test, state, alternateStateFork)
					state.symbolicData.SetLocalScopeData(n.Alternate, alternateStateFork.currentLocalScopeData())
					state.symbolicData.SetGlobalScopeData(n.Alternate, alternateStateFork.currentGlobalScopeData())

					atlernateValue, err = symbolicEval(n.Alternate, alternateStateFork)
					if err != nil {
						return nil, err
					}
					return joinValues([]SymbolicValue{consequentValue, atlernateValue}), nil
				}

				if alternateStateFork != nil {
					state.join(consequentStateFork, alternateStateFork)
				} else {
					state.join(consequentStateFork)
				}

				return consequentValue, nil
			}
			return ANY, nil
		}

		state.addError(makeSymbolicEvalError(node, state, fmtIfExprTestNotBoolBut(test)))
		return ANY, nil
	case *parse.ForStatement:
		iteratedValue, err := symbolicEval(n.IteratedValue, state)
		if err != nil {
			return nil, err
		}

		var kVarname string
		var eVarname string

		if n.KeyIndexIdent != nil {
			kVarname = n.KeyIndexIdent.Name
		}
		if n.ValueElemIdent != nil {
			eVarname = n.ValueElemIdent.Name
		}

		var keyType SymbolicValue = ANY
		var valueType SymbolicValue = ANY

		if iterable, ok := asIterable(iteratedValue).(Iterable); ok {
			if n.Chunked {
				state.addError(makeSymbolicEvalError(node, state, "chunked iteration of iterables is not supported yet"))
			}

			keyType = iterable.IteratorElementKey()
			valueType = iterable.IteratorElementValue()
		} else if streamable, ok := asStreamable(iteratedValue).(StreamSource); ok {
			if n.KeyIndexIdent != nil {
				state.addError(makeSymbolicEvalError(n.KeyIndexIdent, state, KEY_VAR_SHOULD_BE_PROVIDED_ONLY_WHEN_ITERATING_OVER_AN_ITERABLE))
			}
			if n.Chunked {
				valueType = streamable.ChunkedStreamElement()
			} else {
				valueType = streamable.StreamElement()
			}
		} else {
			state.addError(makeSymbolicEvalError(node, state, fmtXisNotIterable(iteratedValue)))
		}

		if n.Body != nil {
			stateFork := state.fork()

			if n.KeyIndexIdent != nil {
				stateFork.setLocal(kVarname, keyType, nil, n.KeyIndexIdent)
				stateFork.symbolicData.SetMostSpecificNodeValue(n.KeyIndexIdent, keyType)
			}
			if n.ValueElemIdent != nil {
				stateFork.setLocal(eVarname, valueType, nil, n.ValueElemIdent)
				stateFork.symbolicData.SetMostSpecificNodeValue(n.ValueElemIdent, valueType)
			}

			stateFork.symbolicData.SetLocalScopeData(n.Body, stateFork.currentLocalScopeData())

			_, err = symbolicEval(n.Body, stateFork)
			if err != nil {
				return nil, err
			}

			state.join(stateFork)
			//we set the local scope data at the for statement, not the body
			state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		}

		return nil, nil
	case *parse.WalkStatement:
		walkedValue, err := symbolicEval(n.Walked, state)
		if err != nil {
			return nil, err
		}

		walkable, ok := walkedValue.(Walkable)

		var nodeMeta, entry SymbolicValue

		if ok {
			entry = walkable.WalkerElement()
			nodeMeta = walkable.WalkerNodeMeta()
		} else {
			state.addError(makeSymbolicEvalError(node, state, fmtXisNotWalkable(walkedValue)))
			entry = ANY
			nodeMeta = ANY
		}

		if n.Body != nil {
			stateFork := state.fork()

			stateFork.setLocal(n.EntryIdent.Name, entry, nil, n.EntryIdent)
			stateFork.symbolicData.SetMostSpecificNodeValue(n.EntryIdent, entry)

			if n.MetaIdent != nil {
				stateFork.setLocal(n.MetaIdent.Name, nodeMeta, nil, n.MetaIdent)
				stateFork.symbolicData.SetMostSpecificNodeValue(n.MetaIdent, nodeMeta)
			}

			stateFork.symbolicData.SetLocalScopeData(n.Body, stateFork.currentLocalScopeData())

			_, blkErr := symbolicEval(n.Body, stateFork)
			if blkErr != nil {
				return nil, blkErr
			}

			state.join(stateFork)
			//we set the local scope data at the for statement, not the body
			state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		}

		state.iterationChange = NoIterationChange
		return nil, nil
	case *parse.SwitchStatement:
		_, err := symbolicEval(n.Discriminant, state)
		if err != nil {
			return nil, err
		}

		var forks []*State

		for _, switchCase := range n.Cases {
			for _, valNode := range switchCase.Values {
				caseValue, err := symbolicEval(valNode, state)
				if err != nil {
					return nil, err
				}

				if switchCase.Block == nil {
					continue
				}

				blockStateFork := state.fork()
				forks = append(forks, blockStateFork)
				narrowPath(n.Discriminant, setExactValue, caseValue, blockStateFork, 0)

				_, err = symbolicEval(switchCase.Block, blockStateFork)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, defaultCase := range n.DefaultCases {
			if defaultCase.Block == nil {
				continue
			}

			blockStateFork := state.fork()
			forks = append(forks, blockStateFork)
			_, err = symbolicEval(defaultCase.Block, blockStateFork)
			if err != nil {
				return nil, err
			}
		}

		state.join(forks...)

		return nil, nil
	case *parse.MatchStatement:
		discriminant, err := symbolicEval(n.Discriminant, state)
		if err != nil {
			return nil, err
		}

		var forks []*State
		var possibleValues []SymbolicValue

		for _, matchCase := range n.Cases {
			for _, valNode := range matchCase.Values { //TODO: fix handling of multi cases
				if valNode.Base().Err != nil {
					continue
				}

				val, err := symbolicEval(valNode, state)
				if err != nil {
					return nil, err
				}

				pattern, ok := val.(Pattern)

				if !ok { //if the value of the case is not a pattern we just check for equality
					patt, err := NewExactValuePattern(val.(Serializable))
					if err == nil {
						pattern = patt
					} else {
						pattern = ANY_PATTERN
						state.addError(makeSymbolicEvalError(valNode, state, err.Error()))
					}
				}

				if matchCase.Block == nil {
					continue
				}

				blockStateFork := state.fork()
				forks = append(forks, blockStateFork)
				patternMatchingValue := pattern.SymbolicValue()
				possibleValues = append(possibleValues, patternMatchingValue)

				narrowPath(n.Discriminant, setExactValue, patternMatchingValue, blockStateFork, 0)

				if matchCase.GroupMatchingVariable != nil {
					variable := matchCase.GroupMatchingVariable.(*parse.IdentifierLiteral)
					groupPattern, ok := pattern.(GroupPattern)

					if !ok {
						state.addError(makeSymbolicEvalError(valNode, state, fmtXisNotAGroupMatchingPattern(pattern)))
					} else {
						ok, groups := groupPattern.MatchGroups(discriminant)
						if ok {
							groupsObj := NewInexactObject(groups, nil, nil)
							blockStateFork.setLocal(variable.Name, groupsObj, nil, matchCase.GroupMatchingVariable)
							state.symbolicData.SetMostSpecificNodeValue(variable, groupsObj)

							_, err := symbolicEval(matchCase.Block, blockStateFork)
							if err != nil {
								return nil, err
							}
						}
					}
				} else {
					_, err = symbolicEval(matchCase.Block, blockStateFork)
					if err != nil {
						return nil, err
					}
				}
			}
		}

		for _, defaultCase := range n.DefaultCases {
			blockStateFork := state.fork()
			forks = append(forks, blockStateFork)

			for _, val := range possibleValues {
				narrowPath(n.Discriminant, removePossibleValue, val, blockStateFork, 0)
			}

			_, err = symbolicEval(defaultCase.Block, blockStateFork)
			if err != nil {
				return nil, err
			}
		}

		state.join(forks...)

		return nil, nil
	case *parse.UnaryExpression:
		operand, err := symbolicEval(n.Operand, state)
		if err != nil {
			return nil, err
		}
		switch n.Operator {
		case parse.NumberNegate:
			switch operand.(type) {
			case *Int:
				return ANY_INT, nil
			case *Float:
				return &Float{}, nil
			default:
				_, ok := operand.(*Bool)
				if !ok {
					state.addError(makeSymbolicEvalError(node, state, fmtOperandOfNumberNegateShouldBeIntOrFloat(operand)))
				}
			}

			return ANY, nil
		case parse.BoolNegate:
			_, ok := operand.(*Bool)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtOperandOfBoolNegateShouldBeBool(operand)))
			}

			return ANY_BOOL, nil
		default:
			return nil, fmt.Errorf("invalid unary operator %d", n.Operator)
		}
	case *parse.BinaryExpression:

		left, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		right, err := symbolicEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		if multi, ok := left.(*Multivalue); ok {
			left = multi.WidenSimpleValues()
		}

		if multi, ok := right.(*Multivalue); ok {
			right = multi.WidenSimpleValues()
		}

		switch n.Operator {
		case parse.Add, parse.Sub, parse.Mul, parse.Div, parse.GreaterThan, parse.LessThan, parse.LessOrEqual, parse.GreaterOrEqual:

			if _, ok := left.(*Int); ok {
				_, ok = right.(*Int)
				if !ok {
					state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "int", Stringify(right))))
				}

				switch n.Operator {
				case parse.Add, parse.Sub, parse.Mul, parse.Div:
					return ANY_INT, nil
				default:
					return ANY_BOOL, nil
				}
			} else if _, ok := left.(*Float); ok {
				_, ok = right.(*Float)
				if !ok {
					state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "float", Stringify(right))))
				}
				switch n.Operator {
				case parse.Add, parse.Sub, parse.Mul, parse.Div:
					return ANY_FLOAT, nil
				default:
					return ANY_BOOL, nil
				}
			} else {
				state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "int or float", Stringify(left))))

				var arithmeticReturnVal SymbolicValue
				switch right.(type) {
				case *Int:
					arithmeticReturnVal = ANY_INT
				case *Float:
					arithmeticReturnVal = ANY_FLOAT
				default:
					state.addError(makeSymbolicEvalError(n.Left, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "int or float", Stringify(right))))
					arithmeticReturnVal = ANY
				}

				switch n.Operator {
				case parse.Add, parse.Sub, parse.Mul, parse.Div:
					return arithmeticReturnVal, nil
				default:
					return ANY_BOOL, nil
				}
			}

		case parse.AddDot, parse.SubDot, parse.MulDot, parse.DivDot, parse.GreaterThanDot, parse.GreaterOrEqualDot, parse.LessThanDot, parse.LessOrEqualDot:
			state.addError(makeSymbolicEvalError(node, state, "operator not implemented yet"))
			return ANY, nil
		case parse.Equal, parse.NotEqual, parse.Is, parse.IsNot:
			return ANY_BOOL, nil
		case parse.In:
			switch right.(type) {
			case Container:
			default:
				state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "container", Stringify(right))))
			}
			return ANY_BOOL, nil
		case parse.NotIn:
			switch right.(type) {
			case Container:
			default:
				state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "container", Stringify(right))))
			}
			return ANY_BOOL, nil
		case parse.Keyof:
			_, ok := left.(*String)
			if !ok {
				state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string", Stringify(left))))
			}

			switch rightVal := right.(type) {
			case *Object:
			default:
				state.addError(makeSymbolicEvalError(n.Right, state, fmtInvalidBinExprCannnotCheckNonObjectHasKey(rightVal)))
			}
			return ANY_BOOL, nil
		case parse.Range, parse.ExclEndRange:
			return &IntRange{}, nil
		case parse.And, parse.Or:
			_, ok := left.(*Bool)

			if !ok {
				state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "boolean", Stringify(left))))
			}

			_, ok = right.(*Bool)
			if !ok {
				state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "boolean", Stringify(right))))
			}
			return ANY_BOOL, nil
		case parse.Match, parse.NotMatch:
			_, ok := right.(Pattern)
			if !ok {
				state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "pattern", Stringify(right))))
			}

			return ANY_BOOL, nil
		case parse.Substrof:

			switch left.(type) {
			case *RuneSlice, *ByteSlice:
			default:
				if _, ok := left.(StringLike); !ok {
					state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string-like", Stringify(left))))
				}
			}

			switch right.(type) {
			case *RuneSlice, *ByteSlice:
			default:
				if _, ok := right.(StringLike); !ok {
					state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "string-like", Stringify(right))))
				}
			}

			return ANY_BOOL, nil
		case parse.SetDifference:
			if _, ok := left.(Pattern); !ok {
				state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "pattern", Stringify(left))))
			}
			return &DifferencePattern{
				Base:    ANY_PATTERN,
				Removed: ANY_PATTERN,
			}, nil
		case parse.NilCoalescing:
			return joinValues([]SymbolicValue{narrowOut(Nil, left), right}), nil
		default:
			return nil, fmt.Errorf(fmtInvalidBinaryOperator(n.Operator))
		}
	case *parse.UpperBoundRangeExpression:
		upperBound, err := symbolicEval(n.UpperBound, state)
		if err != nil {
			return nil, err
		}

		switch upperBound.(type) {
		case *Int:
			return ANY_INT_RANGE, nil
		case *Float:
			return nil, fmt.Errorf("floating point ranges not supported")
		default:
			return ANY_QUANTITY_RANGE, nil
		}
	case *parse.IntegerRangeLiteral:
		return ANY_INT_RANGE, nil
	case *parse.QuantityRangeLiteral:
		lowerBound, err := symbolicEval(n.LowerBound, state)
		if err != nil {
			return nil, err
		}

		element := lowerBound.WidestOfType()

		if n.UpperBound != nil {
			upperBound, err := symbolicEval(n.UpperBound, state)
			if err != nil {
				return nil, err
			}

			if !element.Test(upperBound) {
				state.addError(makeSymbolicEvalError(n.UpperBound, state, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_OF_SAME_TYPE_AS_LOWER_BOUND))
			}
		}

		return NewQuantityRange(element.(Serializable)), nil
	case *parse.RuneRangeExpression:
		return ANY_RUNE_RANGE, nil
	case *parse.FunctionExpression:
		stateFork := state.fork()

		//create a local scope for the function
		stateFork.pushScope()
		defer stateFork.popScope()

		if self, ok := state.getNextSelf(); ok {
			stateFork.setSelf(self)
			defer stateFork.unsetSelf()
		}

		var params []SymbolicValue
		var paramNames []string

		if len(n.Parameters) > 0 {
			params = make([]SymbolicValue, len(n.Parameters))
			paramNames = make([]string, len(n.Parameters))
		}

		//declare arguments
		for i, p := range n.Parameters[:n.NonVariadicParamCount()] {
			name := p.Var.Name
			var paramValue SymbolicValue = ANY
			var paramType Pattern

			if p.Type != nil {
				pattern, err := symbolicallyEvalPatternNode(p.Type, stateFork)
				if err != nil {
					return nil, err
				}
				paramType = pattern
				paramValue = pattern.SymbolicValue()
				state.symbolicData.SetMostSpecificNodeValue(p.Type, pattern)
			}

			stateFork.setLocal(name, paramValue, paramType, p.Var)
			state.symbolicData.SetMostSpecificNodeValue(p.Var, paramValue)
			params[i] = paramValue
			paramNames[i] = name
		}

		if state.recursiveFunctionName != "" {
			tempFn := &InoxFunction{
				node:           n,
				parameters:     params,
				parameterNames: paramNames,
				result:         ANY_SERIALIZABLE,
			}

			state.overrideGlobal(state.recursiveFunctionName, tempFn)
			stateFork.overrideGlobal(state.recursiveFunctionName, tempFn)
			state.recursiveFunctionName = ""
		}

		//declare captured locals
		capturedLocals := map[string]SymbolicValue{}
		for _, e := range n.CaptureList {
			name := e.(*parse.IdentifierLiteral).Name
			info, ok := state.getLocal(name)
			if ok {
				stateFork.setLocal(name, info.value, info.static, e)
				capturedLocals[name] = info.value
			} else {
				stateFork.setLocal(name, ANY, nil, e)
				capturedLocals[name] = ANY
				state.addError(makeSymbolicEvalError(e, state, fmtLocalVarIsNotDeclared(name)))
			}
		}

		if len(capturedLocals) == 0 {
			capturedLocals = nil
		}

		if n.IsVariadic {
			index := n.NonVariadicParamCount()
			variadicParam := n.VariadicParameter()
			paramNames[index] = variadicParam.Var.Name

			param := NewListOf(ANY_SERIALIZABLE)
			params[index] = param

			stateFork.setLocal(variadicParam.Var.Name, param, nil, variadicParam.Var)
		}
		stateFork.symbolicData.SetLocalScopeData(n.Body, stateFork.currentLocalScopeData())

		//-----------------------------

		var signatureReturnType SymbolicValue
		var storedReturnType SymbolicValue
		var err error

		if n.ReturnType != nil {
			pattern, err := symbolicallyEvalPatternNode(n.ReturnType, stateFork)
			if err != nil {
				return nil, err
			}
			signatureReturnType = pattern.SymbolicValue()
		}

		if n.Body == nil {
			goto return_function
		}

		if n.IsBodyExpression {
			storedReturnType, err = symbolicEval(n.Body, stateFork)
			if err != nil {
				return nil, err
			}

			if signatureReturnType != nil {
				storedReturnType = signatureReturnType
				if !signatureReturnType.Test(storedReturnType) {
					state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(storedReturnType, signatureReturnType)))
				}
			}
		} else {
			stateFork.returnType = signatureReturnType

			//execution of body

			_, err := symbolicEval(n.Body, stateFork)
			if err != nil {
				return nil, err
			}

			//check return
			retValue := stateFork.returnValue

			if signatureReturnType != nil {
				storedReturnType = signatureReturnType
				if retValue == nil {
					stateFork.addError(makeSymbolicEvalError(n, stateFork, MISSING_RETURN_IN_FUNCTION))
				} else if stateFork.conditionalReturn {
					stateFork.addError(makeSymbolicEvalError(n, stateFork, MISSING_UNCONDITIONAL_RETURN_IN_FUNCTION))
				}
			} else if retValue == nil {
				storedReturnType = Nil
			} else {
				storedReturnType = retValue
			}
		}

		if expectedFunction, ok := findInMultivalue[*InoxFunction](options.expectedValue); ok && expectedFunction.visitCheckNode != nil {
			visitCheckNode := expectedFunction.visitCheckNode

			parse.Walk(
				n.Body,
				func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
					if _, isBody := node.(*parse.Block); isBody && node == n.Body {
						return parse.Continue, nil
					}

					action, allowed, err := visitCheckNode(visitArgs{node, parent, scopeNode, ancestorChain, after}, expectedFunction.capturedLocals)
					if err != nil {
						return parse.StopTraversal, err
					}
					if !allowed {
						state.addError(makeSymbolicEvalError(node, state, THIS_EXPR_STMT_SYNTAX_IS_NOT_ALLOWED))
						options.setActualValueMismatchIfNotNil()
						return parse.Prune, nil
					}
					return action, nil
				},
				nil,
			)
		}

	return_function:
		return &InoxFunction{
			node:           n,
			parameters:     params,
			parameterNames: paramNames,
			result:         storedReturnType,
			capturedLocals: capturedLocals,
		}, nil
	case *parse.FunctionDeclaration:
		funcName := n.Name.Name

		//declare the function before checking it
		state.setGlobal(funcName, &InoxFunction{node: n.Function, result: ANY_SERIALIZABLE}, GlobalConst, n.Name)
		if state.recursiveFunctionName != "" {
			state.addError(makeSymbolicEvalError(n, state, NESTED_RECURSIVE_FUNCTION_DECLARATION))
		} else {
			state.recursiveFunctionName = funcName
		}

		v, err := symbolicEval(n.Function, state)
		if err == nil {
			state.overrideGlobal(funcName, v)
			state.symbolicData.SetMostSpecificNodeValue(n.Name, v)
			state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
		}
		return nil, err
	case *parse.ReadonlyPatternExpression:
		pattern, err := symbolicallyEvalPatternNode(n.Pattern, state)
		if err != nil {
			return nil, err
		}

		if !pattern.SymbolicValue().IsMutable() {
			return pattern, nil
		}

		potentiallyReadonlyPattern, ok := pattern.(PotentiallyReadonlyPattern)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Pattern, state, PATTERN_IS_NOT_CONVERTIBLE_TO_READONLY_VERSION))
			return pattern, nil
		}
		readonly, err := potentiallyReadonlyPattern.ToReadonlyPattern()
		if err != nil {
			state.addError(makeSymbolicEvalError(n.Pattern, state, err.Error()))
			return pattern, nil
		}
		return readonly, nil
	case *parse.FunctionPatternExpression:
		//KEEP IN SYNC WITH EVALUATION OF FUNCTION EXPRESSIONS

		stateFork := state.fork()

		//create a local scope for the function
		stateFork.pushScope()
		defer stateFork.popScope()

		if self, ok := state.getNextSelf(); ok {
			stateFork.setSelf(self)
			defer stateFork.unsetSelf()
		}

		parameterTypes := make([]SymbolicValue, len(n.Parameters))
		parameterNames := make([]string, len(n.Parameters))
		isVariadic := n.IsVariadic

		//declare arguments
		for paramIndex, p := range n.Parameters[:n.NonVariadicParamCount()] {
			name := p.Var.Name
			var paramType SymbolicValue = ANY

			if p.Type != nil {
				pattern, err := symbolicallyEvalPatternNode(p.Type, stateFork)
				if err != nil {
					return nil, err
				}
				paramType = pattern.SymbolicValue()
			}

			parameterTypes[paramIndex] = paramType
			parameterNames[paramIndex] = name

			stateFork.setLocal(name, paramType, nil, p.Var)
			state.symbolicData.SetMostSpecificNodeValue(p.Var, paramType)
		}

		if n.IsVariadic {
			variadicParam := n.VariadicParameter()
			paramValue := &List{generalElement: ANY_SERIALIZABLE}
			name := variadicParam.Var.Name

			parameterTypes[len(parameterTypes)-1] = paramValue
			parameterNames[len(parameterTypes)-1] = name

			stateFork.setLocal(name, paramValue, nil, variadicParam.Var)
			state.symbolicData.SetMostSpecificNodeValue(variadicParam.Var, paramValue)
		}

		//-----------------------------

		var signatureReturnType SymbolicValue
		var storedReturnType SymbolicValue
		var err error

		if n.ReturnType != nil {
			pattern, err := symbolicallyEvalPatternNode(n.ReturnType, stateFork)
			if err != nil {
				return nil, err
			}
			typ := pattern.SymbolicValue()
			signatureReturnType = typ
		}

		if n.IsBodyExpression {
			storedReturnType, err = symbolicEval(n.Body, stateFork)
			if err != nil {
				return nil, err
			}

			if signatureReturnType != nil {
				storedReturnType = signatureReturnType
				if !signatureReturnType.Test(storedReturnType) {
					state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(storedReturnType, signatureReturnType)))
				}
			}
		} else {
			stateFork.returnType = signatureReturnType

			//execution of body
			if n.Body != nil {
				_, err := symbolicEval(n.Body, stateFork)
				if err != nil {
					return nil, err
				}
			}

			//check return
			retValuePtr := stateFork.returnValue

			if signatureReturnType != nil {
				storedReturnType = signatureReturnType
				if retValuePtr == nil && n.Body != nil {
					stateFork.addError(makeSymbolicEvalError(n, stateFork, MISSING_RETURN_IN_FUNCTION_PATT))
				}
			} else if retValuePtr == nil {
				storedReturnType = Nil
			} else {
				storedReturnType = retValuePtr
			}
		}

		return &FunctionPattern{
			node:           n,
			returnType:     storedReturnType,
			parameters:     parameterTypes,
			parameterNames: parameterNames,
			isVariadic:     isVariadic,
		}, nil
	case *parse.PatternConversionExpression:
		v, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		if patt, ok := v.(Pattern); ok {
			return patt, nil
		}
		return &ExactValuePattern{value: v.(Serializable)}, nil
	case *parse.LazyExpression:
		return &AstNode{Node: n}, nil
	case *parse.MemberExpression:
		left, err := _symbolicEval(n.Left, state, evalOptions{
			doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), node),
		})
		if err != nil {
			return nil, err
		}

		if n.PropertyName == nil { //parsing error
			return ANY, nil
		}

		val := symbolicMemb(left, n.PropertyName.Name, n.Optional, n, state)
		if n.Optional {
			val = joinValues([]SymbolicValue{val, Nil})
		}

		state.symbolicData.SetMostSpecificNodeValue(n.PropertyName, val)

		return val, nil
	case *parse.ComputedMemberExpression:
		_, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		if n.PropertyName == nil { //parsing error
			return ANY, nil
		}

		computedPropertyName, err := symbolicEval(n.PropertyName, state)
		if err != nil {
			return nil, err
		}

		if _, ok := computedPropertyName.(StringLike); !ok {
			state.addError(makeSymbolicEvalError(n.PropertyName, state, fmtComputedPropNameShouldBeAStringNotA(computedPropertyName)))
		}

		return ANY, nil
	case *parse.IdentifierMemberExpression:
		v, err := _symbolicEval(n.Left, state, evalOptions{
			doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), node),
		})
		if err != nil {
			return nil, err
		}

		if n.Err != nil {
			return ANY, nil
		}

		var prevIdent *parse.IdentifierLiteral
		for _, ident := range n.PropertyNames {
			if prevIdent != nil {
				state.symbolicData.SetMostSpecificNodeValue(prevIdent, v)
			}
			v = symbolicMemb(v, ident.Name, false, n, state)
			prevIdent = ident
		}

		state.symbolicData.SetMostSpecificNodeValue(prevIdent, v)

		return v, nil
	case *parse.DynamicMemberExpression:
		left, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}
		iprops, ok := AsIprops(left).(IProps)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtCannotGetDynamicMemberOfValueWithNoProps(left)))
			return ANY, nil
		}

		return NewDynamicValue(symbolicMemb(iprops, n.PropertyName.Name, false, n, state)), nil
	case *parse.ExtractionExpression:
		left, err := symbolicEval(n.Object, state)
		if err != nil {
			return nil, err
		}

		result := &Object{
			entries: make(map[string]Serializable),
			static:  make(map[string]Pattern),
		}

		ignoreProps := false

		switch AsIprops(left).(type) {
		case *DynamicValue:
			state.addError(makeSymbolicEvalError(n.Object, state, EXTRACTION_DOES_NOT_SUPPORT_DYNAMIC_VALUES))
			ignoreProps = true
		case IProps:
		default:
			ignoreProps = true
			state.addError(makeSymbolicEvalError(n.Object, state, fmtValueHasNoProperties(left)))
		}

		for _, key := range n.Keys.Keys {
			name := key.(*parse.IdentifierLiteral).Name

			if ignoreProps {
				result.entries[name] = ANY_SERIALIZABLE
				result.static[name] = getStatic(ANY_SERIALIZABLE)
			} else {
				result.entries[name] = symbolicMemb(left, name, false, n, state).(Serializable)
				result.static[name] = getStatic(result.entries[name])
			}
		}
		return result, nil
	case *parse.DoubleColonExpression:
		left, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		obj, ok := left.(*Object)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, DOUBLE_COLON_EXPRS_ONLY_SUPPORT_OBJ_LHS_FOR_NOW))
			return ANY, nil
		}

		memb := symbolicMemb(obj, n.Element.Name, false, n, state)

		if IsAnyOrAnySerializable(memb) || utils.Ret0(IsSharable(memb)) {
			state.addError(makeSymbolicEvalError(node, state, RHS_OF_DOUBLE_COLON_EXPRS_WITH_OBJ_LHS_SHOULD_BE_THE_NAME_OF_A_MUTABLE_NON_SHARABLE_VALUE_PROPERTY))
		} else if len(options.doubleColonExprAncestorChain) == 0 {
			state.addError(makeSymbolicEvalError(node, state, MISPLACED_DOUBLE_COLON_EXPR))
		} else {
			ancestors := options.doubleColonExprAncestorChain
			rootAncestor := ancestors[0]
			misplaced := true
			switch rootAncestor.(type) {
			case *parse.Assignment:
				if len(ancestors) == 1 {
					break
				}
				misplaced = false
				for i, ancestor := range ancestors[1 : len(ancestors)-1] {
					if !isAllowedAfterMutationDoubleColonExprAncestor(ancestor, ancestors[i+1]) {
						misplaced = true
						break
					}
				}
			case *parse.CallExpression:
				if len(ancestors) == 1 {
					break
				}
				misplaced = false
				for i, ancestor := range ancestors[1 : len(ancestors)-1] {
					if !isAllowedAfterMutationDoubleColonExprAncestor(ancestor, ancestors[i+1]) {
						misplaced = true
						break
					}
				}
			default:
			}
			if misplaced {
				state.addError(makeSymbolicEvalError(node, state, MISPLACED_DOUBLE_COLON_EXPR))
			}
		}

		return memb, nil
	case *parse.IndexExpression:
		val, err := _symbolicEval(n.Indexed, state, evalOptions{
			doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), node),
		})
		if err != nil {
			return nil, err
		}

		index, err := symbolicEval(n.Index, state)
		if err != nil {
			return nil, err
		}

		intIndex, ok := index.(*Int)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtIndexIsNotAnIntButA(index)))
			index = &Int{}
		}

		if indexable, ok := asIndexable(val).(Indexable); ok {
			if intIndex != nil && intIndex.hasValue && indexable.HasKnownLen() && (intIndex.value < 0 || intIndex.value >= int64(indexable.KnownLen())) {
				state.addError(makeSymbolicEvalError(n.Index, state, INDEX_IS_OUT_OF_BOUNDS))
			}
			return indexable.element(), nil
		}

		state.addError(makeSymbolicEvalError(node, state, fmtXisNotIndexable(val)))
		return ANY, nil
	case *parse.SliceExpression:
		slice, err := _symbolicEval(n.Indexed, state, evalOptions{
			doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), node),
		})
		if err != nil {
			return nil, err
		}

		var startIndex *Int
		var endIndex *Int

		if n.StartIndex != nil {
			index, err := symbolicEval(n.StartIndex, state)
			if err != nil {
				return nil, err
			}
			if i, ok := index.(*Int); ok {
				startIndex = i
			} else {
				state.addError(makeSymbolicEvalError(node, state, fmtStartIndexIsNotAnIntButA(index)))
				startIndex = &Int{}
			}
		}

		if n.EndIndex != nil {
			index, err := symbolicEval(n.EndIndex, state)
			if err != nil {
				return nil, err
			}
			if i, ok := index.(*Int); ok {
				endIndex = i
			} else {
				state.addError(makeSymbolicEvalError(node, state, fmtEndIndexIsNotAnIntButA(index)))
				endIndex = &Int{}
			}
		}

		if startIndex != nil && startIndex.hasValue {
			if endIndex != nil && endIndex.hasValue && endIndex.value < startIndex.value {
				state.addError(makeSymbolicEvalError(n.EndIndex, state, END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX))
			}
		}

		if seq, ok := slice.(Sequence); ok {
			if startIndex != nil && startIndex.hasValue && seq.HasKnownLen() && (startIndex.value < 0 || startIndex.value >= int64(seq.KnownLen())) {
				state.addError(makeSymbolicEvalError(n.StartIndex, state, START_INDEX_IS_OUT_OF_BOUNDS))
			}
			return seq.slice(startIndex, endIndex), nil
		} else {
			state.addError(makeSymbolicEvalError(node, state, fmtSequenceExpectedButIs(slice)))
			return ANY, nil
		}

	case *parse.KeyListExpression:
		list := &KeyList{}

		for _, key := range n.Keys {
			list.append(string(key.(parse.IIdentifierLiteral).Identifier()))
		}

		return list, nil
	case *parse.BooleanConversionExpression:
		_, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return ANY_BOOL, nil
	case *parse.PatternIdentifierLiteral:
		patt := state.ctx.ResolveNamedPattern(n.Name)
		if patt == nil {
			state.addError(makeSymbolicEvalError(node, state, fmtPatternIsNotDeclared(n.Name)))
			return &AnyPattern{}, nil
		} else {
			return patt, nil
		}
	case *parse.PatternDefinition:
		pattern, err := symbolicallyEvalPatternNode(n.Right, state)
		if err != nil {
			return nil, err
		}
		//TODO: add checks
		state.symbolicData.SetMostSpecificNodeValue(n.Left, pattern)
		state.ctx.AddNamedPattern(n.Left.Name, pattern, state.inPreinit, state.getCurrentChunkNodePositionOrZero(n.Left))
		state.symbolicData.SetContextData(n, state.ctx.currentData())
		return nil, nil
	case *parse.PatternNamespaceDefinition:
		right, err := symbolicEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		namespace := &PatternNamespace{}
		pos := state.getCurrentChunkNodePositionOrZero(n.Left)

		switch r := right.(type) {
		case *Object:
			if len(r.entries) > 0 {
				namespace.entries = make(map[string]Pattern)
			}
			for k, v := range r.entries {
				if _, ok := v.(Pattern); !ok {
					exactValPatt, err := NewMostAdaptedExactPattern(v)
					if err == nil {
						v = exactValPatt
					} else {
						v = ANY_PATTERN
						state.addError(makeSymbolicEvalError(n.Right, state, err.Error()))
					}
				}
				namespace.entries[k] = v.(Pattern)
			}
			state.ctx.AddPatternNamespace(n.Left.Name, namespace, state.inPreinit, pos)
		case *Record:
			if len(r.entries) > 0 {
				namespace.entries = make(map[string]Pattern)
			}
			for k, v := range r.entries {
				if _, ok := v.(Pattern); !ok {
					exactValPatt, err := NewMostAdaptedExactPattern(v)
					if err == nil {
						v = exactValPatt
					} else {
						v = ANY_PATTERN
						state.addError(makeSymbolicEvalError(n.Right, state, err.Error()))
					}
				}
				namespace.entries[k] = v.(Pattern)
			}
			state.ctx.AddPatternNamespace(n.Left.Name, namespace, state.inPreinit, pos)
		default:
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceShouldBeInitWithNot(right)))
			state.ctx.AddPatternNamespace(n.Left.Name, namespace, state.inPreinit, pos)
		}
		state.symbolicData.SetMostSpecificNodeValue(n.Left, namespace)
		state.symbolicData.SetContextData(n, state.ctx.currentData())

		return nil, nil
	case *parse.PatternNamespaceIdentifierLiteral:
		namespace := state.ctx.ResolvePatternNamespace(n.Name)
		if namespace == nil {
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceIsNotDeclared(n.Name)))
			return ANY, nil
		}
		return namespace, nil
	case *parse.PatternNamespaceMemberExpression:
		v, err := symbolicEval(n.Namespace, state)
		if err != nil {
			return nil, err
		}

		namespace := v.(*PatternNamespace)

		defer func() {
			if result != nil && state.symbolicData != nil {
				state.symbolicData.SetMostSpecificNodeValue(n.MemberName, result)
			}
		}()

		if namespace == nil {
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceIsNotDeclared(n.Namespace.Name)))
			return ANY_PATTERN, nil
		} else {
			if namespace.entries == nil {
				return ANY_PATTERN, nil
			}
			patt := namespace.entries[n.MemberName.Name]

			if patt == nil {
				return ANY_PATTERN, nil
			}
			return patt, nil
		}
	case *parse.OptionalPatternExpression:
		v, err := symbolicEval(n.Pattern, state)
		if err != nil {
			return nil, err
		}

		patt := v.(Pattern)
		if patt.TestValue(Nil) {
			state.addError(makeSymbolicEvalError(node, state, CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL))
			return &AnyPattern{}, nil
		}

		return &OptionalPattern{pattern: patt}, nil
	case *parse.ComplexStringPatternPiece:
		return NewSequenceStringPattern(n), nil
	case *parse.PatternUnion:
		patt := &UnionPattern{}

		for _, case_ := range n.Cases {
			patternElement, err := symbolicallyEvalPatternNode(case_, state)
			if err != nil {
				return nil, fmt.Errorf("failed to symbolically compile a pattern element: %s", err.Error())
			}

			patt.Cases = append(patt.Cases, patternElement)
		}

		return patt, nil
	case *parse.ObjectPatternLiteral:
		pattern := &ObjectPattern{
			entries: make(map[string]Pattern),
			inexact: !n.Exact,
		}

		for _, el := range n.SpreadElements {
			compiledElement, err := symbolicallyEvalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			if objPattern, ok := compiledElement.(*ObjectPattern); ok {
				if objPattern.entries == nil {
					state.addError(makeSymbolicEvalError(el, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT))
				} else {
					for name, vpattern := range objPattern.entries {
						if _, alreadyPresent := pattern.entries[name]; alreadyPresent {
							state.addError(makeSymbolicEvalError(el, state, fmtPropertyShouldNotBePresentInSeveralSpreadPatterns(name)))
							continue
						}
						pattern.entries[name] = vpattern
					}
				}
				// else if objPattern.Inexact {
				// state.addError(makeSymbolicEvalError(el, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT))
				//

			} else {
				state.addError(makeSymbolicEvalError(el, state, fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(compiledElement)))
			}
		}

		for _, p := range n.Properties {
			name := p.Name()
			var err error
			pattern.entries[name], err = symbolicallyEvalPatternNode(p.Value, state)
			if err != nil {
				return nil, err
			}
			if state.symbolicData != nil {
				val, ok := state.symbolicData.GetMostSpecificNodeValue(p.Value)
				if ok {
					state.symbolicData.SetMostSpecificNodeValue(p.Key, val)
				}
			}
			if p.Optional {
				if pattern.optionalEntries == nil {
					pattern.optionalEntries = make(map[string]struct{}, 1)
				}
				pattern.optionalEntries[name] = struct{}{}
			}
		}

		return pattern, nil
	case *parse.RecordPatternLiteral:
		pattern := &RecordPattern{
			entries: make(map[string]Pattern),
			inexact: !n.Exact,
		}
		for _, el := range n.SpreadElements {
			compiledElement, err := symbolicallyEvalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			if recPattern, ok := compiledElement.(*RecordPattern); ok {
				if recPattern.entries == nil {
					state.addError(makeSymbolicEvalError(el, state, CANNOT_SPREAD_REC_PATTERN_THAT_MATCHES_ANY_RECORD))
				} else {
					for name, vpattern := range recPattern.entries {
						if _, alreadyPresent := pattern.entries[name]; alreadyPresent {
							state.addError(makeSymbolicEvalError(el, state, fmtPropertyShouldNotBePresentInSeveralSpreadPatterns(name)))
							continue
						}
						pattern.entries[name] = vpattern
					}
				}
			} else {
				state.addError(makeSymbolicEvalError(el, state, fmtPatternSpreadInRecordPatternShouldBeAnRecordPatternNot(compiledElement)))
			}
		}

		for _, p := range n.Properties {
			name := p.Name()
			var err error

			prevErrCount := len(state.errors())

			entryPattern, err := symbolicallyEvalPatternNode(p.Value, state)
			if err != nil {
				return nil, err
			}

			if _, ok := entryPattern.(*AnyPattern); ok && len(state.errors()) > prevErrCount {
				//AnyPattern may be present due to an issue (invalid pattern call) so
				//we handle this case separately
				pattern.entries[name] = &TypePattern{val: ANY_SERIALIZABLE}
			} else if entryPattern.SymbolicValue().IsMutable() {
				state.addError(makeSymbolicEvalError(p.Value, state, fmtEntriesOfRecordPatternShouldMatchOnlyImmutableValues(name)))
				pattern.entries[name] = &TypePattern{val: ANY_SERIALIZABLE}
			} else {
				pattern.entries[name] = entryPattern
			}

			if state.symbolicData != nil {
				val, ok := state.symbolicData.GetMostSpecificNodeValue(p.Value)
				if ok {
					state.symbolicData.SetMostSpecificNodeValue(p.Key, val)
				}
			}
			if p.Optional {
				if pattern.optionalEntries == nil {
					pattern.optionalEntries = make(map[string]struct{}, 1)
				}
				pattern.optionalEntries[name] = struct{}{}
			}
		}
		return pattern, nil

	case *parse.ListPatternLiteral:
		pattern := &ListPattern{}

		if n.GeneralElement != nil {
			var err error
			pattern.generalElement, err = symbolicallyEvalPatternNode(n.GeneralElement, state)
			if err != nil {
				return nil, err
			}

		} else {
			pattern.elements = make([]Pattern, 0)

			for _, e := range n.Elements {
				symbolicVal, err := symbolicallyEvalPatternNode(e, state)
				if err != nil {
					return nil, err
				}
				pattern.elements = append(pattern.elements, symbolicVal)
			}
		}

		return pattern, nil
	case *parse.TuplePatternLiteral:
		pattern := &TuplePattern{}

		if n.GeneralElement != nil {
			var err error
			pattern.generalElement, err = symbolicallyEvalPatternNode(n.GeneralElement, state)
			if err != nil {
				return nil, err
			}

			if pattern.generalElement.SymbolicValue().IsMutable() {
				state.addError(makeSymbolicEvalError(n.GeneralElement, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES))
				pattern.generalElement = &TypePattern{val: ANY_SERIALIZABLE}
			}

		} else {
			pattern.elements = make([]Pattern, 0)

			for _, e := range n.Elements {
				elemPattern, err := symbolicallyEvalPatternNode(e, state)
				if err != nil {
					return nil, err
				}

				if elemPattern.SymbolicValue().IsMutable() {
					state.addError(makeSymbolicEvalError(e, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES))
					elemPattern = &TypePattern{val: ANY_SERIALIZABLE}
				}

				pattern.elements = append(pattern.elements, elemPattern)
			}
		}

		return pattern, nil
	case *parse.OptionPatternLiteral:
		pattern, err := symbolicallyEvalPatternNode(n.Value, state)
		if err != nil {
			return nil, err
		}

		return NewOptionPattern(n.Name, pattern), nil
	case *parse.ByteSliceLiteral:
		return &ByteSlice{}, nil
	case *parse.ConcatenationExpression:
		if len(n.Elements) == 0 {
			return nil, errors.New("cannot create concatenation with no elements")
		}
		var values []SymbolicValue
		var nodeIndexes []int
		atLeastOneSpread := false

		for elemNodeIndex, elemNode := range n.Elements {
			spreadElem, ok := elemNode.(*parse.ElementSpreadElement)
			if !ok {
				elemVal, err := symbolicEval(elemNode, state)
				if err != nil {
					return nil, err
				}

				if strLike, isStrLike := as(elemVal, STRLIKE_INTERFACE_TYPE).(StringLike); isStrLike {
					elemVal = strLike
				}

				if bytesLike, isBytesLike := as(elemVal, BYTESLIKE_INTERFACE_TYPE).(BytesLike); isBytesLike {
					elemVal = bytesLike
				}

				values = append(values, elemVal)
				nodeIndexes = append(nodeIndexes, elemNodeIndex)
				continue
			}

			//handle spread element
			atLeastOneSpread = true

			spreadVal, err := symbolicEval(spreadElem.Expr, state)
			if err != nil {
				return nil, err
			}

			if iterable, ok := spreadVal.(Iterable); ok {
				iterableElemVal := iterable.IteratorElementValue()

				if strLike, isStrLike := as(iterableElemVal, STRLIKE_INTERFACE_TYPE).(StringLike); isStrLike {
					iterableElemVal = strLike
				}

				if bytesLike, isBytesLike := as(iterableElemVal, BYTESLIKE_INTERFACE_TYPE).(BytesLike); isBytesLike {
					iterableElemVal = bytesLike
				}

				switch iterableElemVal.(type) {
				case StringLike, BytesLike, *Tuple:
					values = append(values, iterableElemVal)
					nodeIndexes = append(nodeIndexes, elemNodeIndex)
				default:
					state.addError(makeSymbolicEvalError(elemNode, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION))
				}
			} else {
				state.addError(makeSymbolicEvalError(n, state, SPREAD_ELEMENT_SHOULD_BE_ITERABLE))
			}
		}

		if len(values) == 0 {
			return ANY, nil
		}

		switch values[0].(type) {
		case StringLike:
			if len(values) == 1 && !atLeastOneSpread {
				return values[0], nil
			}
			for i, elem := range values {
				if _, ok := elem.(StringLike); !ok {
					state.addError(makeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmtStringConcatInvalidElementOfType(elem)))
				}
			}
			return ANY_STR_CONCAT, nil
		case BytesLike:
			if len(values) == 1 && !atLeastOneSpread {
				return values[0], nil
			}
			for i, elem := range values {
				if _, ok := elem.(BytesLike); !ok {
					state.addError(makeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmt.Sprintf("bytes concatenation: invalid element of type %T", elem)))
				}
			}
			return ANY_BYTES_CONCAT, nil
		case *Tuple:
			if len(values) == 1 && !atLeastOneSpread {
				return values[0], nil
			}

			var generalElements []SymbolicValue
			var elements []Serializable

			for i, concatElem := range values {
				if tuple, ok := concatElem.(*Tuple); ok {
					if tuple.HasKnownLen() {
						elements = append(elements, tuple.elements...)
					} else {
						generalElements = append(generalElements, tuple.generalElement)
					}
				} else {
					state.addError(makeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmt.Sprintf("tuple concatenation: invalid element of type %T", concatElem)))
				}
			}

			if elements == nil {
				return NewTupleOf(AsSerializable(joinValues(generalElements)).(Serializable)), nil
			} else {
				return NewTuple(elements...), nil
			}
		default:
			state.addError(makeSymbolicEvalError(n, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION))
			return ANY, nil
		}
	case *parse.AssertionStatement:
		ok, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}
		if _, isBool := ok.(*Bool); !isBool {
			state.addError(makeSymbolicEvalError(node, state, fmtAssertedValueShouldBeBoolNot(ok)))
		}

		if binExpr, ok := n.Expr.(*parse.BinaryExpression); ok && state.symbolicData != nil {
			isVar := parse.IsAnyVariableIdentifier(binExpr.Left)
			if !isVar {
				return nil, nil
			}

			switch binExpr.Operator {
			case parse.Match:
				right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)

				if pattern, ok := right.(Pattern); ok {
					narrowPath(binExpr.Left, setExactValue, pattern.SymbolicValue(), state, 0)
				}
			}
		}
		state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
		state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())

		return nil, nil
	case *parse.RuntimeTypeCheckExpression:
		options.ignoreNodeValue = true
		val, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return val, nil
	case *parse.TestSuiteExpression:
		if n.Meta != nil {
			_, err := symbolicEval(n.Meta, state)
			if err != nil {
				return nil, err
			}
		}

		v, err := symbolicEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		embeddedModule := v.(*AstNode).Node.(*parse.Chunk)

		//TODO: read the manifest to known the permissions
		modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx)
		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		modState.Module = state.Module
		modState.symbolicData = state.symbolicData

		_, err = symbolicEval(embeddedModule, modState)
		if err != nil {
			return nil, err
		}

		for _, err := range modState.errors() {
			state.addError(err)
		}

		return &TestSuite{}, nil
	case *parse.TestCaseExpression:
		if n.Meta != nil {
			_, err := symbolicEval(n.Meta, state)
			if err != nil {
				return nil, err
			}
		}

		v, err := symbolicEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		embeddedModule := v.(*AstNode).Node.(*parse.Chunk)

		//TODO: read the manifest to known the permissions
		modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx)
		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		modState.Module = state.Module
		modState.symbolicData = state.symbolicData

		_, err = symbolicEval(embeddedModule, modState)
		if err != nil {
			return nil, err
		}

		for _, err := range modState.errors() {
			state.addError(err)
		}

		return &TestCase{}, nil
	case *parse.LifetimejobExpression:
		meta, err := symbolicEval(n.Meta, state)
		if err != nil {
			return nil, err
		}

		if meta.IsMutable() {
			state.addError(makeSymbolicEvalError(n.Meta, state, META_VAL_OF_LIFETIMEJOB_SHOULD_BE_IMMUTABLE))
		}

		var subject SymbolicValue = ANY
		var subjectPattern Pattern = ANY_PATTERN

		if n.Subject != nil {
			v, err := symbolicEval(n.Subject, state)
			if err != nil {
				return nil, err
			}
			patt, ok := v.(Pattern)

			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtSubjectOfLifetimeJobShouldBeObjectPatternNot(v)))
			} else {
				subject = patt.SymbolicValue()
				subjectPattern = patt
			}
		}

		v, err := symbolicEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		embeddedModule := v.(*AstNode).Node.(*parse.Chunk)

		//add patterns of parent state
		modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx) //TODO: read the manifest to known the permissions
		state.ctx.ForEachPattern(func(name string, pattern Pattern) {
			modCtx.AddNamedPattern(name, pattern, state.inPreinit, parse.SourcePositionRange{})
		})
		state.ctx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace) {
			modCtx.AddPatternNamespace(name, namespace, state.inPreinit, parse.SourcePositionRange{})
		})

		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		state.forEachGlobal(func(name string, info varSymbolicInfo) {
			modState.setGlobal(name, info.value, info.constness())
		})

		modState.Module = state.Module
		modState.symbolicData = state.symbolicData

		nextSelf, ok := state.getNextSelf()

		if n.Subject == nil { // implicit subject
			if !ok {
				return nil, errors.New("next self should be set")
			}
			modState.topLevelSelf = nextSelf
		} else {
			if ok && !subject.Test(nextSelf) {
				state.addError(makeSymbolicEvalError(node, state, fmtSelfShouldMatchLifetimeJobSubjectPattern(subjectPattern)))
			}
			modState.topLevelSelf = subject
		}

		_, err = symbolicEval(embeddedModule, modState)
		if err != nil {
			return nil, err
		}

		for _, err := range modState.errors() {
			state.addError(err)
		}

		return NewLifetimeJob(subjectPattern), nil
	case *parse.ReceptionHandlerExpression:
		_, err := symbolicEval(n.Handler, state)
		if err != nil {
			return nil, err
		}
		return ANY_SYNC_MSG_HANDLER, nil
	case *parse.SendValueExpression:
		_, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		_, err = symbolicEval(n.Receiver, state)
		if err != nil {
			return nil, err
		}

		return Nil, nil
	case *parse.StringTemplateLiteral:
		_, isPatternAnIdent := n.Pattern.(*parse.PatternIdentifierLiteral)

		if isPatternAnIdent && n.HasInterpolations() {
			state.addError(makeSymbolicEvalError(node, state, STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX))
			return &CheckedString{}, nil
		}

		var namespaceName string
		var namespace *PatternNamespace

		if n.Pattern != nil {
			if !isPatternAnIdent {
				namespaceMembExpr := n.Pattern.(*parse.PatternNamespaceMemberExpression)
				namespaceName = namespaceMembExpr.Namespace.Name
				namespace = state.ctx.ResolvePatternNamespace(namespaceName)

				if namespace == nil {
					state.addError(makeSymbolicEvalError(node, state, fmtCannotInterpolatePatternNamespaceDoesNotExist(namespaceName)))
					return &CheckedString{}, nil
				}

				memberName := namespaceMembExpr.MemberName.Name
				_, ok := namespace.entries[memberName]
				if !ok {
					state.addError(makeSymbolicEvalError(node, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(memberName, namespaceName)))
					return &CheckedString{}, nil
				}
			}

			_, err := symbolicEval(n.Pattern, state)
			if err != nil {
				return nil, err
			}
		}

		for _, slice := range n.Slices {

			switch s := slice.(type) {
			case *parse.StringTemplateSlice:
			case *parse.StringTemplateInterpolation:
				if s.Type != "" {
					memberName := s.Type
					_, ok := namespace.entries[memberName]
					if !ok {
						state.addError(makeSymbolicEvalError(slice, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(memberName, namespaceName)))
						return &CheckedString{}, nil
					}
				}

				e, err := symbolicEval(s.Expr, state)
				if err != nil {
					return nil, err
				}

				switch as(e, STRLIKE_INTERFACE_TYPE).(type) {
				case StringLike:
				case *Int:
				default:
					if n.Pattern == nil {
						state.addError(makeSymbolicEvalError(slice, state, fmtUntypedInterpolationIsNotStringlikeOrIntBut(e)))
					} else {
						state.addError(makeSymbolicEvalError(slice, state, fmtInterpolationIsNotStringlikeOrIntBut(e)))
					}
				}
			}
		}

		if n.Pattern == nil {
			return ANY_STR, nil
		}

		return &CheckedString{}, nil
	case *parse.CssSelectorExpression:
		return ANY_STR, nil
	case *parse.XMLExpression:
		namespace, err := symbolicEval(n.Namespace, state)
		if err != nil {
			return nil, err
		}

		elem, err := symbolicEval(n.Element, state)
		if err != nil {
			return nil, err
		}

		ns, ok := namespace.(*Namespace)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Namespace, state, NAMESPACE_APPLIED_TO_XML_ELEMENT_SHOUD_BE_A_RECORD))
			return ANY, nil
		} else {
			if _, ok := ns.entries[FROM_XML_FACTORY_NAME]; !ok {
				state.addError(makeSymbolicEvalError(n.Namespace, state, MISSING_FACTORY_IN_NAMESPACE_APPLIED_TO_XML_ELEMENT))
				return ANY, nil
			}
			factory := ns.Prop(FROM_XML_FACTORY_NAME)
			goFn, ok := factory.(*GoFunction)
			if !ok {
				state.addError(makeSymbolicEvalError(n.Namespace, state, FROM_XML_FACTORY_IS_NOT_A_GO_FUNCTION))
				return ANY, nil
			}

			if goFn.IsShared() {
				state.addError(makeSymbolicEvalError(n.Namespace, state, FROM_XML_FACTORY_SHOULD_NOT_BE_A_SHARED_FUNCTION))
				return ANY, nil
			}

			utils.PanicIfErr(goFn.LoadSignatureData())

			if len(goFn.NonVariadicParametersExceptCtx()) == 0 {
				state.addError(makeSymbolicEvalError(n.Namespace, state, FROM_XML_FACTORY_SHOULD_HAVE_AT_LEAST_ONE_NON_VARIADIC_PARAM))
				return ANY, nil
			}

			result, _, _, err := goFn.Call(goFunctionCallInput{
				symbolicArgs:      []SymbolicValue{elem},
				nonSpreadArgCount: 1,
				hasSpreadArg:      false,
				state:             state,
				isExt:             false,
				must:              false,
				callLikeNode:      n,
			})

			return result, err
		}
	case *parse.XMLElement:
		var children []SymbolicValue
		name := n.Opening.Name.(*parse.IdentifierLiteral).Name
		var attrs map[string]SymbolicValue
		if len(n.Opening.Attributes) > 0 {
			attrs = make(map[string]SymbolicValue, len(n.Opening.Attributes))

			for _, attr := range n.Opening.Attributes {
				name := attr.Name.(*parse.IdentifierLiteral).Name
				if attr.Value == nil {
					attrs[name] = ANY_STR
					continue
				}
				val, err := symbolicEval(attr.Value, state)
				if err != nil {
					return nil, err
				}
				attrs[name] = val
			}
		}

		for _, child := range n.Children {
			val, err := symbolicEval(child, state)
			if err != nil {
				return nil, err
			}
			children = append(children, val)
		}

		xmlElem := NewXmlElement(name, attrs, children)

		state.symbolicData.SetMostSpecificNodeValue(n.Opening.Name, xmlElem)
		if n.Closing != nil {
			state.symbolicData.SetMostSpecificNodeValue(n.Closing.Name, xmlElem)
		}

		return xmlElem, nil
	case *parse.XMLInterpolation:
		val, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}
		return val, err
	case *parse.XMLText:
		return ANY_STR, nil
	case *parse.UnknownNode:
		return ANY, nil
	default:
		return nil, fmt.Errorf("cannot evaluate %#v (%T)\n%s", node, node, debug.Stack())
	}
}

func symbolicMemb(value SymbolicValue, name string, optionalMembExpr bool, node parse.Node, state *State) (result SymbolicValue) {
	//note: the property of a %serializable is not necessarily serializable (example: Go methods)

	iprops, ok := AsIprops(value).(IProps)
	if !ok {
		state.addError(makeSymbolicEvalError(node, state, fmtValueHasNoProperties(value)))
		return ANY
	}

	defer func() {
		e := recover()
		if e != nil {
			//TODO: add log

			//if err, ok := e.(error); ok && strings.Contains(err.Error(), "nil pointer") {
			//}

			closest, distance, found := utils.FindClosestString(nil, iprops.PropertyNames(), name, MAX_STRING_SUGGESTION_DIFF)
			if !found || (len(closest) >= MAX_STRING_SUGGESTION_DIFF && distance >= MAX_STRING_SUGGESTION_DIFF-1) {
				closest = ""
			}

			if !optionalMembExpr {
				state.addError(makeSymbolicEvalError(node, state, fmtPropOfSymbolicDoesNotExist(name, value, closest)))
			}
			result = ANY
		} else {
			if optIprops, ok := iprops.(OptionalIProps); ok {
				if !optionalMembExpr && utils.SliceContains(optIprops.OptionalPropertyNames(), name) {
					state.addError(makeSymbolicEvalError(node, state, fmtPropertyIsOptionalUseOptionalMembExpr(name)))
				}
			}
		}
	}()

	prop := iprops.Prop(name)
	if prop == nil {
		state.addError(makeSymbolicEvalError(node, state, "symbolic IProp should panic when a non-existing property is accessed"))
		return ANY
	}
	return prop
}

type pathNarrowing int

const (
	setExactValue pathNarrowing = iota
	removePossibleValue
)

func narrowPath(path parse.Node, action pathNarrowing, value SymbolicValue, state *State, ignored int) {
	//TODO: use reEval option in in symbolicEval calls ?

switch_:
	switch node := path.(type) {
	case *parse.Variable:
		switch action {
		case setExactValue:
			state.narrowLocal(node.Name, value, path)
		case removePossibleValue:
			prev, ok := state.getLocal(node.Name)
			if ok {
				state.narrowLocal(node.Name, narrowOut(value, prev.value), path)
			}
		}
	case *parse.GlobalVariable:
		switch action {
		case setExactValue:
			state.narrowGlobal(node.Name, value, path)
		case removePossibleValue:
			prev, ok := state.getGlobal(node.Name)
			if ok {
				state.narrowGlobal(node.Name, narrowOut(value, prev.value), path)
			}
		}
	case *parse.IdentifierLiteral:
		switch action {
		case setExactValue:
			if state.hasLocal(node.Name) {
				state.narrowLocal(node.Name, value, path)
			} else if state.hasGlobal(node.Name) {
				state.narrowGlobal(node.Name, value, path)
			}
		case removePossibleValue:
			if state.hasLocal(node.Name) {
				prev, _ := state.getLocal(node.Name)
				state.narrowLocal(node.Name, narrowOut(value, prev.value), path)
			} else if state.hasGlobal(node.Name) {
				prev, _ := state.getGlobal(node.Name)
				state.narrowGlobal(node.Name, narrowOut(value, prev.value), path)
			}
		}
	case *parse.IdentifierMemberExpression:
		if ignored > 1 {
			panic(errors.New("not supported yet"))
		}

		switch action {
		case setExactValue:
			if ignored == 1 && len(node.PropertyNames) == 1 {
				narrowPath(node.Left, setExactValue, value, state, 0)
				return
			}

			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}
			propName := node.PropertyNames[0].Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			movingIprops := iprops
			ipropsList := []IProps{iprops}

			if len(node.PropertyNames) > 1 {
				for _, _propName := range node.PropertyNames[:len(node.PropertyNames)-ignored-1] {
					if !HasRequiredOrOptionalProperty(movingIprops, _propName.Name) {
						break switch_
					}

					val := movingIprops.Prop(_propName.Name)

					movingIprops, ok = AsIprops(val).(IProps)
					if !ok {
						break switch_
					}
					ipropsList = append(ipropsList, movingIprops)
				}
				var newValue SymbolicValue = value

				//update iprops from right to left
				for i := len(ipropsList) - 1; i >= 0; i-- {
					currentIprops := ipropsList[i]
					currentPropertyName := node.PropertyNames[i].Name
					newValue, err = currentIprops.WithExistingPropReplaced(currentPropertyName, newValue)

					if err == ErrUnassignablePropsMixin {
						break switch_
					}
					if err != nil {
						panic(err)
					}
				}

				narrowPath(node.Left, setExactValue, newValue, state, 0)
			} else {
				newPropValue, err := iprops.WithExistingPropReplaced(propName, value)
				if err == nil {
					narrowPath(node.Left, setExactValue, newPropValue, state, 0)
				} else if err != ErrUnassignablePropsMixin {
					panic(err)
				}
			}

		case removePossibleValue:
			if len(node.PropertyNames) > 1 {
				panic(errors.New("not supported yet"))
			}
			if ignored == 1 {
				narrowPath(node.Left, removePossibleValue, value, state, 0)
			} else {
				left, err := symbolicEval(node.Left, state)
				if err != nil {
					panic(err)
				}

				propName := node.PropertyNames[0].Name

				iprops, ok := AsIprops(left).(IProps)
				if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
					break
				}

				prevPropValue := iprops.Prop(propName)
				newPropValue := narrowOut(value, prevPropValue)

				newRecPrevPropValue, err := iprops.WithExistingPropReplaced(node.PropertyNames[0].Name, newPropValue)
				if err == nil {
					narrowPath(node.Left, setExactValue, newRecPrevPropValue, state, 0)
				} else if err != ErrUnassignablePropsMixin {
					panic(err)
				}
			}
		}
	case *parse.MemberExpression:
		switch action {
		case setExactValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.PropertyName.Name
			iprops, ok := AsIprops(left).(IProps)
			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			newPropValue, err := iprops.WithExistingPropReplaced(node.PropertyName.Name, value)
			if err == nil {
				narrowPath(node.Left, setExactValue, newPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		case removePossibleValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.PropertyName.Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			prevPropValue := iprops.Prop(node.PropertyName.Name)
			newPropValue := narrowOut(value, prevPropValue)

			newRecPrevPropValue, err := iprops.WithExistingPropReplaced(node.PropertyName.Name, newPropValue)
			if err == nil {
				narrowPath(node.Left, setExactValue, newRecPrevPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		}
	case *parse.DoubleColonExpression:
		//almost same logic as parse.MemberExpression

		switch action {
		case setExactValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.Element.Name
			iprops, ok := AsIprops(left).(IProps)
			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			newPropValue, err := iprops.WithExistingPropReplaced(node.Element.Name, value)
			if err == nil {
				narrowPath(node.Left, setExactValue, newPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		case removePossibleValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.Element.Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			prevPropValue := iprops.Prop(node.Element.Name)
			newPropValue := narrowOut(value, prevPropValue)

			newRecPrevPropValue, err := iprops.WithExistingPropReplaced(node.Element.Name, newPropValue)
			if err == nil {
				narrowPath(node.Left, setExactValue, newRecPrevPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		}
	}
}

func handleConstraints(obj *Object, block *parse.InitializationBlock, state *State) error {
	//we first there are only authorized statements & expressions in the initialization block

	err := parse.Walk(block, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		if node == block {
			return parse.Continue, nil
		}

		switch node.(type) {
		case *parse.BinaryExpression:
		case *parse.SelfExpression:
		case *parse.MemberExpression:
		case parse.SimpleValueLiteral:
		default:
			state.addError(makeSymbolicEvalError(node, state, CONSTRAINTS_INIT_BLOCK_EXPLANATION))
		}
		return parse.Continue, nil
	}, nil)

	if err != nil {
		return fmt.Errorf("constraints: error when walking the initialization block: %w", err)
	}

	//

	for _, stmt := range block.Statements {
		switch stmt.(type) {
		case *parse.BinaryExpression:

			constraint := &ComplexPropertyConstraint{
				Expr: stmt,
			}

			parse.Walk(stmt, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if parse.NodeIs(node, &parse.SelfExpression{}) && parse.NodeIs(parent, &parse.MemberExpression{}) {
					constraint.Properties = append(constraint.Properties, parent.(*parse.MemberExpression).PropertyName.Name)
				}
				return parse.Continue, nil
			}, nil)

			obj.complexPropertyConstraints = append(obj.complexPropertyConstraints, constraint)
		default:
			state.addError(makeSymbolicEvalError(stmt, state, CONSTRAINTS_INIT_BLOCK_EXPLANATION))
		}
	}

	return nil
}

func makeSymbolicEvalError(node parse.Node, state *State, msg string) SymbolicEvaluationError {
	locatedMsg := msg
	location := state.getErrorMesssageLocation(node)
	if state.Module != nil {
		locatedMsg = fmt.Sprintf("check(symbolic): %s: %s", location, msg)
	}
	return SymbolicEvaluationError{msg, locatedMsg, location}
}

func makeSymbolicEvalWarning(node parse.Node, state *State, msg string) SymbolicEvaluationWarning {
	locatedMsg := msg
	location := state.getErrorMesssageLocation(node)
	if state.Module != nil {
		locatedMsg = fmt.Sprintf("check(symbolic): warning: %s: %s", location, msg)
	}
	return SymbolicEvaluationWarning{msg, locatedMsg, location}
}

func converTypeToSymbolicValue(t reflect.Type) (SymbolicValue, error) {
	err := fmt.Errorf("cannot convert type to symbolic value : %v", t)

	if t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct {
		v := reflect.New(t.Elem())
		symbolicVal, ok := v.Interface().(SymbolicValue)
		if !ok {
			return nil, err
		}
		return symbolicVal.WidestOfType(), nil
	}

	switch t {
	case SYMBOLIC_VALUE_INTERFACE_TYPE:
		return ANY, nil
	case SERIALIZABLE_INTERFACE_TYPE:
		return ANY_SERIALIZABLE, nil
	case SERIALIZABLE_ITERABLE_INTERFACE_TYPE:
		return ANY_SERIALIZABLE_ITERABLE, nil
	case ITERABLE_INTERFACE_TYPE:
		return ANY_ITERABLE, nil
	case INDEXABLE_INTERFACE_TYPE:
		return ANY_INDEXABLE, nil
	case SEQUENCE_INTERFACE_TYPE:
		return ANY_SEQ_OF_ANY, nil
	case RESOURCE_NAME_INTERFACE_TYPE:
		return ANY_RES_NAME, nil
	case READABLE_INTERFACE_TYPE:
		return ANY_READABLE, nil
	case PATTERN_INTERFACE_TYPE:
		return ANY_PATTERN, nil
	case PROTOCOL_CLIENT_INTERFACE_TYPE:
		return &AnyProtocolClient{}, nil
	case VALUE_RECEIVER_INTERFACE_TYPE:
		return ANY_MSG_RECEIVER, nil
	case STREAMABLE_INTERFACE_TYPE:
		return ANY_STREAM_SOURCE, nil
	case WATCHABLE_INTERFACE_TYPE:
		return ANY_WATCHABLE, nil
	case WRITABLE_INTERFACE_TYPE:
		return ANY_WRITABLE, nil
	case STR_PATTERN_ELEMENT_INTERFACE_TYPE:
		return ANY_STR_PATTERN, nil
	case INTEGRAL_INTERFACE_TYPE:
		return ANY_INTEGRAL, nil
	case FORMAT_INTERFACE_TYPE:
		return ANY_FORMAT, nil
	case IN_MEM_SNAPSHOTABLE:
		return ANY_IN_MEM_SNAPSHOTABLE, nil
	case STRLIKE_INTERFACE_TYPE:
		return ANY_STR_LIKE, nil
	}

	return nil, err
}

func isAllowedAfterMutationDoubleColonExprAncestor(ancestor, deeper parse.Node) bool {
	switch a := ancestor.(type) {
	case *parse.MemberExpression:
		if deeper == a.Left {
			return true
		}
	case *parse.IdentifierMemberExpression:
		if deeper == a.Left {
			return true
		}
	case *parse.IndexExpression:
		if deeper == a.Indexed {
			return true
		}
	case *parse.SliceExpression:
		if deeper == a.Indexed {
			return true
		}
	}
	return false
}
