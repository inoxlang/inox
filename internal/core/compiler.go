package core

import (
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

type CompilationInput struct {
	Mod             *Module
	Globals         map[string]Value
	SymbolicData    *symbolic.SymbolicData
	StaticCheckData *StaticCheckData
	TraceWriter     io.Writer
	Context         *Context
}

// Compile compiles a module to bytecode.
func Compile(input CompilationInput) error {
	c := NewCompiler(input.Mod, input.Globals, input.SymbolicData, input.StaticCheckData, input.Context, input.TraceWriter)
	_, err := c.compileMainChunk(input.Mod.MainChunk)
	if err != nil {
		return err
	}
	return nil
}

// compiler compiles the AST into a bytecode.
type compiler struct {
	module                *Module
	symbolicData          *symbolic.SymbolicData
	staticCheckData       *StaticCheckData
	constants             []Value
	globalSymbols         *symbolTable
	localSymbolTableStack []*symbolTable
	scopes                []compilationScope
	scopeIndex            int
	chunkStack            []*parse.ParsedChunk
	loops                 []*loop
	loopIndex             int
	walkIndex             int
	trace                 io.Writer
	indent                int
	lastOp                Opcode

	context *Context
}

// compilationScope contains the instructions for a scope.
type compilationScope struct {
	instructions []byte
	sourceMap    map[int]instructionSourcePosition
}

// loop is used by the compiler to track the current loop, see LoopKind.
type loop struct {
	kind              LoopKind
	continuePositions []int
	breakPositions    []int
	iteratorSymbol    *symbol
}

type CompileError struct {
	Module  *Module
	Node    parse.Node
	Err     error
	Message string
}

func (e *CompileError) Error() string {
	return e.Message
}

// NewCompiler creates a Compiler.
func NewCompiler(
	mod *Module,
	globals map[string]Value,
	symbolicData *symbolic.SymbolicData,
	staticCheckData *StaticCheckData,
	ctx *Context,
	trace io.Writer,
) *compiler {
	mainScope := compilationScope{
		sourceMap: make(map[int]instructionSourcePosition),
	}

	symbTable := newSymbolTable()
	for name := range globals {
		symbTable.Define(name)
	}

	if symbolicData == nil {
		symbolicData = symbolic.NewSymbolicData()
	}

	return &compiler{
		module:                mod,
		symbolicData:          symbolicData,
		staticCheckData:       staticCheckData,
		globalSymbols:         symbTable,
		localSymbolTableStack: []*symbolTable{},
		scopes:                []compilationScope{mainScope},
		scopeIndex:            0,
		loopIndex:             -1,
		walkIndex:             -1,
		trace:                 trace,
		context:               ctx,
	}
}

// Compile compiles an AST node.
func (c *compiler) Compile(node parse.Node) error {
	if c.trace != nil {
		if node != nil {
			defer func() {
				c.enterTracingBlock(fmt.Sprintf("(%s)", reflect.TypeOf(node).Elem().Name()))
				c.leaveTracingBlock()
			}()
		} else {

			defer func() {
				c.enterTracingBlock("<nil>")
				c.leaveTracingBlock()
			}()
		}
	}

	switch node := node.(type) {
	case *parse.GlobalConstantDeclarations:
		for _, decl := range node.Declarations {
			if err := c.Compile(decl); err != nil {
				return err
			}
		}
	case *parse.GlobalConstantDeclaration:
		c.globalSymbols.Define(node.Ident().Name)

		if err := c.Compile(node.Right); err != nil {
			return err
		}

		c.emit(node, OpSetGlobal, c.addConstant(Str(node.Ident().Name)))
	case *parse.BinaryExpression:
		if node.Operator == parse.And || node.Operator == parse.Or {
			return c.compileLogical(node)
		}

		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Right); err != nil {
			return err
		}

		switch node.Operator {
		case parse.Add, parse.Sub, parse.Mul, parse.Div, parse.LessThan, parse.LessOrEqual, parse.GreaterThan, parse.GreaterOrEqual:
			c.emit(node, OpNumBin, int(node.Operator))
			//TODO: emit other opcodes based on the operands' type.
		case parse.AddDot, parse.SubDot, parse.MulDot, parse.DivDot, parse.LessThanDot, parse.LessOrEqualDot, parse.GreaterThanDot, parse.GreaterOrEqualDot:
			return errors.New("dot operators not supported yet")
		case parse.Range, parse.ExclEndRange:
			exclEnd := 0
			if node.Operator == parse.ExclEndRange {
				exclEnd = 1
			}
			c.emit(node, OpRange, exclEnd)
		case parse.Equal:
			c.emit(node, OpEqual)
		case parse.NotEqual:
			c.emit(node, OpNotEqual)
		case parse.Is:
			c.emit(node, OpIs)
		case parse.IsNot:
			c.emit(node, OpIsNot)
		case parse.Match:
			c.emit(node, OpMatch)
		case parse.NotMatch:
			c.emit(node, OpMatch)
			c.emit(node, OpBooleanNot)
		case parse.In:
			c.emit(node, OpIn)
		case parse.NotIn:
			c.emit(node, OpIn)
			c.emit(node, OpBooleanNot)
		case parse.Substrof:
			c.emit(node, OpSubstrOf)
		case parse.Keyof:
			c.emit(node, OpKeyOf)
		case parse.SetDifference:
			c.emit(node, OpToPattern)
			c.emit(node, OpDoSetDifference)
		case parse.NilCoalescing:
			c.emit(node, OpNilCoalesce)
		default:
			return c.NewError(node, makeInvalidBinaryOperator(node.Operator))
		}
	case *parse.UpperBoundRangeExpression:
		if err := c.Compile(node.UpperBound); err != nil {
			return err
		}
		c.emit(node, OpCreateUpperBoundRange)
	case *parse.IntegerRangeLiteral:
		if err := c.Compile(node.LowerBound); err != nil {
			return err
		}
		if node.UpperBound == nil {
			c.emit(node, OpPushConstant, c.addConstant(Int(math.MaxInt64)))
		} else if err := c.Compile(node.UpperBound); err != nil {
			return err
		}
		c.emit(node, OpCreateIntRange)
	case *parse.FloatRangeLiteral:
		if err := c.Compile(node.LowerBound); err != nil {
			return err
		}
		if node.UpperBound == nil {
			c.emit(node, OpPushConstant, c.addConstant(Float(math.MaxFloat64)))
		} else if err := c.Compile(node.UpperBound); err != nil {
			return err
		}
		c.emit(node, OpCreateFloatRange)
	case *parse.QuantityRangeLiteral:
		qtyRange := mustEvalQuantityRange(node)
		c.emit(node, OpPushConstant, c.addConstant(qtyRange))
	case *parse.RuneRangeExpression:
		if err := c.Compile(node.Lower); err != nil {
			return err
		}
		if err := c.Compile(node.Upper); err != nil {
			return err
		}
		c.emit(node, OpCreateRuneRange)
	case *parse.IntLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Int(node.Value)))
	case *parse.FloatLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Float(node.Value)))
	case *parse.PortLiteral:
		value := utils.Must(evalSimpleValueLiteral(node, nil))
		c.emit(node, OpPushConstant, c.addConstant(value))
	case *parse.BooleanLiteral:
		if node.Value {
			c.emit(node, OpPushTrue)
		} else {
			c.emit(node, OpPushFalse)
		}
	case *parse.UnambiguousIdentifierLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Identifier(node.Name)))
	case *parse.PropertyNameLiteral:
		c.emit(node, OpPushConstant, c.addConstant(PropertyName(node.Name)))
	case *parse.DateLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Date(node.Value)))
	//quantities & rates
	case *parse.QuantityLiteral:
		//This implementation does not allow custom units.
		//Should it be entirely external ? Should most common units be still handled here ?
		q, err := evalQuantity(node.Values, node.Units)
		if err != nil {
			return err
		}
		c.emit(node, OpPushConstant, c.addConstant(q))

	case *parse.RateLiteral:
		q, err := evalQuantity(node.Values, node.Units)
		if err != nil {
			return err
		}

		v, err := evalRate(q, node.DivUnit)
		if err != nil {
			return err
		}
		c.emit(node, OpPushConstant, c.addConstant(v))
	//strings
	case *parse.QuotedStringLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Str(node.Value)))
	case *parse.UnquotedStringLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Str(node.Value)))
	case *parse.MultilineStringLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Str(node.Value)))
	case *parse.RuneLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Rune(node.Value)))
	//paths
	case *parse.AbsolutePathLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Path(node.Value)))
	case *parse.RelativePathLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Path(node.Value)))
	case *parse.AbsolutePathPatternLiteral:
		c.emit(node, OpPushConstant, c.addConstant(PathPattern(node.Value)))
	case *parse.RelativePathPatternLiteral:
		c.emit(node, OpPushConstant, c.addConstant(PathPattern(node.Value)))
	//url & hosts
	case *parse.URLLiteral:
		c.emit(node, OpPushConstant, c.addConstant(URL(node.Value)))
	case *parse.URLPatternLiteral:
		c.emit(node, OpPushConstant, c.addConstant(URLPattern(node.Value)))
	case *parse.SchemeLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Scheme(node.Name)))
	case *parse.HostLiteral:
		c.emit(node, OpPushConstant, c.addConstant(Host(node.Value)))
	case *parse.HostPatternLiteral:
		c.emit(node, OpPushConstant, c.addConstant(HostPattern(node.Value)))
	case *parse.EmailAddressLiteral:
		c.emit(node, OpPushConstant, c.addConstant(EmailAddress(node.Value)))
	case *parse.NilLiteral:
		c.emit(node, OpPushNil)
	case *parse.NamedSegmentPathPatternLiteral:
		c.emit(node, OpPushConstant, c.addConstant(&NamedSegmentPathPattern{node: node}))
	case *parse.PathSlice:
		c.emit(node, OpPushConstant, c.addConstant(Str(node.Value)))
	case *parse.PathPatternSlice:
		c.emit(node, OpPushConstant, c.addConstant(Str(node.Value)))
	case *parse.RegularExpressionLiteral:
		patt := NewRegexPattern(node.Value)
		c.emit(node, OpPushConstant, c.addConstant(patt))
	case *parse.URLQueryParameterValueSlice:
		c.emit(node, OpPushConstant, c.addConstant(Str(node.Value)))
	case *parse.FlagLiteral:
		val := Option{Name: node.Name, Value: Bool(true)}
		c.emit(node, OpPushConstant, c.addConstant(val))
	case *parse.AtHostLiteral:
		c.emit(node, OpResolveHost, c.addConstant(Str(node.Value)))
	case *parse.ByteSliceLiteral:
		c.emit(node, OpPushConstant, c.addConstant(&ByteSlice{Bytes: utils.CopySlice(node.Value), IsDataMutable: true}))
	case *parse.OptionExpression:
		if err := c.Compile(node.Value); err != nil {
			return err
		}

		c.emit(node, OpCreateOption, c.addConstant(Str(node.Name)))
	case *parse.AbsolutePathExpression, *parse.RelativePathExpression:

		var slices []parse.Node

		switch pexpr := node.(type) {
		case *parse.AbsolutePathExpression:
			slices = pexpr.Slices
		case *parse.RelativePathExpression:
			slices = pexpr.Slices
		}

		if len(slices) > math.MaxUint8 {
			return errors.New("too many slices")
		}
		isStaticPathSliceList := &List{underlyingList: &ValueList{}}

		for _, node := range slices {
			_, isStaticPathSlice := node.(*parse.PathSlice)
			isStaticPathSliceList.append(nil, Bool(isStaticPathSlice))

			if err := c.Compile(node); err != nil {
				return err
			}
		}

		c.emit(node, OpCreatePath, len(slices), c.addConstant(isStaticPathSliceList))
		//
	case *parse.PathPatternExpression:

		if len(node.Slices) > math.MaxUint8 {
			return errors.New("too many slices")
		}
		isStaticPathSliceList := &List{underlyingList: &ValueList{}}

		for _, node := range node.Slices {
			_, isStaticPathSlice := node.(*parse.PathPatternSlice)
			isStaticPathSliceList.append(nil, Bool(isStaticPathSlice))

			if err := c.Compile(node); err != nil {
				return err
			}
		}

		c.emit(node, OpCreatePathPattern, len(node.Slices), c.addConstant(isStaticPathSliceList))
		//
	case *parse.URLExpression:
		//compile host
		if err := c.Compile(node.HostPart); err != nil {
			return err
		}

		//compile path
		info := ValMap{
			"path-slice-count": Int(len(node.Path)),
		}
		var isStaticPathSliceList []Serializable

		for _, pathSlice := range node.Path {
			_, isStaticPathSlice := pathSlice.(*parse.PathSlice)
			isStaticPathSliceList = append(isStaticPathSliceList, Bool(isStaticPathSlice))

			if err := c.Compile(pathSlice); err != nil {
				return err
			}
		}
		info["static-path-slices"] = NewTuple(isStaticPathSliceList)
		//compile query

		var queryParamInfo []Serializable

		for _, p := range node.QueryParams {
			param := p.(*parse.URLQueryParameter)
			queryParamInfo = append(queryParamInfo, Str(param.Name), Int(len(param.Value)))

			for i, n := range param.Value {
				if err := c.Compile(n); err != nil {
					return err
				}
				if _, ok := n.(*parse.URLQueryParameterValueSlice); !ok {
					c.emit(node, OptStrQueryParamVal)
				}
				if i != 0 {
					c.emit(node, OpStrConcat)
				}
			}
		}
		info["query-params"] = NewTuple(queryParamInfo)

		c.emit(node, OpCreateURL, c.addConstant(NewRecordFromMap(info)))
	case *parse.HostExpression:
		if err := c.Compile(node.Host); err != nil {
			return err
		}
		c.emit(node, OpCreateHost, c.addConstant(Str(node.Scheme.Name)))
	case *parse.UnaryExpression:
		if err := c.Compile(node.Operand); err != nil {
			return err
		}

		switch node.Operator {
		case parse.NumberNegate:
			c.emit(node, OpMinus)
		case parse.BoolNegate:
			c.emit(node, OpBooleanNot)
		default:
			return c.NewError(node, fmt.Sprintf("invalid unary operator: %d", node.Operator))
		}
	case *parse.IfStatement:

		if err := c.Compile(node.Test); err != nil {
			return err
		}

		// first jump placeholder
		jumpPos1 := c.emit(node, OpJumpIfFalse, 0)
		if err := c.Compile(node.Consequent); err != nil {
			return err
		}
		if node.Alternate != nil {
			// second jump placeholder
			jumpPos2 := c.emit(node, OpJump, 0)

			// update first jump offset
			curPos := len(c.currentInstructions())
			c.changeOperand(jumpPos1, curPos)
			if err := c.Compile(node.Alternate); err != nil {
				return err
			}

			// update second jump offset
			curPos = len(c.currentInstructions())
			c.changeOperand(jumpPos2, curPos)
		} else {
			// update first jump offset
			curPos := len(c.currentInstructions())
			c.changeOperand(jumpPos1, curPos)
		}
	case *parse.IfExpression:
		if err := c.Compile(node.Test); err != nil {
			return err
		}

		// first jump placeholder
		jumpPos1 := c.emit(node, OpJumpIfFalse, 0)
		if err := c.Compile(node.Consequent); err != nil {
			return err
		}
		// second jump placeholder
		jumpPos2 := c.emit(node, OpJump, 0)

		// update first jump offset
		curPos := len(c.currentInstructions())
		c.changeOperand(jumpPos1, curPos)

		if node.Alternate != nil {
			if err := c.Compile(node.Alternate); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}

		// update second jump offset
		curPos = len(c.currentInstructions())
		c.changeOperand(jumpPos2, curPos)

	case *parse.ForStatement:
		itSuffix := strconv.Itoa(int(node.Span.Start))
		itSymbol := c.currentLocalSymbols().Define(":it" + itSuffix)
		streamElemSymbol := c.currentLocalSymbols().Define(":streamElem" + itSuffix)

		if err := c.Compile(node.IteratedValue); err != nil {
			return err
		}

		//iterator initialization
		config := 0
		if node.KeyPattern != nil || node.ValuePattern != nil {
			config = 1
			if node.KeyPattern != nil {
				if err := c.Compile(node.KeyPattern); err != nil {
					return err
				}
			} else {
				c.emit(node, OpPushNil)
			}
			if node.ValuePattern != nil {
				if err := c.Compile(node.ValuePattern); err != nil {
					return err
				}
			} else {
				c.emit(node, OpPushNil)
			}
		}
		_ = 1
		c.emit(node, OpIterInit, config)
		c.emit(node, OpSetLocal, itSymbol.Index)

		// pre-condition position
		preCondPos := len(c.currentInstructions())

		// condition
		c.emit(node, OpGetLocal, itSymbol.Index)
		if node.Chunked {
			c.emit(node, OpIterNextChunk, streamElemSymbol.Index)
		} else {
			c.emit(node, OpIterNext, streamElemSymbol.Index)
		}

		// condition jump position
		postCondPos := c.emit(node, OpJumpIfFalse, 0)

		// enter loop
		loop := c.enterLoop(itSymbol, ForLoop)

		// assign key variable
		if node.KeyIndexIdent != nil && node.KeyIndexIdent.Name != "_" {
			keySymbol := c.currentLocalSymbols().Define(node.KeyIndexIdent.Name)
			c.emit(node, OpGetLocal, itSymbol.Index)
			c.emit(node, OpIterKey)
			c.emit(node, OpSetLocal, keySymbol.Index)
		}

		// assign value variable
		if node.ValueElemIdent != nil && node.ValueElemIdent.Name != "_" {
			valueSymbol := c.currentLocalSymbols().Define(node.ValueElemIdent.Name)
			c.emit(node, OpGetLocal, itSymbol.Index)
			c.emit(node, OpIterValue, streamElemSymbol.Index)
			c.emit(node, OpSetLocal, valueSymbol.Index)
		}

		// body
		if err := c.Compile(node.Body); err != nil {
			c.leaveLoop()
			return err
		}

		c.leaveLoop()

		// post-body position
		postBodyPos := len(c.currentInstructions())

		// back to condition
		c.emit(node, OpJump, preCondPos)

		// post-statement position
		postStmtPos := len(c.currentInstructions())
		c.changeOperand(postCondPos, postStmtPos)

		// update all break/continue jump positions
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, postStmtPos)
		}
		for _, pos := range loop.continuePositions {
			c.changeOperand(pos, postBodyPos)
		}
	case *parse.BreakStatement:
		curLoop := c.currentLoop()
		if curLoop == nil {
			return c.NewError(node, "break not allowed outside loop")
		}
		pos := c.emit(node, OpJump, 0)
		curLoop.breakPositions = append(curLoop.breakPositions, pos)

	case *parse.ContinueStatement:
		curLoop := c.currentLoop()
		if curLoop == nil {
			return c.NewError(node, "continue not allowed outside loop")
		}
		pos := c.emit(node, OpJump, 0)
		curLoop.continuePositions = append(curLoop.continuePositions, pos)
	case *parse.PruneStatement:
		c.emit(node, OpIterPrune, c.currentWalkLoop().iteratorSymbol.Index)
	case *parse.WalkStatement:
		itSuffix := strconv.Itoa(int(node.Span.Start))
		itSymbol := c.currentLocalSymbols().Define(":it" + itSuffix)
		if err := c.Compile(node.Walked); err != nil {
			return err
		}
		c.emit(node, OpWalkerInit)
		c.emit(node, OpSetLocal, itSymbol.Index)

		// pre-condition position
		preCondPos := len(c.currentInstructions())

		// condition
		c.emit(node, OpGetLocal, itSymbol.Index)
		c.emit(node, OpIterNext, -1)

		// condition jump position
		postCondPos := c.emit(node, OpJumpIfFalse, 0)

		// enter loop
		loop := c.enterLoop(itSymbol, WalkLoop)

		// assign key variable
		if node.EntryIdent != nil && node.EntryIdent.Name != "_" {
			keySymbol := c.currentLocalSymbols().Define(node.EntryIdent.Name)
			c.emit(node, OpGetLocal, itSymbol.Index)
			c.emit(node, OpIterValue, -1)
			c.emit(node, OpSetLocal, keySymbol.Index)
		}

		// body statement
		if err := c.Compile(node.Body); err != nil {
			c.leaveLoop()
			return err
		}

		c.leaveLoop()

		// post-body position
		postBodyPos := len(c.currentInstructions())

		// back to condition
		c.emit(node, OpJump, preCondPos)

		// post-statement position
		postStmtPos := len(c.currentInstructions())
		c.changeOperand(postCondPos, postStmtPos)

		// update all break/continue jump positions
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, postStmtPos)
		}
		for _, pos := range loop.continuePositions {
			c.changeOperand(pos, postBodyPos)
		}
	case *parse.SwitchStatement:

		if len(node.Cases) == 0 {
			return nil
		}

		if err := c.Compile(node.Discriminant); err != nil {
			return err
		}

		//  jump placeholders
		var jumpAfterStmtPositions []int

		for i, case_ := range node.Cases {
			for j, valueNode := range case_.Values {
				if i != len(node.Cases)-1 || j != len(case_.Values)-1 {
					c.emit(node, OpCopyTop)
				}

				if err := c.Compile(valueNode); err != nil {
					return err
				}

				c.emit(node, OpEqual)

				// placeholder for jumping to next case
				jumpPos := c.emit(node, OpJumpIfFalse, 0)

				if err := c.Compile(case_.Block); err != nil {
					return err
				}

				jumpAfterStmtPositions = append(jumpAfterStmtPositions, c.emit(node, OpJump, 0))

				curPos := len(c.currentInstructions())
				c.changeOperand(jumpPos, curPos)
			}
		}

		if len(node.DefaultCases) > 0 {
			if err := c.Compile(node.DefaultCases[0].Block); err != nil {
				return err
			}
		}

		curPos := len(c.currentInstructions())
		for _, jump := range jumpAfterStmtPositions {
			c.changeOperand(jump, curPos)
		}
	case *parse.MatchStatement:

		if len(node.Cases) == 0 {
			return nil
		}

		if err := c.Compile(node.Discriminant); err != nil {
			return err
		}

		//  jump placeholders
		var jumpAfterStmtPositions []int

		for i, case_ := range node.Cases {
			for j, valueNode := range case_.Values {
				if i != len(node.Cases)-1 || j != len(case_.Values)-1 {
					c.emit(node, OpCopyTop)
				}

				if err := c.Compile(valueNode); err != nil {
					return err
				}

				if case_.GroupMatchingVariable != nil {
					variable := case_.GroupMatchingVariable.(*parse.IdentifierLiteral)
					s, exists := c.currentLocalSymbols().Resolve(variable.Name)
					if !exists {
						s = c.currentLocalSymbols().Define(variable.Name)
					}
					c.emit(node, OpGroupMatch, s.Index)
				} else {
					c.emit(node, OpMatch)
				}

				// placeholder for jumping to next case
				jumpPos := c.emit(node, OpJumpIfFalse, 0)

				if err := c.Compile(case_.Block); err != nil {
					return err
				}

				jumpAfterStmtPositions = append(jumpAfterStmtPositions, c.emit(node, OpJump, 0))

				curPos := len(c.currentInstructions())
				c.changeOperand(jumpPos, curPos)
			}
		}

		if len(node.DefaultCases) > 0 {
			if err := c.Compile(node.DefaultCases[0].Block); err != nil {
				return err
			}
		}

		afterStmtPos := len(c.currentInstructions())
		for _, jump := range jumpAfterStmtPositions {
			c.changeOperand(jump, afterStmtPos)
		}

	case *parse.Block:
		if len(node.Statements) == 0 {
			return nil
		}

		if len(node.Statements) > 1 {
			for _, stmt := range node.Statements {
				if err := c.Compile(stmt); err != nil {
					return err
				}
				if stmt.Kind() == parse.Expr {
					c.emit(node, OpPop)
				}
			}
		} else {
			if err := c.Compile(node.Statements[0]); err != nil {
				return err
			}
			if node.Statements[0].Kind() == parse.Expr {
				c.emit(node, OpPop)
			}
		}
	case *parse.SynchronizedBlockStatement:
		if len(node.Block.Statements) == 0 {
			return nil
		}

		if len(node.SynchronizedValues) > 255 {
			return c.NewError(node, "too many synchronized values")
		}

		for _, valNode := range node.SynchronizedValues {
			if err := c.Compile(valNode); err != nil {
				return err
			}
		}
		c.emit(node, OpBlockLock, len(node.SynchronizedValues))
		if err := c.Compile(node.Block); err != nil {
			return err
		}
		c.emit(node, OpBlockUnlock)
	case *parse.LocalVariableDeclarations:
		for _, decl := range node.Declarations {
			symbol := c.currentLocalSymbols().Define(decl.Left.(*parse.IdentifierLiteral).Name)
			if err := c.Compile(decl.Right); err != nil {
				return err
			}
			c.emit(node, OpSetLocal, symbol.Index)
		}
	case *parse.Assignment:
		err := c.compileAssign(node, node.Left, node.Right)
		if err != nil {
			return err
		}
	case *parse.MultiAssignment:
		if err := c.Compile(node.Right); err != nil {
			return err
		}

		for i := 0; i < len(node.Variables)-1; i++ {
			c.emit(node, OpCopyTop)
		}

		for i, variable := range node.Variables {
			name := variable.(*parse.IdentifierLiteral).Name
			symbol, exists := c.currentLocalSymbols().Resolve(name)
			if !exists {
				symbol = c.currentLocalSymbols().Define(name)
			}
			c.emit(node, OpPushConstant, c.addConstant(Int(i)))
			if node.Nillable {
				c.emit(node, OpSafeAt)
			} else {
				c.emit(node, OpAt)
			}
			c.emit(node, OpSetLocal, symbol.Index)
		}
	case *parse.IdentifierLiteral:
		if err := c.CompileVar(node); err != nil {
			return err
		}
	case *parse.GlobalVariable:
		_, ok := c.globalSymbols.Resolve(node.Name)
		if !ok {
			return c.NewError(node, fmt.Sprintf("unresolved global reference '%s'", node.Name))
		}

		c.emit(node, OpGetGlobal, c.addConstant(Str(node.Name)))
	case *parse.Variable:
		_, ok := c.globalSymbols.Resolve(node.Name)
		if ok {
			c.emit(node, OpGetGlobal, c.addConstant(Str(node.Name)))
		} else {
			s, ok := c.currentLocalSymbols().Resolve(node.Name)
			if !ok {
				return c.NewError(node, fmt.Sprintf("unresolved local reference '%s'", node.Name))
			}

			c.emit(node, OpGetLocal, s.Index)
		}

	case *parse.ListLiteral:

		spread := false
		lastAppendedCount := 0 //only use if there are spread elements

		for i, elem := range node.Elements {
			if spreadElem, ok := elem.(*parse.ElementSpreadElement); ok {

				if spread && lastAppendedCount != 0 {
					c.emit(node, OpAppend, lastAppendedCount)
					lastAppendedCount = 0
				}

				if !spread {
					c.emit(node, OpCreateList, i)
					spread = true
				}

				if err := c.Compile(spreadElem.Expr); err != nil {
					return err
				}
				c.emit(node, OpSpreadList)

			} else {
				if spread {
					lastAppendedCount++
				}
				if err := c.Compile(elem); err != nil {
					return err
				}
			}

		}

		if spread {
			if lastAppendedCount != 0 {
				c.emit(node, OpAppend, lastAppendedCount)
			}
		} else {
			c.emit(node, OpCreateList, len(node.Elements))
		}
	case *parse.TupleLiteral:

		spread := false
		lastAppendedCount := 0 //only use if there are spread elements

		for i, elem := range node.Elements {
			if spreadElem, ok := elem.(*parse.ElementSpreadElement); ok {

				if spread && lastAppendedCount != 0 {
					c.emit(node, OpAppend, lastAppendedCount)
					lastAppendedCount = 0
				}

				if !spread {
					c.emit(node, OpCreateTuple, i)
					spread = true
				}

				if err := c.Compile(spreadElem.Expr); err != nil {
					return err
				}
				c.emit(node, OpSpreadTuple)

			} else {
				if spread {
					lastAppendedCount++
				}
				if err := c.Compile(elem); err != nil {
					return err
				}
			}

		}

		if spread {
			if lastAppendedCount != 0 {
				c.emit(node, OpAppend, lastAppendedCount)
			}
		} else {
			c.emit(node, OpCreateTuple, len(node.Elements))
		}
	case *parse.ObjectLiteral:
		var key string
		indexKey := 0

		for _, prop := range node.Properties {
			switch n := prop.Key.(type) {
			case *parse.QuotedStringLiteral:
				key = n.Value
				_, err := strconv.ParseUint(key, 10, 32)
				if err == nil {
					indexKey++
				}
			case *parse.IdentifierLiteral:
				key = n.Name
			case nil:
				key = strconv.Itoa(indexKey)
				indexKey++
			default:
				return fmt.Errorf("invalid key type %T", n)
			}

			c.emit(node, OpPushConstant, c.addConstant(Str(key)))

			// value
			if err := c.Compile(prop.Value); err != nil {
				return err
			}
		}

		propCount := len(node.Properties)

		c.emit(node, OpCreateObject, 2*propCount, indexKey, c.addConstant(AstNode{
			Node:  node,
			chunk: c.currentChunk(),
		}))

		for _, el := range node.SpreadElements {
			if err := c.Compile(el.Expr); err != nil {
				return err
			}

			c.emit(node, OpSpreadObject)
		}
	case *parse.RecordLiteral:
		var key string
		indexKey := 0

		for _, prop := range node.Properties {
			switch n := prop.Key.(type) {
			case *parse.QuotedStringLiteral:
				key = n.Value
				_, err := strconv.ParseUint(key, 10, 32)
				if err == nil {
					indexKey++
				}
			case *parse.IdentifierLiteral:
				key = n.Name
			case nil:
				key = strconv.Itoa(indexKey)
				indexKey++
			default:
				return fmt.Errorf("invalid key type %T", n)
			}

			c.emit(node, OpPushConstant, c.addConstant(Str(key)))

			// value
			if err := c.Compile(prop.Value); err != nil {
				return err
			}
		}

		propCount := len(node.Properties)
		c.emit(node, OpCreateRecord, indexKey, 2*propCount)

		if len(node.SpreadElements) > 0 {
			return errors.New("cannot compile spread elements in records: not implemented")
		}

	case *parse.ExtractionExpression:
		if err := c.Compile(node.Object); err != nil {
			return err
		}
		keys := KeyList{}
		for _, e := range node.Keys.Keys {
			keys = append(keys, e.(*parse.IdentifierLiteral).Name)
		}
		c.emit(node, OpExtractProps, c.addConstant(keys))
	case *parse.KeyListExpression:
		for _, elem := range node.Keys {
			if ambiguousIdent, ok := elem.(*parse.IdentifierLiteral); ok {
				c.emit(node, OpPushConstant, c.addConstant(Identifier(ambiguousIdent.Name)))
			} else if err := c.Compile(elem); err != nil {
				return err
			}
		}
		c.emit(node, OpCreateKeyList, len(node.Keys))
	case *parse.DictionaryLiteral:
		for _, entry := range node.Entries {

			if lit, ok := entry.Key.(parse.SimpleValueLiteral); ok && !parse.NodeIs(lit, &parse.IdentifierLiteral{}) {
				key := utils.Must(evalSimpleValueLiteral(lit, &GlobalState{}))
				c.emit(node, OpPushConstant, c.addConstant(key))
			} else {
				if err := c.Compile(entry.Key); err != nil {
					return err
				}
			}

			// value
			if err := c.Compile(entry.Value); err != nil {
				return err
			}
		}
		c.emit(node, OpCreateDict, len(node.Entries)*2)
	case *parse.IdentifierMemberExpression:
		symbol, ok := c.globalSymbols.Resolve(node.Left.Name)
		isGlobal := true
		if !ok {
			isGlobal = false
			symbol, ok = c.currentLocalSymbols().Resolve(node.Left.Name)
		}
		if !ok {
			return c.NewError(node, fmt.Sprintf("unresolved reference '%s'", node.Left.Name))
		}

		if isGlobal {
			c.emit(node, OpGetGlobal, c.addConstant(Str(node.Left.Name)))
		} else {
			c.emit(node, OpGetLocal, symbol.Index)
		}

		for _, p := range node.PropertyNames {
			c.emit(node, OpMemb, c.addConstant(Str(p.Name)))
		}

	case *parse.SelfExpression:
		c.emit(node, OpGetSelf)
	case *parse.MemberExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}

		op := OpMemb
		if node.Optional {
			op = OpOptionalMemb
		}

		c.emit(node, op, c.addConstant(Str(node.PropertyName.Name)))
	case *parse.DoubleColonExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}

		c.emit(node, OpDoubleColonResolve, c.addConstant(Str(node.Element.Name)))
	case *parse.ComputedMemberExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}

		if err := c.Compile(node.PropertyName); err != nil {
			return err
		}

		op := OpComputedMemb
		if node.Optional {
			return errors.New("optional computed member expressions are not supported yet")
		}

		c.emit(node, op)
	case *parse.DynamicMemberExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		c.emit(node, OpDynMemb, c.addConstant(Str(node.PropertyName.Name)))
	case *parse.IndexExpression:
		if err := c.Compile(node.Indexed); err != nil {
			return err
		}
		if err := c.Compile(node.Index); err != nil {
			return err
		}
		c.emit(node, OpAt)
	case *parse.SliceExpression:
		if err := c.Compile(node.Indexed); err != nil {
			return err
		}
		if node.StartIndex != nil {
			if err := c.Compile(node.StartIndex); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}
		if node.EndIndex != nil {
			if err := c.Compile(node.EndIndex); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}
		c.emit(node, OpSlice)
	case *parse.HostAliasDefinition:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		c.emit(node, OpAddHostAlias, c.addConstant(Str(node.Left.Value)))
	case *parse.ListPatternLiteral:
		hasGeneralElem := 0

		if node.GeneralElement != nil {
			hasGeneralElem = 1

			if err := c.Compile(node.GeneralElement); err != nil {
				return err
			}
		} else {
			for _, e := range node.Elements {
				if err := c.Compile(e); err != nil {
					return err
				}
				c.emit(e, OpToPattern)
			}
		}

		c.emit(node, OpCreateListPattern, len(node.Elements), hasGeneralElem)
	case *parse.TuplePatternLiteral:
		hasGeneralElem := 0

		if node.GeneralElement != nil {
			hasGeneralElem = 1

			if err := c.Compile(node.GeneralElement); err != nil {
				return err
			}
		} else {
			for _, e := range node.Elements {
				if err := c.Compile(e); err != nil {
					return err
				}
				c.emit(e, OpToPattern)
			}
		}

		c.emit(node, OpCreateTuplePattern, len(node.Elements), hasGeneralElem)
	case *parse.ObjectPatternLiteral:
		isInexact := 1

		if node.Exact {
			isInexact = 0
		}

		for _, p := range node.Properties {
			name := p.Name()

			c.emit(node, OpPushConstant, c.addConstant(Str(name)))

			if err := c.Compile(p.Value); err != nil {
				return err
			}
			c.emit(p.Value, OpToPattern)

			if p.Optional {
				c.emit(node, OpPushTrue)
			} else {
				c.emit(node, OpPushFalse)
			}
		}

		c.emit(node, OpCreateObjectPattern, 3*len(node.Properties), isInexact)

		for _, e := range node.SpreadElements {
			if err := c.Compile(e.Expr); err != nil {
				return err
			}
			c.emit(node, OpSpreadObjectPattern)
		}

	case *parse.RecordPatternLiteral:
		isInexact := 1

		if node.Exact {
			isInexact = 0
		}

		for _, p := range node.Properties {
			name := p.Name()

			c.emit(node, OpPushConstant, c.addConstant(Str(name)))

			if err := c.Compile(p.Value); err != nil {
				return err
			}
			c.emit(p.Value, OpToPattern)

			if p.Optional {
				c.emit(node, OpPushTrue)
			} else {
				c.emit(node, OpPushFalse)
			}
		}

		c.emit(node, OpCreateRecordPattern, 3*len(node.Properties), isInexact)

		for _, e := range node.SpreadElements {
			if err := c.Compile(e.Expr); err != nil {
				return err
			}
			c.emit(node, OpSpreadRecordPattern)
		}

	case *parse.OptionPatternLiteral:
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(node.Value, OpToPattern)

		c.emit(node, OpCreateOptionPattern, c.addConstant(Str(node.Name)))
	case *parse.PatternUnion:
		for _, e := range node.Cases {
			if err := c.Compile(e); err != nil {
				return err
			}
			c.emit(e, OpToPattern)
		}

		c.emit(node, OpCreateUnionPattern, len(node.Cases))
	case *parse.PatternIdentifierLiteral:
		c.emit(node, OpResolvePattern, c.addConstant(Str(node.Name)))
	case *parse.OptionalPatternExpression:
		if err := c.Compile(node.Pattern); err != nil {
			return err
		}
		c.emit(node, OpCreateOptionalPattern)
	case *parse.ComplexStringPatternPiece:
		return c.CompileStringPatternNode(node)
	case *parse.PatternDefinition:
		if node.IsLazy {
			if err := c.CompileStringPatternNode(node.Right); err != nil {
				return err
			}
		} else {
			if err := c.Compile(node.Right); err != nil {
				return err
			}
		}
		c.emit(node, OpToPattern)
		c.emit(node, OpAddPattern, c.addConstant(Str(node.Left.Name)))
	case *parse.PatternNamespaceIdentifierLiteral:
		c.emit(node, OpResolvePatternNamespace, c.addConstant(Str(node.Name)))
	case *parse.PatternNamespaceDefinition:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		c.emit(node, OpCreatePatternNamespace)
		c.emit(node, OpAddPatternNamespace, c.addConstant(Str(node.Left.Name)))
	case *parse.PatternNamespaceMemberExpression:
		c.emit(node, OpPatternNamespaceMemb, c.addConstant(Str(node.Namespace.Name)), c.addConstant(Str(node.MemberName.Name)))
	case *parse.FunctionDeclaration:
		_, exists := c.globalSymbols.Resolve(node.Name.Name)

		if exists {
			return c.NewError(node, fmt.Sprintf("function %s already declared", node.Name.Name))
		}

		s := c.globalSymbols.Define(node.Name.Name)
		s.IsConstant = true

		if err := c.Compile(node.Function); err != nil {
			return err
		}

		c.emit(node, OpSetGlobal, c.addConstant(Str(node.Name.Name)))
	case *parse.FunctionExpression:
		//enter local scope
		scope := compilationScope{
			sourceMap: make(map[int]instructionSourcePosition),
		}
		c.scopes = append(c.scopes, scope)
		c.localSymbolTableStack = append(c.localSymbolTableStack, newSymbolTable())
		c.scopeIndex++
		if c.trace != nil {
			c.printTrace("ENTER SCOPE", c.scopeIndex)
		}

		currentLocals := c.currentLocalSymbols()

		//define parameters
		for _, p := range node.Parameters {
			currentLocals.Define(p.Var.Name)
		}

		//define captured locals
		for _, e := range node.CaptureList {
			name := e.(*parse.IdentifierLiteral).Name
			currentLocals.Define(name)
		}

		if node.IsBodyExpression {
			if err := c.Compile(node.Body); err != nil {
				return err
			}

			c.emit(node, OpReturn, 1)
		} else { //block
			if err := c.Compile(node.Body); err != nil {
				return err
			}

			instructions := c.currentInstructions()
			if len(instructions) <= 1 || c.lastOp != OpReturn {
				c.emit(node, OpReturn, 0)
			}
		}

		//leave local scope
		localCount := c.currentLocalSymbols().SymbolCount()
		instructions := c.currentInstructions()

		sourceMap := c.currentSourceMap()
		c.scopes = c.scopes[:len(c.scopes)-1]
		c.localSymbolTableStack = c.localSymbolTableStack[:len(c.localSymbolTableStack)-1]
		c.scopeIndex--
		if c.trace != nil {
			c.printTrace("LEAVE SCOPE", c.scopeIndex)
		}

		compiledFunction := &CompiledFunction{
			Instructions: instructions,
			LocalCount:   localCount,
			ParamCount:   len(node.Parameters),
			IsVariadic:   node.IsVariadic,
			SourceMap:    sourceMap,
		}

		var symbolicInoxFunc *symbolic.InoxFunction
		{
			value, ok := c.symbolicData.GetMostSpecificNodeValue(node)
			if ok {
				symbolicInoxFunc, ok = value.(*symbolic.InoxFunction)
				if !ok {
					return c.NewError(node, fmt.Sprintf("invalid type for symbolic value of function expression: %T", value))
				}
			}
		}

		var staticData *FunctionStaticData
		if c.staticCheckData != nil {
			staticData = c.staticCheckData.GetFnData(node)
		}

		c.emit(node, OpPushConstant, c.addConstant(&InoxFunction{
			Node:             node,
			compiledFunction: compiledFunction,
			symbolicValue:    symbolicInoxFunc,
			staticData:       staticData,
		}))

		//if they are captured locals we create (at runtime) a new InoxFunction with the captured locals
		if len(node.CaptureList) != 0 {

			if len(node.CaptureList) > 255 {
				return c.NewError(node, "too many captured locals")
			}

			locals := c.currentLocalSymbols()
			for _, e := range node.CaptureList {
				s, _ := locals.Resolve(e.(*parse.IdentifierLiteral).Name)
				c.emit(node, OpGetLocal, s.Index)
			}

			c.emit(node, BindCapturedLocals, len(node.CaptureList))
		}

	case *parse.FunctionPatternExpression:
		symbolicData, ok := c.symbolicData.GetMostSpecificNodeValue(node)
		var symbFnPattern *symbolic.FunctionPattern
		if ok {
			symbFnPattern, ok = symbolicData.(*symbolic.FunctionPattern)
			if !ok {
				return c.NewError(node, fmt.Sprintf("invalid type for symbolic value of function pattern expression: %T", symbolicData))
			}
		}

		c.emit(node, OpPushConstant, c.addConstant(&FunctionPattern{
			node:          node,
			symbolicValue: symbFnPattern,
		}))
	case *parse.PatternConversionExpression:
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(node, OpToPattern)
	case *parse.LazyExpression:
		c.emit(node, OpPushConstant, c.addConstant(AstNode{
			Node:  node.Expression,
			chunk: c.currentChunk(),
		}))
	case *parse.ReturnStatement:
		if node.Expr == nil {
			c.emit(node, OpReturn, 0)
		} else {
			if err := c.Compile(node.Expr); err != nil {
				return err
			}
			c.emit(node, OpReturn, 1)
		}
	case *parse.YieldStatement:
		if node.Expr == nil {
			c.emit(node, OpYield, 0)
		} else {
			if err := c.Compile(node.Expr); err != nil {
				return err
			}
			c.emit(node, OpYield, 1)
		}
	case *parse.CallExpression:
		c.emit(node, OpPushNil)

		spread := 0
		for _, arg := range node.Arguments {
			switch a := arg.(type) {
			case *parse.SpreadArgument:
				if spread == 1 {
					return errors.New("single argument spread is supported")
				}
				spread = 1
				if err := c.Compile(a.Expr); err != nil {
					return err
				}
			case *parse.IdentifierLiteral:
				if node.CommandLikeSyntax {
					c.emit(a, OpPushConstant, c.addConstant(Identifier(a.Name)))
				} else {
					if err := c.Compile(arg); err != nil {
						return err
					}
				}
			default:
				if err := c.Compile(arg); err != nil {
					return err
				}
			}
		}

		var must = 0
		if node.Must {
			must = 1
		}

		//compiles callee
		switch callee := node.Callee.(type) {
		case *parse.IdentifierMemberExpression:
			symbol, ok := c.globalSymbols.Resolve(callee.Left.Name)
			isGlobal := true
			if !ok {
				isGlobal = false
				symbol, ok = c.currentLocalSymbols().Resolve(callee.Left.Name)
			}
			if !ok {
				return c.NewError(callee, fmt.Sprintf("unresolved reference '%s'", callee.Left.Name))
			}

			if isGlobal {
				c.emit(callee, OpGetGlobal, c.addConstant(Str(callee.Left.Name)))
			} else {
				c.emit(callee, OpGetLocal, symbol.Index)
			}

			for _, p := range callee.PropertyNames[:len(callee.PropertyNames)-1] {
				c.emit(callee, OpMemb, c.addConstant(Str(p.Name)))
			}

			c.emit(callee, OpCopyTop)
			c.emit(callee, OpMemb, c.addConstant(Str(callee.PropertyNames[len(callee.PropertyNames)-1].Name)))
		case *parse.MemberExpression:
			if err := c.Compile(callee.Left); err != nil {
				return err
			}
			c.emit(callee, OpCopyTop)
			c.emit(callee, OpMemb, c.addConstant(Str(callee.PropertyName.Name)))
		case *parse.DoubleColonExpression:
			if err := c.Compile(callee.Left); err != nil {
				return err
			}
			c.emit(callee, OpCopyTop)
			c.emit(callee, OpDoubleColonResolve, c.addConstant(Str(callee.Element.Name)))
		default:
			c.emit(callee, OpPushNil) //no self
			if err := c.Compile(callee); err != nil {
				return err
			}
		}

		c.emit(node, OpCall, len(node.Arguments)-spread, spread, must)
	case *parse.PatternCallExpression:
		if err := c.Compile(node.Callee); err != nil {
			return err
		}

		if len(node.Arguments) > 255 {
			return c.NewError(node, "too many arguments")
		}

		for _, arg := range node.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}

		c.emit(node, OpCallPattern, len(node.Arguments))
	case *parse.PipelineStatement, *parse.PipelineExpression:
		var stages []*parse.PipelineStage
		isStmt := false

		switch e := node.(type) {
		case *parse.PipelineStatement:
			stages = e.Stages
			isStmt = true
		case *parse.PipelineExpression:
			stages = e.Stages
		}

		anon, exists := c.currentLocalSymbols().Resolve("")
		if exists {
			c.emit(node, OpGetLocal, anon.Index)
		} else {
			anon = c.currentLocalSymbols().Define("")
		}

		for _, stage := range stages {
			if err := c.Compile(stage.Expr); err != nil {
				return err
			}

			c.emit(stage.Expr, OpSetLocal, anon.Index)
		}

		//unlike the tree-walking interpreter we push the value only for pipeline expressions
		if !isStmt {
			c.emit(node, OpGetLocal, anon.Index)
		}

		if exists {
			//the original value for $ should be on the top of the stack
			c.emit(node, OpSetLocal, anon.Index)
		}

	case *parse.PermissionDroppingStatement:
		if err := c.Compile(node.Object); err != nil {
			return err
		}
		c.emit(node, OpDropPerms)
	case *parse.InclusionImportStatement:
		if c.module == nil {
			panic(fmt.Errorf("cannot compiule inclusion import statement: provided module is nil"))
		}
		chunk := c.module.InclusionStatementMap[node]
		c.chunkStack = append(c.chunkStack, chunk.ParsedChunk)
		defer func() {
			c.chunkStack = c.chunkStack[:len(c.chunkStack)-1]
		}()

		if c.trace != nil {
			c.printTrace(fmt.Sprintf("ENTER INCLUDED CHUNK  %s", chunk.Name()))
		}

		//compile constants
		if chunk.Node.GlobalConstantDeclarations != nil {
			if err := c.Compile(chunk.Node.GlobalConstantDeclarations); err != nil {
				return err
			}
		}

		//compile statements
		for _, stmt := range chunk.Node.Statements {
			if err := c.Compile(stmt); err != nil {
				return err
			}
			if stmt.Kind() == parse.Expr {
				c.emit(node, OpPop)
			}
		}

		if c.trace != nil {
			c.printTrace(fmt.Sprintf("LEAVE INCLUDED CHUNK  %s", chunk.Name()))
		}
	case *parse.ImportStatement:
		c.globalSymbols.Define(node.Identifier.Name)

		if err := c.Compile(node.Source); err != nil {
			return err
		}

		if err := c.Compile(node.Configuration); err != nil {
			return err
		}

		c.emit(node, OpImport, c.addConstant(Str(node.Identifier.Name)))
	case *parse.SpawnExpression:
		if node.Meta != nil {
			objLit := node.Meta.(*parse.ObjectLiteral)
			//we handle this case separately because objects cannot contain non-serializable values.

			var keys []string
			var types []Pattern

			for _, property := range objLit.Properties {
				propertyName := property.Name() //okay since implicit-key properties are not allowed
				keys = append(keys, propertyName)
				types = append(types, ANYVAL_PATTERN)

				if propertyName != symbolic.LTHREAD_META_GLOBALS_SECTION || !parse.NodeIs(property.Value, (*parse.ObjectLiteral)(nil)) {
					if err := c.Compile(property.Value); err != nil {
						return err
					}
				} else {
					//handle description separately if it's an object literal because non-serializable value are not accepted.
					globalsLit := property.Value.(*parse.ObjectLiteral)

					var globalNames []string
					var globalTypes []Pattern

					for _, prop := range globalsLit.Properties {
						globalName := prop.Name() //okay since implicit-key properties are not allowed
						globalNames = append(globalNames, globalName)
						globalTypes = append(types, ANYVAL_PATTERN)

						if err := c.Compile(prop.Value); err != nil {
							return err
						}
					}

					anonGlobalsStructType := NewStructPattern("", ulid.Make(), globalNames, globalTypes)
					globalCount := len(globalsLit.Properties)
					c.emit(globalsLit, OpCreateStruct, c.addConstant(anonGlobalsStructType), globalCount)
				}
			}

			anonStructType := NewStructPattern("", ulid.Make(), keys, types)
			propCount := len(objLit.Properties)

			c.emit(node.Meta, OpCreateStruct, c.addConstant(anonStructType), propCount)
		} else {
			c.emit(node, OpPushNil)
		}

		routineChunk := node.Module.ToChunk()

		routineMod := &Module{
			MainChunk:  parse.NewParsedChunk(routineChunk, c.module.MainChunk.Source),
			ModuleKind: UserLThreadModule,
		}

		embeddedModCompiler := NewCompiler(routineMod, map[string]Value{}, c.symbolicData, c.staticCheckData, c.context, c.trace)
		isSingleExpr := 0
		calleeName := ""

		for _, name := range c.globalSymbols.SymbolNames() {
			embeddedModCompiler.globalSymbols.Define(name)
		}

		if node.Module.SingleCallExpr {
			isSingleExpr = 1
			callee := node.Module.Statements[0].(*parse.CallExpression).Callee
			calleeName = callee.(*parse.IdentifierLiteral).Name
			embeddedModCompiler.globalSymbols.Define(calleeName)

			if err := c.Compile(callee); err != nil {
				return err
			}

		} else {
			c.emit(node, OpPushNil)
		}

		var globalDescNode parse.Node

		if obj, ok := node.Meta.(*parse.ObjectLiteral); ok {
			val, ok := obj.PropValue(symbolic.LTHREAD_META_GLOBALS_SECTION)
			if ok {
				globalDescNode = val
			}
		}

		switch g := globalDescNode.(type) {
		case *parse.KeyListExpression:
			for _, key := range g.Keys {
				embeddedModCompiler.globalSymbols.Define(key.(*parse.IdentifierLiteral).Name)
			}
		case *parse.ObjectLiteral:
			for _, prop := range g.Properties {
				embeddedModCompiler.globalSymbols.Define(prop.Name())
			}
		}

		_, err := embeddedModCompiler.compileMainChunk(routineMod.MainChunk)
		if err != nil {
			return err
		}

		c.emit(node, OpSpawnLThread, isSingleExpr, c.addConstant(Str(calleeName)), c.addConstant(routineMod))
	case *parse.LifetimejobExpression:
		if err := c.Compile(node.Meta); err != nil {
			return err
		}

		if node.Subject != nil {
			if err := c.Compile(node.Subject); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}

		jobChunk := node.Module.ToChunk()

		jobMod := &Module{
			ModuleKind:       LifetimeJobModule,
			MainChunk:        parse.NewParsedChunk(jobChunk, c.module.MainChunk.Source),
			ManifestTemplate: node.Module.Manifest,
		}

		embeddedModCompiler := NewCompiler(jobMod, map[string]Value{}, c.symbolicData, c.staticCheckData, c.context, c.trace)
		for _, name := range c.globalSymbols.SymbolNames() {
			embeddedModCompiler.globalSymbols.Define(name)
		}

		_, err := embeddedModCompiler.compileMainChunk(jobMod.MainChunk)
		if err != nil {
			return err
		}

		c.emit(node, OpCreateLifetimeJob, c.addConstant(jobMod))
	case *parse.ReceptionHandlerExpression:
		if err := c.Compile(node.Pattern); err != nil {
			return err
		}
		if err := c.Compile(node.Handler); err != nil {
			return err
		}
		c.emit(node, OpCreateReceptionHandler)
	case *parse.SendValueExpression:
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		if err := c.Compile(node.Receiver); err != nil {
			return err
		}
		c.emit(node, OpSendValue)
	case *parse.MappingExpression:
		c.emit(node, OpCreateMapping, c.addConstant(AstNode{
			Node:  node,
			chunk: c.currentChunk(),
		}))
	case *parse.UDataLiteral:

		if err := c.Compile(node.Root); err != nil {
			return err
		}

		for _, entry := range node.Children {
			if err := c.Compile(entry); err != nil {
				return err
			}
		}

		c.emit(node, OpCreateUData, len(node.Children))
	case *parse.UDataEntry:
		if err := c.Compile(node.Value); err != nil {
			return err
		}

		for _, entry := range node.Children {
			if err := c.Compile(entry); err != nil {
				return err
			}
		}

		c.emit(node, OpCreateUdataHiearchyEntry, len(node.Children))

	case *parse.ConcatenationExpression:
		spreadElemSet := make([]Bool, len(node.Elements))

		for i, elemNode := range node.Elements {
			spreadNode, isSpread := elemNode.(*parse.ElementSpreadElement)
			spreadElemSet[i] = Bool(isSpread)

			if isSpread {
				if err := c.Compile(spreadNode.Expr); err != nil {
					return err
				}
			} else if err := c.Compile(elemNode); err != nil {
				return err
			}
		}
		if len(node.Elements) >= (1 << 8) {
			return fmt.Errorf("too many elements in concatenation")
		}
		c.emit(node, OpConcat, len(node.Elements), c.addConstant(NewWrappedBoolList(spreadElemSet...)))
	case *parse.AssertionStatement:
		if err := c.Compile(node.Expr); err != nil {
			return err
		}
		c.emit(node, OpAssert, c.addConstant(AstNode{Node: node}))
		//TODO: support intermediary values
	case *parse.RuntimeTypeCheckExpression:
		if err := c.Compile(node.Expr); err != nil {
			return err
		}
		constantVal := AstNode{Node: node, chunk: c.currentChunk()}
		c.emit(node, OpRuntimeTypecheck, c.addConstant(constantVal))
	case *parse.TestSuiteExpression:
		if node.IsStatement {
			c.emit(node, OpPushNil)
		}

		if node.Meta != nil {
			if err := c.Compile(node.Meta); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}

		c.emit(node, OpCreateTestSuite, c.addConstant(AstNode{
			Node:  node.Module.ToChunk(),
			chunk: c.currentChunk(),
		}))

		if node.IsStatement {
			c.emit(node, OpCopyTop) //self
			c.emit(node, OpMemb, c.addConstant(Str("run")))
			c.emit(node, OpCall, 0, 0, 1)
			c.emit(node, OpPushNil)
			c.emit(node, OpSwap)
			c.emit(node, OpCopyTop) //self
			c.emit(node, OpMemb, c.addConstant(Str("wait_result")))
			c.emit(node, OpCall, 0, 0, 1)
		}
	case *parse.TestCaseExpression:
		if node.IsStatement {
			c.emit(node, OpPushNil)
		}

		if node.Meta != nil {
			if err := c.Compile(node.Meta); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}

		c.emit(node, OpCreateTestCase, c.addConstant(AstNode{
			Node:  node.Module.ToChunk(),
			chunk: c.currentChunk(),
		}))

		if node.IsStatement {
			c.emit(node, OpCopyTop) //self
			c.emit(node, OpMemb, c.addConstant(Str("run")))
			c.emit(node, OpCall, 0, 0, 1)
			c.emit(node, OpPushNil)
			c.emit(node, OpSwap)
			c.emit(node, OpCopyTop) //self
			c.emit(node, OpMemb, c.addConstant(Str("wait_result")))
			c.emit(node, OpCall, 0, 0, 1)
		}

	case *parse.StringTemplateLiteral:
		for _, slice := range node.Slices {
			switch s := slice.(type) {
			case *parse.StringTemplateSlice:
				c.emit(node, OpPushConstant, c.addConstant(Str(s.Value)))
			case *parse.StringTemplateInterpolation:
				if err := c.Compile(s.Expr); err != nil {
					return err
				}
			}
		}

		typed := 0
		if node.Pattern != nil {
			typed = 1
		}

		c.emit(node, OpCreateString, typed, len(node.Slices), c.addConstant(AstNode{
			Node:  node,
			chunk: c.currentChunk(),
		}))
	case *parse.BooleanConversionExpression:
		if err := c.Compile(node.Expr); err != nil {
			return err
		}

		c.emit(node, OpToBool)
	case *parse.XMLExpression:
		if err := c.Compile(node.Element); err != nil {
			return err
		}

		if err := c.Compile(node.Namespace); err != nil {
			return err
		}

		c.emit(node, OpCallFromXMLFactory)
	case *parse.XMLElement:
		name := node.Opening.GetName()

		for _, attr := range node.Opening.Attributes {
			c.emit(node, OpPushConstant, c.addConstant(Str(attr.GetName())))

			if attr.Value == nil {
				c.emit(node, OpPushConstant, c.addConstant(DEFAULT_XML_ATTR_VALUE))
			} else if err := c.Compile(attr.Value); err != nil {
				return err
			}
		}

		for _, child := range node.Children {
			if err := c.Compile(child); err != nil {
				return err
			}
		}

		c.emit(node, OpCreateXMLelem, c.addConstant(Str(name)), len(node.Opening.Attributes), len(node.Children))
	case *parse.XMLInterpolation:
		if err := c.Compile(node.Expr); err != nil {
			return err
		}
	case *parse.XMLText:
		str := Str(html.EscapeString(node.Value))
		c.emit(node, OpPushConstant, c.addConstant(str))
	default:
		return fmt.Errorf("cannot compile %T", node)
	}
	return nil
}

func (c *compiler) compileAssignOperation(node *parse.Assignment, rhs parse.Node) error {

	switch node.Operator {
	case parse.Assign:
		if err := c.Compile(rhs); err != nil {
			return err
		}
	case parse.PlusAssign:
		if err := c.Compile(rhs); err != nil {
			return err
		}
		c.emit(node, OpIntBin, int(parse.Add))
	case parse.MinusAssign:
		if err := c.Compile(rhs); err != nil {
			return err
		}
		c.emit(node, OpIntBin, int(parse.Sub))
	case parse.MulAssign:
		if err := c.Compile(rhs); err != nil {
			return err
		}
		c.emit(node, OpIntBin, int(parse.Mul))
	case parse.DivAssign:
		if err := c.Compile(rhs); err != nil {
			return err
		}
		c.emit(node, OpIntBin, int(parse.Div))
	}

	return nil
}

func (c *compiler) compileAssign(node *parse.Assignment, lhs, rhs parse.Node) error {

	switch l := lhs.(type) {
	case *parse.GlobalVariable, *parse.Variable, *parse.IdentifierLiteral:
		var varname = parse.GetVariableName(lhs)
		_, isGlobalVar := lhs.(*parse.GlobalVariable)

		var symbol *symbol
		var exists bool

		if isGlobalVar {
			symbol, exists = c.globalSymbols.Resolve(varname)
		} else {
			symbol, exists = c.currentLocalSymbols().Resolve(varname)
		}

		if !exists {
			if isGlobalVar {
				symbol = c.globalSymbols.Define(varname)
			} else {
				symbol = c.currentLocalSymbols().Define(varname)
			}
		}

		if node.Operator != parse.Assign {
			if isGlobalVar {
				c.emit(node, OpGetGlobal, c.addConstant(Str(varname)))
			} else {
				c.emit(node, OpGetLocal, symbol.Index)
			}
		}

		if err := c.compileAssignOperation(node, rhs); err != nil {
			return err
		}

		if isGlobalVar {
			c.emit(node, OpSetGlobal, c.addConstant(Str(varname)))
		} else {
			c.emit(node, OpSetLocal, symbol.Index)
		}

	case *parse.IdentifierMemberExpression:
		symbol, ok := c.globalSymbols.Resolve(l.Left.Name)
		isGlobal := true
		if !ok {
			isGlobal = false
			symbol, ok = c.currentLocalSymbols().Resolve(l.Left.Name)
		}
		if !ok {
			return c.NewError(node, fmt.Sprintf("unresolved reference '%s'", l.Left.Name))
		}

		if isGlobal {
			c.emit(node, OpGetGlobal, c.addConstant(Str(l.Left.Name)))
		} else {
			c.emit(node, OpGetLocal, symbol.Index)
		}

		for _, p := range l.PropertyNames[:len(l.PropertyNames)-1] {
			c.emit(node, OpMemb, c.addConstant(Str(p.Name)))
		}

		lastPropName := l.PropertyNames[len(l.PropertyNames)-1].Name

		if node.Operator != parse.Assign {
			c.emit(node, OpCopyTop)
			c.emit(node, OpMemb, c.addConstant(Str(lastPropName)))
		}

		if err := c.compileAssignOperation(node, rhs); err != nil {
			return err
		}

		c.emit(node, OpSetMember, c.addConstant(Str(lastPropName)))
	case *parse.MemberExpression:
		if err := c.Compile(l.Left); err != nil {
			return err
		}

		if node.Operator != parse.Assign {
			c.emit(node, OpCopyTop)
			c.emit(node, OpMemb, c.addConstant(Str(l.PropertyName.Name)))
		}

		if err := c.compileAssignOperation(node, rhs); err != nil {
			return err
		}

		c.emit(node, OpSetMember, c.addConstant(Str(l.PropertyName.Name)))
	case *parse.IndexExpression:
		if err := c.Compile(l.Indexed); err != nil {
			return err
		}

		if err := c.Compile(l.Index); err != nil {
			return err
		}

		if node.Operator != parse.Assign {
			if err := c.Compile(l.Indexed); err != nil {
				return err
			}

			if err := c.Compile(l.Index); err != nil {
				return err
			}
			c.emit(node, OpAt)
		}

		if err := c.compileAssignOperation(node, rhs); err != nil {
			return err
		}

		c.emit(node, OpSetIndex)
	case *parse.SliceExpression:
		if node.Operator != parse.Assign {
			return errors.New("only '=' assignement operator support for slice expressions")
		}

		if err := c.Compile(l.Indexed); err != nil {
			return err
		}

		if l.StartIndex != nil {
			if err := c.Compile(l.StartIndex); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}

		if l.EndIndex != nil {
			if err := c.Compile(l.EndIndex); err != nil {
				return err
			}
		} else {
			c.emit(node, OpPushNil)
		}

		if err := c.Compile(rhs); err != nil {
			return err
		}

		c.emit(node, OpSetSlice)
	default:
		return fmt.Errorf("invalid assigment: invalid LHS: %T", lhs)
	}

	return nil
}

func (c *compiler) CompileVar(node *parse.IdentifierLiteral) error {
	isGlobal := true
	symbol, ok := c.globalSymbols.Resolve(node.Name)
	if !ok {
		isGlobal = false
		symbol, ok = c.currentLocalSymbols().Resolve(node.Name)
	}
	if !ok {
		return c.NewError(node, fmt.Sprintf("unresolved reference '%s'", node.Name))
	}

	if isGlobal {
		c.emit(node, OpGetGlobal, c.addConstant(Str(node.Name)))
	} else {
		c.emit(node, OpGetLocal, symbol.Index)
	}
	return nil
}
func (c *compiler) compileLogical(node *parse.BinaryExpression) error {
	// left
	if err := c.Compile(node.Left); err != nil {
		return err
	}

	// operator
	var jumpPos int
	if node.Operator == parse.And {
		jumpPos = c.emit(node, OpAndJump, 0)
	} else {
		jumpPos = c.emit(node, OpOrJump, 0)
	}

	// right
	if err := c.Compile(node.Right); err != nil {
		return err
	}

	c.changeOperand(jumpPos, len(c.currentInstructions()))
	return nil
}

func (c *compiler) CompileStringPatternNode(node parse.Node) error {
	switch v := node.(type) {
	case *parse.QuotedStringLiteral:
		c.emit(node, OpPushConstant, c.addConstant(NewExactStringPattern(Str(v.Value))))
	case *parse.RuneLiteral:
		c.emit(node, OpPushConstant, c.addConstant(NewExactStringPattern(Str(v.Value))))
	case *parse.RuneRangeExpression:
		patt := NewRuneRangeStringPattern(v.Lower.Value, v.Upper.Value, node)
		c.emit(node, OpPushConstant, c.addConstant(patt))
	case *parse.IntegerRangeLiteral:
		var patt Pattern
		upperBound := int64(math.MaxInt64)

		if v.UpperBound != nil {
			upperBound = v.UpperBound.(*parse.IntLiteral).Value
		}
		patt = NewIntRangeStringPattern(v.LowerBound.Value, upperBound, node)
		c.emit(node, OpPushConstant, c.addConstant(patt))
	case *parse.PatternIdentifierLiteral:
		c.emit(node, OpResolvePattern, c.addConstant(Str(v.Name)))
	case *parse.PatternNamespaceIdentifierLiteral:
		if err := c.Compile(node); err != nil {
			return err
		}
	case *parse.PatternNamespaceMemberExpression:
		if err := c.Compile(node); err != nil {
			return err
		}
	case *parse.PatternUnion:
		for _, case_ := range v.Cases {
			if err := c.CompileStringPatternNode(case_); err != nil {
				return err
			}
		}

		c.emit(node, OpCreateStringUnionPattern, len(v.Cases))
	case *parse.ComplexStringPatternPiece:
		groupNames := make(KeyList, len(v.Elements))

		for i, element := range v.Elements {
			if err := c.CompileStringPatternNode(element.Expr); err != nil {
				return nil
			}

			if element.Ocurrence != parse.ExactlyOneOcurrence {
				c.emit(element, OpCreateRepeatedPatternElement, int(element.Ocurrence), element.ExactOcurrenceCount)
			}
			if element.GroupName != nil {
				groupNames[i] = element.GroupName.Name
			}
		}
		c.emit(node, OpCreateSequenceStringPattern, len(v.Elements), c.addConstant(groupNames))
	case *parse.RegularExpressionLiteral:
		patt := NewRegexPattern(v.Value)
		c.emit(node, OpPushConstant, c.addConstant(patt))
	default:
		return fmt.Errorf("cannot compile string pattern element: %T", v)
	}
	return nil
}

// compileMainChunk compiles the main chunk to bytecode & stores the result in c.module.
func (c *compiler) compileMainChunk(chunk *parse.ParsedChunk) (*Bytecode, error) {
	node := chunk.Node

	//add local scope
	scope := compilationScope{
		sourceMap: make(map[int]instructionSourcePosition),
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++
	c.localSymbolTableStack = append(c.localSymbolTableStack, newSymbolTable())
	if c.trace != nil {
		c.printTrace("ENTER SCOPE", c.scopeIndex)
	}
	c.chunkStack = append(c.chunkStack, chunk)
	defer func() {
		c.chunkStack = c.chunkStack[:len(c.chunkStack)-1]
	}()

	var err error

	//compile constants
	if node.GlobalConstantDeclarations != nil {
		err = c.Compile(node.GlobalConstantDeclarations)
	}

	//compile statements
	if err == nil {
		switch len(node.Statements) {
		case 0:
			c.emit(node, OpPushNil)
		case 1:
			if err := c.Compile(node.Statements[0]); err != nil {
				return nil, err
			}
		default:
			for _, stmt := range node.Statements {
				if err := c.Compile(stmt); err != nil {
					return nil, err
				}
				if stmt.Kind() == parse.Expr {
					c.emit(node, OpPop)
				}
			}
		}
	}

	//leave local scope
	instructions := c.currentInstructions()
	srcMap := c.currentSourceMap()
	localCount := c.currentLocalSymbols().SymbolCount()
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.localSymbolTableStack = c.localSymbolTableStack[:len(c.localSymbolTableStack)-1]
	c.scopeIndex--
	if c.trace != nil {
		c.printTrace("LEAVE SCOPE", c.scopeIndex)
	}

	//we create the bytecode and its main function

	main := &CompiledFunction{
		Instructions: append(instructions, OpSuspendVM),
		SourceMap:    srcMap,
		ParamCount:   0,
		IsVariadic:   false,
		LocalCount:   0,
	}
	main.LocalCount = localCount

	if len(c.constants) > math.MaxUint16 {
		panic("invalid constant count")
	}

	b := &Bytecode{
		module:    c.module,
		constants: c.constants,
		main:      main,
	}

	//we set the .Bytecode field of compiled functions
	for _, constant := range c.constants {
		if fn, ok := constant.(*InoxFunction); ok && fn.compiledFunction != nil {
			fn.compiledFunction.Bytecode = b
		}
	}

	main.Bytecode = b
	c.module.Bytecode = b

	return b, err
}

type LoopKind int

const (
	ForLoop LoopKind = iota
	WalkLoop
)

func (c *compiler) enterLoop(iteratorSymbol *symbol, kind LoopKind) *loop {
	loop := &loop{iteratorSymbol: iteratorSymbol, kind: kind}
	c.loops = append(c.loops, loop)
	c.loopIndex++
	if c.trace != nil {
		c.printTrace("ENTER LOOP", c.loopIndex)
	}
	return loop
}

func (c *compiler) leaveLoop() {
	if c.trace != nil {
		c.printTrace("LEAVE LOOP", c.loopIndex)
	}
	c.loops = c.loops[:len(c.loops)-1]
	c.loopIndex--
}

func (c *compiler) currentLoop() *loop {
	if c.loopIndex >= 0 {
		return c.loops[c.loopIndex]
	}
	return nil
}

func (c *compiler) currentWalkLoop() *loop {
	var lastWalkLoop *loop
	for _, loop := range c.loops {
		if loop.kind == WalkLoop {
			lastWalkLoop = loop
		}
	}
	return lastWalkLoop
}

func (c *compiler) currentInstructions() []byte {
	return c.scopes[c.scopeIndex].instructions
}

func (c *compiler) currentSourceMap() map[int]instructionSourcePosition {
	return c.scopes[c.scopeIndex].sourceMap
}

func (c *compiler) currentLocalSymbols() *symbolTable {
	return c.localSymbolTableStack[c.scopeIndex-1]
}

func (c *compiler) NewError(node parse.Node, msg string) error {
	loc := c.module.MainChunk.GetFormattedNodeLocation(node)

	return &CompileError{
		Node:    node,
		Module:  c.module,
		Message: fmt.Sprintf("compile: %s: %s", loc, msg),
	}
}

func (c *compiler) addConstant(v Value) int {
	c.constants = append(c.constants, v)
	if c.trace != nil {
		c.printTrace(fmt.Sprintf("CONST %04d %s", len(c.constants)-1, Stringify(v, c.context)))
	}
	return len(c.constants) - 1
}

func (c *compiler) addInstruction(b []byte) int {
	posNewIns := len(c.currentInstructions())
	c.scopes[c.scopeIndex].instructions = append(
		c.currentInstructions(), b...)
	return posNewIns
}

func (c *compiler) replaceInstruction(pos int, inst []byte) {
	copy(c.currentInstructions()[pos:], inst)
	if c.trace != nil {
		formatted := FormatInstructions(c.context, c.scopes[c.scopeIndex].instructions[pos:], pos, "", nil)[0]
		s := fmt.Sprintf("REPLACE %s", formatted)
		c.printTrace(s)
	}
}

func (c *compiler) changeOperand(opPos int, operand ...int) {
	op := c.currentInstructions()[opPos]
	inst := MakeInstruction(op, operand...)
	c.replaceInstruction(opPos, inst)
}

func (c *compiler) emit(
	node parse.Node,
	opcode Opcode,
	operands ...int,
) int {
	span := parse.NodeSpan{}
	if node != nil {
		span = node.Base().Span
	}

	inst := MakeInstruction(opcode, operands...)
	pos := c.addInstruction(inst)
	c.scopes[c.scopeIndex].sourceMap[pos] = instructionSourcePosition{
		span:  span,
		chunk: c.chunkStack[len(c.chunkStack)-1],
	}
	if c.trace != nil {
		instructions := c.scopes[c.scopeIndex].instructions[pos:]
		formatted := FormatInstructions(c.context, instructions, pos, "", nil)[0]
		c.printTrace(fmt.Sprintf("EMIT  %s", formatted))
	}
	c.lastOp = opcode
	return pos
}

func (c *compiler) currentChunk() *parse.ParsedChunk {
	return c.chunkStack[len(c.chunkStack)-1]
}

func (c *compiler) printTrace(a ...any) {
	var (
		dots = strings.Repeat(". ", 31)
		n    = len(dots)
	)

	i := 2 * c.indent
	for i > n {
		fmt.Fprint(c.trace, dots)
		i -= n
	}

	fmt.Fprint(c.trace, dots[0:i])
	fmt.Fprintln(c.trace, a...)
}

// func iterateInstructions(
// 	b []byte,
// 	fn func(pos int, opcode Opcode, operands []int) bool,
// ) {
// 	for i := 0; i < len(b); i++ {
// 		numOperands := OpcodeOperands[b[i]]
// 		operands, read := ReadOperands(numOperands, b[i+1:])
// 		if !fn(i, b[i], operands) {
// 			break
// 		}
// 		i += read
// 	}
// }

func (c *compiler) enterTracingBlock(msg string) {
	c.printTrace(msg, "{")
	c.indent++
}

func (c *compiler) leaveTracingBlock() {
	c.indent--
	c.printTrace("}")
}
