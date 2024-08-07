package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"

	"slices"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/maps"
)

var (
	DEFAULT_SWITCH_MATCH_EXPR_RESULT = Nil
)

type EvalCheckInput struct {
	Node   *ast.Chunk
	Module *Module

	//should not be set if UseBaseGlobals is true
	Globals                        map[string]ConcreteGlobalValue
	AdditionalSymbolicGlobalConsts map[string]Value

	UseBaseGlobals                bool
	SymbolicBaseGlobals           map[string]Value
	SymbolicBasePatterns          map[string]Pattern
	SymbolicBasePatternNamespaces map[string]*PatternNamespace

	IsShellChunk         bool
	ShellLocalVars       map[string]any
	ShellTrustedCommands []string
	Context              *Context

	importPositions     []sourcecode.PositionRange
	initialSymbolicData *Data
}

// EvalCheck performs various checks on an AST, most checks are type checks.
// If the returned data is not nil the error is nil or is the combination of checking errors, the list of checking errors
// is stored in the symbolic data.
// If the returned data is nil the error is an unexpected one (it is not about bad code).
// StaticCheck() should be runned before this function.
func EvalCheck(input EvalCheckInput) (*Data, error) {

	state := newSymbolicState(input.Context, input.Module.mainChunk)
	state.Module = input.Module
	state.baseGlobals = input.SymbolicBaseGlobals
	state.basePatterns = input.SymbolicBasePatterns
	state.basePatternNamespaces = input.SymbolicBasePatternNamespaces
	state.importPositions = slices.Clone(input.importPositions)
	state.shellTrustedCommands = input.ShellTrustedCommands

	startingConcreteContext := input.Context.startingConcreteContext
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
			symbolicVal, err := extData.ToSymbolicValue(startingConcreteContext, concreteGlobal.Value, false)
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
			symbolicVal, err := extData.ToSymbolicValue(startingConcreteContext, v, false)
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

func SymbolicEval(node ast.Node, state *State) (result Value, finalErr error) {
	return symbolicEval(node, state)
}

func symbolicEval(node ast.Node, state *State) (result Value, finalErr error) {
	return _symbolicEval(node, state, evalOptions{})
}

type evalOptions struct {
	ignoreNodeValue bool
	expectedValue   Value

	//used to report info to the caller, the primary use is to avoid showing irrelevant errors
	actualValueMismatch *bool

	//used to report info to the caller, the primary use is to avoid showing irrelevant errors
	hasShallowError *bool

	reEval bool

	//used for checking that double-colon expressions are not misplaced
	doubleColonExprAncestorChain []ast.Node

	fallbackResult Value //defaults to ANY, value returned for *ast.MissingExpression.

	neverModifiedArgument bool
}

func (opts evalOptions) setActualValueMismatchIfNotNil() {
	if opts.actualValueMismatch != nil {
		*opts.actualValueMismatch = true
	}
}

func (opts evalOptions) setHasShallowErrors() {
	if opts.hasShallowError != nil {
		*opts.hasShallowError = true
	}
}

func _symbolicEval(node ast.Node, state *State, options evalOptions) (result Value, finalErr error) {
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

		//set most specific node value in most cases

		if utils.Implements[*ast.EmbeddedModule](node) {
			return
		}

		if !options.ignoreNodeValue && !options.reEval && finalErr == nil && result != nil && state.symbolicData != nil {
			state.SetMostSpecificNodeValue(node, result)
		}
		if options.expectedValue != nil && state.symbolicData != nil {
			state.symbolicData.SetExpectedNodeValueInfo(node, ExceptedValueInfo{
				value: options.expectedValue,
			})
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
		case *ast.ObjectLiteral, *ast.RecordLiteral, *ast.DictionaryLiteral,
			*ast.ListLiteral, *ast.TupleLiteral:
		default:
			nodeValue, ok := state.symbolicData.GetMostSpecificNodeValue(node)
			if !ok {
				return nil, fmt.Errorf("no value for node of type %T", nodeValue)
			}
			return nodeValue, nil
		}
	}

	state.resetTestCallMsgBuffers()

	switch n := node.(type) {
	case *ast.Chunk:
		return evalChunk(n, state)
	case *ast.BooleanLiteral:
		return NewBool(n.Value), nil
	case *ast.IntLiteral:
		return &Int{value: n.Value, hasValue: true}, nil
	case *ast.FloatLiteral:
		return NewFloat(n.Value), nil
	case *ast.PortLiteral:
		return &Port{}, nil
	case *ast.QuantityLiteral:
		v, err := extData.GetQuantity(n.Values, n.Units)
		if err != nil {
			state.addError(makeSymbolicEvalErrorFromError(node, state, err))
			return ANY, nil
		}
		return extData.ToSymbolicValue(state.ctx.startingConcreteContext, v, false)
	case *ast.YearLiteral:
		return NewYear(n.Value), nil
	case *ast.DateLiteral:
		return NewDate(n.Value), nil
	case *ast.DateTimeLiteral:
		return NewDateTime(n.Value), nil
	case *ast.RateLiteral:
		v, err := extData.GetRate(n.Values, n.Units, n.DivUnit)
		if err != nil {
			state.addError(makeSymbolicEvalErrorFromError(node, state, err))
			return ANY, nil
		}
		return extData.ToSymbolicValue(state.ctx.startingConcreteContext, v, false)
	case *ast.DoubleQuotedStringLiteral:
		return NewString(n.Value), nil
	case *ast.UnquotedStringLiteral:
		return NewString(n.Value), nil
	case *ast.MultilineStringLiteral:
		return NewString(n.Value), nil
	case *ast.RuneLiteral:
		return NewRune(n.Value), nil
	case *ast.IdentifierLiteral:
		return evalIdentifier(n, state, options)
	case *ast.UnambiguousIdentifierLiteral:
		return &Identifier{name: n.Name}, nil
	case *ast.PropertyNameLiteral:
		return &PropertyName{name: n.Name}, nil
	case *ast.LongValuePathLiteral:
		var segments []ValuePathSegment
		for _, segmentNode := range n.Segments {
			if segmentNode.Base().Err != nil {
				return ANY_LONG_VALUE_PATH, nil
			}
			segment, err := symbolicEval(segmentNode, state)
			if err != nil {
				return nil, err
			}
			segments = append(segments, segment.(ValuePathSegment))
		}
		return NewLongValuePath(segments...), nil
	case *ast.AbsolutePathLiteral:
		if strings.HasSuffix(n.Value, "/...") || strings.Contains(n.Value, "*") {
			state.addWarning(makeSymbolicEvalWarning(node, state, fmtDidYouForgetLeadingPercent(n.Value)))
		}
		return NewPath(n.Value), nil
	case *ast.RelativePathLiteral:
		if strings.HasSuffix(n.Value, "/...") || strings.Contains(n.Value, "*") {
			state.addWarning(makeSymbolicEvalWarning(node, state, fmtDidYouForgetLeadingPercent(n.Value)))
		}
		return NewPath(n.Value), nil
	case *ast.AbsolutePathPatternLiteral:
		return NewPathPattern(n.Value), nil
	case *ast.RelativePathPatternLiteral:
		return NewPathPattern(n.Value), nil
	case *ast.NamedSegmentPathPatternLiteral:
		return &NamedSegmentPathPattern{node: n}, nil
	case *ast.RegularExpressionLiteral:
		return NewRegexPattern(n.Value), nil
	case *ast.PathSlice, *ast.PathPatternSlice:
		return ANY_STRING, nil
	case *ast.URLQueryParameterValueSlice:
		return ANY_STRING, nil
	case *ast.FlagLiteral:
		if _, hasVar := state.get(n.Name); hasVar {
			state.addWarning(makeSymbolicEvalWarning(node, state, THIS_VAL_IS_AN_OPT_LIT_DID_YOU_FORGET_A_SPACE))
		}

		return NewOption(n.Name, TRUE), nil
	case *ast.OptionExpression:
		v, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		return NewOption(n.Name, v), nil
	case *ast.AbsolutePathExpression, *ast.RelativePathExpression:
		var slices []ast.Node

		switch pexpr := n.(type) {
		case *ast.AbsolutePathExpression:
			slices = pexpr.Slices
		case *ast.RelativePathExpression:
			slices = pexpr.Slices
		}

		for _, node := range slices {
			_, isStaticPathSlice := node.(*ast.PathSlice)
			_, err := _symbolicEval(node, state, evalOptions{ignoreNodeValue: isStaticPathSlice})
			if err != nil {
				return nil, err
			}

			if isStaticPathSlice {
				state.SetMostSpecificNodeValue(node, ANY_PATH)
			}
		}

		return ANY_PATH, nil
	case *ast.PathPatternExpression:
		return NewPathPatternFromNode(n, state.currentChunk().Node), nil
	case *ast.URLLiteral:
		return NewUrl(n.Value), nil
	case *ast.SchemeLiteral:
		return NewScheme(n.ValueString()), nil
	case *ast.HostLiteral:
		return NewHost(n.Value), nil
	case *ast.HostPatternLiteral:
		return NewHostPattern(n.Value), nil
	case *ast.URLPatternLiteral:
		return NewUrlPattern(n.Value), nil
	case *ast.URLExpression:
		return evalURLExpression(n, state, options)
	case *ast.NilLiteral:
		return &NilT{}, nil
	case *ast.SelfExpression:
		v, ok := state.getSelf()
		if !ok {
			return nil, errors.New("no self")
		}
		return v, nil
	case *ast.Variable:
		return evalVariable(n, state, options)
	case *ast.ReturnStatement:
		return evalReturnStatement(n, state)
	case *ast.CoyieldStatement:
		if n.Expr == nil {
			return nil, nil
		}

		_, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return nil, nil
	case *ast.YieldStatement:
		return evalYieldStatement(n, state)
	case *ast.BreakStatement:
		return nil, nil
	case *ast.ContinueStatement:
		return nil, nil
	case *ast.PruneStatement:
		return nil, nil
	case *ast.CallExpression:
		return callSymbolicFunc(symbolicFunctionCall{
			callNode:      n,
			calleeNode:    n.Callee,
			state:         state,
			argNodes:      n.Arguments,
			must:          n.Must,
			cmdLineSyntax: n.CommandLikeSyntax,
		})
	case *ast.PatternCallExpression:
		return evalPatternCallExpression(n, state)
	case *ast.PipelineStatement, *ast.PipelineExpression:
		var stages []*ast.PipelineStage

		isExpr := false

		switch e := n.(type) {
		case *ast.PipelineStatement:
			stages = e.Stages
		case *ast.PipelineExpression:
			stages = e.Stages
			isExpr = true
		}

		defer func() {
			state.removeLocal("")
		}()

		var err error

		firstStageResult, err := symbolicEval(stages[0].Expr, state)
		if err != nil {
			return nil, err
		}

		state.overrideLocal("", firstStageResult)

		for _, stage := range stages[1:] {

			switch stage.Expr.(type) {
			case *ast.IdentifierLiteral, *ast.IdentifierMemberExpression:
				prevResult := utils.MustGet(state.getLocal("")).value
				stageResult, err := callSymbolicFunc(symbolicFunctionCall{
					callNode:      stage.Expr,
					calleeNode:    stage.Expr,
					argNodes:      nil,
					argValues:     []Value{prevResult},
					state:         state,
					must:          true,
					cmdLineSyntax: false,
				})
				if err != nil {
					return nil, err
				}
				state.overrideLocal("", stageResult)
			default:
				stageResult, err := symbolicEval(stage.Expr, state)
				if err != nil {
					return nil, err
				}
				state.overrideLocal("", stageResult)
			}

		}

		if isExpr {
			return utils.MustGet(state.getLocal("")).value, nil
		}

		return nil, nil
	case *ast.LocalVariableDeclarations:
		return nil, evalLocalVariableDeclarations(n, state)
	case *ast.GlobalVariableDeclarations:
		return nil, evalGlobalVariableDeclarations(n, state)
	case *ast.Assignment:
		return evalAssignment(n, state)
	case *ast.MultiAssignment:
		return evalMultiAssignment(n, state)
	case *ast.EmbeddedModule:
		return &AstNode{Node: n.ToChunk()}, nil
	case *ast.Block:
		for _, stmt := range n.Statements {
			res, err := symbolicEval(stmt, state)
			if err != nil {
				return nil, err
			}
			checkCallExprWithUnhandledError(stmt, res, state)
		}
		return nil, nil
	case *ast.SynchronizedBlockStatement:
		return evalSynchronizedBlockStatement(n, state)
	case *ast.PermissionDroppingStatement:
		return nil, nil
	case *ast.InclusionImportStatement:
		return evalInclusionImportStatement(n, state)
	case *ast.ImportStatement:
		return evalImportStatement(n, state)
	case *ast.SpawnExpression:
		return evalSpawnExpression(n, state)
	case *ast.MappingExpression:
		return evalMappingExpression(n, state)
	case *ast.TreedataLiteral:
		return evalTreedataLiteral(n, state, options)
	case *ast.TreedataEntry:
		return evalTreedataEntry(n, state, options)
	case *ast.TreedataPair:
		return evalTreedataPair(n, state, options)
	case *ast.ComputeExpression:
		fork := state.fork()

		v, err := symbolicEval(n.Arg, fork)
		if err != nil {
			return nil, err
		}

		if !IsSimpleSymbolicInoxVal(v) {
			state.addError(MakeSymbolicEvalError(n.Arg, state, INVALID_KEY_IN_COMPUTE_EXPRESSION_ONLY_SIMPLE_VALUE_ARE_SUPPORTED))
		}

		return ANY, nil
	case *ast.ObjectLiteral:
		return evalObjectLiteral(n, state, options)
	case *ast.RecordLiteral:
		return evalRecordLiteral(n, state, options)
	case *ast.ListLiteral:
		return evalListLiteral(n, state, options)
	case *ast.TupleLiteral:
		return evalTupleLiteral(n, state, options)
	case *ast.DictionaryLiteral:
		return evalDictionaryLiteral(n, state, options)
	case *ast.IfStatement:
		return evalIfStatement(n, state)
	case *ast.IfExpression:
		return evalIfExpression(n, state, options)
	case *ast.ForStatement, *ast.ForExpression:
		return evalForStatementAndExpr(n, state)
	case *ast.WalkStatement, *ast.WalkExpression:
		return evalWalkStatementAndExpr(n, state)
	case *ast.SwitchStatement:
		return evalSwitchStatement(n, state)
	case *ast.SwitchExpression:
		return evalSwitchExpression(n, state, options)
	case *ast.MatchStatement:
		return evalMatchStatement(n, state)
	case *ast.MatchExpression:
		return evalMatchExpression(n, state, options)
	case *ast.UnaryExpression:
		return evalUnaryExpression(n, state, options)
	case *ast.BinaryExpression:
		return evalBinaryExpression(n, state, options)
	case *ast.UpperBoundRangeExpression:
		upperBound, err := symbolicEval(n.UpperBound, state)
		if err != nil {
			return nil, err
		}

		switch upperBound.(type) {
		case *Int:
			return ANY_INT_RANGE, nil
		case *Float:
			return ANY_FLOAT_RANGE, nil
		default:
			return ANY_QUANTITY_RANGE, nil
		}
	case *ast.IntegerRangeLiteral:
		if n.LowerBound == nil || n.LowerBound.Err != nil {
			return ANY_INT_RANGE, nil
		}

		if n.UpperBound != nil && n.UpperBound.Base().Err != nil {
			return ANY_INT_RANGE, nil
		}

		lowerBound := NewInt(n.LowerBound.Value)
		upperBound := MAX_INT

		intLit, ok := n.UpperBound.(*ast.IntLiteral)

		if ok {
			upperBound = NewInt(intLit.Value)
		}

		return &IntRange{
			hasValue: true,
			start:    lowerBound,
			end:      upperBound,
		}, nil
	case *ast.FloatRangeLiteral:
		if n.LowerBound == nil || n.LowerBound.Err != nil {
			return ANY_FLOAT_RANGE, nil
		}

		if n.UpperBound != nil && n.UpperBound.Base().Err != nil {
			return ANY_FLOAT_RANGE, nil
		}

		lowerBound := NewFloat(n.LowerBound.Value)
		upperBound := MAX_FLOAT

		floatLit, ok := n.UpperBound.(*ast.FloatLiteral)

		if ok {
			upperBound = NewFloat(floatLit.Value)
		}

		return &FloatRange{
			hasValue:     true,
			start:        lowerBound,
			end:          upperBound,
			inclusiveEnd: true,
		}, nil
	case *ast.QuantityRangeLiteral:
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

			if !element.Test(upperBound, RecTestCallState{}) {
				state.addError(MakeSymbolicEvalError(n.UpperBound, state, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_OF_SAME_TYPE_AS_LOWER_BOUND))
			}
		}

		return NewQuantityRange(element.(Serializable)), nil
	case *ast.RuneRangeExpression:
		return ANY_RUNE_RANGE, nil
	case *ast.FunctionExpression:
		return evalFunctionExpression(n, state, options)
	case *ast.FunctionDeclaration:
		return evalFunctionDeclaration(n, state, options)
	case *ast.ReadonlyPatternExpression:
		pattern, err := evalPatternNode(n.Pattern, state)
		if err != nil {
			return nil, err
		}

		if !pattern.SymbolicValue().IsMutable() {
			return pattern, nil
		}

		potentiallyReadonlyPattern, ok := pattern.(PotentiallyReadonlyPattern)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Pattern, state, PATTERN_IS_NOT_CONVERTIBLE_TO_READONLY_VERSION))
			return pattern, nil
		}
		readonly, err := potentiallyReadonlyPattern.ToReadonlyPattern()
		if err != nil {
			state.addError(makeSymbolicEvalErrorFromError(n.Pattern, state, err))
			return pattern, nil
		}
		return readonly, nil
	case *ast.FunctionPatternExpression:
		return evalFunctionPatternExpression(n, state)
	case *ast.PatternConversionExpression:
		return evalPatternNode(n.Value, state)
	case *ast.QuotedExpression:
		return &AstNode{Node: n}, nil
	case *ast.MemberExpression:
		left, err := _symbolicEval(n.Left, state, evalOptions{
			doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), node),
		})
		if err != nil {
			return nil, err
		}

		if n.PropertyName == nil { //parsing error
			return ANY, nil
		}

		accessKind := unspecifiedMemberAccess
		if n.Optional {
			accessKind = optionalMemberAccess
		}

		val := symbolicMemb(left, n.PropertyName.Name, accessKind, n, state)

		state.SetMostSpecificNodeValue(n.PropertyName, val)

		return val, nil
	case *ast.ComputedMemberExpression:
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
			state.addError(MakeSymbolicEvalError(n.PropertyName, state, fmtComputedPropNameShouldBeAStringNotA(computedPropertyName)))
		}

		return ANY, nil
	case *ast.IdentifierMemberExpression:
		v, err := _symbolicEval(n.Left, state, evalOptions{
			doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), node),
		})
		if err != nil {
			return nil, err
		}

		if n.Err != nil {
			return ANY, nil
		}

		var prevIdent *ast.IdentifierLiteral
		for _, ident := range n.PropertyNames {
			if prevIdent != nil {
				state.SetMostSpecificNodeValue(prevIdent, v)
			}
			v = symbolicMemb(v, ident.Name, unspecifiedMemberAccess, n, state)
			prevIdent = ident
		}

		state.SetMostSpecificNodeValue(prevIdent, v)

		return v, nil
	case *ast.ExtractionExpression:
		return evalExtractionExpression(n, state, options)
	case *ast.DoubleColonExpression:
		return evalDoubleColonExpression(n, state, options)
	case *ast.IndexExpression:
		return evalIndexExpression(n, state, options)
	case *ast.SliceExpression:
		return evalSliceExpression(n, state, options)
	case *ast.KeyListExpression:
		list := &KeyList{}

		for _, key := range n.Keys {
			list.append(string(key.(ast.IIdentifierLiteral).Identifier()))
		}

		return list, nil
	case *ast.BooleanConversionExpression:
		_, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return ANY_BOOL, nil
	case *ast.PatternIdentifierLiteral:
		patt := state.ctx.ResolveNamedPattern(n.Name)
		if patt == nil {
			names := state.ctx.AllNamedPatternNames()

			var msg = ""
			closestString, _, ok := utils.FindClosestString(state.ctx.startingConcreteContext, names, n.Name, 2)

			if ok {
				msg = fmtPatternIsNotDeclaredYouProbablyMeant(n.Name, closestString)
			} else {
				msg = fmtPatternIsNotDeclared(n.Name)
			}

			if n.Unprefixed {
				if _, ok := state.get(n.Name); ok {
					msg += fmtDidYouMeanDollarName(n.Name)
				}
			}

			state.addError(MakeSymbolicEvalError(node, state, msg))
			return ANY_PATTERN, nil
		} else {
			return patt, nil
		}
	case *ast.PatternDefinition:
		pattern, err := evalPatternNode(n.Right, state)
		if err != nil {
			return nil, err
		}
		//TODO: add checks
		state.SetMostSpecificNodeValue(n.Left, pattern)

		name, ok := n.PatternName()
		if !ok {
			return nil, nil
		}

		if state.ctx.ResolveNamedPattern(name) == nil {
			state.ctx.AddNamedPattern(name, pattern, state.inPreinit, state.getCurrentChunkNodePositionOrZero(n.Left))
			state.symbolicData.SetContextData(n, state.ctx.currentData())
		} //else there is already static check error about the duplicate definition.

		return nil, nil
	case *ast.PatternNamespaceDefinition:
		return evalPatternNamespaceDefinition(n, state, options)
	case *ast.PatternNamespaceIdentifierLiteral:
		namespace := state.ctx.ResolvePatternNamespace(n.Name)
		if namespace == nil {
			state.addError(MakeSymbolicEvalError(node, state, fmtPatternNamespaceIsNotDeclared(n.Name)))
			return ANY_PATTERN_NAMESPACE, nil
		}
		return namespace, nil
	case *ast.PatternNamespaceMemberExpression:
		prevErrCount := len(state.errors())

		v, err := symbolicEval(n.Namespace, state)
		if err != nil {
			return nil, err
		}

		//if there was an error during the evaluation of the pattern namespace identifier,
		//don't add a useless error.
		if len(state.errors()) > prevErrCount && v == ANY_PATTERN_NAMESPACE {
			return ANY_PATTERN, nil
		}

		namespace := v.(*PatternNamespace)

		defer func() {
			if result != nil && state.symbolicData != nil {
				state.SetMostSpecificNodeValue(n.MemberName, result)
			}
		}()
		namespaceName := n.Namespace.Name
		memberName := n.MemberName.Name

		patt := namespace.entries[memberName] //it's not an issue if namespace.entries is nil

		if patt == nil {
			state.addError(MakeSymbolicEvalError(n.MemberName, state, fmtPatternNamespaceHasNotMember(namespaceName, memberName)))
			return ANY_PATTERN, nil
		}
		return patt, nil
	case *ast.OptionalPatternExpression:
		v, err := symbolicEval(n.Pattern, state)
		if err != nil {
			return nil, err
		}

		patt := v.(Pattern)
		if patt.TestValue(Nil, RecTestCallState{}) {
			state.addError(MakeSymbolicEvalError(node, state, CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL))
			return &AnyPattern{}, nil
		}

		return &OptionalPattern{pattern: patt}, nil
	case *ast.ComplexStringPatternPiece:
		return NewSequenceStringPattern(n, state.currentChunk().Node), nil
	case *ast.PatternUnion:
		patt := &UnionPattern{}

		for _, case_ := range n.Cases {
			patternElement, err := evalPatternNode(case_, state)
			if err != nil {
				return nil, fmt.Errorf("failed to symbolically compile a pattern element: %s", err.Error())
			}

			patt.cases = append(patt.cases, patternElement)
		}

		return patt, nil
	case *ast.ObjectPatternLiteral:
		return evalObjectPatternLiteral(n, state, options)
	case *ast.RecordPatternLiteral:
		return evalRecordPatternLiteral(n, state, options)
	case *ast.ListPatternLiteral:
		return evalListPatternLiteral(n, state, options)
	case *ast.TuplePatternLiteral:
		return evalTuplePatternLiteral(n, state, options)
	case *ast.OptionPatternLiteral:
		pattern, err := evalPatternNode(n.Value, state)
		if err != nil {
			return nil, err
		}

		return NewOptionPattern(n.Name, pattern), nil
	case *ast.ByteSliceLiteral:
		return ANY_BYTE_SLICE, nil
	case *ast.ConcatenationExpression:
		return evalConcatenationExpression(n, state, options)
	case *ast.AssertionStatement:
		ok, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}
		if _, isBool := ok.(*Bool); !isBool {
			state.addError(MakeSymbolicEvalError(node, state, fmtAssertedValueShouldBeBoolNot(ok)))
		}

		if binExpr, ok := n.Expr.(*ast.BinaryExpression); ok && state.symbolicData != nil {
			isVar := ast.IsAnyVariableIdentifier(binExpr.Left)
			if !isVar {
				return nil, nil
			}

			switch binExpr.Operator {
			case ast.Match:
				right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)

				if pattern, ok := right.(Pattern); ok {
					narrowChain(binExpr.Left, setExactValue, pattern.SymbolicValue(), state, 0)
				}
			}
		}
		state.SetLocalScopeData(n, state.currentLocalScopeData())
		state.SetGlobalScopeData(n, state.currentGlobalScopeData())

		return nil, nil
	case *ast.RuntimeTypeCheckExpression:
		options.ignoreNodeValue = true
		val, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		return val, nil
	// case *ast.TestSuiteExpression:
	// 	return evalTestsuiteExpression(n, state, options)
	// case *ast.TestCaseExpression:
	// 	return evalTestcaseExpression(n, state, options)
	case *ast.StringTemplateLiteral:
		return evalStringTemplateLiteral(n, state, options)
	case *ast.CssSelectorExpression:
		return ANY_STRING, nil
	case *ast.MarkupExpression:
		return evalMarkupExpression(n, state, options)
	case *ast.MarkupElement:
		return evalMarkupElement(n, state, options)
	case *ast.MarkupInterpolation:
		return evalMarkupInterpolation(n, state, options)
	case *ast.MarkupPatternExpression:
		return evalMarkupPatternExpression(n, state, options)
	case *ast.MarkupText:
		return ANY_STRING, nil
	case *ast.ExtendStatement:
		return evalExtendStatement(n, state, options)
	case *ast.UnknownNode:
		return ANY, nil
	case *ast.MissingExpression:
		if options.fallbackResult == nil {
			return ANY, nil
		}
		return options.fallbackResult, nil
	default:
		return nil, fmt.Errorf("cannot evaluate %#v (%T)\n%s", node, node, debug.Stack())
	}
}

// evalChunk evaluates the chunk of a file module, includable file, or an embedded module.
func evalChunk(n *ast.Chunk, state *State) (result Value, finalErr error) {
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
			state.SetMostSpecificNodeValue(decl.Left, constVal)
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
		if object, ok := manifestObject.(*Object); ok && !state.hasGlobal(globalnames.MOD_ARGS_VARNAME) {
			parameters := getModuleParameters(object, n.Manifest.Object.(*ast.ObjectLiteral))
			args := make(map[string]Value)
			paramsPattern := NewModuleParamsPattern(parameters)

			for _, param := range parameters {
				args[param.Name] = param.Pattern.SymbolicValue()
			}

			if !state.setGlobal(globalnames.MOD_ARGS_VARNAME, NewModuleArgs(paramsPattern, args), GlobalConst) {
				panic(ErrUnreachable)
			}
		}
	}

	state.SetGlobalScopeData(n, state.currentGlobalScopeData())
	state.symbolicData.SetContextData(n, state.ctx.currentData())

	// Predeclare all Inox functions that don't capture locals.
	for _, stmt := range n.Statements {
		decl, ok := stmt.(*ast.FunctionDeclaration)
		if ok && decl.Function != nil && len(decl.Function.CaptureList) == 0 {
			funcName, ok := decl.Name.(*ast.IdentifierLiteral)
			if !ok {
				continue //unquoted name
			}
			state.setGlobal(funcName.Name, &inoxFunctionToBeDeclared{decl: decl}, GlobalConst, funcName)
		}
	}

	moduleNode, isModule := n.ModuleNode()

	if isModule {
		defer func() {
			state.symbolicData.SetModuleResult(moduleNode, result)
		}()
	}

	//Evaluation of statements.
	if len(n.Statements) == 1 {
		res, err := symbolicEval(n.Statements[0], state)
		if err != nil {
			return nil, err
		}
		checkCallExprWithUnhandledError(n.Statements[0], res, state)

		if state.returnValue != nil {
			if state.conditionalReturn {
				return joinValues([]Value{state.returnValue, Nil}), nil
			}
			return state.returnValue, nil
		}

		if res == nil {
			return Nil, nil
		}

		return res, nil
	}

	for _, stmt := range n.Statements {
		res, err := symbolicEval(stmt, state)

		if err != nil {
			return nil, err
		}

		checkCallExprWithUnhandledError(stmt, res, state)

		if state.returnValue != nil && !state.conditionalReturn {
			//unconditional return
			return state.returnValue, nil
		}
	}

	if state.returnValue == nil {
		return Nil, nil
	}

	if state.conditionalReturn {
		return joinValues([]Value{state.returnValue, Nil}), nil
	}

	return state.returnValue, nil
}

func evalURLExpression(n *ast.URLExpression, state *State, options evalOptions) (_ Value, finalErr error) {

	var (
		host Value
		err  error
	)

	if ast.HasErrorAtAnyDepth(n) {
		return ANY_URL, nil
	}

	if hostExpr, ok := n.HostPart.(*ast.HostExpression); ok {
		scheme, err := symbolicEval(hostExpr.Scheme, state)
		if err != nil {
			return nil, err
		}

		networkHost, err := symbolicEval(hostExpr.Host, state)
		if err != nil {
			return nil, err
		}

		host = ANY_HOST

		if strLike, ok := networkHost.(StringLike); !ok {
			state.addError(MakeSymbolicEvalError(hostExpr.Host, state, fmtTypeOfNetworkHostInterpolationIsAnXButYWasExpected(networkHost, ANY_STR_LIKE)))
			options.setHasShallowErrors()
		} else if s := strLike.GetOrBuildString(); s.IsConcretizable() {
			host = NewHost(scheme.(*Scheme).value + "://" + s.Value())
		}
	} else {
		host, err = _symbolicEval(n.HostPart, state, evalOptions{ignoreNodeValue: true})
		if err != nil {
			return nil, err
		}
	}

	if !ImplOrMultivaluesImplementing[*Host](host) {
		state.addError(MakeSymbolicEvalError(n.HostPart, state, HOST_PART_SHOULD_HAVE_A_HOST_VALUE))
		state.SetMostSpecificNodeValue(n.HostPart, ANY_HOST)
		options.setHasShallowErrors()
	} else {
		state.SetMostSpecificNodeValue(n.HostPart, host)
	}

	//path evaluation

	for _, node := range n.Path {
		_, isStaticPathSlice := node.(*ast.PathSlice)
		_, err := _symbolicEval(node, state, evalOptions{ignoreNodeValue: isStaticPathSlice})
		if err != nil {
			return nil, err
		}

		if isStaticPathSlice {
			state.SetMostSpecificNodeValue(node, ANY_URL)
		}
	}

	//query evaluation

	for _, p := range n.QueryParams {
		param := p.(*ast.URLQueryParameter)

		state.SetMostSpecificNodeValue(param, ANY_URL)

		for _, slice := range param.Value {
			val, err := symbolicEval(slice, state)
			if err != nil {
				return nil, err
			}
			switch val.(type) {
			case StringLike, *Int, *Bool:
			default:
				state.addError(MakeSymbolicEvalError(p, state, fmtValueNotStringifiableToQueryParamValue(val)))
				options.setHasShallowErrors()
			}
		}
	}

	return ANY_URL, nil
}

func evalIdentifier(node *ast.IdentifierLiteral, state *State, evalOptions evalOptions) (Value, error) {
	info, ok := state.get(node.Name)
	if !ok {
		msg := fmtVarIsNotDeclared(node.Name)

		if pattern := state.ctx.ResolveNamedPattern(node.Name); pattern != nil {
			msg += fmtDidYouMeanPercentName(node.Name)
		}

		evalOptions.setHasShallowErrors()

		state.addError(MakeSymbolicEvalError(node, state, msg))
		return ANY, nil
	}

	inoxFn, ok := info.value.(*inoxFunctionToBeDeclared)
	if ok {
		//Properly declare the function.
		_, err := evalFunctionDeclaration(inoxFn.decl, state, evalOptions)
		if err != nil {
			return nil, fmt.Errorf("error while evaluating the function declaration: %w", err)
		}
		varInfo, ok := state.getGlobal(node.Name)
		if !ok {
			return nil, fmt.Errorf("error while evaluating the function declaration: %w", err)
		}
		return varInfo.value, nil
	}

	return info.value, nil
}

func evalVariable(node *ast.Variable, state *State, evalOptions evalOptions) (Value, error) {
	info, ok := state.getGlobal(node.Name)
	if ok {
		inoxFn, ok := info.value.(*inoxFunctionToBeDeclared)
		if ok {
			//Properly declare the function.
			_, err := evalFunctionDeclaration(inoxFn.decl, state, evalOptions)
			if err != nil {
				return nil, fmt.Errorf("error while evaluating the function declaration: %w", err)
			}
			varInfo, ok := state.getGlobal(node.Name)
			if !ok {
				return nil, fmt.Errorf("error while evaluating the function declaration: %w", err)
			}
			return varInfo.value, nil
		}

		return info.value, nil
	}

	info, ok = state.getLocal(node.Name)

	if ok {
		return info.value, nil
	}

	msg := fmtVarIsNotDeclared(node.Name)

	if pattern := state.ctx.ResolveNamedPattern(node.Name); pattern != nil {
		msg += fmt.Sprintf("; did you mean %%%s instead of $%s ?", node.Name, node.Name)
	}

	evalOptions.setHasShallowErrors()
	state.addError(MakeSymbolicEvalError(node, state, msg))
	return ANY, nil
}

func evalReturnStatement(n *ast.ReturnStatement, state *State) (_ Value, finalErr error) {

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

	if state.returnType != nil && !state.returnType.Test(v, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
		if !*deeperMismatch {
			msg, regions := fmtInvalidReturnValue(state.fmtHelper, v, state.returnType, state.testCallMessageBuffer)
			state.addError(MakeSymbolicEvalError(n, state, msg, regions...))
		}
		state.returnValue = state.returnType
	}

	if state.returnValue != nil {
		state.returnValue = joinValues([]Value{state.returnValue, v})
	} else {
		state.returnValue = v
	}

	state.conditionalReturn = false

	return nil, nil
}

func evalYieldStatement(n *ast.YieldStatement, state *State) (_ Value, finalErr error) {

	if n.Expr == nil {
		return nil, nil
	}

	var deeperMismatch *bool
	if state.yieldType != nil {
		deeperMismatch = new(bool)
	}

	value, err := _symbolicEval(n.Expr, state, evalOptions{
		expectedValue:       state.yieldType,
		actualValueMismatch: deeperMismatch,
	})

	if err != nil {
		return nil, err
	}
	v := value

	if state.yieldType != nil && !state.yieldType.Test(v, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
		if !*deeperMismatch {
			msg, regions := fmtInvalidReturnValue(state.fmtHelper, v, state.yieldType, state.testCallMessageBuffer)
			state.addError(MakeSymbolicEvalError(n, state, msg, regions...))
		}
		state.yieldedValue = state.yieldType
	}

	if state.yieldedValue != nil {
		state.yieldedValue = joinValues([]Value{state.yieldedValue, v})
	} else {
		state.yieldedValue = v
	}

	state.conditionalYield = false

	return nil, nil
}

func evalPatternCallExpression(n *ast.PatternCallExpression, state *State) (_ Value, finalErr error) {
	callee, err := symbolicEval(n.Callee, state)
	if err != nil {
		return nil, err
	}

	args := make([]Value, len(n.Arguments))

	errCount := len(state.errors())

	for i, argNode := range n.Arguments {
		arg, err := symbolicEval(argNode, state)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}

	if len(state.errors()) == errCount {
		patt, err := callee.(Pattern).Call(state.ctx, args, n)
		state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
			var location ast.Node = n
			if optionalLocation != nil {
				location = optionalLocation
			}

			state.addError(MakeSymbolicEvalError(location, state, msg))
		})
		state.consumeSymbolicGoFunctionWarnings(func(msg string) {
			state.addWarning(makeSymbolicEvalWarning(n, state, msg))
		})

		if err != nil {
			state.addError(MakeSymbolicEvalError(n, state, err.Error()))
			patt = ANY_PATTERN
		}
		return patt, nil
	}
	return ANY_PATTERN, nil
}

func evalLocalVariableDeclarations(n *ast.LocalVariableDeclarations, state *State) (finalErr error) {
	for _, decl := range n.Declarations {

		//First we evaluate the type annotation and the right hand side.

		var static Pattern
		var staticMatching Value

		if decl.Type != nil {

			type_, err := symbolicEval(decl.Type, state)
			if err != nil {
				return err
			}

			pattern, isPattern := type_.(Pattern)
			if isPattern {
				static = pattern
				staticMatching = static.SymbolicValue()
			} else {
				state.addError(MakeSymbolicEvalError(decl.Type, state, VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN))
			}
		}

		var (
			right Value
			err   error
		)

		if decl.Right != nil {
			deeperMismatch := false
			right, err = _symbolicEval(decl.Right, state, evalOptions{
				expectedValue:       staticMatching,
				actualValueMismatch: &deeperMismatch,
			})
			if err != nil {
				return err
			}

			if static != nil {

				if !static.TestValue(right, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
					if !deeperMismatch {
						msg, regions := fmtNotAssignableToVarOftype(state.fmtHelper, right, static, state.testCallMessageBuffer)
						state.addError(MakeSymbolicEvalError(decl.Right, state, msg, regions...))
					}
					right = ANY
				} else if holder, ok := right.(StaticDataHolder); ok {
					right, err = holder.AddStatic(static) //TODO: use path narowing, values should never be modified directly
					if err != nil {
						state.addError(MakeSymbolicEvalError(decl.Right, state, err.Error()))
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

		//Left hand side

		switch left := decl.Left.(type) {
		case *ast.IdentifierLiteral:
			ident := left
			state.setLocal(ident.Name, right, static, decl.Left)
			state.SetMostSpecificNodeValue(ident, right)
		case *ast.ObjectDestructuration:
			areLocalDeclarations := true
			err := evalObjectDestructuration(left, decl.Right, right, state, areLocalDeclarations)
			if err != nil {
				return err
			}
		default:
			continue
		}
	}
	state.SetLocalScopeData(n, state.currentLocalScopeData())
	return nil
}

func evalGlobalVariableDeclarations(n *ast.GlobalVariableDeclarations, state *State) (finalErr error) {
	for _, decl := range n.Declarations {

		//First we evaluate the type annotation and the right hand side.

		var static Pattern
		var staticMatching Value

		if decl.Type != nil {

			type_, err := symbolicEval(decl.Type, state)
			if err != nil {
				return err
			}

			pattern, isPattern := type_.(Pattern)
			if isPattern {
				static = pattern
				staticMatching = static.SymbolicValue()
			} else {
				state.addError(MakeSymbolicEvalError(decl.Type, state, VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN))
			}
		}

		var (
			right Value
			err   error
		)

		if decl.Right != nil {
			deeperMismatch := false
			right, err = _symbolicEval(decl.Right, state, evalOptions{expectedValue: staticMatching, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return err
			}

			if static != nil {
				if !static.TestValue(right, RecTestCallState{}) {
					if !deeperMismatch {
						msg, regions := fmtNotAssignableToVarOftype(state.fmtHelper, right, static, nil)
						state.addError(MakeSymbolicEvalError(decl.Right, state, msg, regions...))
					}
					right = ANY
				} else if holder, ok := right.(StaticDataHolder); ok {
					right, err = holder.AddStatic(static) //TODO: use path narowing, values should never be modified directly
					if err != nil {
						state.addError(MakeSymbolicEvalError(decl.Right, state, err.Error()))
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

		//Left hand side

		switch left := decl.Left.(type) {
		case *ast.IdentifierLiteral:
			ident := left
			state.setGlobal(ident.Name, right, GlobalVar, decl.Left)
			state.SetMostSpecificNodeValue(ident, right)
		case *ast.ObjectDestructuration:
			areLocalDeclarations := false
			err := evalObjectDestructuration(left, decl.Right, right, state, areLocalDeclarations)
			if err != nil {
				return err
			}
		default:
			continue
		}
	}
	state.SetGlobalScopeData(n, state.currentGlobalScopeData())
	return nil
}

func evalObjectDestructuration(
	destructuration *ast.ObjectDestructuration,
	rightNode ast.Node,
	rightValue Value,
	state *State,
	localDeclarations bool,
) (finalErr error) {
	rightValue = AsIprops(rightValue)
	iprops, ok := rightValue.(IProps)
	if !ok {
		state.addError(MakeSymbolicEvalError(rightNode, state, fmtUnexpectedRhsOfObjectDestructuration(rightValue)))
	}

	for _, prop := range destructuration.Properties {
		validProp, ok := prop.(*ast.ObjectDestructurationProperty)
		if !ok {
			continue
		}

		var variableValue Value = ANY

		nameNode := validProp.NameNode()
		if iprops != nil {
			accessKind := destructurationMemberAccess
			if validProp.Nillable {
				accessKind = optionalDestructurationMemberAccess
			}
			variableValue = symbolicMemb(rightValue, nameNode.Name, accessKind, validProp, state)
		}
		if localDeclarations {
			state.setLocal(nameNode.Name, variableValue, nil /* type annotation is ignored */, nameNode)
		} else {
			state.setGlobal(nameNode.Name, variableValue, GlobalVar, nameNode)
		}
		state.SetMostSpecificNodeValue(nameNode, variableValue)
	}

	return nil
}

func evalAssignment(node *ast.Assignment, state *State) (_ Value, finalErr error) {
	badIntOperationRHS := false
	var __rhs Value

	getRHS := func(expected Value) (value Value, deeperMismatch bool, _ error) {
		if __rhs != nil {
			panic(errors.New("right node already evaluated"))
		}

		var result Value
		var err error
		if expected == nil {
			result, err = symbolicEval(node.Right, state)
		} else {
			result, err = _symbolicEval(node.Right, state, evalOptions{
				expectedValue:       expected,
				actualValueMismatch: &deeperMismatch,
			})
		}

		if err != nil {
			return nil, false, err
		}

		if node.Operator.Int() {
			result = MergeValuesWithSameStaticTypeInMultivalue(result)

			// if the operation requires integer operands we check that RHS is an integer
			if _, ok := result.(*Int); !ok {
				badIntOperationRHS = true
				state.addError(MakeSymbolicEvalError(node.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT))
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

			state.SetLocalScopeData(node, state.currentLocalScopeData())
			state.SetGlobalScopeData(node, state.currentGlobalScopeData())
		}
	}()

	switch lhs := node.Left.(type) {
	case *ast.Variable:
		name := lhs.Name

		if state.hasGlobal(name) {
			//Global variable assignments are not allowed, there should be a static check error.
			return nil, nil
		}

		if state.hasLocal(name) {
			if node.Operator.Int() {
				info, _ := state.getLocal(name)
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}

				lhsValue := MergeValuesWithSameStaticTypeInMultivalue(info.value)

				if _, ok := lhsValue.(*Int); !ok {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
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
			state.setLocal(name, rhs, nil, node.Left)
		}

		//TODO: set to previous value instead ?
		state.SetMostSpecificNodeValue(lhs, __rhs)
		state.SetLocalScopeData(node, state.currentLocalScopeData())
	case *ast.IdentifierLiteral:
		name := lhs.Name

		if state.hasGlobal(name) {
			//Global variable assignments are not allowed, there should be a static check error.
			return nil, nil
		}

		if state.hasLocal(name) {
			if node.Operator.Int() {
				info, _ := state.getLocal(name)
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}

				lhsValue := MergeValuesWithSameStaticTypeInMultivalue(info.value)

				if _, ok := lhsValue.(*Int); !ok {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
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
			state.setLocal(name, rhs, nil, node.Left)
		}

		//TODO: set to previous value instead ?
		state.SetMostSpecificNodeValue(lhs, __rhs)
		state.SetLocalScopeData(node, state.currentLocalScopeData())
	case *ast.MemberExpression:
		object, err := _symbolicEval(lhs.Left, state, evalOptions{
			doubleColonExprAncestorChain: []ast.Node{node},
		})
		if err != nil {
			return nil, err
		}

		if node.Err != nil {
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
				state.addError(MakeSymbolicEvalError(node, state, FmtCannotAssignPropertyOf(val)))
				iprops = &Object{}
			}
		}

		var expectedValue Value
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
			state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
			return nil, nil
		}

		propName := lhs.PropertyName.Name
		hasPrevValue := slices.Contains(iprops.PropertyNames(), propName)

		if hasPrevValue {
			prevValue := iprops.Prop(propName)
			state.SetMostSpecificNodeValue(lhs.PropertyName, prevValue)

			checkNotClonedObjectPropMutation(lhs, state, true)

			if _, ok := iprops.(Serializable); ok {
				if _, ok := rhs.(Serializable); !ok {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
					return nil, nil
				}
			}

			if _, ok := asWatchable(iprops).(Watchable); ok {
				if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
				}
			}

			if node.Operator.Int() {
				widenedPrevValue := MergeValuesWithSameStaticTypeInMultivalue(prevValue)

				if _, ok := widenedPrevValue.(*Int); !ok {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
				}
			} else if badIntOperationRHS {

			} else {
				if newIprops, err := iprops.SetProp(state, node, propName, rhs); err != nil {
					if !deeperMismatch {
						state.addError(makeSymbolicEvalErrorFromError(node, state, err))
					}
				} else {
					narrowChain(lhs.Left, setExactValue, newIprops, state, 0)
				}
			}

		} else {
			nonSerializableErr := false
			if _, ok := iprops.(Serializable); ok {
				if _, ok := rhs.(Serializable); !ok {
					nonSerializableErr = true
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
					rhs = ANY_SERIALIZABLE
				}
			}

			if _, ok := asWatchable(iprops).(Watchable); ok && !nonSerializableErr {
				if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
				}
			}

			if newIprops, err := iprops.SetProp(state, node, propName, rhs); err != nil {
				if !deeperMismatch {
					state.addError(makeSymbolicEvalErrorFromError(node, state, err))
				}
			} else {
				narrowChain(lhs.Left, setExactValue, newIprops, state, 0)
			}
		}

	case *ast.IdentifierMemberExpression:
		v, err := _symbolicEval(lhs.Left, state, evalOptions{
			doubleColonExprAncestorChain: []ast.Node{node},
		})
		if err != nil {
			return nil, err
		}

		for _, ident := range lhs.PropertyNames[:len(lhs.PropertyNames)-1] {
			v = symbolicMemb(v, ident.Name, unspecifiedMemberAccess, lhs, state)
			state.SetMostSpecificNodeValue(ident, v)
		}

		//handle IProps and structs LHS separately

		lastPropNameNode := lhs.PropertyNames[len(lhs.PropertyNames)-1]
		lastPropName := lastPropNameNode.Name

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
				state.addError(MakeSymbolicEvalError(node, state, FmtCannotAssignPropertyOf(val)))
				iprops = &Object{}
			}
		}

		hasPrevValue := slices.Contains(iprops.PropertyNames(), lastPropName)

		var expectedValue Value
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
			state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
			return nil, nil
		}

		if hasPrevValue {
			prevValue := iprops.Prop(lastPropName)
			state.SetMostSpecificNodeValue(lastPropNameNode, prevValue)

			checkNotClonedObjectPropMutation(lhs, state, true)

			if _, ok := iprops.(Serializable); ok {

				if _, ok := rhs.(Serializable); !ok {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
					return nil, nil
				}
			}

			if _, ok := asWatchable(iprops).(Watchable); ok {
				if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
				}
			}

			if _, ok := prevValue.(*Int); !ok && node.Operator.Int() {
				state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
			} else {
				if newIprops, err := iprops.SetProp(state, node, lastPropName, rhs); err != nil {
					if !deeperMismatch {
						state.addError(makeSymbolicEvalErrorFromError(node, state, err))
					}
				} else {
					narrowChain(lhs, setExactValue, newIprops, state, 1)
				}
			}
		} else {
			checkNotClonedObjectPropMutation(lhs, state, true)

			nonSerializableErr := false
			if _, ok := iprops.(Serializable); ok {
				if _, ok := rhs.(Serializable); !ok {
					nonSerializableErr = true
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
					rhs = ANY_SERIALIZABLE
				}
			}

			if _, ok := asWatchable(iprops).(Watchable); ok && !nonSerializableErr {
				if _, ok := asWatchable(rhs).(Watchable); !ok && rhs.IsMutable() {
					state.addError(MakeSymbolicEvalError(node, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE))
				}
			}

			if newIprops, err := iprops.SetProp(state, node, lastPropName, rhs); err != nil {
				if !deeperMismatch {
					state.addError(makeSymbolicEvalErrorFromError(node, state, err))
				}
			} else {
				narrowChain(lhs, setExactValue, newIprops, state, 1)
			}
		}
	case *ast.IndexExpression:
		index, err := symbolicEval(lhs.Index, state)
		if err != nil {
			return nil, err
		}

		intIndex, ok := index.(*Int)
		if !ok {
			state.addError(MakeSymbolicEvalError(node, state, fmtIndexIsNotAnIntButA(index)))
		}

		slice, err := _symbolicEval(lhs.Indexed, state, evalOptions{
			doubleColonExprAncestorChain: []ast.Node{node, lhs},
		})
		if err != nil {
			return nil, err
		}

		checkNotClonedObjectPropMutation(lhs, state, false)

		seq, isMutableSeq := asIndexable(slice).(MutableSequence)
		if isMutableSeq && (!seq.HasKnownLen() ||
			intIndex == nil ||
			!intIndex.hasValue ||
			(intIndex.value >= 0 && intIndex.value < int64(seq.KnownLen()))) {

			var seqElementAtIndex Serializable
			if intIndex != nil && intIndex.hasValue {
				seqElementAtIndex = seq.ElementAt(int(intIndex.value)).(Serializable)
			}

			if IsReadonly(seq) {
				state.addError(MakeSymbolicEvalError(node.Left, state, ErrReadonlyValueCannotBeMutated.Error()))
				break
			}

			//evaluate right
			var deeperMismatch bool
			{
				var expectedValue Value = seqElementAtIndex
				if expectedValue == nil {
					expectedValue = seq.Element()
				}
				_, deeperMismatch, err = getRHS(expectedValue)
				if err != nil {
					return nil, err
				}
			}

			//-----------------------------------------
			if _, ok := slice.(Serializable); ok {
				if _, ok := __rhs.(Serializable); !ok {
					state.addError(MakeSymbolicEvalError(node, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
					break
				}
			}

			if _, ok := asWatchable(slice).(Watchable); ok {
				if _, ok := asWatchable(__rhs).(Watchable); !ok && __rhs.IsMutable() {
					state.addError(MakeSymbolicEvalError(node, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
					break
				}
			}

			ignoreNextAssignabilityError := false

			if node.Operator.Int() {
				if seqElementAtIndex != nil {
					widenedSeqElementAtIndex := MergeValuesWithSameStaticTypeInMultivalue(seqElementAtIndex)

					if !ANY_INT.Test(widenedSeqElementAtIndex, RecTestCallState{}) {
						state.addError(MakeSymbolicEvalError(lhs, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
						ignoreNextAssignabilityError = true
					}
					//note: the element is widened in order to support multivalues such as (1 | 2)
				} else {
					widenedSeqElement := MergeValuesWithSameStaticTypeInMultivalue(seq.Element())
					if !ANY_INT.Test(widenedSeqElement, RecTestCallState{}) {
						state.addError(MakeSymbolicEvalError(lhs, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
						ignoreNextAssignabilityError = true
					}
				}
			}

			if seqElementAtIndex == nil || !seqElementAtIndex.Test(__rhs, RecTestCallState{}) {
				assignable := false
				var staticSeq MutableLengthSequence
				var staticSeqElement Value

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
							staticSeqElement = staticSeq.ElementAt(int(intIndex.value)).(Serializable)
							if staticSeqElement.Test(__rhs, RecTestCallState{}) {
								assignable = true
								narrowChain(lhs.Indexed, setExactValue, staticSeq, state, 0)
							}
						} else {
							state.addError(MakeSymbolicEvalError(node.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENT))
							ignoreNextAssignabilityError = true
							staticSeqElement = staticSeq.Element()
						}
					} else {
						staticSeqElement = staticSeq.Element()
						if staticSeqElement.Test(MergeValuesWithSameStaticTypeInMultivalue(__rhs), RecTestCallState{}) {
							assignable = true
							narrowChain(lhs.Indexed, setExactValue, staticSeq, state, 0)
						}
					}
				}

			add_assignability_error:
				if !assignable && !ignoreNextAssignabilityError && !deeperMismatch {
					var v Value
					if staticSeq != nil {
						v = staticSeqElement
					} else {
						v = seq.Element()
					}
					msg, regions := fmtNotAssignableToElementOfValue(state.fmtHelper, __rhs, v, nil)
					state.addError(MakeSymbolicEvalError(node.Right, state, msg, regions...))
				}
			}
		} else if isMutableSeq && intIndex != nil && intIndex.hasValue && seq.HasKnownLen() {
			state.addError(MakeSymbolicEvalError(lhs.Index, state, INDEX_IS_OUT_OF_BOUNDS))
		} else {
			state.addError(MakeSymbolicEvalError(lhs.Indexed, state, fmtXisNotAMutableSequence(slice)))
			slice = NewListOf(ANY_SERIALIZABLE)
		}

		return nil, nil
	case *ast.SliceExpression:
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
			state.addError(MakeSymbolicEvalError(node, state, fmtStartIndexIsNotAnIntButA(startIndex)))
		}

		endIntIndex, ok := endIndex.(*Int)
		if !ok {
			state.addError(MakeSymbolicEvalError(node, state, fmtEndIndexIsNotAnIntButA(endIndex)))
		}

		if startIntIndex != nil && endIntIndex != nil && startIntIndex.hasValue && endIntIndex.hasValue &&
			endIntIndex.value < startIntIndex.value {
			state.addError(MakeSymbolicEvalError(lhs.EndIndex, state, END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX))
		}

		slice, err := _symbolicEval(lhs.Indexed, state, evalOptions{
			doubleColonExprAncestorChain: []ast.Node{node, lhs},
		})
		if err != nil {
			return nil, err
		}

		checkNotClonedObjectPropMutation(lhs, state, false)

		seq, isMutableSeq := slice.(MutableSequence)
		if isMutableSeq && (!seq.HasKnownLen() ||
			startIntIndex == nil ||
			!startIntIndex.hasValue ||
			(startIntIndex.value >= 0 && startIntIndex.value < int64(seq.KnownLen()))) {

			if IsReadonly(seq) {
				state.addError(MakeSymbolicEvalError(node.Left, state, ErrReadonlyValueCannotBeMutated.Error()))
				break
			}

			//in order to simplify the validation logic the assignment is considered valid
			//if and only if the static type of LHS is a sequence of unknown length whose .element() matches the
			//elements of RHS.

			// get static
			var lhsInfo *varSymbolicInfo

			switch indexed := lhs.Indexed.(type) {
			case *ast.Variable:
				info, ok := state.getGlobal(indexed.Name)
				if ok {
					lhsInfo = &info
				}

				info, ok = state.getLocal(indexed.Name)
				if !ok {
					break
				}
			case *ast.IdentifierLiteral:
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
			var rightSeqElement Value
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
				var expectedValue Value = NewAnySequenceOf(staticSeq.Element())
				_, deeperMismatch, err = getRHS(expectedValue)
				if err != nil {
					return nil, err
				}

				rightSeq, ok := __rhs.(Sequence)
				if !ok {
					state.addError(MakeSymbolicEvalError(node.Right, state, fmtSequenceExpectedButIs(__rhs)))
					break
				}

				//---------------------------
				if _, ok := slice.(Serializable); ok {
					if _, ok := __rhs.(Serializable); !ok {
						state.addError(MakeSymbolicEvalError(node, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
						break
					}
				}

				if _, ok := asWatchable(slice).(Watchable); ok {
					if _, ok := asWatchable(__rhs).(Watchable); !ok && __rhs.IsMutable() {
						state.addError(MakeSymbolicEvalError(node, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
						break
					}
				}

				if rightSeq.HasKnownLen() && startIntIndex != nil && endIntIndex != nil &&
					startIntIndex.hasValue && endIntIndex.hasValue && startIntIndex.value >= 0 &&
					endIntIndex.value >= startIntIndex.value && endIntIndex.value-startIntIndex.value != int64(rightSeq.KnownLen()) {
					expectedLength := endIntIndex.value - startIntIndex.value
					invalidRHSLength = true
					state.addError(MakeSymbolicEvalError(node.Right, state, fmtRHSSequenceShouldHaveLenOf(int(expectedLength))))
				}

				rightSeqElement = rightSeq.Element()
			}

			if staticSeq.HasKnownLen() {
				if !invalidRHSLength && endIntIndex != nil && endIntIndex.hasValue && startIntIndex != nil && startIntIndex.hasValue &&
					staticSeq.KnownLen() > int(startIntIndex.value) {
					//conservatively assume not assignable
				} else {
					state.addError(MakeSymbolicEvalError(node.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENTS))
					ignoreNextAssignabilityError = true
				}
			} else {
				staticSeqElement := staticSeq.Element()
				if staticSeqElement.Test(MergeValuesWithSameStaticTypeInMultivalue(rightSeqElement), RecTestCallState{}) {
					assignable = true
					narrowChain(lhs.Indexed, setExactValue, staticSeq, state, 0)
				}
			}

		add_slice_assignability_error:
			if !assignable && !ignoreNextAssignabilityError && !deeperMismatch {
				b := seq
				if staticSeq != nil {
					b = staticSeq
				}
				msg, regions := fmtSeqOfXNotAssignableToSliceOfTheValue(state.fmtHelper, rightSeqElement, b)
				state.addError(MakeSymbolicEvalError(node.Right, state, msg, regions...))
			}

		} else if isMutableSeq && startIntIndex != nil && startIntIndex.hasValue && seq.HasKnownLen() {
			state.addError(MakeSymbolicEvalError(lhs.StartIndex, state, START_INDEX_IS_OUT_OF_BOUNDS))
		} else {
			state.addError(MakeSymbolicEvalError(lhs.Indexed, state, fmtMutableSequenceExpectedButIs(slice)))
			slice = NewListOf(ANY_SERIALIZABLE)
		}

		return nil, nil
	default:
		return nil, fmt.Errorf("invalid assignment: left hand side is a(n) %T", node.Left)
	}

	return nil, nil
}

func evalMultiAssignment(n *ast.MultiAssignment, state *State) (_ Value, finalErr error) {
	isNillable := n.Nillable
	right, err := symbolicEval(n.Right, state)
	startRight := right

	if err != nil {
		return nil, err
	}

	seq, ok := right.(Sequence)
	if !ok {
		state.addError(MakeSymbolicEvalError(n, state, fmtSeqExpectedButIs(startRight)))
		right = &List{generalElement: ANY_SERIALIZABLE}

		for _, var_ := range n.Variables {
			name := var_.(*ast.IdentifierLiteral).Name

			if !state.hasLocal(name) {
				state.setLocal(name, ANY, nil, var_)
			}
			state.SetMostSpecificNodeValue(var_, ANY)
		}
	} else {
		if seq.HasKnownLen() && seq.KnownLen() < len(n.Variables) && !isNillable {
			state.addError(MakeSymbolicEvalError(n, state, fmtSequenceShouldHaveLengthGreaterOrEqualTo(len(n.Variables))))
		}

		for i, var_ := range n.Variables {
			name := var_.(*ast.IdentifierLiteral).Name

			val := seq.ElementAt(i)
			if isNillable && (!seq.HasKnownLen() || i >= seq.KnownLen() && isNillable) {
				val = joinValues([]Value{val, Nil})
			}

			if state.hasLocal(name) {
				state.updateLocal(name, val, n)
			} else {
				state.setLocal(name, val, nil, var_)
			}
			state.SetMostSpecificNodeValue(var_, val)
		}
	}

	state.SetLocalScopeData(n, state.currentLocalScopeData())
	return nil, nil
}

func evalIfStatement(n *ast.IfStatement, state *State) (_ Value, finalErr error) {
	test, err := symbolicEval(n.Test, state)
	if err != nil {
		return nil, err
	}

	if _, ok := test.(*Bool); !ok {
		state.addError(MakeSymbolicEvalError(n.Test, state, fmtIfStmtTestShouldBeBoolBut(test)))
	}

	if n.Consequent != nil {
		//consequent
		var consequentStateFork *State
		{
			consequentStateFork = state.fork()
			narrow(true, n.Test, state, consequentStateFork)
			state.SetLocalScopeData(n.Consequent, consequentStateFork.currentLocalScopeData())
			state.SetGlobalScopeData(n.Consequent, consequentStateFork.currentGlobalScopeData())

			_, err = symbolicEval(n.Consequent, consequentStateFork)
			if err != nil {
				return nil, err
			}
		}

		var alternateStateFork *State
		if n.Alternate != nil {
			alternateStateFork = state.fork()
			narrow(false, n.Test, state, alternateStateFork)
			state.SetLocalScopeData(n.Alternate, alternateStateFork.currentLocalScopeData())
			state.SetGlobalScopeData(n.Alternate, alternateStateFork.currentGlobalScopeData())

			_, err = symbolicEval(n.Alternate, alternateStateFork)
			if err != nil {
				return nil, err
			}
		}

		if alternateStateFork != nil {
			areAllOutcomesCovered := true
			state.join(areAllOutcomesCovered, consequentStateFork, alternateStateFork)
		} else {
			areAllOutcomesCovered := false
			state.join(areAllOutcomesCovered, consequentStateFork)
		}
	}

	return nil, nil
}

func evalIfExpression(n *ast.IfExpression, state *State, options evalOptions) (_ Value, finalErr error) {

	test, err := symbolicEval(n.Test, state)
	if err != nil {
		return nil, err
	}

	var consequentValue Value
	var alternateValue Value

	if _, ok := test.(*Bool); ok {
		if n.Consequent == nil {
			return ANY, nil
		}

		deeperValueMismatch := false

		consequentStateFork := state.fork()
		narrow(true, n.Test, state, consequentStateFork)
		state.SetLocalScopeData(n.Consequent, consequentStateFork.currentLocalScopeData())
		state.SetGlobalScopeData(n.Consequent, consequentStateFork.currentGlobalScopeData())

		consequentValue, err = _symbolicEval(n.Consequent, consequentStateFork, evalOptions{
			actualValueMismatch: &deeperValueMismatch,
			expectedValue:       options.expectedValue,
		})

		if err != nil {
			return nil, err
		}

		if options.expectedValue != nil &&
			!deeperValueMismatch &&
			!options.expectedValue.Test(consequentValue, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {

			options.setActualValueMismatchIfNotNil()
			msg, regions := fmtValueIsAnXButYWasExpected(state.fmtHelper, consequentValue, options.expectedValue, state.testCallMessageBuffer)

			state.addError(MakeSymbolicEvalError(n.Consequent, state, msg, regions...))
		} else if deeperValueMismatch {
			options.setActualValueMismatchIfNotNil()
			deeperValueMismatch = false //reset so that we can use the variable for the alternate value.
		}

		var alternateStateFork *State
		if n.Alternate != nil {
			alternateStateFork := state.fork()
			narrow(false, n.Test, state, alternateStateFork)
			state.SetLocalScopeData(n.Alternate, alternateStateFork.currentLocalScopeData())
			state.SetGlobalScopeData(n.Alternate, alternateStateFork.currentGlobalScopeData())

			alternateValue, err = _symbolicEval(n.Alternate, alternateStateFork, evalOptions{
				actualValueMismatch: &deeperValueMismatch,
				expectedValue:       options.expectedValue,
			})

			if err != nil {
				return nil, err
			}

			state.resetTestCallMsgBuffers()

			if options.expectedValue != nil &&
				!deeperValueMismatch &&
				!options.expectedValue.Test(alternateValue, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {

				options.setActualValueMismatchIfNotNil()
				msg, regions := fmtValueIsAnXButYWasExpected(state.fmtHelper, alternateValue, options.expectedValue, state.testCallMessageBuffer)
				state.addError(MakeSymbolicEvalError(n.Alternate, state, msg, regions...))
			} else if deeperValueMismatch {
				options.setActualValueMismatchIfNotNil()
			}

			return joinValues([]Value{consequentValue, alternateValue}), nil
		}

		if alternateStateFork != nil {
			areAllOutcomesCovered := true
			state.join(areAllOutcomesCovered, consequentStateFork, alternateStateFork)
		} else {
			areAllOutcomesCovered := false
			state.join(areAllOutcomesCovered, consequentStateFork)
		}

		return consequentValue, nil
	}
	//else: the condition's result is not a boolean

	options.setHasShallowErrors()

	state.addError(MakeSymbolicEvalError(n.Test, state, fmtIfExprTestShouldBeBoolBut(test)))
	return ANY, nil
}

func evalForStatementAndExpr(n ast.Node, state *State) (_ Value, finalErr error) {

	var (
		iteratedValueNode, body       ast.Node
		keyIndexIdent, valueElemIdent *ast.IdentifierLiteral
		chunked                       bool
		isForExpr                     bool
	)

	var forExprListElement Value

	if stmt, ok := n.(*ast.ForStatement); ok {
		iteratedValueNode = stmt.IteratedValue
		keyIndexIdent = stmt.KeyIndexIdent
		valueElemIdent = stmt.ValueElemIdent
		if stmt.Body != nil {
			body = stmt.Body
		}
		chunked = stmt.Chunked

		if iteratedValueNode == nil {
			return
		}
	} else {
		expr := n.(*ast.ForExpression)
		iteratedValueNode = expr.IteratedValue
		keyIndexIdent = expr.KeyIndexIdent
		valueElemIdent = expr.ValueElemIdent
		if expr.Body != nil {
			body = expr.Body
		}
		chunked = expr.Chunked
		isForExpr = true

		if iteratedValueNode == nil {
			return ANY, nil
		}
	}

	iteratedValue, err := symbolicEval(iteratedValueNode, state)
	if err != nil {
		return nil, err
	}

	var kVarname string
	var eVarname string

	if keyIndexIdent != nil {
		kVarname = keyIndexIdent.Name
	}
	if valueElemIdent != nil {
		eVarname = valueElemIdent.Name
	}

	var keyType Value = ANY
	var valueType Value = ANY
	evaluateBody := true

	if iterable, ok := asIterable(iteratedValue).(Iterable); ok {
		if chunked {
			state.addError(MakeSymbolicEvalError(n, state, "chunked iteration of iterables is not supported yet"))
		}

		keyType = iterable.IteratorElementKey()
		valueType = iterable.IteratorElementValue()

		//If we are not in an initial check call of an Inox function and the iterated value is an empty indexable,
		//we do not evaluate the body. This is done to not have some irrelevant errors.
		if indexable, ok := asIndexable(iterable).(Indexable); ok && indexable.HasKnownLen() && indexable.KnownLen() == 0 {
			call, ok := state.currentInoxCall()
			if ok && !call.isInitialCheckCall {
				evaluateBody = false
			}
		}

	} else if streamable, ok := asStreamable(iteratedValue).(StreamSource); ok {
		if keyIndexIdent != nil {
			state.addError(MakeSymbolicEvalError(keyIndexIdent, state, KEY_VAR_SHOULD_BE_PROVIDED_ONLY_WHEN_ITERATING_OVER_AN_ITERABLE))
		}
		if chunked {
			valueType = streamable.ChunkedStreamElement()
		} else {
			valueType = streamable.StreamElement()
		}
	} else {
		state.addError(MakeSymbolicEvalError(iteratedValueNode, state, fmtXisNotIterable(iteratedValue)))
	}

	if body != nil && evaluateBody {
		stateFork := state.fork()

		if keyIndexIdent != nil {
			stateFork.setLocal(kVarname, keyType, nil, keyIndexIdent)
			stateFork.symbolicData.SetMostSpecificNodeValue(keyIndexIdent, keyType)
		}
		if valueElemIdent != nil {
			stateFork.setLocal(eVarname, valueType, nil, valueElemIdent)
			stateFork.symbolicData.SetMostSpecificNodeValue(valueElemIdent, valueType)
		}

		stateFork.symbolicData.SetLocalScopeData(body, stateFork.currentLocalScopeData())

		res, err := symbolicEval(body, stateFork)
		if err != nil {
			return nil, err
		}

		if isForExpr {
			var stepResult Value
			if _, isBlockBody := body.(*ast.Block); isBlockBody {

				if stateFork.yieldedValue != nil {
					stepResult = stateFork.yieldedValue
				}

			} else {
				stepResult = res
			}

			stateFork.yieldedValue = nil
			stateFork.conditionalYield = false

			if stepResult != nil {
				elem, ok := AsSerializable(stepResult).(Serializable)
				if !ok {
					state.addError(MakeSymbolicEvalError(body, state, ELEMENTS_PRODUCED_BY_A_FOR_EXPR_SHOULD_BE_SERIALIZABLE))
					elem = ANY_SERIALIZABLE
				}
				forExprListElement = elem
			}
		}

		areAllOutcomesCovered := false //The iterated value can be empty.

		state.join(areAllOutcomesCovered, stateFork)
		//we set the local scope data at the for statement or expression, not the body
		state.SetLocalScopeData(n, state.currentLocalScopeData())
	}

	if isForExpr {
		if forExprListElement == nil {
			return EMPTY_LIST, nil
		}
		elem := AsSerializableChecked(forExprListElement)
		return NewListOf(elem), nil
	}

	return nil, nil
}

func evalWalkStatementAndExpr(n ast.Node, state *State) (_ Value, finalErr error) {

	var (
		walkedValueNode, body ast.Node
		metaIdent, entryIdent *ast.IdentifierLiteral
		isWalkExpr            bool
	)

	var walkExprListElement Value

	if stmt, ok := n.(*ast.WalkStatement); ok {
		walkedValueNode = stmt.Walked
		metaIdent = stmt.MetaIdent
		entryIdent = stmt.EntryIdent
		if stmt.Body != nil {
			body = stmt.Body
		}

		if walkedValueNode == nil {
			return
		}
	} else {
		expr := n.(*ast.WalkExpression)
		walkedValueNode = expr.Walked
		metaIdent = expr.MetaIdent
		entryIdent = expr.EntryIdent
		if expr.Body != nil {
			body = expr.Body
		}
		isWalkExpr = true

		if walkedValueNode == nil {
			return ANY, nil
		}
	}

	walkedValue, err := symbolicEval(walkedValueNode, state)
	if err != nil {
		return nil, err
	}

	walkable, ok := walkedValue.(Walkable)

	var nodeMeta, entry Value

	if ok {
		entry = walkable.WalkerElement()
		nodeMeta = walkable.WalkerNodeMeta()
	} else {
		state.addError(MakeSymbolicEvalError(walkedValueNode, state, fmtXisNotWalkable(walkedValue)))
		entry = ANY
		nodeMeta = ANY
	}

	if body != nil {
		stateFork := state.fork()

		stateFork.setLocal(entryIdent.Name, entry, nil, entryIdent)
		stateFork.symbolicData.SetMostSpecificNodeValue(entryIdent, entry)

		if metaIdent != nil {
			stateFork.setLocal(metaIdent.Name, nodeMeta, nil, metaIdent)
			stateFork.symbolicData.SetMostSpecificNodeValue(metaIdent, nodeMeta)
		}

		stateFork.symbolicData.SetLocalScopeData(body, stateFork.currentLocalScopeData())

		res, blkErr := symbolicEval(body, stateFork)
		if blkErr != nil {
			return nil, blkErr
		}

		if isWalkExpr {
			var stepResult Value
			if _, isBlockBody := body.(*ast.Block); isBlockBody {

				if stateFork.yieldedValue != nil {
					stepResult = stateFork.yieldedValue
				}

			} else {
				stepResult = res
			}

			stateFork.yieldedValue = nil
			stateFork.conditionalYield = false

			if stepResult != nil {
				elem, ok := AsSerializable(stepResult).(Serializable)
				if !ok {
					state.addError(MakeSymbolicEvalError(body, state, ELEMENTS_PRODUCED_BY_A_WALK_EXPR_SHOULD_BE_SERIALIZABLE))
					elem = ANY_SERIALIZABLE
				}
				walkExprListElement = elem
			}
		}

		areAllOutcomesCovered := false //The walked value can be empty.

		state.join(areAllOutcomesCovered, stateFork)
		//we set the local scope data at the walk statement or expression, not the body
		state.SetLocalScopeData(n, state.currentLocalScopeData())
	}

	if isWalkExpr {
		if walkExprListElement == nil {
			return EMPTY_LIST, nil
		}
		elem := AsSerializableChecked(walkExprListElement)
		return NewListOf(elem), nil
	}

	return nil, nil
}

func evalSwitchStatement(n *ast.SwitchStatement, state *State) (_ Value, finalErr error) {

	_, err := _symbolicEval(n.Discriminant, state, evalOptions{
		fallbackResult: ANY,
	})

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
			narrowChain(n.Discriminant, setExactValue, caseValue, blockStateFork, 0)

			_, err = symbolicEval(switchCase.Block, blockStateFork)
			if err != nil {
				return nil, err
			}
		}
	}

	hasValidDefaultCase := false

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
		hasValidDefaultCase = true
	}

	areAllOutcomesCovered := hasValidDefaultCase

	state.join(areAllOutcomesCovered, forks...)

	return nil, nil
}

func evalSwitchExpression(n *ast.SwitchExpression, state *State, options evalOptions) (_ Value, finalErr error) {

	_, err := _symbolicEval(n.Discriminant, state, evalOptions{
		fallbackResult: DEFAULT_SWITCH_MATCH_EXPR_RESULT,
	})

	if err != nil {
		return nil, err
	}

	var forks []*State

	var results []Value
	deeperValueMismatch := false

	for _, switchCase := range n.Cases {
		for _, valNode := range switchCase.Values {
			caseValue, err := symbolicEval(valNode, state)
			if err != nil {
				return nil, err
			}

			if switchCase.Result == nil {
				continue
			}

			blockStateFork := state.fork()
			forks = append(forks, blockStateFork)
			narrowChain(n.Discriminant, setExactValue, caseValue, blockStateFork, 0)

			result, err := _symbolicEval(switchCase.Result, blockStateFork, evalOptions{
				actualValueMismatch: &deeperValueMismatch,
				expectedValue:       options.expectedValue,
			})

			if err != nil {
				return nil, err
			}

			results = append(results, result)

			if options.expectedValue != nil &&
				!deeperValueMismatch &&
				!options.expectedValue.Test(result, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {

				options.setActualValueMismatchIfNotNil()

				msg, regions := fmtValueIsAnXButYWasExpected(state.fmtHelper, result, options.expectedValue, state.testCallMessageBuffer)
				state.addError(MakeSymbolicEvalError(switchCase.Result, state, msg, regions...))
			} else if deeperValueMismatch {
				options.setActualValueMismatchIfNotNil()
				deeperValueMismatch = false //reset so that we can use the variable for other results.
			}

		}
	}

	hasValidDefaultCase := false

	for _, defaultCase := range n.DefaultCases {
		if defaultCase.Result == nil {
			continue
		}

		blockStateFork := state.fork()
		forks = append(forks, blockStateFork)
		result, err := _symbolicEval(defaultCase.Result, blockStateFork, evalOptions{
			actualValueMismatch: &deeperValueMismatch,
			expectedValue:       options.expectedValue,
		})

		if err != nil {
			return nil, err
		}

		results = append(results, result)
		hasValidDefaultCase = true

		if options.expectedValue != nil &&
			!deeperValueMismatch &&
			!options.expectedValue.Test(result, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {

			msg, regions := fmtValueIsAnXButYWasExpected(state.fmtHelper, result, options.expectedValue, state.testCallMessageBuffer)
			state.addError(MakeSymbolicEvalError(defaultCase.Result, state, msg, regions...))
		} else if deeperValueMismatch {
			options.setActualValueMismatchIfNotNil()
			deeperValueMismatch = false //reset so that we can use the variable for other results.
		}

	}

	areAllOutcomesCovered := hasValidDefaultCase
	state.join(areAllOutcomesCovered, forks...)

	if len(n.DefaultCases) == 0 {
		results = append(results, DEFAULT_SWITCH_MATCH_EXPR_RESULT)
	}

	return joinValues(results), nil
}

func evalMatchStatement(n *ast.MatchStatement, state *State) (_ Value, finalErr error) {

	discriminant, err := _symbolicEval(n.Discriminant, state, evalOptions{
		fallbackResult: ANY,
	})

	if err != nil {
		return nil, err
	}

	var forks []*State
	var possibleValues []Value

	for _, matchCase := range n.Cases {
		for _, valNode := range matchCase.Values { //TODO: fix handling of multi cases
			if valNode.Base().Err != nil {
				continue
			}

			errCount := len(state.errors())

			val, err := symbolicEval(valNode, state)
			if err != nil {
				return nil, err
			}

			newEvalErr := len(state.errors()) > errCount
			pattern, ok := val.(Pattern)

			if !ok { //if the value of the case is not a pattern we just check for equality
				serializable, ok := AsSerializable(val).(Serializable)

				if !ok {
					if !newEvalErr {
						state.addError(MakeSymbolicEvalError(valNode, state, AN_EXACT_VALUE_USED_AS_MATCH_CASE_SHOULD_BE_SERIALIZABLE))
					}
					continue
				} else {
					patt, err := NewExactValuePattern(serializable)
					if err == nil {
						pattern = patt
					} else {
						state.addError(MakeSymbolicEvalError(valNode, state, err.Error()))
						continue
					}
				}
			}

			if matchCase.Block == nil {
				continue
			}

			blockStateFork := state.fork()
			forks = append(forks, blockStateFork)
			patternMatchingValue := pattern.SymbolicValue()
			possibleValues = append(possibleValues, patternMatchingValue)

			narrowChain(n.Discriminant, setExactValue, patternMatchingValue, blockStateFork, 0)

			if matchCase.GroupMatchingVariable != nil {
				variable := matchCase.GroupMatchingVariable.(*ast.IdentifierLiteral)
				groupPattern, ok := pattern.(GroupPattern)

				if !ok {
					state.addError(MakeSymbolicEvalError(valNode, state, fmtXisNotAGroupMatchingPattern(pattern)))
				} else {
					_, possible, groups := groupPattern.MatchGroups(discriminant)
					if possible {
						groupsObj := NewInexactObject(groups, nil, nil)
						blockStateFork.setLocal(variable.Name, groupsObj, nil, matchCase.GroupMatchingVariable)
						state.SetMostSpecificNodeValue(variable, groupsObj)

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

	hasValidDefaultCase := false

	for _, defaultCase := range n.DefaultCases {
		if defaultCase.Block == nil {
			continue
		}

		blockStateFork := state.fork()
		forks = append(forks, blockStateFork)

		for _, val := range possibleValues {
			narrowChain(n.Discriminant, removePossibleValue, val, blockStateFork, 0)
		}

		_, err = symbolicEval(defaultCase.Block, blockStateFork)
		if err != nil {
			return nil, err
		}
		hasValidDefaultCase = true
	}

	areAllOutcomesCovered := hasValidDefaultCase

	state.join(areAllOutcomesCovered, forks...)

	return nil, nil
}

func evalMatchExpression(n *ast.MatchExpression, state *State, options evalOptions) (_ Value, finalErr error) {

	discriminant, err := _symbolicEval(n.Discriminant, state, evalOptions{
		fallbackResult: DEFAULT_SWITCH_MATCH_EXPR_RESULT,
	})

	if err != nil {
		return nil, err
	}

	var forks []*State
	var possibleValues []Value
	var results []Value

	deeperValueMismatch := false

	for _, matchCase := range n.Cases {
		for _, valNode := range matchCase.Values { //TODO: fix handling of multi cases
			if valNode.Base().Err != nil {
				continue
			}

			errCount := len(state.errors())

			val, err := symbolicEval(valNode, state)
			if err != nil {
				return nil, err
			}

			newEvalErr := len(state.errors()) > errCount
			pattern, ok := val.(Pattern)

			if !ok { //if the value of the case is not a pattern we just check for equality
				serializable, ok := AsSerializable(val).(Serializable)

				if !ok {
					if !newEvalErr {
						state.addError(MakeSymbolicEvalError(valNode, state, AN_EXACT_VALUE_USED_AS_MATCH_CASE_SHOULD_BE_SERIALIZABLE))
					}
					continue
				} else {
					patt, err := NewExactValuePattern(serializable)
					if err == nil {
						pattern = patt
					} else {
						state.addError(MakeSymbolicEvalError(valNode, state, err.Error()))
						continue
					}
				}
			}

			if matchCase.Result == nil {
				continue
			}

			blockStateFork := state.fork()
			forks = append(forks, blockStateFork)
			patternMatchingValue := pattern.SymbolicValue()
			possibleValues = append(possibleValues, patternMatchingValue)

			narrowChain(n.Discriminant, setExactValue, patternMatchingValue, blockStateFork, 0)

			evaluateResult := false

			if matchCase.GroupMatchingVariable != nil {
				variable := matchCase.GroupMatchingVariable.(*ast.IdentifierLiteral)
				groupPattern, ok := pattern.(GroupPattern)

				if !ok {
					state.addError(MakeSymbolicEvalError(valNode, state, fmtXisNotAGroupMatchingPattern(pattern)))
				} else {
					_, possible, groups := groupPattern.MatchGroups(discriminant)
					if possible {
						groupsObj := NewInexactObject(groups, nil, nil)
						blockStateFork.setLocal(variable.Name, groupsObj, nil, matchCase.GroupMatchingVariable)
						state.SetMostSpecificNodeValue(variable, groupsObj)

						evaluateResult = true
					}
				}
			} else {
				evaluateResult = true
			}

			if !evaluateResult {
				continue
			}

			result, err := _symbolicEval(matchCase.Result, blockStateFork, evalOptions{
				actualValueMismatch: &deeperValueMismatch,
				expectedValue:       options.expectedValue,
			})

			if err != nil {
				return nil, err
			}

			results = append(results, result)

			if options.expectedValue != nil &&
				!deeperValueMismatch &&
				!options.expectedValue.Test(result, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
				options.setActualValueMismatchIfNotNil()

				msg, regions := fmtValueIsAnXButYWasExpected(state.fmtHelper, result, options.expectedValue, state.testCallMessageBuffer)
				state.addError(MakeSymbolicEvalError(matchCase.Result, state, msg, regions...))
			} else if deeperValueMismatch {
				options.setActualValueMismatchIfNotNil()
				deeperValueMismatch = false //reset so that we can use the variable for other results.
			}
		}
	}

	hasValidDefaultCase := false

	for _, defaultCase := range n.DefaultCases {
		if defaultCase.Result == nil {
			continue
		}

		blockStateFork := state.fork()
		forks = append(forks, blockStateFork)

		for _, val := range possibleValues {
			narrowChain(n.Discriminant, removePossibleValue, val, blockStateFork, 0)
		}

		result, err := _symbolicEval(defaultCase.Result, blockStateFork, evalOptions{
			actualValueMismatch: &deeperValueMismatch,
			expectedValue:       options.expectedValue,
		})

		if err != nil {
			return nil, err
		}

		results = append(results, result)
		hasValidDefaultCase = true

		if options.expectedValue != nil &&
			!deeperValueMismatch &&
			!options.expectedValue.Test(result, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
			options.setActualValueMismatchIfNotNil()

			msg, regions := fmtValueIsAnXButYWasExpected(state.fmtHelper, result, options.expectedValue, state.testCallMessageBuffer)
			state.addError(MakeSymbolicEvalError(defaultCase.Result, state, msg, regions...))
		} else if deeperValueMismatch {
			options.setActualValueMismatchIfNotNil()
			deeperValueMismatch = false //reset so that we can use the variable for other results.
		}
	}

	if len(n.DefaultCases) == 0 {
		results = append(results, DEFAULT_SWITCH_MATCH_EXPR_RESULT)
	}

	areAllOutcomesCovered := hasValidDefaultCase

	state.join(areAllOutcomesCovered, forks...)

	return joinValues(results), nil
}

func evalUnaryExpression(n *ast.UnaryExpression, state *State, options evalOptions) (_ Value, finalErr error) {
	operand, err := symbolicEval(n.Operand, state)
	if err != nil {
		return nil, err
	}
	switch n.Operator {
	case ast.NumberNegate:
		switch {
		case ImplOrMultivaluesImplementing[*Int](operand):
			return ANY_INT, nil
		case ImplOrMultivaluesImplementing[*Float](operand):
			return ANY_FLOAT, nil
		default:
			state.addError(MakeSymbolicEvalError(n, state, fmtOperandOfNumberNegateShouldBeIntOrFloat(operand)))
		}

		return ANY, nil
	case ast.BoolNegate:
		_, ok := operand.(*Bool)
		if !ok {
			state.addError(MakeSymbolicEvalError(n, state, fmtOperandOfBoolNegateShouldBeBool(operand)))
		}

		return ANY_BOOL, nil
	default:
		return nil, fmt.Errorf("invalid unary operator %d", n.Operator)
	}
}

func evalBinaryExpression(n *ast.BinaryExpression, state *State, options evalOptions) (_ Value, finalErr error) {
	left, err := symbolicEval(n.Left, state)
	if err != nil {
		return nil, err
	}

	right, err := symbolicEval(n.Right, state)
	if err != nil {
		return nil, err
	}

	if multi, ok := left.(IMultivalue); ok {
		left = multi.OriginalMultivalue().WidenSimpleValues()
	}

	if multi, ok := right.(IMultivalue); ok {
		right = multi.OriginalMultivalue().WidenSimpleValues()
	}

	left = MergeValuesWithSameStaticTypeInMultivalue(left)
	right = MergeValuesWithSameStaticTypeInMultivalue(right)

	switch n.Operator {
	case ast.GreaterThan, ast.LessThan, ast.LessOrEqual, ast.GreaterOrEqual:
		_, ok := left.(Comparable)
		if !ok {
			state.addError(MakeSymbolicEvalError(n, state, LEFT_OPERAND_DOES_NOT_IMPL_COMPARABLE_))
			return ANY_BOOL, nil
		}
		_, ok = right.(Comparable)
		if !ok {
			state.addError(MakeSymbolicEvalError(n, state, RIGHT_OPERAND_DOES_NOT_IMPL_COMPARABLE_))
			return ANY_BOOL, nil
		}

		if !haveSameGoTypes(left, right) {
			state.addError(MakeSymbolicEvalError(n, state, OPERANDS_NOT_COMPARABLE_BECAUSE_DIFFERENT_TYPES))
		}
		return ANY_BOOL, nil
	case ast.Add, ast.Sub, ast.Mul, ast.Div:
		return evalArithmeticBinaryExpression(left, right, n, state)
	case ast.AddDot, ast.SubDot, ast.MulDot, ast.DivDot, ast.GreaterThanDot, ast.GreaterOrEqualDot, ast.LessThanDot, ast.LessOrEqualDot:
		state.addError(MakeSymbolicEvalError(n, state, "operator not implemented yet"))
		return ANY, nil
	case ast.Equal, ast.NotEqual, ast.Is, ast.IsNot:
		return ANY_BOOL, nil
	case ast.In:
		switch right.(type) {
		case Container:
		default:
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "container", Stringify(right))))
		}
		_, ok := AsSerializable(left).(Serializable)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(left))))
		}
		return ANY_BOOL, nil
	case ast.NotIn:
		switch right.(type) {
		case Container:
		default:
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "container", Stringify(right))))
		}
		_, ok := AsSerializable(left).(Serializable)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(left))))
		}
		return ANY_BOOL, nil
	case ast.Keyof:
		_, ok := left.(*String)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string", Stringify(left))))
		}

		switch rightVal := right.(type) {
		case *Object:
		default:
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtInvalidBinExprCannnotCheckNonObjectHasKey(rightVal)))
		}
		return ANY_BOOL, nil
	case ast.Urlof:
		if !ImplOrMultivaluesImplementing[*URL](left) {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "url", Stringify(left))))
		}

		switch right.(type) {
		case *Any, *AnySerializable:
		default:
			if !ImplOrMultivaluesImplementing[UrlHolder](right) {
				state.addWarning(makeSymbolicEvalWarning(n.Right, state, RIGHT_OPERAND_MAY_NOT_HAVE_A_URL))
			}
		}
		return ANY_BOOL, nil
	case ast.Range, ast.ExclEndRange:
		switch left := left.(type) {
		case *Int:
			if !ANY_INT.Test(right, RecTestCallState{}) {
				msg := fmtRightOperandOfBinaryShouldBeLikeLeftOperand(n.Operator, Stringify(left), Stringify(ANY_INT))
				state.addError(MakeSymbolicEvalError(n.Right, state, msg))
				return ANY_INT_RANGE, nil
			}

			rightInt := right.(*Int)
			inclusiveEnd := ANY_INT

			if n.Operator == ast.Range {
				inclusiveEnd = rightInt
			} else if n.Operator == ast.ExclEndRange && rightInt.HasValue() {
				inclusiveEnd = NewInt(rightInt.Value() - 1)
			}

			return &IntRange{
				hasValue: true,
				start:    left,
				end:      inclusiveEnd,
			}, nil
		case *Float:
			if !ANY_FLOAT.Test(right, RecTestCallState{}) {
				msg := fmtRightOperandOfBinaryShouldBeLikeLeftOperand(n.Operator, Stringify(left), Stringify(ANY_FLOAT))
				state.addError(MakeSymbolicEvalError(n.Right, state, msg))
				return ANY_FLOAT_RANGE, nil
			}

			rightFloat := right.(*Float)

			return &FloatRange{
				hasValue:     true,
				inclusiveEnd: n.Operator == ast.Range,
				start:        left,
				end:          rightFloat,
			}, nil
		default:
			if _, ok := left.(Serializable); !ok {
				state.addError(MakeSymbolicEvalError(n.Right, state, OPERANDS_OF_BINARY_RANGE_EXPRS_SHOULD_BE_SERIALIZABLE))
				return ANY_QUANTITY_RANGE, nil
			}

			if !left.WidestOfType().Test(right, RecTestCallState{}) {
				msg := fmtRightOperandOfBinaryShouldBeLikeLeftOperand(n.Operator, Stringify(left.WidestOfType()), Stringify(right))
				state.addError(MakeSymbolicEvalError(n.Right, state, msg))
			}

			return &QuantityRange{element: left.WidestOfType().(Serializable)}, nil
		}
	case ast.And, ast.Or:
		_, ok := left.(*Bool)

		if !ok {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "boolean", Stringify(left))))
		}

		_, ok = right.(*Bool)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "boolean", Stringify(right))))
		}
		return ANY_BOOL, nil
	case ast.Match, ast.NotMatch:
		_, ok := right.(Pattern)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "pattern", Stringify(right))))
		}

		return ANY_BOOL, nil
	case ast.As:
		pattern, ok := right.(Pattern)
		if ok {
			val := pattern.SymbolicValue()
			narrowChain(n.Left, setExactValue, val, state, 0)
			return val, nil
		} else {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "pattern", Stringify(right))))
			return left, nil
		}
	case ast.Substrof:

		switch left.(type) {
		case BytesLike, StringLike:
		default:
			if _, ok := left.(StringLike); !ok {
				state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string-like or bytes-like", Stringify(left))))
			}
		}

		switch right.(type) {
		case BytesLike, StringLike:
		default:
			if _, ok := right.(StringLike); !ok {
				state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "string-like", Stringify(right))))
			}
		}

		return ANY_BOOL, nil
	case ast.SetDifference:
		if _, ok := left.(Pattern); !ok {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "pattern", Stringify(left))))
		}
		return &DifferencePattern{
			Base:    ANY_PATTERN,
			Removed: ANY_PATTERN,
		}, nil
	case ast.NilCoalescing:
		return joinValues([]Value{narrowOut(Nil, left), right}), nil
	case ast.PairComma:
		leftSerializable, ok := AsSerializable(left).(Serializable)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(left))))
			leftSerializable = ANY_SERIALIZABLE
		} else if leftSerializable.IsMutable() {
			state.addError(MakeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBeImmutable(n.Operator)))
			leftSerializable = ANY_SERIALIZABLE
		}

		rightSerializable, ok := AsSerializable(right).(Serializable)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(right))))
			rightSerializable = ANY_SERIALIZABLE
		} else if rightSerializable.IsMutable() {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBeImmutable(n.Operator)))
			rightSerializable = ANY_SERIALIZABLE
		}

		return NewOrderedPair(leftSerializable, rightSerializable), nil
	default:
		return nil, fmt.Errorf(fmtInvalidBinaryOperator(n.Operator))
	}
}

// +, -, *, /
func evalArithmeticBinaryExpression(left, right Value, n *ast.BinaryExpression, state *State) (Value, error) {
	if _, ok := left.(*Int); ok {
		_, ok = right.(*Int)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandForIntArithmetic(right, n.Operator)))
		}

		return ANY_INT, nil
	} else if _, ok := left.(*Float); ok {
		_, ok = right.(*Float)
		if !ok {
			state.addError(MakeSymbolicEvalError(n.Right, state, fmtRightOperandForFloatArithmetic(right, n.Operator)))
		}
		return ANY_FLOAT, nil
	}

	_, isRightInt := right.(*Int)
	_, isRightFloat := right.(*Float)

	var failureResult Value = ANY

	if isRightInt || isRightFloat {
		failureResult = right.WidestOfType()
	}

	switch n.Operator {
	case ast.Add:
		iadd, ok := left.(IPseudoAdd)
		if !ok {
			err := MakeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
			state.addError(err)
			return failureResult, nil
		}

		return iadd.Add(right, n, state)
	case ast.Sub:
		isub, ok := left.(IPseudoSub)
		if !ok {
			err := MakeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
			state.addError(err)
			return failureResult, nil
		}

		return isub.Sub(right, n, state)
	case ast.Mul:
		err := MakeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
		state.addError(err)
		return failureResult, nil
	case ast.Div:
		err := MakeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
		state.addError(err)
		return failureResult, nil
	default:
		panic(ErrUnreachable)
	}
}

func evalFunctionExpression(n *ast.FunctionExpression, state *State, options evalOptions) (_ Value, finalErr error) {
	stateFork := state.fork()

	//create a local scope for the function
	stateFork.pushScope()
	defer stateFork.popScope()

	if self, ok := state.getNextSelf(); ok {
		stateFork.setSelf(self)
		defer stateFork.unsetSelf()
	}

	var params []Value
	var paramNames []string

	if len(n.Parameters) > 0 {
		params = make([]Value, len(n.Parameters))
		paramNames = make([]string, len(n.Parameters))
	}

	//declare arguments
	for i, p := range n.Parameters[:n.NonVariadicParamCount()] {
		paramNameIdent, ok := p.Var.(*ast.IdentifierLiteral)
		if !ok {
			return ANY_INOX_FUNC, nil
		}
		name := paramNameIdent.Name
		var paramValue Value = ANY
		var paramType Pattern

		if p.Type != nil {
			pattern, err := evalPatternNode(p.Type, stateFork)
			if err != nil {
				return nil, err
			}
			paramType = pattern
			paramValue = pattern.SymbolicValue()
			state.SetMostSpecificNodeValue(p.Type, pattern)
		}

		stateFork.setLocal(name, paramValue, paramType, p.Var)
		state.SetMostSpecificNodeValue(p.Var, paramValue)
		params[i] = paramValue
		paramNames[i] = name
	}

	if n.IsVariadic && !utils.Implements[*ast.IdentifierLiteral](n.VariadicParameter().Var) {
		return ANY_INOX_FUNC, nil
	}

	var signatureReturnType Value

	if n.ReturnType != nil {
		pattern, err := evalPatternNode(n.ReturnType, stateFork)
		if err != nil {
			return nil, err
		}
		signatureReturnType = pattern.SymbolicValue()
	}

	if state.recursiveFunctionName != "" {
		//set a temporary value for the function

		tempFn := &InoxFunction{
			node:           n,
			nodeChunk:      state.currentChunk().Node,
			parameters:     params,
			parameterNames: paramNames,
			result:         ANY_SERIALIZABLE,
		}

		if signatureReturnType != nil {
			tempFn.result = signatureReturnType
		}

		state.overrideGlobal(state.recursiveFunctionName, tempFn)
		stateFork.overrideGlobal(state.recursiveFunctionName, tempFn)
		state.recursiveFunctionName = ""
	}

	//declare captured locals
	capturedLocals := map[string]Value{}
	for _, e := range n.CaptureList {
		name := e.(*ast.IdentifierLiteral).Name
		info, ok := state.getLocal(name)
		if ok {
			stateFork.setLocal(name, info.value, info.static, e)
			capturedLocals[name] = info.value
		} else {
			stateFork.setLocal(name, ANY, nil, e)
			capturedLocals[name] = ANY
			state.addError(MakeSymbolicEvalError(e, state, fmtLocalVarIsNotDeclared(name)))
		}
	}

	if len(capturedLocals) == 0 {
		capturedLocals = nil
	}

	if n.IsVariadic {
		index := n.NonVariadicParamCount()
		variadicParam := n.VariadicParameter()
		paramNameIdent, ok := variadicParam.Var.(*ast.IdentifierLiteral)
		if !ok {
			return ANY, nil
		}
		paramNames[index] = paramNameIdent.Name

		var elemType Value = ANY

		if variadicParam.Type != nil {
			pattern, err := evalPatternNode(variadicParam.Type, stateFork)
			if err != nil {
				return nil, err
			}
			elemType = pattern.SymbolicValue()
		}

		paramType := ANY_ARRAY
		if elemType != ANY {
			paramType = NewArrayOf(elemType)
		}

		params[index] = paramType
		stateFork.setLocal(paramNameIdent.Name, paramType, nil, paramNameIdent)
	}
	stateFork.symbolicData.SetLocalScopeData(n.Body, stateFork.currentLocalScopeData())

	//-----------------------------

	var storedReturnType Value
	var err error

	if n.Body == nil {
		goto return_function
	}

	state.pushInoxCall(inoxCallInfo{
		calleeFnExpr:       n,
		isInitialCheckCall: true,
	})

	if n.IsBodyExpression {
		//execution of body

		storedReturnType, err = symbolicEval(n.Body, stateFork)
		if err != nil {
			return nil, err
		}

		//check return

		if signatureReturnType != nil {

			if !signatureReturnType.Test(storedReturnType, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
				msg, regions := fmtInvalidReturnValue(state.fmtHelper, storedReturnType, signatureReturnType, state.testCallMessageBuffer)
				state.addError(MakeSymbolicEvalError(n.Body, state, msg, regions...))
			}
			storedReturnType = signatureReturnType
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
				stateFork.addError(MakeSymbolicEvalError(n, stateFork, MISSING_RETURN_IN_FUNCTION))
			} else if stateFork.conditionalReturn {
				stateFork.addError(MakeSymbolicEvalError(n, stateFork, MISSING_UNCONDITIONAL_RETURN_IN_FUNCTION))
			}
		} else if retValue == nil {
			storedReturnType = Nil
		} else {
			storedReturnType = retValue
		}
	}

	state.popCall()

	//check that the body does not contain forbidden node types.

	if expectedFunction, ok := findInMultivalue[*InoxFunction](options.expectedValue); ok && expectedFunction.visitCheckNode != nil {
		visitCheckNode := expectedFunction.visitCheckNode

		ast.Walk(
			n.Body,
			func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
				if _, isBody := node.(*ast.Block); isBody && node == n.Body {
					return ast.ContinueTraversal, nil
				}

				action, allowed, err := visitCheckNode(visitArgs{node, parent, scopeNode, ancestorChain, after}, expectedFunction.capturedLocals)
				if err != nil {
					return ast.StopTraversal, err
				}
				if !allowed {
					msg := THIS_EXPR_STMT_SYNTAX_IS_NOT_ALLOWED

					if expectedFunction.forbiddenNodeExplanation != "" {
						msg += "; " + expectedFunction.forbiddenNodeExplanation
					}
					state.addError(MakeSymbolicEvalError(node, state, msg))
					options.setActualValueMismatchIfNotNil()
					return ast.Prune, nil
				}
				return action, nil
			},
			nil,
		)
	}

return_function:
	return &InoxFunction{
		node:           n,
		nodeChunk:      state.currentChunk().Node,
		parameters:     params,
		parameterNames: paramNames,
		result:         storedReturnType,
		capturedLocals: capturedLocals,
	}, nil
}

func evalFunctionDeclaration(n *ast.FunctionDeclaration, state *State, options evalOptions) (_ Value, finalErr error) {
	nameIdent, ok := n.Name.(*ast.IdentifierLiteral)
	if !ok {
		return nil, nil
	}
	funcName := nameIdent.Name

	info, preDeclared := state.getGlobal(funcName)
	if preDeclared && !utils.Implements[*inoxFunctionToBeDeclared](info.value) { //properly declared
		return nil, nil
	}

	startValue := &InoxFunction{node: n.Function, result: ANY_SERIALIZABLE}

	if !preDeclared {
		if n.Function != nil && len(n.Function.CaptureList) == 0 {
			return nil, fmt.Errorf("internal error: a value should already be associated with the name %s", funcName)
		}
		//declare the function before checking it
		state.setGlobal(funcName, startValue, GlobalConst, n.Name, n.Name)
	} else {
		state.overrideGlobal(funcName, startValue)
	}

	if n.Function == nil {
		state.SetMostSpecificNodeValue(n.Name, startValue)
		state.SetGlobalScopeData(n, state.currentGlobalScopeData())
		return nil, nil
	}

	if state.recursiveFunctionName != "" {
		state.addError(MakeSymbolicEvalError(n, state, NESTED_RECURSIVE_FUNCTION_DECLARATION))
	} else {
		state.recursiveFunctionName = funcName
	}

	v, err := symbolicEval(n.Function, state)
	if err == nil {
		state.symbolicData.UpdateAllPreviousGlobalScopeDataWithInoxFunction(state.currentChunk().Node, funcName, v.(*InoxFunction))

		state.overrideGlobal(funcName, v)
		state.SetMostSpecificNodeValue(n.Name, v)
		state.SetGlobalScopeData(n, state.currentGlobalScopeData())
	}
	return nil, err
}

func evalFunctionPatternExpression(n *ast.FunctionPatternExpression, state *State) (_ Value, finalErr error) {

	//KEEP IN SYNC WITH EVALUATION OF FUNCTION EXPRESSIONS

	stateFork := state.fork()

	// create a local scope for the function
	stateFork.pushScope()
	defer stateFork.popScope()

	if self, ok := state.getNextSelf(); ok {
		stateFork.setSelf(self)
		defer stateFork.unsetSelf()
	}

	parameterTypes := make([]Value, len(n.Parameters))
	parameterNames := make([]string, len(n.Parameters))
	isVariadic := n.IsVariadic

	// declare arguments
	for paramIndex, p := range n.Parameters[:n.NonVariadicParamCount()] {
		name := "_"

		if p.Var != nil {
			paramNameIdent, ok := p.Var.(*ast.IdentifierLiteral)
			if !ok {
				return ANY_FUNCTION_PATTERN, nil
			}
			name = paramNameIdent.Name
		}

		var paramType Value = ANY

		if p.Type != nil {
			pattern, err := evalPatternNode(p.Type, stateFork)
			if err != nil {
				return nil, err
			}
			paramType = pattern.SymbolicValue()
		}

		parameterTypes[paramIndex] = paramType
		parameterNames[paramIndex] = name

		if p.Var != nil {
			stateFork.setLocal(name, paramType, nil, p.Var)
			state.SetMostSpecificNodeValue(p.Var, paramType)
		}
	}

	if n.IsVariadic {
		variadicParam := n.VariadicParameter()
		paramNameIdent, ok := variadicParam.Var.(*ast.IdentifierLiteral)
		if !ok {
			return ANY_FUNCTION_PATTERN, nil
		}

		name := paramNameIdent.Name

		var elemType Value = ANY

		if variadicParam.Type != nil {
			pattern, err := evalPatternNode(variadicParam.Type, stateFork)
			if err != nil {
				return nil, err
			}
			elemType = pattern.SymbolicValue()
		}

		paramType := ANY_ARRAY
		if elemType != ANY {
			paramType = NewArrayOf(elemType)
		}

		parameterTypes[len(parameterTypes)-1] = paramType
		parameterNames[len(parameterTypes)-1] = name

		stateFork.setLocal(name, paramType, nil, paramNameIdent)
		state.SetMostSpecificNodeValue(paramNameIdent, paramType)
	}

	//-----------------------------

	var returnType Value = Nil

	if n.ReturnType != nil {
		pattern, err := evalPatternNode(n.ReturnType, stateFork)
		if err != nil {
			return nil, err
		}
		typ := pattern.SymbolicValue()
		returnType = typ
	}

	//TODO: update firstOptionalParamIndex when function patterns support optional paramters

	function := NewFunction(parameterTypes, parameterNames, -1, isVariadic, []Value{returnType})
	function.patternNode = n
	function.patternNodeChunk = state.currentChunk().Node
	function.formattedPatternNodeLocation = state.currentChunk().GetFormattedNodeLocation(n)

	return &FunctionPattern{
		function: function,
	}, nil
}

func evalSynchronizedBlockStatement(n *ast.SynchronizedBlockStatement, state *State) (_ Value, finalErr error) {
	for _, valNode := range n.SynchronizedValues {
		val, err := symbolicEval(valNode, state)
		if err != nil {
			return nil, err
		}

		if !val.IsMutable() {
			continue
		}

		if potentiallySharable, ok := val.(PotentiallySharable); !ok || !utils.Ret0(potentiallySharable.IsSharable()) {
			state.addError(MakeSymbolicEvalError(n, state, fmtSynchronizedValueShouldBeASharableValueOrImmutableNot(val)))
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
}

func evalInclusionImportStatement(n *ast.InclusionImportStatement, state *State) (_ Value, finalErr error) {
	if state.Module == nil {
		panic(fmt.Errorf("cannot evaluate inclusion import statement: global state's module is nil"))
	}
	chunk, ok := state.Module.inclusionStatementMap[n]
	if !ok { //included file does not exist or is a folder
		return nil, nil
	}
	state.pushChunk(chunk.ParsedChunkSource, n)
	defer state.popChunk()

	_, err := symbolicEval(chunk.Node, state)
	state.SetLocalScopeData(n, state.currentLocalScopeData())
	state.SetGlobalScopeData(n, state.currentGlobalScopeData())
	state.symbolicData.SetContextData(n, state.ctx.currentData())
	return nil, err
}

func evalImportStatement(n *ast.ImportStatement, state *State) (_ Value, finalErr error) {

	setResultAsAny := func() {
		value := ANY
		state.setGlobal(n.Identifier.Name, value, GlobalConst)

		state.SetMostSpecificNodeValue(n.Identifier, value)
		state.SetGlobalScopeData(n, state.currentGlobalScopeData())
	}

	//Retrieve the imported module.

	var pathOrURL string

	switch src := n.Source.(type) {
	case *ast.RelativePathLiteral:
		pathOrURL = src.Value
	case *ast.AbsolutePathLiteral:
		pathOrURL = src.Value
	case *ast.URLLiteral:
		pathOrURL = src.Value
	default:
		panic(ErrUnreachable)
	}

	if !strings.HasSuffix(pathOrURL, inoxconsts.INOXLANG_FILE_EXTENSION) {
		state.addError(MakeSymbolicEvalError(n.Source, state, IMPORTED_MOD_PATH_MUST_END_WITH_IX))
		setResultAsAny()
		return nil, nil
	}

	importedModule, ok := state.Module.directlyImportedModules[n]
	if !ok {
		setResultAsAny()
		return nil, nil
	}

	//Create the context of the imported module.

	// TODO: use concrete context with permissions of imported module
	importedModuleContext := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
	for name, basePattern := range state.basePatterns {
		importedModuleContext.AddNamedPattern(name, basePattern, false)
	}

	for name, basePatternNamespace := range state.basePatternNamespaces {
		importedModuleContext.AddPatternNamespace(name, basePatternNamespace, false)
	}

	importPositions := append(slices.Clone(state.importPositions), state.getErrorMesssageLocation(n)...)

	//Check the imported module with a separate symbolic *State.

	data, err := EvalCheck(EvalCheckInput{
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

	//Set the value of the result variable.

	result, ok := data.moduleResults[importedModule.mainChunk.Node]
	if !ok {
		result = ANY
	}
	state.setGlobal(n.Identifier.Name, result, GlobalConst)

	state.SetMostSpecificNodeValue(n.Identifier, result)
	state.SetGlobalScopeData(n, state.currentGlobalScopeData())

	//Retrieve the parameters of the module by getting the value of mod-args.

	var moduleParams *ModuleParamsPattern

	modArgsVarData, _, ok := data.GetGlobalVarData(importedModule.mainChunk.Node, nil, globalnames.MOD_ARGS_VARNAME)
	if ok {
		moduleArgs, ok := modArgsVarData.Value.(*ModuleArgs)
		if ok {
			moduleParams = moduleArgs.typ
		}
	}

	//Evaluate the import configuration.

	objLit, ok := n.Configuration.(*ast.ObjectLiteral)

	if !ok { //parsing error
		return nil, nil
	}

	expectedArgumentsObject := EXACT_EMPTY_OBJECT

	if moduleParams != nil {
		expectedArgumentsObject = moduleParams.ArgumentsObject()
	}

	hasImportedModuleParameters := len(expectedArgumentsObject.PropertyNames()) > 0

	importConfig, err := _symbolicEval(objLit, state, evalOptions{
		expectedValue: NewInexactObject(
			map[string]Serializable{
				inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME: expectedArgumentsObject,
			},
			//The property is optional if the module does not have parameters.
			utils.If(!hasImportedModuleParameters, map[string]struct{}{inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME: {}}, nil),
			nil,
		),
	})

	if err != nil {
		return nil, err
	}

	configObject := importConfig.(*Object)

	_, _, hasProp := configObject.GetProperty(inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME)
	if !hasProp && hasImportedModuleParameters {
		state.addError(MakeSymbolicEvalError(objLit, state, THE_ARGUMENTS_PROP_IS_REQUIRED_IN_IMPORT_CONFIG_BECAUSE_IMPORTED_MODULE_HAS_PARAMS))
	}

	//We do not have to check $args against $expectedArgumentsObject because any mismatch
	//is reported during the evaluating of the object literal.

	return nil, nil
}

func evalSpawnExpression(node *ast.SpawnExpression, state *State) (_ Value, finalErr error) {
	var actualGlobals = map[string]Value{}
	var embeddedModule *ast.Chunk

	var meta map[string]Value
	var globals any
	var permListingNode *ast.ObjectLiteral

	//check permissions
	if !state.ctx.HasAPermissionWithKindAndType(permbase.Create, permbase.LTHREAD_PERM_TYPENAME) {
		warningSpan := sourcecode.NodeSpan{Start: node.Span.Start, End: node.Span.Start + 2}
		state.addWarning(makeSymbolicEvalWarningWithSpan(warningSpan, state, POSSIBLE_MISSING_PERM_TO_CREATE_A_LTHREAD))
	}

	meta = map[string]Value{}
	if objLit, ok := node.Meta.(*ast.ObjectLiteral); ok { //$ok will be false if node.Meta is nil

		for _, sectionProp := range objLit.Properties {
			if sectionProp.HasNoKey() {
				//okay because there sould be a static check error
				continue
			}
			sectionName := sectionProp.Name()

			if sectionName == LTHREAD_META_GLOBALS_SECTION {
				globalsObjectLit, ok := sectionProp.Value.(*ast.ObjectLiteral)
				//handle description separately if it's an object literal because non-serializable value are not accepted.
				if ok {
					globalMap := map[string]Value{}
					globals = globalMap

					for _, prop := range globalsObjectLit.Properties {
						if prop.HasNoKey() {
							//okay because there sould be a static check error
							continue
						}
						globalName := prop.Name() //okay since implicit-key properties are not allowed
						globalVal, err := symbolicEval(prop.Value, state)
						if err != nil {
							return nil, err
						}
						pattern, ok := state.getStaticOfNode(prop.Value)
						if ok {
							globalVal = pattern.SymbolicValue()
						}
						globalMap[globalName] = globalVal
					}
					continue
				}
			} else if sectionName == LTHREAD_META_ALLOW_SECTION && utils.Implements[*ast.ObjectLiteral](sectionProp.Value) {
				permListingNode = sectionProp.Value.(*ast.ObjectLiteral)
			}

			propertyVal, err := symbolicEval(sectionProp.Value, state)
			if err != nil {
				return nil, err
			}
			meta[sectionName] = propertyVal
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
		case LTHREAD_META_GLOBALS_SECTION:
			globals = v
		case LTHREAD_META_GROUP_SECTION:
			_, ok := v.(*LThreadGroup)
			if !ok {
				state.addError(MakeSymbolicEvalError(node.Meta, state, fmtGroupPropertyNotLThreadGroup(v)))
			}
		case LTHREAD_META_ALLOW_SECTION:
		default:
			state.addWarning(makeSymbolicEvalWarning(node.Meta, state, fmtUnknownSectionInLThreadMetadata(k)))
		}
	}

	switch g := globals.(type) {
	case map[string]Value:
		for k, v := range g {
			symVal, err := ShareOrClone(v, state)
			if err != nil {
				state.addError(MakeSymbolicEvalError(node.Meta, state, err.Error()))
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

	v, err := symbolicEval(node.Module, state)
	if err != nil {
		return nil, err
	}

	if symbolicNode, ok := v.(*AstNode); ok {
		if embeddedMod, ok := symbolicNode.Node.(*ast.Chunk); ok {
			embeddedModule = embeddedMod
		} else {
			varname := ast.GetVariableName(node.Module)
			state.addError(MakeSymbolicEvalError(node, state, fmtValueOfVarShouldBeAModuleNode(varname)))
		}
	} else {
		varname := ast.GetVariableName(node.Module)
		state.addError(MakeSymbolicEvalError(node, state, fmtValueOfVarShouldBeAModuleNode(varname)))
	}

	var concreteCtx ConcreteContext = state.ctx.startingConcreteContext
	if permListingNode != nil && extData.EstimatePermissionsFromListingNode != nil {
		perms, err := extData.EstimatePermissionsFromListingNode(permListingNode)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate permission of spawned lthread: %w", err)
		}
		concreteCtx = extData.CreateConcreteContext(perms)
	}

	_ = permListingNode

	//TODO: check the allow section to know the permissions
	modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, concreteCtx, state.ctx)
	modState := newSymbolicState(modCtx, &parse.ParsedChunkSource{
		Node: embeddedModule,
		ParsedChunkSourceBase: sourcecode.ParsedChunkSourceBase{
			Source: state.currentChunk().Source,
		},
	})
	modState.Module = state.Module
	modState.symbolicData = state.symbolicData

	for k, v := range actualGlobals {
		modState.setGlobal(k, v, GlobalConst)
	}

	if node.Module.SingleCallExpr {
		calleeNode := node.Module.Statements[0].(*ast.CallExpression).Callee

		switch calleeNode := calleeNode.(type) {
		case *ast.IdentifierLiteral:
			calleeName := calleeNode.Name
			info, ok := state.get(calleeName)
			if ok {
				modState.setGlobal(calleeName, info.value, GlobalConst)
			}
		case *ast.IdentifierMemberExpression:
			if calleeNode.Err != nil {
				break
			}

			varInfo, ok := state.get(calleeNode.Left.Name)

			if !ok {
				break
			}

			modState.setGlobal(calleeNode.Left.Name, varInfo.value, GlobalConst)

			_, ok = varInfo.value.(*Namespace)
			if !ok || len(calleeNode.PropertyNames) != 1 {
				state.addError(MakeSymbolicEvalError(calleeNode.Left, state, INVALID_SPAWN_EXPR_WITH_SHORTHAND_SYNTAX_CALLEE_SHOULD_BE_AN_FN_IDENTIFIER_OR_A_NAMESPACE_METHOD))
				return ANY_LTHREAD, nil
			}
		}

	}

	_, err = symbolicEval(embeddedModule, modState)
	if err != nil {
		return nil, err
	}

	for _, err := range modState.errors() {
		state.addError(err)
	}

	for _, warning := range modState.warnings() {
		state.addWarning(warning)
	}

	return ANY_LTHREAD, nil
}

func evalMappingExpression(n *ast.MappingExpression, state *State) (_ Value, finalErr error) {
	mapping := &Mapping{}

	for _, entry := range n.Entries {
		fork := state.fork()
		fork.pushScope()

		switch e := entry.(type) {
		case *ast.StaticMappingEntry:
			_, err := symbolicEval(e.Value, fork)
			if err != nil {
				return nil, err
			}
		case *ast.DynamicMappingEntry:
			key, err := symbolicEval(e.Key, fork)
			if err != nil {
				return nil, err
			}

			keyVarname := e.KeyVar.(*ast.IdentifierLiteral).Name
			keyVal := key
			if patt, ok := key.(Pattern); ok {
				keyVal = patt.SymbolicValue()
			}
			fork.setLocal(keyVarname, keyVal, nil, e.KeyVar)
			state.SetMostSpecificNodeValue(e.KeyVar, keyVal)

			if e.GroupMatchingVariable != nil {
				matchingVarName := e.GroupMatchingVariable.(*ast.IdentifierLiteral).Name
				anyObj := NewAnyObject()
				fork.setLocal(matchingVarName, anyObj, nil, e.GroupMatchingVariable)
				state.SetMostSpecificNodeValue(e.GroupMatchingVariable, anyObj)
			}

			_, err = symbolicEval(e.ValueComputation, fork)
			if err != nil {
				return nil, err
			}
		}
	}

	return mapping, nil
}

func evalTreedataLiteral(n *ast.TreedataLiteral, state *State, options evalOptions) (Value, error) {

	value, err := symbolicEval(n.Root, state)
	if err != nil {
		return nil, err
	}

	if value.IsMutable() {
		state.addError(MakeSymbolicEvalError(n.Root, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
	} else if _, ok := AsSerializable(value).(Serializable); !ok {
		state.addError(MakeSymbolicEvalError(n.Root, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
	}

	for _, child := range n.Children {
		_, err := symbolicEval(child, state)
		if err != nil {
			return nil, err
		}
	}
	return &Treedata{}, nil
}

func evalTreedataEntry(n *ast.TreedataEntry, state *State, options evalOptions) (Value, error) {
	value, err := symbolicEval(n.Value, state)
	if err != nil {
		return nil, err
	}

	if value.IsMutable() {
		state.addError(MakeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
	} else if _, ok := AsSerializable(value).(Serializable); !ok {
		state.addError(MakeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
	}

	for _, child := range n.Children {
		_, err := symbolicEval(child, state)
		if err != nil {
			return nil, err
		}
	}

	return &TreedataHiearchyEntry{}, nil
}

func evalTreedataPair(n *ast.TreedataPair, state *State, options evalOptions) (Value, error) {
	value, err := symbolicEval(n.Key, state)
	if err != nil {
		return nil, err
	}

	var (
		first  Serializable
		second Serializable = ANY_SERIALIZABLE
	)

	if value.IsMutable() {
		state.addError(MakeSymbolicEvalError(n.Key, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
		first = ANY_SERIALIZABLE
	} else if serializable, ok := AsSerializable(value).(Serializable); ok {
		first = serializable
	} else {
		state.addError(MakeSymbolicEvalError(n.Key, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
		first = ANY_SERIALIZABLE
	}

	if n.Value != nil {
		value, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		if value.IsMutable() {
			state.addError(MakeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
			second = ANY_SERIALIZABLE
		} else if serializable, ok := AsSerializable(value).(Serializable); ok {
			second = serializable
		} else {
			state.addError(MakeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
			second = ANY_SERIALIZABLE
		}
	}

	return NewOrderedPair(first, second), nil
}

func evalObjectLiteral(n *ast.ObjectLiteral, state *State, options evalOptions) (Value, error) {
	entries := map[string]Serializable{}

	var (
		keys  []string
		props []*ast.ObjectProperty
	)

	var noKeyProps []*ast.ObjectProperty

	//get all keys and properties without a key.
	for _, p := range n.Properties {
		var key string
		hasKey := true

		//add the key
		switch n := p.Key.(type) {
		case *ast.DoubleQuotedStringLiteral:
			key = n.Value
		case *ast.IdentifierLiteral:
			key = n.Name
		case nil: //no key
			hasKey = false
		default:
			return nil, fmt.Errorf("invalid key type %T", n)
		}

		if hasKey && key == inoxconsts.IMPLICIT_PROP_NAME { //not allowed (static check error)
			continue
		}

		if hasKey {
			keys = append(keys, key)
			props = append(props, p)
		} else {
			noKeyProps = append(noKeyProps, p)
		}
	}

	for _, el := range n.SpreadElements {
		_, isExtractionExpr := el.Expr.(*ast.ExtractionExpression)

		evaluatedElement, err := symbolicEval(el.Expr, state)
		if err != nil {
			return nil, err
		}

		if !isExtractionExpr {
			continue
		}

		object := evaluatedElement.(*Object)

		for _, key := range el.Expr.(*ast.ExtractionExpression).Keys.Keys {
			name := key.(*ast.IdentifierLiteral).Name
			v, _, ok := object.GetProperty(name)
			if !ok {
				panic(fmt.Errorf("missing property %s", name))
			}

			serializable, ok := AsSerializable(v).(Serializable)
			if !ok {
				state.addError(MakeSymbolicEvalError(el, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				serializable = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(v).(Watchable); !ok && v.IsMutable() {
				state.addError(MakeSymbolicEvalError(el, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
			}

			entries[name] = serializable
		}
	}

	expectedObj, ok := findInMultivalue[*Object](options.expectedValue)
	if !ok {
		expectedObj = &Object{}
	}

	isExact := options.neverModifiedArgument && len(n.SpreadElements) == 0

	obj := NewObject(isExact, entries, nil, nil)
	if expectedObj.readonly {
		obj.readonly = true
	}

	prevNextSelf, restoreNextSelf := state.getNextSelf()
	if restoreNextSelf {
		state.unsetNextSelf()
	}
	state.setNextSelf(obj)

	//add allowed missing properties
	{
		var properties []string
		expectedObj.ForEachEntry(func(propName string, propValue Value) error {
			if slices.Contains(keys, propName) {
				return nil
			}
			properties = append(properties, propName)
			return nil
		})
		sort.Strings(properties)
		state.symbolicData.SetAllowedNonPresentProperties(n, properties)
	}

	//evaluate all properties
	for i, key := range keys {
		p := props[i]
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

			state.addError(MakeSymbolicEvalError(p.Key, state, msg))
		}

		var (
			propVal         Value
			err             error
			serializable    Serializable
			hasShallowError = false
		)

		if p.Value == nil {
			propVal = ANY_SERIALIZABLE
			serializable = ANY_SERIALIZABLE

			if expectedPropVal != nil && !IsAnySerializable(expectedPropVal) {
				options.setActualValueMismatchIfNotNil()
			}
		} else {
			propVal, err = _symbolicEval(p.Value, state, evalOptions{
				expectedValue:       expectedPropVal,
				actualValueMismatch: &deeperMismatch,
				hasShallowError:     &hasShallowError,
			})
			if err != nil {
				return nil, err
			}

			if p.Type != nil {
				_propType, err := symbolicEval(p.Type, state)
				if err != nil {
					return nil, err
				}
				static = _propType.(Pattern)

				if !static.TestValue(propVal, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
					expected := static.SymbolicValue()
					if !hasShallowError {
						msg, regions := fmtNotAssignableToPropOfType(state.fmtHelper, propVal, expected, nil)
						state.addError(MakeSymbolicEvalError(p.Value, state, msg, regions...))
					}
					propVal = expected
				}
			} else if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(propVal, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
				options.setActualValueMismatchIfNotNil()
				if !hasShallowError {
					msg, regions := fmtNotAssignableToPropOfType(state.fmtHelper, propVal, expectedPropVal, state.testCallMessageBuffer)
					state.addError(MakeSymbolicEvalError(p.Value, state, msg, regions...))
				}
			}

			serializable, ok = AsSerializable(propVal).(Serializable)
			if !ok {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				serializable = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(propVal).(Watchable); !ok && propVal.IsMutable() {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
			}

			//additional checks if expected object is readonly
			if expectedObj.readonly && !IsReadonlyOrImmutable(propVal) {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p.Key, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE))
			}
		}

		obj.initNewProp(key, serializable, static)
		state.SetMostSpecificNodeValue(p.Key, propVal)
	}

	//Evaluate elements.
	var noKeyValues []Serializable
	for _, p := range noKeyProps {
		propVal, err := symbolicEval(p.Value, state)
		if err != nil {
			return nil, err
		}

		state.SetMostSpecificNodeValue(p.Value, propVal)

		if expectedObj.readonly && !IsReadonlyOrImmutable(propVal) {
			state.addError(MakeSymbolicEvalError(p.Value, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE))

			noKeyValues = append(noKeyValues, ANY_SERIALIZABLE)
		} else {
			serializable, ok := AsSerializable(propVal).(Serializable)
			if !ok {
				state.addError(MakeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				serializable = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(propVal).(Watchable); !ok && propVal.IsMutable() {
				state.addError(MakeSymbolicEvalError(p, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
			}
			noKeyValues = append(noKeyValues, serializable)
		}
	}

	if len(noKeyValues) > 0 {
		obj.initNewProp(inoxconsts.IMPLICIT_PROP_NAME, NewList(noKeyValues...), nil)
	}

	state.unsetNextSelf()
	if restoreNextSelf {
		state.setNextSelf(prevNextSelf)
	}

	// evaluate meta properties

	for _, p := range n.MetaProperties {
		switch p.Name() {
		case inoxconsts.CONSTRAINTS_KEY:
			if err := handleConstraints(obj, p.Initialization, state); err != nil {
				return nil, err
			}
		case inoxconsts.VISIBILITY_KEY:
			//
		default:
			state.addError(MakeSymbolicEvalError(p, state, fmtCannotInitializedMetaProp(p.Name())))
		}
	}

	return obj, nil
}

func evalRecordLiteral(n *ast.RecordLiteral, state *State, options evalOptions) (Value, error) {
	entries := map[string]Serializable{}
	rec := NewBoundEntriesRecord(entries)

	var (
		keys       []string
		props      []*ast.ObjectProperty
		noKeyProps []*ast.ObjectProperty
	)

	//get all keys and properties without a key.
	for _, p := range n.Properties {
		var key string
		hasKey := true

		//add the key
		switch n := p.Key.(type) {
		case *ast.DoubleQuotedStringLiteral:
			key = n.Value
		case *ast.IdentifierLiteral:
			key = n.Name
		case nil: //no key
			hasKey = false
		default:
			return nil, fmt.Errorf("invalid key type %T", n)
		}

		if hasKey && key == inoxconsts.IMPLICIT_PROP_NAME { //not allowed (static check error)
			continue
		}

		if hasKey {
			keys = append(keys, key)
			props = append(props, p)
		} else {
			noKeyProps = append(noKeyProps, p)
		}
	}

	expectedRecord, ok := findInMultivalue[*Record](options.expectedValue)
	if ok && expectedRecord.entries != nil {
		var properties []string
		expectedRecord.ForEachEntry(func(propName string, _ Value) error {
			if slices.Contains(keys, propName) {
				return nil
			}
			properties = append(properties, propName)
			return nil
		})

		sort.Strings(properties)
		state.symbolicData.SetAllowedNonPresentProperties(n, properties)
	} else {
		expectedRecord = &Record{}
	}

	//evaluate properties
	for i, p := range props {
		key := keys[i]

		expectedPropVal := expectedRecord.entries[key]
		deeperMismatch := false
		hasShallowError := false

		if p.Value == nil {
			entries[key] = ANY_SERIALIZABLE

			if expectedPropVal != nil && !IsAnySerializable(expectedPropVal) {
				options.setActualValueMismatchIfNotNil()
			}
		} else {
			v, err := _symbolicEval(p.Value, state, evalOptions{
				expectedValue:       expectedPropVal,
				actualValueMismatch: &deeperMismatch,
				hasShallowError:     &hasShallowError,
			})
			if err != nil {
				return nil, err
			}

			if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(v, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
				options.setActualValueMismatchIfNotNil()
				if !hasShallowError {
					msg, regions := fmtNotAssignableToPropOfType(state.fmtHelper, v, expectedPropVal, state.testCallMessageBuffer)
					state.addError(MakeSymbolicEvalError(p.Value, state, msg, regions...))
				}
			}

			serializable, ok := AsSerializable(v).(Serializable)
			if !ok {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				entries[key] = ANY_SERIALIZABLE
			} else if v.IsMutable() {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p.Value, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable(key)))
				entries[key] = ANY_SERIALIZABLE
			} else {
				entries[key] = serializable
			}
		}
	}

	//evaluate elements.
	var noKeyValues []Serializable
	for _, p := range noKeyProps {
		hasShallowError := false

		propVal, err := _symbolicEval(p.Value, state, evalOptions{hasShallowError: &hasShallowError})
		if err != nil {
			return nil, err
		}

		state.SetMostSpecificNodeValue(p.Value, propVal)

		serializable, ok := AsSerializable(propVal).(Serializable)
		if !ok {
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p.Value, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
			serializable = ANY_SERIALIZABLE
		} else if propVal.IsMutable() {
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(p.Value, state, INVALID_ELEM_ELEMS_OF_RECORD_SHOULD_BE_IMMUTABLE))
			serializable = ANY_SERIALIZABLE
		}

		noKeyValues = append(noKeyValues, serializable)
	}

	if len(noKeyValues) > 0 {
		entries[inoxconsts.IMPLICIT_PROP_NAME] = NewTuple(noKeyValues...)
	}

	for _, el := range n.SpreadElements {
		state.addError(MakeSymbolicEvalError(el, state, PROP_SPREAD_IN_REC_NOT_SUPP_YET))
		break
		// evaluatedElement, err := symbolicEval(el.Expr, state)
		// if err != nil {
		// 	return nil, err
		// }

		// object := evaluatedElement.(*SymbolicObject)

		// for _, key := range el.Expr.(*ast.ExtractionExpression).Keys.Keys {
		// 	name := key.(*ast.IdentifierLiteral).Name
		// 	v, ok := object.getProperty(name)
		// 	if !ok {
		// 		panic(fmt.Errorf("missing property %s", name))
		// 	}
		// 	rec.updateProperty(name, v)
		// }
	}

	return rec, nil
}

func evalListLiteral(n *ast.ListLiteral, state *State, options evalOptions) (Value, error) {
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
			var e Value
			hasShallowError := false

			spreadElemNode, ok := elemNode.(*ast.ElementSpreadElement)
			if ok {
				val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{
					expectedValue:       resultList,
					actualValueMismatch: &deeperMismatch,
					hasShallowError:     &hasShallowError,
				})
				if err != nil {
					return nil, err
				}

				list, isList := val.(*List)
				if isList {
					e = list.Element()
				} else {
					state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST))
					e = generalElem
				}
			} else {
				e, err = _symbolicEval(elemNode, state, evalOptions{
					expectedValue:       generalElem,
					actualValueMismatch: &deeperMismatch,
					hasShallowError:     &hasShallowError,
				})
				if err != nil {
					return nil, err
				}
			}

			if hasShallowError {
				continue
			}

			if !generalElem.Test(e, RecTestCallState{}) && !deeperMismatch {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListAnnotated(e, generalElemPattern.(Pattern))))
			}

			e = AsSerializable(e)
			_, ok = e.(Serializable)
			if !ok {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
				e = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(e).(Watchable); !ok && e.IsMutable() {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
			}
		}

		return resultList, nil
	}

	var expectedElement Value = nil
	if expectedList != nil {
		expectedElement = expectedList.Element()
	} else {
		//we do not search for a Sequence because we could find a sequence that is not a list
		expectedSeq, ok := findInMultivalue[*AnySequenceOf](options.expectedValue)
		if ok {
			expectedElement = expectedSeq.Element()
		}
	}

	if len(n.Elements) == 0 {
		if expectedList != nil && expectedList.readonly {
			return EMPTY_READONLY_LIST, nil
		}
		return EMPTY_LIST, nil
	}

	for _, elemNode := range n.Elements {
		var e Value
		deeperMismatch := false
		hasShallowError := false

		spreadElemNode, ok := elemNode.(*ast.ElementSpreadElement)
		if ok {
			val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{
				expectedValue:       expectedElement,
				actualValueMismatch: &deeperMismatch,
				hasShallowError:     &hasShallowError,
			})
			if err != nil {
				return nil, err
			}

			list, isList := val.(*List)
			if isList {
				e = list.Element()
			} else {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST))
				if expectedElement != nil {
					e = expectedElement
				} else {
					continue
				}
			}
		} else {
			var err error
			e, err = _symbolicEval(elemNode, state, evalOptions{
				expectedValue:       expectedElement,
				actualValueMismatch: &deeperMismatch,
				hasShallowError:     &hasShallowError,
			})
			if err != nil {
				return nil, err
			}
		}

		if deeperMismatch {
			options.setActualValueMismatchIfNotNil()
		} else if expectedElement != nil && !expectedElement.Test(e, RecTestCallState{}) && !deeperMismatch {
			options.setActualValueMismatchIfNotNil()
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListofValues(e, expectedElement)))
		}

		e = AsSerializable(e)
		_, ok = e.(Serializable)
		if !ok {
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
			e = ANY_SERIALIZABLE
		} else if _, ok := asWatchable(e).(Watchable); !ok && e.IsMutable() {
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
		}

		elements = append(elements, AsSerializableChecked(e))
	}

	resultList := NewList(elements...)
	if expectedList != nil && expectedList.readonly {
		resultList.readonly = true
	}
	return resultList, nil
}

func evalTupleLiteral(n *ast.TupleLiteral, state *State, options evalOptions) (Value, error) {
	elements := make([]Serializable, 0)

	if n.TypeAnnotation != nil {
		generalElemPattern, err := symbolicEval(n.TypeAnnotation, state)
		if err != nil {
			return nil, err
		}

		generalElem := generalElemPattern.(Pattern).SymbolicValue().(Serializable)
		deeperMismatch := false
		hasShallowError := false

		for _, elemNode := range n.Elements {
			spreadElemNode, ok := elemNode.(*ast.ElementSpreadElement)
			var e Value
			if ok {
				val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{hasShallowError: &hasShallowError})
				if err != nil {
					return nil, err
				}

				tuple, isTuple := val.(*Tuple)
				if isTuple {
					e = tuple.Element()
				} else {
					state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_TUPLE))
					e = generalElem
				}
			} else {
				e, err = _symbolicEval(elemNode, state, evalOptions{expectedValue: generalElem, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}
			}

			if e.IsMutable() {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE))
				e = ANY_SERIALIZABLE
			}

			e = AsSerializable(e)
			_, ok = e.(Serializable)
			if !ok {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
				e = ANY_SERIALIZABLE
			}

			if !generalElem.Test(e, RecTestCallState{}) {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(e, generalElemPattern.(Pattern))))
			}
		}

		return NewTupleOf(generalElem), nil
	}

	expectedTuple, ok := findInMultivalue[*Tuple](options.expectedValue)
	var expectedElement Value = nil
	if ok {
		expectedElement = expectedTuple.Element()
	} else {
		//we do not search for a Sequence because we could find a sequence that is not a tuple
		expectedSeq, ok := findInMultivalue[*AnySequenceOf](options.expectedValue)
		if ok {
			expectedElement = expectedSeq.Element()
		}
	}

	if len(n.Elements) == 0 {
		return EMPTY_TUPLE, nil
	}

	for _, elemNode := range n.Elements {
		var e Value
		deeperMismatch := false
		hasShallowError := false

		spreadElemNode, ok := elemNode.(*ast.ElementSpreadElement)
		if ok {
			val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{
				expectedValue:       expectedElement,
				actualValueMismatch: &deeperMismatch,
				hasShallowError:     &hasShallowError,
			})
			if err != nil {
				return nil, err
			}

			tuple, isTuple := val.(*Tuple)
			if isTuple {
				e = tuple.Element()
			} else {
				state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_TUPLE))
				if expectedElement != nil {
					e = expectedElement
				} else {
					continue
				}
			}
		} else {
			var err error
			e, err = _symbolicEval(elemNode, state, evalOptions{
				expectedValue:       expectedElement,
				actualValueMismatch: &deeperMismatch,
				hasShallowError:     &hasShallowError,
			})
			if err != nil {
				return nil, err
			}
		}

		if deeperMismatch {
			options.setActualValueMismatchIfNotNil()
		} else if expectedElement != nil && !expectedElement.Test(e, RecTestCallState{}) && !deeperMismatch {
			options.setActualValueMismatchIfNotNil()
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListofValues(e, expectedElement)))
		}

		if e.IsMutable() {
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE))
			e = ANY_SERIALIZABLE
		}

		e = AsSerializable(e)
		_, ok = e.(Serializable)
		if !ok {
			state.addErrorIf(!hasShallowError, MakeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
			e = ANY_SERIALIZABLE
		}

		elements = append(elements, AsSerializableChecked(e))
	}
	return NewTuple(elements...), nil
}

func evalDictionaryLiteral(n *ast.DictionaryLiteral, state *State, options evalOptions) (Value, error) {

	entries := make(map[string]Serializable)
	keys := make(map[string]Serializable)

	expectedDictionary, ok := findInMultivalue[*Dictionary](options.expectedValue)
	if ok && expectedDictionary.entries != nil {
		var keys []string
		expectedDictionary.ForEachEntry(func(_ Serializable, keyRepr string, _ Value) error {
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
		keyRepr := parse.SPrint(entry.Key, state.currentChunk().Node, parse.PrintConfig{})

		expectedEntryValue, _ := expectedDictionary.get(keyRepr)
		deeperMismatch := false

		//Evaluate the value

		entryValue, err := _symbolicEval(entry.Value, state, evalOptions{expectedValue: expectedEntryValue, actualValueMismatch: &deeperMismatch})
		if err != nil {
			return nil, err
		}

		//Check the value

		if deeperMismatch {
			options.setActualValueMismatchIfNotNil()
		} else if expectedEntryValue != nil && !deeperMismatch && !expectedEntryValue.Test(entryValue, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
			options.setActualValueMismatchIfNotNil()

			msg, regions := fmtNotAssignableToEntryOfExpectedValue(state.fmtHelper, entryValue, expectedEntryValue, state.testCallMessageBuffer)
			state.addError(MakeSymbolicEvalError(entry.Value, state, msg, regions...))
		}

		serializable, ok := AsSerializable(entryValue).(Serializable)
		if !ok {
			state.addError(MakeSymbolicEvalError(entry.Value, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
			entryValue = ANY_SERIALIZABLE
		} else {
			entryValue = serializable
			if _, ok := asWatchable(entryValue).(Watchable); !ok && entryValue.IsMutable() {
				state.addError(MakeSymbolicEvalError(entry.Value, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
			}
		}

		//Evaluate the key

		entryKey, err := symbolicEval(entry.Key, state)
		if err != nil {
			return nil, err
		}

		//Check the key

		serializable, ok = AsSerializable(entryKey).(Serializable)
		if !ok {
			state.addError(MakeSymbolicEvalError(entry.Key, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE))
			entryKey = ANY_SERIALIZABLE
		} else {
			entryKey = serializable
			if _, ok := asWatchable(entryKey).(Watchable); !ok && entryKey.IsMutable() {
				state.addError(MakeSymbolicEvalError(entry.Key, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE))
			}
		}

		entries[keyRepr] = entryValue.(Serializable)
		keys[keyRepr] = entryKey.(Serializable)
		state.SetMostSpecificNodeValue(entry.Key, entryKey)
	}

	return NewDictionary(entries, keys), nil
}

func evalObjectPatternLiteral(n *ast.ObjectPatternLiteral, state *State, options evalOptions) (Value, error) {

	pattern := &ObjectPattern{
		entries: make(map[string]Pattern),
		inexact: !n.Exact(),
	}

	for _, el := range n.SpreadElements {
		compiledElement, err := evalPatternNode(el.Expr, state)
		if err != nil {
			return nil, err
		}

		if objPattern, ok := compiledElement.(*ObjectPattern); ok {
			if objPattern.entries == nil {
				state.addError(MakeSymbolicEvalError(el, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT))
			} else {
				for name, vpattern := range objPattern.entries {
					if _, alreadyPresent := pattern.entries[name]; alreadyPresent {
						state.addError(MakeSymbolicEvalError(el, state, fmtPropertyShouldNotBePresentInSeveralSpreadPatterns(name)))
						continue
					}
					pattern.entries[name] = vpattern
				}
			}
			// else if objPattern.Inexact {
			// state.addError(makeSymbolicEvalError(el, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT))
			//

		} else {
			state.addError(MakeSymbolicEvalError(el, state, fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(compiledElement)))
		}
	}

	for _, p := range n.Properties {
		name := p.Name()
		var propertyValuePattern Pattern
		var err error

		if p.Value == nil {
			propertyValuePattern = &TypePattern{val: ANY_SERIALIZABLE}
		} else {
			prevErrCount := len(state.errors())

			propertyValuePattern, err = evalPatternNode(p.Value, state)
			if err != nil {
				return nil, err
			}

			//check that the pattern has serializable values
			_, ok := AsSerializable(propertyValuePattern.SymbolicValue()).(Serializable)

			if !ok {
				if len(state.errors()) > prevErrCount {
					//don't add an irrelevant error

					propertyValuePattern = &TypePattern{val: ANY_SERIALIZABLE}
				} else {
					state.addError(MakeSymbolicEvalError(p.Value, state, PROPERTY_PATTERNS_IN_OBJECT_AND_REC_PATTERNS_MUST_HAVE_SERIALIZABLE_VALUEs))
					propertyValuePattern = &TypePattern{val: ANY_SERIALIZABLE}
				}
			}
		}

		pattern.entries[name] = propertyValuePattern

		if state.symbolicData != nil {
			val, ok := state.symbolicData.GetMostSpecificNodeValue(p.Value)
			if ok {
				state.SetMostSpecificNodeValue(p.Key, val)
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
}

func evalRecordPatternLiteral(n *ast.RecordPatternLiteral, state *State, options evalOptions) (Value, error) {
	pattern := &RecordPattern{
		entries: make(map[string]Pattern),
		inexact: !n.Exact(),
	}
	for _, el := range n.SpreadElements {
		compiledElement, err := evalPatternNode(el.Expr, state)
		if err != nil {
			return nil, err
		}

		if recPattern, ok := compiledElement.(*RecordPattern); ok {
			if recPattern.entries == nil {
				state.addError(MakeSymbolicEvalError(el, state, CANNOT_SPREAD_REC_PATTERN_THAT_MATCHES_ANY_RECORD))
			} else {
				for name, vpattern := range recPattern.entries {
					if _, alreadyPresent := pattern.entries[name]; alreadyPresent {
						state.addError(MakeSymbolicEvalError(el, state, fmtPropertyShouldNotBePresentInSeveralSpreadPatterns(name)))
						continue
					}
					pattern.entries[name] = vpattern
				}
			}
		} else {
			state.addError(MakeSymbolicEvalError(el, state, fmtPatternSpreadInRecordPatternShouldBeAnRecordPatternNot(compiledElement)))
		}
	}

	for _, p := range n.Properties {
		name := p.Name()

		prevErrCount := len(state.errors())

		if p.Value == nil {
			pattern.entries[name] = &TypePattern{val: ANY_SERIALIZABLE}
		} else {
			entryPattern, err := evalPatternNode(p.Value, state)
			if err != nil {
				return nil, err
			}

			errorDuringEval := len(state.errors()) > prevErrCount

			if _, ok := entryPattern.(*AnyPattern); ok && errorDuringEval {
				//AnyPattern may be present due to an issue (invalid pattern call) so
				//we handle this case separately
				pattern.entries[name] = &TypePattern{val: ANY_SERIALIZABLE}
			} else if entryPattern.SymbolicValue().IsMutable() {
				state.addError(MakeSymbolicEvalError(p.Value, state, fmtEntriesOfRecordPatternShouldMatchOnlyImmutableValues(name)))
				pattern.entries[name] = &TypePattern{val: ANY_SERIALIZABLE}
			} else {
				//check that the pattern has serializable values
				_, ok := AsSerializable(entryPattern.SymbolicValue()).(Serializable)

				if !ok {
					if errorDuringEval {
						//don't add an irrelevant error

						entryPattern = &TypePattern{val: ANY_SERIALIZABLE}
					} else {
						state.addError(MakeSymbolicEvalError(p.Value, state, PROPERTY_PATTERNS_IN_OBJECT_AND_REC_PATTERNS_MUST_HAVE_SERIALIZABLE_VALUEs))
						entryPattern = &TypePattern{val: ANY_SERIALIZABLE}
					}
				}

				pattern.entries[name] = entryPattern
			}
		}

		if state.symbolicData != nil {
			val, ok := state.symbolicData.GetMostSpecificNodeValue(p.Value)
			if ok {
				state.SetMostSpecificNodeValue(p.Key, val)
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
}

func evalListPatternLiteral(n *ast.ListPatternLiteral, state *State, options evalOptions) (Value, error) {
	pattern := &ListPattern{}

	if n.GeneralElement != nil {
		var err error
		pattern.generalElement, err = evalPatternNode(n.GeneralElement, state)
		if err != nil {
			return nil, err
		}

		//TODO: cache .SymbolicValue() for big patterns
		if _, ok := AsSerializable(pattern.generalElement.SymbolicValue()).(Serializable); !ok {
			pattern.generalElement = &TypePattern{val: ANY_SERIALIZABLE}
			state.addError(MakeSymbolicEvalError(n.GeneralElement, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
		}
	} else {
		pattern.elements = make([]Pattern, 0)

		for _, e := range n.Elements {
			elemPattern, err := evalPatternNode(e, state)
			if err != nil {
				return nil, err
			}

			if _, ok := AsSerializable(elemPattern.SymbolicValue()).(Serializable); !ok {
				elemPattern = &TypePattern{val: ANY_SERIALIZABLE}
				state.addError(MakeSymbolicEvalError(e, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
			}

			pattern.elements = append(pattern.elements, elemPattern)
		}
	}

	return pattern, nil
}

func evalTuplePatternLiteral(n *ast.TuplePatternLiteral, state *State, options evalOptions) (Value, error) {
	pattern := &TuplePattern{}

	if n.GeneralElement != nil {
		var err error
		pattern.generalElement, err = evalPatternNode(n.GeneralElement, state)
		if err != nil {
			return nil, err
		}

		generalElement := pattern.generalElement.SymbolicValue()

		if generalElement.IsMutable() {
			state.addError(MakeSymbolicEvalError(n.GeneralElement, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES))
			pattern.generalElement = &TypePattern{val: ANY_SERIALIZABLE}
		} else if _, ok := AsSerializable(generalElement).(Serializable); !ok {
			pattern.generalElement = &TypePattern{val: ANY_SERIALIZABLE}
			state.addError(MakeSymbolicEvalError(n.GeneralElement, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
		}

	} else {
		pattern.elements = make([]Pattern, 0)

		for _, e := range n.Elements {
			elemPattern, err := evalPatternNode(e, state)
			if err != nil {
				return nil, err
			}

			element := elemPattern.SymbolicValue()

			if element.IsMutable() {
				state.addError(MakeSymbolicEvalError(e, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES))
				elemPattern = &TypePattern{val: ANY_SERIALIZABLE}
			} else if _, ok := AsSerializable(element).(Serializable); !ok {
				elemPattern = &TypePattern{val: ANY_SERIALIZABLE}
				state.addError(MakeSymbolicEvalError(e, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
			}

			pattern.elements = append(pattern.elements, elemPattern)
		}
	}

	return pattern, nil
}

func evalConcatenationExpression(n *ast.ConcatenationExpression, state *State, options evalOptions) (Value, error) {
	if len(n.Elements) == 0 {
		return nil, errors.New("cannot create concatenation with no elements")
	}
	var values []Value
	var nodeIndexes []int
	atLeastOneSpread := false

	for elemNodeIndex, elemNode := range n.Elements {
		spreadElem, ok := elemNode.(*ast.ElementSpreadElement)
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
				state.addError(MakeSymbolicEvalError(elemNode, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION))
			}
		} else {
			state.addError(MakeSymbolicEvalError(n, state, SPREAD_ELEMENT_SHOULD_BE_ITERABLE))
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
				state.addError(MakeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmtStringConcatInvalidElementOfType(elem)))
			}
		}
		//We don't know if the result will be a String or a StringConcatenation.
		return ANY_STR_LIKE, nil
	case BytesLike:
		if len(values) == 1 && !atLeastOneSpread {
			return values[0], nil
		}
		for i, elem := range values {
			if _, ok := elem.(BytesLike); !ok {
				state.addError(MakeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmt.Sprintf("bytes concatenation: invalid element of type %T", elem)))
			}
		}
		//We don't know if the result will be a ByteSlice or a BytesConcatenation.
		return ANY_BYTES_LIKE, nil
	case *Tuple:
		if len(values) == 1 && !atLeastOneSpread {
			return values[0], nil
		}

		var generalElements []Value
		var elements []Serializable

		for i, concatElem := range values {
			if tuple, ok := concatElem.(*Tuple); ok {
				if tuple.HasKnownLen() {
					elements = append(elements, tuple.elements...)
				} else {
					generalElements = append(generalElements, tuple.generalElement)
				}
			} else {
				state.addError(MakeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmt.Sprintf("tuple concatenation: invalid element of type %T", concatElem)))
			}
		}

		if elements == nil {
			return NewTupleOf(AsSerializableChecked(joinValues(generalElements))), nil
		} else {
			return NewTuple(elements...), nil
		}
	default:
		state.addError(MakeSymbolicEvalError(n, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION))
		return ANY, nil
	}
}

func evalExtractionExpression(n *ast.ExtractionExpression, state *State, options evalOptions) (Value, error) {
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
	case IProps:
	default:
		ignoreProps = true
		state.addError(MakeSymbolicEvalError(n.Object, state, fmtValueHasNoProperties(left)))
	}

	for _, key := range n.Keys.Keys {
		name := key.(*ast.IdentifierLiteral).Name

		if ignoreProps {
			result.entries[name] = ANY_SERIALIZABLE
			result.static[name] = getStatic(ANY_SERIALIZABLE)
		} else {
			result.entries[name] = symbolicMemb(left, name, unspecifiedMemberAccess, n, state).(Serializable)
			result.static[name] = getStatic(result.entries[name])
		}
	}
	return result, nil
}

func evalIndexExpression(n *ast.IndexExpression, state *State, options evalOptions) (Value, error) {
	val, err := _symbolicEval(n.Indexed, state, evalOptions{
		doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), n),
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
		state.addError(MakeSymbolicEvalError(n, state, fmtIndexIsNotAnIntButA(index)))
		index = &Int{}
	}

	if indexable, ok := asIndexable(val).(Indexable); ok {
		if intIndex != nil && intIndex.hasValue && indexable.HasKnownLen() && (intIndex.value < 0 || intIndex.value >= int64(indexable.KnownLen())) {
			state.addError(MakeSymbolicEvalError(n.Index, state, INDEX_IS_OUT_OF_BOUNDS))
		}
		return indexable.Element(), nil
	}

	state.addError(MakeSymbolicEvalError(n, state, fmtXisNotIndexable(val)))
	return ANY, nil
}

func evalSliceExpression(n *ast.SliceExpression, state *State, options evalOptions) (Value, error) {
	slice, err := _symbolicEval(n.Indexed, state, evalOptions{
		doubleColonExprAncestorChain: append(slices.Clone(options.doubleColonExprAncestorChain), n),
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
			state.addError(MakeSymbolicEvalError(n, state, fmtStartIndexIsNotAnIntButA(index)))
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
			state.addError(MakeSymbolicEvalError(n, state, fmtEndIndexIsNotAnIntButA(index)))
			endIndex = &Int{}
		}
	}

	if startIndex != nil && startIndex.hasValue {
		if endIndex != nil && endIndex.hasValue && endIndex.value < startIndex.value {
			state.addError(MakeSymbolicEvalError(n.EndIndex, state, END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX))
		}
	}

	if seq, ok := slice.(Sequence); ok {
		if startIndex != nil && startIndex.hasValue && seq.HasKnownLen() && (startIndex.value < 0 || startIndex.value >= int64(seq.KnownLen())) {
			state.addError(MakeSymbolicEvalError(n.StartIndex, state, START_INDEX_IS_OUT_OF_BOUNDS))
		}
		return seq.slice(startIndex, endIndex), nil
	} else {
		state.addError(MakeSymbolicEvalError(n, state, fmtSequenceExpectedButIs(slice)))
		return ANY, nil
	}

}

func evalPatternNamespaceDefinition(n *ast.PatternNamespaceDefinition, state *State, options evalOptions) (Value, error) {
	right, err := symbolicEval(n.Right, state)
	if err != nil {
		return nil, err
	}

	namespace := &PatternNamespace{}
	pos := state.getCurrentChunkNodePositionOrZero(n.Left)
	var namespaceName string

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
					state.addError(MakeSymbolicEvalError(n.Right, state, err.Error()))
				}
			}
			namespace.entries[k] = v.(Pattern)
		}
		name, ok := n.NamespaceName()
		if ok {
			namespaceName = name
		}
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
					state.addError(MakeSymbolicEvalError(n.Right, state, err.Error()))
				}
			}
			namespace.entries[k] = v.(Pattern)
		}
		name, ok := n.NamespaceName()
		if ok {
			namespaceName = name
		}
	default:
		state.addError(MakeSymbolicEvalError(n, state, fmtPatternNamespaceShouldBeInitWithNot(right)))
		name, ok := n.NamespaceName()
		if ok {
			namespaceName = name
		}
	}

	if namespaceName != "" && state.ctx.ResolvePatternNamespace(namespaceName) == nil {
		state.ctx.AddPatternNamespace(namespaceName, namespace, state.inPreinit, pos)
		state.SetMostSpecificNodeValue(n.Left, namespace)
		state.symbolicData.SetContextData(n, state.ctx.currentData())
	}

	return nil, nil
}

// func evalTestsuiteExpression(n *ast.TestSuiteExpression, state *State, options evalOptions) (Value, error) {
// 	var testedProgram *TestedProgram

// 	if n.Meta != nil {
// 		var err error
// 		_, testedProgram, err = checkTestItemMeta(n.Meta, state, false)
// 		if err != nil {
// 			return nil, err
// 		}
// 	} else if state.testedProgram != nil {
// 		//inherit tested program
// 		testedProgram = state.testedProgram
// 	}

// 	v, err := symbolicEval(n.Module, state)
// 	if err != nil {
// 		return nil, err
// 	}

// 	embeddedModule := v.(*AstNode).Node.(*ast.Chunk)

// 	//TODO: read the manifest to known the permissions
// 	modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
// 	state.ctx.CopyNamedPatternsIn(modCtx)
// 	state.ctx.CopyPatternNamespacesIn(modCtx)

// 	modState := newSymbolicState(modCtx, &parse.ParsedChunkSource{
// 		Node: embeddedModule,
// 		ParsedChunkSourceBase: sourcecode.ParsedChunkSourceBase{
// 			Source: state.currentChunk().Source,
// 		},
// 	})
// 	modState.Module = state.Module
// 	modState.symbolicData = state.symbolicData
// 	//TODO: modState.testedProgram = testedProgram
// 	state.forEachGlobal(func(name string, info varSymbolicInfo) {
// 		modState.setGlobal(name, info.value, GlobalConst)
// 	})

// 	//evaluate
// 	_, err = symbolicEval(embeddedModule, modState)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, err := range modState.errors() {
// 		state.addError(err)
// 	}

// 	for _, warning := range modState.warnings() {
// 		state.addWarning(warning)
// 	}

// 	return &TestSuite{}, nil
// }

// func evalTestcaseExpression(n *ast.TestCaseExpression, state *State, options evalOptions) (Value, error) {
// 	var currentTest *CurrentTest = ANY_CURRENT_TEST
// 	var testedProgram *TestedProgram

// 	if n.Meta != nil {
// 		test, program, err := checkTestItemMeta(n.Meta, state, true)
// 		if err != nil {
// 			return nil, err
// 		}
// 		currentTest = test
// 		testedProgram = program
// 	} else if state.testedProgram != nil {
// 		//inherit tested program
// 		testedProgram = state.testedProgram
// 		currentTest = &CurrentTest{testedProgram: testedProgram}
// 	}

// 	v, err := symbolicEval(n.Module, state)
// 	if err != nil {
// 		return nil, err
// 	}

// 	embeddedModule := v.(*AstNode).Node.(*ast.Chunk)

// 	//TODO: read the manifest to known the permissions
// 	modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
// 	state.ctx.CopyNamedPatternsIn(modCtx)
// 	state.ctx.CopyPatternNamespacesIn(modCtx)

// 	modState := newSymbolicState(modCtx, &parse.ParsedChunkSource{
// 		Node: embeddedModule,
// 		ParsedChunkSourceBase: sourcecode.ParsedChunkSourceBase{
// 			Source: state.currentChunk().Source,
// 		},
// 	})
// 	modState.Module = state.Module
// 	modState.symbolicData = state.symbolicData
// 	modState.testedProgram = testedProgram
// 	state.forEachGlobal(func(name string, info varSymbolicInfo) {
// 		modState.setGlobal(name, info.value, GlobalConst)
// 	})

// 	//add the __test global
// 	modState.setGlobal(globalnames.CURRENT_TEST, currentTest, GlobalConst)

// 	//evaluate
// 	_, err = symbolicEval(embeddedModule, modState)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, err := range modState.errors() {
// 		state.addError(err)
// 	}

// 	for _, warning := range modState.warnings() {
// 		state.addWarning(warning)
// 	}

// 	return &TestCase{}, nil
// }

func evalStringTemplateLiteral(n *ast.StringTemplateLiteral, state *State, options evalOptions) (Value, error) {
	_, isPatternAnIdent := n.Pattern.(*ast.PatternIdentifierLiteral)

	if isPatternAnIdent && n.HasInterpolations() {
		state.addError(MakeSymbolicEvalError(n, state, STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX))
		return &CheckedString{}, nil
	}

	var namespaceName string
	var namespace *PatternNamespace

	if n.Pattern != nil {
		if !isPatternAnIdent {
			namespaceMembExpr := n.Pattern.(*ast.PatternNamespaceMemberExpression)
			namespaceName = namespaceMembExpr.Namespace.Name
			namespace = state.ctx.ResolvePatternNamespace(namespaceName)

			if namespace == nil {
				state.addError(MakeSymbolicEvalError(n, state, fmtCannotInterpolatePatternNamespaceDoesNotExist(namespaceName)))
				return &CheckedString{}, nil
			}

			memberName := namespaceMembExpr.MemberName.Name
			_, ok := namespace.entries[memberName]
			if !ok {
				state.addError(MakeSymbolicEvalError(n, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(memberName, namespaceName)))
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
		case *ast.StringTemplateSlice:
		case *ast.StringTemplateInterpolation:
			if s.Type != "" {
				memberName := s.Type
				_, ok := namespace.entries[memberName]
				if !ok {
					state.addError(MakeSymbolicEvalError(slice, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(memberName, namespaceName)))
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
					state.addError(MakeSymbolicEvalError(slice, state, fmtUntypedInterpolationIsNotStringlikeOrIntBut(e)))
				} else {
					state.addError(MakeSymbolicEvalError(slice, state, fmtInterpolationIsNotStringlikeOrIntBut(e)))
				}
			}
		}
	}

	if n.Pattern == nil {
		return ANY_STRING, nil
	}

	return &CheckedString{}, nil
}

func evalMarkupExpression(n *ast.MarkupExpression, state *State, options evalOptions) (Value, error) {

	var namespaceErrorNode ast.Node = n

	var namespace Value

	if n.Namespace != nil {
		namespaceErrorNode = n.Namespace
		var err error

		namespace, err = symbolicEval(n.Namespace, state)
		if err != nil {
			return nil, err
		}
	} else {
		varInfo, ok := state.getGlobal(globalnames.HTML_NS)
		if !ok {
			state.addError(MakeSymbolicEvalError(n, state, HTML_NS_IS_NOT_DEFINED))
			return ANY, nil
		}
		namespace = varInfo.value
	}

	ns, ok := namespace.(*Namespace)
	if !ok {
		_, err := symbolicEval(n.Element, state)
		if err != nil {
			return nil, err
		}

		state.addError(MakeSymbolicEvalError(namespaceErrorNode, state, NAMESPACE_APPLIED_TO_MARKUP_ELEMENT_SHOUD_BE_A_RECORD))
		return ANY, nil
	} else {
		factory, ok := ns.entries[FROM_MARKUP_FACTORY_NAME]
		if !ok {
			state.addError(MakeSymbolicEvalError(namespaceErrorNode, state, MISSING_FACTORY_IN_NAMESPACE_APPLIED_TO_MARKUP_ELEMENT))
			return ANY, nil
		}
		goFn, ok := factory.(*GoFunction)
		if !ok {
			state.addError(MakeSymbolicEvalError(namespaceErrorNode, state, FROM_MARKUP_FACTORY_IS_NOT_A_GO_FUNCTION))
			return ANY, nil
		}

		if goFn.IsShared() {
			state.addError(MakeSymbolicEvalError(namespaceErrorNode, state, FROM_MARKUP_FACTORY_SHOULD_NOT_BE_A_SHARED_FUNCTION))
			return ANY, nil
		}

		utils.PanicIfErr(goFn.LoadSignatureData())

		if len(goFn.NonVariadicParametersExceptCtx()) == 0 {
			state.addError(MakeSymbolicEvalError(namespaceErrorNode, state, FROM_MARKUP_FACTORY_SHOULD_HAVE_AT_LEAST_ONE_NON_VARIADIC_PARAM))
			return ANY, nil
		}

		if goFn.fn != nil {
			checkMarkupInterpolation := state.checkMarkupInterpolation
			defer func() {
				state.checkMarkupInterpolation = checkMarkupInterpolation
			}()
			state.checkMarkupInterpolation = markupInterpolationCheckingFunctions[reflect.ValueOf(goFn.fn).Pointer()]
		}

		elem, err := symbolicEval(n.Element, state)
		if err != nil {
			return nil, err
		}

		result, _, _, err := goFn.Call(goFunctionCallInput{
			symbolicArgs:      []Value{elem},
			nonSpreadArgCount: 1,
			hasSpreadArg:      false,
			state:             state,
			isExt:             false,
			must:              false,
			callLikeNode:      n,
		})

		state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
			var location ast.Node = n
			if optionalLocation != nil {
				location = optionalLocation
			}

			state.addError(MakeSymbolicEvalError(location, state, msg))
		})
		state.consumeSymbolicGoFunctionWarnings(func(msg string) {
			state.addWarning(makeSymbolicEvalWarning(n, state, msg))
		})

		return result, err
	}
}

func evalMarkupElement(n *ast.MarkupElement, state *State, options evalOptions) (Value, error) {
	var children []Value
	name := n.Opening.Name.(*ast.IdentifierLiteral).Name
	var attrs map[string]Value
	if len(n.Opening.Attributes) > 0 {
		attrs = make(map[string]Value, len(n.Opening.Attributes))

		for _, regularAttr := range n.Opening.Attributes {
			name := regularAttr.Name.(*ast.IdentifierLiteral).Name
			if regularAttr.Value == nil { //no value
				//See ../markup.go and the evaluation of *ast.MarkupElement in ../tree_walk_eval.go.
				attrs[name] = EMPTY_STRING
				continue
			}
			val, err := symbolicEval(regularAttr.Value, state)
			if err != nil {
				return nil, err
			}
			attrs[name] = val
		}
	}

	for _, childNode := range n.Children {
		child, err := symbolicEval(childNode, state)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	markupElem := NewNonInterpretedMarkupElement(name, attrs, children)
	markupElem.sourceNode = &MarkupSourceNode{
		Node:  n,
		Chunk: state.currentChunk(),
	}

	state.SetMostSpecificNodeValue(n.Opening.Name, markupElem)
	if n.Closing != nil {
		state.SetMostSpecificNodeValue(n.Closing.Name, markupElem)
	}

	return markupElem, nil
}

func evalMarkupInterpolation(n *ast.MarkupInterpolation, state *State, options evalOptions) (Value, error) {
	val, err := symbolicEval(n.Expr, state)
	if err != nil {
		return nil, err
	}

	if state.checkMarkupInterpolation != nil {
		msg := state.checkMarkupInterpolation(n.Expr, val)
		if msg != "" {
			state.addError(MakeSymbolicEvalError(n.Expr, state, msg))
		}
	}

	return val, err
}

func evalMarkupPatternExpression(n *ast.MarkupPatternExpression, state *State, options evalOptions) (Value, error) {
	return ANY_MARKUP_PATTERN, evalMarkupPatternElement(n.Element, state)
}

func evalMarkupPatternElement(node *ast.MarkupPatternElement, state *State) error {

	attributes := map[string]StringPattern{}

	//Evaluate attributes.

	for _, attr := range node.Opening.Attributes {
		patternAttribute := attr.(*ast.MarkupPatternAttribute)
		attrName := patternAttribute.GetName()

		var stringPattern StringPattern

		if patternAttribute.Type != nil {
			val, err := symbolicEval(patternAttribute.Type, state)
			if err != nil {
				return err
			}

			var values []Value
			if mv, ok := val.(IMultivalue); ok {
				values = mv.OriginalMultivalue().getValues()
			} else {
				values = []Value{val}
			}

		check_all_attr_values:
			for _, v := range values {
				switch v := v.(type) {
				case StringPattern:
					stringPattern = v
				case Pattern:
					strPattern, ok := v.StringPattern()
					if !ok {
						msg := fmtPatternForAttributeDoesNotHaveCorrespStrPattern(attrName)
						state.addError(MakeSymbolicEvalError(patternAttribute.Type, state, msg))
						break check_all_attr_values
					}
					stringPattern = strPattern
				case StringLike:
					stringPattern = ANY_EXACT_STR_PATTERN
				case *Bool:
					stringPattern = ANY_EXACT_STR_PATTERN
				case *Int:
					stringPattern = ANY_EXACT_STR_PATTERN
				case ResourceName:
					stringPattern = ANY_EXACT_STR_PATTERN
				case *Rune:
					stringPattern = ANY_EXACT_STR_PATTERN
				default:
					//Note: floats are not supported because they do not have a unique representation.
					msg := fmtUnexpectedValForAttrX(attrName)
					state.addError(MakeSymbolicEvalError(patternAttribute.Type, state, msg))
					break check_all_attr_values
				}

			}
		} else {
			stringPattern = ANY_REGEX_PATTERN
		}

		attributes[attrName] = stringPattern
	}

	//Evaluate children nodes.

	if node.RawElementContent == "" {
		for _, child := range node.Children {
			switch child := child.(type) {
			case *ast.MarkupText: //ok
			case *ast.MarkupPatternWildcard: //ok
			case *ast.MarkupPatternElement:
				err := evalMarkupPatternElement(child, state)
				if err != nil {
					return err
				}
			case *ast.MarkupPatternInterpolation:
				val, err := symbolicEval(child.Expr, state)
				if err != nil {
					return err
				}

				var values []Value
				if mv, ok := val.(IMultivalue); ok {
					values = mv.OriginalMultivalue().getValues()
				} else {
					values = []Value{val}
				}

			check_all_values:
				for _, v := range values {
					switch v.(type) {
					case *MarkupPattern, StringLike, *Bool, *Int, ResourceName, *Rune: //ok
					default:
						state.addError(MakeSymbolicEvalError(child.Expr, state, UNEXPECTED_VAL_FOR_MARKUP_PATTERN_INTERP))
						break check_all_values
					}
				}
			}
		}
	}

	return nil
}

func evalDoubleColonExpression(n *ast.DoubleColonExpression, state *State, options evalOptions) (Value, error) {
	left, err := symbolicEval(n.Left, state)
	if err != nil {
		return nil, err
	}

	extensions := state.ctx.GetExtensions(left)
	if !state.inNonInitialInoxCall() {
		state.symbolicData.SetAvailableTypeExtensions(n, extensions)
	}

	obj, isLeftObject := left.(*Object)
	//url, isLeftURL := left.(*URL) //ignore URL multivalues because the property resolution would be too ambiguous.

	switch {
	case n.Element != nil && isLeftObject && HasRequiredOrOptionalProperty(obj, n.Element.Name):
		elementName := n.Element.Name

		//get actual value of the property.

		memb := symbolicMemb(obj, elementName, unspecifiedMemberAccess, n, state)
		state.SetMostSpecificNodeValue(n.Element, memb)

		if IsAnyOrAnySerializable(memb) || utils.Ret0(IsSharable(memb)) {
			state.addError(MakeSymbolicEvalError(n, state, RHS_OF_DOUBLE_COLON_EXPRS_WITH_OBJ_LHS_SHOULD_BE_THE_NAME_OF_A_MUTABLE_NON_SHARABLE_VALUE_PROPERTY))
		} else if len(options.doubleColonExprAncestorChain) == 0 {
			state.addError(MakeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR))
		} else {
			ancestors := options.doubleColonExprAncestorChain
			rootAncestor := ancestors[0]
			misplaced := true
			switch rootAncestor.(type) {
			case *ast.Assignment:
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
			case *ast.CallExpression:
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
				state.addError(MakeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR))
			}
		}
		return memb, nil
	// case isLeftURL && url.hasValue:
	// 	//resolve

	// 	valAtURL, err := GetValueAtURL(url, state)
	// 	if err != nil {
	// 		state.addError(MakeSymbolicEvalError(n.Left, state, err.Error()))
	// 		return ANY_SERIALIZABLE, nil
	// 	}

	// 	iprops, ok := valAtURL.(IProps)
	// 	if !ok {
	// 		state.addError(MakeSymbolicEvalError(n.Element, state, fmtValueAtURLHasNoProperties(valAtURL)))
	// 		state.SetMostSpecificNodeValue(n.Element, ANY_SERIALIZABLE)
	// 		return ANY_SERIALIZABLE, nil
	// 	}
	// 	state.symbolicData.SetURLReferencedEntity(n, iprops)

	// 	if n.Element == nil {
	// 		//parsing error.
	// 		return ANY_SERIALIZABLE, nil
	// 	}

	// 	elementName := n.Element.Name

	// 	if !slices.Contains(iprops.PropertyNames(), elementName) {
	// 		state.addError(MakeSymbolicEvalError(n.Element, state, fmtValueAtURLDoesNotHavePropX(valAtURL, elementName)))
	// 		state.SetMostSpecificNodeValue(n.Element, ANY_SERIALIZABLE)
	// 		return ANY_SERIALIZABLE, nil
	// 	}

	// 	val := iprops.Prop(elementName)
	// 	state.SetMostSpecificNodeValue(n.Element, val)

	// 	return val, nil
	default:
		if n.Element == nil {
			//parsing error.
			return ANY, nil
		}
		elementName := n.Element.Name

		//use extenions
		var extension *TypeExtension
		var expr propertyExpression

	loop_over_extensions:
		for _, ext := range extensions {
			for _, propExpr := range ext.PropertyExpressions {
				if propExpr.Name == elementName {
					expr = propExpr
					extension = ext
					break loop_over_extensions
				}
			}
		}

		//if found
		if expr != (propertyExpression{}) {
			var result Value

			if expr.Method != nil {
				if len(options.doubleColonExprAncestorChain) == 0 {
					state.addError(MakeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR_EXT_METHOD_CAN_ONLY_BE_CALLED))
					return ANY, nil
				}

				//check not misplaced
				misplaced := true
				ancestors := options.doubleColonExprAncestorChain
				rootAncestor := ancestors[0]
				switch rootAncestor.(type) {
				case *ast.CallExpression:
					misplaced = false
				default:
				}
				if misplaced {
					state.addError(MakeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR_EXT_METHOD_CAN_ONLY_BE_CALLED))
				}

				result = expr.Method
			} else { //evaluate the property's expression
				prevSelf, restoreSelf := state.getSelf()
				if restoreSelf {
					state.unsetSelf()
				}
				state.setSelf(left)

				defer func() {
					state.unsetSelf()
					if restoreSelf {
						state.setSelf(prevSelf)
					}
				}()

				result, err = symbolicEval(expr.Expression, state)
				if err != nil {
					return nil, err
				}
			}
			state.SetMostSpecificNodeValue(n.Element, result)
			if !state.inNonInitialInoxCall() {
				state.symbolicData.SetUsedTypeExtension(n, extension)
			}
			return result, nil
		}
		//not found (error)

		var suggestion string
		var names []string
		for _, ext := range extensions {
			for _, propExpr := range ext.PropertyExpressions {
				names = append(names, propExpr.Name)
			}
		}

		closest, _, ok := utils.FindClosestString(state.ctx.startingConcreteContext, names, elementName, 2)
		if ok {
			suggestion = closest
		}

		state.addError(MakeSymbolicEvalError(n, state, fmtExtensionsDoNotProvideTheXProp(elementName, suggestion)))
		return ANY_SERIALIZABLE, nil
	}
}

func evalExtendStatement(n *ast.ExtendStatement, state *State, options evalOptions) (Value, error) {
	if n.Err != nil && n.Err.Kind == parse.UnterminatedExtendStmt {
		return nil, nil
	}

	pattern, err := evalPatternNode(n.ExtendedPattern, state)
	if err != nil {
		return nil, err
	}

	if !IsConcretizable(pattern) {
		state.addError(MakeSymbolicEvalError(n.ExtendedPattern, state, EXTENDED_PATTERN_MUST_BE_CONCRETIZABLE_AT_CHECK_TIME))
		return nil, nil
	}

	extendedValue := pattern.SymbolicValue()

	if _, ok := extendedValue.(Serializable); !ok {
		state.addError(MakeSymbolicEvalError(n.ExtendedPattern, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
		return nil, nil
	}

	objLit, ok := n.Extension.(*ast.ObjectLiteral)
	if !ok {
		// there is already a parsing error
		return nil, nil
	}

	extendedValueIprops, _ := extendedValue.(IProps)

	extension := &TypeExtension{
		Id:              state.currentChunk().GetFormattedNodeLocation(n),
		Statement:       n,
		ExtendedPattern: pattern,
	}

	stateFork := state.fork() //used for evaluating computed properties

	for _, prop := range objLit.Properties {
		if prop.HasNoKey() {
			state.addError(MakeSymbolicEvalError(prop, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS))
			continue
		}

		key := prop.Name()

		expr, ok := parse.ParseExpression(key)
		_, isIdent := expr.(*ast.IdentifierLiteral)
		if !ok || !isIdent {
			state.addError(MakeSymbolicEvalError(prop.Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS))
			continue
		}

		if extendedValueIprops != nil {
			if HasRequiredOrOptionalProperty(extendedValueIprops, key) {
				state.addError(MakeSymbolicEvalError(prop.Key, state, fmtExtendedValueAlreadyHasAnXProperty(key)))
				continue
			}
		}

		switch v := prop.Value.(type) {
		case *ast.FunctionExpression:
			prevNextSelf, restoreNextSelf := state.getNextSelf()
			if restoreNextSelf {
				state.unsetNextSelf()
			}
			state.setNextSelf(extendedValue)

			inoxFn, err := symbolicEval(v, state)
			if err != nil {
				return nil, err
			}

			extension.PropertyExpressions = append(extension.PropertyExpressions, propertyExpression{
				Name:   key,
				Method: inoxFn.(*InoxFunction),
			})

			state.unsetNextSelf()
			if restoreNextSelf {
				state.setNextSelf(prevNextSelf)
			}
		default: //computed property
			prevNextSelf, restoreSelf := stateFork.getSelf()
			if restoreSelf {
				stateFork.unsetSelf()
			}
			stateFork.setSelf(extendedValue)

			_, err := symbolicEval(v, stateFork)
			if err != nil {
				return nil, err
			}

			stateFork.unsetSelf()
			if restoreSelf {
				stateFork.setSelf(prevNextSelf)
			}

			extension.PropertyExpressions = append(extension.PropertyExpressions, propertyExpression{
				Name:       key,
				Expression: v,
			})
		}
	}

	// entries := map[string]Serializable{}
	// indexKey := 0

	// var (
	// 	keyArray        [32]string
	// 	keys            = keyArray[:0]
	// 	keyToProp       memds.Map32[string, *ast.ObjectProperty]
	// 	dependencyGraph memds.Graph32[string]

	// 	selfDependentArray [32]string
	// 	selfDependent      = selfDependentArray[:0]

	// 	hasMethods      bool
	// 	hasLifetimeJobs bool
	// )

	// //first iteration of the properties: we get all keys
	// for _, p := range n.Properties {
	// 	var key string

	// 	//add the key
	// 	switch n := p.Key.(type) {
	// 	case *ast.QuotedStringLiteral:
	// 		key = n.Value
	// 		_, err := strconv.ParseUint(key, 10, 32)
	// 		if err == nil {
	// 			//see Check function
	// 			indexKey++
	// 		}
	// 	case *ast.IdentifierLiteral:
	// 		key = n.Name
	// 	case nil:
	// 		key = strconv.Itoa(indexKey)
	// 		indexKey++
	// 	default:
	// 		return nil, fmt.Errorf("invalid key type %T", n)
	// 	}

	// 	dependencyGraph.AddNode(key)
	// 	keys = append(keys, key)
	// 	keyToProp.Set(key, p)
	// }

	// for _, el := range n.SpreadElements {
	// 	_, isExtractionExpr := el.Expr.(*ast.ExtractionExpression)

	// 	evaluatedElement, err := symbolicEval(el.Expr, state)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	if !isExtractionExpr {
	// 		continue
	// 	}

	// 	object := evaluatedElement.(*Object)

	// 	for _, key := range el.Expr.(*ast.ExtractionExpression).Keys.Keys {
	// 		name := key.(*ast.IdentifierLiteral).Name
	// 		v, _, ok := object.GetProperty(name)
	// 		if !ok {
	// 			panic(fmt.Errorf("missing property %s", name))
	// 		}

	// 		serializable, ok := v.(Serializable)
	// 		if !ok {
	// 			state.addError(makeSymbolicEvalError(el, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
	// 			serializable = ANY_SERIALIZABLE
	// 		} else if _, ok := asWatchable(v).(Watchable); !ok && v.IsMutable() {
	// 			state.addError(makeSymbolicEvalError(el, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
	// 		}

	// 		entries[name] = serializable
	// 	}
	// }

	// if indexKey != 0 {
	// 	// TODO: implicit prop count
	// }

	// //second iteration of the properties: we build a graph of dependencies
	// for i, p := range n.Properties {
	// 	dependentKey := keys[i]
	// 	dependentKeyId, _ := dependencyGraph.IdOfNode(dependentKey)

	// 	if _, ok := p.Value.(*ast.FunctionExpression); !ok {
	// 		continue
	// 	}

	// 	hasMethods = true

	// 	// find the method's dependencies
	// 	ast.Walk(p.Value, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {

	// 		if ast.IsScopeContainerNode(node) && node != p.Value {
	// 			return ast.Prune, nil
	// 		}

	// 		switch node.(type) {
	// 		case *ast.SelfExpression:
	// 			dependencyName := ""

	// 			switch p := parent.(type) {
	// 			case *ast.MemberExpression:
	// 				dependencyName = p.PropertyName.Name
	// 			case *ast.DynamicMemberExpression:
	// 				dependencyName = p.PropertyName.Name
	// 			}

	// 			if dependencyName == "" {
	// 				break
	// 			}

	// 			depId, ok := dependencyGraph.IdOfNode(dependencyName)
	// 			if !ok {
	// 				//?
	// 				return ast.ContinueTraversal, nil
	// 			}

	// 			if dependentKeyId == depId {
	// 				selfDependent = append(selfDependent, dependentKey)
	// 			} else if !dependencyGraph.HasEdgeFromTo(dependentKeyId, depId) {
	// 				// dependentKey ->- depKey
	// 				dependencyGraph.AddEdge(dependentKeyId, depId)
	// 			}
	// 		}
	// 		return ast.ContinueTraversal, nil
	// 	}, nil)
	// }

	// // we sort the keys based on the dependency graph

	// var dependencyChainCountsCache = make(map[memds.NodeId]int, len(keys))
	// var getDependencyChainDepth func(memds.NodeId, []memds.NodeId) int
	// var cycles [][]string

	// getDependencyChainDepth = func(nodeId memds.NodeId, chain []memds.NodeId) int {
	// 	for _, id := range chain {
	// 		if nodeId == id && len(chain) >= 1 {
	// 			cycle := make([]string, 0, len(chain))

	// 			for _, id := range chain {
	// 				cycle = append(cycle, "."+keys[id])
	// 			}
	// 			cycles = append(cycles, cycle)
	// 			return 0
	// 		}
	// 	}

	// 	chain = append(chain, nodeId)

	// 	if v, ok := dependencyChainCountsCache[nodeId]; ok {
	// 		return v
	// 	}

	// 	depth_ := 0
	// 	directDependencies := dependencyGraph.IteratorDirectlyReachableNodes(nodeId)

	// 	for directDependencies.Next() {
	// 		dep := directDependencies.Node()
	// 		count := 1 + getDependencyChainDepth(dep.Id(), chain)
	// 		if count > depth_ {
	// 			depth_ = count
	// 		}
	// 	}

	// 	dependencyChainCountsCache[nodeId] = depth_
	// 	return depth_
	// }

	// expectedObj, ok := findInMultivalue[*Object](options.expectedValue)
	// if !ok {
	// 	expectedObj = &Object{}
	// }

	// sort.Slice(keys, func(i, j int) bool {
	// 	keyA := keys[i]
	// 	keyB := keys[j]

	// 	// we move all implicit lifetime jobs at the end
	// 	p1 := keyToProp.MustGet(keyA)
	// 	if _, ok := p1.Value.(*ast.LifetimejobExpression); ok {
	// 		hasLifetimeJobs = true
	// 		if p1.HasImplicitKey() {
	// 			return false
	// 		}
	// 	}
	// 	p2 := keyToProp.MustGet(keyB)
	// 	if _, ok := p2.Value.(*ast.LifetimejobExpression); ok && p2.HasImplicitKey() {
	// 		return true
	// 	}

	// 	idA := dependencyGraph.MustGetIdOfNode(keyA)
	// 	idB := dependencyGraph.MustGetIdOfNode(keyB)

	// 	return getDependencyChainDepth(idA, nil) < getDependencyChainDepth(idB, nil)
	// })

	// isExact := options.neverModifiedArgument && len(n.SpreadElements) == 0 && !hasMethods && !hasLifetimeJobs

	// obj := NewObject(isExact, entries, nil, nil)
	// if expectedObj.readonly {
	// 	obj.readonly = true
	// }

	// if len(cycles) > 0 {
	// 	state.addError(makeSymbolicEvalError(n, state, fmtMethodCyclesDetected(cycles)))
	// 	return ANY_OBJ, nil
	// }

	// prevNextSelf, restoreNextSelf := state.getNextSelf()
	// if restoreNextSelf {
	// 	state.unsetNextSelf()
	// }
	// state.setNextSelf(obj)

	// //add allowed missing properties
	// {
	// 	var properties []string
	// 	expectedObj.ForEachEntry(func(propName string, propValue Value) error {
	// 		if slices.Contains(keys, propName) {
	// 			return nil
	// 		}
	// 		properties = append(properties, propName)
	// 		return nil
	// 	})
	// 	state.symbolicData.SetAllowedNonPresentProperties(n, properties)
	// }

	// //evaluate properties
	// for _, key := range keys {
	// 	p := keyToProp.MustGet(key)

	// 	var static Pattern

	// 	expectedPropVal := expectedObj.entries[key]
	// 	deeperMismatch := false

	// 	if p.Key != nil && expectedObj.exact && expectedPropVal == nil {
	// 		closest, _, ok := utils.FindClosestString(state.ctx.startingConcreteContext, maps.Keys(expectedObj.entries), p.Name(), 2)
	// 		options.setActualValueMismatchIfNotNil()

	// 		msg := ""
	// 		if ok {
	// 			msg = fmtUnexpectedPropertyDidYouMeanElse(key, closest)
	// 		} else {
	// 			msg = fmtUnexpectedProperty(key)
	// 		}

	// 		state.addError(makeSymbolicEvalError(p.Key, state, msg))
	// 	}

	// 	var (
	// 		propVal      Value
	// 		err          error
	// 		serializable Serializable
	// 	)

	// 	if p.Value == nil {
	// 		propVal = ANY_SERIALIZABLE
	// 		serializable = ANY_SERIALIZABLE
	// 	} else {
	// 		propVal, err = _symbolicEval(p.Value, state, evalOptions{expectedValue: expectedPropVal, actualValueMismatch: &deeperMismatch})
	// 		if err != nil {
	// 			return nil, err
	// 		}

	// 		if p.Type != nil {
	// 			_propType, err := symbolicEval(p.Type, state)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			static = _propType.(Pattern)
	// 			if !static.TestValue(propVal, RecTestCallState{}) {
	// 				expected := static.SymbolicValue()
	// 				state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfType(propVal, expected)))
	// 				propVal = expected
	// 			}
	// 		} else if deeperMismatch {
	// 			options.setActualValueMismatchIfNotNil()
	// 		} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(propVal, RecTestCallState{}) {
	// 			options.setActualValueMismatchIfNotNil()
	// 			state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfType(propVal, expectedPropVal)))
	// 		}

	// 		serializable, ok = propVal.(Serializable)
	// 		if !ok {
	// 			state.addError(makeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
	// 			serializable = ANY_SERIALIZABLE
	// 		} else if _, ok := asWatchable(propVal).(Watchable); !ok && propVal.IsMutable() {
	// 			state.addError(makeSymbolicEvalError(p, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
	// 		}

	// 		//additional checks if expected object is readonly
	// 		if expectedObj.readonly {
	// 			if _, ok := propVal.(*LifetimeJob); ok {
	// 				state.addError(makeSymbolicEvalError(p, state, LIFETIME_JOBS_NOT_ALLOWED_IN_READONLY_OBJECTS))
	// 			} else if !IsReadonlyOrImmutable(propVal) {
	// 				state.addError(makeSymbolicEvalError(p.Key, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE))
	// 			}
	// 		}
	// 	}

	// 	obj.initNewProp(key, serializable, static)
	// 	state.SetMostSpecificNodeValue(p.Key, propVal)
	// }
	// state.unsetNextSelf()
	// if restoreNextSelf {
	// 	state.setNextSelf(prevNextSelf)
	// }

	for _, metaProp := range objLit.MetaProperties {
		state.addError(MakeSymbolicEvalError(metaProp, state, META_PROPERTIES_NOT_ALLOWED_IN_EXTENSION_OBJECT))
	}

	state.ctx.AddTypeExtension(extension)
	state.symbolicData.SetContextData(n, state.ctx.currentData())

	return nil, nil
}

type memberAccessKind int

const (
	unspecifiedMemberAccess memberAccessKind = iota
	optionalMemberAccess
	destructurationMemberAccess
	optionalDestructurationMemberAccess
)

func symbolicMemb(value Value, name string, accessKind memberAccessKind, node ast.Node, state *State) (result Value) {
	//note: the property of a %serializable is not necessarily serializable (example: Go methods)

	isOptionalAccess := accessKind == optionalMemberAccess || accessKind == optionalDestructurationMemberAccess

	iprops, ok := AsIprops(value).(IProps)
	if !ok {
		state.addError(MakeSymbolicEvalError(node, state, fmtValueHasNoProperties(value)))
		return ANY
	}

	defer func() {
		e := recover()

		if e != nil {
			//TODO: add log

			//if err, ok := e.(error); ok && strings.Contains(err.Error(), "nil pointer") {
			//}

			concreteCtx := state.ctx.startingConcreteContext
			closest, distance, found := utils.FindClosestString(concreteCtx, iprops.PropertyNames(), name, MAX_STRING_SUGGESTION_DIFF)
			if !found || (len(closest) >= MAX_STRING_SUGGESTION_DIFF && distance >= MAX_STRING_SUGGESTION_DIFF-1) {
				closest = ""
			}

			if !isOptionalAccess {
				state.addError(MakeSymbolicEvalError(node, state, fmtPropOfDoesNotExist(name, value, closest)))
			}
			result = ANY
		} else {
			if optIprops, ok := iprops.(OptionalIProps); ok {
				if !isOptionalAccess && slices.Contains(optIprops.OptionalPropertyNames(), name) {
					msg := ""
					if accessKind == destructurationMemberAccess {
						msg = fmtPropertyIsOptionalUseAnOptionalDestructuration(name)
					} else {
						msg = fmtPropertyIsOptionalUseOptionalMembExpr(name)
					}
					state.addError(MakeSymbolicEvalError(node, state, msg))
				}
			}
		}
	}()

	propValue := iprops.Prop(name)

	if propValue == nil {
		state.addError(MakeSymbolicEvalError(node, state, "symbolic IProp should panic when a non-existing property is accessed"))
		return ANY
	}
	if isOptionalAccess {
		propValue = joinValues([]Value{propValue, Nil})
	}

	return propValue
}

func handleConstraints(obj *Object, block *ast.InitializationBlock, state *State) error {
	//we first there are only authorized statements & expressions in the initialization block

	err := ast.Walk(block, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {

		if node == block {
			return ast.ContinueTraversal, nil
		}

		switch node.(type) {
		case *ast.BinaryExpression:
		case *ast.SelfExpression:
		case *ast.MemberExpression:
		case ast.SimpleValueLiteral:
		default:
			state.addError(MakeSymbolicEvalError(node, state, CONSTRAINTS_INIT_BLOCK_EXPLANATION))
		}
		return ast.ContinueTraversal, nil
	}, nil)

	if err != nil {
		return fmt.Errorf("constraints: error when walking the initialization block: %w", err)
	}

	//

	for _, stmt := range block.Statements {
		switch stmt.(type) {
		case *ast.BinaryExpression:

			constraint := &ComplexPropertyConstraint{
				Expr: stmt,
			}

			ast.Walk(stmt, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
				if utils.Implements[*ast.SelfExpression](node) && utils.Implements[*ast.MemberExpression](parent) {
					constraint.Properties = append(constraint.Properties, parent.(*ast.MemberExpression).PropertyName.Name)
				}
				return ast.ContinueTraversal, nil
			}, nil)

			obj.complexPropertyConstraints = append(obj.complexPropertyConstraints, constraint)
		default:
			state.addError(MakeSymbolicEvalError(stmt, state, CONSTRAINTS_INIT_BLOCK_EXPLANATION))
		}
	}

	return nil
}

func MakeSymbolicEvalError(node ast.Node, state *State, msg string, regions ...commonfmt.RegionInfo) EvaluationError {
	locatedMsg := msg
	location := state.getErrorMesssageLocation(node)

	var locatedMessageRegions []commonfmt.RegionInfo

	if state.Module != nil {
		locatedMsg = fmt.Sprintf("check(symbolic): %s: %s", location, msg)
		prefixLen := len(locatedMsg) - len(msg)

		locatedMessageRegions := slices.Clone(locatedMessageRegions)
		for i := range locatedMessageRegions {
			locatedMessageRegions[i].Start += int32(prefixLen)
			locatedMessageRegions[i].End += int32(prefixLen)
		}
	}
	return EvaluationError{
		Message:        msg,
		MessageRegions: regions,

		Location:              location,
		LocatedMessageRegions: locatedMessageRegions,
		LocatedMessage:        locatedMsg,
	}
}

func makeSymbolicEvalErrorFromError(node ast.Node, state *State, err error) EvaluationError {
	if e, ok := err.(EvaluationError); ok {
		return e
	}
	return MakeSymbolicEvalError(node, state, err.Error())
}

func makeSymbolicEvalWarning(node ast.Node, state *State, msg string) EvaluationWarning {
	return makeSymbolicEvalWarningWithSpan(node.Base().Span, state, msg)
}

func makeSymbolicEvalWarningWithSpan(nodeSpan sourcecode.NodeSpan, state *State, msg string) EvaluationWarning {
	locatedMsg := msg
	location := state.getErrorMesssageLocationOfSpan(nodeSpan)
	if state.Module != nil {
		locatedMsg = fmt.Sprintf("check(symbolic): warning: %s: %s", location, msg)
	}
	return EvaluationWarning{msg, locatedMsg, location}
}

func converTypeToSymbolicValue(t reflect.Type, allowOptionalParam bool) (result Value, optionalParam bool, _ error) {
	err := fmt.Errorf("cannot convert type to symbolic value : %#v", t)

	if t.Implements(OPTIONAL_PARAM_TYPE) {
		if !allowOptionalParam {
			return nil, true, errors.New("optionalParam implementations are not allowed")
		}

		if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
			return nil, true, errors.New("unexpected optionalParam implementation")
		}

		field, ok := t.Elem().FieldByName("Value")
		if !ok {
			return nil, true, errors.New("unexpected optionalParam implementation")
		}
		typ, _, err := converTypeToSymbolicValue(field.Type.Elem(), false)
		if err != nil {
			return nil, true, err
		}
		return typ, true, nil
	}

	if t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct {
		v := reflect.New(t.Elem())
		symbolicVal, ok := v.Interface().(Value)
		if !ok {
			return nil, false, err
		}
		return symbolicVal.WidestOfType(), false, nil
	}

	switch t {
	case SYMBOLIC_VALUE_INTERFACE_TYPE:
		result = ANY
	case SERIALIZABLE_INTERFACE_TYPE:
		result = ANY_SERIALIZABLE
	case SERIALIZABLE_ITERABLE_INTERFACE_TYPE:
		result = ANY_SERIALIZABLE_ITERABLE
	case ITERABLE_INTERFACE_TYPE:
		result = ANY_ITERABLE
	case INDEXABLE_INTERFACE_TYPE:
		result = ANY_INDEXABLE
	case SEQUENCE_INTERFACE_TYPE:
		result = ANY_SEQ_OF_ANY
	case RESOURCE_NAME_INTERFACE_TYPE:
		result = ANY_RES_NAME
	case READABLE_INTERFACE_TYPE:
		result = ANY_READABLE
	case PATTERN_INTERFACE_TYPE:
		result = ANY_PATTERN
	case PROTOCOL_CLIENT_INTERFACE_TYPE:
		result = &AnyProtocolClient{}
	case STREAMABLE_INTERFACE_TYPE:
		result = ANY_STREAM_SOURCE
	case WATCHABLE_INTERFACE_TYPE:
		result = ANY_WATCHABLE
	case WRITABLE_INTERFACE_TYPE:
		result = ANY_WRITABLE
	case STR_PATTERN_ELEMENT_INTERFACE_TYPE:
		result = ANY_STR_PATTERN
	case INTEGRAL_INTERFACE_TYPE:
		result = ANY_INTEGRAL
	case FORMAT_INTERFACE_TYPE:
		result = ANY_FORMAT
	case IN_MEM_SNAPSHOTABLE:
		result = ANY_IN_MEM_SNAPSHOTABLE
	case STRLIKE_INTERFACE_TYPE:
		result = ANY_STR_LIKE
	case VALUEPATH_INTERFACE_TYPE:
		result = ANY_VALUE_PATH
	default:
		return nil, false, err
	}

	return
}

func isAllowedAfterMutationDoubleColonExprAncestor(ancestor, deeper ast.Node) bool {
	switch a := ancestor.(type) {
	case *ast.MemberExpression:
		if deeper == a.Left {
			return true
		}
	case *ast.IdentifierMemberExpression:
		if deeper == a.Left {
			return true
		}
	case *ast.IndexExpression:
		if deeper == a.Indexed {
			return true
		}
	case *ast.SliceExpression:
		if deeper == a.Indexed {
			return true
		}
	}
	return false
}
