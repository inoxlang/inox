package core

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"math"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

func NewTreeWalkState(ctx *Context, constants ...map[string]Value) *TreeWalkState {
	global := NewGlobalState(ctx, constants...)

	return NewTreeWalkStateWithGlobal(global)
}

func NewTreeWalkStateWithGlobal(global *GlobalState) *TreeWalkState {
	var chunkStack []*parse.ParsedChunk
	if global.Module != nil {
		chunkStack = append(chunkStack, global.Module.MainChunk)
	}

	return &TreeWalkState{
		LocalScopeStack: []map[string]Value{},
		chunkStack:      chunkStack,
		constantVars:    map[string]bool{},
		Global:          global,
	}
}

// A TreeWalkState stores all the data accessed during the tree walking evaluation.
type TreeWalkState struct {
	Global          *GlobalState
	LocalScopeStack []map[string]Value
	frameInfo       []StackFrameInfo //used for debugging only, the list is reversed
	chunkStack      []*parse.ParsedChunk
	constantVars    map[string]bool
	postHandle      func(node parse.Node, val Value, err error) (Value, error)

	debug           *Debugger
	returnValue     Value           //return value from a function or module
	iterationChange IterationChange //break, continue, prune
	self            Value           //value of self in methods
	entryComputeFn  func(v Value) (Value, error)
}

func (state TreeWalkState) currentChunk() *parse.ParsedChunk {
	if len(state.chunkStack) == 0 {
		state.chunkStack = append(state.chunkStack, state.Global.Module.MainChunk)
	}
	return state.chunkStack[len(state.chunkStack)-1]
}

func (state *TreeWalkState) pushChunk(chunk *parse.ParsedChunk) {
	state.chunkStack = append(state.chunkStack, chunk)
}

func (state *TreeWalkState) popChunk() {
	state.chunkStack = state.chunkStack[:len(state.chunkStack)-1]
}

func (state *TreeWalkState) SetGlobal(name string, value Value, constness GlobalConstness) (ok bool) {
	if state.constantVars[name] {
		return false
	}

	state.Global.Globals.Set(name, value)

	if constness == GlobalConst {
		state.constantVars[name] = true
	}

	if watchable, ok := value.(SystemGraphNodeValue); ok {
		state.Global.ProposeSystemGraph(watchable, name)
	}

	return true
}

func (state *TreeWalkState) HasGlobal(name string) bool {
	return state.Global.Globals.Has(name)
}

func (state *TreeWalkState) Get(name string) (Value, bool) {
	for i := len(state.LocalScopeStack) - 1; i >= 0; i-- {
		if v, ok := state.LocalScopeStack[i][name]; ok {
			return v, true
		}
	}
	val := state.Global.Globals.Get(name)
	return val, val != nil
}

func (state *TreeWalkState) CurrentLocalScope() map[string]Value {
	if len(state.LocalScopeStack) == 0 {
		return nil
	}
	return state.LocalScopeStack[len(state.LocalScopeStack)-1]
}

func (state *TreeWalkState) PushScope() {
	state.LocalScopeStack = append(state.LocalScopeStack, make(map[string]Value))
}

func (state *TreeWalkState) PopScope() {
	state.LocalScopeStack = state.LocalScopeStack[:len(state.LocalScopeStack)-1]
}

func (state *TreeWalkState) GetGlobalState() *GlobalState {
	return state.Global
}

func (state *TreeWalkState) AttachDebugger(debugger *Debugger) {
	if state.debug != nil {
		panic(ErrDebuggerAlreadyAttached)
	}

	if !state.Global.Debugger.CompareAndSwap(nil, debugger) {
		panic(ErrDebuggerAlreadyAttached)
	}

	state.debug = debugger
}

func (state *TreeWalkState) DetachDebugger() {
	state.debug = nil
	state.Global.Debugger.Store((*Debugger)(nil))
}

func (state *TreeWalkState) updateStackTrace(currentStmt parse.Node) {
	currentFrame := state.frameInfo[len(state.frameInfo)-1]
	currentFrame.Node = currentStmt

	line, col := currentFrame.Chunk.GetLineColumn(currentStmt)
	currentFrame.StatementStartLine = line
	currentFrame.StatementStartColumn = col

	state.frameInfo[len(state.frameInfo)-1] = currentFrame
}

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

		if err != nil && state.Global.Module != nil && state.Global.Module.Name() != "" {
			if !strings.HasPrefix(err.Error(), state.Global.Module.Name()) {
				locationPartBuff := bytes.NewBuffer(nil)
				var positionStack parse.SourcePositionStack

				//TODO: get whole position stack
				for i, chunk := range state.chunkStack {
					if i == 0 {
						positionStack = append(positionStack, chunk.GetSourcePosition(node.Base().Span))
					}

					chunk.FormatNodeLocation(locationPartBuff, node) //TODO: fix
					locationPartBuff.WriteRune(' ')
				}
				location := locationPartBuff.String()
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
			err = state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permkind.Use, Name: n.Left.Name})
			if err != nil {
				return nil, err
			}
		}

		for _, idents := range n.PropertyNames {
			v = v.(IProps).Prop(state.Global.Ctx, idents.Name)
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
			queryValue := Str("")
			param := p.(*parse.URLQueryParameter)
			queryParamNames = append(queryParamNames, Str(param.Name))

			for _, slice := range param.Value {
				val, err := TreeWalkEval(slice, state)
				if err != nil {
					return nil, err
				}
				stringified, err := stringifyQueryParamValue(val)
				if err != nil {
					return nil, err
				}
				queryValue += Str(stringified)
			}
			queryValues = append(queryValues, queryValue)
		}

		return NewURL(host, pathSlices, isStaticPathSliceList, queryParamNames, queryValues)
	case *parse.HostExpression:
		hostnamePort, err := TreeWalkEval(n.Host, state)
		if err != nil {
			return nil, err
		}
		return NewHost(hostnamePort, n.Scheme.Name)
	case *parse.Variable:
		v, ok := state.CurrentLocalScope()[n.Name]

		if !ok {
			return nil, errors.New("variable " + n.Name + " is not declared")
		}
		return v, nil
	case *parse.GlobalVariable:
		err := state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permkind.Read, Name: n.Name})
		if err != nil {
			return nil, err
		}
		v, ok := state.Global.Globals.CheckedGet(n.Name)

		if !ok {
			return nil, errors.New("global variable " + n.Name + " is not declared")
		}
		return v, nil
	case *parse.ReturnStatement:
		if n.Expr == nil {
			state.returnValue = Nil
			return Nil, nil
		}

		value, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		state.returnValue = value
		return Nil, nil
	case *parse.YieldStatement:
		if n.Expr == nil {
			state.returnValue = Nil
			return Nil, nil
		}

		value, err := TreeWalkEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		if state.Global.LThread == nil {
			panic(errors.New("failed to yield: no associated lthread"))
		}
		state.Global.LThread.yield(state.Global.Ctx, value)
		return Nil, nil
	case *parse.BreakStatement:
		state.iterationChange = BreakIteration
		return Nil, nil
	case *parse.ContinueStatement:
		state.iterationChange = ContinueIteration
		return Nil, nil
	case *parse.PruneStatement:
		state.iterationChange = PruneWalk
		return Nil, nil
	case *parse.CallExpression:

		var (
			callee Value
			self   Value
		)

		//we first get the callee
		switch c := n.Callee.(type) {
		case *parse.IdentifierLiteral:
			err := state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permkind.Use, Name: c.Name})
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
				err = state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permkind.Use, Name: c.Left.Name})
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

			if obj, ok := left.(*Object); ok {
				self = obj
			} else {
				self = nil
			}

			callee = left.(IProps).Prop(state.Global.Ctx, c.PropertyName.Name)
		case *parse.DoubleColonExpression:
			elementName := c.Element.Name
			extendedValue, err := TreeWalkEval(c.Left, state)
			if err != nil {
				return nil, err
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
				//extension methods should never be accessible
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

		return callee.(Pattern).Call(args)
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
			name := decl.Left.(*parse.IdentifierLiteral).Name

			right, err := TreeWalkEval(decl.Right, state)
			if err != nil {
				return nil, err
			}
			currentScope[name] = right
		}
		return Nil, nil
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
				return intMul(left().(Int), right.(Int))
			}

			return right, nil
		}

		switch lhs := n.Left.(type) {
		case *parse.Variable:
			currentLocalScope := state.CurrentLocalScope()

			name := lhs.Name
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
			currentLocalScope := state.CurrentLocalScope()

			name := lhs.Name
			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			right, err = handleAssignmentOperation(utils.Ret(currentLocalScope[name]), right)
			if err != nil {
				return nil, err
			}

			currentLocalScope[name] = right
		case *parse.GlobalVariable:
			name := lhs.Name
			alreadyDefined := state.Global.Globals.Has(name)
			if alreadyDefined {
				if _, ok := state.constantVars[name]; ok {
					return nil, errors.New("attempt to assign a constant global")
				}

				err := state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permkind.Update, Name: name})
				if err != nil {
					return nil, err
				}
			} else {
				err = state.Global.Ctx.CheckHasPermission(GlobalVarPermission{Kind_: permkind.Create, Name: name})
				if err != nil {
					return nil, err
				}
			}

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			getLeft := func() Value {
				return state.Global.Globals.Get(name)
			}
			right, err = handleAssignmentOperation(getLeft, right)
			if err != nil {
				return nil, err
			}

			state.SetGlobal(name, right, GlobalVar)
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
			getLeft := func() Value {
				return left.(*Object).Prop(state.Global.Ctx, key)
			}

			right, err = handleAssignmentOperation(getLeft, right)
			if err != nil {
				return nil, err
			}

			return nil, left.(IProps).SetProp(state.Global.Ctx, key, right)
		case *parse.IdentifierMemberExpression:
			v, err := TreeWalkEval(lhs.Left, state)
			if err != nil {
				return nil, err
			}

			for _, idents := range lhs.PropertyNames[:len(lhs.PropertyNames)-1] {
				v = v.(IProps).Prop(state.Global.Ctx, idents.Name)
			}

			right, err := TreeWalkEval(n.Right, state)
			if err != nil {
				return nil, err
			}

			lastPropName := lhs.PropertyNames[len(lhs.PropertyNames)-1].Name

			iprops := v.(IProps)
			getLeft := func() Value {
				return iprops.Prop(state.Global.Ctx, lastPropName)
			}

			right, err = handleAssignmentOperation(getLeft, right)
			if err != nil {
				return nil, err
			}

			return nil, iprops.SetProp(state.Global.Ctx, lastPropName, right)
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
	case *parse.HostAliasDefinition:
		name := n.Left.Value[1:]
		value, err := TreeWalkEval(n.Right, state)
		if err != nil {
			return nil, err
		}
		state.Global.Ctx.AddHostAlias(name, value.(Host))

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
		defer func() {
			state.returnValue = nil
			state.iterationChange = NoIterationChange
			if manageLocalScope {
				state.PopScope()
			}
		}()

		//CONSTANTS
		if n.GlobalConstantDeclarations != nil {
			for _, decl := range n.GlobalConstantDeclarations.Declarations {
				if !state.SetGlobal(decl.Ident().Name, utils.Must(TreeWalkEval(decl.Right, state)), GlobalConst) {
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
			case BreakIteration, ContinueIteration, PruneWalk:
				break loop
			}
		}
		return Nil, nil
	case *parse.SynchronizedBlockStatement:
		var lockedValues []PotentiallySharable
		defer func() {
			for _, val := range utils.ReversedSlice(lockedValues) {
				val.ForceUnlock()
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
			potentiallySharable.ForceLock()

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
		state.pushChunk(chunk.ParsedChunk)
		defer state.popChunk()

		if state.debug != nil {
			frameCount := len(state.frameInfo)
			prevChunk := state.frameInfo[frameCount-1].Chunk
			prevName := state.frameInfo[frameCount-1].Name

			state.frameInfo[frameCount-1].Chunk = chunk.ParsedChunk
			state.frameInfo[frameCount-1].Name = chunk.Name()
			defer func() {
				state.frameInfo[frameCount-1].Chunk = prevChunk
				state.frameInfo[frameCount-1].Name = prevName
			}()
		}

		return TreeWalkEval(chunk.Node, state)
	case *parse.ImportStatement:
		varPerm := GlobalVarPermission{permkind.Create, n.Identifier.Name}
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
							globals := &Struct{}
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
							globals.structType = NewStructPattern("", ulid.Make(), keys, types)
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

		state.Global.Globals.Foreach(func(name string, v Value, isConstant bool) error {
			if isConstant {
				actualGlobals[name] = v
				startConstants = append(startConstants, name)
			}
			return nil
		})

		switch g := globalsDesc.(type) {
		case *Struct:
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

			calleeIdent := n.Module.Statements[0].(*parse.CallExpression).Callee.(*parse.IdentifierLiteral)
			callee, _ := state.Get(calleeIdent.Name)
			actualGlobals[calleeIdent.Name] = callee
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

		parsedChunk := &parse.ParsedChunk{
			Node:   chunk,
			Source: state.currentChunk().Source,
		}

		routineMod := &Module{
			MainChunk:  parsedChunk,
			ModuleKind: UserLThreadModule,
		}

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
	case *parse.UDataLiteral:
		rootVal, err := TreeWalkEval(n.Root, state)
		if err != nil {
			return nil, err
		}

		var children []UDataHiearchyEntry

		for _, entry := range n.Children {
			child, err := TreeWalkEval(entry, state)
			if err != nil {
				return nil, err
			}

			children = append(children, child.(UDataHiearchyEntry))
		}

		udata := &UData{
			Root:            rootVal.(Serializable),
			HiearchyEntries: children,
		}

		return udata, nil
	case *parse.UDataEntry:
		nodeVal, err := TreeWalkEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		var children []UDataHiearchyEntry

		for _, entry := range n.Children {
			child, err := TreeWalkEval(entry, state)
			if err != nil {
				return nil, err
			}

			children = append(children, child.(UDataHiearchyEntry))
		}

		return UDataHiearchyEntry{
			Value:    nodeVal.(Serializable),
			Children: children,
		}, nil
	case *parse.ObjectLiteral:
		finalObj := &Object{}

		indexKey := 0
		for _, p := range n.Properties {
			v, err := TreeWalkEval(p.Value, state)
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

		finalObj.sortProps()
		// add handlers before because jobs can mutate the object
		if err := finalObj.addMessageHandlers(state.Global.Ctx); err != nil {
			return nil, err
		}
		if err := finalObj.instantiateLifetimeJobs(state.Global.Ctx); err != nil {
			return nil, err
		}

		if indexKey != 0 {
			finalObj.implicitPropCount = indexKey
		}

		initializeMetaproperties(finalObj, n.MetaProperties)
		return finalObj, nil
	case *parse.RecordLiteral:
		finalRecord := &Record{}

		indexKey := 0
		for _, p := range n.Properties {
			v, err := TreeWalkEval(p.Value, state)
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
		finalRecord.sortProps()

		if indexKey != 0 {
			finalRecord.implicitPropCount = indexKey
		}

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

			keyRepr := string(GetRepresentation(k.(Serializable), state.Global.Ctx))
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

				_, err := TreeWalkEval(n.Body, state)
				if err != nil {
					return nil, err
				}
				if state.returnValue != nil {
					return Nil, nil
				}
				switch state.iterationChange {
				case BreakIteration:
					state.iterationChange = NoIterationChange
					break iterable_iteration
				case ContinueIteration:
					state.iterationChange = NoIterationChange
					index++
					continue iterable_iteration
				case PruneWalk:
					return Nil, nil
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
					return nil, state.Global.Ctx.Err()
				default:
				}

				var next Value
				var streamErr error

				if chunked {
					sizeRange := NewIncludedEndIntRange(DEFAULT_MIN_STREAM_CHUNK_SIZE, DEFAULT_MAX_STREAM_CHUNK_SIZE)
					next, streamErr = stream.WaitNextChunk(state.Global.Ctx, nil, sizeRange, STREAM_ITERATION_WAIT_TIMEOUT)
				} else {
					next, streamErr = stream.WaitNext(state.Global.Ctx, nil, STREAM_ITERATION_WAIT_TIMEOUT)
				}

				if streamErr == nil || (chunked && next.(*DataChunk) != nil) {
					scope[eVarname] = next

					//evalute body & handle return/break/continue/prune

					_, err := TreeWalkEval(n.Body, state)
					if err != nil {
						return nil, err
					}
					if state.returnValue != nil {
						return Nil, nil
					}
					switch state.iterationChange {
					case BreakIteration:
						state.iterationChange = NoIterationChange
						break stream_iteration
					case ContinueIteration:
						state.iterationChange = NoIterationChange
						continue stream_iteration
					case PruneWalk:
						return Nil, nil
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
					return nil, streamErr
				}
			}
		} else {
			return nil, fmt.Errorf("cannot iterate %#v", iteratedValue)
		}
		return Nil, nil
	case *parse.WalkStatement:
		walkable, err := TreeWalkEval(n.Walked, state)
		if err != nil {
			return nil, err
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
			return nil, err
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
				return nil, blkErr
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
			}
		}

		state.iterationChange = NoIterationChange

		return nil, err
	case *parse.SwitchStatement:
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
					_, err := TreeWalkEval(switchCase.Block, state)
					if err != nil {
						return nil, err
					}
					goto switch_end
				}
			}
		}
		//if we are here there was no match
		if len(n.DefaultCases) > 0 {
			_, err := TreeWalkEval(n.DefaultCases[0].Block, state)
			if err != nil {
				return nil, err
			}
		}
	switch_end:

		return Nil, nil
	case *parse.MatchStatement:
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

						_, err := TreeWalkEval(matchCase.Block, state)
						if err != nil {
							return nil, err
						}
						goto match_end
					}

				} else if pattern.Test(state.Global.Ctx, discriminant) {
					_, err := TreeWalkEval(matchCase.Block, state)
					if err != nil {
						return nil, err
					}
					goto match_end
				}
			}
		}

		//if we are here there was no match
		if len(n.DefaultCases) > 0 {
			_, err := TreeWalkEval(n.DefaultCases[0].Block, state)
			if err != nil {
				return nil, err
			}
		}
	match_end:

		return Nil, nil
	case *parse.UnaryExpression:

		operand, err := TreeWalkEval(n.Operand, state)
		if err != nil {
			return nil, err
		}
		switch n.Operator {
		case parse.NumberNegate:
			if i, ok := operand.(Int); ok {
				if i == -i {
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

		left, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
		}

		right, err := TreeWalkEval(n.Right, state)
		if err != nil {
			return nil, err
		}

		switch n.Operator {
		case parse.Add, parse.Sub, parse.Mul, parse.Div, parse.GreaterThan, parse.GreaterOrEqual, parse.LessThan, parse.LessOrEqual:

			if _, ok := left.(Int); ok {
				switch n.Operator {
				case parse.Add:
					return intAdd(left.(Int), right.(Int))
				case parse.Sub:
					return intSub(left.(Int), right.(Int))
				case parse.Mul:
					return intMul(left.(Int), right.(Int))
				case parse.Div:
					return intDiv(left.(Int), right.(Int))
				case parse.GreaterThan:
					return Bool(left.(Int) > right.(Int)), nil
				case parse.GreaterOrEqual:
					return Bool(left.(Int) >= right.(Int)), nil
				case parse.LessThan:
					return Bool(left.(Int) < right.(Int)), nil
				case parse.LessOrEqual:
					return Bool(left.(Int) <= right.(Int)), nil
				}
			}

			leftF := left.(Float)
			rightF := right.(Float)

			if math.IsNaN(float64(leftF)) || math.IsInf(float64(leftF), 0) {
				return nil, ErrNaNinfinityOperand
			}

			if math.IsNaN(float64(rightF)) || math.IsInf(float64(rightF), 0) {
				return nil, ErrNaNinfinityOperand
			}

			switch n.Operator {
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
			case parse.GreaterThan:
				return Bool(leftF > rightF), nil
			case parse.GreaterOrEqual:
				return Bool(leftF >= rightF), nil
			case parse.LessThan:
				return Bool(leftF < rightF), nil
			case parse.LessOrEqual:
				return Bool(leftF <= rightF), nil
			}
			panic(ErrUnreachable)
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
				return Bool(rightVal.Contains(state.Global.Ctx, left)), nil
			default:
				return nil, fmt.Errorf("invalid binary expression: cannot check if value is inside a %T", rightVal)
			}
		case parse.NotIn:
			switch rightVal := right.(type) {
			case Container:
				return !Bool(rightVal.Contains(state.Global.Ctx, left)), nil
			default:
				return nil, fmt.Errorf("invalid binary expression: cannot check if value is inside a(n) %T", rightVal)
			}

		case parse.Keyof:
			key, ok := left.(Str)
			if !ok {
				return nil, fmt.Errorf("invalid binary expression: keyof: left operand is not a string, but a %T", left)
			}

			switch rightVal := right.(type) {
			case *Object:
				return Bool(rightVal.HasProp(state.Global.Ctx, string(key))), nil
			default:
				return nil, fmt.Errorf("invalid binary expression: cannot check if non object has a key: %T", rightVal)
			}
		case parse.Range, parse.ExclEndRange:
			switch left.(type) {
			case Int:
				return IntRange{
					inclusiveEnd: n.Operator == parse.Range,
					Start:        int64(left.(Int)),
					End:          int64(right.(Int)),
					Step:         1,
				}, nil
			case Float:
				return FloatRange{
					inclusiveEnd: n.Operator == parse.Range,
					Start:        float64(left.(Float)),
					End:          float64(right.(Float)),
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
			return Bool(strings.Contains(right.(WrappedString).UnderlyingString(), left.(WrappedString).UnderlyingString())), nil
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
		default:
			return nil, errors.New("invalid binary operator " + strconv.Itoa(int(n.Operator)))
		}
	case *parse.UpperBoundRangeExpression:
		upperBound, err := TreeWalkEval(n.UpperBound, state)
		if err != nil {
			return nil, err
		}

		switch v := upperBound.(type) {
		case Int:
			return IntRange{
				unknownStart: true,
				inclusiveEnd: true,
				End:          int64(v),
				Step:         1,
			}, nil
		case Float:
			return FloatRange{
				unknownStart: true,
				inclusiveEnd: true,
				End:          float64(v),
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
			inclusiveEnd: true,
			Start:        n.LowerBound.Value,
			End:          upperBound,
			Step:         1,
		}, nil
	case *parse.FloatRangeLiteral:
		upperBound := float64(math.MaxFloat64)

		if n.UpperBound != nil {
			upperBound = n.UpperBound.(*parse.FloatLiteral).Value
		}

		return FloatRange{
			unknownStart: false,
			inclusiveEnd: true,
			Start:        n.LowerBound.Value,
			End:          upperBound,
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

		var staticData *FunctionStaticData
		var capturedGlobals []capturedGlobal
		if state.Global.StaticCheckData != nil {
			staticData = state.Global.StaticCheckData.GetFnData(n)
		}

		return &InoxFunction{
			Node:                   n,
			treeWalkCapturedLocals: capturedLocals,
			symbolicValue:          symbolicInoxFunc,
			staticData:             staticData,
			capturedGlobals:        capturedGlobals,
		}, nil
	case *parse.FunctionDeclaration:
		funcName := n.Name.Name
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
		return NewExactValuePattern(v.(Serializable)), nil
	case *parse.LazyExpression:
		return AstNode{
			Node:  n.Expression,
			chunk: state.currentChunk(),
		}, nil
	case *parse.SelfExpression:
		return state.self, nil
	case *parse.MemberExpression:
		left, err := TreeWalkEval(n.Left, state)
		if err != nil {
			return nil, err
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

		return Bool(coerceToBool(valueToConvert)), nil
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

		state.Global.Ctx.AddNamedPattern(n.Left.Name, right)
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

		state.Global.Ctx.AddPatternNamespace(n.Left.Name, ns)
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
			entryPatterns: make(map[string]Pattern),
			inexact:       !n.Exact,
		}
		for _, p := range n.Properties {
			name := p.Name()
			var err error
			pattern.entryPatterns[name], err = evalPatternNode(p.Value, state)
			if p.Optional {
				if pattern.optionalEntries == nil {
					pattern.optionalEntries = make(map[string]struct{}, 1)
				}
				pattern.optionalEntries[name] = struct{}{}
			}
			if err != nil {
				return nil, fmt.Errorf("failed to compile object pattern literal, error when evaluating value for '%s': %s", name, err.Error())
			}
		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := evalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			object := evaluatedElement.(*ObjectPattern)

			for name, vpattern := range object.entryPatterns {
				//priority to property pattern defined earlier
				if _, alreadyPresent := pattern.entryPatterns[name]; alreadyPresent {
					continue
				}

				pattern.entryPatterns[name] = vpattern
				if _, ok := object.optionalEntries[name]; !ok {
					continue
				}
				//set as optional
				if pattern.optionalEntries == nil {
					pattern.optionalEntries = map[string]struct{}{}
				}
				pattern.optionalEntries[name] = struct{}{}
			}
		}

		return pattern, nil
	case *parse.RecordPatternLiteral:
		pattern := &RecordPattern{
			entryPatterns: make(map[string]Pattern),
			inexact:       !n.Exact,
		}
		for _, p := range n.Properties {
			name := p.Name()
			var err error
			pattern.entryPatterns[name], err = evalPatternNode(p.Value, state)
			if p.Optional {
				if pattern.optionalEntries == nil {
					pattern.optionalEntries = make(map[string]struct{}, 1)
				}
				pattern.optionalEntries[name] = struct{}{}
			}
			if err != nil {
				return nil, fmt.Errorf("failed to compile record pattern literal, error when evaluating value for '%s': %s", name, err.Error())
			}
		}

		for _, el := range n.SpreadElements {
			evaluatedElement, err := evalPatternNode(el.Expr, state)
			if err != nil {
				return nil, err
			}

			object := evaluatedElement.(*RecordPattern)

			for name, vpattern := range object.entryPatterns {
				//priority to property pattern defined earlier
				if _, alreadyPresent := pattern.entryPatterns[name]; alreadyPresent {
					continue
				}
				pattern.entryPatterns[name] = vpattern
				if _, ok := object.optionalEntries[name]; !ok {
					continue
				}
				//set as optional
				if pattern.optionalEntries == nil {
					pattern.optionalEntries = map[string]struct{}{}
				}
				pattern.optionalEntries[name] = struct{}{}
			}
		}

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

		return concatValues(state.Global.Ctx, values)
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
			panic(&AssertionError{msg: "assertion is false", data: data})
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

		suite, err := NewTestSuite(meta, chunk, state.Global)
		if err != nil {
			return nil, err
		}

		if n.IsStatement {
			lthread, err := suite.Run(state.Global.Ctx)
			if err != nil {
				return nil, err
			}
			return lthread.WaitResult(state.Global.Ctx)
		} else {
			return suite, nil
		}
	case *parse.TestCaseExpression:
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

		testCase, err := NewTestCase(meta, chunk, state.Global)
		if err != nil {
			return nil, err
		}

		if n.IsStatement {
			lthread, err := testCase.Run(state.Global.Ctx)
			if err != nil {
				return nil, err
			}
			return lthread.WaitResult(state.Global.Ctx)
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

		parsedChunk := &parse.ParsedChunk{
			Node:   chunk,
			Source: state.Global.Module.MainChunk.Source,
		}

		jobMod := &Module{
			ModuleKind:       LifetimeJobModule,
			MainChunk:        parsedChunk,
			ManifestTemplate: parsedChunk.Node.Manifest,
		}

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
				sliceValues = append(sliceValues, Str(s.Value))
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
		return Str(selector.String()), nil
	case parse.SimpleValueLiteral:
		return evalSimpleValueLiteral(n, state.Global)
	case *parse.XMLExpression:
		xmlElem, err := TreeWalkEval(n.Element, state)
		if err != nil {
			return nil, err
		}

		namespace, err := TreeWalkEval(n.Namespace, state)
		if err != nil {
			return nil, err
		}

		ns := namespace.(*Namespace)
		factory := ns.Prop(state.Global.Ctx, symbolic.FROM_XML_FACTORY_NAME).(*GoFunction)

		return factory.Call([]any{xmlElem}, state.Global, nil, false, false)
	case *parse.XMLElement:
		name := n.Opening.GetName()

		var attrs []XMLAttribute

		for _, attrNode := range n.Opening.Attributes {
			attr := XMLAttribute{name: attrNode.GetName()}
			if attrNode.Value != nil {
				attrValue, err := TreeWalkEval(attrNode.Value, state)
				if err != nil {
					return nil, err
				}
				attr.value = attrValue
			} else {
				attr.value = DEFAULT_XML_ATTR_VALUE
			}

			attrs = append(attrs, attr)
		}

		var children []Value

		for _, child := range n.Children {
			childValue, err := TreeWalkEval(child, state)
			if err != nil {
				return nil, err
			}
			children = append(children, childValue)
		}

		return NewXmlElement(name, attrs, children), nil
	case *parse.XMLText:
		return Str(html.EscapeString(n.Value)), nil
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
	default:
		return nil, fmt.Errorf("cannot evaluate %#v (%T)\n%s", node, node, debug.Stack())
	}
}

type TreeWalkCall struct {
	callee             Value
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

	state.PushScope()
	prevSelf := state.self
	state.self = self

	if capturedGlobals != nil || isSharedFunction {
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
		if ok, err := IsResultWithError(ret); ok {
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
	case *parse.QuotedStringLiteral:
		return NewExactStringPattern(Str(v.Value)), nil
	case *parse.RuneLiteral:
		return NewExactStringPattern(Str(v.Value)), nil
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

			if element.Ocurrence == parse.ExactlyOneOcurrence {
				subpatterns = append(subpatterns, patternElement)
			} else {
				subpatterns = append(subpatterns, &RepeatedPatternElement{
					//regexp:            regexp.MustCompile(subpatternRegex),
					ocurrenceModifier: element.Ocurrence,
					exactCount:        element.ExactOcurrenceCount,
					element:           patternElement,
				})
			}
		}

		return NewSequenceStringPattern(v, subpatterns, groupNames)
	case *parse.RegularExpressionLiteral:
		return NewRegexPattern(v.Value), nil
	default:
		return nil, fmt.Errorf("cannot evalute string pattern element: %T", v)
	}
}
