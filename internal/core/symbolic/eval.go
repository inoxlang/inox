package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"

	"slices"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
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
	SYMBOLIC_VALUE_INTERFACE_TYPE        = reflect.TypeOf((*Value)(nil)).Elem()
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
	VALUEPATH_INTERFACE_TYPE           = reflect.TypeOf((*ValuePath)(nil)).Elem()

	OPTIONAL_PARAM_TYPE = reflect.TypeOf((*optionalParam)(nil)).Elem()

	ANY_READABLE = &AnyReadable{}
	ANY_READER   = &Reader{}

	SUPPORTED_PARSING_ERRORS = []parse.ParsingErrorKind{
		parse.UnterminatedMemberExpr, parse.UnterminatedDoubleColonExpr,
		parse.UnterminatedExtendStmt,
		parse.UnterminatedStructDefinition,
		parse.MissingBlock, parse.MissingFnBody,
		parse.MissingEqualsSignInDeclaration,
		parse.MissingObjectPropertyValue,
		parse.MissingObjectPatternProperty,
		parse.ExtractionExpressionExpected,
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

type EvalCheckInput struct {
	Node   *parse.Chunk
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

	//nil if no project
	ProjectFilesystem billy.Filesystem

	importPositions     []parse.SourcePositionRange
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
	state.projectFilesystem = input.ProjectFilesystem

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

func SymbolicEval(node parse.Node, state *State) (result Value, finalErr error) {
	return symbolicEval(node, state)
}

func symbolicEval(node parse.Node, state *State) (result Value, finalErr error) {
	return _symbolicEval(node, state, evalOptions{})
}

type evalOptions struct {
	ignoreNodeValue     bool
	expectedValue       Value
	actualValueMismatch *bool
	reEval              bool

	//used for checking that double-colon expressions are not misplaced
	doubleColonExprAncestorChain []parse.Node

	neverModifiedArgument bool
}

func (opts evalOptions) setActualValueMismatchIfNotNil() {
	if opts.actualValueMismatch != nil {
		*opts.actualValueMismatch = true
	}
}

func _symbolicEval(node parse.Node, state *State, options evalOptions) (result Value, finalErr error) {
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

		if utils.Implements[*parse.EmbeddedModule](node) {
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
	case *parse.Chunk:
		return evalChunk(n, state)
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
			state.addError(makeSymbolicEvalError(node, state, err.Error()))
			return ANY, nil
		}
		return extData.ToSymbolicValue(v, false)
	case *parse.YearLiteral:
		return NewYear(n.Value), nil
	case *parse.DateLiteral:
		return NewDate(n.Value), nil
	case *parse.DateTimeLiteral:
		return NewDateTime(n.Value), nil
	case *parse.RateLiteral:
		v, err := extData.GetRate(n.Values, n.Units, n.DivUnit)
		if err != nil {
			state.addError(makeSymbolicEvalError(node, state, err.Error()))
			return ANY, nil
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
			msg := fmtVarIsNotDeclared(n.Name)

			if pattern := state.ctx.ResolveNamedPattern(n.Name); pattern != nil {
				msg += fmt.Sprintf("; did you mean %%%s ? In this location patterns require a leading '%%'.", n.Name)
			}

			state.addError(makeSymbolicEvalError(node, state, msg))
			return ANY, nil
		}
		return info.value, nil
	case *parse.UnambiguousIdentifierLiteral:
		return &Identifier{name: n.Name}, nil
	case *parse.PropertyNameLiteral:
		return &PropertyName{name: n.Name}, nil
	case *parse.LongValuePathLiteral:
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
	case *parse.AbsolutePathLiteral:
		if strings.HasSuffix(n.Value, "/...") || strings.Contains(n.Value, "*") {
			state.addWarning(makeSymbolicEvalWarning(node, state, fmtDidYouForgetLeadingPercent(n.Value)))
		}
		return NewPath(n.Value), nil
	case *parse.RelativePathLiteral:
		if strings.HasSuffix(n.Value, "/...") || strings.Contains(n.Value, "*") {
			state.addWarning(makeSymbolicEvalWarning(node, state, fmtDidYouForgetLeadingPercent(n.Value)))
		}
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
		return ANY_STRING, nil
	case *parse.URLQueryParameterValueSlice:
		return ANY_STRING, nil
	case *parse.FlagLiteral:
		if _, hasVar := state.get(n.Name); hasVar {
			state.addWarning(makeSymbolicEvalWarning(node, state, THIS_VAL_IS_AN_OPT_LIT_DID_YOU_FORGET_A_SPACE))
		}

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
		return NewPathPatternFromNode(n, state.currentChunk().Node), nil
	case *parse.URLLiteral:
		return NewUrl(n.Value), nil
	case *parse.SchemeLiteral:
		return NewScheme(n.ValueString()), nil
	case *parse.HostLiteral:
		return NewHost(n.Value), nil
	case *parse.AtHostLiteral:
		if n.Value == "" {
			return ANY_HOST, nil
		}
		value := state.ctx.ResolveHostAlias(n.Name())
		if value == nil {
			state.addError(makeSymbolicEvalError(n, state, fmtHostAliasIsNotDeclared(n.Name())))
			return ANY_HOST, nil
		}
		return value, nil
	case *parse.HostPatternLiteral:
		return NewHostPattern(n.Value), nil
	case *parse.URLPatternLiteral:
		return NewUrlPattern(n.Value), nil
	case *parse.URLExpression:
		return evalURLExpression(n, state)
	case *parse.NilLiteral:
		return &NilT{}, nil
	case *parse.SelfExpression:
		v, ok := state.getSelf()
		if !ok {
			return nil, errors.New("no self")
		}
		return v, nil
	case *parse.Variable:
		info, ok := state.getLocal(n.Name)
		if !ok {
			msg := fmtLocalVarIsNotDeclared(n.Name)

			if pattern := state.ctx.ResolveNamedPattern(n.Name); pattern != nil {
				msg += fmt.Sprintf("; did you mean %%%s instead of $%s ?", n.Name, n.Name)
			}

			state.addError(makeSymbolicEvalError(node, state, msg))
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
		return evalReturnStatement(n, state)
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
		return evalPatternCallExpression(n, state)
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

		var res Value
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
		return nil, evalLocalVariableDeclarations(n, state)
	case *parse.GlobalVariableDeclarations:
		return nil, evalGlobalVariableDeclarations(n, state)
	case *parse.Assignment:
		return evalAssignment(n, state)
	case *parse.MultiAssignment:
		return evalMultiAssignment(n, state)
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
	case *parse.EmbeddedModule:
		return &AstNode{Node: n.ToChunk()}, nil
	case *parse.Block:
		for _, stmt := range n.Statements {
			res, err := symbolicEval(stmt, state)
			if err != nil {
				return nil, err
			}
			checkCallExprWithUnhandledError(stmt, res, state)
		}
		return nil, nil
	case *parse.SynchronizedBlockStatement:
		return evalSynchronizedBlockStatement(n, state)
	case *parse.PermissionDroppingStatement:
		return nil, nil
	case *parse.InclusionImportStatement:
		return evalInclusionImportStatement(n, state)
	case *parse.ImportStatement:
		return evalImportStatement(n, state)
	case *parse.SpawnExpression:
		return evalSpawnExpression(n, state)
	case *parse.MappingExpression:
		return evalMappingExpression(n, state)
	case *parse.TreedataLiteral:
		return evalTreedataLiteral(n, state, options)
	case *parse.TreedataEntry:
		return evalTreedataEntry(n, state, options)
	case *parse.TreedataPair:
		return evalTreedataPair(n, state, options)
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
		return evalObjectLiteral(n, state, options)
	case *parse.RecordLiteral:
		return evalRecordLiteral(n, state, options)
	case *parse.ListLiteral:
		return evalListLiteral(n, state, options)
	case *parse.TupleLiteral:
		return evalTupleLiteral(n, state, options)
	case *parse.DictionaryLiteral:
		return evalDictionaryLiteral(n, state, options)
	case *parse.IfStatement:
		return evalIfStatement(n, state)
	case *parse.IfExpression:
		return evalIfExpression(n, state)
	case *parse.ForStatement:
		return evalForStatement(n, state)
	case *parse.WalkStatement:
		return evalWalkStatement(n, state)
	case *parse.SwitchStatement:
		return evalSwitchStatement(n, state)
	case *parse.MatchStatement:
		return evalMatchStatement(n, state)
	case *parse.UnaryExpression:
		return evalUnaryExpression(n, state, options)
	case *parse.BinaryExpression:
		return evalBinaryExpression(n, state, options)
	case *parse.UpperBoundRangeExpression:
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
	case *parse.IntegerRangeLiteral:
		if n.LowerBound == nil || n.LowerBound.Err != nil {
			return ANY_INT_RANGE, nil
		}

		if n.UpperBound != nil && n.UpperBound.Base().Err != nil {
			return ANY_INT_RANGE, nil
		}

		lowerBound := NewInt(n.LowerBound.Value)
		upperBound := MAX_INT

		intLit, ok := n.UpperBound.(*parse.IntLiteral)

		if ok {
			upperBound = NewInt(intLit.Value)
		}

		return &IntRange{
			hasValue: true,
			start:    lowerBound,
			end:      upperBound,
		}, nil
	case *parse.FloatRangeLiteral:
		if n.LowerBound == nil || n.LowerBound.Err != nil {
			return ANY_FLOAT_RANGE, nil
		}

		if n.UpperBound != nil && n.UpperBound.Base().Err != nil {
			return ANY_FLOAT_RANGE, nil
		}

		lowerBound := NewFloat(n.LowerBound.Value)
		upperBound := MAX_FLOAT

		floatLit, ok := n.UpperBound.(*parse.FloatLiteral)

		if ok {
			upperBound = NewFloat(floatLit.Value)
		}

		return &FloatRange{
			hasValue:     true,
			start:        lowerBound,
			end:          upperBound,
			inclusiveEnd: true,
		}, nil
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

			if !element.Test(upperBound, RecTestCallState{}) {
				state.addError(makeSymbolicEvalError(n.UpperBound, state, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_OF_SAME_TYPE_AS_LOWER_BOUND))
			}
		}

		return NewQuantityRange(element.(Serializable)), nil
	case *parse.RuneRangeExpression:
		return ANY_RUNE_RANGE, nil
	case *parse.FunctionExpression:
		return evalFunctionExpression(n, state, options)
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
		pattern, err := evalPatternNode(n.Pattern, state)
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
		return evalFunctionPatternExpression(n, state)
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
			val = joinValues([]Value{val, Nil})
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
		return evalExtractionExpression(n, state, options)
	case *parse.DoubleColonExpression:
		return evalDoubleColonExpression(n, state, options)
	case *parse.IndexExpression:
		return evalIndexExpression(n, state, options)
	case *parse.SliceExpression:
		return evalSliceExpression(n, state, options)
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
			names := state.ctx.AllNamedPatternNames()

			var msg = ""
			closestString, _, ok := utils.FindClosestString(state.ctx.startingConcreteContext, names, n.Name, 2)

			if ok {
				msg = fmtPatternIsNotDeclaredYouProbablyMeant(n.Name, closestString)
			} else {
				msg = fmtPatternIsNotDeclared(n.Name)
			}

			state.addError(makeSymbolicEvalError(node, state, msg))
			return ANY_PATTERN, nil
		} else {
			return patt, nil
		}
	case *parse.PatternDefinition:
		pattern, err := evalPatternNode(n.Right, state)
		if err != nil {
			return nil, err
		}
		//TODO: add checks
		state.symbolicData.SetMostSpecificNodeValue(n.Left, pattern)

		name, ok := n.PatternName()
		if !ok {
			return nil, nil
		}

		if state.ctx.ResolveNamedPattern(name) == nil {
			state.ctx.AddNamedPattern(name, pattern, state.inPreinit, state.getCurrentChunkNodePositionOrZero(n.Left))
			state.symbolicData.SetContextData(n, state.ctx.currentData())
		} //else there is already static check error about the duplicate definition.

		return nil, nil
	case *parse.PatternNamespaceDefinition:
		return evalPatternNamespaceDefinition(n, state, options)
	case *parse.PatternNamespaceIdentifierLiteral:
		namespace := state.ctx.ResolvePatternNamespace(n.Name)
		if namespace == nil {
			state.addError(makeSymbolicEvalError(node, state, fmtPatternNamespaceIsNotDeclared(n.Name)))
			return ANY_PATTERN_NAMESPACE, nil
		}
		return namespace, nil
	case *parse.PatternNamespaceMemberExpression:
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
				state.symbolicData.SetMostSpecificNodeValue(n.MemberName, result)
			}
		}()
		namespaceName := n.Namespace.Name
		memberName := n.MemberName.Name

		patt := namespace.entries[memberName] //it's not an issue if namespace.entries is nil

		if patt == nil {
			state.addError(makeSymbolicEvalError(n.MemberName, state, fmtPatternNamespaceHasNotMember(namespaceName, memberName)))
			return ANY_PATTERN, nil
		}
		return patt, nil
	case *parse.OptionalPatternExpression:
		v, err := symbolicEval(n.Pattern, state)
		if err != nil {
			return nil, err
		}

		patt := v.(Pattern)
		if patt.TestValue(Nil, RecTestCallState{}) {
			state.addError(makeSymbolicEvalError(node, state, CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL))
			return &AnyPattern{}, nil
		}

		return &OptionalPattern{pattern: patt}, nil
	case *parse.ComplexStringPatternPiece:
		return NewSequenceStringPattern(n, state.currentChunk().Node), nil
	case *parse.PatternUnion:
		patt := &UnionPattern{}

		for _, case_ := range n.Cases {
			patternElement, err := evalPatternNode(case_, state)
			if err != nil {
				return nil, fmt.Errorf("failed to symbolically compile a pattern element: %s", err.Error())
			}

			patt.cases = append(patt.cases, patternElement)
		}

		return patt, nil
	case *parse.ObjectPatternLiteral:
		return evalObjectPatternLiteral(n, state, options)
	case *parse.RecordPatternLiteral:
		return evalRecordPatternLiteral(n, state, options)
	case *parse.ListPatternLiteral:
		return evalListPatternLiteral(n, state, options)
	case *parse.TuplePatternLiteral:
		return evalTuplePatternLiteral(n, state, options)
	case *parse.OptionPatternLiteral:
		pattern, err := evalPatternNode(n.Value, state)
		if err != nil {
			return nil, err
		}

		return NewOptionPattern(n.Name, pattern), nil
	case *parse.ByteSliceLiteral:
		return ANY_BYTE_SLICE, nil
	case *parse.ConcatenationExpression:
		return evalConcatenationExpression(n, state, options)
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
					narrowChain(binExpr.Left, setExactValue, pattern.SymbolicValue(), state, 0)
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
		return evalTestsuiteExpression(n, state, options)
	case *parse.TestCaseExpression:
		return evalTestcaseExpression(n, state, options)
	case *parse.LifetimejobExpression:
		return evalLifetimejobExpression(n, state, options)
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
		return evalStringTemplateLiteral(n, state, options)
	case *parse.CssSelectorExpression:
		return ANY_STRING, nil
	case *parse.XMLExpression:
		return evalXMLExpression(n, state, options)
	case *parse.XMLElement:
		return evalXMLElement(n, state, options)
	case *parse.XMLInterpolation:

		val, err := symbolicEval(n.Expr, state)
		if err != nil {
			return nil, err
		}

		if state.checkXMLInterpolation != nil {
			msg := state.checkXMLInterpolation(n.Expr, val)
			if msg != "" {
				state.addError(makeSymbolicEvalError(n.Expr, state, msg))
			}
		}

		return val, err
	case *parse.XMLText:
		return ANY_STRING, nil
	case *parse.ExtendStatement:
		return evalExtendStatement(n, state, options)
	case *parse.StructDefinition:
		//handled in evalChunk
		return nil, nil
	// case *parse.PointerType:
	// 	//only misplaced PointerType nodes are evaluated using the symbolicEval function.
	// 	return ANY_PATTERN, nil
	case *parse.NewExpression:
		return evalNewExpression(n, state, options)
	case *parse.DereferenceExpression:
		//not supported yet
		return ANY, nil
	case *parse.UnknownNode:
		return ANY, nil
	default:
		return nil, fmt.Errorf("cannot evaluate %#v (%T)\n%s", node, node, debug.Stack())
	}
}

func evalChunk(n *parse.Chunk, state *State) (_ Value, finalErr error) {
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
			args := make(map[string]Value)

			var paramNames []string
			var paramPatterns []Pattern

			for _, param := range parameters {
				paramNames = append(paramNames, param.name)
				paramPatterns = append(paramPatterns, param.pattern)
				args[param.name] = param.pattern.SymbolicValue()
			}

			paramsPattern := NewModuleParamsPattern(paramNames, paramPatterns)

			if !state.setGlobal(extData.MOD_ARGS_VARNAME, NewModuleArgs(paramsPattern, args), GlobalConst) {
				panic(ErrUnreachable)
			}
		}
	}

	state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
	state.symbolicData.SetContextData(n, state.ctx.currentData())

	//register all structs defined in the current module

	if n == state.Module.mainChunk.Node {
		defineStructs(state.Module.mainChunk, n.Statements, state)
	}

	//evaluation of statements
	if len(n.Statements) == 1 {
		res, err := symbolicEval(n.Statements[0], state)
		if err != nil {
			return nil, err
		}
		checkCallExprWithUnhandledError(n.Statements[0], res, state)
		if state.returnValue != nil && !state.conditionalReturn {
			return state.returnValue, nil
		}

		if res == nil && state.returnValue != nil {
			return joinValues([]Value{state.returnValue, Nil}), nil
		}
		return res, nil
	}

	var returnValue Value
	for _, stmt := range n.Statements {
		res, err := symbolicEval(stmt, state)

		if err != nil {
			return nil, err
		}

		checkCallExprWithUnhandledError(stmt, res, state)

		if state.returnValue != nil {
			if state.conditionalReturn {
				returnValue = state.returnValue
				continue
			}
			return state.returnValue, nil
		}
	}

	return returnValue, nil
}

func evalURLExpression(n *parse.URLExpression, state *State) (_ Value, finalErr error) {

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
}

func evalReturnStatement(n *parse.ReturnStatement, state *State) (_ Value, finalErr error) {

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

	if state.returnType != nil && !state.returnType.Test(v, RecTestCallState{}) {
		if !*deeperMismatch {
			state.addError(makeSymbolicEvalError(n, state, fmtInvalidReturnValue(v, state.returnType)))
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

func evalPatternCallExpression(n *parse.PatternCallExpression, state *State) (_ Value, finalErr error) {
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
		patt, err := callee.(Pattern).Call(state.ctx, args)
		state.consumeSymbolicGoFunctionErrors(func(msg string) {
			state.addError(makeSymbolicEvalError(n, state, msg))
		})
		state.consumeSymbolicGoFunctionWarnings(func(msg string) {
			state.addWarning(makeSymbolicEvalWarning(n, state, msg))
		})

		if err != nil {
			state.addError(makeSymbolicEvalError(n, state, err.Error()))
			patt = ANY_PATTERN
		}
		return patt, nil
	}
	return ANY_PATTERN, nil
}

func evalLocalVariableDeclarations(n *parse.LocalVariableDeclarations, state *State) (finalErr error) {
	for _, decl := range n.Declarations {
		name := decl.Left.(*parse.IdentifierLiteral).Name

		var static Pattern
		var staticMatching Value

		if decl.Type != nil {
			_, ok := decl.Type.(*parse.PointerType)
			if ok {
				state.addError(makeSymbolicEvalError(decl.Type, state, "pointer types are not supported in variable declarations yet"))
				return nil
			}

			type_, err := symbolicEval(decl.Type, state)
			if err != nil {
				return err
			}

			pattern, isPattern := type_.(Pattern)
			if isPattern {
				static = pattern
				staticMatching = static.SymbolicValue()
			} else {
				state.addError(makeSymbolicEvalError(decl.Type, state, VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN))
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
						state.addError(makeSymbolicEvalError(decl.Right, state, fmtNotAssignableToVarOftype(right, static)))
					}
					right = ANY
				} else if holder, ok := right.(StaticDataHolder); ok {
					right, err = holder.AddStatic(static) //TODO: use path narowing, values should never be modified directly
					if err != nil {
						state.addError(makeSymbolicEvalError(decl.Right, state, err.Error()))
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
	return nil
}

func evalGlobalVariableDeclarations(n *parse.GlobalVariableDeclarations, state *State) (finalErr error) {
	for _, decl := range n.Declarations {
		name := decl.Left.(*parse.IdentifierLiteral).Name

		var static Pattern
		var staticMatching Value

		if decl.Type != nil {
			_, ok := decl.Type.(*parse.PointerType)
			if ok {
				state.addError(makeSymbolicEvalError(decl.Type, state, "pointer types are not supported in variable declarations yet"))
				return nil
			}

			type_, err := symbolicEval(decl.Type, state)
			if err != nil {
				return err
			}

			pattern, isPattern := type_.(Pattern)
			if isPattern {
				static = pattern
				staticMatching = static.SymbolicValue()
			} else {
				state.addError(makeSymbolicEvalError(decl.Type, state, VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN))
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
						state.addError(makeSymbolicEvalError(decl.Right, state, fmtNotAssignableToVarOftype(right, static)))
					}
					right = ANY
				} else if holder, ok := right.(StaticDataHolder); ok {
					right, err = holder.AddStatic(static) //TODO: use path narowing, values should never be modified directly
					if err != nil {
						state.addError(makeSymbolicEvalError(decl.Right, state, err.Error()))
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

		state.setGlobal(name, right, GlobalVar, decl.Left)
		state.symbolicData.SetMostSpecificNodeValue(decl.Left, right)
	}
	state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
	return nil
}

func evalAssignment(node *parse.Assignment, state *State) (_ Value, finalErr error) {
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
				state.addError(makeSymbolicEvalError(node.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT))
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

			state.symbolicData.SetLocalScopeData(node, state.currentLocalScopeData())
			state.symbolicData.SetGlobalScopeData(node, state.currentGlobalScopeData())
		}
	}()

	switch lhs := node.Left.(type) {
	case *parse.Variable:
		name := lhs.Name

		if state.hasLocal(name) {
			if node.Operator.Int() {
				info, _ := state.getLocal(name)
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}

				lhsValue := MergeValuesWithSameStaticTypeInMultivalue(info.value)

				if _, ok := lhsValue.(*Int); !ok {
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
			state.setLocal(name, rhs, nil, node.Left)
		}

		//TODO: set to previous value instead ?
		state.symbolicData.SetMostSpecificNodeValue(lhs, __rhs)
		state.symbolicData.SetLocalScopeData(node, state.currentLocalScopeData())
	case *parse.IdentifierLiteral:
		name := lhs.Name

		if state.hasLocal(name) {
			if node.Operator.Int() {
				info, _ := state.getLocal(name)
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}

				lhsValue := MergeValuesWithSameStaticTypeInMultivalue(info.value)

				if _, ok := lhsValue.(*Int); !ok {
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
			if node.Operator.Int() {
				info, _ := state.getGlobal(name)
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}

				lhsValue := MergeValuesWithSameStaticTypeInMultivalue(info.value)

				if _, ok := lhsValue.(*Int); !ok {
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
			state.setLocal(name, rhs, nil, node.Left)
		}

		//TODO: set to previous value instead ?
		state.symbolicData.SetMostSpecificNodeValue(lhs, __rhs)
		state.symbolicData.SetLocalScopeData(node, state.currentLocalScopeData())
	case *parse.GlobalVariable:
		name := lhs.Name

		info, alreadyDefined := state.getGlobal(name)
		if alreadyDefined && info.isConstant {
			state.addError(makeSymbolicEvalError(node, state, fmtAttempToAssignConstantGlobal(name)))
		}

		if state.hasGlobal(name) {
			if node.Operator.Int() {
				info, _ := state.getGlobal(name)
				rhs, _, err := getRHS(nil)
				if err != nil {
					return nil, err
				}

				lhsValue := MergeValuesWithSameStaticTypeInMultivalue(info.value)

				if _, ok := lhsValue.(*Int); !ok {
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
			state.setGlobal(name, rhs, GlobalVar, node.Left)
		}

		//TODO: set to previous value instead ?
		state.symbolicData.SetMostSpecificNodeValue(lhs, __rhs)
		state.symbolicData.SetGlobalScopeData(node, state.currentGlobalScopeData())
	case *parse.MemberExpression:
		object, err := _symbolicEval(lhs.Left, state, evalOptions{
			doubleColonExprAncestorChain: []parse.Node{node},
		})
		if err != nil {
			return nil, err
		}

		if node.Err != nil {
			return nil, nil
		}

		//handle IProps and structs LHS separately

		ptr, ok := object.(*Pointer)
		if ok {
			strct, ok := ptr.value.(*Struct)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, POINTED_VALUE_HAS_NO_PROPERTIES))
				return ANY, nil
			}
			fieldName := lhs.PropertyName.Name
			field, ok := strct.typ.FieldByName(fieldName)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtStructDoesnotHaveField(fieldName)))
				return nil, nil
			}

			rhs, _, err := getRHS(nil)
			if err != nil {
				return nil, err
			}
			rhs = MergeValuesWithSameStaticTypeInMultivalue(rhs)

			if node.Operator.Int() && !utils.Implements[*IntType](field.Type) {
				state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
			} else if !field.Type.TestValue(rhs, RecTestCallState{}) {
				state.addError(makeSymbolicEvalError(node, state, fmtNotAssignableToFieldOfType(rhs, field.Type)))
			}

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
			state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
			return nil, nil
		}

		propName := lhs.PropertyName.Name
		hasPrevValue := utils.SliceContains(iprops.PropertyNames(), propName)

		if hasPrevValue {
			prevValue := iprops.Prop(propName)
			state.symbolicData.SetMostSpecificNodeValue(lhs.PropertyName, prevValue)

			checkNotClonedObjectPropMutation(lhs, state, true)

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

			if node.Operator.Int() {
				widenedPrevValue := MergeValuesWithSameStaticTypeInMultivalue(prevValue)

				if _, ok := widenedPrevValue.(*Int); !ok {
					state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
				}
			} else if badIntOperationRHS {

			} else {
				if newIprops, err := iprops.SetProp(propName, rhs); err != nil {
					if !deeperMismatch {
						state.addError(makeSymbolicEvalError(node, state, err.Error()))
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
				narrowChain(lhs.Left, setExactValue, newIprops, state, 0)
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

		//handle IProps and structs LHS separately

		lastPropNameNode := lhs.PropertyNames[len(lhs.PropertyNames)-1]
		lastPropName := lastPropNameNode.Name

		ptr, ok := v.(*Pointer)
		if ok {
			strct, ok := ptr.value.(*Struct)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, POINTED_VALUE_HAS_NO_PROPERTIES))
				return ANY, nil
			}
			fieldName := lastPropName
			field, ok := strct.typ.FieldByName(fieldName)
			if !ok {
				state.addError(makeSymbolicEvalError(node, state, fmtStructDoesnotHaveField(fieldName)))
				return nil, nil
			}

			rhs, _, err := getRHS(nil)
			if err != nil {
				return nil, err
			}
			rhs = MergeValuesWithSameStaticTypeInMultivalue(rhs)

			if node.Operator.Int() && !utils.Implements[*IntType](field.Type) {
				state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
			} else if !field.Type.TestValue(rhs, RecTestCallState{}) {
				state.addError(makeSymbolicEvalError(node, state, fmtNotAssignableToFieldOfType(rhs, field.Type)))
			}
			return nil, nil
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

		hasPrevValue := utils.SliceContains(iprops.PropertyNames(), lastPropName)

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
			state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE))
			return nil, nil
		}

		if hasPrevValue {
			prevValue := iprops.Prop(lastPropName)
			state.symbolicData.SetMostSpecificNodeValue(lastPropNameNode, prevValue)

			checkNotClonedObjectPropMutation(lhs, state, true)

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

			if _, ok := prevValue.(*Int); !ok && node.Operator.Int() {
				state.addError(makeSymbolicEvalError(node, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
			} else {
				if newIprops, err := iprops.SetProp(lastPropName, rhs); err != nil {
					if !deeperMismatch {
						state.addError(makeSymbolicEvalError(node, state, err.Error()))
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
				narrowChain(lhs, setExactValue, newIprops, state, 1)
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
				state.addError(makeSymbolicEvalError(node.Left, state, ErrReadonlyValueCannotBeMutated.Error()))
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

			if node.Operator.Int() {
				if seqElementAtIndex != nil {
					widenedSeqElementAtIndex := MergeValuesWithSameStaticTypeInMultivalue(seqElementAtIndex)

					if !ANY_INT.Test(widenedSeqElementAtIndex, RecTestCallState{}) {
						state.addError(makeSymbolicEvalError(lhs, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
						ignoreNextAssignabilityError = true
					}
					//note: the element is widened in order to support multivalues such as (1 | 2)
				} else {
					widenedSeqElement := MergeValuesWithSameStaticTypeInMultivalue(seq.Element())
					if !ANY_INT.Test(widenedSeqElement, RecTestCallState{}) {
						state.addError(makeSymbolicEvalError(lhs, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT))
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
							state.addError(makeSymbolicEvalError(node.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENT))
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
					if staticSeq != nil {
						state.addError(makeSymbolicEvalError(node.Right, state, fmtNotAssignableToElementOfValue(__rhs, staticSeqElement)))
					} else {
						state.addError(makeSymbolicEvalError(node.Right, state, fmtNotAssignableToElementOfValue(__rhs, seq.Element())))
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

		checkNotClonedObjectPropMutation(lhs, state, false)

		seq, isMutableSeq := slice.(MutableSequence)
		if isMutableSeq && (!seq.HasKnownLen() ||
			startIntIndex == nil ||
			!startIntIndex.hasValue ||
			(startIntIndex.value >= 0 && startIntIndex.value < int64(seq.KnownLen()))) {

			if IsReadonly(seq) {
				state.addError(makeSymbolicEvalError(node.Left, state, ErrReadonlyValueCannotBeMutated.Error()))
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
					state.addError(makeSymbolicEvalError(node.Right, state, fmtSequenceExpectedButIs(__rhs)))
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
					state.addError(makeSymbolicEvalError(node.Right, state, fmtRHSSequenceShouldHaveLenOf(int(expectedLength))))
				}

				rightSeqElement = rightSeq.Element()
			}

			if staticSeq.HasKnownLen() {
				if !invalidRHSLength && endIntIndex != nil && endIntIndex.hasValue && startIntIndex != nil && startIntIndex.hasValue &&
					staticSeq.KnownLen() > int(startIntIndex.value) {
					//conservatively assume not assignable
				} else {
					state.addError(makeSymbolicEvalError(node.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENTS))
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
				if staticSeq != nil {
					state.addError(makeSymbolicEvalError(node.Right, state, fmtSeqOfXNotAssignableToSliceOfTheValue(rightSeqElement, staticSeq)))
				} else {
					state.addError(makeSymbolicEvalError(node.Right, state, fmtSeqOfXNotAssignableToSliceOfTheValue(rightSeqElement, seq)))
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
		return nil, fmt.Errorf("invalid assignment: left hand side is a(n) %T", node.Left)
	}

	return nil, nil
}

func evalMultiAssignment(n *parse.MultiAssignment, state *State) (_ Value, finalErr error) {
	isNillable := n.Nillable
	right, err := symbolicEval(n.Right, state)
	startRight := right

	if err != nil {
		return nil, err
	}

	seq, ok := right.(Sequence)
	if !ok {
		state.addError(makeSymbolicEvalError(n, state, fmtSeqExpectedButIs(startRight)))
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
			state.addError(makeSymbolicEvalError(n, state, fmtSequenceShouldHaveLengthGreaterOrEqualTo(len(n.Variables))))
		}

		for i, var_ := range n.Variables {
			name := var_.(*parse.IdentifierLiteral).Name

			val := seq.ElementAt(i)
			if isNillable && (!seq.HasKnownLen() || i >= seq.KnownLen() && isNillable) {
				val = joinValues([]Value{val, Nil})
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
}

func evalIfStatement(n *parse.IfStatement, state *State) (_ Value, finalErr error) {
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
}

func evalIfExpression(n *parse.IfExpression, state *State) (_ Value, finalErr error) {

	test, err := symbolicEval(n.Test, state)
	if err != nil {
		return nil, err
	}

	var consequentValue Value
	var atlernateValue Value

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
				return joinValues([]Value{consequentValue, atlernateValue}), nil
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

	state.addError(makeSymbolicEvalError(n, state, fmtIfExprTestNotBoolBut(test)))
	return ANY, nil
}

func evalForStatement(n *parse.ForStatement, state *State) (_ Value, finalErr error) {
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

	var keyType Value = ANY
	var valueType Value = ANY
	evaluateBody := true

	if iterable, ok := asIterable(iteratedValue).(Iterable); ok {
		if n.Chunked {
			state.addError(makeSymbolicEvalError(n, state, "chunked iteration of iterables is not supported yet"))
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
		if n.KeyIndexIdent != nil {
			state.addError(makeSymbolicEvalError(n.KeyIndexIdent, state, KEY_VAR_SHOULD_BE_PROVIDED_ONLY_WHEN_ITERATING_OVER_AN_ITERABLE))
		}
		if n.Chunked {
			valueType = streamable.ChunkedStreamElement()
		} else {
			valueType = streamable.StreamElement()
		}
	} else {
		state.addError(makeSymbolicEvalError(n, state, fmtXisNotIterable(iteratedValue)))
	}

	if n.Body != nil && evaluateBody {
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
}

func evalWalkStatement(n *parse.WalkStatement, state *State) (_ Value, finalErr error) {
	walkedValue, err := symbolicEval(n.Walked, state)
	if err != nil {
		return nil, err
	}

	walkable, ok := walkedValue.(Walkable)

	var nodeMeta, entry Value

	if ok {
		entry = walkable.WalkerElement()
		nodeMeta = walkable.WalkerNodeMeta()
	} else {
		state.addError(makeSymbolicEvalError(n, state, fmtXisNotWalkable(walkedValue)))
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
}

func evalSwitchStatement(n *parse.SwitchStatement, state *State) (_ Value, finalErr error) {
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
			narrowChain(n.Discriminant, setExactValue, caseValue, blockStateFork, 0)

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
}

func evalMatchStatement(n *parse.MatchStatement, state *State) (_ Value, finalErr error) {
	discriminant, err := symbolicEval(n.Discriminant, state)
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
						state.addError(makeSymbolicEvalError(valNode, state, AN_EXACT_VALUE_USED_AS_MATCH_CASE_SHOULD_BE_SERIALIZABLE))
					}
					continue
				} else {
					patt, err := NewExactValuePattern(serializable)
					if err == nil {
						pattern = patt
					} else {
						state.addError(makeSymbolicEvalError(valNode, state, err.Error()))
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
			narrowChain(n.Discriminant, removePossibleValue, val, blockStateFork, 0)
		}

		_, err = symbolicEval(defaultCase.Block, blockStateFork)
		if err != nil {
			return nil, err
		}
	}

	state.join(forks...)

	return nil, nil
}

func evalUnaryExpression(n *parse.UnaryExpression, state *State, options evalOptions) (_ Value, finalErr error) {
	operand, err := symbolicEval(n.Operand, state)
	if err != nil {
		return nil, err
	}
	switch n.Operator {
	case parse.NumberNegate:
		switch {
		case ImplementsOrIsMultivalueWithAllValuesImplementing[*Int](operand):
			return ANY_INT, nil
		case ImplementsOrIsMultivalueWithAllValuesImplementing[*Float](operand):
			return ANY_FLOAT, nil
		default:
			state.addError(makeSymbolicEvalError(n, state, fmtOperandOfNumberNegateShouldBeIntOrFloat(operand)))
		}

		return ANY, nil
	case parse.BoolNegate:
		_, ok := operand.(*Bool)
		if !ok {
			state.addError(makeSymbolicEvalError(n, state, fmtOperandOfBoolNegateShouldBeBool(operand)))
		}

		return ANY_BOOL, nil
	default:
		return nil, fmt.Errorf("invalid unary operator %d", n.Operator)
	}
}

func evalBinaryExpression(n *parse.BinaryExpression, state *State, options evalOptions) (_ Value, finalErr error) {
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

	left = MergeValuesWithSameStaticTypeInMultivalue(left)
	right = MergeValuesWithSameStaticTypeInMultivalue(right)

	switch n.Operator {
	case parse.GreaterThan, parse.LessThan, parse.LessOrEqual, parse.GreaterOrEqual:
		_, ok := left.(Comparable)
		if !ok {
			state.addError(makeSymbolicEvalError(n, state, LEFT_OPERAND_DOES_NOT_IMPL_COMPARABLE_))
			return ANY_BOOL, nil
		}
		_, ok = right.(Comparable)
		if !ok {
			state.addError(makeSymbolicEvalError(n, state, RIGHT_OPERAND_DOES_NOT_IMPL_COMPARABLE_))
			return ANY_BOOL, nil
		}

		if !haveSameGoTypes(left, right) {
			state.addError(makeSymbolicEvalError(n, state, OPERANDS_NOT_COMPARABLE_BECAUSE_DIFFERENT_TYPES))
		}
		return ANY_BOOL, nil
	case parse.Add, parse.Sub, parse.Mul, parse.Div:
		return evalArithmeticBinaryExpression(left, right, n, state)
	case parse.AddDot, parse.SubDot, parse.MulDot, parse.DivDot, parse.GreaterThanDot, parse.GreaterOrEqualDot, parse.LessThanDot, parse.LessOrEqualDot:
		state.addError(makeSymbolicEvalError(n, state, "operator not implemented yet"))
		return ANY, nil
	case parse.Equal, parse.NotEqual, parse.Is, parse.IsNot:
		return ANY_BOOL, nil
	case parse.In:
		switch right.(type) {
		case Container:
		default:
			state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "container", Stringify(right))))
		}
		_, ok := AsSerializable(left).(Serializable)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(left))))
		}
		return ANY_BOOL, nil
	case parse.NotIn:
		switch right.(type) {
		case Container:
		default:
			state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "container", Stringify(right))))
		}
		_, ok := AsSerializable(left).(Serializable)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(left))))
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
	case parse.Urlof:
		if !ImplementsOrIsMultivalueWithAllValuesImplementing[*URL](left) {
			state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "url", Stringify(left))))
		}

		switch right.(type) {
		case *Any, *AnySerializable:
		default:
			if !ImplementsOrIsMultivalueWithAllValuesImplementing[UrlHolder](right) {
				state.addWarning(makeSymbolicEvalWarning(n.Right, state, RIGHT_OPERAND_MAY_NOT_HAVE_A_URL))
			}
		}
		return ANY_BOOL, nil
	case parse.Range, parse.ExclEndRange:
		switch left := left.(type) {
		case *Int:
			if !ANY_INT.Test(right, RecTestCallState{}) {
				msg := fmtRightOperandOfBinaryShouldBeLikeLeftOperand(n.Operator, Stringify(left), Stringify(ANY_INT))
				state.addError(makeSymbolicEvalError(n.Right, state, msg))
				return ANY_INT_RANGE, nil
			}

			rightInt := right.(*Int)
			inclusiveEnd := ANY_INT

			if n.Operator == parse.Range {
				inclusiveEnd = rightInt
			} else if n.Operator == parse.ExclEndRange && rightInt.HasValue() {
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
				state.addError(makeSymbolicEvalError(n.Right, state, msg))
				return ANY_FLOAT_RANGE, nil
			}

			rightFloat := right.(*Float)

			return &FloatRange{
				hasValue:     true,
				inclusiveEnd: n.Operator == parse.Range,
				start:        left,
				end:          rightFloat,
			}, nil
		default:
			if _, ok := left.(Serializable); !ok {
				state.addError(makeSymbolicEvalError(n.Right, state, OPERANDS_OF_BINARY_RANGE_EXPRS_SHOULD_BE_SERIALIZABLE))
				return ANY_QUANTITY_RANGE, nil
			}

			if !left.WidestOfType().Test(right, RecTestCallState{}) {
				msg := fmtRightOperandOfBinaryShouldBeLikeLeftOperand(n.Operator, Stringify(left.WidestOfType()), Stringify(right))
				state.addError(makeSymbolicEvalError(n.Right, state, msg))
			}

			return &QuantityRange{element: left.WidestOfType().(Serializable)}, nil
		}
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
		case BytesLike, StringLike:
		default:
			if _, ok := left.(StringLike); !ok {
				state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "string-like or bytes-like", Stringify(left))))
			}
		}

		switch right.(type) {
		case BytesLike, StringLike:
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
		return joinValues([]Value{narrowOut(Nil, left), right}), nil
	case parse.PairComma:
		leftSerializable, ok := AsSerializable(left).(Serializable)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(left))))
			leftSerializable = ANY_SERIALIZABLE
		} else if leftSerializable.IsMutable() {
			state.addError(makeSymbolicEvalError(n.Left, state, fmtLeftOperandOfBinaryShouldBeImmutable(n.Operator)))
			leftSerializable = ANY_SERIALIZABLE
		}

		rightSerializable, ok := AsSerializable(right).(Serializable)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBe(n.Operator, "serializable", Stringify(right))))
			rightSerializable = ANY_SERIALIZABLE
		} else if rightSerializable.IsMutable() {
			state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandOfBinaryShouldBeImmutable(n.Operator)))
			rightSerializable = ANY_SERIALIZABLE
		}

		return NewOrderedPair(leftSerializable, rightSerializable), nil
	default:
		return nil, fmt.Errorf(fmtInvalidBinaryOperator(n.Operator))
	}
}

// +, -, *, /
func evalArithmeticBinaryExpression(left, right Value, n *parse.BinaryExpression, state *State) (Value, error) {
	if _, ok := left.(*Int); ok {
		_, ok = right.(*Int)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandForIntArithmetic(right, n.Operator)))
		}

		return ANY_INT, nil
	} else if _, ok := left.(*Float); ok {
		_, ok = right.(*Float)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Right, state, fmtRightOperandForFloatArithmetic(right, n.Operator)))
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
	case parse.Add:
		iadd, ok := left.(IPseudoAdd)
		if !ok {
			err := makeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
			state.addError(err)
			return failureResult, nil
		}

		return iadd.Add(right, n, state)
	case parse.Sub:
		isub, ok := left.(IPseudoSub)
		if !ok {
			err := makeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
			state.addError(err)
			return failureResult, nil
		}

		return isub.Sub(right, n, state)
	case parse.Mul:
		err := makeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
		state.addError(err)
		return failureResult, nil
	case parse.Div:
		err := makeSymbolicEvalError(n.Left, state, fmtExpectedLeftOperandForArithmetic(left, n.Operator))
		state.addError(err)
		return failureResult, nil
	default:
		panic(ErrUnreachable)
	}
}

func evalFunctionExpression(n *parse.FunctionExpression, state *State, options evalOptions) (_ Value, finalErr error) {
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
		name := p.Var.Name
		var paramValue Value = ANY
		var paramType Pattern

		if p.Type != nil {
			pattern, err := evalPatternNode(p.Type, stateFork)
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
		stateFork.setLocal(variadicParam.Var.Name, paramType, nil, variadicParam.Var)
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
			if !signatureReturnType.Test(storedReturnType, RecTestCallState{}) {
				state.addError(makeSymbolicEvalError(n.Body, state, fmtInvalidReturnValue(storedReturnType, signatureReturnType)))
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

	state.popCall()

	//check that the body does not contain forbidden node types.

	if expectedFunction, ok := findInMultivalue[*InoxFunction](options.expectedValue); ok && expectedFunction.visitCheckNode != nil {
		visitCheckNode := expectedFunction.visitCheckNode

		parse.Walk(
			n.Body,
			func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if _, isBody := node.(*parse.Block); isBody && node == n.Body {
					return parse.ContinueTraversal, nil
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
		nodeChunk:      state.currentChunk().Node,
		parameters:     params,
		parameterNames: paramNames,
		result:         storedReturnType,
		capturedLocals: capturedLocals,
	}, nil
}

func evalFunctionPatternExpression(n *parse.FunctionPatternExpression, state *State) (_ Value, finalErr error) {

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
			name = p.Var.Name
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
			state.symbolicData.SetMostSpecificNodeValue(p.Var, paramType)
		}
	}

	if n.IsVariadic {
		variadicParam := n.VariadicParameter()
		name := variadicParam.Var.Name
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

		stateFork.setLocal(name, paramType, nil, variadicParam.Var)
		state.symbolicData.SetMostSpecificNodeValue(variadicParam.Var, paramType)
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

func evalSynchronizedBlockStatement(n *parse.SynchronizedBlockStatement, state *State) (_ Value, finalErr error) {
	for _, valNode := range n.SynchronizedValues {
		val, err := symbolicEval(valNode, state)
		if err != nil {
			return nil, err
		}

		if !val.IsMutable() {
			continue
		}

		if potentiallySharable, ok := val.(PotentiallySharable); !ok || !utils.Ret0(potentiallySharable.IsSharable()) {
			state.addError(makeSymbolicEvalError(n, state, fmtSynchronizedValueShouldBeASharableValueOrImmutableNot(val)))
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

func evalInclusionImportStatement(n *parse.InclusionImportStatement, state *State) (_ Value, finalErr error) {
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
	state.symbolicData.SetLocalScopeData(n, state.currentLocalScopeData())
	state.symbolicData.SetGlobalScopeData(n, state.currentGlobalScopeData())
	state.symbolicData.SetContextData(n, state.ctx.currentData())
	return nil, err
}

func evalImportStatement(n *parse.ImportStatement, state *State) (_ Value, finalErr error) {
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

	if !strings.HasSuffix(pathOrURL, inoxconsts.INOXLANG_FILE_EXTENSION) {
		state.addError(makeSymbolicEvalError(n.Source, state, IMPORTED_MOD_PATH_MUST_END_WITH_IX))
		return nil, nil
	}

	importedModule, ok := state.Module.directlyImportedModules[n]
	if !ok {
		return nil, nil
	}

	// TODO: use concrete context with permissions of imported module
	importedModuleContext := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
	for name, basePattern := range state.basePatterns {
		importedModuleContext.AddNamedPattern(name, basePattern, false)
	}

	for name, basePatternNamespace := range state.basePatternNamespaces {
		importedModuleContext.AddPatternNamespace(name, basePatternNamespace, false)
	}

	importPositions := append(slices.Clone(state.importPositions), state.getErrorMesssageLocation(n)...)

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

		ProjectFilesystem: state.projectFilesystem,
	})

	if data == nil && err != nil {
		return nil, err
	}

	return nil, nil
}

func evalSpawnExpression(node *parse.SpawnExpression, state *State) (_ Value, finalErr error) {
	var actualGlobals = map[string]Value{}
	var embeddedModule *parse.Chunk

	var meta map[string]Value
	var globals any
	var permListingNode *parse.ObjectLiteral

	//check permissions
	if !state.ctx.HasAPermissionWithKindAndType(permkind.Create, permkind.LTHREAD_PERM_TYPENAME) {
		warningSpan := parse.NodeSpan{Start: node.Span.Start, End: node.Span.Start + 2}
		state.addWarning(makeSymbolicEvalWarningWithSpan(warningSpan, state, POSSIBLE_MISSING_PERM_TO_CREATE_A_LTHREAD))
	}

	meta = map[string]Value{}
	if objLit, ok := node.Meta.(*parse.ObjectLiteral); ok { //$ok will be false if node.Meta is nil

		for _, sectionProp := range objLit.Properties {
			if sectionProp.HasImplicitKey() {
				//okay because there sould be a static check error
				continue
			}
			sectionName := sectionProp.Name()

			if sectionName == LTHREAD_META_GLOBALS_SECTION {
				globalsObjectLit, ok := sectionProp.Value.(*parse.ObjectLiteral)
				//handle description separately if it's an object literal because non-serializable value are not accepted.
				if ok {
					globalMap := map[string]Value{}
					globals = globalMap

					for _, prop := range globalsObjectLit.Properties {
						if prop.HasImplicitKey() {
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
			} else if sectionName == LTHREAD_META_ALLOW_SECTION && utils.Implements[*parse.ObjectLiteral](sectionProp.Value) {
				permListingNode = sectionProp.Value.(*parse.ObjectLiteral)
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
				state.addError(makeSymbolicEvalError(node.Meta, state, fmtGroupPropertyNotLThreadGroup(v)))
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
				state.addError(makeSymbolicEvalError(node.Meta, state, err.Error()))
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
		if embeddedMod, ok := symbolicNode.Node.(*parse.Chunk); ok {
			embeddedModule = embeddedMod
		} else {
			varname := parse.GetVariableName(node.Module)
			state.addError(makeSymbolicEvalError(node, state, fmtValueOfVarShouldBeAModuleNode(varname)))
		}
	} else {
		varname := parse.GetVariableName(node.Module)
		state.addError(makeSymbolicEvalError(node, state, fmtValueOfVarShouldBeAModuleNode(varname)))
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
		Node:   embeddedModule,
		Source: state.currentChunk().Source,
	})
	modState.Module = state.Module
	modState.symbolicData = state.symbolicData

	for k, v := range actualGlobals {
		modState.setGlobal(k, v, GlobalConst)
	}

	if node.Module.SingleCallExpr {
		calleeNode := node.Module.Statements[0].(*parse.CallExpression).Callee

		switch calleeNode := calleeNode.(type) {
		case *parse.IdentifierLiteral:
			calleeName := calleeNode.Name
			info, ok := state.get(calleeName)
			if ok {
				modState.setGlobal(calleeName, info.value, GlobalConst)
			}
		case *parse.IdentifierMemberExpression:
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
				state.addError(makeSymbolicEvalError(calleeNode.Left, state, INVALID_SPAWN_EXPR_WITH_SHORTHAND_SYNTAX_CALLEE_SHOULD_BE_AN_FN_IDENTIFIER_OR_A_NAMESPACE_METHOD))
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

func evalMappingExpression(n *parse.MappingExpression, state *State) (_ Value, finalErr error) {
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
}

func evalTreedataLiteral(n *parse.TreedataLiteral, state *State, options evalOptions) (Value, error) {

	value, err := symbolicEval(n.Root, state)
	if err != nil {
		return nil, err
	}

	if value.IsMutable() {
		state.addError(makeSymbolicEvalError(n.Root, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
	} else if _, ok := AsSerializable(value).(Serializable); !ok {
		state.addError(makeSymbolicEvalError(n.Root, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
	}

	for _, child := range n.Children {
		_, err := symbolicEval(child, state)
		if err != nil {
			return nil, err
		}
	}
	return &Treedata{}, nil
}

func evalTreedataEntry(n *parse.TreedataEntry, state *State, options evalOptions) (Value, error) {
	value, err := symbolicEval(n.Value, state)
	if err != nil {
		return nil, err
	}

	if value.IsMutable() {
		state.addError(makeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
	} else if _, ok := AsSerializable(value).(Serializable); !ok {
		state.addError(makeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
	}

	for _, child := range n.Children {
		_, err := symbolicEval(child, state)
		if err != nil {
			return nil, err
		}
	}

	return &TreedataHiearchyEntry{}, nil
}

func evalTreedataPair(n *parse.TreedataPair, state *State, options evalOptions) (Value, error) {
	value, err := symbolicEval(n.Key, state)
	if err != nil {
		return nil, err
	}

	var (
		first  Serializable
		second Serializable = ANY_SERIALIZABLE
	)

	if value.IsMutable() {
		state.addError(makeSymbolicEvalError(n.Key, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
		first = ANY_SERIALIZABLE
	} else if serializable, ok := AsSerializable(value).(Serializable); ok {
		first = serializable
	} else {
		state.addError(makeSymbolicEvalError(n.Key, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
		first = ANY_SERIALIZABLE
	}

	if n.Value != nil {
		value, err := symbolicEval(n.Value, state)
		if err != nil {
			return nil, err
		}

		if value.IsMutable() {
			state.addError(makeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_IMMUTABLE))
			second = ANY_SERIALIZABLE
		} else if serializable, ok := AsSerializable(value).(Serializable); ok {
			second = serializable
		} else {
			state.addError(makeSymbolicEvalError(n.Value, state, VALUES_INSIDE_A_TREEDATA_SHOULD_BE_SERIALIZABLE))
			second = ANY_SERIALIZABLE
		}
	}

	return NewOrderedPair(first, second), nil
}

func evalObjectLiteral(n *parse.ObjectLiteral, state *State, options evalOptions) (Value, error) {
	entries := map[string]Serializable{}

	var (
		keys            []string
		props           []*parse.ObjectProperty
		hasLifetimeJobs = false
	)

	var noKeyProps []*parse.ObjectProperty

	//get all keys and properties without a key.
	for _, p := range n.Properties {
		var key string
		hasKey := true

		//add the key
		switch n := p.Key.(type) {
		case *parse.QuotedStringLiteral:
			key = n.Value
		case *parse.IdentifierLiteral:
			key = n.Name
		case nil: //no key
			hasKey = false
		default:
			return nil, fmt.Errorf("invalid key type %T", n)
		}

		if hasKey && key == inoxconsts.IMPLICIT_PROP_NAME { //not allowed (static check error)
			continue
		}

		_, ok := p.Value.(*parse.LifetimejobExpression)

		if ok {
			if hasKey { //error
				continue
			}
			hasLifetimeJobs = true
		}

		if hasKey {
			keys = append(keys, key)
			props = append(props, p)
		} else {
			noKeyProps = append(noKeyProps, p)
		}
	}

	for _, el := range n.SpreadElements {
		_, isExtractionExpr := el.Expr.(*parse.ExtractionExpression)

		evaluatedElement, err := symbolicEval(el.Expr, state)
		if err != nil {
			return nil, err
		}

		if !isExtractionExpr {
			continue
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

	expectedObj, ok := findInMultivalue[*Object](options.expectedValue)
	if !ok {
		expectedObj = &Object{}
	}

	isExact := options.neverModifiedArgument && len(n.SpreadElements) == 0 && !hasLifetimeJobs

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

			state.addError(makeSymbolicEvalError(p.Key, state, msg))
		}

		var (
			propVal      Value
			err          error
			serializable Serializable
		)

		if p.Value == nil {
			propVal = ANY_SERIALIZABLE
			serializable = ANY_SERIALIZABLE
		} else {
			propVal, err = _symbolicEval(p.Value, state, evalOptions{expectedValue: expectedPropVal, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}

			if p.Type != nil {
				_propType, err := symbolicEval(p.Type, state)
				if err != nil {
					return nil, err
				}
				static = _propType.(Pattern)
				if !static.TestValue(propVal, RecTestCallState{}) {
					expected := static.SymbolicValue()
					state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfType(propVal, expected)))
					propVal = expected
				}
			} else if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(propVal, RecTestCallState{}) {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfType(propVal, expectedPropVal)))
			}

			serializable, ok = propVal.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				serializable = ANY_SERIALIZABLE
			} else if _, ok := asWatchable(propVal).(Watchable); !ok && propVal.IsMutable() {
				state.addError(makeSymbolicEvalError(p, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE))
			}

			//additional checks if expected object is readonly
			if expectedObj.readonly && !IsReadonlyOrImmutable(propVal) {
				state.addError(makeSymbolicEvalError(p.Key, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE))
			}
		}

		obj.initNewProp(key, serializable, static)
		state.symbolicData.SetMostSpecificNodeValue(p.Key, propVal)
	}

	//evaluate properties that don't have a key.
	var noKeyValues []Serializable
	for _, p := range noKeyProps {
		propVal, err := symbolicEval(p.Value, state)
		if err != nil {
			return nil, err
		}
		state.symbolicData.SetMostSpecificNodeValue(p.Key, propVal)

		//additional checks if expected object is readonly
		if expectedObj.readonly {
			if _, ok := propVal.(*LifetimeJob); ok {
				state.addError(makeSymbolicEvalError(p, state, LIFETIME_JOBS_NOT_ALLOWED_IN_READONLY_OBJECTS))
			} else if !IsReadonlyOrImmutable(propVal) {
				state.addError(makeSymbolicEvalError(p.Key, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE))
			}
		}
		noKeyValues = append(noKeyValues, AsSerializableChecked(propVal))
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
}

func evalRecordLiteral(n *parse.RecordLiteral, state *State, options evalOptions) (Value, error) {
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
		expectedRecord.ForEachEntry(func(propName string, _ Value) error {
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

		if p.Value == nil {
			entries[key] = ANY_SERIALIZABLE
		} else {
			v, err := _symbolicEval(p.Value, state, evalOptions{expectedValue: expectedPropVal, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}

			if deeperMismatch {
				options.setActualValueMismatchIfNotNil()
			} else if expectedPropVal != nil && !deeperMismatch && !expectedPropVal.Test(v, RecTestCallState{}) {
				options.setActualValueMismatchIfNotNil()
				state.addError(makeSymbolicEvalError(p.Value, state, fmtNotAssignableToPropOfType(v, expectedPropVal)))
			}

			serializable, ok := v.(Serializable)
			if !ok {
				state.addError(makeSymbolicEvalError(p, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE))
				entries[key] = ANY_SERIALIZABLE
			} else if v.IsMutable() {
				state.addError(makeSymbolicEvalError(p.Value, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable(key)))
				entries[key] = ANY_SERIALIZABLE
			} else {
				entries[key] = serializable
			}
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
}

func evalListLiteral(n *parse.ListLiteral, state *State, options evalOptions) (Value, error) {
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

			spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
			if ok {
				val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: resultList, actualValueMismatch: &deeperMismatch})
				if err != nil {
					return nil, err
				}

				list, isList := val.(*List)
				if isList {
					e = list.Element()
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

			if !generalElem.Test(e, RecTestCallState{}) && !deeperMismatch {
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

		spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
		if ok {
			val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: expectedElement, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}

			list, isList := val.(*List)
			if isList {
				e = list.Element()
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
		} else if expectedElement != nil && !expectedElement.Test(e, RecTestCallState{}) && !deeperMismatch {
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

		elements = append(elements, AsSerializableChecked(e))
	}

	resultList := NewList(elements...)
	if expectedList != nil && expectedList.readonly {
		resultList.readonly = true
	}
	return resultList, nil
}

func evalTupleLiteral(n *parse.TupleLiteral, state *State, options evalOptions) (Value, error) {
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
			var e Value
			if ok {
				val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{})
				if err != nil {
					return nil, err
				}

				tuple, isTuple := val.(*Tuple)
				if isTuple {
					e = tuple.Element()
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

			if !generalElem.Test(e, RecTestCallState{}) {
				state.addError(makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(e, generalElemPattern.(Pattern))))
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

		spreadElemNode, ok := elemNode.(*parse.ElementSpreadElement)
		if ok {
			val, err := _symbolicEval(spreadElemNode.Expr, state, evalOptions{expectedValue: expectedElement, actualValueMismatch: &deeperMismatch})
			if err != nil {
				return nil, err
			}

			tuple, isTuple := val.(*Tuple)
			if isTuple {
				e = tuple.Element()
			} else {
				state.addError(makeSymbolicEvalError(spreadElemNode.Expr, state, SPREAD_ELEMENT_SHOULD_BE_A_TUPLE))
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
		} else if expectedElement != nil && !expectedElement.Test(e, RecTestCallState{}) && !deeperMismatch {
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

		elements = append(elements, AsSerializableChecked(e))
	}
	return NewTuple(elements...), nil
}

func evalDictionaryLiteral(n *parse.DictionaryLiteral, state *State, options evalOptions) (Value, error) {

	entries := make(map[string]Serializable)
	keys := make(map[string]Serializable)

	expectedDictionary, ok := findInMultivalue[*Dictionary](options.expectedValue)
	if ok && expectedDictionary.entries != nil {
		var keys []string
		expectedDictionary.ForEachEntry(func(keyRepr string, _ Value) error {
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

		v, err := _symbolicEval(entry.Value, state, evalOptions{expectedValue: expectedEntryValue, actualValueMismatch: &deeperMismatch})
		if err != nil {
			return nil, err
		}
		if deeperMismatch {
			options.setActualValueMismatchIfNotNil()
		} else if expectedEntryValue != nil && !deeperMismatch && !expectedEntryValue.Test(v, RecTestCallState{}) {
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
}

func evalObjectPatternLiteral(n *parse.ObjectPatternLiteral, state *State, options evalOptions) (Value, error) {

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
					state.addError(makeSymbolicEvalError(p.Value, state, PROPERTY_PATTERNS_IN_OBJECT_AND_REC_PATTERNS_MUST_HAVE_SERIALIZABLE_VALUEs))
					propertyValuePattern = &TypePattern{val: ANY_SERIALIZABLE}
				}
			}
		}

		pattern.entries[name] = propertyValuePattern

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
}

func evalRecordPatternLiteral(n *parse.RecordPatternLiteral, state *State, options evalOptions) (Value, error) {
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
				state.addError(makeSymbolicEvalError(p.Value, state, fmtEntriesOfRecordPatternShouldMatchOnlyImmutableValues(name)))
				pattern.entries[name] = &TypePattern{val: ANY_SERIALIZABLE}
			} else {
				//check that the pattern has serializable values
				_, ok := AsSerializable(entryPattern.SymbolicValue()).(Serializable)

				if !ok {
					if errorDuringEval {
						//don't add an irrelevant error

						entryPattern = &TypePattern{val: ANY_SERIALIZABLE}
					} else {
						state.addError(makeSymbolicEvalError(p.Value, state, PROPERTY_PATTERNS_IN_OBJECT_AND_REC_PATTERNS_MUST_HAVE_SERIALIZABLE_VALUEs))
						entryPattern = &TypePattern{val: ANY_SERIALIZABLE}
					}
				}

				pattern.entries[name] = entryPattern
			}
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
}

func evalListPatternLiteral(n *parse.ListPatternLiteral, state *State, options evalOptions) (Value, error) {
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
			state.addError(makeSymbolicEvalError(n.GeneralElement, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
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
				state.addError(makeSymbolicEvalError(e, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
			}

			pattern.elements = append(pattern.elements, elemPattern)
		}
	}

	return pattern, nil
}

func evalTuplePatternLiteral(n *parse.TuplePatternLiteral, state *State, options evalOptions) (Value, error) {
	pattern := &TuplePattern{}

	if n.GeneralElement != nil {
		var err error
		pattern.generalElement, err = evalPatternNode(n.GeneralElement, state)
		if err != nil {
			return nil, err
		}

		generalElement := pattern.generalElement.SymbolicValue()

		if generalElement.IsMutable() {
			state.addError(makeSymbolicEvalError(n.GeneralElement, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES))
			pattern.generalElement = &TypePattern{val: ANY_SERIALIZABLE}
		} else if _, ok := AsSerializable(generalElement).(Serializable); !ok {
			pattern.generalElement = &TypePattern{val: ANY_SERIALIZABLE}
			state.addError(makeSymbolicEvalError(n.GeneralElement, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
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
				state.addError(makeSymbolicEvalError(e, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES))
				elemPattern = &TypePattern{val: ANY_SERIALIZABLE}
			} else if _, ok := AsSerializable(element).(Serializable); !ok {
				elemPattern = &TypePattern{val: ANY_SERIALIZABLE}
				state.addError(makeSymbolicEvalError(e, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
			}

			pattern.elements = append(pattern.elements, elemPattern)
		}
	}

	return pattern, nil
}

func evalConcatenationExpression(n *parse.ConcatenationExpression, state *State, options evalOptions) (Value, error) {
	if len(n.Elements) == 0 {
		return nil, errors.New("cannot create concatenation with no elements")
	}
	var values []Value
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
		//We don't know if the result will be a String or a StringConcatenation.
		return ANY_STR_LIKE, nil
	case BytesLike:
		if len(values) == 1 && !atLeastOneSpread {
			return values[0], nil
		}
		for i, elem := range values {
			if _, ok := elem.(BytesLike); !ok {
				state.addError(makeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmt.Sprintf("bytes concatenation: invalid element of type %T", elem)))
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
				state.addError(makeSymbolicEvalError(n.Elements[nodeIndexes[i]], state, fmt.Sprintf("tuple concatenation: invalid element of type %T", concatElem)))
			}
		}

		if elements == nil {
			return NewTupleOf(AsSerializableChecked(joinValues(generalElements))), nil
		} else {
			return NewTuple(elements...), nil
		}
	default:
		state.addError(makeSymbolicEvalError(n, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION))
		return ANY, nil
	}
}

func evalExtractionExpression(n *parse.ExtractionExpression, state *State, options evalOptions) (Value, error) {
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
}

func evalIndexExpression(n *parse.IndexExpression, state *State, options evalOptions) (Value, error) {
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
		state.addError(makeSymbolicEvalError(n, state, fmtIndexIsNotAnIntButA(index)))
		index = &Int{}
	}

	if indexable, ok := asIndexable(val).(Indexable); ok {
		if intIndex != nil && intIndex.hasValue && indexable.HasKnownLen() && (intIndex.value < 0 || intIndex.value >= int64(indexable.KnownLen())) {
			state.addError(makeSymbolicEvalError(n.Index, state, INDEX_IS_OUT_OF_BOUNDS))
		}
		return indexable.Element(), nil
	}

	state.addError(makeSymbolicEvalError(n, state, fmtXisNotIndexable(val)))
	return ANY, nil
}

func evalSliceExpression(n *parse.SliceExpression, state *State, options evalOptions) (Value, error) {
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
			state.addError(makeSymbolicEvalError(n, state, fmtStartIndexIsNotAnIntButA(index)))
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
			state.addError(makeSymbolicEvalError(n, state, fmtEndIndexIsNotAnIntButA(index)))
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
		state.addError(makeSymbolicEvalError(n, state, fmtSequenceExpectedButIs(slice)))
		return ANY, nil
	}

}

func evalPatternNamespaceDefinition(n *parse.PatternNamespaceDefinition, state *State, options evalOptions) (Value, error) {
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
					state.addError(makeSymbolicEvalError(n.Right, state, err.Error()))
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
					state.addError(makeSymbolicEvalError(n.Right, state, err.Error()))
				}
			}
			namespace.entries[k] = v.(Pattern)
		}
		name, ok := n.NamespaceName()
		if ok {
			namespaceName = name
		}
	default:
		state.addError(makeSymbolicEvalError(n, state, fmtPatternNamespaceShouldBeInitWithNot(right)))
		name, ok := n.NamespaceName()
		if ok {
			namespaceName = name
		}
	}

	if namespaceName != "" && state.ctx.ResolvePatternNamespace(namespaceName) == nil {
		state.ctx.AddPatternNamespace(namespaceName, namespace, state.inPreinit, pos)
		state.symbolicData.SetMostSpecificNodeValue(n.Left, namespace)
		state.symbolicData.SetContextData(n, state.ctx.currentData())
	}

	return nil, nil
}

func evalTestsuiteExpression(n *parse.TestSuiteExpression, state *State, options evalOptions) (Value, error) {
	var testedProgram *TestedProgram

	if n.Meta != nil {
		var err error
		_, testedProgram, err = checkTestItemMeta(n.Meta, state, false)
		if err != nil {
			return nil, err
		}
	} else if state.testedProgram != nil {
		//inherit tested program
		testedProgram = state.testedProgram
	}

	v, err := symbolicEval(n.Module, state)
	if err != nil {
		return nil, err
	}

	embeddedModule := v.(*AstNode).Node.(*parse.Chunk)

	//TODO: read the manifest to known the permissions
	modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
	state.ctx.CopyNamedPatternsIn(modCtx)
	state.ctx.CopyPatternNamespacesIn(modCtx)
	state.ctx.CopyHostAliasesIn(modCtx)

	modState := newSymbolicState(modCtx, &parse.ParsedChunkSource{
		Node:   embeddedModule,
		Source: state.currentChunk().Source,
	})
	modState.Module = state.Module
	modState.symbolicData = state.symbolicData
	modState.testedProgram = testedProgram
	state.forEachGlobal(func(name string, info varSymbolicInfo) {
		modState.setGlobal(name, info.value, GlobalConst)
	})

	//evaluate
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

	return &TestSuite{}, nil
}

func evalTestcaseExpression(n *parse.TestCaseExpression, state *State, options evalOptions) (Value, error) {
	var currentTest *CurrentTest = ANY_CURRENT_TEST
	var testedProgram *TestedProgram

	if n.Meta != nil {
		test, program, err := checkTestItemMeta(n.Meta, state, true)
		if err != nil {
			return nil, err
		}
		currentTest = test
		testedProgram = program
	} else if state.testedProgram != nil {
		//inherit tested program
		testedProgram = state.testedProgram
		currentTest = &CurrentTest{testedProgram: testedProgram}
	}

	v, err := symbolicEval(n.Module, state)
	if err != nil {
		return nil, err
	}

	embeddedModule := v.(*AstNode).Node.(*parse.Chunk)

	//TODO: read the manifest to known the permissions
	modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
	state.ctx.CopyNamedPatternsIn(modCtx)
	state.ctx.CopyPatternNamespacesIn(modCtx)
	state.ctx.CopyHostAliasesIn(modCtx)

	modState := newSymbolicState(modCtx, &parse.ParsedChunkSource{
		Node:   embeddedModule,
		Source: state.currentChunk().Source,
	})
	modState.Module = state.Module
	modState.symbolicData = state.symbolicData
	modState.testedProgram = testedProgram
	state.forEachGlobal(func(name string, info varSymbolicInfo) {
		modState.setGlobal(name, info.value, GlobalConst)
	})

	//add the __test global
	modState.setGlobal(globalnames.CURRENT_TEST, currentTest, GlobalConst)

	//evaluate
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

	return &TestCase{}, nil
}

func evalLifetimejobExpression(n *parse.LifetimejobExpression, state *State, options evalOptions) (Value, error) {

	meta, err := symbolicEval(n.Meta, state)
	if err != nil {
		return nil, err
	}

	if meta.IsMutable() {
		state.addError(makeSymbolicEvalError(n.Meta, state, META_VAL_OF_LIFETIMEJOB_SHOULD_BE_IMMUTABLE))
	}

	var subject Value = ANY
	var subjectPattern Pattern = ANY_PATTERN

	if n.Subject != nil {
		v, err := symbolicEval(n.Subject, state)
		if err != nil {
			return nil, err
		}
		patt, ok := v.(Pattern)

		if !ok {
			state.addError(makeSymbolicEvalError(n, state, fmtSubjectOfLifetimeJobShouldBeObjectPatternNot(v)))
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

	//TODO: read the manifest to known the permissions
	modCtx := NewSymbolicContext(state.ctx.startingConcreteContext, state.ctx.startingConcreteContext, state.ctx)
	state.ctx.ForEachPattern(func(name string, pattern Pattern, knowPosition bool, position parse.SourcePositionRange) {
		modCtx.AddNamedPattern(name, pattern, state.inPreinit, parse.SourcePositionRange{})
	})
	state.ctx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace, knowPosition bool, position parse.SourcePositionRange) {
		modCtx.AddPatternNamespace(name, namespace, state.inPreinit, parse.SourcePositionRange{})
	})

	modState := newSymbolicState(modCtx, &parse.ParsedChunkSource{
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
		if ok && !subject.Test(nextSelf, RecTestCallState{}) {
			state.addError(makeSymbolicEvalError(n, state, fmtSelfShouldMatchLifetimeJobSubjectPattern(subjectPattern)))
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
}

func evalStringTemplateLiteral(n *parse.StringTemplateLiteral, state *State, options evalOptions) (Value, error) {
	_, isPatternAnIdent := n.Pattern.(*parse.PatternIdentifierLiteral)

	if isPatternAnIdent && n.HasInterpolations() {
		state.addError(makeSymbolicEvalError(n, state, STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX))
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
				state.addError(makeSymbolicEvalError(n, state, fmtCannotInterpolatePatternNamespaceDoesNotExist(namespaceName)))
				return &CheckedString{}, nil
			}

			memberName := namespaceMembExpr.MemberName.Name
			_, ok := namespace.entries[memberName]
			if !ok {
				state.addError(makeSymbolicEvalError(n, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(memberName, namespaceName)))
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
		return ANY_STRING, nil
	}

	return &CheckedString{}, nil
}

func evalXMLExpression(n *parse.XMLExpression, state *State, options evalOptions) (Value, error) {

	namespace, err := symbolicEval(n.Namespace, state)
	if err != nil {
		return nil, err
	}

	ns, ok := namespace.(*Namespace)
	if !ok {
		_, err := symbolicEval(n.Element, state)
		if err != nil {
			return nil, err
		}

		state.addError(makeSymbolicEvalError(n.Namespace, state, NAMESPACE_APPLIED_TO_XML_ELEMENT_SHOUD_BE_A_RECORD))
		return ANY, nil
	} else {
		factory, ok := ns.entries[FROM_XML_FACTORY_NAME]
		if !ok {
			state.addError(makeSymbolicEvalError(n.Namespace, state, MISSING_FACTORY_IN_NAMESPACE_APPLIED_TO_XML_ELEMENT))
			return ANY, nil
		}
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

		if goFn.fn != nil {
			checkXMLInterpolation := state.checkXMLInterpolation
			defer func() {
				state.checkXMLInterpolation = checkXMLInterpolation
			}()
			state.checkXMLInterpolation = xmlInterpolationCheckingFunctions[reflect.ValueOf(goFn.fn).Pointer()]
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

		state.consumeSymbolicGoFunctionErrors(func(msg string) {
			state.addError(makeSymbolicEvalError(n, state, msg))
		})
		state.consumeSymbolicGoFunctionWarnings(func(msg string) {
			state.addWarning(makeSymbolicEvalWarning(n, state, msg))
		})

		return result, err
	}
}

func evalXMLElement(n *parse.XMLElement, state *State, options evalOptions) (Value, error) {
	var children []Value
	name := n.Opening.Name.(*parse.IdentifierLiteral).Name
	var attrs map[string]Value
	if len(n.Opening.Attributes) > 0 {
		attrs = make(map[string]Value, len(n.Opening.Attributes))

		for _, attr := range n.Opening.Attributes {
			name := attr.Name.(*parse.IdentifierLiteral).Name
			if attr.Value == nil {
				attrs[name] = ANY_STRING
				continue
			}
			val, err := symbolicEval(attr.Value, state)
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

	xmlElem := NewXmlElement(name, attrs, children)

	state.symbolicData.SetMostSpecificNodeValue(n.Opening.Name, xmlElem)
	if n.Closing != nil {
		state.symbolicData.SetMostSpecificNodeValue(n.Closing.Name, xmlElem)
	}

	return xmlElem, nil
}

func evalDoubleColonExpression(n *parse.DoubleColonExpression, state *State, options evalOptions) (Value, error) {
	left, err := symbolicEval(n.Left, state)
	if err != nil {
		return nil, err
	}

	extensions := state.ctx.GetExtensions(left)
	state.symbolicData.SetAllTypeExtensions(n, extensions)

	obj, isLeftObject := left.(*Object)
	url, isLeftURL := left.(*URL) //ignore URL multivalues because the property resolution would be too ambiguous.

	switch {
	case n.Element != nil && isLeftObject && HasRequiredOrOptionalProperty(obj, n.Element.Name):
		elementName := n.Element.Name

		//get actual value of the property.

		memb := symbolicMemb(obj, elementName, false, n, state)
		state.symbolicData.SetMostSpecificNodeValue(n.Element, memb)

		if IsAnyOrAnySerializable(memb) || utils.Ret0(IsSharable(memb)) {
			state.addError(makeSymbolicEvalError(n, state, RHS_OF_DOUBLE_COLON_EXPRS_WITH_OBJ_LHS_SHOULD_BE_THE_NAME_OF_A_MUTABLE_NON_SHARABLE_VALUE_PROPERTY))
		} else if len(options.doubleColonExprAncestorChain) == 0 {
			state.addError(makeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR))
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
				state.addError(makeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR))
			}
		}
		return memb, nil
	case isLeftURL && url.hasValue:
		//resolve

		valAtURL, err := GetValueAtURL(url, state)
		if err != nil {
			state.addError(makeSymbolicEvalError(n.Left, state, err.Error()))
			return ANY_SERIALIZABLE, nil
		}

		iprops, ok := valAtURL.(IProps)
		if !ok {
			state.addError(makeSymbolicEvalError(n.Element, state, fmtValueAtURLHasNoProperties(valAtURL)))
			state.symbolicData.SetMostSpecificNodeValue(n.Element, ANY_SERIALIZABLE)
			return ANY_SERIALIZABLE, nil
		}
		state.symbolicData.SetURLReferencedEntity(n, iprops)

		if n.Element == nil {
			//parsing error.
			return ANY_SERIALIZABLE, nil
		}

		elementName := n.Element.Name

		if !slices.Contains(iprops.PropertyNames(), elementName) {
			state.addError(makeSymbolicEvalError(n.Element, state, fmtValueAtURLDoesNotHavePropX(valAtURL, elementName)))
			state.symbolicData.SetMostSpecificNodeValue(n.Element, ANY_SERIALIZABLE)
			return ANY_SERIALIZABLE, nil
		}

		val := iprops.Prop(elementName)
		state.symbolicData.SetMostSpecificNodeValue(n.Element, val)

		return val, nil
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
					state.addError(makeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR_EXT_METHOD_CAN_ONLY_BE_CALLED))
					return ANY, nil
				}

				//check not misplaced
				misplaced := true
				ancestors := options.doubleColonExprAncestorChain
				rootAncestor := ancestors[0]
				switch rootAncestor.(type) {
				case *parse.CallExpression:
					misplaced = false
				default:
				}
				if misplaced {
					state.addError(makeSymbolicEvalError(n, state, MISPLACED_DOUBLE_COLON_EXPR_EXT_METHOD_CAN_ONLY_BE_CALLED))
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
			state.symbolicData.SetMostSpecificNodeValue(n.Element, result)
			state.symbolicData.SetUsedTypeExtension(n, extension)
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

		state.addError(makeSymbolicEvalError(n, state, fmtExtensionsDoNotProvideTheXProp(elementName, suggestion)))
		return ANY_SERIALIZABLE, nil
	}
}

func evalExtendStatement(n *parse.ExtendStatement, state *State, options evalOptions) (Value, error) {
	if n.Err != nil && n.Err.Kind == parse.UnterminatedExtendStmt {
		return nil, nil
	}

	pattern, err := evalPatternNode(n.ExtendedPattern, state)
	if err != nil {
		return nil, err
	}

	if !IsConcretizable(pattern) {
		state.addError(makeSymbolicEvalError(n.ExtendedPattern, state, EXTENDED_PATTERN_MUST_BE_CONCRETIZABLE_AT_CHECK_TIME))
		return nil, nil
	}

	extendedValue := pattern.SymbolicValue()

	if _, ok := extendedValue.(Serializable); !ok {
		state.addError(makeSymbolicEvalError(n.ExtendedPattern, state, ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED))
		return nil, nil
	}

	objLit, ok := n.Extension.(*parse.ObjectLiteral)
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
		if prop.HasImplicitKey() {
			state.addError(makeSymbolicEvalError(prop, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS))
			continue
		}

		key := prop.Name()
		if extData.IsIndexKey(key) {
			state.addError(makeSymbolicEvalError(prop.Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS))
			continue
		}

		expr, ok := parse.ParseExpression(key)
		_, isIdent := expr.(*parse.IdentifierLiteral)
		if !ok || !isIdent {
			state.addError(makeSymbolicEvalError(prop.Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS))
			continue
		}

		if extendedValueIprops != nil {
			if HasRequiredOrOptionalProperty(extendedValueIprops, key) {
				state.addError(makeSymbolicEvalError(prop.Key, state, fmtExtendedValueAlreadyHasAnXProperty(key)))
				continue
			}
		}

		switch v := prop.Value.(type) {
		case *parse.FunctionExpression:
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
	// 	keyToProp       memds.Map32[string, *parse.ObjectProperty]
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
	// 	case *parse.QuotedStringLiteral:
	// 		key = n.Value
	// 		_, err := strconv.ParseUint(key, 10, 32)
	// 		if err == nil {
	// 			//see Check function
	// 			indexKey++
	// 		}
	// 	case *parse.IdentifierLiteral:
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
	// 	_, isExtractionExpr := el.Expr.(*parse.ExtractionExpression)

	// 	evaluatedElement, err := symbolicEval(el.Expr, state)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	if !isExtractionExpr {
	// 		continue
	// 	}

	// 	object := evaluatedElement.(*Object)

	// 	for _, key := range el.Expr.(*parse.ExtractionExpression).Keys.Keys {
	// 		name := key.(*parse.IdentifierLiteral).Name
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

	// 	if _, ok := p.Value.(*parse.FunctionExpression); !ok {
	// 		continue
	// 	}

	// 	hasMethods = true

	// 	// find the method's dependencies
	// 	parse.Walk(p.Value, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

	// 		if parse.IsScopeContainerNode(node) && node != p.Value {
	// 			return parse.Prune, nil
	// 		}

	// 		switch node.(type) {
	// 		case *parse.SelfExpression:
	// 			dependencyName := ""

	// 			switch p := parent.(type) {
	// 			case *parse.MemberExpression:
	// 				dependencyName = p.PropertyName.Name
	// 			case *parse.DynamicMemberExpression:
	// 				dependencyName = p.PropertyName.Name
	// 			}

	// 			if dependencyName == "" {
	// 				break
	// 			}

	// 			depId, ok := dependencyGraph.IdOfNode(dependencyName)
	// 			if !ok {
	// 				//?
	// 				return parse.ContinueTraversal, nil
	// 			}

	// 			if dependentKeyId == depId {
	// 				selfDependent = append(selfDependent, dependentKey)
	// 			} else if !dependencyGraph.HasEdgeFromTo(dependentKeyId, depId) {
	// 				// dependentKey ->- depKey
	// 				dependencyGraph.AddEdge(dependentKeyId, depId)
	// 			}
	// 		}
	// 		return parse.ContinueTraversal, nil
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
	// 	if _, ok := p1.Value.(*parse.LifetimejobExpression); ok {
	// 		hasLifetimeJobs = true
	// 		if p1.HasImplicitKey() {
	// 			return false
	// 		}
	// 	}
	// 	p2 := keyToProp.MustGet(keyB)
	// 	if _, ok := p2.Value.(*parse.LifetimejobExpression); ok && p2.HasImplicitKey() {
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
	// 	state.symbolicData.SetMostSpecificNodeValue(p.Key, propVal)
	// }
	// state.unsetNextSelf()
	// if restoreNextSelf {
	// 	state.setNextSelf(prevNextSelf)
	// }

	for _, metaProp := range objLit.MetaProperties {
		state.addError(makeSymbolicEvalError(metaProp, state, META_PROPERTIES_NOT_ALLOWED_IN_EXTENSION_OBJECT))
	}

	state.ctx.AddTypeExtension(extension)
	state.symbolicData.SetContextData(n, state.ctx.currentData())

	return nil, nil
}

func evalNewExpression(n *parse.NewExpression, state *State, options evalOptions) (Value, error) {
	comptimeTypes := state.symbolicData.getModuleComptimeTypes(state.Module.mainChunk.Node, false)

	switch typeNode := n.Type.(type) {
	case *parse.PatternIdentifierLiteral:
		typeName := typeNode.Name
		patt := state.ctx.ResolveNamedPattern(typeName)
		if patt != nil && !IsNameOfBuiltinComptimeType(typeName) {
			state.addError(makeSymbolicEvalError(typeNode, state, ONLY_COMPILE_TIME_TYPES_CAN_BE_USED_IN_NEW_EXPRS))
			return ANY, nil
		}

		comptimeType, ok := comptimeTypes.GetPointerType(typeName)
		if !ok {
			state.addError(makeSymbolicEvalError(typeNode, state, fmtCompileTimeTypeIsNotDefined(typeName)))
			return ANY, nil
		}

		return comptimeType.SymbolicValue(), nil
	case *parse.PointerType:
		state.addError(makeSymbolicEvalError(typeNode, state, POINTER_TYPES_CANNOT_BE_USED_IN_NEW_EXPRS_YET))
	default:
		state.addError(makeSymbolicEvalError(typeNode, state, ONLY_COMPILE_TIME_TYPES_CAN_BE_USED_AS_STRUCT_FIELD_TYPES))
	}
	return ANY, nil
}

func symbolicMemb(value Value, name string, optionalMembExpr bool, node parse.Node, state *State) (result Value) {
	//note: the property of a %serializable is not necessarily serializable (example: Go methods)

	pointer, ok := value.(*Pointer)
	if ok {
		strct, ok := pointer.value.(*Struct)
		if !ok {
			state.addError(makeSymbolicEvalError(node, state, POINTED_VALUE_HAS_NO_PROPERTIES))
			return ANY
		}
		if optionalMembExpr {
			state.addError(makeSymbolicEvalError(node, state, OPTIONAL_MEMBER_EXPRS_NOT_ALLOWED_FOR_STRUCT_FIELDS))
			return ANY
		}
		field, ok := strct.typ.FieldByName(name)
		if ok {
			return field.Type.SymbolicValue()
		}
		state.addError(makeSymbolicEvalError(node, state, fmtStructDoesnotHaveField(name)))
		return ANY
	}

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
				state.addError(makeSymbolicEvalError(node, state, fmtPropOfDoesNotExist(name, value, closest)))
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

func handleConstraints(obj *Object, block *parse.InitializationBlock, state *State) error {
	//we first there are only authorized statements & expressions in the initialization block

	err := parse.Walk(block, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		if node == block {
			return parse.ContinueTraversal, nil
		}

		switch node.(type) {
		case *parse.BinaryExpression:
		case *parse.SelfExpression:
		case *parse.MemberExpression:
		case parse.SimpleValueLiteral:
		default:
			state.addError(makeSymbolicEvalError(node, state, CONSTRAINTS_INIT_BLOCK_EXPLANATION))
		}
		return parse.ContinueTraversal, nil
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
				if utils.Implements[*parse.SelfExpression](node) && utils.Implements[*parse.MemberExpression](parent) {
					constraint.Properties = append(constraint.Properties, parent.(*parse.MemberExpression).PropertyName.Name)
				}
				return parse.ContinueTraversal, nil
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
	return makeSymbolicEvalWarningWithSpan(node.Base().Span, state, msg)
}

func makeSymbolicEvalWarningWithSpan(nodeSpan parse.NodeSpan, state *State, msg string) SymbolicEvaluationWarning {
	locatedMsg := msg
	location := state.getErrorMesssageLocationOfSpan(nodeSpan)
	if state.Module != nil {
		locatedMsg = fmt.Sprintf("check(symbolic): warning: %s: %s", location, msg)
	}
	return SymbolicEvaluationWarning{msg, locatedMsg, location}
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
	case VALUE_RECEIVER_INTERFACE_TYPE:
		result = ANY_MSG_RECEIVER
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
