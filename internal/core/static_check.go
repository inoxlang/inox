package core

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	CHECK_ERR_PREFIX  = "check: "
	MAX_NAME_BYTE_LEN = 64
)

var (
	STATIC_CHECK_DATA_PROP_NAMES = []string{"errors"}
	ErrForbiddenNodeinPreinit    = errors.New("forbidden node type in preinit block")

	_ parse.LocatedError = &StaticCheckError{}
)

type StaticCheckInput struct {
	State                  *GlobalState //mainly used when checking imported modules
	Node                   parse.Node
	Module                 *Module
	Chunk                  *parse.ParsedChunkSource
	ParentChecker          *checker
	Globals                GlobalVariables
	AdditionalGlobalConsts []string
	ShellLocalVars         map[string]Value
	Patterns               map[string]Pattern
	PatternNamespaces      map[string]*PatternNamespace
}

// StaticCheck performs various checks on an AST, like checking duplicate declarations and keys or checking that statements like return,
// break and continue are not misplaced. No type checks are performed.
func StaticCheck(input StaticCheckInput) (*StaticCheckData, error) {
	if input.State == nil {
		return nil, errors.New("missing state")
	}

	globals := make(map[parse.Node]map[string]globalVarInfo)

	var module parse.Node //ok if nil

	switch input.Node.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
		module = input.Node
	}

	globals[module] = map[string]globalVarInfo{}
	input.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
		globals[module][name] = globalVarInfo{isConst: true, isStartConstant: true}
		return nil
	})

	for _, name := range input.AdditionalGlobalConsts {
		globals[module][name] = globalVarInfo{isConst: true}
	}

	shellLocalVars := make(map[string]bool)

	localVars := make(map[parse.Node]map[string]localVarInfo)
	localVars[module] = map[string]localVarInfo{}
	for k := range input.ShellLocalVars {
		localVars[module][k] = localVarInfo{}
		shellLocalVars[k] = true
	}

	patterns := make(map[parse.Node]map[string]int)
	patterns[module] = map[string]int{}
	for k := range input.Patterns {
		patterns[module][k] = 0
	}

	patternNamespaces := make(map[parse.Node]map[string]int)
	patternNamespaces[module] = map[string]int{}
	for k := range input.PatternNamespaces {
		patternNamespaces[module][k] = 0
	}

	checker := &checker{
		checkInput:        input,
		fnDecls:           make(map[parse.Node]map[string]*fnDeclInfo),
		structDefs:        make(map[parse.Node]map[string]int),
		globalVars:        globals,
		localVars:         localVars,
		shellLocalVars:    shellLocalVars,
		properties:        make(map[*parse.ObjectLiteral]*propertyInfo),
		patterns:          patterns,
		patternNamespaces: patternNamespaces,
		currentModule:     input.Module,
		chunk:             input.Chunk,
		store:             make(map[parse.Node]interface{}),
		data: &StaticCheckData{
			fnData:                                 map[*parse.FunctionExpression]*FunctionStaticData{},
			mappingData:                            map[*parse.MappingExpression]*MappingStaticData{},
			firstForbiddenPosForGlobalElementDecls: make(map[parse.Node]int32, 0),
			functionsToDeclareEarly:                make(map[parse.Node]*[]*parse.FunctionDeclaration, 0),
		},
	}

	if module != nil {

		if chunk, ok := module.(*parse.Chunk); ok {
			checker.defineStructs(module, chunk.Statements)
			checker.precheckTopLevelStatements(chunk)
		} else {
			checker.defineStructs(module, module.(*parse.EmbeddedModule).Statements)
		}
	}

	err := checker.check(input.Node)
	if err != nil {
		return nil, err
	}

	checker.data.combinedErrors = combineStaticCheckErrors(checker.data.errors...)
	return checker.data, checker.data.combinedErrors
}

// see Check function.
type checker struct {
	currentModule            *Module //can be nil
	chunk                    *parse.ParsedChunkSource
	inclusionImportStatement *parse.InclusionImportStatement // can be nil
	moduleImportStatement    *parse.ImportStatement          //can be nil
	parentChecker            *checker                        //can be nil
	checkInput               StaticCheckInput

	//key: *parse.Chunk|*parse.EmbeddedModule
	fnDecls map[parse.Node]map[string]*fnDeclInfo

	//key: *parse.Chunk|*parse.EmbeddedModule
	structDefs map[parse.Node]map[string]int

	//key: *parse.Chunk|*parse.EmbeddedModule
	globalVars map[parse.Node]map[string]globalVarInfo

	//key: *parse.Chunk|*parse.EmbeddedModule|*parse.FunctionExpression
	localVars map[parse.Node]map[string]localVarInfo

	properties map[*parse.ObjectLiteral]*propertyInfo

	//key: *parse.Chunk|*parse.EmbeddedModule
	patterns map[parse.Node]map[string]int

	//key: *parse.Chunk|*parse.EmbeddedModule
	patternNamespaces map[parse.Node]map[string]int

	shellLocalVars map[string]bool

	store map[parse.Node]any

	data *StaticCheckData
}

type fnDeclInfo struct {
	node           *parse.FunctionDeclaration
	capturedLocals []string
	module         parse.Node //*parse.Chunk|*parse.EmbeddedModule
}

// globalVarInfo represents the information stored about a global variable during checking.
type globalVarInfo struct {
	isConst         bool
	isStartConstant bool
	fnExpr          *parse.FunctionExpression
}

// locallVarInfo represents the information stored about a local variable during checking.
type localVarInfo struct {
	isGroupMatchingVar bool
}

// propertyInfo represents the information stored about the properties of an object literal during checking.
type propertyInfo struct {
	known map[string]bool
}

func (checker *checker) makeCheckingError(node parse.Node, s string) *StaticCheckError {
	location := checker.getSourcePositionStack(node)

	return NewStaticCheckError(s, location)
}

func (checker *checker) makeCheckingWarning(node parse.Node, s string) *StaticCheckWarning {
	location := checker.getSourcePositionStack(node)

	return NewStaticCheckWarning(s, location)
}

func (checker *checker) getSourcePositionStack(node parse.Node) parse.SourcePositionStack {
	var sourcePositionStack parse.SourcePositionStack

	if checker.parentChecker != nil {
		var importStmt parse.Node
		if checker.inclusionImportStatement != nil {
			importStmt = checker.inclusionImportStatement
		} else if checker.moduleImportStatement != nil {
			importStmt = checker.moduleImportStatement
		}
		sourcePositionStack = checker.parentChecker.getSourcePositionStack(importStmt)
	}

	sourcePositionStack = append(sourcePositionStack, checker.chunk.GetSourcePosition(node.Base().Span))
	return sourcePositionStack
}

func (checker *checker) addError(node parse.Node, s string) {
	checker.data.errors = append(checker.data.errors, checker.makeCheckingError(node, s))
}

func (checker *checker) addWarning(node parse.Node, s string) {
	checker.data.warnings = append(checker.data.warnings, checker.makeCheckingWarning(node, s))
}

func (c *checker) defineStructs(closestModule parse.Node, statements []parse.Node) {

	//Define structs from included chunks.
	for _, stmt := range statements {
		inclusionStmt, ok := stmt.(*parse.InclusionImportStatement)
		if !ok {
			continue
		}
		includedChunk := c.currentModule.InclusionStatementMap[inclusionStmt]
		if includedChunk == nil { //File not found
			return
		}
		c.defineStructs(closestModule, includedChunk.Node.Statements)
	}

	//Define other structs.
	for _, stmt := range statements {
		structDef, ok := stmt.(*parse.StructDefinition)
		if !ok {
			continue
		}

		name, ok := structDef.GetName()
		if ok {
			defs := c.getModStructDefs(closestModule)
			_, alreadyDefined := defs[name]
			if alreadyDefined {
				c.addError(structDef.Name, text.FmtInvalidStructDefAlreadyDeclared(name))
			} else {
				defs[name] = 0
			}
		}

		if structDef.Body == nil {
			continue
		}

		//check for duplicate member definitions.
		names := make([]string, 0, len(structDef.Body.Definitions))

		for _, memberDefinition := range structDef.Body.Definitions {
			name := ""
			var nameNode parse.Node

			switch def := memberDefinition.(type) {
			case *parse.StructFieldDefinition:
				name = def.Name.Name
				nameNode = def.Name
			case *parse.FunctionDeclaration:
				name = def.Name.Name
				nameNode = def.Name
			default:
				continue
			}

			if slices.Contains(names, name) {
				c.addError(nameNode, text.FmtAnXFieldOrMethodIsAlreadyDefined(name))
			} else {
				names = append(names, name)
			}
		}
	}
}

func (checker *checker) check(node parse.Node) error {
	checkNode := func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		return checker.checkSingleNode(node, parent, scopeNode, ancestorChain, after), nil
	}
	postCheckNode := func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		return checker.postCheckSingleNode(node, parent, scopeNode, ancestorChain, after), nil
	}
	return parse.Walk(node, checkNode, postCheckNode)
}

func (checker *checker) getLocalVarsInScope(scopeNode parse.Node) map[string]localVarInfo {
	if !parse.IsScopeContainerNode(scopeNode) {
		panic(fmt.Errorf("a %T is not a scope container", scopeNode))
	}

	variables, ok := checker.localVars[scopeNode]
	if !ok {
		variables = make(map[string]localVarInfo)
		checker.localVars[scopeNode] = variables
	}
	return variables
}

func (checker *checker) varExists(name string, ancestorChain []parse.Node) bool {
	var closestModule parse.Node

	checkGlobalVar := false

loop:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		if !parse.IsScopeContainerNode(ancestorChain[i]) {
			continue
		}

		scopeNode := ancestorChain[i]

		if checkGlobalVar {
			switch scopeNode.(type) {
			case *parse.Chunk, *parse.EmbeddedModule:
				closestModule = scopeNode
				break loop
			}
		}

		vars, ok := checker.localVars[scopeNode]
		if ok {
			if _, ok := vars[name]; ok {
				return true
			}
		}

		checkGlobalVar = true

		switch scopeNode.(type) {
		case *parse.Chunk, *parse.EmbeddedModule:
			closestModule = scopeNode
			break loop
		}
	}

	globalVars := checker.getModGlobalVars(closestModule)
	_, ok := globalVars[name]
	return ok
}

func (checker *checker) doGlobalVarExist(name string, closestModule parse.Node) bool {
	globals := checker.getModGlobalVars(closestModule)
	_, ok := globals[name]
	return ok
}

func (checker *checker) setScopeLocalVars(scopeNode parse.Node, vars map[string]localVarInfo) {
	checker.localVars[scopeNode] = vars
}

func (checker *checker) getScopeLocalVarsCopy(scopeNode parse.Node) map[string]localVarInfo {
	variables := checker.getLocalVarsInScope(scopeNode)

	varsCopy := make(map[string]localVarInfo)
	for k, v := range variables {
		varsCopy[k] = v
	}
	return varsCopy
}

func (checker *checker) getModGlobalVars(module parse.Node) map[string]globalVarInfo {
	variables, ok := checker.globalVars[module]
	if !ok {
		variables = make(map[string]globalVarInfo)
		checker.globalVars[module] = variables
	}
	return variables
}

func (checker *checker) getModFunctionDecls(mod parse.Node) map[string]*fnDeclInfo {
	fns, ok := checker.fnDecls[mod]
	if !ok {
		fns = make(map[string]*fnDeclInfo)
		checker.fnDecls[mod] = fns
	}
	return fns
}

func (checker *checker) isDeclaredFunctionName(name string, mod parse.Node) bool {
	fns, ok := checker.fnDecls[mod]
	if !ok {
		return false
	}
	_, ok = fns[name]
	return ok
}

func (checker *checker) getModStructDefs(mod parse.Node) map[string]int {
	defs, ok := checker.structDefs[mod]
	if !ok {
		defs = make(map[string]int)
		checker.structDefs[mod] = defs
	}
	return defs
}

func (checker *checker) getModPatterns(mod parse.Node) map[string]int {
	patterns, ok := checker.patterns[mod]
	if !ok {
		patterns = make(map[string]int)
		checker.patterns[mod] = patterns
	}
	return patterns
}

func (checker *checker) getModPatternNamespaces(module parse.Node) map[string]int {
	namespaces, ok := checker.patternNamespaces[module]
	if !ok {
		namespaces = make(map[string]int)
		checker.patternNamespaces[module] = namespaces
	}
	return namespaces
}

func (checker *checker) getPropertyInfo(obj *parse.ObjectLiteral) *propertyInfo {
	propInfo, ok := checker.properties[obj]
	if !ok {
		propInfo = &propertyInfo{known: make(map[string]bool, 0)}
		checker.properties[obj] = propInfo
	}
	return propInfo
}

func findClosestModule(ancestorChain []parse.Node) parse.Node {
	var closestModule parse.Node

	for _, n := range ancestorChain {
		switch n.(type) {
		case *parse.Chunk, *parse.EmbeddedModule:
			closestModule = n
		}
	}

	return closestModule
}

func findClosest[T any](ancestorChain []parse.Node) T {
	var closest T

	for _, n := range ancestorChain {
		switch node := n.(type) {
		case T:
			closest = node
		}
	}

	return closest
}

func findClosestScopeContainerNode(ancestorChain []parse.Node) parse.Node {
	var closest parse.Node

	for _, n := range ancestorChain {
		if parse.IsScopeContainerNode(n) {
			closest = n
		}
	}

	return closest
}

// checkSingleNode perform checks on a single node.
func (c *checker) checkSingleNode(n, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) parse.TraversalAction {
	closestModule := findClosestModule(ancestorChain)
	closestAssertion := findClosest[*parse.AssertionStatement](ancestorChain)
	inPreinitBlock := findClosest[*parse.PreinitStatement](ancestorChain) != nil

	//Check that the node is allowed in assertions.

	if closestAssertion != nil {
		switch n := n.(type) {
		case
			//variables
			*parse.Variable, *parse.IdentifierLiteral,

			*parse.BinaryExpression, *parse.UnaryExpression, *parse.URLExpression,
			parse.SimpleValueLiteral, *parse.IntegerRangeLiteral, *parse.FloatRangeLiteral,

			//data structure literals
			*parse.ObjectLiteral, *parse.ObjectProperty, *parse.ListLiteral, *parse.RecordLiteral,

			//member-like expressions
			*parse.MemberExpression, *parse.IdentifierMemberExpression, *parse.DoubleColonExpression,
			*parse.IndexExpression, *parse.SliceExpression,

			//patterns
			*parse.PatternIdentifierLiteral,
			*parse.ObjectPatternLiteral, *parse.ObjectPatternProperty, *parse.RecordPatternLiteral,
			*parse.ListPatternLiteral, *parse.TuplePatternLiteral,
			*parse.FunctionPatternExpression,
			*parse.PatternNamespaceIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
			*parse.OptionPatternLiteral, *parse.OptionalPatternExpression,
			*parse.ComplexStringPatternPiece, *parse.PatternPieceElement, *parse.PatternGroupName,
			*parse.PatternUnion,
			*parse.PatternCallExpression:
		case *parse.CallExpression:
			allowed := false

			ident, ok := n.Callee.(*parse.IdentifierLiteral)
			if ok {
				switch ident.Name {
				case globalnames.LEN_FN:
					allowed = true
				}
			}

			if !allowed {
				c.addError(n, text.FmtFollowingNodeTypeNotAllowedInAssertions(n))
			}
		default:
			if !parse.NodeIsSimpleValueLiteral(n) {
				c.addError(n, text.FmtFollowingNodeTypeNotAllowedInAssertions(n))
			}
		}
	}

	//Actually check the node.

	switch node := n.(type) {
	case *parse.IntegerRangeLiteral:
		if upperBound, ok := node.UpperBound.(*parse.IntLiteral); ok && node.LowerBound.Value > upperBound.Value {
			c.addError(n, text.LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND)
		}
	case *parse.FloatRangeLiteral:
		if upperBound, ok := node.UpperBound.(*parse.FloatLiteral); ok && node.LowerBound.Value > upperBound.Value {
			c.addError(n, text.LOWER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND)
		}
	case *parse.QuantityLiteral:
		return c.checkQuantityLiteral(node)
	case *parse.RateLiteral:
		return c.checkRateLiteral(node)
	case *parse.URLLiteral:
		if strings.HasPrefix(node.Value, "mem://") && utils.Must(url.Parse(node.Value)).Host != MEM_HOSTNAME {
			c.addError(node, text.INVALID_MEM_HOST_ONLY_VALID_VALUE)
		}
	case *parse.HostLiteral:
		if strings.HasPrefix(node.Value, "mem://") && utils.Must(url.Parse(node.Value)).Host != MEM_HOSTNAME {
			c.addError(node, text.INVALID_MEM_HOST_ONLY_VALID_VALUE)
		}
	case *parse.ObjectLiteral:
		return c.checkObjectLiteral(node)
	case *parse.RecordLiteral:
		return c.checkRecordLiteral(node)
	case *parse.ObjectPatternLiteral, *parse.RecordPatternLiteral:
		return c.checkObjectRecordPatternLiteral(node)
	case *parse.DictionaryLiteral:
		return c.checkDictionaryLiteral(node)
	case *parse.SpawnExpression:
		return c.checkSpawnExpr(node, closestModule)
	case *parse.LifetimejobExpression:
		return c.checkLifetimejobExpr(node, parent, closestModule)
	case *parse.ReceptionHandlerExpression:
		if prop, ok := parent.(*parse.ObjectProperty); !ok || !prop.HasNoKey() {
			c.addError(node, text.MISPLACED_RECEPTION_HANDLER_EXPRESSION)
		}

	case *parse.MappingExpression:
		//
	case *parse.StaticMappingEntry:
		return c.checkStaticMappingEntry(node)
	case *parse.DynamicMappingEntry:
		return c.checkDynamicMappingEntry(node)
	case *parse.ComputeExpression:
		return c.checkComputeExpr(node, scopeNode, ancestorChain)
	case *parse.InclusionImportStatement:
		return c.checkInclusionImportStmt(node, parent, closestModule, inPreinitBlock)
	case *parse.ImportStatement:
		return c.checkImportStmt(node, parent, closestModule)
	case *parse.GlobalConstantDeclarations:
		return c.checkGlobalConstDecls(node, parent, closestModule)
	case *parse.LocalVariableDeclarations:
		return c.checkLocalVarDecls(node, scopeNode, closestModule)
	case *parse.GlobalVariableDeclarations:
		return c.checkGlobalVarDecls(node, parent, scopeNode, closestModule)
	case *parse.Assignment, *parse.MultiAssignment:
		return c.checkAssignment(node, scopeNode, closestModule)
	case *parse.ForStatement:
		return c.checkForStmt(node, scopeNode, closestModule)
	case *parse.ForExpression:
		return c.checkForExpression(node, scopeNode, closestModule)
	case *parse.WalkStatement:
		return c.checkWalkStmt(node, scopeNode, closestModule)
	case *parse.ReadonlyPatternExpression:
		return c.checkReadonlyPatternExpr(node, parent)
	case *parse.CallExpression:
		return c.checkCallExpression(node, scopeNode, closestModule)
	case *parse.FunctionDeclaration:
		return c.checkFuncDecl(node, parent, closestModule)
	case *parse.FunctionExpression:
		return c.checkFuncExpr(node, closestModule, ancestorChain)
	case *parse.FunctionPatternExpression:
		return c.checkFuncPatternExpr(node, closestModule)
	case *parse.ReturnStatement:
		return c.checkReturnStatement(node, ancestorChain)
	case *parse.CoyieldStatement:
		return c.checkCoyieldStmt(node, ancestorChain)
	case *parse.BreakStatement, *parse.ContinueStatement:
		return c.checkBreakContinueStmt(node, ancestorChain)
	case *parse.YieldStatement:
		return c.checkYieldStmt(node, ancestorChain)
	case *parse.PruneStatement:
		return c.checkPruneStmt(node, ancestorChain)
	case *parse.SwitchStatement:
		return c.checkSwitchStatement(node, scopeNode, closestModule)
	case *parse.MatchStatement:
		return c.checkMatchStatement(node, scopeNode, closestModule)
	case *parse.MatchStatementCase:
		return c.checkMatchCase(node, scopeNode, closestModule)
	case *parse.Variable:
		return c.checkVariable(node, scopeNode, ancestorChain, closestModule)
	case *parse.IdentifierLiteral:
		return c.checkIdentifier(node, parent, scopeNode, closestModule, ancestorChain)
	case *parse.SelfExpression, *parse.SendValueExpression:
		return c.checkSelfExprAndSendValExpr(n, parent, ancestorChain)
	case *parse.PatternDefinition:
		return c.checkPatternDef(node, parent, closestModule, inPreinitBlock)
	case *parse.PatternNamespaceDefinition:
		return c.checkPatternNamespaceDefinition(node, parent, closestModule, inPreinitBlock)
	case *parse.PatternNamespaceIdentifierLiteral:
		return c.checkPatternNamespaceIdentifier(node, closestModule)
	case *parse.PatternIdentifierLiteral:
		return c.checkPatternIdentifier(node, parent, closestModule, ancestorChain)
	case *parse.RuntimeTypeCheckExpression:
		return c.checkRuntimeTypeCheckExpr(node, parent)
	case *parse.DynamicMemberExpression:
		if node.Optional {
			c.addError(node, text.OPTIONAL_DYN_MEMB_EXPR_NOT_SUPPORTED_YET)
		}
	case *parse.ExtendStatement:
		if _, ok := parent.(*parse.Chunk); !ok {
			c.addError(node, text.MISPLACED_EXTEND_STATEMENT_TOP_LEVEL_STMT)
			return parse.ContinueTraversal
		}
	case *parse.StructDefinition:
		if parent != closestModule {
			c.addError(node, text.MISPLACED_STRUCT_DEF_TOP_LEVEL_STMT)
			return parse.ContinueTraversal
		}
		//already defined.
		return parse.ContinueTraversal
	case *parse.NewExpression:
		return c.checkNewExpr(node)
	case *parse.StructInitializationLiteral:
		return c.checkStructInitLiteral(node)
	case *parse.PointerType:
		return c.checkPointerType(node, parent)
	case *parse.DereferenceExpression:
		c.addError(node, "dereference expressions are not supported yet")
	case *parse.TestSuiteExpression:
		return c.checkTestSuiteExpr(node, ancestorChain)
	case *parse.TestCaseExpression:
		return c.checkTestCaseExpr(node, ancestorChain)
	case *parse.EmbeddedModule:
		return c.checkEmbeddedModule(node, parent, closestModule, ancestorChain)
	}

	return parse.ContinueTraversal
}

func (c *checker) precheckTopLevelStatements(module parse.Node) {
	chunk, isChunk := module.(*parse.Chunk)
	isIncludedChunk := isChunk && chunk.IncludableChunkDesc != nil
	embeddedMod, isEmbeddedMod := module.(*parse.EmbeddedModule)

	if !isChunk && !isEmbeddedMod {
		panic(fmt.Errorf("precheckTopLevelStatements should be called on a chunk or embedded module"))
	}

	var statements []parse.Node

	if isChunk {
		statements = chunk.Statements
	} else {
		statements = embeddedMod.Statements
	}

	for _, stmt := range statements {
		switch stmt := stmt.(type) {
		//definitions
		case *parse.PatternDefinition:
		case *parse.PatternNamespaceDefinition:
		case *parse.ExtendStatement:
		case *parse.StructDefinition:
		case *parse.FunctionDeclaration:
			c.precheckTopLevelFuncDecl(stmt, module)
		//simple literals
		case parse.SimpleValueLiteral:
		//inclusion imports
		case *parse.InclusionImportStatement:
		//otter nodes
		default:
			if isIncludedChunk {
				c.addError(stmt, text.AN_INCLUDABLE_FILE_CAN_ONLY_CONTAIN_DEFINITIONS)
			}
		}
	}
}

func (c *checker) checkQuantityLiteral(node *parse.QuantityLiteral) parse.TraversalAction {

	var prevMultiplier string
	var prevUnit string
	var prevDurationUnitValue time.Duration

	for partIndex := 0; partIndex < len(node.Values); partIndex++ {
		if node.Values[partIndex] < 0 {
			c.addError(node, ErrNegQuantityNotSupported.Error())
			return parse.ContinueTraversal
		}

		i := 0
		var multiplier string

		switch node.Units[partIndex][0] {
		case 'k', 'M', 'G', 'T':
			multiplier = node.Units[partIndex]
			i++
		default:
		}

		prevMultiplier = multiplier
		_ = prevMultiplier

		if i > 0 && len(node.Units[partIndex]) == 1 {
			c.addError(node, text.FmtNonSupportedUnit(node.Units[0]))
			return parse.ContinueTraversal
		}

		unit := node.Units[partIndex][i:]

		switch unit {
		case "x", LINE_COUNT_UNIT, RUNE_COUNT_UNIT, BYTE_COUNT_UNIT:
			if partIndex != 0 || prevUnit != "" {
				c.addError(node, text.INVALID_QUANTITY)
				return parse.ContinueTraversal
			}
			prevUnit = unit
		case "h", "mn", "s", "ms", "us", "ns":
			var durationUnitValue time.Duration

			switch unit {
			case "h":
				durationUnitValue = time.Hour
			case "mn":
				durationUnitValue = time.Minute
			case "s":
				durationUnitValue = time.Second
			case "ms":
				durationUnitValue = time.Millisecond
			case "us":
				durationUnitValue = time.Microsecond
			case "ns":
				durationUnitValue = time.Nanosecond
			}

			if prevUnit != "" && (prevDurationUnitValue == 0 || durationUnitValue >= prevDurationUnitValue) {
				c.addError(node, text.INVALID_QUANTITY)
				return parse.ContinueTraversal
			}

			prevDurationUnitValue = durationUnitValue
			prevUnit = unit
		case "%":
			if partIndex != 0 || prevUnit != "" {
				c.addError(node, text.INVALID_QUANTITY)
				return parse.ContinueTraversal
			}
			if i == 0 {
				prevUnit = unit
				break
			}
			fallthrough
		default:
			c.addError(node, text.FmtNonSupportedUnit(node.Units[0]))
			return parse.ContinueTraversal
		}
	}

	_, err := evalQuantity(node.Values, node.Units)
	if err != nil {
		c.addError(node, err.Error())
	}

	return parse.ContinueTraversal
}

func (c *checker) checkRateLiteral(node *parse.RateLiteral) parse.TraversalAction {
	lastUnit1 := node.Units[len(node.Units)-1]
	rateUnit := node.DivUnit

	switch rateUnit {
	case "s":
		i := 0
		switch lastUnit1[0] {
		case 'k', 'M', 'G', 'T':
			i++
		default:
		}
		switch lastUnit1[i:] {
		case "x", BYTE_COUNT_UNIT:
			return parse.ContinueTraversal
		}
	}
	c.addError(node, text.INVALID_RATE)
	return parse.ContinueTraversal
}

func (c *checker) checkObjectLiteral(node *parse.ObjectLiteral) parse.TraversalAction {
	action, keys := shallowCheckObjectRecordProperties(node.Properties, node.SpreadElements, true, func(n parse.Node, msg string) {
		c.addError(n, msg)
	})

	if action != parse.ContinueTraversal {
		return action
	}

	propInfo := c.getPropertyInfo(node)
	for k := range keys {
		propInfo.known[k] = true
	}

	for _, metaprop := range node.MetaProperties {
		switch metaprop.Name() {
		case VISIBILITY_KEY:
			checkVisibilityInitializationBlock(propInfo, metaprop.Initialization, func(n parse.Node, msg string) {
				c.addError(n, msg)
			})
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkRecordLiteral(node *parse.RecordLiteral) parse.TraversalAction {
	action, _ := shallowCheckObjectRecordProperties(node.Properties, node.SpreadElements, false, func(n parse.Node, msg string) {
		c.addError(n, msg)
	})

	return action
}

func (c *checker) checkObjectRecordPatternLiteral(node parse.Node) parse.TraversalAction {
	keys := map[string]struct{}{}

	var propertyNodes []*parse.ObjectPatternProperty
	var spreadElementsNodes []*parse.PatternPropertySpreadElement
	var otherPropsNodes []*parse.OtherPropsExpr
	var isExact bool

	switch node := node.(type) {
	case *parse.ObjectPatternLiteral:
		propertyNodes = node.Properties
		spreadElementsNodes = node.SpreadElements
		otherPropsNodes = node.OtherProperties
		isExact = node.Exact()
	case *parse.RecordPatternLiteral:
		propertyNodes = node.Properties
		spreadElementsNodes = node.SpreadElements
		otherPropsNodes = node.OtherProperties
		isExact = node.Exact()
	}

	// look for duplicate keys
	for _, prop := range propertyNodes {
		var k string

		switch n := prop.Key.(type) {
		case *parse.DoubleQuotedStringLiteral:
			k = n.Value
		case *parse.IdentifierLiteral:
			k = n.Name
		case nil:
			continue
		}

		if len(k) > MAX_NAME_BYTE_LEN {
			c.addError(prop.Key, text.FmtNameIsTooLong(k))
		}

		if parse.IsMetadataKey(k) {
			c.addError(prop.Key, text.OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS)
		} else if _, found := keys[k]; found {
			c.addError(prop, text.FmtDuplicateKey(k))
		}

		keys[k] = struct{}{}
	}

	// also look for duplicate keys
	for _, element := range spreadElementsNodes {
		extractionExpr, ok := element.Expr.(*parse.ExtractionExpression)
		if !ok {
			continue
		}

		for _, key := range extractionExpr.Keys.Keys {
			name := key.(*parse.IdentifierLiteral).Name

			_, found := keys[name]
			if found {
				c.addError(key, text.FmtDuplicateKey(name))
				return parse.ContinueTraversal
			}
			keys[name] = struct{}{}
		}
	}

	//check that if the pattern is exact there are no other otherprops nodes other than otherprops(no)
	if isExact {
		for _, prop := range otherPropsNodes {
			patternIdent, ok := prop.Pattern.(*parse.PatternIdentifierLiteral)

			if !ok || patternIdent.Name != parse.NO_OTHERPROPS_PATTERN_NAME {
				c.addError(prop, text.UNEXPECTED_OTHER_PROPS_EXPR_OTHERPROPS_NO_IS_PRESENT)
			}
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkDictionaryLiteral(node *parse.DictionaryLiteral) parse.TraversalAction {
	keys := map[string]bool{}

	// look for duplicate keys
	for _, entry := range node.Entries {

		keyNode, ok := entry.Key.(parse.SimpleValueLiteral)
		if !ok {
			//there is a parsing error
			continue
		}

		keyRepr := keyNode.ValueString()

		if keys[keyRepr] {
			c.addError(entry.Key, text.FmtDuplicateDictKey(keyRepr))
		} else {
			keys[keyRepr] = true
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkSpawnExpr(node *parse.SpawnExpression, closestModule parse.Node) parse.TraversalAction {

	var globals = make(map[string]globalVarInfo)
	var globalDescNode parse.Node

	// add constant globals
	parentModuleGlobals := c.getModGlobalVars(closestModule)
	for name, info := range parentModuleGlobals {
		if info.isStartConstant {
			globals[name] = info
		}
	}

	// add globals passed by user
	if obj, ok := node.Meta.(*parse.ObjectLiteral); ok {
		if len(obj.SpreadElements) > 0 {
			c.addError(node.Meta, text.INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED)
		}

		for _, prop := range obj.Properties {
			if prop.HasNoKey() {
				c.addError(node.Meta, text.INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED)
			}
		}

		val, ok := obj.PropValue(symbolic.LTHREAD_META_GLOBALS_SECTION)
		if ok {
			globalDescNode = val
		}
	} else if node.Meta != nil {
		c.addError(node.Meta, text.INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED)
	}

	switch desc := globalDescNode.(type) {
	case *parse.KeyListExpression:
		for _, ident := range desc.Keys {
			globVarName := ident.(*parse.IdentifierLiteral).Name
			if !c.doGlobalVarExist(globVarName, closestModule) {
				c.addError(globalDescNode, text.FmtCannotPassGlobalThatIsNotDeclaredToLThread(globVarName))
			}
			globals[globVarName] = globalVarInfo{isConst: true}
		}
	case *parse.ObjectLiteral:
		if len(desc.SpreadElements) > 0 {
			c.addError(desc, text.INVALID_SPAWN_GLOBALS_SHOULD_BE)
		}

		for _, prop := range desc.Properties {
			if prop.HasNoKey() {
				c.addError(desc, text.INVALID_SPAWN_GLOBALS_SHOULD_BE)
				continue
			}
			globals[prop.Name()] = globalVarInfo{isConst: true}
		}
	case *parse.NilLiteral:
	case nil:
	default:
		c.addError(node, text.INVALID_SPAWN_GLOBALS_SHOULD_BE)
	}

	if node.Module != nil && node.Module.SingleCallExpr {
		calleeNode := node.Module.Statements[0].(*parse.CallExpression).Callee

		switch calleeNode := calleeNode.(type) {
		case *parse.IdentifierLiteral:
			globals[calleeNode.Name] = globalVarInfo{isConst: true}
		case *parse.IdentifierMemberExpression:
			globals[calleeNode.Left.Name] = globalVarInfo{isConst: true}
		}
	}

	embeddedModuleGlobals := c.getModGlobalVars(node.Module)

	for name, info := range globals {
		embeddedModuleGlobals[name] = info
	}

	c.defineStructs(node.Module, node.Module.Statements)
	c.precheckTopLevelStatements(node.Module)

	return parse.ContinueTraversal
}

func (c *checker) checkLifetimejobExpr(node *parse.LifetimejobExpression, parent, closestModule parse.Node) parse.TraversalAction {

	lifetimeJobGlobals := c.getModGlobalVars(node.Module)

	for name, info := range c.getModGlobalVars(closestModule) {
		lifetimeJobGlobals[name] = info
	}

	lifetimeJobPatterns := c.getModPatterns(node.Module)

	for name, info := range c.getModPatterns(closestModule) {
		lifetimeJobPatterns[name] = info
	}

	lifetimeJobPatternNamespaces := c.getModPatternNamespaces(node.Module)

	for name, info := range c.getModPatternNamespaces(closestModule) {
		lifetimeJobPatternNamespaces[name] = info
	}

	if node.Subject != nil {
		return parse.ContinueTraversal
	}

	if prop, ok := parent.(*parse.ObjectProperty); !ok || !prop.HasNoKey() {
		c.addError(node, text.MISSING_LIFETIMEJOB_SUBJECT_PATTERN_NOT_AN_IMPLICIT_OBJ_PROP)
	}

	return parse.ContinueTraversal
}

func (c *checker) checkStaticMappingEntry(node *parse.StaticMappingEntry) parse.TraversalAction {
	switch node.Key.(type) {
	case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
	default:
		if !parse.NodeIsSimpleValueLiteral(node.Key) {
			c.addError(node.Key, text.INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS)
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkDynamicMappingEntry(node *parse.DynamicMappingEntry) parse.TraversalAction {
	switch node.Key.(type) {
	case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
	default:
		if !parse.NodeIsSimpleValueLiteral(node.Key) {
			c.addError(node.Key, text.INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS)
		}
	}

	localVars := c.getLocalVarsInScope(node)
	varname := node.KeyVar.(*parse.IdentifierLiteral).Name
	localVars[varname] = localVarInfo{}

	if node.GroupMatchingVariable != nil {
		varname := node.GroupMatchingVariable.(*parse.IdentifierLiteral).Name
		localVars[varname] = localVarInfo{}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkComputeExpr(node *parse.ComputeExpression, scopeNode parse.Node, ancestorChain []parse.Node) parse.TraversalAction {
	if _, ok := scopeNode.(*parse.DynamicMappingEntry); !ok {
		c.addError(node, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
	} else {
	ancestor_loop:
		for i := len(ancestorChain) - 1; i >= 0; i-- {
			ancestor := ancestorChain[i]
			if ancestor == scopeNode {
				break
			}

			switch a := ancestor.(type) {
			case *parse.StaticMappingEntry:
				c.addError(node, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
				break ancestor_loop
			case *parse.DynamicMappingEntry:
				if a.Key == node || i < len(ancestorChain)-1 && ancestorChain[i+1] == a.Key {
					c.addError(node, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
				}
				break ancestor_loop
			}
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkInclusionImportStmt(node *parse.InclusionImportStatement, parent, closestModule parse.Node, inPreinitBlock bool) parse.TraversalAction {
	// if the import is performed by the preinit block, prune the traversal.
	if _, ok := parent.(*parse.Block); ok && inPreinitBlock {
		return parse.Prune
	}

	if _, ok := parent.(*parse.Chunk); !ok {
		c.addError(node, text.MISPLACED_INCLUSION_IMPORT_STATEMENT_TOP_LEVEL_STMT)
		return parse.ContinueTraversal
	}

	includedChunk := c.currentModule.InclusionStatementMap[node]
	if includedChunk == nil { //File not found
		return parse.ContinueTraversal
	}

	globals := make(map[parse.Node]map[string]globalVarInfo)
	globals[includedChunk.Node] = map[string]globalVarInfo{}

	// add globals to child checker
	c.checkInput.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
		globals[includedChunk.Node][name] = globalVarInfo{isConst: isStartConstant}
		return nil
	})

	// add defined patterns & pattern namespaces to child checker
	patterns := make(map[parse.Node]map[string]int)
	patterns[includedChunk.Node] = map[string]int{}
	for k := range c.checkInput.Patterns {
		patterns[includedChunk.Node][k] = 0
	}

	patternNamespaces := make(map[parse.Node]map[string]int)
	patternNamespaces[includedChunk.Node] = map[string]int{}
	for k := range c.checkInput.PatternNamespaces {
		patternNamespaces[includedChunk.Node][k] = 0
	}

	chunkChecker := &checker{
		parentChecker:            c,
		checkInput:               c.checkInput,
		fnDecls:                  make(map[parse.Node]map[string]*fnDeclInfo),
		structDefs:               make(map[parse.Node]map[string]int),
		globalVars:               globals,
		localVars:                make(map[parse.Node]map[string]localVarInfo),
		properties:               make(map[*parse.ObjectLiteral]*propertyInfo),
		patterns:                 patterns,
		patternNamespaces:        patternNamespaces,
		currentModule:            c.currentModule,
		chunk:                    includedChunk.ParsedChunkSource,
		inclusionImportStatement: node,
		store:                    make(map[parse.Node]any),
		data: &StaticCheckData{
			fnData:                                 map[*parse.FunctionExpression]*FunctionStaticData{},
			mappingData:                            map[*parse.MappingExpression]*MappingStaticData{},
			firstForbiddenPosForGlobalElementDecls: c.data.firstForbiddenPosForGlobalElementDecls,
			functionsToDeclareEarly:                c.data.functionsToDeclareEarly,
		},
	}

	chunkChecker.precheckTopLevelStatements(includedChunk.Node)

	err := chunkChecker.check(includedChunk.Node)
	if err != nil {
		panic(err)
	}

	if len(chunkChecker.data.errors) != 0 {
		c.data.errors = append(c.data.errors, chunkChecker.data.errors...)
	}

	if len(chunkChecker.data.warnings) != 0 {
		c.data.warnings = append(c.data.warnings, chunkChecker.data.warnings...)
	}

	for k, v := range chunkChecker.data.fnData {
		c.data.fnData[k] = v
	}

	for k, v := range chunkChecker.data.mappingData {
		c.data.mappingData[k] = v
	}

	// include all global data & top level local variables
	for k, v := range chunkChecker.fnDecls[includedChunk.Node] {
		if c.checkInput.Globals.Has(k) {
			continue
		}

		fnDecls := c.getModFunctionDecls(closestModule)
		if _, ok := fnDecls[k]; ok {
			// handled in next loop
		} else {
			fnDecls[k] = v
		}
	}

	for k, v := range chunkChecker.globalVars[includedChunk.Node] {
		if c.checkInput.Globals.Has(k) {
			continue
		}

		globalVars := c.getModGlobalVars(closestModule)
		if _, ok := globalVars[k]; ok {
			c.addError(node, text.FmtCannotShadowGlobalVariable(k))
		} else {
			globalVars[k] = v
		}
	}

	for k, v := range chunkChecker.localVars[includedChunk.Node] {
		localVars := c.getLocalVarsInScope(closestModule)
		if _, ok := localVars[k]; ok {
			c.addError(node, text.FmtCannotShadowLocalVariable(k))
		} else {
			localVars[k] = v
		}
	}

	for k, v := range chunkChecker.patterns[includedChunk.Node] {
		if _, ok := c.checkInput.Patterns[k]; ok {
			continue
		}

		patterns := c.getModPatterns(closestModule)
		if _, ok := patterns[k]; ok {
			c.addError(node, text.FmtPatternAlreadyDeclared(k))
		} else {
			patterns[k] = v
		}
	}

	for k, v := range chunkChecker.patternNamespaces[includedChunk.Node] {
		if _, ok := c.checkInput.PatternNamespaces[k]; ok {
			continue
		}

		namespaces := c.getModPatternNamespaces(closestModule)
		if _, ok := namespaces[k]; ok {
			c.addError(node, text.FmtPatternNamespaceAlreadyDeclared(k))
		} else {
			namespaces[k] = v
		}
	}

	if v, ok := chunkChecker.store[includedChunk.Node]; ok {
		panic(fmt.Errorf("data stored for included chunk %#v : %#v", includedChunk.Node, v))
	}

	return parse.ContinueTraversal
}

func (c *checker) checkImportStmt(node *parse.ImportStatement, parent, closestModule parse.Node) parse.TraversalAction {
	if c.inclusionImportStatement != nil {
		c.addError(node, text.MODULE_IMPORTS_NOT_ALLOWED_IN_INCLUDABLE_FILES)
		return parse.Prune
	}

	if _, ok := parent.(*parse.Chunk); !ok {
		c.addError(node, text.MISPLACED_MOD_IMPORT_STATEMENT_TOP_LEVEL_STMT)
		return parse.Prune
	}

	name := node.Identifier.Name
	variables := c.getModGlobalVars(closestModule)

	_, alreadyUsed := variables[name]
	if alreadyUsed {
		c.addError(node, text.FmtInvalidImportStmtAlreadyDeclaredGlobal(name))
		return parse.ContinueTraversal
	}
	variables[name] = globalVarInfo{isConst: true}

	if c.inclusionImportStatement != nil || node.Source == nil {
		return parse.ContinueTraversal
	}

	var importedModuleSource GoString

	switch node.Source.(type) {
	case *parse.URLLiteral, *parse.AbsolutePathLiteral, *parse.RelativePathLiteral:
		value, err := EvalSimpleValueLiteral(node.Source.(parse.SimpleValueLiteral), nil)
		if err != nil {
			panic(ErrUnreachable)
		}
		src, err := getSourceFromImportSource(value, c.currentModule, c.checkInput.State.Ctx)
		if err != nil {
			c.addError(node, fmt.Sprintf("failed to resolve location of imported module: %s", err.Error()))
			return parse.ContinueTraversal
		}
		importedModuleSource = src
	default:
		return parse.ContinueTraversal
	}

	importedModule := c.currentModule.DirectlyImportedModules[importedModuleSource.UnderlyingString()]
	importedModuleNode := importedModule.MainChunk.Node

	globals := make(map[parse.Node]map[string]globalVarInfo)
	globals[importedModuleNode] = map[string]globalVarInfo{}

	//add base globals to child checker
	for globalName := range c.checkInput.State.SymbolicBaseGlobalsForImportedModule {
		globals[importedModuleNode][globalName] = globalVarInfo{isConst: true, isStartConstant: true}
	}

	//add module arguments variable to child checker
	globals[importedModuleNode][globalnames.MOD_ARGS_VARNAME] = globalVarInfo{isConst: true, isStartConstant: true}

	//add base patterns & pattern namespaces to child checker
	basePatterns, basePatternNamespaces := c.checkInput.State.GetBasePatternsForImportedModule()

	patterns := make(map[parse.Node]map[string]int)
	patterns[importedModuleNode] = map[string]int{}
	for k := range basePatterns {
		patterns[importedModuleNode][k] = 0
	}

	patternNamespaces := make(map[parse.Node]map[string]int)
	patternNamespaces[importedModuleNode] = map[string]int{}
	for k := range basePatternNamespaces {
		patternNamespaces[importedModuleNode][k] = 0
	}

	chunkChecker := &checker{
		parentChecker:         c,
		checkInput:            c.checkInput,
		fnDecls:               make(map[parse.Node]map[string]*fnDeclInfo),
		structDefs:            make(map[parse.Node]map[string]int),
		globalVars:            globals,
		localVars:             make(map[parse.Node]map[string]localVarInfo),
		properties:            make(map[*parse.ObjectLiteral]*propertyInfo),
		patterns:              patterns,
		patternNamespaces:     patternNamespaces,
		currentModule:         importedModule,
		chunk:                 importedModule.MainChunk,
		moduleImportStatement: node,
		store:                 make(map[parse.Node]any),
		data: &StaticCheckData{
			fnData:                                 map[*parse.FunctionExpression]*FunctionStaticData{},
			mappingData:                            map[*parse.MappingExpression]*MappingStaticData{},
			firstForbiddenPosForGlobalElementDecls: c.data.firstForbiddenPosForGlobalElementDecls,
			functionsToDeclareEarly:                c.data.functionsToDeclareEarly,
		},
	}

	chunkChecker.precheckTopLevelStatements(importedModuleNode)

	err := chunkChecker.check(importedModuleNode)
	if err != nil {
		panic(err)
	}

	if len(chunkChecker.data.errors) != 0 {
		c.data.errors = append(c.data.errors, chunkChecker.data.errors...)
	}

	if len(chunkChecker.data.warnings) != 0 {
		c.data.warnings = append(c.data.warnings, chunkChecker.data.warnings...)
	}

	if v, ok := chunkChecker.store[importedModuleNode]; ok {
		panic(fmt.Errorf("data stored for included chunk %#v : %#v", importedModuleNode, v))
	}
	return parse.ContinueTraversal
}

func (c *checker) checkGlobalConstDecls(node *parse.GlobalConstantDeclarations, parent, closestModule parse.Node) parse.TraversalAction {
	globalVars := c.getModGlobalVars(closestModule)

	inIncludedChunk := c.chunk.Node.IncludableChunkDesc != nil

	for _, decl := range node.Declarations {
		ident, ok := decl.Left.(*parse.IdentifierLiteral)
		if !ok {
			continue
		}
		name := ident.Name

		_, alreadyUsed := globalVars[name]
		if alreadyUsed {
			c.addError(decl, text.FmtInvalidConstDeclGlobalAlreadyDeclared(name))
			return parse.ContinueTraversal
		}

		globalVars[name] = globalVarInfo{isConst: true}

		//Check that there are not forbidden node types.
		parse.Walk(decl.Right, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
			switch n := node.(type) {
			case
				//variables
				*parse.Variable, *parse.IdentifierLiteral,

				*parse.BinaryExpression, *parse.UnaryExpression, *parse.URLExpression,
				parse.SimpleValueLiteral, *parse.IntegerRangeLiteral, *parse.FloatRangeLiteral,

				//immutable data structure literals
				*parse.RecordLiteral, *parse.ObjectProperty, *parse.TupleLiteral, *parse.TreedataLiteral,
				*parse.TreedataEntry, *parse.TreedataPair,

				//member-like expressions
				*parse.MemberExpression, *parse.IdentifierMemberExpression, *parse.DoubleColonExpression,
				*parse.IndexExpression, *parse.SliceExpression,

				//patterns
				*parse.PatternIdentifierLiteral,
				*parse.ObjectPatternLiteral, *parse.ObjectPatternProperty, *parse.RecordPatternLiteral,
				*parse.ListPatternLiteral, *parse.TuplePatternLiteral,
				*parse.FunctionPatternExpression,
				*parse.PatternNamespaceIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
				*parse.OptionPatternLiteral, *parse.OptionalPatternExpression,
				*parse.ComplexStringPatternPiece, *parse.PatternPieceElement, *parse.PatternGroupName,
				*parse.PatternUnion,
				*parse.PatternCallExpression:
				//ok
			case *parse.CallExpression:
				if inIncludedChunk {
					c.addError(n.Callee, text.CALL_EXPRS_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS_OF_INCLUDABLE_FILES)
					return parse.Prune, nil
				}

				switch callee := n.Callee.(type) {
				case *parse.IdentifierLiteral:
					if !slices.Contains(USABLE_GLOBALS_IN_PREINIT, callee.Name) {
						c.addError(n.Callee, text.A_LIMITED_NUMBER_OF_BUILTINS_ARE_ALLOWED_TO_BE_CALLED_IN_GLOBAL_CONST_DECLS)
						return parse.Prune, nil
					}
				case *parse.MemberExpression, *parse.IdentifierMemberExpression:
				default:
					c.addError(n, text.CALLED_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS)
					return parse.Prune, nil
				}
			default:
				c.addError(n, text.FmtFollowingNodeTypeNotAllowedInGlobalConstantDeclarations(n))
				return parse.Prune, nil
			}
			return parse.ContinueTraversal, nil
		}, nil)

	}

	return parse.ContinueTraversal
}

func (c *checker) checkLocalVarDecls(node *parse.LocalVariableDeclarations, scopeNode, closestModule parse.Node) parse.TraversalAction {
	localVars := c.getLocalVarsInScope(scopeNode)

	for _, decl := range node.Declarations {
		ident, ok := decl.Left.(*parse.IdentifierLiteral)
		if !ok { //invalid
			continue
		}
		name := ident.Name

		globalVariables := c.getModGlobalVars(closestModule)

		if _, alreadyDefined := globalVariables[name]; alreadyDefined {
			c.addError(decl, text.FmtCannotShadowGlobalVariable(name))
			return parse.ContinueTraversal
		}

		_, alreadyUsed := localVars[name]
		if alreadyUsed {
			c.addError(decl, text.FmtInvalidLocalVarDeclAlreadyDeclared(name))
			return parse.ContinueTraversal
		}
		localVars[name] = localVarInfo{}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkGlobalVarDecls(node *parse.GlobalVariableDeclarations, parentNode, scopeNode, closestModule parse.Node) parse.TraversalAction {
	globalVars := c.getModGlobalVars(closestModule)

	//Check the declarations are not misplaced.

	if !SamePointer(parentNode, closestModule) {
		c.addError(node, text.MISPLACED_GLOBAL_VAR_DECLS_TOP_LEVEL_STMT)
		return parse.Prune
	}

	firstForbiddenPos := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
	if firstForbiddenPos != 0 && node.Base().Span.Start >= firstForbiddenPos {
		c.addError(node, text.MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN)
		return parse.Prune
	}

	//Check each declaration.

	for _, decl := range node.Declarations {
		ident, ok := decl.Left.(*parse.IdentifierLiteral)
		if !ok { //invalid
			continue
		}
		name := ident.Name

		localVariables := c.getLocalVarsInScope(scopeNode)

		if _, alreadyDefined := localVariables[name]; alreadyDefined {
			c.addError(decl, text.FmtCannotShadowLocalVariable(name))
			return parse.ContinueTraversal
		}

		_, alreadyUsed := globalVars[name]
		if alreadyUsed {

			fnDecls := c.getModFunctionDecls(closestModule)
			_, isFunc := fnDecls[name]

			msg := ""
			if isFunc {
				msg = text.FmtInvalidAssignmentNameIsFuncName(name)
			} else {
				msg = text.FmtInvalidGlobalVarDeclAlreadyDeclared(name)
			}

			c.addError(decl, msg)
			return parse.ContinueTraversal
		}
		globalVars[name] = globalVarInfo{}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkAssignment(node parse.Node, scopeNode, closestModule parse.Node) parse.TraversalAction {
	var names []string

	if assignment, ok := node.(*parse.Assignment); ok {

		switch left := assignment.Left.(type) {
		case *parse.Variable:

			if left.Name == "" { //$
				c.addError(node, text.INVALID_ASSIGNMENT_ANONYMOUS_VAR_CANNOT_BE_ASSIGNED)
				return parse.ContinueTraversal
			}

			globalVariables := c.getModGlobalVars(closestModule)

			if _, isGlobal := globalVariables[left.Name]; isGlobal {
				if c.isDeclaredFunctionName(left.Name, closestModule) {
					c.addError(node, text.FmtInvalidAssignmentNameIsFuncName(left.Name))
				} else {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
				}
				return parse.ContinueTraversal
			}

			//Local variable

			localVars := c.getLocalVarsInScope(scopeNode)

			_, alreadyDefined := localVars[left.Name]

			if !alreadyDefined && assignment.Operator != parse.Assign {
				c.addError(node, text.FmtInvalidVariableAssignmentVarDoesNotExist(left.Name))
			}

			names = append(names, left.Name)
		case *parse.IdentifierLiteral:
			globalVariables := c.getModGlobalVars(closestModule)

			if _, alreadyDefined := globalVariables[left.Name]; alreadyDefined {
				if c.isDeclaredFunctionName(left.Name, closestModule) {
					c.addError(node, text.FmtInvalidAssignmentNameIsFuncName(left.Name))
				} else {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
				}
				return parse.ContinueTraversal
			}

			localVars := c.getLocalVarsInScope(scopeNode)

			_, alreadyDefined := localVars[left.Name]
			if !alreadyDefined && assignment.Operator != parse.Assign {
				c.addError(node, text.FmtInvalidVariableAssignmentVarDoesNotExist(left.Name))
			}

			if !alreadyDefined {
				globalVariables := c.getModGlobalVars(closestModule)

				if _, isGlobal := globalVariables[left.Name]; isGlobal {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
					return parse.ContinueTraversal
				}
			}

			names = append(names, left.Name)
		case *parse.IdentifierMemberExpression:

			for _, ident := range left.PropertyNames {
				if parse.IsMetadataKey(ident.Name) {
					c.addError(node, text.FmtInvalidMemberAssignmentCannotModifyMetaProperty(ident.Name))
				}
			}
		case *parse.MemberExpression:
			curr := left
			var ok bool
			for {
				if parse.IsMetadataKey(curr.PropertyName.Name) {
					c.addError(node, text.FmtInvalidMemberAssignmentCannotModifyMetaProperty(curr.PropertyName.Name))
					break
				}
				if curr, ok = curr.Left.(*parse.MemberExpression); !ok {
					break
				}
			}
		case *parse.SliceExpression:
			if assignment.Operator != parse.Assign {
				c.addError(node, text.INVALID_ASSIGNMENT_EQUAL_ONLY_SUPPORTED_ASSIGNMENT_OPERATOR_FOR_SLICE_EXPRS)
			}
		}
	} else {
		assignment := node.(*parse.MultiAssignment)

		for _, variable := range assignment.Variables {
			ident, ok := variable.(*parse.IdentifierLiteral)
			if !ok { //invalid
				continue
			}
			name := ident.Name

			globalVariables := c.getModGlobalVars(closestModule)

			if _, alreadyDefined := globalVariables[name]; alreadyDefined {
				if c.isDeclaredFunctionName(name, closestModule) {
					c.addError(node, text.FmtInvalidAssignmentNameIsFuncName(name))
				} else {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
				}
			} else {
				names = append(names, name)
			}
		}
	}

	for _, name := range names {
		variables := c.getLocalVarsInScope(scopeNode)
		variables[name] = localVarInfo{}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkForStmt(node *parse.ForStatement, scopeNode, closestModule parse.Node) parse.TraversalAction {
	localVariablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	localVars := c.getLocalVarsInScope(scopeNode)
	globalVars := c.getModGlobalVars(closestModule)

	c.store[node] = localVariablesBeforeStmt

	if node.KeyIndexIdent != nil {
		name := node.KeyIndexIdent.Name

		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(node.KeyIndexIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(node.KeyIndexIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}

	if node.ValueElemIdent != nil {
		name := node.ValueElemIdent.Name

		if _, alreadyDefined := localVars[name]; alreadyDefined {
			c.addError(node.ValueElemIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(node.ValueElemIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkForExpression(node *parse.ForExpression, scopeNode, closestModule parse.Node) parse.TraversalAction {
	localVariablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	localVars := c.getLocalVarsInScope(scopeNode)
	globalVars := c.getModGlobalVars(closestModule)

	c.store[node] = localVariablesBeforeStmt

	if node.KeyIndexIdent != nil {
		name := node.KeyIndexIdent.Name

		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(node.KeyIndexIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(node.KeyIndexIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}

	if node.ValueElemIdent != nil {
		name := node.ValueElemIdent.Name

		if _, alreadyDefined := localVars[name]; alreadyDefined {
			c.addError(node.ValueElemIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(node.ValueElemIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkWalkStmt(node *parse.WalkStatement, scopeNode, closestModule parse.Node) parse.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	localVars := c.getLocalVarsInScope(scopeNode)
	globalVars := c.getModGlobalVars(closestModule)

	c.store[node] = variablesBeforeStmt

	if node.EntryIdent != nil {
		name := node.EntryIdent.Name
		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(node.EntryIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(node.EntryIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkReadonlyPatternExpr(node *parse.ReadonlyPatternExpression, parent parse.Node) parse.TraversalAction {
	ok := false
	switch p := parent.(type) {
	case *parse.FunctionParameter:
		ok = p.Type == node
	default:
	}

	if !ok {
		c.addError(node, text.MISPLACED_READONLY_PATTERN_EXPRESSION)
	}

	return parse.ContinueTraversal
}

func (c *checker) precheckTopLevelFuncDecl(stmt *parse.FunctionDeclaration, module parse.Node) {
	globalVars := c.getModGlobalVars(module)
	fnDecls := c.getModFunctionDecls(module)

	_, alreadyDeclared := fnDecls[stmt.Name.Name]
	if alreadyDeclared {
		c.addError(stmt, text.FmtInvalidFnDeclAlreadyDeclared(stmt.Name.Name))
		return
	}

	//Pre-declare the functions that don't capture locals.
	if len(stmt.Function.CaptureList) == 0 {
		globalVars[stmt.Name.Name] = globalVarInfo{isConst: true, fnExpr: stmt.Function}

		fns := c.data.functionsToDeclareEarly[module]
		if fns == nil {
			fns = new([]*parse.FunctionDeclaration)
			c.data.functionsToDeclareEarly[module] = fns
		}
		*fns = append(*fns, stmt)

		info := &fnDeclInfo{node: stmt, module: module}
		fnDecls[stmt.Name.Name] = info
	}
}

func (c *checker) checkCallExpression(node *parse.CallExpression, scopeNode, closestModule parse.Node) parse.TraversalAction {

	return parse.ContinueTraversal
}

func (c *checker) checkFuncDecl(node *parse.FunctionDeclaration, parent, closestModule parse.Node) parse.TraversalAction {
	switch parent.(type) {
	case *parse.Chunk, *parse.EmbeddedModule: //valid location
		fnDecls := c.getModFunctionDecls(closestModule)
		globalVars := c.getModGlobalVars(closestModule)
		localVars := c.getLocalVarsInScope(closestModule)

		if len(node.Function.CaptureList) == 0 {
			_, ok := fnDecls[node.Name.Name]
			if !ok {
				c.addError(node, "function has no been pre-checked by the static checker")
				return parse.ContinueTraversal
			}

			_, ok = c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
			if !ok {
				c.data.firstForbiddenPosForGlobalElementDecls[closestModule] = node.Span.Start
			}
		} else {
			declInfo := &fnDeclInfo{node: node, module: closestModule}

			for _, captured := range node.Function.CaptureList {
				if ident, ok := captured.(*parse.IdentifierLiteral); ok {
					_, isLocal := localVars[ident.Name]
					_, isGlobal := globalVars[ident.Name]

					if isLocal {
						declInfo.capturedLocals = append(declInfo.capturedLocals, ident.Name)
					} else if !isGlobal {
						c.addError(node, text.FmtInvalidOrMisplacedFnDeclShouldBeAfterCapturedVarDeclaration(ident.Name))
					}
				}
			}

			fnDecls[node.Name.Name] = declInfo
			globalVars[node.Name.Name] = globalVarInfo{isConst: true, fnExpr: node.Function}
		}

	case *parse.StructBody:
		//struct method
	default:
		c.addError(node, text.INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT)
		return parse.ContinueTraversal
	}

	return parse.ContinueTraversal
}

func (c *checker) checkFuncExpr(node *parse.FunctionExpression, closestModule parse.Node, ancestorChain []parse.Node) parse.TraversalAction {
	fnLocalVars := c.getLocalVarsInScope(node)

	//we check that the captured variable exists & is a local
	for _, e := range node.CaptureList {
		ident, ok := e.(*parse.IdentifierLiteral)
		if !ok { //invalid
			continue
		}
		name := ident.Name

		if !c.varExists(name, ancestorChain) {
			c.addError(e, text.FmtVarIsNotDeclared(name))
		} else if c.doGlobalVarExist(name, closestModule) {
			c.addError(node, text.FmtCannotPassGlobalToFunction(name))
		}

		fnLocalVars[name] = localVarInfo{}
	}

	for _, p := range node.Parameters {
		name := p.Var.Name

		globalVariables := c.getModGlobalVars(closestModule)

		if _, alreadyDefined := globalVariables[name]; alreadyDefined {
			c.addError(p, text.FmtParameterCannotShadowGlobalVariable(name))
			return parse.ContinueTraversal
		}

		fnLocalVars[name] = localVarInfo{}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkFuncPatternExpr(node *parse.FunctionPatternExpression, closestModule parse.Node) parse.TraversalAction {
	fnLocalVars := c.getLocalVarsInScope(node)

	for _, p := range node.Parameters {
		if p.Var == nil {
			continue
		}

		name := p.Var.Name

		globalVariables := c.getModGlobalVars(closestModule)

		if _, alreadyDefined := globalVariables[name]; alreadyDefined {
			c.addError(p, text.FmtParameterCannotShadowGlobalVariable(name))
			return parse.ContinueTraversal
		}

		fnLocalVars[name] = localVarInfo{}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkReturnStatement(node *parse.ReturnStatement, ancestorChain []parse.Node) parse.TraversalAction {

	//Go up until we find a module, function body, or an invalid node.
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		ancestor := ancestorChain[i]

		if parse.IsTheTopLevel(ancestor) {
			return parse.ContinueTraversal //ok
		}

		switch ancestor.(type) {
		case *parse.FunctionExpression:
			return parse.ContinueTraversal //ok
		case *parse.IfStatement, *parse.ForStatement, *parse.WalkStatement,
			*parse.SwitchStatement, *parse.SwitchStatementCase, *parse.DefaultCaseWithBlock,
			*parse.MatchStatement, *parse.MatchStatementCase,
			*parse.SynchronizedBlockStatement, *parse.Block:
		default:
			c.addError(node, text.MISPLACED_RETURN_STATEMENT)
			return parse.ContinueTraversal //ok
		}
	}

	c.addError(node, text.MISPLACED_RETURN_STATEMENT)
	return parse.ContinueTraversal
}

func (c *checker) checkCoyieldStmt(node *parse.CoyieldStatement, ancestorChain []parse.Node) parse.TraversalAction {
	ok := c.checkInput.Module != nil && c.checkInput.Module.IsEmbedded()

	for i := len(ancestorChain) - 1; i >= 0; i-- {
		if !parse.IsScopeContainerNode(ancestorChain[i]) {
			continue
		}

		if ok && ancestorChain[i] != c.checkInput.Node {
			ok = false
			break
		}

		switch ancestorChain[i].(type) {
		case *parse.EmbeddedModule:
			ok = true
		}
		break
	}

	if !ok {
		c.addError(node, text.MISPLACE_COYIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES)
	}
	return parse.ContinueTraversal
}

func (c *checker) checkBreakContinueStmt(node parse.Node, ancestorChain []parse.Node) parse.TraversalAction {
	iterativeStmtIndex := -1

	//we search for the last iterative statement or expression in the ancestor chain
loop0:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *parse.ForStatement, *parse.WalkStatement,
			*parse.ForExpression:
			iterativeStmtIndex = i
			break loop0
		}
	}

	if iterativeStmtIndex < 0 {
		c.addError(node, text.BREAK_AND_CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT)
		return parse.ContinueTraversal
	}

	for i := iterativeStmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *parse.IfStatement, *parse.SwitchStatement, *parse.SwitchStatementCase,
			*parse.MatchStatementCase, *parse.MatchStatement, *parse.Block:
		default:
			c.addError(node, text.BREAK_AND_CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT)
			return parse.ContinueTraversal
		}

	}
	return parse.ContinueTraversal
}

func (c *checker) checkYieldStmt(node parse.Node, ancestorChain []parse.Node) parse.TraversalAction {
	iterativeStmtIndex := -1

	//we search for the last iterative statement or expression in the ancestor chain
loop0:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *parse.ForExpression:
			iterativeStmtIndex = i
			break loop0
		}
	}

	if iterativeStmtIndex < 0 {
		c.addError(node, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_EXPR)
		return parse.ContinueTraversal
	}

	for i := iterativeStmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *parse.IfStatement, *parse.SwitchStatement, *parse.SwitchStatementCase,
			*parse.MatchStatementCase, *parse.MatchStatement, *parse.Block,
			*parse.ForStatement, *parse.WalkStatement:
		default:
			c.addError(node, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_EXPR)
			return parse.ContinueTraversal
		}

	}
	return parse.ContinueTraversal
}

func (c *checker) checkPruneStmt(node *parse.PruneStatement, ancestorChain []parse.Node) parse.TraversalAction {
	walkStmtIndex := -1
	//we search for the last walk statement in the ancestor chain
loop1:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *parse.WalkStatement:
			walkStmtIndex = i
			break loop1
		}
	}

	if walkStmtIndex < 0 {
		c.addError(node, text.PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMT)
		return parse.ContinueTraversal
	}

	for i := walkStmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *parse.IfStatement, *parse.SwitchStatement, *parse.MatchStatement, *parse.Block, *parse.ForStatement:
		default:
			c.addError(node, text.PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMT)
			return parse.ContinueTraversal
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkSwitchStatement(node *parse.SwitchStatement, scopeNode, closestModule parse.Node) parse.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	c.store[node] = variablesBeforeStmt

	//default case uniqueness is checked by the parser.

	return parse.ContinueTraversal
}

func (c *checker) checkMatchStatement(node *parse.MatchStatement, scopeNode, closestModule parse.Node) parse.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	c.store[node] = variablesBeforeStmt

	//default case uniqueness is checked by the parser.

	return parse.ContinueTraversal
}

func (c *checker) checkMatchCase(node *parse.MatchStatementCase, scopeNode, closestModule parse.Node) parse.TraversalAction {

	//define the variables named after groups if the literal is used as a case in a match statement

	if node.GroupMatchingVariable == nil {
		return parse.ContinueTraversal
	}

	variable := node.GroupMatchingVariable.(*parse.IdentifierLiteral)

	if _, alreadyDefined := c.getModGlobalVars(closestModule)[variable.Name]; alreadyDefined {
		c.addError(variable, text.FmtCannotShadowGlobalVariable(variable.Name))
		return parse.ContinueTraversal
	}

	localVars := c.getLocalVarsInScope(scopeNode)

	if info, alreadyDefined := localVars[variable.Name]; alreadyDefined && info != (localVarInfo{isGroupMatchingVar: true}) {
		c.addError(variable, text.FmtCannotShadowLocalVariable(variable.Name))
		return parse.ContinueTraversal
	}

	localVars[variable.Name] = localVarInfo{isGroupMatchingVar: true}

	return parse.ContinueTraversal
}

func (c *checker) checkVariable(node *parse.Variable, scopeNode parse.Node, ancestorChain []parse.Node, closestModule parse.Node) parse.TraversalAction {
	if len(node.Name) > MAX_NAME_BYTE_LEN {
		c.addError(node, text.FmtNameIsTooLong(node.Name))
		return parse.ContinueTraversal
	}

	if node.Name == "" {
		return parse.ContinueTraversal
	}

	globalVars := c.getModGlobalVars(closestModule)

	if globalVarInfo, exists := globalVars[node.Name]; exists {
		parent := ancestorChain[len(ancestorChain)-1]

		if len(node.Name) > MAX_NAME_BYTE_LEN {
			c.addError(node, text.FmtNameIsTooLong(node.Name))
			return parse.ContinueTraversal
		}

		if _, isAssignment := parent.(*parse.Assignment); isAssignment {
			return parse.ContinueTraversal
		}

		if _, isLazyExpr := scopeNode.(*parse.LazyExpression); isLazyExpr {
			return parse.ContinueTraversal
		}

		if _, ok := scopeNode.(*parse.ExtendStatement); ok {
			c.addError(node, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES)
			return parse.ContinueTraversal
		}

		if _, ok := scopeNode.(*parse.StructDefinition); ok {
			c.addError(node, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS)
			return parse.ContinueTraversal
		}

		if !exists {
			c.addError(node, text.FmtGlobalVarIsNotDeclared(node.Name))
			return parse.ContinueTraversal
		}

		fnDecls := c.getModFunctionDecls(closestModule)

		if fnDecls[node.Name] != nil && fnDecls[node.Name].module == closestModule {
			//If the global variable is a function then no global declarations (patterns, global variables)
			//should be located after this reference.

			if _, ok := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]; !ok {
				topLevelStmt, ok := parse.FindClosestTopLevelStatement(node, ancestorChain)
				if !ok {
					panic(ErrUnreachable)
				}
				c.data.firstForbiddenPosForGlobalElementDecls[closestModule] = topLevelStmt.Base().Span.Start
			}
		}

		switch scope := scopeNode.(type) {
		case *parse.FunctionExpression:
			c.data.addFnCapturedGlobal(scope, node.Name, &globalVarInfo)
		case *parse.EmbeddedModule:
			embeddedModIndex := -1
			for i, ancestor := range ancestorChain {
				if ancestor == scope {
					embeddedModIndex = i
					break
				}
			}

			if embeddedModIndex < 0 {
				panic(ErrUnreachable)
			}

			if embeddedModIndex == 0 {
				break
			}

			_, ok := ancestorChain[embeddedModIndex-1].(*parse.LifetimejobExpression)
			if ok {
				parentScopeNode := findClosestScopeContainerNode(ancestorChain[:embeddedModIndex-1])
				if fnExpr, ok := parentScopeNode.(*parse.FunctionExpression); ok {
					c.data.addFnCapturedGlobal(fnExpr, node.Name, &globalVarInfo)
				}
			}
		case *parse.DynamicMappingEntry, *parse.StaticMappingEntry:
			mappingExpr := findClosest[*parse.MappingExpression](ancestorChain)
			c.data.addMappingCapturedGlobal(mappingExpr, node.Name)
		}

		return parse.ContinueTraversal
	}

	//Local variable

	if _, isLazyExpr := scopeNode.(*parse.LazyExpression); isLazyExpr {
		return parse.ContinueTraversal
	}

	if _, ok := scopeNode.(*parse.ExtendStatement); ok {
		c.addError(node, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES)
		return parse.ContinueTraversal
	}

	if _, ok := scopeNode.(*parse.StructDefinition); ok {
		c.addError(node, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS)
		return parse.ContinueTraversal
	}

	variables := c.getLocalVarsInScope(scopeNode)
	_, exist := variables[node.Name]

	if !exist {
		c.addError(node, text.FmtVarIsNotDeclared(node.Name))
		return parse.ContinueTraversal
	}

	return parse.ContinueTraversal
}

func (c *checker) checkIdentifier(node *parse.IdentifierLiteral, parent, scopeNode, closestModule parse.Node, ancestorChain []parse.Node) parse.TraversalAction {

	if len(node.Name) > MAX_NAME_BYTE_LEN {
		c.addError(node, text.FmtNameIsTooLong(node.Name))
		return parse.ContinueTraversal
	}

	if _, ok := scopeNode.(*parse.LazyExpression); ok {
		return parse.ContinueTraversal
	}

	//we check the parent to know if the identifier refers to a variable
	switch p := parent.(type) {
	case *parse.CallExpression:
		if p.CommandLikeSyntax && !node.IncludedIn(p.Callee) {
			return parse.ContinueTraversal

		}
	case *parse.FunctionDeclaration:
		return parse.ContinueTraversal
	case *parse.ObjectProperty:
		if p.Key == node {
			return parse.ContinueTraversal
		}
	case *parse.ObjectPatternProperty:
		if p.Key == node {
			return parse.ContinueTraversal

		}
	case *parse.ObjectMetaProperty:
		if p.Key == node {
			return parse.ContinueTraversal

		}
	case *parse.StructDefinition:
		if p.Name == node {
			return parse.ContinueTraversal

		}

	case *parse.StructFieldDefinition:
		if p.Name == node {
			return parse.ContinueTraversal

		}
	case *parse.NewExpression:
		if p.Type == node {
			return parse.ContinueTraversal

		}
	case *parse.StructFieldInitialization:
		if p.Name == node {
			return parse.ContinueTraversal

		}
	case *parse.IdentifierMemberExpression:
		if node != p.Left {
			return parse.ContinueTraversal

		}
	case *parse.DynamicMemberExpression:
		if node != p.Left {
			return parse.ContinueTraversal

		}
	case *parse.PatternNamespaceMemberExpression:
		return parse.ContinueTraversal

	case *parse.DoubleColonExpression:
		if node == p.Element {
			return parse.ContinueTraversal

		}
	case *parse.DynamicMappingEntry:
		if node == p.KeyVar || node == p.GroupMatchingVariable {
			return parse.ContinueTraversal

		}
	case *parse.ForStatement, *parse.ForExpression, *parse.WalkStatement,
		*parse.ObjectLiteral, *parse.MemberExpression, *parse.QuantityLiteral, *parse.RateLiteral,

		*parse.KeyListExpression:
		return parse.ContinueTraversal

	case *parse.XMLOpeningElement:
		if node == p.Name {
			return parse.ContinueTraversal

		}
	case *parse.XMLClosingElement:
		if node == p.Name {
			return parse.ContinueTraversal

		}
	case *parse.XMLAttribute:
		if node == p.Name {
			return parse.ContinueTraversal

		}
	}

	if _, ok := scopeNode.(*parse.ExtendStatement); ok {
		c.addError(node, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES)
		return parse.ContinueTraversal
	}

	if _, ok := scopeNode.(*parse.StructDefinition); ok {
		c.addError(node, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS)
		return parse.ContinueTraversal
	}

	if !c.varExists(node.Name, ancestorChain) {
		if node.Name == "const" {
			c.addError(node, text.VAR_CONST_NOT_DECLARED_IF_YOU_MEANT_TO_DECLARE_CONSTANTS_GLOBAL_CONST_DECLS_ONLY_SUPPORTED_AT_THE_START_OF_THE_MODULE)
		} else {
			c.addError(node, text.FmtVarIsNotDeclared(node.Name))
		}
		return parse.ContinueTraversal
	}

	// if the variable is a global in a function expression or in a mapping entry we capture it
	if c.doGlobalVarExist(node.Name, closestModule) {
		fnDecls := c.getModFunctionDecls(closestModule)

		if fnDecls[node.Name] != nil && fnDecls[node.Name].module == closestModule {

			//If the identifier references a  function then no global declarations (patterns, global variables)
			//should be located after this reference.

			if _, ok := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]; !ok {
				topLevelStmt, ok := parse.FindClosestTopLevelStatement(node, ancestorChain)
				if !ok {
					panic(ErrUnreachable)
				}
				c.data.firstForbiddenPosForGlobalElementDecls[closestModule] = topLevelStmt.Base().Span.Start
			}
		}

		globalVarInfo := c.getModGlobalVars(closestModule)[node.Name]

		switch scope := scopeNode.(type) {
		case *parse.FunctionExpression:
			c.data.addFnCapturedGlobal(scope, node.Name, &globalVarInfo)
		case *parse.EmbeddedModule:
			embeddedModIndex := -1
			for i, ancestor := range ancestorChain {
				if ancestor == scope {
					embeddedModIndex = i
					break
				}
			}

			if embeddedModIndex < 0 {
				panic(ErrUnreachable)
			}

			if embeddedModIndex == 0 {
				break
			}

			_, ok := ancestorChain[embeddedModIndex-1].(*parse.LifetimejobExpression)
			if ok {
				parentScopeNode := findClosestScopeContainerNode(ancestorChain[:embeddedModIndex-1])
				if fnExpr, ok := parentScopeNode.(*parse.FunctionExpression); ok {
					c.data.addFnCapturedGlobal(fnExpr, node.Name, &globalVarInfo)
				}
			}
		case *parse.DynamicMappingEntry, *parse.StaticMappingEntry:
			mappingExpr := findClosest[*parse.MappingExpression](ancestorChain)
			c.data.addMappingCapturedGlobal(mappingExpr, node.Name)
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkSelfExprAndSendValExpr(node, parent parse.Node, ancestorChain []parse.Node) parse.TraversalAction {
	isSelfExpr := true

	var objectLiteral *parse.ObjectLiteral
	var misplacementErr = text.SELF_ACCESSIBILITY_EXPLANATION
	isInExtensionMethod := false
	inReceptionHandler := false
	isSelfInStructMethod := false

	switch node.(type) {
	case *parse.SendValueExpression:
		isSelfExpr = false
		misplacementErr = text.MISPLACED_SENDVAL_EXPR
	}

loop:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		if !parse.IsScopeContainerNode(ancestorChain[i]) {
			continue
		}

		switch a := ancestorChain[i].(type) {
		case *parse.InitializationBlock:
			switch i {
			case 0:
			default:
				switch ancestorChain[i-1].(type) {
				case *parse.ObjectMetaProperty:
					if i == 1 {
						c.addError(node, text.CANNOT_CHECK_OBJECT_METAPROP_WITHOUT_PARENT)
						break
					}
				}

				switch ancestor := ancestorChain[i-2].(type) {
				case *parse.ObjectLiteral:
					objectLiteral = ancestor
				default:
				}
			}
			break loop
		case *parse.FunctionExpression:
			//Determine if the function is the method of an object, extension or struct.

			j := i - 1

			if j == -1 {
				break loop
			}

			maybeInReceptionHandler := false

			if _, ok := ancestorChain[j].(*parse.ReceptionHandlerExpression); ok {
				j--
				maybeInReceptionHandler = true
			}

			switch ancestorChain[j].(type) {
			case *parse.ObjectProperty:
				if j == 0 {
					c.addError(node, text.CANNOT_CHECK_OBJECT_PROP_WITHOUT_PARENT)
					break loop
				}
				j--

				objLit, ok := ancestorChain[j].(*parse.ObjectLiteral)
				if ok && j-1 >= 0 {

					if maybeInReceptionHandler {
						inReceptionHandler = true
						objectLiteral = objLit
					}

					isInExtensionMethod =
						utils.Implements[*parse.ExtendStatement](ancestorChain[j-1]) &&
							ancestorChain[j-1].(*parse.ExtendStatement).Extension == objLit

					if isInExtensionMethod {
						objectLiteral = objLit
					}
				}
			case *parse.FunctionDeclaration:
				if j == 0 {
					c.addError(node, text.CANNOT_CHECK_STRUCT_METHOD_DEF_WITHOUT_PARENT)
					break loop
				}
				_, ok := ancestorChain[j-1].(*parse.StructBody)
				isSelfInStructMethod = ok && isSelfExpr
			}

			break loop
		case *parse.EmbeddedModule: //ok if lifetime job
			if i == 0 || !utils.Implements[*parse.LifetimejobExpression](ancestorChain[i-1]) {
				c.addError(node, misplacementErr)
			}
			return parse.ContinueTraversal
		case *parse.Chunk:
			if c.currentModule != nil && c.currentModule.ModuleKind == LifetimeJobModule { // ok
				return parse.ContinueTraversal
			}
		case *parse.ExtendStatement:
			if isSelfExpr && node.Base().IncludedIn(a.Extension) { //ok
				return parse.ContinueTraversal
			}
		}
	}

	if !isSelfInStructMethod {
		if objectLiteral == nil {
			c.addError(node, misplacementErr)
			return parse.ContinueTraversal
		}

		_ = inReceptionHandler

		propInfo := c.getPropertyInfo(objectLiteral)

		switch p := parent.(type) {
		case *parse.MemberExpression:
			if !propInfo.known[p.PropertyName.Name] && !isInExtensionMethod {
				c.addError(p, text.FmtObjectDoesNotHaveProp(p.PropertyName.Name))
			}
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkPatternDef(node *parse.PatternDefinition, parent, closestModule parse.Node, inPreinitBlock bool) parse.TraversalAction {
	switch parent.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		if !inPreinitBlock {
			c.addError(node, text.MISPLACED_PATTERN_DEF_NOT_TOP_LEVEL_STMT)
			return parse.Prune
		}
	}

	firstForbiddenPos := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
	if firstForbiddenPos != 0 && node.Base().Span.Start >= firstForbiddenPos {
		c.addError(node, text.MISPLACED_PATTERN_DEF_AFTER_FN_DECL_OR_REF_TO_FN)
		return parse.Prune
	}

	patternName, ok := node.PatternName()
	if ok {
		patterns := c.getModPatterns(closestModule)

		if _, alreadyDefined := patterns[patternName]; alreadyDefined && !inPreinitBlock {
			c.addError(node, text.FmtPatternAlreadyDeclared(patternName))
		} else {
			patterns[patternName] = 0
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkPatternNamespaceDefinition(node *parse.PatternNamespaceDefinition, parent, closestModule parse.Node, inPreinitBlock bool) parse.TraversalAction {
	switch parent.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		if !inPreinitBlock {
			c.addError(node, text.MISPLACED_PATTERN_NS_DEF_NOT_TOP_LEVEL_STMT)
			return parse.Prune
		}
	}

	firstForbiddenPos := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
	if firstForbiddenPos != 0 && node.Base().Span.Start >= firstForbiddenPos {
		c.addError(node, text.MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN)
		return parse.Prune
	}

	namespaceName, ok := node.NamespaceName()
	if ok {
		namespaces := c.getModPatternNamespaces(closestModule)
		if _, alreadyDefined := namespaces[namespaceName]; alreadyDefined && !inPreinitBlock {
			c.addError(node, text.FmtPatternNamespaceAlreadyDeclared(namespaceName))
		} else {
			namespaces[namespaceName] = 0
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkPatternNamespaceIdentifier(node *parse.PatternNamespaceIdentifierLiteral, closestModule parse.Node) parse.TraversalAction {
	namespaceName := node.Name
	namespaces := c.getModPatternNamespaces(closestModule)

	if _, alreadyDefined := namespaces[namespaceName]; !alreadyDefined {
		c.addError(node, text.FmtPatternNamespaceIsNotDeclared(namespaceName))
	}

	return parse.ContinueTraversal
}

func (c *checker) checkPatternIdentifier(node *parse.PatternIdentifierLiteral, parent, closestModule parse.Node, ancestorChain []parse.Node) parse.TraversalAction {

	if _, ok := parent.(*parse.OtherPropsExpr); ok && node.Name == parse.NO_OTHERPROPS_PATTERN_NAME {
		return parse.ContinueTraversal

	}

	if def, ok := parent.(*parse.StructDefinition); ok && def.Name == node {
		return parse.ContinueTraversal

	}

	//Check if struct type.
	stuctDefs := c.getModStructDefs(closestModule)
	_, ok := stuctDefs[node.Name]
	if ok {
		//Check that the node is not misplaced.
		errMsg := ""
		switch parent := parent.(type) {
		case *parse.PointerType, *parse.StructFieldDefinition, *parse.NewExpression:
			//ok
		case *parse.FunctionParameter:
			errMsg = text.STRUCT_TYPES_NOT_ALLOWED_AS_PARAMETER_TYPES
		case *parse.FunctionExpression:
			if node == parent.ReturnType {
				errMsg = text.STRUCT_TYPES_NOT_ALLOWED_AS_RETURN_TYPES
			} else {
				errMsg = text.MISPLACED_STRUCT_TYPE_NAME
			}
		default:
			errMsg = text.MISPLACED_STRUCT_TYPE_NAME
		}

		if errMsg != "" {
			c.addError(node, errMsg)
		}

		return parse.ContinueTraversal

	}

	//Ignore the check if the pattern identifier refers to a pattern that is not yet defined.

	for _, a := range ancestorChain {
		if def, ok := a.(*parse.PatternDefinition); ok && def.IsLazy {
			return parse.ContinueTraversal
		}
	}

	//Check that the pattern is declared.

	name := node.Name
	patterns := c.getModPatterns(closestModule)
	if _, ok := patterns[name]; !ok {
		errMsg := ""
		switch parent.(type) {
		case *parse.PointerType, *parse.NewExpression:
			errMsg = text.FmtStructTypeIsNotDefined(name)
		default:
			errMsg = text.FmtPatternIsNotDeclared(name)
		}
		c.addError(node, errMsg)
	}
	return parse.ContinueTraversal
}

func (c *checker) checkRuntimeTypeCheckExpr(node *parse.RuntimeTypeCheckExpression, parent parse.Node) parse.TraversalAction {
	switch p := parent.(type) {
	case *parse.CallExpression:
		for _, arg := range p.Arguments {
			if node == arg {
				return parse.ContinueTraversal
			}
		}

		c.addError(node, text.MISPLACED_RUNTIME_TYPECHECK_EXPRESSION)
	default:
		c.addError(node, text.MISPLACED_RUNTIME_TYPECHECK_EXPRESSION)
	}
	return parse.ContinueTraversal
}

func (c *checker) checkNewExpr(node *parse.NewExpression) parse.TraversalAction {
	typ := node.Type
	switch typ.(type) {
	case *parse.PatternIdentifierLiteral:
		//ok, the identifier will be checked next
	//TODO: support slices
	case nil:
		return parse.ContinueTraversal
	default:
		c.addError(node.Type, text.A_STRUCT_TYPE_NAME_IS_EXPECTED)
		return parse.ContinueTraversal
	}
	return parse.ContinueTraversal
}

func (c *checker) checkStructInitLiteral(node *parse.StructInitializationLiteral) parse.TraversalAction {
	// look for duplicate field names
	fieldNames := make([]string, 0, len(node.Fields))

	for _, field := range node.Fields {
		fieldInit, ok := field.(*parse.StructFieldInitialization)
		if ok {
			name := fieldInit.Name.Name
			if slices.Contains(fieldNames, name) {
				c.addError(fieldInit.Name, text.FmtDuplicateFieldName(name))
			} else {
				fieldNames = append(fieldNames, name)
			}
		}
	}
	return parse.ContinueTraversal
}

func (c *checker) checkPointerType(node *parse.PointerType, parent parse.Node) parse.TraversalAction {
	patternIdent, ok := node.ValueType.(*parse.PatternIdentifierLiteral)
	if !ok {
		c.addError(node.ValueType, text.A_STRUCT_TYPE_IS_EXPECTED_AFTER_THE_STAR)
	} else {
		//Check that the node is not misplaced.
		switch parent := parent.(type) {
		case *parse.StructFieldDefinition, *parse.FunctionParameter:
			//ok
		case *parse.FunctionExpression:
			if node != parent.ReturnType {
				c.addError(node, text.MISPLACED_POINTER_TYPE)
			}
		case *parse.LocalVariableDeclaration:
			if node != parent.Type {
				c.addError(node, text.MISPLACED_POINTER_TYPE)
			}
		default:
			c.addError(node, text.MISPLACED_POINTER_TYPE)
		}

		if symbolic.IsNameOfBuiltinComptimeType(patternIdent.Name) {
			//do not check the pattern identifier.
			return parse.Prune
		}

	}
	return parse.ContinueTraversal
}

func (c *checker) checkTestSuiteExpr(node *parse.TestSuiteExpression, ancestorChain []parse.Node) parse.TraversalAction {
	hasSubsuiteStmt := false
	hasTestCaseStmt := false

	for _, stmt := range node.Module.Statements {
		switch stmt := stmt.(type) {
		case *parse.TestCaseExpression:
			if stmt.IsStatement {
				hasTestCaseStmt = true
			}
		case *parse.TestSuiteExpression:
			if stmt.IsStatement {
				hasSubsuiteStmt = true
			}
		}
	}

	if hasSubsuiteStmt && hasTestCaseStmt {
		for _, stmt := range node.Module.Statements {
			switch stmt := stmt.(type) {
			case *parse.TestCaseExpression:
				if stmt.IsStatement {
					c.addError(stmt, text.TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT)
				}
			case *parse.TestSuiteExpression:
				if stmt.IsStatement {
					hasSubsuiteStmt = true
				}
			}
		}
	}

	// check the statement is not in a testcase
	if node.IsStatement {

	search_test_case:
		for i := len(ancestorChain) - 1; i >= 0; i-- {
			switch ancestorChain[i].(type) {
			case *parse.EmbeddedModule:
				if i-1 <= 0 {
					break search_test_case
				}
				testCaseExpr, ok := ancestorChain[i-1].(*parse.TestCaseExpression)
				if ok && testCaseExpr.IsStatement {
					c.addError(node, text.TEST_SUITE_STMTS_NOT_ALLOWED_INSIDE_TEST_CASE_STMTS)
					break search_test_case
				}
			}
		}
	}

	return parse.ContinueTraversal
}

func (c *checker) checkTestCaseExpr(node *parse.TestCaseExpression, ancestorChain []parse.Node) parse.TraversalAction {
	inTestSuite := false

search_test_suite:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *parse.EmbeddedModule:
			if i-1 <= 0 {
				break search_test_suite
			}
			testSuiteExpr, ok := ancestorChain[i-1].(*parse.TestSuiteExpression)
			if ok {
				inTestSuite = testSuiteExpr.Module == ancestorChain[i]
				break search_test_suite
			}
		}
	}

	if !inTestSuite && node.IsStatement && (c.currentModule == nil || c.currentModule.ModuleKind != TestSuiteModule) {
		c.addError(node, text.TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES)
	}

	return parse.ContinueTraversal
}

func (c *checker) checkEmbeddedModule(node *parse.EmbeddedModule, parent, parentModule parse.Node, ancestorChain []parse.Node) parse.TraversalAction {
	globals := c.getModGlobalVars(node)
	patterns := c.getModPatterns(node)
	patternNamespaces := c.getModPatternNamespaces(node)

	parentModuleGlobals := c.getModGlobalVars(parentModule)
	parentModulePatterns := c.getModPatterns(parentModule)
	parentModulePatternNamespaces := c.getModPatternNamespaces(parentModule)

	switch parent.(type) {
	case *parse.TestSuiteExpression:
		//inherit globals
		for name, info := range parentModuleGlobals {
			if slices.Contains(globalnames.TEST_ITEM_NON_INHERITED_GLOBALS, name) {
				continue
			}
			globals[name] = info
		}

		//inherit patterns
		for name, info := range parentModulePatterns {
			patterns[name] = info
		}
		for name, info := range parentModulePatternNamespaces {
			patternNamespaces[name] = info
		}

	case *parse.TestCaseExpression:
		globals[globalnames.CURRENT_TEST] = globalVarInfo{isConst: true, isStartConstant: true}

		//inherit globals
		for name, info := range parentModuleGlobals {
			if slices.Contains(globalnames.TEST_ITEM_NON_INHERITED_GLOBALS, name) {
				continue
			}
			globals[name] = info
		}

		//inherit patterns
		for name, info := range parentModulePatterns {
			patterns[name] = info
		}
		for name, info := range parentModulePatternNamespaces {
			patternNamespaces[name] = info
		}
	}

	return parse.ContinueTraversal
}

// checkSingleNode perform post checks on a single node.
func (checker *checker) postCheckSingleNode(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) parse.TraversalAction {

	closestModule := findClosestModule(ancestorChain)
	_ = closestModule

	switch n := node.(type) {
	case *parse.ObjectLiteral:
		//manifest

		if utils.Implements[*parse.Manifest](parent) {
			if len(ancestorChain) < 3 {
				checker.addError(parent, text.CANNOT_CHECK_MANIFEST_WITHOUT_PARENT)
				break
			}

			chunk := ancestorChain[len(ancestorChain)-2]
			isEmbeddedModule := utils.Implements[*parse.EmbeddedModule](chunk)

			if isEmbeddedModule {
				var moduleKind ModuleKind
				switch ancestorChain[len(ancestorChain)-3].(type) {
				case *parse.LifetimejobExpression:
					moduleKind = LifetimeJobModule
				case *parse.SpawnExpression:
					moduleKind = UserLThreadModule
				case *parse.TestSuiteExpression:
					moduleKind = TestSuiteModule
				case *parse.TestCaseExpression:
					moduleKind = TestCaseModule
				default:
					panic(ErrUnreachable)
				}

				checkManifestObject(manifestStaticCheckArguments{
					objLit:                n,
					ignoreUnknownSections: true,
					moduleKind:            moduleKind,
					onError: func(n parse.Node, msg string) {
						checker.addError(n, msg)
					},
				})
			} //else: the manifest of regular modules is already checked during the pre-init phase
		}
	case *parse.ForStatement, *parse.ForExpression, *parse.WalkStatement:
		varsBefore := checker.store[node].(map[string]localVarInfo)
		checker.setScopeLocalVars(scopeNode, varsBefore)
	case *parse.SwitchStatement, *parse.MatchStatement:
		varsBefore, ok := checker.store[node]
		if ok {
			checker.setScopeLocalVars(scopeNode, varsBefore.(map[string]localVarInfo))
		}
	}
	return parse.ContinueTraversal
}

func checkVisibilityInitializationBlock(propInfo *propertyInfo, block *parse.InitializationBlock, onError func(n parse.Node, msg string)) {
	if len(block.Statements) != 1 || !utils.Implements[*parse.ObjectLiteral](block.Statements[0]) {
		onError(block, text.INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ)
		return
	}

	objLiteral := block.Statements[0].(*parse.ObjectLiteral)

	if len(objLiteral.MetaProperties) != 0 {
		onError(objLiteral, text.INVALID_VISIB_DESC_SHOULDNT_HAVE_METAPROPS)
	}

	for _, prop := range objLiteral.Properties {
		if prop.HasNoKey() {
			onError(objLiteral, text.INVALID_VISIB_DESC_SHOULDNT_HAVE_ELEMENTS)
			return
		}

		switch prop.Name() {
		case "public":
			_, ok := prop.Value.(*parse.KeyListExpression)
			if !ok {
				onError(prop, text.VAL_SHOULD_BE_KEYLIST_LIT)
				return
			}
		case "visible_by":
			dict, ok := prop.Value.(*parse.DictionaryLiteral)
			if !ok {
				onError(prop, text.VAL_SHOULD_BE_DICT_LIT)
				return
			}

			for _, entry := range dict.Entries {
				switch keyNode := entry.Key.(type) {
				case *parse.UnambiguousIdentifierLiteral:
					switch keyNode.Name {
					case "self":
						_, ok := entry.Value.(*parse.KeyListExpression)
						if !ok {
							onError(entry, text.VAL_SHOULD_BE_KEYLIST_LIT)
							return
						}
					default:
						onError(entry, text.INVALID_VISIBILITY_DESC_KEY)
					}
				default:
					onError(entry, text.INVALID_VISIBILITY_DESC_KEY)
					return
				}
			}
		default:
			onError(prop, text.INVALID_VISIBILITY_DESC_KEY)
			return
		}
	}
}

func shallowCheckObjectRecordProperties(
	properties []*parse.ObjectProperty,
	spreadElements []*parse.PropertySpreadElement,
	isObject bool,
	addError func(n parse.Node, msg string),
) (parse.TraversalAction, map[string]struct{}) {
	keys := map[string]struct{}{}
	hasElements := false

	// look for duplicate keys
	for _, prop := range properties {
		var k string

		if prop.Type != nil {
			addError(prop.Type, "type annotation of properties is not allowed")
		}

		switch n := prop.Key.(type) {
		case *parse.DoubleQuotedStringLiteral:
			k = n.Value
		case *parse.IdentifierLiteral:
			k = n.Name
		case nil:
			if _, ok := keys[inoxconsts.IMPLICIT_PROP_NAME]; ok && !hasElements {
				addError(prop.Value, text.ELEMENTS_NOT_ALLOWED_IF_EMPTY_PROP_NAME)
				continue
			}
			keys[inoxconsts.IMPLICIT_PROP_NAME] = struct{}{}
			hasElements = true
			continue
		default:
			continue
		}

		if k == inoxconsts.IMPLICIT_PROP_NAME && hasElements {
			addError(prop.Key, text.EMPTY_PROP_NAME_NOT_ALLOWED_IF_ELEMENTS)
			continue
		}

		if len(k) > MAX_NAME_BYTE_LEN {
			addError(prop.Key, text.FmtNameIsTooLong(k))
		}

		if parse.IsMetadataKey(k) {
			addError(prop.Key, text.OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS)
		} else if _, found := keys[k]; found {
			addError(prop, text.FmtDuplicateKey(k))
		}

		keys[k] = struct{}{}
	}

	// also look for duplicate keys
	for _, element := range spreadElements {

		extractionExpr, isValid := element.Expr.(*parse.ExtractionExpression)
		if !isValid {
			continue
		}

		for _, key := range extractionExpr.Keys.Keys {
			name := key.(*parse.IdentifierLiteral).Name

			_, found := keys[name]
			if found {
				addError(key, text.FmtDuplicateKey(name))
				return parse.ContinueTraversal, nil
			}
			keys[name] = struct{}{}
		}
	}

	return parse.ContinueTraversal, keys
}

// CombineParsingErrorValues combines errors into a single error with a multiline message.
func CombineParsingErrorValues(errs []Error, positions []parse.SourcePositionRange) error {

	if len(errs) == 0 {
		return nil
	}

	goErrors := make([]error, len(errs))
	for i, e := range errs {
		if i < len(positions) {
			goErrors[i] = fmt.Errorf("%s %w", positions[i].String(), e.goError)
		} else {
			goErrors[i] = e.goError
		}
	}

	return utils.CombineErrors(goErrors...)
}

// combineStaticCheckErrors combines static check errors into a single error with a multiline message.
func combineStaticCheckErrors(errs ...*StaticCheckError) error {

	goErrors := make([]error, len(errs))
	for i, e := range errs {
		goErrors[i] = e
	}
	return utils.CombineErrors(goErrors...)
}

type StaticCheckError struct {
	Message        string
	LocatedMessage string
	Location       parse.SourcePositionStack
}

func NewStaticCheckError(s string, location parse.SourcePositionStack) *StaticCheckError {
	return &StaticCheckError{
		Message:        CHECK_ERR_PREFIX + s,
		LocatedMessage: CHECK_ERR_PREFIX + location.String() + s,
		Location:       location,
	}
}

func (err StaticCheckError) Error() string {
	return err.LocatedMessage
}

func (err StaticCheckError) Err() Error {
	//TODO: cache (thread safe)
	return NewError(err, createRecordFromSourcePositionStack(err.Location))
}

func (err StaticCheckError) MessageWithoutLocation() string {
	return err.Message
}

func (err StaticCheckError) LocationStack() parse.SourcePositionStack {
	return err.Location
}

type StaticCheckWarning struct {
	Message        string
	LocatedMessage string
	Location       parse.SourcePositionStack
}

func NewStaticCheckWarning(s string, location parse.SourcePositionStack) *StaticCheckWarning {
	return &StaticCheckWarning{
		Message:        CHECK_ERR_PREFIX + s,
		LocatedMessage: CHECK_ERR_PREFIX + location.String() + s,
		Location:       location,
	}
}

func (err StaticCheckWarning) MessageWithoutLocation() string {
	return err.Message
}

func (err StaticCheckWarning) LocationStack() parse.SourcePositionStack {
	return err.Location
}
