package internal

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"

	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
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
	CTX_PTR_TYPE                    = reflect.TypeOf((*Context)(nil))
	ERROR_TYPE                      = reflect.TypeOf((*Error)(nil))
	SYMBOLIC_VALUE_INTERFACE_TYPE   = reflect.TypeOf((*SymbolicValue)(nil)).Elem()
	ITERABLE_INTERFACE_TYPE         = reflect.TypeOf((*Iterable)(nil)).Elem()
	INDEXABLE_INTERFACE_TYPE        = reflect.TypeOf((*Indexable)(nil)).Elem()
	SEQUENCE_INTERFACE_TYPE         = reflect.TypeOf((*Sequence)(nil)).Elem()
	MUTABLE_SEQUENCE_INTERFACE_TYPE = reflect.TypeOf((*MutableSequence)(nil)).Elem()
	INTEGRAL_INTERFACE_TYPE         = reflect.TypeOf((*Integral)(nil)).Elem()
	WRITABLE_INTERFACE_TYPE         = reflect.TypeOf((*Writable)(nil)).Elem()
	STRLIKE_INTERFACE_TYPE          = reflect.TypeOf((*StringLike)(nil)).Elem()

	IPROPS_INTERFACE_TYPE              = reflect.TypeOf((*IProps)(nil)).Elem()
	PROTOCOL_CLIENT_INTERFACE_TYPE     = reflect.TypeOf((*ProtocolClient)(nil)).Elem()
	READABLE_INTERFACE_TYPE            = reflect.TypeOf((*Readable)(nil)).Elem()
	PATTERN_INTERFACE_TYPE             = reflect.TypeOf((*Pattern)(nil)).Elem()
	RESOURCE_NAME_INTERFACE_TYPE       = reflect.TypeOf((*ResourceName)(nil)).Elem()
	VALUE_RECEIVER_INTERFACE_TYPE      = reflect.TypeOf((*MessageReceiver)(nil)).Elem()
	STREAMABLE_INTERFACE_TYPE          = reflect.TypeOf((*StreamSource)(nil)).Elem()
	WATCHABLE_INTERFACE_TYPE           = reflect.TypeOf((*Watchable)(nil)).Elem()
	STR_PATTERN_ELEMENT_INTERFACE_TYPE = reflect.TypeOf((*StringPatternElement)(nil)).Elem()
	FORMAT_INTERFACE_TYPE              = reflect.TypeOf((*Format)(nil)).Elem()
	IN_MEM_SNAPSHOTABLE                = reflect.TypeOf((*InMemorySnapshotable)(nil)).Elem()

	ANY_READABLE = &AnyReadable{}
)

type SymbolicEvalCheckInput struct {
	Node         parse.Node
	Module       *Module
	GlobalConsts map[string]interface{}

	IsShellChunk   bool
	ShellLocalVars map[string]interface{}
	Context        *Context
	//InitialSymbolicData *SymbolicData
}

// SymbolicEvalCheck performs various checks on an AST, most checks are type checks.
// If the returned data is not nil the error is nil or is the combination of checking errors, the list of checking errors
// is stored in the symbolic data.
// If the returned data is nil the error is an unexpected one (it is not about bad code).
// StaticCheck() should be runned before this function.
func SymbolicEvalCheck(input SymbolicEvalCheckInput) (*SymbolicData, error) {

	state := newSymbolicState(input.Context, input.Module.MainChunk)
	state.Module = input.Module

	for k, v := range input.GlobalConsts {
		symbolicVal, err := extData.ToSymbolicValue(v, false)
		if err != nil {
			return nil, fmt.Errorf("cannot convert global %s: %s", k, err)
		}
		state.setGlobal(k, symbolicVal, GlobalConst)
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

	//

	data := NewSymbolicData()
	state.symbolicData = data

	_, err := symbolicEval(input.Node, state)

	finalErrBuff := bytes.NewBuffer(nil)
	if err != nil { //unexpected error
		finalErrBuff.WriteString(err.Error())
		finalErrBuff.WriteRune('\n')
		return nil, err
	}

	if len(state.errors) == 0 { //no error in checked code
		return data, nil
	}

	data.errors = state.errors

	for _, err := range state.errors {
		finalErrBuff.WriteString(err.Error())
		finalErrBuff.WriteRune('\n')
	}

	return data, errors.New(finalErrBuff.String())
}

func symbolicEval(node parse.Node, state *State) (result SymbolicValue, finalErr error) {
	defer func() {
		if finalErr == nil && result != nil && state.symbolicData != nil {
			state.symbolicData.SetNodeValue(node, result)
		}
	}()

	switch n := node.(type) {
	case *parse.BooleanLiteral:
		return &Bool{}, nil
	case *parse.IntLiteral:
		return &Int{}, nil
	case *parse.FloatLiteral:
		return &Float{}, nil
	case *parse.PortLiteral:
		return &Port{}, nil
	case *parse.QuantityLiteral:
		values := make([]float64, len(n.Units))

		v, err := extData.GetQuantity(values, n.Units)
		if err != nil {
			return nil, err
		}
		return extData.ToSymbolicValue(v, false)
	case *parse.DateLiteral:
		return &Date{}, nil
	case *parse.RateLiteral:
		return ANY, nil
	case *parse.QuotedStringLiteral, *parse.UnquotedStringLiteral, *parse.MultilineStringLiteral:
		return &String{}, nil
	case *parse.RuneLiteral:
		return &Rune{}, nil
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
	case *parse.AbsolutePathLiteral, *parse.RelativePathLiteral:
		return &Path{}, nil
	case *parse.AbsolutePathPatternLiteral, *parse.RelativePathPatternLiteral:
		return &PathPattern{}, nil
	case *parse.NamedSegmentPathPatternLiteral:
		return &NamedSegmentPathPattern{node: n}, nil
	case *parse.RegularExpressionLiteral:
		return &RegexPattern{}, nil
	case *parse.PathSlice, *parse.PathPatternSlice:
		return &String{}, nil
	case *parse.URLQueryParameterValueSlice:
		return &String{}, nil
	case *parse.FlagLiteral:
		return NewOption(n.Name), nil
	case *parse.OptionExpression:
		return NewOption(n.Name), nil
	case *parse.AbsolutePathExpression, *parse.RelativePathExpression:
		return &Path{}, nil
	case *parse.PathPatternExpression:
		return &PathPattern{}, nil
	case *parse.URLLiteral:
		return &URL{}, nil
	case *parse.SchemeLiteral:
		return &Host{}, nil
	case *parse.HostLiteral:
		return &Host{}, nil
	case *parse.AtHostLiteral:
		return ANY, nil
	case *parse.EmailAddressLiteral:
		return &EmailAddress{}, nil
	case *parse.HostPatternLiteral:
		return &HostPattern{}, nil
	case *parse.URLPatternLiteral:
		return &URLPattern{}, nil
	case *parse.URLExpression:
		return &URL{}, nil
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

		value, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}
		v := value

		if state.returnType == nil || (*state.returnType).Test(v) {
			state.returnValue = &v
		} else {
			state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(v, *state.returnType)))
			state.returnValue = &v
		}

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

		for i, argNode := range n.Arguments {
			arg, err := symbolicEval(argNode, state)
			if err != nil {
				return nil, err
			}
			args[i] = arg
		}

		patt, err := callee.(Pattern).Call(state.ctx, args)
		state.consumeSymbolicGoFunctionErrors(func(msg string) {
			state.addError(makeSymbolicEvalError(n, state, msg))
		})

		if errors.Is(err, ErrPatternNotCallable) {
			state.addError(makeSymbolicEvalError(n.Callee, state, ErrPatternNotCallable.Error()))
			patt = &AnyPattern{}
		} else if err != nil {
			return nil, err
		}
		return patt, nil
	case *parse.PipelineStatement, *parse.PipelineExpression:
		var stages []*parse.PipelineStage

		switch e := n.(type) {
		case *parse.PipelineStatement:
			stages = e.Stages
		case *parse.PipelineExpression:
			stages = e.Stages
		}

		if savedAnonymousValue, hasValue := state.getLocal(""); hasValue {
			defer func() {
				state.updateLocal("", savedAnonymousValue.value, n)
			}()
		}

		var res SymbolicValue
		var err error

		for _, stage := range stages {
			res, err = symbolicEval(stage.Expr, state)
			if err != nil {
				return nil, err
			}
			state.setLocal("", res, nil)
		}

		return res, nil
	case *parse.LocalVariableDeclarations:
		for _, decl := range n.Declarations {
			name := decl.Left.(*parse.IdentifierLiteral).Name
			right, err := symbolicEval(decl.Right, state)
			if err != nil {
				return nil, err
			}

			var static Pattern
			if decl.Type != nil {
				type_, err := symbolicEval(decl.Type, state)
				if err != nil {
					return nil, err
				}
				static = type_.(Pattern)

				widenedRight := right
				for !isAny(widenedRight) && !static.TestValue(widenedRight) {
					widenedRight = widenOrAny(widenedRight)
				}

				if !static.TestValue(widenedRight) {
					state.addError(makeSymbolicEvalError(decl.Right, state, fmtNotAssignableToVarOftype(right, static)))
					right = ANY
				} else {
					if holder, ok := right.(StaticDataHolder); ok {
						holder.AddStatic(static) //TODO: use path narowing, values should never be modified directly
					}
				}
			}

			state.setLocal(name, right, static)
			state.symbolicData.SetNodeValue(decl.Left, right)
		}
		return nil, nil
	case *parse.Assignment:

		right, err := symbolicEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		badIntOperationRHS := false

		// if the operation requires integer operands we check that RHS is an integer
		if n.Operator.Int() {
			if _, ok := right.(*Int); !ok {
				badIntOperationRHS = true
				state.addError(makeSymbolicEvalError(n.Right, state, INVALID_INT_OPER_ASSIGN_RHS_NOT_INT))
			}
		}

		switch lhs := n.Left.(type) {
		case *parse.Variable:
			name := lhs.Name

			if state.hasLocal(name) {
				if n.Operator.Int() {
					info, _ := state.getLocal(name)

					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateLocal(name, right, node)
					}
				} else {
					state.updateLocal(name, right, node)
				}

			} else {
				state.setLocal(name, right, nil)
			}
			state.symbolicData.SetNodeValue(lhs, right)
		case *parse.IdentifierLiteral:
			name := lhs.Name

			if state.hasLocal(name) {
				if n.Operator.Int() {
					info, _ := state.getLocal(name)
					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateLocal(name, right, node)
					}
				} else {
					state.updateLocal(name, right, node)
				}

			} else {
				state.setLocal(name, right, nil)
			}
			state.symbolicData.SetNodeValue(lhs, right)
		case *parse.GlobalVariable:
			name := lhs.Name

			info, alreadyDefined := state.getGlobal(name)
			if alreadyDefined && info.isConstant {
				state.addError(makeSymbolicEvalError(node, state, fmtAttempToAssignConstantGlobal(name)))
			}

			if state.hasGlobal(name) {
				if n.Operator.Int() {
					info, _ := state.getGlobal(name)
					if _, ok := info.value.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
					} else if !badIntOperationRHS {
						state.updateLocal(name, right, node)
					}
				} else {
					state.updateLocal(name, right, node)
				}

			} else {
				state.setGlobal(name, right, GlobalVar)
			}

			state.symbolicData.SetNodeValue(lhs, right)
		case *parse.MemberExpression:
			object, err := symbolicEval(lhs.Left, state)
			if err != nil {
				return nil, err
			}

			var iprops IProps
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
				case nil:
					return nil, errors.New("nil value")
				default:
					state.addError(makeSymbolicEvalError(node, state, FmtCannotAssignPropertyOf(val)))
					iprops = &Object{}
				}
			}

			propName := lhs.PropertyName.Name
			hasPrevValue := utils.SliceContains(iprops.PropertyNames(), propName)

			if hasPrevValue {
				prevValue := iprops.Prop(propName)

				if n.Operator.Int() {
					if _, ok := prevValue.(*Int); !ok {
						state.addError(makeSymbolicEvalError(node, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
					}
				} else if badIntOperationRHS {

				} else {
					if newIprops, err := iprops.SetProp(propName, right); err != nil {
						state.addError(makeSymbolicEvalError(node, state, err.Error()))
					} else {
						narrowPath(lhs.Left, setExactValue, newIprops, state, 0)
					}
				}

			} else {
				if newIprops, err := iprops.SetProp(propName, right); err != nil {
					state.addError(makeSymbolicEvalError(node, state, err.Error()))
				} else {
					narrowPath(lhs.Left, setExactValue, newIprops, state, 0)
				}
			}

		case *parse.IdentifierMemberExpression:
			v, err := symbolicEval(lhs.Left, state)
			if err != nil {
				return nil, err
			}

			for _, idents := range lhs.PropertyNames[:len(lhs.PropertyNames)-1] {
				v = symbolicMemb(v, idents.Name, lhs, state)
			}

			var iprops IProps
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
				case nil:
					return nil, errors.New("nil value")
				default:
					state.addError(makeSymbolicEvalError(node, state, FmtCannotAssignPropertyOf(val)))
					iprops = &Object{}
				}
			}

			lastPropName := lhs.PropertyNames[len(lhs.PropertyNames)-1].Name
			hasPrevValue := utils.SliceContains(iprops.PropertyNames(), lastPropName)

			if hasPrevValue {
				prevValue := iprops.Prop(lastPropName)

				if _, ok := prevValue.(*Int); !ok && n.Operator.Int() {
					state.addError(makeSymbolicEvalError(node, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
				} else {
					if newIprops, err := iprops.SetProp(lastPropName, right); err != nil {
						state.addError(makeSymbolicEvalError(node, state, err.Error()))
					} else {
						narrowPath(lhs, setExactValue, newIprops, state, 1)
					}
				}
			} else {
				if newIprops, err := iprops.SetProp(lastPropName, right); err != nil {
					state.addError(makeSymbolicEvalError(node, state, err.Error()))
				} else {
					narrowPath(lhs, setExactValue, newIprops, state, 1)
				}
			}
		case *parse.IndexExpression:
			slice, err := symbolicEval(lhs.Indexed, state)
			if err != nil {
				return nil, err
			}

			if seq, ok := asIndexable(slice).(MutableSequence); ok {
				if n.Operator.Int() && !(&Int{}).Test(seq.element()) {
					state.addError(makeSymbolicEvalError(lhs, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
				}
			} else {
				state.addError(makeSymbolicEvalError(lhs.Indexed, state, fmtXisNotAMutableSequence(slice)))
				slice = &List{generalElement: ANY}
			}

			index, err := symbolicEval(lhs.Index, state)
			if err != nil {
				return nil, err
			}

			_, err = symbolicEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			if _, ok := index.(*Int); !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtIndexIsNotAnIntButA(index)))
			}

			return nil, nil
		case *parse.SliceExpression:
			slice, err := symbolicEval(lhs.Indexed, state)
			if err != nil {
				return nil, err
			}

			if _, ok := slice.(MutableSequence); ok {
				if n.Operator.Int() {
					state.addError(makeSymbolicEvalError(lhs, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT))
				}
			} else {
				state.addError(makeSymbolicEvalError(lhs.Indexed, state, fmtMutableSequenceExpectedButIs(slice)))
			}

			startIndex, err := symbolicEval(lhs.StartIndex, state)
			if err != nil {
				return nil, err
			}

			endIndex, err := symbolicEval(lhs.EndIndex, state)
			if err != nil {
				return nil, err
			}

			_, err = symbolicEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			if _, ok := startIndex.(*Int); !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtStartIndexIsNotAnIntButA(startIndex)))
			}

			if _, ok := endIndex.(*Int); !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtEndIndexIsNotAnIntButA(endIndex)))
			}

			return nil, nil
		default:
			return nil, fmt.Errorf("invalid assignment: left hand side is a(n) %T", n.Left)
		}

		return nil, nil
	case *parse.MultiAssignment:
		right, err := symbolicEval(n.Right, state)

		if err != nil {
			return nil, err
		}

		for !isAny(right) {
			if _, ok := right.(*List); !ok {
				right = widenOrAny(right)
			} else {
				break
			}
		}

		list, ok := right.(*List)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtListExpectedButIs(right)))
			right = &List{generalElement: ANY}

			for _, var_ := range n.Variables {
				name := var_.(*parse.IdentifierLiteral).Name

				if state.hasLocal(name) {
					state.symbolicData.SetNodeValue(var_, ANY)
				} else {
					state.setLocal(name, ANY, nil)
				}
				state.symbolicData.SetNodeValue(var_, ANY)
			}
		} else {
			if list.HasKnownLen() && list.knownLen() < 2 {
				state.addError(makeSymbolicEvalError(node, state, LIST_SHOULD_HAVE_LEN_GEQ_TWO))
			}

			for i, var_ := range n.Variables {
				name := var_.(*parse.IdentifierLiteral).Name

				val := list.elementAt(i)
				if state.hasLocal(name) {
					state.updateLocal(name, val, n)
				} else {
					state.setLocal(name, val, nil)
				}
				state.symbolicData.SetNodeValue(var_, val)
			}
		}

		return nil, nil
	case *parse.HostAliasDefinition:
		name := n.Left.Value[1:]
		value, err := symbolicEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		if host, ok := value.(*Host); ok {
			state.ctx.AddHostAlias(name, host)
		} else {
			state.addError(makeSymbolicEvalError(node, state, fmtCannotCreateHostAliasWithA(value)))
			state.ctx.AddHostAlias(name, &Host{})
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

		//CONSTANTS
		if n.GlobalConstantDeclarations != nil {
			for _, decl := range n.GlobalConstantDeclarations.Declarations {
				constVal, err := symbolicEval(decl.Right, state)
				if err != nil {
					return nil, err
				}
				if !state.setGlobal(decl.Left.Name, constVal, GlobalConst) {
					return nil, fmt.Errorf("failed to set global '%s'", decl.Left.Name)
				}
			}
		}

		//STATEMENTS

		if len(n.Statements) == 1 {
			res, err := symbolicEval(n.Statements[0], state)
			if err != nil {
				return nil, err
			}
			if state.returnValue != nil {
				return *state.returnValue, nil
			}

			return res, nil
		}

		for _, stmt := range n.Statements {
			_, err := symbolicEval(stmt, state)

			if err != nil {
				return nil, err
			}
			if state.returnValue != nil {
				return *state.returnValue, nil
			}
		}

		return nil, nil
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
		chunk := state.Module.InclusionStatementMap[n]
		state.pushChunk(chunk.ParsedChunk)
		defer state.popChunk()

		return symbolicEval(chunk.Node, state)
	case *parse.ImportStatement:
		state.setGlobal(n.Identifier.Name, ANY, GlobalConst)
		return nil, nil
	case *parse.SpawnExpression:
		var actualGlobals = map[string]SymbolicValue{}
		var embeddedModule *parse.Chunk

		var meta SymbolicValue

		if n.Meta != nil {
			v, err := symbolicEval(n.Meta, state)
			if err != nil {
				return nil, err
			}
			meta = v
		}

		var globals SymbolicValue

		if obj, ok := meta.(*Object); ok {
			val, _, ok := obj.GetProperty("globals")
			if ok {
				globals = val
			}
			val, _, ok = obj.GetProperty("group")
			if ok {
				_, ok := val.(*RoutineGroup)
				if !ok {
					state.addError(makeSymbolicEvalError(n.Meta, state, fmtGroupPropertyNotRoutineGroup(val)))
				}
			}
		}

		switch g := globals.(type) {
		case *Object:
			for k, v := range g.entries {
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

		modCtx := NewSymbolicContext()
		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		modState.Module = state.Module

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

		for _, err := range modState.errors {
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
				fork.setLocal(keyVarname, keyVal, nil)
				state.symbolicData.SetNodeValue(e.KeyVar, keyVal)

				if e.GroupMatchingVariable != nil {
					matchingVarName := e.GroupMatchingVariable.(*parse.IdentifierLiteral).Name
					anyObj := NewAnyObject()
					fork.setLocal(matchingVarName, anyObj, nil)
					state.symbolicData.SetNodeValue(e.GroupMatchingVariable, anyObj)
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
		entries := map[string]SymbolicValue{}
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
				entries[name] = v
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

		obj := NewObject(entries, nil)

		if len(cycles) > 0 {
			state.addError(makeSymbolicEvalError(node, state, fmtMethodCyclesDetected(cycles)))
			return &Object{}, nil
		} else {
			prevNextSelf, restoreNextSelf := state.getNextSelf()
			if restoreNextSelf {
				state.unsetNextSelf()
			}
			state.setNextSelf(obj)

			for _, key := range keys {
				p := keyToProp[key]

				var static Pattern

				propVal, err := symbolicEval(p.Value, state)
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
				}

				obj.initNewProp(key, propVal, static)
				state.symbolicData.SetNodeValue(p.Key, propVal)
			}
			state.unsetNextSelf()
			if restoreNextSelf {
				state.setNextSelf(prevNextSelf)
			}
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
				state.addError(makeSymbolicEvalError(node, state, fmtCannotInitializedMetaProp(p.Name())))
			}
		}

		return obj, nil
	case *parse.RecordLiteral:
		entries := map[string]SymbolicValue{}
		rec := &Record{}

		if len(n.Properties)+len(n.SpreadElements) > 0 {
			rec = NewBoundEntriesRecord(entries)
		}

		indexKey := 0
		for _, p := range n.Properties {
			v, err := symbolicEval(p.Value, state)
			if err != nil {
				return nil, err
			}

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

			if v.IsMutable() {
				state.addError(makeSymbolicEvalError(p.Value, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable(key)))
				entries[key] = ANY
			} else {
				entries[key] = v
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

		if indexKey != 0 {
			entries[extData.IMPLICIT_KEY_LEN_KEY] = &Int{}
		}

		return rec, nil
	case *parse.ListLiteral:
		elements := make([]SymbolicValue, 0)

		if n.TypeAnnotation != nil {
			generalElemPattern, err := symbolicEval(n.TypeAnnotation, state)
			if err != nil {
				return nil, err
			}

			generalElem := generalElemPattern.(Pattern).SymbolicValue()

			for _, elemNode := range n.Elements {
				e, err := symbolicEval(elemNode, state)
				if err != nil {
					return nil, err
				}
				if !generalElem.Test(e) {
					state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListAnnotated(e, generalElemPattern.(Pattern))))
				}
			}

			return NewListOf(generalElem), nil
		}

		if !n.HasSpreadElements() {
			for _, elemNode := range n.Elements {
				e, err := symbolicEval(elemNode, state)
				if err != nil {
					return nil, err
				}
				elements = append(elements, e)
			}
			return NewList(elements...), nil
		}

		return NewListOf(ANY), nil
	case *parse.TupleLiteral:
		elements := make([]SymbolicValue, 0)

		if n.HasSpreadElements() {
			return nil, errors.New("spread elements not supported yet in tuple literals")
		}

		if n.TypeAnnotation != nil {
			generalElemPattern, err := symbolicEval(n.TypeAnnotation, state)
			if err != nil {
				return nil, err
			}

			generalElem := generalElemPattern.(Pattern).SymbolicValue()

			for _, elemNode := range n.Elements {
				e, err := symbolicEval(elemNode, state)
				if err != nil {
					return nil, err
				}
				if e.IsMutable() {
					state.addError(makeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE))
					e = ANY
				}

				if !generalElem.Test(e) {
					state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(e, generalElemPattern.(Pattern))))
				}
			}

			return NewTupleOf(generalElem), nil
		}

		if len(n.Elements) > 0 {
			for _, elemNode := range n.Elements {
				e, err := symbolicEval(elemNode, state)
				if err != nil {
					return nil, err
				}

				if e.IsMutable() {
					state.addError(makeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE))
					e = ANY
				}

				elements = append(elements, e)
			}
			return NewTuple(elements...), nil
		}

		return NewTupleOf(ANY), nil
	case *parse.DictionaryLiteral:
		entries := make(map[string]SymbolicValue)
		keys := make(map[string]SymbolicValue)

		for _, entry := range n.Entries {
			keyRepr := parse.SPrint(entry.Key, parse.PrintConfig{TrimStart: true})

			v, err := symbolicEval(entry.Value, state)
			if err != nil {
				return nil, err
			}

			entries[keyRepr] = v

			node, ok := parse.ParseExpression(keyRepr)
			if !ok {
				panic(fmt.Errorf("invalid key representation '%s'", keyRepr))
			}
			//TODO: refactor
			key, _ := symbolicEval(node, newSymbolicState(NewSymbolicContext(), nil))
			keys[keyRepr] = key
			state.symbolicData.SetNodeValue(entry.Key, key)
		}

		return NewDictionary(entries, keys), nil
	case *parse.IfStatement:
		test, err := symbolicEval(n.Test, state)
		if err != nil {
			return nil, err
		}

		if _, ok := test.(*Bool); !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtIfStmtTestNotBoolBut(test)))
		}

		//consequent
		var consequentStateFork *State
		{
			consequentStateFork = state.fork()
			if boolConvExpr, ok := n.Test.(*parse.BooleanConversionExpression); ok {
				narrowPath(boolConvExpr.Expr, removePossibleValue, Nil, consequentStateFork, 0)
			}

			_, err = symbolicEval(n.Consequent, consequentStateFork)
			if err != nil {
				return nil, err
			}
		}

		var alternateStateFork *State
		if n.Alternate != nil {
			alternateStateFork = state.fork()
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

		return nil, nil
	case *parse.IfExpression:
		test, err := symbolicEval(n.Test, state)
		if err != nil {
			return nil, err
		}

		if _, ok := test.(*Bool); ok {
			consequentStateFork := state.fork()
			_, err = symbolicEval(n.Consequent, consequentStateFork)
			if err != nil {
				return nil, err
			}

			if n.Alternate != nil {
				alternateStateFork := state.fork()
				_, err = symbolicEval(n.Alternate, alternateStateFork)
				if err != nil {
					return nil, err
				}
			}
			return ANY, nil
		} else {
			state.addError(makeSymbolicEvalError(node, state, fmtIfExprTestNotBoolBut(test)))
		}
		return nil, nil
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

		defer func() {
			if n.KeyIndexIdent != nil {
				state.deleteLocal(kVarname)
			}
			if n.ValueElemIdent != nil {
				state.deleteLocal(eVarname)
			}
		}()

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

		if n.KeyIndexIdent != nil {
			state.setLocal(kVarname, keyType, nil)
			state.symbolicData.SetNodeValue(n.KeyIndexIdent, keyType)
		}
		if n.ValueElemIdent != nil {
			state.setLocal(eVarname, valueType, nil)
			state.symbolicData.SetNodeValue(n.ValueElemIdent, valueType)
		}

		_, err = symbolicEval(n.Body, state)
		if err != nil {
			return nil, err
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

		state.setLocal(n.EntryIdent.Name, entry, nil)
		state.symbolicData.SetNodeValue(n.EntryIdent, entry)

		if n.MetaIdent != nil {
			state.setLocal(n.MetaIdent.Name, nodeMeta, nil)
			state.symbolicData.SetNodeValue(n.MetaIdent, nodeMeta)
		}

		_, blkErr := symbolicEval(n.Body, state)
		if blkErr != nil {
			return nil, blkErr
		}

		state.iterationChange = NoIterationChange
		return nil, nil
	case *parse.SwitchStatement:
		_, err := symbolicEval(n.Discriminant, state)
		if err != nil {
			return nil, err
		}
		for _, switchCase := range n.Cases {
			for _, valNode := range switchCase.Values {
				_, err := symbolicEval(valNode, state)
				if err != nil {
					return nil, err
				}

				blockStateFork := state.fork()
				_, err = symbolicEval(switchCase.Block, blockStateFork)
				if err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	case *parse.MatchStatement:
		discriminant, err := symbolicEval(n.Discriminant, state)
		if err != nil {
			return nil, err
		}

		var forks []*State

		for _, matchCase := range n.Cases {
			for _, valNode := range matchCase.Values { //TODO: fix handling of multi cases
				val, err := symbolicEval(valNode, state)
				if err != nil {
					return nil, err
				}

				pattern, ok := val.(Pattern)

				if !ok { //if the value of the case is not a pattern we just check for equality
					pattern = NewExactValuePattern(val)
				}

				blockStateFork := state.fork()
				forks = append(forks, blockStateFork)
				narrowPath(n.Discriminant, setExactValue, pattern.SymbolicValue(), blockStateFork, 0)

				if matchCase.GroupMatchingVariable != nil {
					variable := matchCase.GroupMatchingVariable.(*parse.IdentifierLiteral)
					groupPattern, ok := pattern.(GroupPattern)

					if !ok {
						state.addError(makeSymbolicEvalError(node, state, fmtXisNotAGroupMatchingPattern(pattern)))
					} else {
						ok, groups := groupPattern.MatchGroups(discriminant)
						if ok {
							groupsObj := NewObject(groups, nil)
							blockStateFork.setLocal(variable.Name, groupsObj, nil)
							state.symbolicData.SetNodeValue(variable, groupsObj)

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
				return &Int{}, nil
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

			return &Bool{}, nil
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

		switch n.Operator {
		case parse.Add, parse.Sub, parse.Mul, parse.Div, parse.GreaterThan, parse.LessThan, parse.LessOrEqual, parse.GreaterOrEqual:

			if _, ok := left.(*Int); ok {
				_, ok = right.(*Int)
				if !ok {
					state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "int", right.String())))
				}

				switch n.Operator {
				case parse.Add, parse.Sub, parse.Mul, parse.Div:
					return &Int{}, nil
				default:
					return &Bool{}, nil
				}
			} else if _, ok := left.(*Float); ok {
				_, ok = right.(*Float)
				if !ok {
					state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "float", right.String())))
				}
				switch n.Operator {
				case parse.Add, parse.Sub, parse.Mul, parse.Div:
					return &Float{}, nil
				default:
					return &Bool{}, nil
				}
			} else {
				state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "int or float", left.String())))

				switch right.(type) {
				case *Int:
					return &Int{}, nil
				case *Float:
					return &Float{}, nil
				default:
					state.addError(makeSymbolicEvalError(n.Left, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "int or float", right.String())))
					return ANY, nil
				}
			}

		case parse.AddDot, parse.SubDot, parse.MulDot, parse.DivDot, parse.GreaterThanDot, parse.GreaterOrEqualDot, parse.LessThanDot, parse.LessOrEqualDot:
			state.addError(makeSymbolicEvalError(node, state, "operator not implemented yet"))
			return ANY, nil
		case parse.Equal, parse.NotEqual, parse.Is, parse.IsNot:
			return &Bool{}, nil
		case parse.In:
			switch right.(type) {
			case *List, *Object:
			default:
				state.addError(makeSymbolicEvalError(node, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "iterable", right.String())))
			}
			return &Bool{}, nil
		case parse.NotIn:
			switch right.(type) {
			case *List, *Object:
			default:
				state.addError(makeSymbolicEvalError(node, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "iterable", right.String())))
			}
			return &Bool{}, nil
		case parse.Keyof:
			_, ok := left.(*String)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string", left.String())))
			}

			switch rightVal := right.(type) {
			case *Object:
			default:
				state.addError(makeSymbolicEvalError(node, state, fmtInvalidBinExprCannnotCheckNonObjectHasKey(rightVal)))
			}
			return &Bool{}, nil
		case parse.Range, parse.ExclEndRange:
			return &IntRange{}, nil
		case parse.And, parse.Or:
			_, ok := left.(*Bool)

			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "boolean", left.String())))
			}

			_, ok = right.(*Bool)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "boolean", right.String())))
			}
			return &Bool{}, nil
		case parse.Match, parse.NotMatch:
			_, ok := right.(Pattern)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "pattern", right.String())))
			}

			return &Bool{}, nil
		case parse.Substrof:

			switch left.(type) {
			case *RuneSlice, *ByteSlice:
			default:
				if _, ok := left.(StringLike); !ok {
					state.addError(makeSymbolicEvalError(node, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string-like", left.String())))
				}
			}

			switch right.(type) {
			case *RuneSlice, *ByteSlice:
			default:
				if _, ok := right.(StringLike); !ok {
					state.addError(makeSymbolicEvalError(node, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "string-like", right.String())))
				}
			}

			return &Bool{}, nil
		case parse.SetDifference:
			if _, ok := left.(Pattern); !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "pattern", left.String())))
			}
			return &DifferencePattern{
				Base:    &AnyPattern{},
				Removed: &AnyPattern{},
			}, nil
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
			return &IntRange{}, nil
		case *Float:
			return nil, fmt.Errorf("floating point ranges not supported")
		default:
			return &QuantityRange{}, nil
		}
	case *parse.IntegerRangeLiteral:
		return &IntRange{}, nil
	case *parse.RuneRangeExpression:
		return &RuneRange{}, nil
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
			}

			stateFork.setLocal(name, paramValue, paramType)
			state.symbolicData.SetNodeValue(p.Var, paramValue)
			params[i] = paramValue
			paramNames[i] = name
		}

		//declare captured locals
		capturedLocals := map[string]SymbolicValue{}
		for _, e := range n.CaptureList {
			name := e.(*parse.IdentifierLiteral).Name
			info, ok := state.getLocal(name)
			if ok {
				stateFork.setLocal(name, info.value, info.static)
				capturedLocals[name] = info.value
			} else {
				stateFork.setLocal(name, ANY, nil)
				capturedLocals[name] = ANY
				state.addError(makeSymbolicEvalError(e, state, fmtLocalVarIsNotDeclared(name)))
			}
		}

		if n.IsVariadic {
			variadicParam := n.VariadicParameter()
			stateFork.setLocal(variadicParam.Var.Name, &List{generalElement: ANY}, nil)
		}

		//-----------------------------

		var signatureReturnType *SymbolicValue
		var storedReturnType SymbolicValue
		var err error

		if n.ReturnType != nil {
			pattern, err := symbolicallyEvalPatternNode(n.ReturnType, stateFork)
			if err != nil {
				return nil, err
			}
			typ := pattern.SymbolicValue()
			signatureReturnType = &typ
		}

		if n.IsBodyExpression {
			storedReturnType, err = symbolicEval(n.Body, stateFork)
			if err != nil {
				return nil, err
			}

			if signatureReturnType != nil {
				storedReturnType = *signatureReturnType
				if !(*signatureReturnType).Test(storedReturnType) {
					state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(storedReturnType, *signatureReturnType)))
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
			retValuePtr := stateFork.returnValue

			if signatureReturnType != nil {
				storedReturnType = *signatureReturnType
				if retValuePtr == nil {
					stateFork.addError(makeSymbolicEvalError(n, stateFork, MISSING_RETURN_IN_FUNCTION))
				}
			} else if retValuePtr == nil {
				storedReturnType = Nil
			} else {
				storedReturnType = *retValuePtr
			}
		}

		if len(capturedLocals) == 0 {
			capturedLocals = nil
		}

		return &InoxFunction{
			node:           n,
			parameters:     params,
			parameterNames: paramNames,
			returnType:     storedReturnType,
			capturedLocals: capturedLocals,
		}, nil
	case *parse.FunctionDeclaration:
		funcName := n.Name.Name

		//declare the function before checking it
		state.setGlobal(funcName, &InoxFunction{node: n.Function, returnType: ANY}, GlobalConst)
		v, err := symbolicEval(n.Function, state)
		if err == nil {
			state.overrideGlobal(funcName, v)
		}
		return nil, err
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

		//declare arguments
		for _, p := range n.Parameters[:n.NonVariadicParamCount()] {
			name := p.Var.Name
			var paramType SymbolicValue = ANY

			if p.Type != nil {
				pattern, err := symbolicallyEvalPatternNode(p.Type, stateFork)
				if err != nil {
					return nil, err
				}
				paramType = pattern.SymbolicValue()
			}

			stateFork.setLocal(name, paramType, nil)
			state.symbolicData.SetNodeValue(p.Var, paramType)
		}

		if n.IsVariadic {
			variadicParam := n.VariadicParameter()
			paramValue := &List{generalElement: ANY}
			stateFork.setLocal(variadicParam.Var.Name, paramValue, nil)
			state.symbolicData.SetNodeValue(variadicParam.Var, paramValue)
		}

		//-----------------------------

		var signatureReturnType *SymbolicValue
		var storedReturnType SymbolicValue
		var err error

		if n.ReturnType != nil {
			pattern, err := symbolicallyEvalPatternNode(n.ReturnType, stateFork)
			if err != nil {
				return nil, err
			}
			typ := pattern.SymbolicValue()
			signatureReturnType = &typ
		}

		if n.IsBodyExpression {
			storedReturnType, err = symbolicEval(n.Body, stateFork)
			if err != nil {
				return nil, err
			}

			if signatureReturnType != nil {
				storedReturnType = *signatureReturnType
				if !(*signatureReturnType).Test(storedReturnType) {
					state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(storedReturnType, *signatureReturnType)))
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
				storedReturnType = *signatureReturnType
				if retValuePtr == nil && n.Body != nil {
					stateFork.addError(makeSymbolicEvalError(n, stateFork, MISSING_RETURN_IN_FUNCTION_PATT))
				}
			} else if retValuePtr == nil {
				storedReturnType = Nil
			} else {
				storedReturnType = *retValuePtr
			}
		}

		return &FunctionPattern{
			node:       n,
			returnType: storedReturnType,
		}, nil
	case *parse.PatternConversionExpression:
		v, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		if patt, ok := v.(Pattern); ok {
			return patt, nil
		}
		return &ExactValuePattern{value: v}, nil
	case *parse.LazyExpression:
		return &AstNode{Node: n}, nil
	case *parse.MemberExpression:
		left, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		val := symbolicMemb(left, n.PropertyName.Name, n, state)
		state.symbolicData.SetNodeValue(n.PropertyName, val)

		return val, nil
	case *parse.IdentifierMemberExpression:
		v, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		var prevIdent *parse.IdentifierLiteral
		for _, ident := range n.PropertyNames {
			if prevIdent != nil {
				state.symbolicData.SetNodeValue(prevIdent, v)
			}
			v = symbolicMemb(v, ident.Name, n, state)
			prevIdent = ident
		}

		state.symbolicData.SetNodeValue(prevIdent, v)

		return v, nil
	case *parse.DynamicMemberExpression:
		left, err := symbolicEval(n.Left, state)
		if err != nil {
			return nil, err
		}
		iprops, ok := asIprops(left).(IProps)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtCannotGetDynamicMemberOfValueWithNoProps(left)))
			return ANY, nil
		}

		return NewDynamicValue(symbolicMemb(iprops, n.PropertyName.Name, n, state)), nil
	case *parse.ExtractionExpression:
		left, err := symbolicEval(n.Object, state)
		if err != nil {
			return nil, err
		}
		result := &Object{
			entries: make(map[string]SymbolicValue),
		}

		for _, key := range n.Keys.Keys {
			name := key.(*parse.IdentifierLiteral).Name
			result.entries[name] = symbolicMemb(left, name, n, state)
		}
		return result, nil
	case *parse.IndexExpression:
		val, err := symbolicEval(n.Indexed, state)
		if err != nil {
			return nil, err
		}

		index, err := symbolicEval(n.Index, state)
		if err != nil {
			return nil, err
		}

		if _, ok := index.(*Int); !ok {
			state.addError(makeSymbolicEvalError(node, state, fmtIndexIsNotAnIntButA(index)))
			index = &Int{}
		}

		if indexable, ok := asIndexable(val).(Indexable); ok {
			return indexable.element(), nil
		}

		state.addError(makeSymbolicEvalError(node, state, fmtXisNotIndexable(val)))
		return ANY, nil
	case *parse.SliceExpression:
		slice, err := symbolicEval(n.Indexed, state)
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

		if seq, ok := slice.(Sequence); ok {
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
		//TODO: check coercion
		return &Bool{}, nil
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
		state.ctx.AddNamedPattern(n.Left.Name, pattern)
		return nil, nil
	case *parse.PatternNamespaceDefinition:
		right, err := symbolicEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		namespace := &PatternNamespace{}

		switch r := right.(type) {
		case *Object:
			if len(r.entries) > 0 {
				namespace.entries = make(map[string]Pattern)
			}
			for k, v := range r.entries {
				namespace.entries[k] = v.(Pattern)
			}
			state.ctx.AddPatternNamespace(n.Left.Name, namespace)
		case *Record:
			if len(r.entries) > 0 {
				namespace.entries = make(map[string]Pattern)
			}
			for k, v := range r.entries {
				namespace.entries[k] = v.(Pattern)
			}
			state.ctx.AddPatternNamespace(n.Left.Name, namespace)
		default:
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceShouldBeInitWithNot(right)))
			state.ctx.AddPatternNamespace(n.Left.Name, &PatternNamespace{})
		}
		return nil, nil
	case *parse.PatternNamespaceIdentifierLiteral:
		namespace := state.ctx.ResolvePatternNamespace(n.Name)
		if namespace == nil {
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceIsNotDeclared(n.Name)))
			return ANY, nil
		}
		return namespace, nil
	case *parse.PatternNamespaceMemberExpression:
		namespace := state.ctx.ResolvePatternNamespace(n.Namespace.Name)
		if namespace == nil {
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceIsNotDeclared(n.Namespace.Name)))
			return &AnyPattern{}, nil
		} else {
			if namespace.entries == nil {
				return &AnyPattern{}, nil
			}
			patt := namespace.entries[n.MemberName.Name]

			if patt == nil {
				return &AnyPattern{}, nil
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
		return &SequenceStringPattern{}, nil
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
			Entries: make(map[string]Pattern),
			Inexact: n.Inexact,
		}
		for _, p := range n.Properties {
			name := p.Name()
			var err error
			pattern.Entries[name], err = symbolicallyEvalPatternNode(p.Value, state)
			if err != nil {
				return nil, err
			}
		}

		for _, el := range n.SpreadElements {
			compiledElement, err := symbolicallyEvalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			if objPattern, ok := compiledElement.(*ObjectPattern); ok {
				if objPattern.Entries == nil {
					state.addError(makeSymbolicEvalError(el, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT))
				} else {
					for name, vpattern := range objPattern.Entries {
						pattern.Entries[name] = vpattern
					}
				}
				// else if objPattern.Inexact {
				// state.addError(makeSymbolicEvalError(el, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT))
				//

			} else {
				state.addError(makeSymbolicEvalError(el, state, fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(compiledElement)))
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
	case *parse.OptionPatternLiteral:
		return &OptionPattern{}, nil
	case *parse.ByteSliceLiteral:
		return &ByteSlice{}, nil
	case *parse.ConcatenationExpression:
		if len(n.Elements) == 0 {
			return nil, errors.New("cannot create concatenation with no elements")
		}
		values := make([]SymbolicValue, len(n.Elements))
		for i, elemNode := range n.Elements {
			elem, err := symbolicEval(elemNode, state)
			if err != nil {
				return nil, err
			}
			values[i] = elem
		}

		switch values[0].(type) {
		case StringLike:
			if len(values) == 1 {
				return values[0], nil
			}
			for i, elem := range values {
				if _, ok := elem.(StringLike); !ok {
					state.addError(makeSymbolicEvalError(n.Elements[i], state, fmt.Sprintf("string concatenation: invalid element of type %T", elem)))
				}
			}
			return &StringConcatenation{}, nil
		case BytesLike:
			if len(values) == 1 {
				return values[0], nil
			}
			for i, elem := range values {
				if _, ok := elem.(BytesLike); !ok {
					state.addError(makeSymbolicEvalError(n.Elements[i], state, fmt.Sprintf("bytes concatenation: invalid element of type %T", elem)))
				}
			}
			return &BytesConcatenation{}, nil
		case *Tuple:
			if len(values) == 1 {
				return values[0], nil
			}

			var generalElements []SymbolicValue
			var elements []SymbolicValue

			for i, concatElem := range values {
				if tuple, ok := concatElem.(*Tuple); ok {
					if tuple.HasKnownLen() {
						elements = append(elements, tuple.elements...)
					} else {
						generalElements = append(generalElements, tuple.generalElement)
					}
				} else {
					state.addError(makeSymbolicEvalError(n.Elements[i], state, fmt.Sprintf("tuple concatenation: invalid element of type %T", concatElem)))
				}
			}

			if elements == nil {
				return NewTupleOf(joinValues(generalElements)), nil
			} else {
				return NewTuple(elements...), nil
			}
		default:
			return nil, errors.New("only string, bytes & tuple concatenations are supported for now")
		}
	case *parse.AssertionStatement:
		ok, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}
		if _, isBool := ok.(*Bool); !isBool {
			state.addError(makeSymbolicEvalError(node, state, fmtAssertedValueShouldBeBoolNot(ok)))
		}

		if binExpr, ok := n.Expr.(*parse.BinaryExpression); ok {
			isVar := parse.IsAnyVariableIdentifier(binExpr.Left)
			if !isVar {
				return nil, nil
			}

			switch binExpr.Operator {
			case parse.Match:

				switch n := binExpr.Right.(type) {
				case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
					*parse.ObjectPatternLiteral, *parse.ListPatternLiteral, *parse.FunctionPatternExpression:
					pattern, err := symbolicallyEvalPatternNode(n, state)
					if err != nil {
						return nil, err
					}

					name := parse.GetVariableName(binExpr.Left)
					if !state.hasGlobal(name) && !state.hasLocal(name) {
						break
					}

					if state.hasLocal(name) {
						state.updateLocal(name, pattern.SymbolicValue(), node)
					} else {
						state.updateGlobal(name, pattern.SymbolicValue(), node)
					}
				default:
				}
			}
		}
		return nil, nil
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

		modCtx := NewSymbolicContext()
		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		modState.Module = state.Module

		_, err = symbolicEval(embeddedModule, modState)
		if err != nil {
			return nil, err
		}

		for _, err := range modState.errors {
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

		modCtx := NewSymbolicContext()
		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		modState.Module = state.Module

		_, err = symbolicEval(embeddedModule, modState)
		if err != nil {
			return nil, err
		}

		for _, err := range modState.errors {
			state.addError(err)
		}

		return &TestCase{}, nil
	case *parse.LifetimejobExpression:
		_, err := symbolicEval(n.Meta, state)
		if err != nil {
			return nil, err
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
		modCtx := NewSymbolicContext()
		state.ctx.ForEachPattern(func(name string, pattern Pattern) {
			modCtx.AddNamedPattern(name, pattern)
		})
		state.ctx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace) {
			modCtx.AddPatternNamespace(name, namespace)
		})

		modState := newSymbolicState(modCtx, &parse.ParsedChunk{
			Node:   embeddedModule,
			Source: state.currentChunk().Source,
		})
		state.forEachGlobal(func(name string, info varSymbolicInfo) {
			modState.setGlobal(name, info.value, info.constness())
		})

		modState.Module = state.Module

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

		for _, err := range modState.errors {
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

		for _, slice := range n.Slices {

			switch s := slice.(type) {
			case *parse.StringTemplateSlice:
			case *parse.StringTemplateInterpolation:
				memberName := s.Type
				_, ok := namespace.entries[memberName]
				if !ok {
					state.addError(makeSymbolicEvalError(node, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(memberName, namespaceName)))
					return &CheckedString{}, nil
				}

				e, err := symbolicEval(s.Expr, state)
				if err != nil {
					return nil, err
				}

				if _, ok := e.(*String); !ok {
					state.addError(makeSymbolicEvalError(node, state, fmtInterpolationIsNotStringBut(e)))
				}
			}
		}

		return &CheckedString{}, nil
	case *parse.CssSelectorExpression:
		return &String{}, nil
	default:
		return nil, fmt.Errorf("cannot evaluate %#v (%T)\n%s", node, node, debug.Stack())
	}
}

func callSymbolicFunc(callNode *parse.CallExpression, calleeNode parse.Node, state *State, arguments interface{}, must bool, cmdLineSyntax bool) (SymbolicValue, error) {
	var (
		callee SymbolicValue
		err    error
		self   *Object
	)
	//we first get the callee

	switch c := calleeNode.(type) {
	case *parse.IdentifierLiteral, *parse.IdentifierMemberExpression, *parse.Variable, *parse.MemberExpression:
		callee, err = symbolicEval(callNode.Callee, state)
		if err != nil {
			return nil, err
		}
		state.symbolicData.SetNodeValue(callNode.Callee, callee)
	case *parse.FunctionDeclaration, *parse.FunctionExpression:
		callee = &AstNode{Node: c}
	default:
		return nil, fmt.Errorf("(symbolic) cannot call a(n) %T", c)
	}

	var extState *State
	isSharedFunction := false

	if inoxFn, ok := callee.(*InoxFunction); ok {
		isSharedFunction = inoxFn.IsShared()
		if isSharedFunction {
			extState = inoxFn.originState
		}

	} else if goFn, ok := callee.(*GoFunction); ok {
		isSharedFunction = goFn.IsShared()
		if isSharedFunction {
			extState = goFn.originState
		}
	} else if _, ok := callee.(*Function); ok {
		// ok
	} else {
		state.addError(makeSymbolicEvalError(callNode, state, fmtCannotCall(callee)))
		return ANY, nil
	}

	//EVALUATION OF ARGUMENTS

	args := make([]SymbolicValue, 0)
	nonSpreadArgCount := 0
	hasSpreadArg := false

	var argNodes []parse.Node

	if l, ok := arguments.([]SymbolicValue); ok {
		args = l
	} else {
		argNodes = arguments.([]parse.Node)

		for _, argn := range argNodes {

			if spreadArg, ok := argn.(*parse.SpreadArgument); ok {
				hasSpreadArg = true
				v, err := symbolicEval(spreadArg.Expr, state)
				if err != nil {
					return nil, err
				}

				list, ok := v.(*List)

				if ok {
					for _, e := range list.elements {
						//same logic for non spread arguments
						if isSharedFunction {
							shared, err := ShareOrClone(e, state)
							if err != nil {
								state.addError(makeSymbolicEvalError(argn, state, err.Error()))
								shared = ANY
							}
							e = shared
						}
						args = append(args, e)
					}
				} else {
					state.addError(makeSymbolicEvalError(argn, state, fmtSpreadArgumentShouldBeList(v)))
				}

			} else {
				nonSpreadArgCount++

				if ident, ok := argn.(*parse.IdentifierLiteral); ok && cmdLineSyntax {
					args = append(args, &Identifier{ident.Name})
				} else {
					arg, err := symbolicEval(argn, state)
					if err != nil {
						return nil, err
					}
					if isSharedFunction {
						shared, err := ShareOrClone(arg, state)
						if err != nil {
							state.addError(makeSymbolicEvalError(argn, state, err.Error()))
							shared = ANY
						}
						arg = shared
					}
					args = append(args, arg)
				}
			}

		}
	}

	//EXECUTION

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
	case *GoFunction:
		//GO FUNCTION

		result, err := f.Call(goFunctionCallInput{
			symbolicArgs:      args,
			nonSpreadArgCount: nonSpreadArgCount,
			hasSpreadArg:      hasSpreadArg,
			state:             state,
			extState:          extState,
			isExt:             isSharedFunction,
			must:              must,
			callNode:          callNode,
		})

		state.consumeSymbolicGoFunctionErrors(func(msg string) {
			state.addError(makeSymbolicEvalError(callNode, state, msg))
		})

		return result, err
	}

	//inox function | unknown type function
	var (
		nonVariadicParamCount int
		parameters            []*parse.FunctionParameter
		variadicParam         *parse.FunctionParameter
		returnType            parse.Node
		isBodyExpression      bool
		isVariadic            bool
	)

	if _, ok := callee.(*InoxFunction); ok {
		nonVariadicParamCount, parameters, variadicParam, returnType, isBodyExpression =
			fn.SignatureInformation()
	} else {
		nonVariadicParamCount, parameters, variadicParam, returnType, isBodyExpression =
			callee.(*Function).pattern.node.SignatureInformation()
	}

	isVariadic = variadicParam != nil

	if isVariadic {
		if nonSpreadArgCount < nonVariadicParamCount {
			state.addError(makeSymbolicEvalError(callNode, state, fmtInvalidNumberOfNonSpreadArgs(nonSpreadArgCount, nonVariadicParamCount)))
			//if they are not enough arguments we use the parameter types to set their value

			for i := len(args); i < nonVariadicParamCount; i++ {
				var paramType SymbolicValue

				paramTypeNode := parameters[i].Type
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
	} else if hasSpreadArg || len(args) != len(parameters) {

		if hasSpreadArg {
			state.addError(makeSymbolicEvalError(callNode, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS))
		} else {
			state.addError(makeSymbolicEvalError(callNode, state, fmtInvalidNumberOfArgs(len(args), len(parameters))))
		}

		if len(args) > len(parameters) {
			//if they are too many arguments we just ignore them
			args = args[:len(parameters)]
		} else {
			//if they are not enough arguments we use the parameter types to set their value
			for i := len(args); i < len(parameters); i++ {
				var paramType SymbolicValue

				paramTypeNode := parameters[i].Type
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

	for i, arg := range args {
		var paramTypeNode parse.Node

		if i >= nonVariadicParamCount {
			paramTypeNode = variadicParam.Type
		} else {
			paramTypeNode = parameters[i].Type
		}

		if paramTypeNode == nil {
			continue
		}

		pattern, err := symbolicallyEvalPatternNode(paramTypeNode, state)
		if err != nil {
			return nil, err
		}

		paramType := pattern.SymbolicValue()

		widenedArg := arg
		for !isAny(widenedArg) && !paramType.Test(widenedArg) {
			widenedArg = widenOrAny(widenedArg)
		}

		if !paramType.Test(widenedArg) {
			var node parse.Node = callNode
			if argNodes != nil {
				node = argNodes[i]
			}

			state.addError(makeSymbolicEvalError(node, state, FmtInvalidArg(i, arg, paramType)))
			args[i] = paramType
		} else {
			args[i] = widenedArg
		}
	}

	if fn == nil { // *Function
		patt, err := symbolicEval(returnType, state)
		if err != nil {
			return nil, err
		}
		return patt.(Pattern).SymbolicValue(), nil
	} //else Inox function

	//declare parameters

	state.pushScope()
	defer state.popScope()

	if self != nil {
		state.setSelf(self)
	}

	for i, p := range parameters[:nonVariadicParamCount] {
		name := p.Var.Name
		state.setLocal(name, args[i], nil)
	}

	for name, val := range capturedLocals {
		state.setLocal(name, val, nil)
	}

	if isVariadic {
		variadicArgs := NewList(args[nonVariadicParamCount:]...)
		name := variadicParam.Var.Name
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
			// we do this to prevent invalid return statements to add an error
			any := SymbolicValue(ANY)
			state.returnType = &any

			//execute body

			_, err = symbolicEval(fn.Body, state)
			if err != nil {
				return nil, err
			}

			//we retrieve and post process the return value

			retValuePtr := state.returnValue
			defer func() {
				state.returnValue = nil
				state.returnType = nil
			}()

			if retValuePtr == nil {
				return &NilT{}, nil
			}

			ret = *state.returnValue
		}

		if must {
			if list, isList := ret.(*List); isList && list.HasKnownLen() && list.knownLen() != 0 {
				lastElem := list.elements[len(list.elements)-1]

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

func symbolicMemb(value SymbolicValue, name string, node parse.Node, state *State) (result SymbolicValue) {

	if _, ok := value.(*Any); ok {
		return ANY
	}

	iprops, ok := asIprops(value).(IProps)
	if !ok {
		state.addError(makeSymbolicEvalError(node, state, fmt.Sprintf("value has no properties: %s", value.String())))
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

			state.addError(makeSymbolicEvalError(node, state, fmtPropOfSymbolicDoesNotExist(name, value, closest)))
			result = ANY
		}
	}()

	return iprops.Prop(name)
}

type pathNarrowing int

const (
	setExactValue pathNarrowing = iota
	removePossibleValue
)

func narrowPath(path parse.Node, action pathNarrowing, value SymbolicValue, state *State, ignored int) {
	switch node := path.(type) {
	case *parse.Variable:
		switch action {
		case setExactValue:
			state.updateLocal(node.Name, value, path)
		case removePossibleValue:
			prev, ok := state.getLocal(node.Name)
			if ok {
				state.updateLocal(node.Name, narrowOut(value, prev.static.SymbolicValue()), path)
			}
		}
	case *parse.GlobalVariable:
		switch action {
		case setExactValue:
			state.updateGlobal(node.Name, value, path)
		case removePossibleValue:
			prev, ok := state.getGlobal(node.Name)
			if ok {
				state.updateGlobal(node.Name, narrowOut(value, prev.static.SymbolicValue()), path)
			}
		}
	case *parse.IdentifierLiteral:
		switch action {
		case setExactValue:
			if state.hasLocal(node.Name) {
				state.updateLocal(node.Name, value, path)
			} else if state.hasGlobal(node.Name) {
				state.updateGlobal(node.Name, value, path)
			}
		case removePossibleValue:
			if state.hasLocal(node.Name) {
				prev, _ := state.getLocal(node.Name)
				state.updateLocal(node.Name, narrowOut(value, prev.static.SymbolicValue()), path)
			} else if state.hasGlobal(node.Name) {
				prev, _ := state.getGlobal(node.Name)
				state.updateGlobal(node.Name, narrowOut(value, prev.static.SymbolicValue()), path)
			}
		}
	case *parse.IdentifierMemberExpression:
		if ignored > 1 || len(node.PropertyNames) > 1 {
			panic(errors.New("not supported yet"))
		}

		switch action {
		case setExactValue:
			if ignored == 1 {
				narrowPath(node.Left, setExactValue, value, state, 0)
			} else {
				left, err := symbolicEval(node.Left, state)
				if err != nil {
					panic(err)
				}

				iprops := asIprops(left).(IProps)
				newPropValue, err := iprops.WithExistingPropReplaced(node.PropertyNames[0].Name, value)
				if err == nil {
					narrowPath(node.Left, setExactValue, newPropValue, state, 0)
				} else if err != ErrUnassignablePropsMixin {
					panic(err)
				}
			}
		case removePossibleValue:
			if ignored == 1 {
				narrowPath(node.Left, removePossibleValue, value, state, 0)
			} else {
				left, err := symbolicEval(node.Left, state)
				if err != nil {
					panic(err)
				}

				iprops := asIprops(left).(IProps)
				prevPropValue := iprops.Prop(node.PropertyNames[0].Name)
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

			iprops := asIprops(left).(IProps)
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

			iprops := asIprops(left).(IProps)
			prevPropValue := iprops.Prop(node.PropertyName.Name)
			newPropValue := narrowOut(value, prevPropValue)

			newRecPrevPropValue, err := iprops.WithExistingPropReplaced(node.PropertyName.Name, newPropValue)
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

func converReflectValToSymbolicValue(r reflect.Value) (SymbolicValue, error) {
	t := r.Type()
	err := fmt.Errorf("cannot convert value of following type to symbolic value : %v", t)

	if t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct {
		symbolicVal, ok := r.Interface().(SymbolicValue)
		if !ok {
			return nil, err
		}
		return symbolicVal, nil
	}

	switch t {
	case SYMBOLIC_VALUE_INTERFACE_TYPE, ITERABLE_INTERFACE_TYPE, RESOURCE_NAME_INTERFACE_TYPE, READABLE_INTERFACE_TYPE,
		STREAMABLE_INTERFACE_TYPE, WATCHABLE_INTERFACE_TYPE, VALUE_RECEIVER_INTERFACE_TYPE, PROTOCOL_CLIENT_INTERFACE_TYPE,
		STR_PATTERN_ELEMENT_INTERFACE_TYPE, INTEGRAL_INTERFACE_TYPE, FORMAT_INTERFACE_TYPE, IN_MEM_SNAPSHOTABLE,
		STRLIKE_INTERFACE_TYPE:
		return r.Interface().(SymbolicValue), nil
	}

	return nil, err
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
	case ITERABLE_INTERFACE_TYPE:
		return ANY_ITERABLE, nil
	case INDEXABLE_INTERFACE_TYPE:
		return ANY_INDEXABLE, nil
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
		return ANY_STR_PATTERN_ELEM, nil
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
