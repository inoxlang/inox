package core

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// TreeWalkEval evaluates a node, panics are always recovered so this function should not panic.
func TreeWalkEval(node parse.Node, state *TreeWalkState) (result Value, err error) {
	defer func() {

		var assertionErr *AssertionError

		if e := recover(); e != nil {
			if er, ok := e.(error); ok {
				if errors.As(er, &assertionErr) {
					assertionErr = assertionErr.ShallowCopy()
					er = assertionErr
				}

				err = fmt.Errorf("core: error: %w %s", er, debug.Stack())
			} else {
				err = fmt.Errorf("core: %#v", e)
			}
		}

		if err != nil && state.Global.Module != nil && state.Global.Module.Name() != "" &&
			!strings.HasPrefix(err.Error(), state.Global.Module.Name()) {

			positionStack, location := state.formatLocation(node)
			if assertionErr != nil {
				assertionErr.msg = location + " " + assertionErr.msg
			}

			err = fmt.Errorf("%s %w", location, err)

			if len(positionStack) > 0 {
				err = LocatedEvalError{
					error:    err,
					Message:  err.Error(),
					Location: positionStack,
				}
			}
		}

		if state.postHandle != nil {
			result, err = state.postHandle(node, result, err)
		}

		// TODO: unlock locked values
	}()

	if state.Global.Ctx != nil {
		select {
		case <-state.Global.Ctx.Done():
			panic(state.Global.Ctx.Err())
		default:
		}
	}

	if state.earlyFunctionDeclarationsPosition >= 0 && node.Base().Span.Start >= state.earlyFunctionDeclarationsPosition {
		state.earlyFunctionDeclarationsPosition = -1 //Prevent infinite recursion.

		//Declare functions that can be called before their definition statement.

		decls := state.earlyFunctionDeclarations

		for _, decl := range decls {
			_, err := TreeWalkEval(decl, state)
			if err != nil {
				return nil, fmt.Errorf("failed to declare function %s: %w", decl.Name.(*parse.IdentifierLiteral).Name, err)
			}
		}
	}

	switch n := node.(type) {
	case *parse.IdentifierLiteral:
		v, ok := state.Global.Globals.CheckedGet(n.Name)
		if !ok {
			v, ok = state.CurrentLocalScope()[n.Name]
		}

		if !ok {
			return nil, errors.New("variable " + n.Name + " is not declared")
		}
		return v, nil
	case *parse.IdentifierMemberExpression:
		v, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		if state.HasGlobal(n.Left.Name) {
			err = state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permbase.Use, Name: n.Left.Name})
			if err != nil {
				return nil, err
			}
		}

		for i, propNameIdent := range n.PropertyNames {
			var propContainer symbolic.Value
			if i == 0 {
				propContainer, _ = state.Global.SymbolicData.GetMostSpecificNodeValue(n.Left)
			} else {
				propContainer, _ = state.Global.SymbolicData.GetMostSpecificNodeValue(n.PropertyNames[i-1])
			}

			structPtr, ok := v.(*Struct)
			if ok {
				symbType := propContainer.(*symbolic.Pointer).Type()
				concreteType := state.getConcreteType(symbType).(*PointerType)
				retrievalInfo := concreteType.StructFieldRetrieval(propNameIdent.Name)

				helper := structHelperFromPtr(structPtr, int(concreteType.ValueSize()))
				v = helper.GetValue(retrievalInfo)
			} else {
				v = v.(IProps).Prop(state.Global.Ctx, propNameIdent.Name)
			}
		}
		return v, nil
	case *parse.OptionExpression:
		value, err := TreeWalkEval(n.Value, state)
		if err != nil {
			return nil, err
		}
		return Option{Name: n.Name, Value: value}, nil
	case *parse.AbsolutePathExpression, *parse.RelativePathExpression:

		var slices []parse.Node

		switch pexpr := n.(type) {
		case *parse.AbsolutePathExpression:
			slices = pexpr.Slices
		case *parse.RelativePathExpression:
			slices = pexpr.Slices
		}

		var args []Value
		var isStaticPathSliceList = make([]bool, len(slices))

		for i, node := range slices {
			_, isStaticPathSlice := node.(*parse.PathSlice)
			isStaticPathSliceList[i] = isStaticPathSlice
			pathSlice, err := TreeWalkEval(node, state)
			if err != nil {
				return nil, err
			}
			args = append(args, pathSlice)
		}

		return NewPath(args, isStaticPathSliceList)
	case *parse.PathPatternExpression:
		var args []Value
		var isStaticPathSliceList = make([]bool, len(n.Slices))

		for i, node := range n.Slices {
			_, isStaticPathSlice := node.(*parse.PathPatternSlice)
			isStaticPathSliceList[i] = isStaticPathSlice
			pathSlice, err := TreeWalkEval(node, state)
			if err != nil {
				return nil, err
			}
			args = append(args, pathSlice)
		}

		return NewPathPattern(args, isStaticPathSliceList)
	case *parse.URLExpression:
		host, err := TreeWalkEval(n.HostPart, state)
		if err != nil {
			return nil, err
		}

		//path evaluation

		var pathSlices []Value
		var isStaticPathSliceList = make([]bool, len(n.Path))

		//path evaluation

		for i, node := range n.Path {
			_, isStaticPathSliceList[i] = node.(*parse.PathSlice)

			pathSlice, err := TreeWalkEval(node, state)
			if err != nil {
				return nil, err
			}
			pathSlices = append(pathSlices, pathSlice)
		}

		//query evaluation

		var queryParamNames []Value
		var queryValues []Value

		queryBuff := bytes.NewBufferString("")
		if len(n.QueryParams) != 0 {
			queryBuff.WriteRune('?')
		}

		for _, p := range n.QueryParams {
			queryValue := String("")
			param := p.(*parse.URLQueryParameter)
			queryParamNames = append(queryParamNames, String(param.Name))

			for _, slice := range param.Value {
				val, err := TreeWalkEval(slice, state)
				if err != nil {
					return nil, err
				}
				stringified, err := stringifyQueryParamValue(val)
				if err != nil {
					return nil, err
				}
				queryValue += String(stringified)
			}
			queryValues = append(queryValues, queryValue)
		}

		return NewURL(host, pathSlices, isStaticPathSliceList, queryParamNames, queryValues)
	case *parse.HostExpression:
		hostnamePort, err := TreeWalkEval(n.Host, state)
		if err != nil {
			return nil, err
		}
		return NewHost(hostnamePort.(StringLike), n.Scheme.Name)
	case *parse.Variable:

		if val, ok := state.Global.Globals.CheckedGet(n.Name); ok {
			err := state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permbase.Read, Name: n.Name})
			if err != nil {
				return nil, err
			}
			return val, nil
		}

		v, ok := state.CurrentLocalScope()[n.Name]

		if !ok {
			return nil, errors.New("variable " + n.Name + " is not declared")
		}
		return v, nil
	case *parse.ReturnStatement:
		if n.Expr == nil {
			state.returnValue = Nil
			return nil, nil
		}

		value, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		state.returnValue = value
		return nil, nil
	case *parse.CoyieldStatement:
		if n.Expr == nil {
			state.returnValue = Nil
			return nil, nil
		}

		value, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		if state.Global.LThread == nil {
			panic(errors.New("failed to yield: no associated lthread"))
		}
		state.Global.LThread.yield(state.Global.Ctx, value)
		return nil, nil
	case *parse.BreakStatement:
		state.iterationChange = BreakIteration
		return nil, nil
	case *parse.ContinueStatement:
		state.iterationChange = ContinueIteration
		return nil, nil
	case *parse.YieldStatement:
		if n.Expr == nil {
			state.yieldedValue = Nil
			state.iterationChange = YieldItem
			return nil, nil
		}

		value, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		state.yieldedValue = value
		state.iterationChange = YieldItem

		return nil, nil
	case *parse.PruneStatement:
		state.iterationChange = PruneWalk
		return nil, nil
	case *parse.CallExpression:

		var (
			callee Value
			self   Value
		)

		//we first get the callee
		switch c := n.Callee.(type) {
		case *parse.IdentifierLiteral:
			err := state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permbase.Use, Name: c.Name})
			if err != nil {
				return nil, err
			}
			callee, err = TreeWalkEval(c, state)
			if err != nil {
				return nil, err
			}
		case *parse.IdentifierMemberExpression:
			v, err := TreeWalkEval(c.Left, state)
			if err != nil {
				return nil, err
			}

			if state.HasGlobal(c.Left.Name) {
				err = state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permbase.Use, Name: c.Left.Name})
				if err != nil {
					return nil, err
				}
			}

			for _, idents := range c.PropertyNames {
				if obj, ok := v.(*Object); ok {
					self = obj
				} else {
					self = nil
				}
				v = v.(IProps).Prop(state.Global.Ctx, idents.Name)
			}
			callee = v
		case *parse.Variable:
			callee, err = TreeWalkEval(n.Callee, state)
			if err != nil {
				return nil, err
			}
		case *parse.MemberExpression:
			var left Value

			innerMembExpr, ok := c.Left.(*parse.MemberExpression)
			if ok {
				propName := innerMembExpr.PropertyName.Name
				inner, err := TreeWalkEval(innerMembExpr.Left, state)
				if err != nil {
					return nil, err
				}

				iprops := inner.(IProps)
				left = iprops.Prop(state.Global.Ctx, propName)
			} else {
				left, err = TreeWalkEval(c.Left, state)
				if err != nil {
					return nil, err
				}
			}

			//we don't update self (self == nil)

			callee = left.(IProps).Prop(state.Global.Ctx, c.PropertyName.Name)
		case *parse.DoubleColonExpression:
			elementName := c.Element.Name
			extendedValue, err := TreeWalkEval(c.Left, state)
			if err != nil {
				return nil, err
			}

			_, ok := state.Global.SymbolicData.GetURLReferencedEntity(c)
			if ok {
				return nil, errors.New(symbolic.DIRECTLY_CALLING_METHOD_OF_URL_REF_ENTITY_NOT_ALLOWED)
			}

			self = extendedValue

			symbolicExtension, ok := state.Global.SymbolicData.GetUsedTypeExtension(c)
			if !ok {
				panic(ErrUnreachable)
			}

			extension := state.Global.Ctx.GetTypeExtension(symbolicExtension.Id)
			if extension == nil {
				panic(ErrUnreachable)
			}

			for _, propExpr := range extension.propertyExpressions {
				if propExpr.name != elementName {
					continue
				}
				if propExpr.method != nil {
					callee = propExpr.method
				} else {
					panic(parse.ErrUnreachable)
				}

			}

			if callee == nil {
				panic(parse.ErrUnreachable)
			}
		default:
			return nil, fmt.Errorf("cannot call a(n) %T", c)
		}

		if callee == nil {
			return nil, fmt.Errorf("cannot call nil %#v", n.Callee)
		}

		return TreeWalkCallFunc(TreeWalkCall{
			callee:        callee,
			callNode:      n,
			self:          self,
			state:         state,
			arguments:     n.Arguments,
			must:          n.Must,
			cmdLineSyntax: n.CommandLikeSyntax,
		})
	case *parse.PatternCallExpression:
		callee, err := TreeWalkEval(n.Callee, state)
		if err != nil {
			return nil, err
		}

		args := make([]Serializable, len(n.Arguments))

		for i, argNode := range n.Arguments {
			arg, err := TreeWalkEval(argNode, state)
			if err != nil {
				return nil, err
			}
			args[i] = arg.(Serializable)
		}

		return callee.(Pattern).Call(state.Global.Ctx, args)
	case *parse.PipelineStatement, *parse.PipelineExpression:
		var stages []*parse.PipelineStage

		switch e := n.(type) {
		case *parse.PipelineStatement:
			stages = e.Stages
		case *parse.PipelineExpression:
			stages = e.Stages
		}

		scope := state.CurrentLocalScope()
		if savedAnonymousValue, hasValue := scope[""]; hasValue {
			defer func() {
				scope[""] = savedAnonymousValue
			}()
		}

		var res Value

		for _, stage := range stages {
			res, err = TreeWalkEval(stage.Expr, state)
			if err != nil {
				return nil, err
			}
			scope[""] = res
		}

		//unlike the bytecode interpreter we return the value even for pipe statement
		//it's useful for the shell
		return res, nil
	case *parse.LocalVariableDeclarations:
		currentScope := state.CurrentLocalScope()

		for _, decl := range n.Declarations {
			right, err := TreeWalkEval(decl.Right, state)
			if err != nil {
				return nil, err
			}

			switch left := decl.Left.(type) {
			case *parse.IdentifierLiteral:
				name := left.Name
				currentScope[name] = right
			case *parse.ObjectDestructuration:
				for _, prop := range left.Properties {
					validProp := prop.(*parse.ObjectDestructurationProperty)

					propName := validProp.PropertyName.Name
					name := validProp.NameNode().Name
					iprops := right.(IProps)

					var varValue Value

					if validProp.Nillable {
						if slices.Contains(iprops.PropertyNames(state.Global.Ctx), propName) {
							//TODO: make thread safe (Time-of-check / time-of-use)
							varValue = iprops.Prop(state.Global.Ctx, propName)
						} else {
							varValue = Nil
						}
					} else {
						varValue = iprops.Prop(state.Global.Ctx, propName)
					}
					currentScope[name] = varValue
				}
			default:
				panic(ErrUnreachable)
			}
		}
		return nil, nil
	case *parse.GlobalVariableDeclarations:
		for _, decl := range n.Declarations {

			right, err := TreeWalkEval(decl.Right, state)
			if err != nil {
				return nil, err
			}

			switch left := decl.Left.(type) {
			case *parse.IdentifierLiteral:
				name := left.Name
				err := precheckGlobalVariableDeclaration(name, state)
				if err != nil {
					return nil, err
				}
				state.SetGlobal(name, right, GlobalVar)
			case *parse.ObjectDestructuration:
				for _, prop := range left.Properties {
					validProp := prop.(*parse.ObjectDestructurationProperty)

					propName := validProp.PropertyName.Name
					name := validProp.NameNode().Name
					iprops := right.(IProps)

					var varValue Value

					if validProp.Nillable {
						if slices.Contains(iprops.PropertyNames(state.Global.Ctx), propName) {
							//TODO: make thread safe (Time-of-check / time-of-use)
							varValue = iprops.Prop(state.Global.Ctx, propName)
						} else {
							varValue = Nil
						}
					} else {
						varValue = iprops.Prop(state.Global.Ctx, propName)
					}
					err := precheckGlobalVariableDeclaration(name, state)
					if err != nil {
						return nil, err
					}
					state.SetGlobal(name, varValue, GlobalVar)
				}
			default:
				panic(ErrUnreachable)
			}

		}
		return nil, nil
	case *parse.Assignment:

		handleAssignmentOperation := func(left func() Value, right Value) (Value, error) {
			switch n.Operator {
			case parse.PlusAssign:
				return intAdd(left().(Int), right.(Int))
			case parse.MinusAssign:
				return intSub(left().(Int), right.(Int))
			case parse.MulAssign:
				return intMul(left().(Int), right.(Int))
			case parse.DivAssign:
				return intDiv(left().(Int), right.(Int))
			}

			return right, nil
		}

		switch lhs := n.Left.(type) {
		case *parse.Variable:
			name := lhs.Name

			if state.HasGlobal(name) {
				return nil, errors.New("attempt to assign a global variable or constant")
			}

			currentLocalScope := state.CurrentLocalScope()

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			right, err = handleAssignmentOperation(utils.Ret(currentLocalScope[name]), right)
			if err != nil {
				return nil, err
			}

			currentLocalScope[name] = right
		case *parse.IdentifierLiteral:
			name := lhs.Name

			if state.HasGlobal(name) {
				return nil, errors.New("attempt to assign a global variable or constant")
			}

			currentLocalScope := state.CurrentLocalScope()

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			right, err = handleAssignmentOperation(utils.Ret(currentLocalScope[name]), right)
			if err != nil {
				return nil, err
			}

			currentLocalScope[name] = right
		case *parse.MemberExpression:
			left, err := TreeWalkEval(lhs.Left, state)
			if err != nil {
				return nil, err
			}

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}
			key := lhs.PropertyName.Name

			var getLeft func() Value
			var storeValue func(v Value) error

			structPtr, ok := left.(*Struct)
			if ok {
				val := utils.MustGet(state.Global.SymbolicData.GetMostSpecificNodeValue(lhs.Left))
				symbType := val.(*symbolic.Pointer).Type()
				concreteType := state.getConcreteType(symbType).(*PointerType)
				retrievalInfo := concreteType.StructFieldRetrieval(key)

				helper := structHelperFromPtr(structPtr, int(concreteType.ValueSize()))

				getLeft = func() Value {
					return helper.GetValue(retrievalInfo)
				}
				storeValue = func(v Value) error {
					helper.SetValue(retrievalInfo, v)
					return nil
				}
			} else {
				getLeft = func() Value {
					return left.(IProps).Prop(state.Global.Ctx, key)
				}
				storeValue = func(v Value) error {
					return left.(IProps).SetProp(state.Global.Ctx, key, v)
				}
			}

			right, err = handleAssignmentOperation(getLeft, right)
			if err != nil {
				return nil, err
			}

			return nil, storeValue(right)
		case *parse.IdentifierMemberExpression:
			left, err := TreeWalkEval(lhs.Left, state)
			if err != nil {
				return nil, err
			}

			for i, propNameIdent := range lhs.PropertyNames[:len(lhs.PropertyNames)-1] {
				var symbPropContainer symbolic.Value
				if i == 0 {
					symbPropContainer, _ = state.Global.SymbolicData.GetMostSpecificNodeValue(lhs.Left)
				} else {
					symbPropContainer, _ = state.Global.SymbolicData.GetMostSpecificNodeValue(lhs.PropertyNames[i-1])
				}

				structPtr, ok := left.(*Struct)
				if ok {
					symbType := symbPropContainer.(*symbolic.Pointer).Type()
					concreteType := state.getConcreteType(symbType).(*PointerType)
					retrievalInfo := concreteType.StructFieldRetrieval(propNameIdent.Name)

					helper := structHelperFromPtr(structPtr, int(concreteType.ValueSize()))
					left = helper.GetValue(retrievalInfo)
				} else {
					left = left.(IProps).Prop(state.Global.Ctx, propNameIdent.Name)
				}
			}

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			lastPropName := lhs.PropertyNames[len(lhs.PropertyNames)-1].Name

			var getLeft func() Value
			var storeValue func(v Value) error

			structPtr, ok := left.(*Struct)
			if ok {
				propContainerNode := lhs.Left
				if len(lhs.PropertyNames) > 1 {
					propContainerNode = lhs.PropertyNames[len(lhs.PropertyNames)-2]
				}

				symbPropContainer := utils.MustGet(state.Global.SymbolicData.GetMostSpecificNodeValue(propContainerNode))
				symbType := symbPropContainer.(*symbolic.Pointer).Type()
				concreteType := state.getConcreteType(symbType).(*PointerType)
				retrievalInfo := concreteType.StructFieldRetrieval(lastPropName)

				helper := structHelperFromPtr(structPtr, int(concreteType.ValueSize()))

				getLeft = func() Value {
					return helper.GetValue(retrievalInfo)
				}
				storeValue = func(v Value) error {
					helper.SetValue(retrievalInfo, v)
					return nil
				}
			} else {
				getLeft = func() Value {
					return left.(IProps).Prop(state.Global.Ctx, lastPropName)
				}
				storeValue = func(v Value) error {
					return left.(IProps).SetProp(state.Global.Ctx, lastPropName, v)
				}
			}

			right, err = handleAssignmentOperation(getLeft, right)
			if err != nil {
				return nil, err
			}

			return nil, storeValue(right)
		case *parse.IndexExpression:
			slice, err := TreeWalkEval(lhs.Indexed, state)
			if err != nil {
				return nil, err
			}

			index, err := TreeWalkEval(lhs.Index, state)
			if err != nil {
				return nil, err
			}

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			sequence := slice.(MutableSequence)
			i := int(index.(Int))
			getLeft := func() Value {
				return sequence.At(state.Global.Ctx, i)
			}

			right, err = handleAssignmentOperation(getLeft, right)
			if err != nil {
				return nil, err
			}

			sequence.set(state.Global.Ctx, i, right)
		case *parse.SliceExpression:
			slice, err := TreeWalkEval(lhs.Indexed, state)
			if err != nil {
				return nil, err
			}

			startIndex, err := TreeWalkEval(lhs.StartIndex, state)
			if err != nil {
				return nil, err
			}

			endIndex, err := TreeWalkEval(lhs.EndIndex, state)
			if err != nil {
				return nil, err
			}

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			if startIndex.(Int) >= endIndex.(Int) {
				return nil, fmt.Errorf("start index should be less than end index")
			}

			if s, ok := slice.(MutableSequence); ok {
				s.SetSlice(state.Global.Ctx, int(startIndex.(Int)), int(endIndex.(Int)), right.(Sequence))
			}

		default:
			return nil, fmt.Errorf("invalid assignment: left hand side is a(n) %T", n.Left)
		}

		return Nil, nil
	case *parse.MultiAssignment:
		right, err := TreeWalkEval(n.Right, state)

		if err != nil {
			return nil, err
		}

		list := right.(Sequence)
		scope := state.CurrentLocalScope()

		listLength := list.Len()
		valueReceivingVars := len(n.Variables)
		if n.Nillable {
			valueReceivingVars = min(listLength, len(n.Variables))
		}
		//for now we don't check the length + fast fail because we need to have the same behaviour
		//as the bytecode interpreter

		for i, var_ := range n.Variables[0:valueReceivingVars] {
			scope[var_.(*parse.IdentifierLiteral).Name] = list.At(state.Global.Ctx, i)
		}

		for _, var_ := range n.Variables[valueReceivingVars:] {
			scope[var_.(*parse.IdentifierLiteral).Name] = Nil
		}

		return Nil, nil
	case *parse.Chunk:
		manageLocalScope := !n.IsShellChunk && len(state.chunkStack) <= 1

		// If we are in the shell or in an included chunk we have to keep the top local scope.
		// We PushScope() and defer popScope() only if we are not in the shell / in included chunk.
		if manageLocalScope {
			state.LocalScopeStack = nil //we only keep the global scope
			state.PushScope()

			if state.debug != nil {
				chunk := state.Global.Module.MainChunk
				line, col := chunk.GetLineColumn(chunk.Node)

				state.frameInfo = append(state.frameInfo, StackFrameInfo{
					Node:        n,
					Name:        chunk.Name(),
					Chunk:       chunk,
					StartLine:   line,
					StartColumn: col,
					Id:          state.debug.shared.getNextStackFrameId(),

					StatementStartLine:   1,
					StatementStartColumn: 1,
				})

				defer func() {
					state.frameInfo = state.frameInfo[:len(state.frameInfo)-1]
				}()
			}
		}

		state.returnValue = nil
		state.yieldedValue = nil

		defer func() {
			state.returnValue = nil
			state.iterationChange = NoIterationChange
			if manageLocalScope {
				state.PopScope()
			}
		}()

		//Update information about early function declarations.

		earlyFunctionDeclarationsPosition := state.earlyFunctionDeclarationsPosition
		earlyFunctionDeclarations := state.earlyFunctionDeclarations

		defer func() {
			state.earlyFunctionDeclarationsPosition = earlyFunctionDeclarationsPosition
			state.earlyFunctionDeclarations = earlyFunctionDeclarations
		}()
		state.earlyFunctionDeclarationsPosition = -1
		state.earlyFunctionDeclarations = nil

		if staticCheckData := state.Global.StaticCheckData; staticCheckData != nil {
			earlyDeclarationsPosition, ok := staticCheckData.GetEarlyFunctionDeclarationsPosition(n)
			if ok {
				state.earlyFunctionDeclarationsPosition = earlyDeclarationsPosition
				declarations := slices.Clone(staticCheckData.GetFunctionsToDeclareEarly(n))
				state.earlyFunctionDeclarations = declarations
			}
		}

		//CONSTANTS
		if n.GlobalConstantDeclarations != nil {
			for _, decl := range n.GlobalConstantDeclarations.Declarations {

				value, err := TreeWalkEval(decl.Right, state)
				if err != nil {
					return nil, err
				}

				if !state.SetGlobal(decl.Ident().Name, value, GlobalConst) {
					return nil, fmt.Errorf("failed to set global '%s'", decl.Ident().Name)
				}
			}
		}

		//STATEMENTS

		if len(n.Statements) == 1 {
			stmt := n.Statements[0]
			if state.debug != nil {
				state.updateStackTrace(stmt)
				state.debug.beforeInstruction(stmt, state.frameInfo, nil)
			}

			res, err := TreeWalkEval(stmt, state)
			if err != nil {
				if state.debug != nil {
					state.updateStackTrace(stmt)
					state.debug.beforeInstruction(stmt, state.frameInfo, err)
				}
				return nil, err
			}
			if state.returnValue != nil {
				return state.returnValue, nil
			}

			return res, nil
		}

		for _, stmt := range n.Statements {
			if state.debug != nil {
				state.updateStackTrace(stmt)
				state.debug.beforeInstruction(stmt, state.frameInfo, nil)
			}

			_, err = TreeWalkEval(stmt, state)

			if err != nil {
				if state.debug != nil {
					state.updateStackTrace(stmt)
					state.debug.beforeInstruction(stmt, state.frameInfo, err)
				}
				return nil, err
			}
			if state.returnValue != nil {
				return state.returnValue, nil
			}
		}

		return Nil, nil
	case *parse.EmbeddedModule:
		return ValOf(n.ToChunk()), nil
	case *parse.Block:
	loop:
		for _, stmt := range n.Statements {
			if state.debug != nil {
				state.updateStackTrace(stmt)
				state.debug.beforeInstruction(stmt, state.frameInfo, nil)
			}

			_, err := TreeWalkEval(stmt, state)
			if err != nil {
				if state.debug != nil {
					state.updateStackTrace(stmt)
					state.debug.beforeInstruction(stmt, state.frameInfo, err)
				}
				return nil, err
			}

			if state.returnValue != nil {
				return Nil, nil
			}

			switch state.iterationChange {
			case BreakIteration, ContinueIteration, YieldItem, PruneWalk:
				break loop
			}
		}
		return Nil, nil
	case *parse.SynchronizedBlockStatement:
		var lockedValues []PotentiallySharable
		defer func() {
			for _, val := range utils.ReversedSlice(lockedValues) {
				val.SmartUnlock(state.Global)
			}
			var newLockedValues []PotentiallySharable

			// update list of locked values
		loop:
			for _, lockedVal := range state.Global.lockedValues {
				for _, unlockedVal := range lockedValues {
					if lockedVal == unlockedVal {
						continue loop
					}
				}
				newLockedValues = append(newLockedValues, lockedVal)
			}
			state.Global.lockedValues = newLockedValues
		}()

		for _, valueNode := range n.SynchronizedValues {
			val, err := TreeWalkEval(valueNode, state)
			if err != nil {
				return nil, err
			}

			if !val.IsMutable() {
				continue
			}

			potentiallySharable := val.(PotentiallySharable)
			if !utils.Ret0(potentiallySharable.IsSharable(state.Global)) {
				return nil, ErrCannotLockUnsharableValue
			}

			for _, locked := range state.Global.lockedValues {
				if potentiallySharable == locked {
					continue
				}
			}

			potentiallySharable.Share(state.Global)
			potentiallySharable.SmartLock(state.Global)

			// update list of locked values
			state.Global.lockedValues = append(state.Global.lockedValues, potentiallySharable)
			lockedValues = append(lockedValues, potentiallySharable)
		}

		return TreeWalkEval(n.Block, state)
	case *parse.PermissionDroppingStatement:
		permissionListing, err := EvaluatePermissionListingObjectNode(n.Object, PreinitArgs{
			RunningState: state,
			ParentState:  state.Global,
		})
		if err != nil {
			return nil, err
		}

		perms, err := getPermissionsFromListing(state.Global.Ctx, permissionListing, nil, nil, false)
		if err != nil {
			return nil, err
		}

		state.Global.Ctx.DropPermissions(perms)
		return Nil, nil
	case *parse.InclusionImportStatement:
		if state.Global.Module == nil {
			panic(fmt.Errorf("cannot evaluate inclusion import statement: global state's module is nil"))
		}
		chunk := state.Global.Module.InclusionStatementMap[n]
		state.pushImportedChunk(chunk.ParsedChunkSource, n)
		defer state.popImportedChunk()

		if state.debug != nil {
			frameCount := len(state.frameInfo)
			prevChunk := state.frameInfo[frameCount-1].Chunk
			prevName := state.frameInfo[frameCount-1].Name

			state.frameInfo[frameCount-1].Chunk = chunk.ParsedChunkSource
			state.frameInfo[frameCount-1].Name = chunk.Name()
			defer func() {
				state.frameInfo[frameCount-1].Chunk = prevChunk
				state.frameInfo[frameCount-1].Name = prevName
			}()
		}

		if state.Global.TestingState.IsTestingEnabled && !state.Global.TestingState.IsImportTestingEnabled && !state.forceDisableTesting {
			state.forceDisableTesting = true
			defer func() {
				state.forceDisableTesting = false
			}()
		}

		return TreeWalkEval(chunk.Node, state)
	case *parse.ImportStatement:
		varPerm := GlobalVarPermission{permbase.Create, n.Identifier.Name}
		if err := state.Global.Ctx.CheckHasPermission(varPerm); err != nil {
			return nil, fmt.Errorf("import: %s", err.Error())
		}

		src, err := TreeWalkEval(n.Source, state)
		if err != nil {
			return nil, err
		}

		configObj, err := TreeWalkEval(n.Configuration.(*parse.ObjectLiteral), state)
		if err != nil {
			return nil, err
		}

		config, err := buildImportConfig(configObj.(*Object), src.(ResourceName), state.Global)
		if err != nil {
			return nil, err
		}

		result, err := ImportWaitModule(config)
		if err != nil {
			return nil, err
		}

		state.SetGlobal(n.Identifier.Name, result, GlobalConst)
		return Nil, nil
	case *parse.SpawnExpression:
		var (
			group       *LThreadGroup
			globalsDesc Value
			permListing *Object
		)

		if n.Meta != nil {
			meta := map[string]Value{}
			if objLit, ok := n.Meta.(*parse.ObjectLiteral); ok {

				for _, property := range objLit.Properties {
					propertyName := property.Name() //okay since implicit-key properties are not allowed

					if propertyName == symbolic.LTHREAD_META_GLOBALS_SECTION {
						globalsObjectLit, ok := property.Value.(*parse.ObjectLiteral)
						//handle description separately if it's an object literal because non-serializable value are not accepted.
						if ok {
							globals := &ModuleArgs{}
							var keys []string
							var types []Pattern

							for _, prop := range globalsObjectLit.Properties {
								globalName := prop.Name() //okay since implicit-key properties are not allowed
								globalVal, err := TreeWalkEval(prop.Value, state)
								if err != nil {
									return nil, err
								}

								keys = append(keys, globalName)
								globals.values = append(globals.values, globalVal)
								types = append(types, ANYVAL_PATTERN)
							}
							globals.pattern = NewModuleParamsPattern(keys, types)
							meta[propertyName] = globals
							continue
						}
					}

					propertyVal, err := TreeWalkEval(property.Value, state)
					if err != nil {
						return nil, err
					}
					meta[propertyName] = propertyVal
				}
			} else {
				return nil, errors.New("meta should be an object")
			}

			group, globalsDesc, permListing, err = readLThreadMeta(meta, state.Global.Ctx)
			if err != nil {
				return nil, err
			}
		}

		var ctx *Context
		var chunk *parse.Chunk
		var startConstants []string
		actualGlobals := make(map[string]Value)

		state.Global.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
			if isStartConstant {
				actualGlobals[name] = v
				startConstants = append(startConstants, name)
			}
			return nil
		})

		switch g := globalsDesc.(type) {
		case *ModuleArgs:
			for k, v := range g.ValueMap() {
				actualGlobals[k] = v
			}
		case KeyList:
			for _, name := range g {
				actualGlobals[name] = state.Global.Globals.Get(name)
			}
		case NilT:
			break
		case nil:
		default:
			return nil, fmt.Errorf("spawn expression: globals: only objects and keylists are supported, not %T", g)
		}

		if n.Module.SingleCallExpr {
			chunk = &parse.Chunk{
				NodeBase:   n.Module.NodeBase,
				Statements: n.Module.Statements,
			}

			calleeNode := n.Module.Statements[0].(*parse.CallExpression).Callee
			var callee Value

			switch calleeNode := calleeNode.(type) {
			case *parse.IdentifierLiteral:
				callee, _ = state.Get(calleeNode.Name)
				actualGlobals[calleeNode.Name] = callee
			case *parse.IdentifierMemberExpression:
				namespace, _ := state.Get(calleeNode.Left.Name)
				actualGlobals[calleeNode.Left.Name] = namespace
			default:
				panic(ErrUnreachable)
			}
		} else {
			expr, err := TreeWalkEval(n.Module, state)
			if err != nil {
				return nil, err
			}

			chunk = expr.(AstNode).Node.(*parse.Chunk)
		}

		var grantedPerms []Permission

		if permListing != nil {
			grantedPerms, err = getPermissionsFromListing(state.Global.Ctx, permListing, nil, nil, true)
			if err != nil {
				return nil, err
			}

			for _, perm := range grantedPerms {
				if err := state.Global.Ctx.CheckHasPermission(perm); err != nil {
					return nil, fmt.Errorf("spawn: cannot allow permission: %w", err)
				}
			}

			ctx = NewContext(ContextConfig{
				Permissions:          grantedPerms,
				ForbiddenPermissions: state.Global.Ctx.forbiddenPermissions,
				ParentContext:        state.Global.Ctx,
			})
		} else {
			removedPerms := IMPLICITLY_REMOVED_ROUTINE_PERMS
			remainingPerms := RemovePerms(state.Global.Ctx.GetGrantedPermissions(), IMPLICITLY_REMOVED_ROUTINE_PERMS)

			ctx = NewContext(ContextConfig{
				ParentContext:        state.Global.Ctx,
				Permissions:          remainingPerms,
				ForbiddenPermissions: removedPerms,
			})
		}

		parsedChunk := &parse.ParsedChunkSource{
			Node:   chunk,
			Source: state.currentChunk().Source,
		}

		routineMod := WrapLowerModule(&inoxmod.Module{
			MainChunk:    parsedChunk,
			TopLevelNode: n.Module,
			Kind:         UserLThreadModule,
		})

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state.Global,
			Globals:      GlobalVariablesFromMap(actualGlobals, startConstants),
			Module:       routineMod,
			LthreadCtx:   ctx,
		})

		if err != nil {
			return nil, err
		}

		if group != nil {
			group.Add(lthread)
		}

		return lthread, nil
	case *parse.MappingExpression:
		return NewMapping(n, state.Global)
	case *parse.ComputeExpression:
		key, err := TreeWalkEval(n.Arg, state)
		if err != nil {
			return nil, err
		}
		return state.entryComputeFn(key)
	case *parse.TreedataLiteral:
		rootVal, err := TreeWalkEval(n.Root, state)
		if err != nil {
			return nil, err
		}

		var children []TreedataHiearchyEntry

		for _, entry := range n.Children {
			child, err := TreeWalkEval(entry, state)
			if err != nil {
				return nil, err
			}

			children = append(children, child.(TreedataHiearchyEntry))
		}

		treedata := &Treedata{
			Root:            rootVal.(Serializable),
			HiearchyEntries: children,
		}

		return treedata, nil
	case *parse.TreedataEntry:
		nodeVal, err := TreeWalkEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		var children []TreedataHiearchyEntry

		for _, entry := range n.Children {
			child, err := TreeWalkEval(entry, state)
			if err != nil {
				return nil, err
			}

			children = append(children, child.(TreedataHiearchyEntry))
		}

		return TreedataHiearchyEntry{
			Value:    nodeVal.(Serializable),
			Children: children,
		}, nil
	case *parse.TreedataPair:
		firstVal, err := TreeWalkEval(n.Key, state)
		if err != nil {
			return nil, err
		}
		secondVal, err := TreeWalkEval(n.Value, state)
		if err != nil {
			return nil, err
		}
		return NewOrderedPair(firstVal.(Serializable), secondVal.(Serializable)), nil
	case *parse.ObjectLiteral:
		finalObj := &Object{}

		//created from no key properties
		var elements []Serializable
		elemListIndex := 0 //index of ""

		for _, p := range n.Properties {
			v, err := TreeWalkEval(p.Value, state)
			if err != nil {
				return nil, err
			}

			var key string

			switch n := p.Key.(type) {
			case *parse.DoubleQuotedStringLiteral:
				key = n.Value
			case *parse.IdentifierLiteral:
				key = n.Name
			case nil: //no key
				elements = append(elements, v.(Serializable))
				if !slices.Contains(finalObj.keys, inoxconsts.IMPLICIT_PROP_NAME) {
					elemListIndex = len(finalObj.values)
					finalObj.keys = append(finalObj.keys, inoxconsts.IMPLICIT_PROP_NAME)
					finalObj.values = append(finalObj.values, nil) //reserve location
				}
				continue
			default:
				return nil, fmt.Errorf("invalid key type %T", n)
			}

			finalObj.keys = append(finalObj.keys, key)
			finalObj.values = append(finalObj.values, v.(Serializable))
		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := TreeWalkEval(el.Expr, state)
			if err != nil {
				return nil, err
			}

			object := evaluatedElement.(*Object)

			for _, key := range el.Expr.(*parse.ExtractionExpression).Keys.Keys {
				name := key.(*parse.IdentifierLiteral).Name
				finalObj.keys = append(finalObj.keys, name)
				finalObj.values = append(finalObj.values, object.Prop(state.Global.Ctx, name).(Serializable))
			}
		}

		if len(elements) > 0 {
			list := NewWrappedValueList(elements...)
			finalObj.values[elemListIndex] = list
		}

		finalObj.sortProps()
		// add handlers before because jobs can mutate the object
		if err := finalObj.addMessageHandlers(state.Global.Ctx); err != nil {
			return nil, err
		}
		if err := finalObj.instantiateLifetimeJobs(state.Global.Ctx); err != nil {
			return nil, err
		}

		initializeMetaproperties(finalObj, n.MetaProperties)
		return finalObj, nil
	case *parse.RecordLiteral:
		finalRecord := &Record{}

		//created from no key properties
		var elements []Serializable
		elemListIndex := 0 //index of ""

		for _, p := range n.Properties {
			v, err := TreeWalkEval(p.Value, state)
			if err != nil {
				return nil, err
			}

			var key string

			switch n := p.Key.(type) {
			case *parse.DoubleQuotedStringLiteral:
				key = n.Value
			case *parse.IdentifierLiteral:
				key = n.Name
			case nil: //no key
				elements = append(elements, v.(Serializable))
				if !slices.Contains(finalRecord.keys, inoxconsts.IMPLICIT_PROP_NAME) {
					elemListIndex = len(finalRecord.values)
					finalRecord.keys = append(finalRecord.keys, inoxconsts.IMPLICIT_PROP_NAME)
					finalRecord.values = append(finalRecord.values, nil) //reserve location
				}
				continue
			default:
				return nil, fmt.Errorf("invalid key type %T", n)
			}

			finalRecord.keys = append(finalRecord.keys, key)
			finalRecord.values = append(finalRecord.values, v.(Serializable))
		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := TreeWalkEval(el.Expr, state)
			if err != nil {
				return nil, err
			}

			object := evaluatedElement.(*Object)

			for _, key := range el.Expr.(*parse.ExtractionExpression).Keys.Keys {
				name := key.(*parse.IdentifierLiteral).Name
				finalRecord.keys = append(finalRecord.keys, name)
				finalRecord.values = append(finalRecord.values, object.Prop(state.Global.Ctx, name).(Serializable))
			}
		}

		if len(elements) > 0 {
			tuple := NewTuple(elements)
			finalRecord.values[elemListIndex] = tuple
		}

		finalRecord.sortProps()

		return finalRecord, nil
	case *parse.ListLiteral:
		var elements []Serializable

		if len(n.Elements) > 0 {
			elements = make([]Serializable, 0, len(n.Elements))
		}

		for _, en := range n.Elements {

			if spreadElem, ok := en.(*parse.ElementSpreadElement); ok {
				e, err := TreeWalkEval(spreadElem.Expr, state)
				if err != nil {
					return nil, err
				}
				elements = append(elements, e.(*List).GetOrBuildElements(state.Global.Ctx)...)

			} else {
				e, err := TreeWalkEval(en, state)
				if err != nil {
					return nil, err
				}
				elements = append(elements, e.(Serializable))
			}
		}

		var elemType Pattern

		if n.TypeAnnotation != nil {
			v, err := TreeWalkEval(n.TypeAnnotation, state)
			if err != nil {
				return nil, err
			}
			elemType = v.(Pattern)
		}

		return createBestSuitedList(state.Global.Ctx, elements, elemType), nil
	case *parse.TupleLiteral:
		tuple := &Tuple{
			elements: make([]Serializable, 0),
		}

		for _, en := range n.Elements {

			if spreadElem, ok := en.(*parse.ElementSpreadElement); ok {
				e, err := TreeWalkEval(spreadElem.Expr, state)
				if err != nil {
					return nil, err
				}
				tuple.elements = append(tuple.elements, e.(*Tuple).elements...)

			} else {
				e, err := TreeWalkEval(en, state)
				if err != nil {
					return nil, err
				}
				tuple.elements = append(tuple.elements, e.(Serializable))
			}
		}

		return tuple, nil
	case *parse.DictionaryLiteral:
		dict := Dictionary{
			entries: map[string]Serializable{},
			keys:    map[string]Serializable{},
		}

		for _, entry := range n.Entries {
			k, err := TreeWalkEval(entry.Key, state)
			if err != nil {
				return nil, err
			}

			v, err := TreeWalkEval(entry.Value, state)
			if err != nil {
				return nil, err
			}

			keyRepr := dict.getKeyRepr(state.Global.Ctx, k.(Serializable))
			dict.entries[keyRepr] = v.(Serializable)
			dict.keys[keyRepr] = k.(Serializable)
		}

		return &dict, nil
	case *parse.IfStatement:
		test, err := TreeWalkEval(n.Test, state)
		if err != nil {
			return nil, err
		}

		if boolean, ok := test.(Bool); ok {
			var err error
			if boolean {
				_, err = TreeWalkEval(n.Consequent, state)
			} else if n.Alternate != nil {
				_, err = TreeWalkEval(n.Alternate, state)
			}

			if err != nil {
				return nil, err
			}

			return Nil, nil
		} else {
			return nil, fmt.Errorf("if statement's test is not a boolean but a %T", test)
		}
	case *parse.IfExpression:
		test, err := TreeWalkEval(n.Test, state)
		if err != nil {
			return nil, err
		}

		var val Value

		if boolean, ok := test.(Bool); ok {
			var err error
			if boolean {
				val, err = TreeWalkEval(n.Consequent, state)
			} else if n.Alternate != nil {
				val, err = TreeWalkEval(n.Alternate, state)
			} else {
				val = Nil
			}

			if err != nil {
				return nil, err
			}

			return val, nil
		} else {
			return nil, fmt.Errorf("if statement expression's test is not a boolean but a %T", test)
		}
	case *parse.ForStatement:
		err := evalForStatement(n, state)
		return nil, err
	case *parse.ForExpression:
		return evalForExpression(n, state)
	case *parse.WalkStatement:
		err := evalWalkStatement(n, state)
		return nil, err
	case *parse.SwitchStatement:
		err := evalSwitchStatement(n, state)
		return nil, err
	case *parse.SwitchExpression:
		return evalSwitchExpression(n, state)
	case *parse.MatchStatement:
		err := evalMatchStatement(n, state)
		return nil, err
	case *parse.MatchExpression:
		return evalMatchExpression(n, state)
	case *parse.UnaryExpression:

		operand, err := TreeWalkEval(n.Operand, state)
		if err != nil {
			return nil, err
		}
		switch n.Operator {
		case parse.NumberNegate:
			if i, ok := operand.(Int); ok {
				if i == -i && i != 0 {
					return nil, ErrNegationWithOverflow
				}
				return -i, nil
			}
			return -operand.(Float), nil
		case parse.BoolNegate:
			return !operand.(Bool), nil
		default:
			return nil, fmt.Errorf("invalid unary operator %d", n.Operator)
		}
	case *parse.BinaryExpression:
		return evalBinaryExpression(n, state)
	case *parse.UpperBoundRangeExpression:
		upperBound, err := TreeWalkEval(n.UpperBound, state)
		if err != nil {
			return nil, err
		}

		switch v := upperBound.(type) {
		case Int:
			return IntRange{
				unknownStart: true,
				end:          int64(v),
				step:         1,
			}, nil
		case Float:
			return FloatRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          float64(v),
			}, nil
		default:
			return QuantityRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          v.(Serializable),
			}, nil
		}
	case *parse.IntegerRangeLiteral:
		upperBound := int64(math.MaxInt64)

		if n.UpperBound != nil {
			upperBound = n.UpperBound.(*parse.IntLiteral).Value
		}

		return IntRange{
			unknownStart: false,
			start:        n.LowerBound.Value,
			end:          upperBound,
			step:         1,
		}, nil
	case *parse.FloatRangeLiteral:
		upperBound := float64(math.MaxFloat64)

		if n.UpperBound != nil {
			upperBound = n.UpperBound.(*parse.FloatLiteral).Value
		}

		return FloatRange{
			unknownStart: false,
			inclusiveEnd: true,
			start:        n.LowerBound.Value,
			end:          upperBound,
		}, nil
	case *parse.QuantityRangeLiteral:
		return mustEvalQuantityRange(n), nil
	case *parse.RuneRangeExpression:
		return RuneRange{
			Start: n.Lower.Value,
			End:   n.Upper.Value,
		}, nil
	case *parse.FunctionExpression:
		localScope := state.CurrentLocalScope()
		capturedLocals := map[string]Value{}
		for _, e := range n.CaptureList {
			name := e.(*parse.IdentifierLiteral).Name
			shared, err := ShareOrClone(localScope[name], state.Global)
			if err != nil {
				return nil, fmt.Errorf("failed to share captured local value: %w", err)
			}
			capturedLocals[name] = shared
		}

		var symbolicInoxFunc *symbolic.InoxFunction
		{
			value, ok := state.Global.SymbolicData.GetMostSpecificNodeValue(node)
			if ok {
				symbolicInoxFunc, ok = value.(*symbolic.InoxFunction)
				if !ok {
					return nil, fmt.Errorf("invalid type for symbolic value of function expression: %T", value)
				}
			}
		}

		var staticData *staticcheck.FunctionData
		var capturedGlobals []capturedGlobal
		if state.Global.StaticCheckData != nil {
			staticData = state.Global.StaticCheckData.GetFnData(n)
		}

		return &InoxFunction{
			Node:                   n,
			Chunk:                  state.currentChunk(),
			treeWalkCapturedLocals: capturedLocals,
			symbolicValue:          symbolicInoxFunc,
			staticData:             staticData,
			capturedGlobals:        capturedGlobals,
		}, nil
	case *parse.FunctionDeclaration:
		funcName := n.Name.(*parse.IdentifierLiteral).Name
		if val, ok := state.GetGlobal(funcName); ok { //Function pre-eclared before this statement or re-declaration in shell.

			if !state.currentChunk().Node.IsShellChunk {
				//Function pre-declared before this statement
				return nil, nil
			}

			fn, ok := val.(*InoxFunction)
			if !ok {
				panic(ErrUnreachable)
			}

			if fn.Chunk == state.currentChunk() {
				//Function pre-declared before this statement
				return nil, nil
			}

			//Re-declaration in shell.
		}

		localScope := state.CurrentLocalScope()
		capturedLocals := map[string]Value{}

		for _, e := range n.Function.CaptureList {
			name := e.(*parse.IdentifierLiteral).Name
			capturedLocals[name] = localScope[name]
		}

		val, err := TreeWalkEval(n.Function, state)
		if err != nil {
			return nil, err
		}

		state.SetGlobal(funcName, val, GlobalConst)
		return Nil, nil

	case *parse.FunctionPatternExpression:
		symbolicData, ok := state.Global.SymbolicData.GetMostSpecificNodeValue(node)
		var symbFnPattern *symbolic.FunctionPattern
		if ok {
			symbFnPattern, ok = symbolicData.(*symbolic.FunctionPattern)
			if !ok {
				return nil, fmt.Errorf("invalide type for symboli value of function pattern expression: %T", symbolicData)
			}
		}

		return &FunctionPattern{
			node:          n,
			nodeChunk:     state.currentChunk().Node,
			symbolicValue: symbFnPattern,
		}, nil
	case *parse.PatternConversionExpression:
		v, err := TreeWalkEval(n.Value, state)
		if err != nil {
			return nil, err
		}
		if patt, ok := v.(Pattern); ok {
			return patt, nil
		}
		return evalPatternNode(n.Value, state)
	case *parse.QuotedExpression:
		return AstNode{
			Node:   n.Expression,
			Chunk_: state.currentChunk(),
		}, nil
	case *parse.SelfExpression:
		return state.self, nil
	case *parse.MemberExpression:
		left, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		structPtr, ok := left.(*Struct)
		if ok {
			val := utils.MustGet(state.Global.SymbolicData.GetMostSpecificNodeValue(n.Left))
			symbType := val.(*symbolic.Pointer).Type()
			concreteType := state.getConcreteType(symbType).(*PointerType)
			retrievalInfo := concreteType.StructFieldRetrieval(n.PropertyName.Name)

			helper := structHelperFromPtr(structPtr, int(concreteType.ValueSize()))
			return helper.GetValue(retrievalInfo), nil
		}

		iprops := left.(IProps)
		propName := n.PropertyName.Name

		if n.Optional && !utils.SliceContains(iprops.PropertyNames(state.Global.Ctx), propName) {
			return Nil, nil
		}

		return iprops.Prop(state.Global.Ctx, propName), nil
	case *parse.DoubleColonExpression:
		left, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		elementName := n.Element.Name

		_, ok := state.Global.SymbolicData.GetURLReferencedEntity(n)
		if ok { //load entity or value
			url, ok := left.(URL)
			if !ok {
				panic(ErrUnreachable)
			}

			value, err := GetOrLoadValueAtURL(state.Global.Ctx, url, state.Global)
			if err != nil {
				return nil, err
			}

			//return property
			iprops, ok := value.(IProps)
			if !ok {
				return nil, fmt.Errorf("value/entity at %s has no properties", url)
			}
			return iprops.Prop(state.Global.Ctx, elementName), nil
		}

		symbolicExtension, ok := state.Global.SymbolicData.GetUsedTypeExtension(n)

		if ok {
			extension := state.Global.Ctx.GetTypeExtension(symbolicExtension.Id)
			if extension == nil {
				panic(ErrUnreachable)
			}

			for _, propExpr := range extension.propertyExpressions {
				if propExpr.name != elementName {
					continue
				}
				//extension methods should never be accessible
				if propExpr.method != nil {
					panic(parse.ErrUnreachable)
				}
				state.PushScope()
				prevSelf := state.self
				state.self = left

				defer func() {
					state.PopScope()
					state.self = prevSelf
				}()

				computedPropValue, err := TreeWalkEval(propExpr.expression, state)
				if err != nil {
					return nil, fmt.Errorf("failed to compute property value: %w", err)
				}

				return computedPropValue, nil
			}

			panic(ErrUnreachable)
		} else {
			obj, ok := left.(*Object)
			if !ok {
				panic(ErrUnreachable)
			}
			return obj.PropNotStored(state.Global.Ctx, elementName), nil
		}

	case *parse.ComputedMemberExpression:
		left, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		iprops := left.(IProps)
		propNameVal, err := TreeWalkEval(n.PropertyName, state)
		if err != nil {
			return nil, err
		}

		propName := propNameVal.(StringLike).GetOrBuildString()

		if n.Optional && !utils.SliceContains(iprops.PropertyNames(state.Global.Ctx), propName) {
			return Nil, nil
		}

		return iprops.Prop(state.Global.Ctx, propName), nil
	case *parse.DynamicMemberExpression:
		left, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
		}
		return NewDynamicMemberValue(state.Global.Ctx, left, n.PropertyName.Name)
	case *parse.ExtractionExpression:
		left, err := TreeWalkEval(n.Object, state)
		if err != nil {
			return nil, err
		}
		result := &Object{}

		for _, key := range n.Keys.Keys {
			name := key.(*parse.IdentifierLiteral).Name
			prop := left.(IProps).Prop(state.Global.Ctx, name)
			result.SetProp(state.Global.Ctx, name, prop)
		}
		return result, nil
	case *parse.IndexExpression:
		list, err := TreeWalkEval(n.Indexed, state)
		if err != nil {
			return nil, err
		}

		index, err := TreeWalkEval(n.Index, state)
		if err != nil {
			return nil, err
		}

		return list.(Indexable).At(state.Global.Ctx, int(index.(Int))), nil
	case *parse.SliceExpression:
		slice, err := TreeWalkEval(n.Indexed, state)
		if err != nil {
			return nil, err
		}

		var startIndex Value = Int(0)
		if n.StartIndex != nil {
			startIndex, err = TreeWalkEval(n.StartIndex, state)
			if err != nil {
				return nil, err
			}
		}

		var endIndex Value = Int(math.MaxInt)
		if n.EndIndex != nil {
			endIndex, err = TreeWalkEval(n.EndIndex, state)
			if err != nil {
				return nil, err
			}
		}

		start := int(startIndex.(Int))
		if start < 0 {
			return nil, ErrNegativeLowerIndex
		}

		end := int(endIndex.(Int))
		s := slice.(Sequence)
		end = min(end, s.Len())
		return s.slice(start, end), nil
	case *parse.KeyListExpression:
		list := KeyList{}

		for _, key := range n.Keys {
			list = append(list, string(key.(parse.IIdentifierLiteral).Identifier()))
		}

		return list, nil
	case *parse.BooleanConversionExpression:
		valueToConvert, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return Bool(coerceToBool(state.Global.Ctx, valueToConvert)), nil
	case *parse.PatternDefinition:
		var right Pattern
		if n.IsLazy {
			right, err = evalStringPatternNode(n.Right, state, true)
			if err != nil {
				return nil, err
			}
		} else {
			right, err = evalPatternNode(n.Right, state)
			if err != nil {
				return nil, err
			}
		}

		name := utils.MustGet(n.PatternName())
		state.Global.Ctx.AddNamedPattern(name, right)
		return Nil, nil
	case *parse.PatternNamespaceDefinition:
		right, err := TreeWalkEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		ns, err := CreatePatternNamespace(state.Global.Ctx, right)
		if err != nil {
			return nil, err
		}
		name := utils.MustGet(n.NamespaceName())
		state.Global.Ctx.AddPatternNamespace(name, ns)
		return Nil, nil
	case *parse.PatternIdentifierLiteral:
		return resolvePattern(n, state.Global)
	case *parse.PatternNamespaceMemberExpression:
		return resolvePattern(n, state.Global)
	case *parse.PatternNamespaceIdentifierLiteral:
		return resolvePattern(n, state.Global)
	case *parse.OptionalPatternExpression:
		patt, err := TreeWalkEval(n.Pattern, state)
		if err != nil {
			return nil, err
		}
		return NewOptionalPattern(state.Global.Ctx, patt.(Pattern))
	case *parse.ComplexStringPatternPiece:
		return evalStringPatternNode(n, state, false)
	case *parse.PatternUnion:
		var cases []Pattern

		for _, case_ := range n.Cases {
			patternElement, err := evalPatternNode(case_, state)
			if err != nil {
				return nil, fmt.Errorf("failed to compile a pattern element: %s", err.Error())
			}

			cases = append(cases, patternElement)
		}

		return &UnionPattern{
			node:  node,
			cases: cases,
		}, nil
	case *parse.ObjectPatternLiteral:
		pattern := &ObjectPattern{
			inexact: !n.Exact(),
		}
		for _, p := range n.Properties {
			name := p.Name()
			var err error

			entryPatten, err := evalPatternNode(p.Value, state)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate object pattern literal, error when evaluating pattern for '%s': %s", name, err.Error())
			}

			pattern.entries = append(pattern.entries, ObjectPatternEntry{
				Name:       name,
				Pattern:    entryPatten.(Pattern),
				IsOptional: p.Optional,
			})
		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := evalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			spreadPattern := evaluatedElement.(*ObjectPattern)

			for _, entry := range spreadPattern.entries {
				//priority to property pattern defined earlier.
				if pattern.HasRequiredOrOptionalEntry(entry.Name) {
					//already present.
					continue
				}

				pattern.entries = append(pattern.entries, ObjectPatternEntry{
					Name:       entry.Name,
					Pattern:    entry.Pattern,
					IsOptional: entry.IsOptional,
					//ignore dependencies
				})
			}
		}

		pattern.init()
		return pattern, nil
	case *parse.RecordPatternLiteral:
		pattern := &RecordPattern{
			inexact: !n.Exact(),
		}
		for _, p := range n.Properties {
			name := p.Name()
			var err error

			entryPatten, err := evalPatternNode(p.Value, state)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate record pattern literal, error when evaluating pattern for '%s': %s", name, err.Error())
			}

			pattern.entries = append(pattern.entries, RecordPatternEntry{
				Name:       name,
				Pattern:    entryPatten.(Pattern),
				IsOptional: p.Optional,
			})
		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := evalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			spreadPattern := evaluatedElement.(*RecordPattern)

			for _, entry := range spreadPattern.entries {
				//priority to property pattern defined earlier.
				if pattern.HasRequiredOrOptionalEntry(entry.Name) {
					//already present.
					continue
				}

				pattern.entries = append(pattern.entries, RecordPatternEntry{
					Name:       entry.Name,
					Pattern:    entry.Pattern,
					IsOptional: entry.IsOptional,
				})
			}
		}

		pattern.init()
		return pattern, nil
	case *parse.ListPatternLiteral:

		var pattern *ListPattern
		if n.GeneralElement != nil {
			pattern = &ListPattern{}

			elementPattern, err := evalPatternNode(n.GeneralElement, state)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate list pattern literal, error when evaluating element: %s", err.Error())
			}
			pattern.generalElementPattern = elementPattern
		} else {
			pattern = &ListPattern{
				elementPatterns: []Pattern{},
			}

			for _, e := range n.Elements {
				var err error
				elementPattern, err := evalPatternNode(e, state)

				if err != nil {
					return nil, fmt.Errorf("failed to evaluate list pattern literal, error when evaluating an element: %s", err.Error())
				}

				pattern.elementPatterns = append(pattern.elementPatterns, elementPattern)
			}
		}

		return pattern, nil
	case *parse.TuplePatternLiteral:

		var pattern *TuplePattern
		if n.GeneralElement != nil {
			pattern = &TuplePattern{}

			elementPattern, err := evalPatternNode(n.GeneralElement, state)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate tuple pattern literal, error when evaluating element: %s", err.Error())
			}
			pattern.generalElementPattern = elementPattern
		} else {
			pattern = &TuplePattern{
				elementPatterns: []Pattern{},
			}

			for _, e := range n.Elements {
				var err error
				elementPattern, err := evalPatternNode(e, state)

				if err != nil {
					return nil, fmt.Errorf("failed to evaluate tuple pattern literal, error when evaluating an element: %s", err.Error())
				}

				pattern.elementPatterns = append(pattern.elementPatterns, elementPattern)
			}
		}

		return pattern, nil
	case *parse.OptionPatternLiteral:
		valuePattern, err := evalPatternNode(n.Value, state)

		if err != nil {
			return nil, fmt.Errorf("failed to evaluate an option pattern literal: %s", err.Error())
		}

		return &OptionPattern{name: n.Name, value: valuePattern}, nil
	case *parse.ConcatenationExpression:
		var values []Value
		ctx := state.Global.Ctx

		for _, elemNode := range n.Elements {
			spreadNode, isSpread := elemNode.(*parse.ElementSpreadElement)
			if isSpread {
				elemNode = spreadNode.Expr
			}

			elem, err := TreeWalkEval(elemNode, state)
			if err != nil {
				return nil, err
			}

			if !isSpread {
				values = append(values, elem)
				continue
			}

			//spread element
			it := elem.(Iterable).Iterator(ctx, IteratorConfiguration{})
			for it.Next(ctx) {
				values = append(values, it.Value(ctx))
			}
		}

		switch values[0].(type) {
		case BytesLike:
			bytesLikes := utils.MapSlice(values, func(e Value) BytesLike { return e.(BytesLike) })
			return ConcatBytesLikes(bytesLikes...)
		case StringLike:
			strLikes := utils.MapSlice(values, func(e Value) StringLike { return e.(StringLike) })
			return ConcatStringLikes(strLikes...)
		case *Tuple:
			tuples := utils.MapSlice(values, func(e Value) *Tuple { return e.(*Tuple) })
			return ConcatTuples(tuples...), nil
		default:
			return nil, fmt.Errorf("unsupported type")
		}
	case *parse.AssertionStatement:
		data := &AssertionData{
			assertionStatement: n,
			intermediaryValues: map[parse.Node]Value{},
		}

		originalHandler := state.postHandle
		defer func() {
			state.postHandle = originalHandler
		}()

		state.postHandle = func(node parse.Node, val Value, err error) (Value, error) {
			data.intermediaryValues[node] = val
			if originalHandler != nil {
				return originalHandler(node, val, err)
			}
			return val, err
		}

		ok, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		if !ok.(Bool) {
			modKind := state.Global.Module.Kind
			isTestAssertion := modKind == TestSuiteModule || modKind == TestCaseModule
			var testModule *Module
			if isTestAssertion {
				testModule = state.Global.Module
			}

			panic(&AssertionError{
				msg:             "assertion is false",
				data:            data,
				isTestAssertion: isTestAssertion,
				testModule:      testModule,
			})
		}

		return Nil, nil
	case *parse.RuntimeTypeCheckExpression:
		val, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		pattern, ok := state.Global.SymbolicData.GetRuntimeTypecheckPattern(node)
		if !ok {
			return nil, ErrMissinggRuntimeTypecheckSymbData
		}
		if pattern != nil { //enabled
			patt := pattern.(Pattern)
			if !patt.Test(state.Global.Ctx, val) {
				return nil, FormatRuntimeTypeCheckFailed(patt, state.Global.Ctx)
			}
		}

		return val, nil
	case *parse.TestSuiteExpression:
		if (!state.Global.TestingState.IsTestingEnabled || state.forceDisableTesting) && n.IsStatement {
			return Nil, nil
		}

		var meta Value = Nil
		if n.Meta != nil {
			var err error
			meta, err = TreeWalkEval(n.Meta, state)
			if err != nil {
				return nil, err
			}
		}

		expr, err := TreeWalkEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		chunk := expr.(AstNode).Node.(*parse.Chunk)

		suite, err := NewTestSuite(TestSuiteCreationInput{
			Meta:             meta,
			Node:             n,
			EmbeddedModChunk: chunk,
			ParentChunk:      state.currentChunk(),
			ParentState:      state.Global,
		})
		if err != nil {
			return nil, err
		}

		//execute the suite if the node is a statement
		if n.IsStatement {
			if !state.Global.TestingState.IsTestingEnabled {
				return Nil, nil
			}

			if ok, _ := state.Global.TestingState.Filters.IsTestEnabled(suite, state.Global); !ok {
				return Nil, nil
			}

			lthread, err := suite.Run(state.Global.Ctx)
			if err != nil {
				return nil, err
			}
			_, err = lthread.WaitResult(state.Global.Ctx)
			if err != nil {
				return nil, err
			}

			err = func() error {
				if !lthread.state.TestingState.ResultsLock.TryLock() {
					return errors.New("test results should not be locked")
				}
				defer lthread.state.TestingState.ResultsLock.Unlock()

				testCaseResults := lthread.state.TestingState.CaseResults
				testSuiteResults := lthread.state.TestingState.SuiteResults

				result, err := NewTestSuiteResult(state.Global.Ctx, testCaseResults, testSuiteResults, suite)
				if err != nil {
					return err
				}

				state.Global.TestingState.ResultsLock.Lock()
				defer state.Global.TestingState.ResultsLock.Unlock()

				state.Global.TestingState.SuiteResults = append(state.Global.TestingState.SuiteResults, result)
				return nil
			}()

			if err != nil {
				return nil, err
			}

			return Nil, nil
		} else {
			return suite, nil
		}
	case *parse.TestCaseExpression:
		if (!state.Global.TestingState.IsTestingEnabled || state.forceDisableTesting) && n.IsStatement {
			return Nil, nil
		}

		var meta Value = Nil
		if n.Meta != nil {
			var err error
			meta, err = TreeWalkEval(n.Meta, state)
			if err != nil {
				return nil, err
			}
		}

		expr, err := TreeWalkEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		chunk := expr.(AstNode).Node.(*parse.Chunk)

		positionStack, formattedLocation := state.formatLocation(node)

		testCase, err := NewTestCase(TestCaseCreationInput{
			Node: n,

			Meta:        meta,
			ModChunk:    chunk,
			ParentState: state.Global,
			ParentChunk: state.currentChunk(),

			PositionStack:     positionStack,
			FormattedLocation: formattedLocation,
		})
		if err != nil {
			return nil, err
		}

		//execute the test case if the node is a statement
		if n.IsStatement {
			if ok, _ := state.Global.TestingState.Filters.IsTestEnabled(testCase, state.Global); !ok {
				return Nil, nil
			}

			lthread, err := testCase.Run(state.Global.Ctx)
			if err != nil {
				return nil, err
			}

			result, err := lthread.WaitResult(state.Global.Ctx)

			if state.Global.Module.Kind != TestSuiteModule {
				return Nil, nil
			}

			err = func() error {
				if !lthread.state.TestingState.ResultsLock.TryLock() {
					return errors.New("test results should not be locked")
				}
				defer lthread.state.TestingState.ResultsLock.Unlock()

				testCaseResult, err := NewTestCaseResult(state.Global.Ctx, result, err, testCase)
				if err != nil {
					return err
				}

				state.Global.TestingState.ResultsLock.Lock()
				defer state.Global.TestingState.ResultsLock.Unlock()

				state.Global.TestingState.CaseResults = append(state.Global.TestingState.CaseResults, testCaseResult)
				return nil
			}()

			if err != nil {
				return nil, err
			}
			return Nil, nil
		} else {
			return testCase, nil
		}
	case *parse.LifetimejobExpression:
		meta, err := TreeWalkEval(n.Meta, state)
		if err != nil {
			return nil, err
		}

		var subjectPattern Pattern

		if n.Subject != nil {
			v, err := TreeWalkEval(n.Subject, state)
			if err != nil {
				return nil, err
			}
			subjectPattern = v.(Pattern)
		}

		mod, err := TreeWalkEval(n.Module, state)
		if err != nil {
			return nil, err
		}

		chunk := mod.(AstNode).Node.(*parse.Chunk)

		parsedChunk := &parse.ParsedChunkSource{
			Node:   chunk,
			Source: state.currentChunk().Source,
		}

		jobMod := WrapLowerModule(&inoxmod.Module{
			Kind:             LifetimeJobModule,
			TopLevelNode:     n.Module,
			MainChunk:        parsedChunk,
			ManifestTemplate: parsedChunk.Node.Manifest,
		})

		job, err := NewLifetimeJob(meta, subjectPattern, jobMod, state.Global)
		if err != nil {
			return nil, err
		}

		return job, nil
	case *parse.ReceptionHandlerExpression:
		pattern, err := TreeWalkEval(n.Pattern, state)
		if err != nil {
			return nil, err
		}
		handler, err := TreeWalkEval(n.Handler, state)
		if err != nil {
			return nil, err
		}
		return NewSynchronousMessageHandler(state.Global.Ctx, handler.(*InoxFunction), pattern.(Pattern)), nil
	case *parse.SendValueExpression:
		if state.self == nil {
			panic(ErrSelfNotDefined)
		}

		value, err := TreeWalkEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		v, err := TreeWalkEval(n.Receiver, state)
		if err != nil {
			return nil, err
		}

		if receiver, ok := v.(MessageReceiver); ok {
			if err := SendVal(state.Global.Ctx, value, receiver, state.self); err != nil {
				return nil, err
			}
		}

		return Nil, nil
	case *parse.StringTemplateLiteral:
		var sliceValues []Value

		for _, slice := range n.Slices {
			switch s := slice.(type) {
			case *parse.StringTemplateSlice:
				sliceValues = append(sliceValues, String(s.Value))
			case *parse.StringTemplateInterpolation:
				sliceValue, err := TreeWalkEval(s.Expr, state)
				if err != nil {
					return nil, err
				}
				sliceValues = append(sliceValues, sliceValue)
			}
		}

		if n.Pattern == nil {
			return NewStringFromSlices(sliceValues, n, state.Global.Ctx)
		}

		return NewCheckedString(sliceValues, n, state.Global.Ctx)
	case *parse.CssSelectorExpression:
		selector := bytes.NewBufferString("")

		for _, element := range n.Elements {
			switch e := element.(type) {
			case *parse.CssCombinator:
				switch e.Name {
				case ">", "+", "~":
					selector.WriteRune(' ')
					selector.WriteString(e.Name)
					selector.WriteRune(' ')
				case " ":
					selector.WriteRune(' ')
				}
			case *parse.CssTypeSelector:
				selector.WriteString(e.Name)
			case *parse.CssClassSelector:
				selector.WriteRune('.')
				selector.WriteString(e.Name)
			case *parse.CssPseudoClassSelector:
				selector.WriteRune(':')
				selector.WriteString(e.Name)
			case *parse.CssPseudoElementSelector:
				selector.WriteString(`::`)
				selector.WriteString(e.Name)
			case *parse.CssIdSelector:
				selector.WriteRune('#')
				selector.WriteString(e.Name)
			case *parse.CssAttributeSelector:
				selector.WriteRune('[')
				selector.WriteString(e.AttributeName.Name)
				selector.WriteString(`="`)

				val, err := TreeWalkEval(e.Value, state)
				if err != nil {
					return nil, err
				}
				selector.WriteString(fmt.Sprint(val))
				selector.WriteString(`"]`)
			}

		}
		return String(selector.String()), nil
	case parse.SimpleValueLiteral:
		return EvalSimpleValueLiteral(n, state.Global)
	case *parse.XMLExpression:
		xmlElem, err := TreeWalkEval(n.Element, state)
		if err != nil {
			return nil, err
		}

		var namespace Value
		if n.Namespace == nil {
			name := globalnames.HTML_NS
			ns, ok := state.GetGlobal(name)
			if !ok {
				return nil, errors.New("global variable " + name + " is not declared")
			}
			namespace = ns
		} else {
			namespace, err = TreeWalkEval(n.Namespace, state)
			if err != nil {
				return nil, err
			}
		}

		ns := namespace.(*Namespace)
		factory := ns.Prop(state.Global.Ctx, symbolic.FROM_XML_FACTORY_NAME).(*GoFunction)

		return factory.Call([]any{xmlElem}, state.Global, nil, false, false)
	case *parse.XMLElement:
		name := n.Opening.GetName()

		var attrs []XMLAttribute

		for _, attrNode := range n.Opening.Attributes {
			var attr XMLAttribute
			if regularAttr, ok := attrNode.(*parse.XMLAttribute); ok {
				attr.name = regularAttr.GetName()
				if regularAttr.Value != nil {
					attrValue, err := TreeWalkEval(regularAttr.Value, state)
					if err != nil {
						return nil, err
					}
					attr.value = attrValue
				} else {
					attr.value = DEFAULT_XML_ATTR_VALUE
				}
			} else {
				shorthand := attrNode.(*parse.HyperscriptAttributeShorthand)
				attr.name = inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME
				attr.value = String(shorthand.Value)
			}

			attrs = append(attrs, attr)
		}

		var children []Value

		if n.RawElementContent != "" {
			return NewRawTextXmlElement(name, attrs, n.RawElementContent), nil
		}

		for _, child := range n.Children {
			childValue, err := TreeWalkEval(child, state)
			if err != nil {
				return nil, err
			}
			children = append(children, childValue)
		}

		return NewXmlElement(name, attrs, children), nil
	case *parse.XMLText:
		//we assume factories will properly escape the string.
		return String(n.Value), nil
	case *parse.XMLInterpolation:
		val, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return val, nil

	case *parse.ExtendStatement:
		pattern, err := evalPatternNode(n.ExtendedPattern, state)
		if err != nil {
			return nil, err
		}

		lastCtxData, ok := state.Global.SymbolicData.GetContextData(n, nil)
		if !ok {
			panic(ErrUnreachable)
		}
		symbolicExtension := lastCtxData.Extensions[len(lastCtxData.Extensions)-1]

		if symbolicExtension.Statement != n {
			panic(ErrUnreachable)
		}

		extension := &TypeExtension{
			extendedPattern:   pattern,
			symbolicExtension: symbolicExtension,
		}

		for _, symbolicPropExpr := range symbolicExtension.PropertyExpressions {
			if symbolicPropExpr.Expression != nil {
				extension.propertyExpressions = append(extension.propertyExpressions, propertyExpression{
					name:       symbolicPropExpr.Name,
					expression: symbolicPropExpr.Expression,
				})
			}
		}

		objLit := n.Extension.(*parse.ObjectLiteral)

		for _, prop := range objLit.Properties {
			fnExpr, ok := prop.Value.(*parse.FunctionExpression)

			if !ok {
				continue
			}
			inoxFn, err := TreeWalkEval(fnExpr, state)
			if err != nil {
				return nil, err
			}
			extension.propertyExpressions = append(extension.propertyExpressions, propertyExpression{
				name:   prop.Name(),
				method: inoxFn.(*InoxFunction),
			})
		}

		state.Global.Ctx.AddTypeExtension(extension)
		return nil, nil
	case *parse.StructDefinition:
		return nil, nil
	case *parse.NewExpression:
		val, ok := state.Global.SymbolicData.GetMostSpecificNodeValue(n)
		if !ok {
			return nil, fmt.Errorf("no symbolic value found")
		}
		symbPtrType := val.(*symbolic.Pointer).Type()

		ptrType := state.getConcreteType(symbPtrType).(*PointerType)
		ptr := ptrType.New(state.Global.Heap)
		structPtr := (*Struct)(unsafe.Pointer(ptr))

		initLiteral, ok := n.Initialization.(*parse.StructInitializationLiteral)
		if ok {
			//initialize
			structType := ptrType.ValueType().(*StructType)
			helper := structHelperFromPtr(structPtr, int(structType.GoType().Size()))
			for _, init := range initLiteral.Fields {
				structFieldInit := init.(*parse.StructFieldInitialization)
				fieldName := structFieldInit.Name.Name
				initialFieldValue, err := TreeWalkEval(structFieldInit.Value, state)
				if err != nil {
					return nil, err
				}
				fieldRetrievalInfo := structType.FieldRetrievalInfo(fieldName)
				helper.SetValue(fieldRetrievalInfo, initialFieldValue)
			}
		}

		return structPtr, nil
	default:
		return nil, fmt.Errorf("cannot evaluate %#v (%T)\n%s", node, node, debug.Stack())
	}
}

func evalPatternNode(node parse.Node, state *TreeWalkState) (Pattern, error) {
	switch n := node.(type) {
	case *parse.ComplexStringPatternPiece:
		return evalStringPatternNode(node, state, false)
	default:
		val, err := TreeWalkEval(n, state)
		if err != nil {
			return nil, err
		}
		return toPattern(val), nil
	}
}

func evalStringPatternNode(node parse.Node, state *TreeWalkState, lazy bool) (StringPattern, error) {
	switch v := node.(type) {
	case *parse.DoubleQuotedStringLiteral:
		return NewExactStringPattern(String(v.Value)), nil
	case *parse.MultilineStringLiteral:
		return NewExactStringPattern(String(v.Value)), nil
	case *parse.RuneLiteral:
		return NewExactStringPattern(String(v.Value)), nil
	case *parse.RuneRangeExpression:
		return NewRuneRangeStringPattern(v.Lower.Value, v.Upper.Value, node), nil
	case *parse.IntegerRangeLiteral:
		upperBound := int64(math.MaxInt64)

		if v.UpperBound != nil {
			upperBound = v.UpperBound.(*parse.IntLiteral).Value
		}
		return NewIntRangeStringPattern(v.LowerBound.Value, upperBound, node), nil
	case *parse.PatternIdentifierLiteral:
		pattern := state.Global.Ctx.ResolveNamedPattern(v.Name)
		if pattern == nil {
			if lazy {
				return &DynamicStringPatternElement{name: v.Name, ctx: state.Global.Ctx}, nil
			}
			return nil, fmt.Errorf("failed to resolve a pattern identifier literal: %s", v.Name)
		}

		stringPatternElem, ok := pattern.(StringPattern)
		if !ok {
			return nil, fmt.Errorf("not a string pattern element: %T", pattern)
		}

		return stringPatternElem, nil
	case *parse.PatternNamespaceMemberExpression:
		val, err := TreeWalkEval(node, state)
		if err != nil {
			return nil, err
		}

		patt, ok := val.(StringPattern)
		if !ok {
			return nil, fmt.Errorf("pattern %%%s of namespace %s is not a string pattern", v.MemberName.Name, v.Namespace.Name)
		}

		return patt, nil
	case *parse.PatternUnion:
		var cases []StringPattern

		for _, case_ := range v.Cases {
			patternElement, err := evalStringPatternNode(case_, state, lazy)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate a pattern element: %s", err.Error())
			}
			cases = append(cases, patternElement)
		}

		return NewUnionStringPattern(node, cases)
	case *parse.ComplexStringPatternPiece:
		var subpatterns []StringPattern
		var groupNames = make(KeyList, len(v.Elements))

		for elementIndex, element := range v.Elements {
			patternElement, err := evalStringPatternNode(element.Expr, state, lazy)
			if err != nil {
				return nil, fmt.Errorf("failed to compile a pattern piece: %s", err.Error())
			}

			if element.GroupName != nil {
				groupNames[elementIndex] = element.GroupName.Name
			}

			if element.Quantifier == parse.ExactlyOneOccurrence {
				subpatterns = append(subpatterns, patternElement)
			} else {
				subpatterns = append(subpatterns, newRepeatedPatternElement(element.Quantifier, element.ExactOcurrenceCount, patternElement))
			}
		}

		return NewSequenceStringPattern(v, state.currentChunk().Node, subpatterns, groupNames)
	case *parse.RegularExpressionLiteral:
		return NewRegexPattern(v.Value), nil
	default:
		return nil, fmt.Errorf("cannot evalute string pattern element: %T", v)
	}
}

func evalBinaryExpression(n *parse.BinaryExpression, state *TreeWalkState) (Value, error) {
	left, err := TreeWalkEval(n.Left, state)
	if err != nil {
		return nil, err
	}

	right, err := TreeWalkEval(n.Right, state)
	if err != nil {
		return nil, err
	}

	switch n.Operator {
	case parse.GreaterThan, parse.GreaterOrEqual, parse.LessThan, parse.LessOrEqual:
		comparable := left.(Comparable)
		comparisonResult, ok := comparable.Compare(right)
		if !ok { //not comparable
			leftF, ok := left.(Float)
			if ok && (math.IsNaN(float64(leftF)) || math.IsInf(float64(leftF), 0)) {
				return nil, ErrNaNinfinityOperand
			}

			rightF, ok := right.(Float)
			if ok && (math.IsNaN(float64(rightF)) || math.IsInf(float64(rightF), 0)) {
				return nil, ErrNaNinfinityOperand
			}

			return nil, ErrNotComparable
		}

		switch n.Operator {
		case parse.GreaterThan:
			return Bool(comparisonResult > 0), nil
		case parse.GreaterOrEqual:
			return Bool(comparisonResult >= 0), nil
		case parse.LessThan:
			return Bool(comparisonResult < 0), nil
		case parse.LessOrEqual:
			return Bool(comparisonResult <= 0), nil
		}
		panic(ErrUnreachable)
	case parse.Add, parse.Sub, parse.Mul, parse.Div:
		return evalArithmeticBinaryExpression(left, right, n.Operator)
	case parse.Equal:
		return Bool(left.Equal(state.Global.Ctx, right, map[uintptr]uintptr{}, 0)), nil
	case parse.NotEqual:
		return Bool(!left.Equal(state.Global.Ctx, right, map[uintptr]uintptr{}, 0)), nil
	case parse.Is:
		return Bool(Same(left, right)), nil
	case parse.IsNot:
		return Bool(!Same(left, right)), nil
	case parse.In:
		switch rightVal := right.(type) {
		case Container:
			return Bool(rightVal.Contains(state.Global.Ctx, left.(Serializable))), nil
		default:
			return nil, fmt.Errorf("invalid binary expression: cannot check if value is inside a %T", rightVal)
		}
	case parse.NotIn:
		switch rightVal := right.(type) {
		case Container:
			return !Bool(rightVal.Contains(state.Global.Ctx, left.(Serializable))), nil
		default:
			return nil, fmt.Errorf("invalid binary expression: cannot check if value is inside a(n) %T", rightVal)
		}

	case parse.Keyof:
		key, ok := left.(String)
		if !ok {
			return nil, fmt.Errorf("invalid binary expression: keyof: left operand is not a string, but a %T", left)
		}

		switch rightVal := right.(type) {
		case *Object:
			return Bool(rightVal.HasProp(state.Global.Ctx, string(key))), nil
		default:
			return nil, fmt.Errorf("invalid binary expression: cannot check if non object has a key: %T", rightVal)
		}
	case parse.Urlof:
		url, ok := left.(URL)
		if !ok {
			return nil, fmt.Errorf("invalid binary expression: keyof: left operand is not a URL, but a %T", left)
		}

		urlHolder, isUrlHolder := right.(UrlHolder)

		var result = false
		if isUrlHolder {
			actualURL, ok := urlHolder.URL()
			if ok {
				result = url.Equal(state.Global.Ctx, actualURL, nil, 0)
			}
		}

		return Bool(result), nil
	case parse.Range, parse.ExclEndRange:
		switch left.(type) {
		case Int:
			end := right.(Int)
			if n.Operator == parse.ExclEndRange {
				end--
			}
			return IntRange{
				start: int64(left.(Int)),
				end:   int64(end),
				step:  1,
			}, nil
		case Float:
			return FloatRange{
				inclusiveEnd: n.Operator == parse.Range,
				start:        float64(left.(Float)),
				end:          float64(right.(Float)),
			}, nil
		default:
			return QuantityRange{
				inclusiveEnd: n.Operator == parse.Range,
				start:        left.(Serializable),
				end:          right.(Serializable),
			}, nil
		}
	case parse.And:
		return left.(Bool) && right.(Bool), nil
	case parse.Or:
		return left.(Bool) || right.(Bool), nil
	case parse.Match, parse.NotMatch:
		ok := right.(Pattern).Test(state.Global.Ctx, left)
		if n.Operator == parse.NotMatch {
			ok = !ok
		}
		return Bool(ok), nil
	case parse.Substrof:
		return Bool(isSubstrOf(state.Global.Ctx, left, right)), nil
	case parse.SetDifference:
		if _, ok := right.(Pattern); !ok {
			right = NewExactValuePattern(right.(Serializable))
		}
		return &DifferencePattern{base: left.(Pattern), removed: right.(Pattern)}, nil
	case parse.NilCoalescing:
		if _, ok := left.(NilT); !ok {
			return left, nil
		}
		return right, nil
	case parse.PairComma:
		return NewOrderedPair(left.(Serializable), right.(Serializable)), nil
	default:
		return nil, errors.New("invalid binary operator " + strconv.Itoa(int(n.Operator)))
	}
}

func evalArithmeticBinaryExpression(left, right Value, operator parse.BinaryOperator) (Value, error) {
	if _, ok := left.(Int); ok {
		switch operator {
		case parse.Add:
			return intAdd(left.(Int), right.(Int))
		case parse.Sub:
			return intSub(left.(Int), right.(Int))
		case parse.Mul:
			return intMul(left.(Int), right.(Int))
		case parse.Div:
			return intDiv(left.(Int), right.(Int))
		}
	}

	if leftF, ok := left.(Float); ok {
		rightF := right.(Float)

		if math.IsNaN(float64(leftF)) || math.IsInf(float64(leftF), 0) {
			return nil, ErrNaNinfinityOperand
		}

		if math.IsNaN(float64(rightF)) || math.IsInf(float64(rightF), 0) {
			return nil, ErrNaNinfinityOperand
		}

		switch operator {
		case parse.Add:
			return leftF + rightF, nil
		case parse.Sub:
			return leftF - rightF, nil
		case parse.Mul:
			f := leftF * rightF
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				return nil, ErrNaNinfinityResult
			}
			return f, nil
		case parse.Div:
			f := leftF / rightF
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				return nil, ErrNaNinfinityResult
			}
			return f, nil
		}
		panic(ErrUnreachable)
	}

	switch operator {
	case parse.Add:
		return left.(IPseudoAdd).Add(right)
	case parse.Sub:
		return left.(IPseudoSub).Sub(right)
	case parse.Mul:
	case parse.Div:
	}
	panic(ErrUnreachable)
}

func evalForStatement(n *parse.ForStatement, state *TreeWalkState) error {
	iteratedValue, err := TreeWalkEval(n.IteratedValue, state)
	scope := state.CurrentLocalScope()
	if err != nil {
		return err
	}

	var keyPattern Pattern
	var valuePattern Pattern

	if n.KeyPattern != nil {
		v, err := TreeWalkEval(n.KeyPattern, state)
		if err != nil {
			return err
		}
		keyPattern = v.(Pattern)
	}

	if n.ValuePattern != nil {
		v, err := TreeWalkEval(n.ValuePattern, state)
		if err != nil {
			return err
		}
		valuePattern = v.(Pattern)
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
			delete(scope, kVarname)
		}
		if n.ValueElemIdent != nil {
			delete(scope, eVarname)
		}
	}()

	if iterable, ok := iteratedValue.(Iterable); ok {
		if n.Chunked {
			return errors.New("chunked iteration of iterables is not supported yet")
		}

		it := iterable.Iterator(state.Global.Ctx, IteratorConfiguration{
			KeyFilter:   keyPattern,
			ValueFilter: valuePattern,
		})
		index := 0

	iterable_iteration:
		for it.HasNext(state.Global.Ctx) {
			it.Next(state.Global.Ctx)

			if n.KeyIndexIdent != nil {
				scope[kVarname] = it.Key(state.Global.Ctx)
			}
			if n.ValueElemIdent != nil {
				scope[eVarname] = it.Value(state.Global.Ctx)
			}

			//Evalute body

			_, err := TreeWalkEval(n.Body, state)
			if err != nil {
				return err
			}

			//Handle return/break/continue/yield/prune

			if state.returnValue != nil {
				return nil
			}

			switch state.iterationChange {
			case BreakIteration:
				state.iterationChange = NoIterationChange
				break iterable_iteration
			case ContinueIteration:
				state.iterationChange = NoIterationChange
				index++
				continue iterable_iteration
			case YieldItem:
				return nil
			case PruneWalk:
				return nil
			}
			index++
		}
	} else if stremable, ok := iteratedValue.(StreamSource); ok {
		stream := stremable.Stream(state.Global.Ctx, &ReadableStreamConfiguration{
			Filter: valuePattern,
		})
		defer stream.Stop()

		chunked := n.Chunked

	stream_iteration:
		for {
			select {
			case <-state.Global.Ctx.Done():
				return state.Global.Ctx.Err()
			default:
			}

			var next Value
			var streamErr error

			if chunked {
				sizeRange := NewIntRange(DEFAULT_MIN_STREAM_CHUNK_SIZE, DEFAULT_MAX_STREAM_CHUNK_SIZE)
				next, streamErr = stream.WaitNextChunk(state.Global.Ctx, nil, sizeRange, STREAM_ITERATION_WAIT_TIMEOUT)
			} else {
				next, streamErr = stream.WaitNext(state.Global.Ctx, nil, STREAM_ITERATION_WAIT_TIMEOUT)
			}

			nextChunk, _ := next.(*DataChunk)

			if streamErr == nil || (nextChunk != nil && nextChunk.ElemCount() > 0) {
				scope[eVarname] = next

				//Evalute body

				_, err := TreeWalkEval(n.Body, state)
				if err != nil {
					return err
				}

				//Handle return/break/continue/yield/prune

				if state.returnValue != nil {
					return nil
				}

				switch state.iterationChange {
				case BreakIteration:
					state.iterationChange = NoIterationChange
					break stream_iteration
				case ContinueIteration:
					state.iterationChange = NoIterationChange
					continue stream_iteration
				case YieldItem:
					return nil
				case PruneWalk:
					return nil
				}
			}

			if errors.Is(streamErr, ErrEndOfStream) {
				break stream_iteration
			}
			if (chunked && errors.Is(streamErr, ErrStreamChunkWaitTimeout)) ||
				(!chunked && errors.Is(streamErr, ErrStreamElemWaitTimeout)) {
				continue stream_iteration
			}
			if streamErr != nil {
				return streamErr
			}
		}
	} else {
		return fmt.Errorf("cannot iterate %#v", iteratedValue)
	}
	return nil
}

func evalForExpression(n *parse.ForExpression, state *TreeWalkState) (Value, error) {
	iteratedValue, err := TreeWalkEval(n.IteratedValue, state)
	scope := state.CurrentLocalScope()
	if err != nil {
		return nil, err
	}

	var keyPattern Pattern
	var valuePattern Pattern

	if n.KeyPattern != nil {
		v, err := TreeWalkEval(n.KeyPattern, state)
		if err != nil {
			return nil, err
		}
		keyPattern = v.(Pattern)
	}

	if n.ValuePattern != nil {
		v, err := TreeWalkEval(n.ValuePattern, state)
		if err != nil {
			return nil, err
		}
		valuePattern = v.(Pattern)
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
			delete(scope, kVarname)
		}
		if n.ValueElemIdent != nil {
			delete(scope, eVarname)
		}
	}()

	var elements []Serializable

	if iterable, ok := iteratedValue.(Iterable); ok {
		if n.Chunked {
			return nil, errors.New("chunked iteration of iterables is not supported yet")
		}

		it := iterable.Iterator(state.Global.Ctx, IteratorConfiguration{
			KeyFilter:   keyPattern,
			ValueFilter: valuePattern,
		})
		index := 0

	iterable_iteration:
		for it.HasNext(state.Global.Ctx) {
			it.Next(state.Global.Ctx)

			if n.KeyIndexIdent != nil {
				scope[kVarname] = it.Key(state.Global.Ctx)
			}
			if n.ValueElemIdent != nil {
				scope[eVarname] = it.Value(state.Global.Ctx)
			}

			//Evaluate body.

			_, isBlockBody := n.Body.(*parse.Block)

			elem, err := TreeWalkEval(n.Body, state)
			if err != nil {
				return nil, err
			}

			if !isBlockBody {
				elements = append(elements, elem.(Serializable))
				index++
			}

			//Handle break/continue/yield. Return and yield statements are not allowed in the body.

			switch state.iterationChange {
			case BreakIteration:
				state.iterationChange = NoIterationChange
				break iterable_iteration
			case ContinueIteration:
				state.iterationChange = NoIterationChange
			case YieldItem:
				state.iterationChange = NoIterationChange
				elements = append(elements, state.yieldedValue.(Serializable))
				state.yieldedValue = nil
			}

			index++
		}
	} else if stremable, ok := iteratedValue.(StreamSource); ok {
		stream := stremable.Stream(state.Global.Ctx, &ReadableStreamConfiguration{
			Filter: valuePattern,
		})
		defer stream.Stop()

		chunked := n.Chunked

	stream_iteration_for_expr:
		for {
			select {
			case <-state.Global.Ctx.Done():
				return nil, state.Global.Ctx.Err()
			default:
			}

			var next Value
			var streamErr error

			if chunked {
				sizeRange := NewIntRange(DEFAULT_MIN_STREAM_CHUNK_SIZE, DEFAULT_MAX_STREAM_CHUNK_SIZE)
				next, streamErr = stream.WaitNextChunk(state.Global.Ctx, nil, sizeRange, STREAM_ITERATION_WAIT_TIMEOUT)
			} else {
				next, streamErr = stream.WaitNext(state.Global.Ctx, nil, STREAM_ITERATION_WAIT_TIMEOUT)
			}

			nextChunk, _ := next.(*DataChunk)

			if streamErr == nil || (nextChunk != nil && nextChunk.ElemCount() > 0) {
				scope[eVarname] = next

				//Evaluate body.

				elem, err := TreeWalkEval(n.Body, state)
				if err != nil {
					return nil, err
				}

				_, isBlockBody := n.Body.(*parse.Block)

				if !isBlockBody {
					elements = append(elements, elem.(Serializable))
					continue stream_iteration_for_expr
				}

				//Handle break/continue/yield. Return and yield statements are not allowed in the body.

				switch state.iterationChange {
				case BreakIteration:
					state.iterationChange = NoIterationChange
					break stream_iteration_for_expr
				case ContinueIteration:
					state.iterationChange = NoIterationChange
					continue stream_iteration_for_expr
				case YieldItem:
					state.iterationChange = NoIterationChange
					elements = append(elements, state.yieldedValue.(Serializable))
					state.yieldedValue = nil
					continue stream_iteration_for_expr
				}
			}

			if errors.Is(streamErr, ErrEndOfStream) {
				break stream_iteration_for_expr
			}
			if (chunked && errors.Is(streamErr, ErrStreamChunkWaitTimeout)) ||
				(!chunked && errors.Is(streamErr, ErrStreamElemWaitTimeout)) {
				continue stream_iteration_for_expr
			}
			if streamErr != nil {
				return nil, streamErr
			}
		}
	} else {
		return nil, fmt.Errorf("cannot iterate %#v", iteratedValue)
	}
	return NewWrappedValueList(elements...), nil
}

func evalWalkStatement(n *parse.WalkStatement, state *TreeWalkState) error {
	walkable, err := TreeWalkEval(n.Walked, state)
	if err != nil {
		return err
	}
	scope := state.CurrentLocalScope()
	entryName := n.EntryIdent.Name
	defer func() {
		delete(scope, entryName)
	}()

	//we check the permissions

	//

	walker, err := walkable.(Walkable).Walker(state.Global.Ctx)
	if err != nil {
		return err
	}

walk_loop:
	for {
		if !walker.HasNext(state.Global.Ctx) {
			break
		}
		walker.Next(state.Global.Ctx)
		entry := walker.Value(state.Global.Ctx)
		scope[entryName] = entry

		_, blkErr := TreeWalkEval(n.Body, state)
		if blkErr != nil {
			return blkErr
		}

		switch state.iterationChange {
		case PruneWalk:
			state.iterationChange = NoIterationChange
			walker.Prune(state.Global.Ctx)
		case BreakIteration:
			break walk_loop
		case ContinueIteration:
			state.iterationChange = NoIterationChange
			continue
		case YieldItem:
			return nil
		}
	}

	state.iterationChange = NoIterationChange

	return err
}

func evalSwitchStatement(n *parse.SwitchStatement, state *TreeWalkState) error {
	discriminant, err := TreeWalkEval(n.Discriminant, state)
	if err != nil {
		return err
	}

	for _, switchCase := range n.Cases {
		for _, valNode := range switchCase.Values {
			val, err := TreeWalkEval(valNode, state)
			if err != nil {
				return err
			}
			if discriminant.Equal(state.Global.Ctx, val, map[uintptr]uintptr{}, 0) {
				_, err := TreeWalkEval(switchCase.Block, state)
				if err != nil {
					return err
				}
				goto switch_end
			}
		}
	}
	//if we are here there was no match
	if len(n.DefaultCases) > 0 {
		_, err := TreeWalkEval(n.DefaultCases[0].Block, state)
		if err != nil {
			return err
		}
	}
switch_end:

	if state.iterationChange == BreakIteration {
		state.iterationChange = NoIterationChange
	}

	return nil
}

func evalSwitchExpression(n *parse.SwitchExpression, state *TreeWalkState) (Value, error) {
	discriminant, err := TreeWalkEval(n.Discriminant, state)
	if err != nil {
		return nil, err
	}

	for _, switchCase := range n.Cases {
		for _, valNode := range switchCase.Values {
			val, err := TreeWalkEval(valNode, state)
			if err != nil {
				return nil, err
			}
			if discriminant == val {
				return TreeWalkEval(switchCase.Result, state)
			}
		}
	}
	//if we are here there was no match
	if len(n.DefaultCases) > 0 {
		return TreeWalkEval(n.DefaultCases[0].Result, state)
	}
	return DEFAULT_SWITCH_MATCH_EXPR_RESULT, nil
}

func evalMatchStatement(n *parse.MatchStatement, state *TreeWalkState) error {
	discriminant, err := TreeWalkEval(n.Discriminant, state)
	if err != nil {
		return err
	}

	for _, matchCase := range n.Cases {

		for _, valNode := range matchCase.Values {
			m, err := TreeWalkEval(valNode, state)
			if err != nil {
				return err
			}

			pattern, ok := m.(Pattern)

			if !ok { //if the value of the case is not a pattern we just check for equality
				pattern = &ExactValuePattern{value: m.(Serializable)}
			}

			if matchCase.GroupMatchingVariable != nil {
				variable := matchCase.GroupMatchingVariable.(*parse.IdentifierLiteral)

				groupPattern, _ := pattern.(GroupPattern)
				groups, ok, err := groupPattern.MatchGroups(state.Global.Ctx, discriminant.(Serializable))

				if err != nil {
					return fmt.Errorf("match statement: group maching: %w", err)
				}
				if ok {
					state.CurrentLocalScope()[variable.Name] = objFrom(groups)

					_, err := TreeWalkEval(matchCase.Block, state)
					if err != nil {
						return err
					}
					goto match_end
				}

			} else if pattern.Test(state.Global.Ctx, discriminant) {
				_, err := TreeWalkEval(matchCase.Block, state)
				if err != nil {
					return err
				}
				goto match_end
			}
		}
	}

	//if we are here there was no match
	if len(n.DefaultCases) > 0 {
		_, err := TreeWalkEval(n.DefaultCases[0].Block, state)
		if err != nil {
			return err
		}
	}
match_end:

	if state.iterationChange == BreakIteration {
		state.iterationChange = NoIterationChange
	}

	return nil
}

func evalMatchExpression(n *parse.MatchExpression, state *TreeWalkState) (Value, error) {
	discriminant, err := TreeWalkEval(n.Discriminant, state)
	if err != nil {
		return nil, err
	}

	for _, matchCase := range n.Cases {

		for _, valNode := range matchCase.Values {
			m, err := TreeWalkEval(valNode, state)
			if err != nil {
				return nil, err
			}

			pattern, ok := m.(Pattern)

			if !ok { //if the value of the case is not a pattern we just check for equality
				pattern = &ExactValuePattern{value: m.(Serializable)}
			}

			if matchCase.GroupMatchingVariable != nil {
				variable := matchCase.GroupMatchingVariable.(*parse.IdentifierLiteral)

				groupPattern, _ := pattern.(GroupPattern)
				groups, ok, err := groupPattern.MatchGroups(state.Global.Ctx, discriminant.(Serializable))

				if err != nil {
					return nil, fmt.Errorf("match statement: group maching: %w", err)
				}
				if ok {
					state.CurrentLocalScope()[variable.Name] = objFrom(groups)
					return TreeWalkEval(matchCase.Result, state)
				}

			} else if pattern.Test(state.Global.Ctx, discriminant) {
				return TreeWalkEval(matchCase.Result, state)
			}
		}
	}

	//if we are here there was no match
	if len(n.DefaultCases) > 0 {
		return TreeWalkEval(n.DefaultCases[0].Result, state)
	}

	return DEFAULT_SWITCH_MATCH_EXPR_RESULT, nil
}

func precheckGlobalVariableDeclaration(name string, state *TreeWalkState) error {
	alreadyDefined := state.Global.Globals.Has(name)
	if alreadyDefined {
		if _, ok := state.constantVars[name]; ok {
			return errors.New("attempt to assign a constant global")
		}

		return state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permbase.Update, Name: name})
	} else {
		return state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permbase.Create, Name: name})
	}
}
