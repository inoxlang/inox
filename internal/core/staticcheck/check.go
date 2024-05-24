package staticcheck

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

const (
	CHECK_ERR_PREFIX  = "check: "
	MAX_NAME_BYTE_LEN = 64
)

var (
	ErrForbiddenNodeinPreinit = errors.New("forbidden node type in preinit block")
	ErrUnreachable            = errors.New("unreachable")

	_ sourcecode.StackLocatedError = &Error{}
)

type Input struct {
	Node                   ast.Node
	Module                 *inoxmod.Module //optional
	CheckContext           inoxmod.Context
	Chunk                  *parse.ParsedChunkSource
	ParentChecker          *checker
	GlobalsInfo            map[string]GlobalVarInfo
	AdditionalGlobalConsts []string
	ShellLocalVars         []string
	Patterns               map[string]struct{}
	PatternNamespaces      map[string][]string

	BaseGlobalsForImportedModule           map[string]GlobalVarInfo
	BasePatternsForImportedModule          map[string]struct{}
	BasePatternNamespacesForImportedModule map[string][]string
}

// Check performs various checks on an AST, like checking duplicate declarations and keys or checking that statements like return,
// break and continue are not misplaced. No type checks are performed.
func Check(input Input) (*Data, error) {
	globals := make(map[ast.Node]map[string]GlobalVarInfo)

	var module ast.Node //ok if nil

	switch input.Node.(type) {
	case *ast.Chunk, *ast.EmbeddedModule:
		module = input.Node
	}

	if input.GlobalsInfo == nil {
		input.GlobalsInfo = map[string]GlobalVarInfo{}
	}
	globals[module] = maps.Clone(input.GlobalsInfo)

	for _, name := range input.AdditionalGlobalConsts {
		globals[module][name] = GlobalVarInfo{IsConst: true}
	}

	shellLocalVars := make(map[string]bool)

	localVars := make(map[ast.Node]map[string]localVarInfo)
	localVars[module] = map[string]localVarInfo{}
	for _, k := range input.ShellLocalVars {
		localVars[module][k] = localVarInfo{}
		shellLocalVars[k] = true
	}

	patterns := make(map[ast.Node]map[string]int)
	patterns[module] = map[string]int{}
	for k := range input.Patterns {
		patterns[module][k] = 0
	}

	patternNamespaces := make(map[ast.Node]map[string]patternNamespaceInfo)
	patternNamespaces[module] = map[string]patternNamespaceInfo{}
	for name, patterns := range input.PatternNamespaces {
		info := patternNamespaceInfo{patterns: make(map[string]int, len(patterns))}
		for _, patternName := range patterns {
			info.patterns[patternName] = 0
		}
		patternNamespaces[module][name] = info
	}

	checker := &checker{
		checkInput:        input,
		fnDecls:           make(map[ast.Node]map[string]*fnDeclInfo),
		structDefs:        make(map[ast.Node]map[string]int),
		globalVars:        globals,
		localVars:         localVars,
		shellLocalVars:    shellLocalVars,
		properties:        make(map[*ast.ObjectLiteral]*propertyInfo),
		patterns:          patterns,
		patternNamespaces: patternNamespaces,
		currentModule:     input.Module,
		chunk:             input.Chunk,
		store:             make(map[ast.Node]interface{}),
		data: &Data{
			fnData:                                 map[*ast.FunctionExpression]*FunctionData{},
			mappingData:                            map[*ast.MappingExpression]*MappingData{},
			firstForbiddenPosForGlobalElementDecls: make(map[ast.Node]int32, 0),
			functionsToDeclareEarly:                make(map[ast.Node]*[]*ast.FunctionDeclaration, 0),
		},
	}

	if module != nil {

		if chunk, ok := module.(*ast.Chunk); ok {
			checker.defineStructs(module, chunk.Statements)
			checker.precheckTopLevelStatements(chunk)
		} else {
			checker.defineStructs(module, module.(*ast.EmbeddedModule).Statements)
		}
	}

	err := checker.check(input.Node)
	if err != nil {
		return nil, err
	}

	checker.data.combinedErrors = CombineStaticCheckErrors(checker.data.errors...)
	return checker.data, checker.data.combinedErrors
}

// see Check function.
type checker struct {
	currentModule             *inoxmod.Module //can be nil
	chunk                     *parse.ParsedChunkSource
	inclusionImportStatement  *ast.InclusionImportStatement // can be nil
	moduleImportStatement     *ast.ImportStatement          //can be nil
	parentChecker             *checker                      //can be nil
	checkInput                Input
	furthestAssertionStmt     *ast.AssertionStatement
	furthestPreinitStmt       *ast.PreinitStatement
	furthestMarkupPatternExpr *ast.MarkupPatternExpression

	//key: *ast.Chunk|*ast.EmbeddedModule
	fnDecls map[ast.Node]map[string]*fnDeclInfo

	//key: *ast.Chunk|*ast.EmbeddedModule
	structDefs map[ast.Node]map[string]int

	//key: *ast.Chunk|*ast.EmbeddedModule
	globalVars map[ast.Node]map[string]GlobalVarInfo

	//key: *ast.Chunk|*ast.EmbeddedModule|*ast.FunctionExpression
	localVars map[ast.Node]map[string]localVarInfo

	properties map[*ast.ObjectLiteral]*propertyInfo

	//key: *ast.Chunk|*ast.EmbeddedModule
	patterns map[ast.Node]map[string]int

	//key: *ast.Chunk|*ast.EmbeddedModule
	patternNamespaces map[ast.Node]map[string]patternNamespaceInfo

	shellLocalVars map[string]bool

	store map[ast.Node]any

	data *Data
}

type fnDeclInfo struct {
	node           *ast.FunctionDeclaration
	capturedLocals []string
	module         ast.Node //*ast.Chunk|*ast.EmbeddedModule
}

// GlobalVarInfo represents the information stored about a global variable during checking.
type GlobalVarInfo struct {
	IsConst         bool
	IsStartConstant bool
	FnExpr          *ast.FunctionExpression
}

type patternNamespaceInfo struct {
	patterns map[string]int
}

// locallVarInfo represents the information stored about a local variable during checking.
type localVarInfo struct {
	isGroupMatchingVar bool
}

// propertyInfo represents the information stored about the properties of an object literal during checking.
type propertyInfo struct {
	known map[string]bool
}

func (checker *checker) makeCheckingError(node ast.Node, s string) *Error {
	location := checker.getSourcePositionStack(node)

	return NewError(s, location)
}

func (checker *checker) makeCheckingWarning(node ast.Node, s string) *StaticCheckWarning {
	location := checker.getSourcePositionStack(node)

	return NewStaticCheckWarning(s, location)
}

func (checker *checker) getSourcePositionStack(node ast.Node) sourcecode.SourcePositionStack {
	var sourcePositionStack sourcecode.SourcePositionStack

	if checker.parentChecker != nil {
		var importStmt ast.Node
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

func (checker *checker) addError(node ast.Node, s string) {
	checker.data.errors = append(checker.data.errors, checker.makeCheckingError(node, s))
}

func (checker *checker) addWarning(node ast.Node, s string) {
	checker.data.warnings = append(checker.data.warnings, checker.makeCheckingWarning(node, s))
}

func (c *checker) defineStructs(closestModule ast.Node, statements []ast.Node) {

	//Define structs from included chunks.
	for _, stmt := range statements {
		inclusionStmt, ok := stmt.(*ast.InclusionImportStatement)
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
		structDef, ok := stmt.(*ast.StructDefinition)
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
			var nameNode ast.Node

			switch def := memberDefinition.(type) {
			case *ast.StructFieldDefinition:
				name = def.Name.Name
				nameNode = def.Name
			case *ast.FunctionDeclaration:
				funcName, ok := def.Name.(*ast.IdentifierLiteral)
				if !ok {
					continue //unquoted name
				}
				name = funcName.Name
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

func (checker *checker) check(node ast.Node) error {
	checkNode := func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		return checker.checkSingleNode(node, parent, scopeNode, ancestorChain, after), nil
	}
	postCheckNode := func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		return checker.postCheckSingleNode(node, parent, scopeNode, ancestorChain, after), nil
	}
	return ast.Walk(node, checkNode, postCheckNode)
}

func (checker *checker) getLocalVarsInScope(scopeNode ast.Node) map[string]localVarInfo {
	if !ast.IsScopeContainerNode(scopeNode) {
		panic(fmt.Errorf("a %T is not a scope container", scopeNode))
	}

	variables, ok := checker.localVars[scopeNode]
	if !ok {
		variables = make(map[string]localVarInfo)
		checker.localVars[scopeNode] = variables
	}
	return variables
}

func (checker *checker) varExists(name string, ancestorChain []ast.Node) bool {
	var closestModule ast.Node

	checkGlobalVar := false

loop:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		if !ast.IsScopeContainerNode(ancestorChain[i]) {
			continue
		}

		scopeNode := ancestorChain[i]

		if checkGlobalVar {
			switch scopeNode.(type) {
			case *ast.Chunk, *ast.EmbeddedModule:
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
		case *ast.Chunk, *ast.EmbeddedModule:
			closestModule = scopeNode
			break loop
		}
	}

	globalVars := checker.getModGlobalVars(closestModule)
	_, ok := globalVars[name]
	return ok
}

func (checker *checker) doGlobalVarExist(name string, closestModule ast.Node) bool {
	globals := checker.getModGlobalVars(closestModule)
	_, ok := globals[name]
	return ok
}

func (checker *checker) setScopeLocalVars(scopeNode ast.Node, vars map[string]localVarInfo) {
	checker.localVars[scopeNode] = vars
}

func (checker *checker) getScopeLocalVarsCopy(scopeNode ast.Node) map[string]localVarInfo {
	variables := checker.getLocalVarsInScope(scopeNode)

	varsCopy := make(map[string]localVarInfo)
	for k, v := range variables {
		varsCopy[k] = v
	}
	return varsCopy
}

func (checker *checker) getModGlobalVars(module ast.Node) map[string]GlobalVarInfo {
	variables, ok := checker.globalVars[module]
	if !ok {
		variables = make(map[string]GlobalVarInfo)
		checker.globalVars[module] = variables
	}
	return variables
}

func (checker *checker) getModFunctionDecls(mod ast.Node) map[string]*fnDeclInfo {
	fns, ok := checker.fnDecls[mod]
	if !ok {
		fns = make(map[string]*fnDeclInfo)
		checker.fnDecls[mod] = fns
	}
	return fns
}

func (checker *checker) isDeclaredFunctionName(name string, mod ast.Node) bool {
	fns, ok := checker.fnDecls[mod]
	if !ok {
		return false
	}
	_, ok = fns[name]
	return ok
}

func (checker *checker) getModStructDefs(mod ast.Node) map[string]int {
	defs, ok := checker.structDefs[mod]
	if !ok {
		defs = make(map[string]int)
		checker.structDefs[mod] = defs
	}
	return defs
}

func (checker *checker) getModPatterns(mod ast.Node) map[string]int {
	patterns, ok := checker.patterns[mod]
	if !ok {
		patterns = make(map[string]int)
		checker.patterns[mod] = patterns
	}
	return patterns
}

func (checker *checker) getModPatternNamespaces(module ast.Node) map[string]patternNamespaceInfo {
	namespaces, ok := checker.patternNamespaces[module]
	if !ok {
		namespaces = make(map[string]patternNamespaceInfo)
		checker.patternNamespaces[module] = namespaces
	}
	return namespaces
}

func (checker *checker) getPropertyInfo(obj *ast.ObjectLiteral) *propertyInfo {
	propInfo, ok := checker.properties[obj]
	if !ok {
		propInfo = &propertyInfo{known: make(map[string]bool, 0)}
		checker.properties[obj] = propInfo
	}
	return propInfo
}

func findClosestModule(ancestorChain []ast.Node) ast.Node {
	var closestModule ast.Node

	for _, n := range ancestorChain {
		switch n.(type) {
		case *ast.Chunk, *ast.EmbeddedModule:
			closestModule = n
		}
	}

	return closestModule
}

func findClosest[T any](ancestorChain []ast.Node) T {
	var closest T

	for _, n := range ancestorChain {
		switch node := n.(type) {
		case T:
			closest = node
		}
	}

	return closest
}

func findClosestScopeContainerNode(ancestorChain []ast.Node) ast.Node {
	var closest ast.Node

	for _, n := range ancestorChain {
		if ast.IsScopeContainerNode(n) {
			closest = n
		}
	}

	return closest
}

// checkSingleNode perform checks on a single node.
func (c *checker) checkSingleNode(n, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) ast.TraversalAction {
	var (
		closestModule  = findClosestModule(ancestorChain)
		inPreinitBlock bool
	)

	if c.furthestAssertionStmt != nil && n.Base().Err == nil {
		closestAssertion := findClosest[*ast.AssertionStatement](ancestorChain)

		//Check that the node is allowed in assertions.

		if closestAssertion != nil {
			allowed := c.isNodeAllowedInAssertions(n)
			if !allowed {
				c.addError(n, text.FmtFollowingNodeTypeNotAllowedInAssertions(n))
				return ast.Prune
			}
		}
	}

	if c.furthestMarkupPatternExpr != nil && n.Base().Err == nil {
		//Check that the node is allowed in markup patterns.

		closestMarkupPatternExpr := findClosest[*ast.MarkupPatternExpression](ancestorChain)
		if closestMarkupPatternExpr != nil {
			c.checkNodeInMarkupPattern(n, parent)
		}
	}
	if c.furthestPreinitStmt != nil {
		inPreinitBlock = findClosest[*ast.PreinitStatement](ancestorChain) != nil
	}

	//Actually check the node.

	switch node := n.(type) {
	case *ast.IntegerRangeLiteral:
		if upperBound, ok := node.UpperBound.(*ast.IntLiteral); ok && node.LowerBound.Value > upperBound.Value {
			c.addError(n, text.LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND)
		}
	case *ast.FloatRangeLiteral:
		if upperBound, ok := node.UpperBound.(*ast.FloatLiteral); ok && node.LowerBound.Value > upperBound.Value {
			c.addError(n, text.LOWER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND)
		}
	case *ast.QuantityLiteral:
		return c.checkQuantityLiteral(node)
	case *ast.RateLiteral:
		return c.checkRateLiteral(node)
	case *ast.URLLiteral:
		u := utils.Must(url.Parse(node.Value))

		if strings.HasPrefix(node.Value, "mem://") && u.Host != inoxconsts.MEM_HOSTNAME {
			c.addError(node, text.INVALID_MEM_HOST_ONLY_VALID_VALUE)
		} else if u.User.String() != "" {
			c.addError(node, text.CREDENTIALS_NOT_ALLOWED_IN_URLS)
		}
	case *ast.HostLiteral:
		if strings.HasPrefix(node.Value, "mem://") && utils.Must(url.Parse(node.Value)).Host != inoxconsts.MEM_HOSTNAME {
			c.addError(node, text.INVALID_MEM_HOST_ONLY_VALID_VALUE)
		}
	case *ast.ObjectLiteral:
		return c.checkObjectLiteral(node)
	case *ast.RecordLiteral:
		return c.checkRecordLiteral(node)
	case *ast.ObjectPatternLiteral, *ast.RecordPatternLiteral:
		return c.checkObjectRecordPatternLiteral(node)
	case *ast.DictionaryLiteral:
		return c.checkDictionaryLiteral(node)
	case *ast.SpawnExpression:
		return c.checkSpawnExpr(node, closestModule)
	case *ast.ReceptionHandlerExpression:
		if prop, ok := parent.(*ast.ObjectProperty); !ok || !prop.HasNoKey() {
			c.addError(node, text.MISPLACED_RECEPTION_HANDLER_EXPRESSION)
		}

	case *ast.MappingExpression:
		//
	case *ast.StaticMappingEntry:
		return c.checkStaticMappingEntry(node)
	case *ast.DynamicMappingEntry:
		return c.checkDynamicMappingEntry(node)
	case *ast.ComputeExpression:
		return c.checkComputeExpr(node, scopeNode, ancestorChain)
	case *ast.InclusionImportStatement:
		return c.checkInclusionImportStmt(node, parent, closestModule, inPreinitBlock)
	case *ast.ImportStatement:
		return c.checkImportStmt(node, parent, closestModule)
	case *ast.GlobalConstantDeclarations:
		return c.checkGlobalConstDecls(node, parent, closestModule)
	case *ast.LocalVariableDeclarations:
		return c.checkLocalVarDecls(node, scopeNode, closestModule)
	case *ast.GlobalVariableDeclarations:
		return c.checkGlobalVarDecls(node, parent, scopeNode, closestModule)
	case *ast.Assignment, *ast.MultiAssignment:
		return c.checkAssignment(node, scopeNode, closestModule)
	case *ast.ForStatement:
		return c.checkForStmt(node, scopeNode, closestModule)
	case *ast.ForExpression:
		return c.checkForExpression(node, scopeNode, closestModule)
	case *ast.WalkStatement:
		return c.checkWalkStmt(node, scopeNode, closestModule)
	case *ast.WalkExpression:
		return c.checkWalkExpr(node, scopeNode, closestModule)
	case *ast.ReadonlyPatternExpression:
		return c.checkReadonlyPatternExpr(node, parent)
	case *ast.CallExpression:
		return c.checkCallExpression(node, scopeNode, closestModule)
	case *ast.FunctionDeclaration:
		return c.checkFuncDecl(node, parent, closestModule)
	case *ast.FunctionExpression:
		return c.checkFuncExpr(node, closestModule, ancestorChain)
	case *ast.FunctionPatternExpression:
		return c.checkFuncPatternExpr(node, closestModule)
	case *ast.ReturnStatement:
		return c.checkReturnStatement(node, ancestorChain)
	case *ast.CoyieldStatement:
		return c.checkCoyieldStmt(node, ancestorChain)
	case *ast.BreakStatement:
		return c.checkBreakStmt(node, ancestorChain)
	case *ast.ContinueStatement:
		return c.checkContinueStmt(node, ancestorChain)
	case *ast.YieldStatement:
		return c.checkYieldStmt(node, ancestorChain)
	case *ast.PruneStatement:
		return c.checkPruneStmt(node, ancestorChain)
	case *ast.SwitchStatement:
		return c.checkSwitchStatement(node, scopeNode, closestModule)
	case *ast.MatchStatement:
		return c.checkMatchStatement(node, scopeNode, closestModule)
	case *ast.MatchStatementCase:
		return c.checkMatchCase(node, scopeNode, closestModule)
	case *ast.Variable:
		return c.checkVariable(node, scopeNode, ancestorChain, closestModule)
	case *ast.IdentifierLiteral:
		return c.checkIdentifier(node, parent, scopeNode, closestModule, ancestorChain)
	case *ast.SelfExpression, *ast.SendValueExpression:
		return c.checkSelfExprAndSendValExpr(n, parent, ancestorChain)
	case *ast.PatternDefinition:
		return c.checkPatternDef(node, parent, closestModule, inPreinitBlock)
	case *ast.PatternNamespaceDefinition:
		return c.checkPatternNamespaceDefinition(node, parent, closestModule, inPreinitBlock)
	case *ast.PatternIdentifierLiteral:
		return c.checkPatternIdentifier(node, parent, closestModule, ancestorChain)
	case *ast.PatternNamespaceIdentifierLiteral:
		return c.checkPatternNamespaceIdentifier(node, closestModule)
	case *ast.PatternNamespaceMemberExpression:
		return c.checkPatternNamespaceMember(node, closestModule)
	case *ast.RuntimeTypeCheckExpression:
		return c.checkRuntimeTypeCheckExpr(node, parent)
	case *ast.ExtendStatement:
		if _, ok := parent.(*ast.Chunk); !ok {
			c.addError(node, text.MISPLACED_EXTEND_STATEMENT_TOP_LEVEL_STMT)
			return ast.ContinueTraversal
		}
	case *ast.StructDefinition:
		if parent != closestModule {
			c.addError(node, text.MISPLACED_STRUCT_DEF_TOP_LEVEL_STMT)
			return ast.ContinueTraversal
		}
		//already defined.
		return ast.ContinueTraversal
	case *ast.NewExpression:
		return c.checkNewExpr(node)
	case *ast.StructInitializationLiteral:
		return c.checkStructInitLiteral(node)
	case *ast.PointerType:
		return c.checkPointerType(node, parent)
	case *ast.DereferenceExpression:
		c.addError(node, "dereference expressions are not supported yet")
	case *ast.TestSuiteExpression:
		return c.checkTestSuiteExpr(node, ancestorChain)
	case *ast.TestCaseExpression:
		return c.checkTestCaseExpr(node, ancestorChain)
	case *ast.EmbeddedModule:
		return c.checkEmbeddedModule(node, parent, closestModule, ancestorChain)
	case *ast.AssertionStatement:
		if c.furthestAssertionStmt == nil {
			c.furthestAssertionStmt = node
		}
	case *ast.PreinitStatement:
		if c.furthestPreinitStmt == nil {
			c.furthestPreinitStmt = node
		}
	case *ast.MarkupPatternExpression:
		if c.furthestMarkupPatternExpr == nil {
			c.furthestMarkupPatternExpr = node
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) precheckTopLevelStatements(module ast.Node) {
	chunk, isChunk := module.(*ast.Chunk)
	isIncludedChunk := isChunk && chunk.IncludableChunkDesc != nil
	embeddedMod, isEmbeddedMod := module.(*ast.EmbeddedModule)

	if !isChunk && !isEmbeddedMod {
		panic(fmt.Errorf("precheckTopLevelStatements should be called on a chunk or embedded module"))
	}

	var statements []ast.Node

	if isChunk {
		statements = chunk.Statements
	} else {
		statements = embeddedMod.Statements
	}

	for _, stmt := range statements {
		switch stmt := stmt.(type) {
		//definitions
		case *ast.PatternDefinition:
		case *ast.PatternNamespaceDefinition:
		case *ast.ExtendStatement:
		case *ast.StructDefinition:
		case *ast.FunctionDeclaration:
			c.precheckTopLevelFuncDecl(stmt, module)
		//simple literals
		case ast.SimpleValueLiteral:
		//inclusion imports
		case *ast.InclusionImportStatement:
		//otter nodes
		default:
			if isIncludedChunk {
				c.addError(stmt, text.AN_INCLUDABLE_FILE_CAN_ONLY_CONTAIN_DEFINITIONS)
			}
		}
	}
}

func (c *checker) isNodeAllowedInAssertions(n ast.Node) (allowed bool) {
	switch n := n.(type) {
	case
		//variables
		*ast.Variable, *ast.IdentifierLiteral,

		*ast.BinaryExpression, *ast.UnaryExpression, *ast.URLExpression,
		ast.SimpleValueLiteral, *ast.IntegerRangeLiteral, *ast.FloatRangeLiteral,

		//data structure literals
		*ast.ObjectLiteral, *ast.ObjectProperty, *ast.ListLiteral, *ast.RecordLiteral,

		//member-like expressions
		*ast.MemberExpression, *ast.IdentifierMemberExpression, *ast.DoubleColonExpression,
		*ast.IndexExpression, *ast.SliceExpression,

		//patterns
		*ast.PatternIdentifierLiteral,
		*ast.ObjectPatternLiteral, *ast.ObjectPatternProperty, *ast.RecordPatternLiteral,
		*ast.ListPatternLiteral, *ast.TuplePatternLiteral,
		*ast.FunctionPatternExpression,
		*ast.PatternNamespaceIdentifierLiteral, *ast.PatternNamespaceMemberExpression,
		*ast.OptionPatternLiteral, *ast.OptionalPatternExpression,
		*ast.ComplexStringPatternPiece, *ast.PatternPieceElement, *ast.PatternGroupName,
		*ast.PatternUnion,
		*ast.PatternCallExpression:
		allowed = true
	case *ast.CallExpression:
		ident, ok := n.Callee.(*ast.IdentifierLiteral)
		if ok {
			switch ident.Name {
			case globalnames.LEN_FN:
				allowed = true
			}
		}
	}

	return
}

func (c *checker) checkNodeInMarkupPattern(n, parent ast.Node) {

	switch parent := parent.(type) {
	case *ast.MarkupPatternAttribute:
		if n != parent.Type {
			return
		}
		switch n.(type) {
		case
			ast.SimpleValueLiteral,

			//variables
			*ast.Variable, *ast.MemberExpression,

			//patterns
			*ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression:
		default:
			c.addError(n, text.ONLY_X_ARE_SUPPORTED_AS_PATTERNS_FOR_MARKUP_PATTERN_ATTRIBUTES)
			return
		}
	case *ast.MarkupPatternInterpolation:
		switch n.(type) {
		case
			ast.SimpleValueLiteral,

			//variables
			*ast.Variable, *ast.MemberExpression,

			//patterns
			*ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression:
		default:
			c.addError(n, text.ONLY_X_ARE_SUPPORTED_IN_MARKUP_PATTERN_INTERPOLATIONS)
			return
		}
	}

	switch n.(type) {
	case
		//markup
		*ast.MarkupPatternElement, *ast.MarkupPatternOpeningTag, *ast.MarkupPatternClosingTag,
		*ast.MarkupPatternAttribute, *ast.MarkupPatternWildcard, *ast.MarkupPatternInterpolation,
		*ast.MarkupText,

		ast.SimpleValueLiteral,

		//variables
		*ast.Variable, *ast.IdentifierLiteral, *ast.MemberExpression,

		//patterns
		*ast.PatternIdentifierLiteral, *ast.PatternNamespaceIdentifierLiteral, *ast.PatternNamespaceMemberExpression:
	default:
		c.addError(n, text.FmtFollowingNodeTypeNotAllowedInMarkupPatterns(n))
	}

}

func (c *checker) checkQuantityLiteral(node *ast.QuantityLiteral) ast.TraversalAction {

	var prevMultiplier string
	var prevUnit string
	var prevDurationUnitValue time.Duration

	for partIndex := 0; partIndex < len(node.Values); partIndex++ {
		if node.Values[partIndex] < 0 {
			c.addError(node, ErrNegQuantityNotSupported.Error())
			return ast.ContinueTraversal
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
			return ast.ContinueTraversal
		}

		unit := node.Units[partIndex][i:]

		switch unit {
		case "x", inoxconsts.LINE_COUNT_UNIT, inoxconsts.RUNE_COUNT_UNIT, inoxconsts.BYTE_COUNT_UNIT:
			if partIndex != 0 || prevUnit != "" {
				c.addError(node, text.INVALID_QUANTITY)
				return ast.ContinueTraversal
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
				return ast.ContinueTraversal
			}

			prevDurationUnitValue = durationUnitValue
			prevUnit = unit
		case "%":
			if partIndex != 0 || prevUnit != "" {
				c.addError(node, text.INVALID_QUANTITY)
				return ast.ContinueTraversal
			}
			if i == 0 {
				prevUnit = unit
				break
			}
			fallthrough
		default:
			c.addError(node, text.FmtNonSupportedUnit(node.Units[0]))
			return ast.ContinueTraversal
		}
	}

	err := CheckQuantity(node.Values, node.Units)
	if err != nil {
		c.addError(node, err.Error())
	}

	return ast.ContinueTraversal
}

func (c *checker) checkRateLiteral(node *ast.RateLiteral) ast.TraversalAction {
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
		case "x", inoxconsts.BYTE_COUNT_UNIT:
			return ast.ContinueTraversal
		}
	}
	c.addError(node, text.INVALID_RATE)
	return ast.ContinueTraversal
}

func (c *checker) checkObjectLiteral(node *ast.ObjectLiteral) ast.TraversalAction {
	action, keys := shallowCheckObjectRecordProperties(node.Properties, node.SpreadElements, true, func(n ast.Node, msg string) {
		c.addError(n, msg)
	})

	if action != ast.ContinueTraversal {
		return action
	}

	propInfo := c.getPropertyInfo(node)
	for k := range keys {
		propInfo.known[k] = true
	}

	for _, metaprop := range node.MetaProperties {
		switch metaprop.Name() {
		case inoxconsts.VISIBILITY_KEY:
			checkVisibilityInitializationBlock(propInfo, metaprop.Initialization, func(n ast.Node, msg string) {
				c.addError(n, msg)
			})
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkRecordLiteral(node *ast.RecordLiteral) ast.TraversalAction {
	action, _ := shallowCheckObjectRecordProperties(node.Properties, node.SpreadElements, false, func(n ast.Node, msg string) {
		c.addError(n, msg)
	})

	return action
}

func (c *checker) checkObjectRecordPatternLiteral(node ast.Node) ast.TraversalAction {
	keys := map[string]struct{}{}

	var propertyNodes []*ast.ObjectPatternProperty
	var spreadElementsNodes []*ast.PatternPropertySpreadElement
	var otherPropsNodes []*ast.OtherPropsExpr
	var isExact bool

	switch node := node.(type) {
	case *ast.ObjectPatternLiteral:
		propertyNodes = node.Properties
		spreadElementsNodes = node.SpreadElements
		otherPropsNodes = node.OtherProperties
		isExact = node.Exact()
	case *ast.RecordPatternLiteral:
		propertyNodes = node.Properties
		spreadElementsNodes = node.SpreadElements
		otherPropsNodes = node.OtherProperties
		isExact = node.Exact()
	}

	// look for duplicate keys
	for _, prop := range propertyNodes {
		var k string

		switch n := prop.Key.(type) {
		case *ast.DoubleQuotedStringLiteral:
			k = n.Value
		case *ast.IdentifierLiteral:
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
		extractionExpr, ok := element.Expr.(*ast.ExtractionExpression)
		if !ok {
			continue
		}

		for _, key := range extractionExpr.Keys.Keys {
			name := key.(*ast.IdentifierLiteral).Name

			_, found := keys[name]
			if found {
				c.addError(key, text.FmtDuplicateKey(name))
				return ast.ContinueTraversal
			}
			keys[name] = struct{}{}
		}
	}

	//check that if the pattern is exact there are no other otherprops nodes other than otherprops(no)
	if isExact {
		for _, prop := range otherPropsNodes {
			patternIdent, ok := prop.Pattern.(*ast.PatternIdentifierLiteral)

			if !ok || patternIdent.Name != parse.NO_OTHERPROPS_PATTERN_NAME {
				c.addError(prop, text.UNEXPECTED_OTHER_PROPS_EXPR_OTHERPROPS_NO_IS_PRESENT)
			}
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkDictionaryLiteral(node *ast.DictionaryLiteral) ast.TraversalAction {
	keys := map[string]bool{}

	// look for duplicate keys
	for _, entry := range node.Entries {

		keyNode, ok := entry.Key.(ast.SimpleValueLiteral)
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

	return ast.ContinueTraversal
}

func (c *checker) checkSpawnExpr(node *ast.SpawnExpression, closestModule ast.Node) ast.TraversalAction {

	var globals = make(map[string]GlobalVarInfo)
	var globalDescNode ast.Node

	// add constant globals
	parentModuleGlobals := c.getModGlobalVars(closestModule)
	for name, info := range parentModuleGlobals {
		if info.IsStartConstant {
			globals[name] = info
		}
	}

	// add globals passed by user
	if obj, ok := node.Meta.(*ast.ObjectLiteral); ok {
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
	case *ast.KeyListExpression:
		for _, ident := range desc.Keys {
			globVarName := ident.(*ast.IdentifierLiteral).Name
			if !c.doGlobalVarExist(globVarName, closestModule) {
				c.addError(globalDescNode, text.FmtCannotPassGlobalThatIsNotDeclaredToLThread(globVarName))
			}
			globals[globVarName] = GlobalVarInfo{IsConst: true}
		}
	case *ast.ObjectLiteral:
		if len(desc.SpreadElements) > 0 {
			c.addError(desc, text.INVALID_SPAWN_GLOBALS_SHOULD_BE)
		}

		for _, prop := range desc.Properties {
			if prop.HasNoKey() {
				c.addError(desc, text.INVALID_SPAWN_GLOBALS_SHOULD_BE)
				continue
			}
			globals[prop.Name()] = GlobalVarInfo{IsConst: true}
		}
	case *ast.NilLiteral:
	case nil:
	default:
		c.addError(node, text.INVALID_SPAWN_GLOBALS_SHOULD_BE)
	}

	if node.Module != nil && node.Module.SingleCallExpr {
		calleeNode := node.Module.Statements[0].(*ast.CallExpression).Callee

		switch calleeNode := calleeNode.(type) {
		case *ast.IdentifierLiteral:
			globals[calleeNode.Name] = GlobalVarInfo{IsConst: true}
		case *ast.IdentifierMemberExpression:
			globals[calleeNode.Left.Name] = GlobalVarInfo{IsConst: true}
		}
	}

	embeddedModuleGlobals := c.getModGlobalVars(node.Module)

	for name, info := range globals {
		embeddedModuleGlobals[name] = info
	}

	c.defineStructs(node.Module, node.Module.Statements)
	c.precheckTopLevelStatements(node.Module)

	return ast.ContinueTraversal
}

func (c *checker) checkStaticMappingEntry(node *ast.StaticMappingEntry) ast.TraversalAction {
	switch node.Key.(type) {
	case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression:
	default:
		if !ast.NodeIsSimpleValueLiteral(node.Key) {
			c.addError(node.Key, text.INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS)
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkDynamicMappingEntry(node *ast.DynamicMappingEntry) ast.TraversalAction {
	switch node.Key.(type) {
	case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression:
	default:
		if !ast.NodeIsSimpleValueLiteral(node.Key) {
			c.addError(node.Key, text.INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS)
		}
	}

	localVars := c.getLocalVarsInScope(node)
	varname := node.KeyVar.(*ast.IdentifierLiteral).Name
	localVars[varname] = localVarInfo{}

	if node.GroupMatchingVariable != nil {
		varname := node.GroupMatchingVariable.(*ast.IdentifierLiteral).Name
		localVars[varname] = localVarInfo{}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkComputeExpr(node *ast.ComputeExpression, scopeNode ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	if _, ok := scopeNode.(*ast.DynamicMappingEntry); !ok {
		c.addError(node, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
	} else {
	ancestor_loop:
		for i := len(ancestorChain) - 1; i >= 0; i-- {
			ancestor := ancestorChain[i]
			if ancestor == scopeNode {
				break
			}

			switch a := ancestor.(type) {
			case *ast.StaticMappingEntry:
				c.addError(node, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
				break ancestor_loop
			case *ast.DynamicMappingEntry:
				if a.Key == node || i < len(ancestorChain)-1 && ancestorChain[i+1] == a.Key {
					c.addError(node, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
				}
				break ancestor_loop
			}
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkInclusionImportStmt(node *ast.InclusionImportStatement, parent, closestModule ast.Node, inPreinitBlock bool) ast.TraversalAction {
	// if the import is performed by the preinit block, prune the traversal.
	if _, ok := parent.(*ast.Block); ok && inPreinitBlock {
		return ast.Prune
	}

	if _, ok := parent.(*ast.Chunk); !ok {
		c.addError(node, text.MISPLACED_INCLUSION_IMPORT_STATEMENT_TOP_LEVEL_STMT)
		return ast.ContinueTraversal
	}

	includedChunk := c.currentModule.InclusionStatementMap[node]
	if includedChunk == nil { //File not found
		return ast.ContinueTraversal
	}

	// child checker's globals
	globals := make(map[ast.Node]map[string]GlobalVarInfo)
	globals[includedChunk.Node] = maps.Clone(c.checkInput.GlobalsInfo)

	// add defined patterns & pattern namespaces to child checker
	patterns := make(map[ast.Node]map[string]int)
	patterns[includedChunk.Node] = map[string]int{}
	for k := range c.checkInput.Patterns {
		patterns[includedChunk.Node][k] = 0
	}

	patternNamespaces := make(map[ast.Node]map[string]patternNamespaceInfo)
	patternNamespaces[includedChunk.Node] = map[string]patternNamespaceInfo{}
	for name, patterns := range c.checkInput.PatternNamespaces {
		info := patternNamespaceInfo{patterns: make(map[string]int, len(patterns))}
		for _, patternName := range patterns {
			info.patterns[patternName] = 0
		}
		patternNamespaces[includedChunk.Node][name] = info
	}

	chunkChecker := &checker{
		parentChecker:            c,
		checkInput:               c.checkInput,
		fnDecls:                  make(map[ast.Node]map[string]*fnDeclInfo),
		structDefs:               make(map[ast.Node]map[string]int),
		globalVars:               globals,
		localVars:                make(map[ast.Node]map[string]localVarInfo),
		properties:               make(map[*ast.ObjectLiteral]*propertyInfo),
		patterns:                 patterns,
		patternNamespaces:        patternNamespaces,
		currentModule:            c.currentModule,
		chunk:                    includedChunk.ParsedChunkSource,
		inclusionImportStatement: node,
		store:                    make(map[ast.Node]any),
		data: &Data{
			fnData:                                 map[*ast.FunctionExpression]*FunctionData{},
			mappingData:                            map[*ast.MappingExpression]*MappingData{},
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
		if _, ok := c.checkInput.GlobalsInfo[k]; ok {
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
		if _, ok := c.checkInput.GlobalsInfo[k]; ok {
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

	return ast.ContinueTraversal
}

func (c *checker) checkImportStmt(node *ast.ImportStatement, parent, closestModule ast.Node) ast.TraversalAction {
	if c.inclusionImportStatement != nil {
		c.addError(node, text.MODULE_IMPORTS_NOT_ALLOWED_IN_INCLUDABLE_FILES)
		return ast.Prune
	}

	if _, ok := parent.(*ast.Chunk); !ok {
		c.addError(node, text.MISPLACED_MOD_IMPORT_STATEMENT_TOP_LEVEL_STMT)
		return ast.Prune
	}

	name := node.Identifier.Name
	variables := c.getModGlobalVars(closestModule)

	_, alreadyUsed := variables[name]
	if alreadyUsed {
		c.addError(node, text.FmtInvalidImportStmtAlreadyDeclaredGlobal(name))
		return ast.ContinueTraversal
	}
	variables[name] = GlobalVarInfo{IsConst: true}

	if c.inclusionImportStatement != nil || node.Source == nil {
		return ast.ContinueTraversal
	}

	var importedModuleSource string

	switch node.Source.(type) {
	case *ast.URLLiteral, *ast.AbsolutePathLiteral, *ast.RelativePathLiteral:
		sourceName, err := GetCheckImportedModuleSourceName(node.Source, c.currentModule, c.checkInput.CheckContext)
		if err != nil {
			c.addError(node, fmt.Sprintf("failed to resolve location of imported module: %s", err.Error()))
			return ast.ContinueTraversal
		}
		importedModuleSource = sourceName
	default:
		return ast.ContinueTraversal
	}

	importedModule := c.currentModule.DirectlyImportedModules[importedModuleSource]
	importedModuleNode := importedModule.MainChunk.Node

	globals := make(map[ast.Node]map[string]GlobalVarInfo)
	globals[importedModuleNode] = map[string]GlobalVarInfo{}

	//add base globals to child checker
	for globalName, info := range c.checkInput.BaseGlobalsForImportedModule {
		globals[importedModuleNode][globalName] = info
	}

	//add module arguments variable to child checker
	globals[importedModuleNode][globalnames.MOD_ARGS_VARNAME] = GlobalVarInfo{IsConst: true, IsStartConstant: true}

	//add base patterns & pattern namespaces to child checker

	patterns := make(map[ast.Node]map[string]int)
	patterns[importedModuleNode] = map[string]int{}
	for patternName := range c.checkInput.BasePatternsForImportedModule {
		patterns[importedModuleNode][patternName] = 0
	}

	patternNamespaces := make(map[ast.Node]map[string]patternNamespaceInfo)
	patternNamespaces[importedModuleNode] = map[string]patternNamespaceInfo{}
	for patternNamespaceName, patterns := range c.checkInput.BasePatternNamespacesForImportedModule {
		info := patternNamespaceInfo{
			patterns: map[string]int{},
		}
		for _, patternName := range patterns {
			info.patterns[patternName] = 0
		}
		patternNamespaces[importedModuleNode][patternNamespaceName] = info
	}

	chunkChecker := &checker{
		parentChecker:         c,
		checkInput:            c.checkInput,
		fnDecls:               make(map[ast.Node]map[string]*fnDeclInfo),
		structDefs:            make(map[ast.Node]map[string]int),
		globalVars:            globals,
		localVars:             make(map[ast.Node]map[string]localVarInfo),
		properties:            make(map[*ast.ObjectLiteral]*propertyInfo),
		patterns:              patterns,
		patternNamespaces:     patternNamespaces,
		currentModule:         importedModule,
		chunk:                 importedModule.MainChunk,
		moduleImportStatement: node,
		store:                 make(map[ast.Node]any),
		data: &Data{
			fnData:                                 map[*ast.FunctionExpression]*FunctionData{},
			mappingData:                            map[*ast.MappingExpression]*MappingData{},
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
	return ast.ContinueTraversal
}

func (c *checker) checkGlobalConstDecls(node *ast.GlobalConstantDeclarations, parent, closestModule ast.Node) ast.TraversalAction {
	globalVars := c.getModGlobalVars(closestModule)

	inIncludedChunk := c.chunk.Node.IncludableChunkDesc != nil

	for _, decl := range node.Declarations {
		ident, ok := decl.Left.(*ast.IdentifierLiteral)
		if !ok {
			continue
		}
		name := ident.Name

		_, alreadyUsed := globalVars[name]
		if alreadyUsed {
			c.addError(decl, text.FmtInvalidConstDeclGlobalAlreadyDeclared(name))
			return ast.ContinueTraversal
		}

		globalVars[name] = GlobalVarInfo{IsConst: true}

		//Check that there are not forbidden node types.
		ast.Walk(decl.Right, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
			switch n := node.(type) {
			case
				//variables
				*ast.Variable, *ast.IdentifierLiteral,

				*ast.BinaryExpression, *ast.UnaryExpression, *ast.URLExpression,
				ast.SimpleValueLiteral, *ast.IntegerRangeLiteral, *ast.FloatRangeLiteral,

				//immutable data structure literals
				*ast.RecordLiteral, *ast.ObjectProperty, *ast.TupleLiteral, *ast.TreedataLiteral,
				*ast.TreedataEntry, *ast.TreedataPair,

				//member-like expressions
				*ast.MemberExpression, *ast.IdentifierMemberExpression, *ast.DoubleColonExpression,
				*ast.IndexExpression, *ast.SliceExpression,

				//patterns
				*ast.PatternIdentifierLiteral,
				*ast.ObjectPatternLiteral, *ast.ObjectPatternProperty, *ast.RecordPatternLiteral,
				*ast.ListPatternLiteral, *ast.TuplePatternLiteral,
				*ast.FunctionPatternExpression,
				*ast.PatternNamespaceIdentifierLiteral, *ast.PatternNamespaceMemberExpression,
				*ast.OptionPatternLiteral, *ast.OptionalPatternExpression,
				*ast.ComplexStringPatternPiece, *ast.PatternPieceElement, *ast.PatternGroupName,
				*ast.PatternUnion,
				*ast.PatternCallExpression:
				//ok
			case *ast.CallExpression:
				if inIncludedChunk {
					c.addError(n.Callee, text.CALL_EXPRS_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS_OF_INCLUDABLE_FILES)
					return ast.Prune, nil
				}

				switch callee := n.Callee.(type) {
				case *ast.IdentifierLiteral:
					if !slices.Contains(globalnames.USABLE_GLOBALS_IN_PREINIT, callee.Name) {
						c.addError(n.Callee, text.A_LIMITED_NUMBER_OF_BUILTINS_ARE_ALLOWED_TO_BE_CALLED_IN_GLOBAL_CONST_DECLS)
						return ast.Prune, nil
					}
				case *ast.MemberExpression, *ast.IdentifierMemberExpression:
				default:
					c.addError(n, text.CALLED_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS)
					return ast.Prune, nil
				}
			default:
				c.addError(n, text.FmtFollowingNodeTypeNotAllowedInGlobalConstantDeclarations(n))
				return ast.Prune, nil
			}
			return ast.ContinueTraversal, nil
		}, nil)

	}

	return ast.ContinueTraversal
}

func (c *checker) checkLocalVarDecls(node *ast.LocalVariableDeclarations, scopeNode, closestModule ast.Node) ast.TraversalAction {
	localVars := c.getLocalVarsInScope(scopeNode)
	globalVariables := c.getModGlobalVars(closestModule)

	for _, decl := range node.Declarations {
		switch left := decl.Left.(type) {
		case *ast.IdentifierLiteral:
			ident := left
			c.checkLocalVarDecl(ident, localVars, globalVariables)
		case *ast.ObjectDestructuration:
			destructuration := left
			for _, prop := range destructuration.Properties {
				validProp, ok := prop.(*ast.ObjectDestructurationProperty)
				if !ok {
					continue
				}

				nameNode := validProp.NameNode()
				c.checkLocalVarDecl(nameNode, localVars, globalVariables)
			}
		default:
			continue
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkLocalVarDecl(
	node *ast.IdentifierLiteral,
	localVars map[string]localVarInfo,
	globalVariables map[string]GlobalVarInfo,
) {
	name := node.Name
	if _, alreadyDefined := globalVariables[name]; alreadyDefined {
		c.addError(node, text.FmtCannotShadowGlobalVariable(name))
		return
	}

	_, alreadyUsed := localVars[name]
	if alreadyUsed {
		c.addError(node, text.FmtInvalidLocalVarDeclAlreadyDeclared(name))
		return
	}

	localVars[name] = localVarInfo{}
}

func (c *checker) checkGlobalVarDecls(node *ast.GlobalVariableDeclarations, parentNode, scopeNode, closestModule ast.Node) ast.TraversalAction {
	globalVariables := c.getModGlobalVars(closestModule)

	//Check the declarations are not misplaced.

	if !SamePointer(parentNode, closestModule) {
		c.addError(node, text.MISPLACED_GLOBAL_VAR_DECLS_TOP_LEVEL_STMT)
		return ast.Prune
	}

	firstForbiddenPos := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
	if firstForbiddenPos != 0 && node.Base().Span.Start >= firstForbiddenPos {
		c.addError(node, text.MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN)
		return ast.Prune
	}

	//Check each declaration.

	localVariables := c.getLocalVarsInScope(scopeNode)

	for _, decl := range node.Declarations {
		switch left := decl.Left.(type) {
		case *ast.IdentifierLiteral:
			ident := left
			c.checkGlobalVarDecl(ident, localVariables, globalVariables, closestModule)
		case *ast.ObjectDestructuration:
			destructuration := left
			for _, prop := range destructuration.Properties {
				validProp, ok := prop.(*ast.ObjectDestructurationProperty)
				if !ok {
					continue
				}

				nameNode := validProp.NameNode()
				c.checkGlobalVarDecl(nameNode, localVariables, globalVariables, closestModule)
			}
		default:
			continue
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkGlobalVarDecl(
	ident *ast.IdentifierLiteral,
	localVariables map[string]localVarInfo,
	globalVariables map[string]GlobalVarInfo,
	closestModule ast.Node,
) {
	name := ident.Name

	if _, alreadyDefined := localVariables[name]; alreadyDefined {
		c.addError(ident, text.FmtCannotShadowLocalVariable(name))
		return
	}

	_, alreadyUsed := globalVariables[name]
	if alreadyUsed {

		fnDecls := c.getModFunctionDecls(closestModule)
		_, isFunc := fnDecls[name]

		msg := ""
		if isFunc {
			msg = text.FmtInvalidAssignmentNameIsFuncName(name)
		} else {
			msg = text.FmtInvalidGlobalVarDeclAlreadyDeclared(name)
		}

		c.addError(ident, msg)
		return
	}
	globalVariables[name] = GlobalVarInfo{}
}

func (c *checker) checkAssignment(node ast.Node, scopeNode, closestModule ast.Node) ast.TraversalAction {
	var names []string

	if assignment, ok := node.(*ast.Assignment); ok {

		switch left := assignment.Left.(type) {
		case *ast.Variable:

			if left.Name == "" { //$
				c.addError(node, text.INVALID_ASSIGNMENT_ANONYMOUS_VAR_CANNOT_BE_ASSIGNED)
				return ast.ContinueTraversal
			}

			globalVariables := c.getModGlobalVars(closestModule)

			if _, isGlobal := globalVariables[left.Name]; isGlobal {
				if c.isDeclaredFunctionName(left.Name, closestModule) {
					c.addError(node, text.FmtInvalidAssignmentNameIsFuncName(left.Name))
				} else {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
				}
				return ast.ContinueTraversal
			}

			//Local variable

			localVars := c.getLocalVarsInScope(scopeNode)

			_, alreadyDefined := localVars[left.Name]

			if !alreadyDefined && assignment.Operator != ast.Assign {
				c.addError(node, text.FmtInvalidVariableAssignmentVarDoesNotExist(left.Name))
			}

			names = append(names, left.Name)
		case *ast.IdentifierLiteral:
			globalVariables := c.getModGlobalVars(closestModule)

			if _, alreadyDefined := globalVariables[left.Name]; alreadyDefined {
				if c.isDeclaredFunctionName(left.Name, closestModule) {
					c.addError(node, text.FmtInvalidAssignmentNameIsFuncName(left.Name))
				} else {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
				}
				return ast.ContinueTraversal
			}

			localVars := c.getLocalVarsInScope(scopeNode)

			_, alreadyDefined := localVars[left.Name]
			if !alreadyDefined && assignment.Operator != ast.Assign {
				c.addError(node, text.FmtInvalidVariableAssignmentVarDoesNotExist(left.Name))
			}

			if !alreadyDefined {
				globalVariables := c.getModGlobalVars(closestModule)

				if _, isGlobal := globalVariables[left.Name]; isGlobal {
					c.addError(node, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED)
					return ast.ContinueTraversal
				}
			}

			names = append(names, left.Name)
		case *ast.IdentifierMemberExpression:

			for _, ident := range left.PropertyNames {
				if parse.IsMetadataKey(ident.Name) {
					c.addError(node, text.FmtInvalidMemberAssignmentCannotModifyMetaProperty(ident.Name))
				}
			}
		case *ast.MemberExpression:
			curr := left
			var ok bool
			for {
				if parse.IsMetadataKey(curr.PropertyName.Name) {
					c.addError(node, text.FmtInvalidMemberAssignmentCannotModifyMetaProperty(curr.PropertyName.Name))
					break
				}
				if curr, ok = curr.Left.(*ast.MemberExpression); !ok {
					break
				}
			}
		case *ast.SliceExpression:
			if assignment.Operator != ast.Assign {
				c.addError(node, text.INVALID_ASSIGNMENT_EQUAL_ONLY_SUPPORTED_ASSIGNMENT_OPERATOR_FOR_SLICE_EXPRS)
			}
		}
	} else {
		assignment := node.(*ast.MultiAssignment)

		for _, variable := range assignment.Variables {
			ident, ok := variable.(*ast.IdentifierLiteral)
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

	return ast.ContinueTraversal
}

func (c *checker) checkForStmt(node *ast.ForStatement, scopeNode, closestModule ast.Node) ast.TraversalAction {
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

	return ast.ContinueTraversal
}

func (c *checker) checkForExpression(node *ast.ForExpression, scopeNode, closestModule ast.Node) ast.TraversalAction {
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

	return ast.ContinueTraversal
}

func (c *checker) checkWalkStmt(node *ast.WalkStatement, scopeNode, closestModule ast.Node) ast.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	localVars := c.getLocalVarsInScope(scopeNode)
	globalVars := c.getModGlobalVars(closestModule)

	c.store[node] = variablesBeforeStmt

	metaIdent := node.MetaIdent
	entryIdent := node.EntryIdent

	if metaIdent != nil {
		name := metaIdent.Name
		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(metaIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(metaIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}

	if entryIdent != nil {
		name := entryIdent.Name
		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(entryIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(entryIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkWalkExpr(node *ast.WalkExpression, scopeNode, closestModule ast.Node) ast.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	localVars := c.getLocalVarsInScope(scopeNode)
	globalVars := c.getModGlobalVars(closestModule)

	c.store[node] = variablesBeforeStmt

	metaIdent := node.MetaIdent
	entryIdent := node.EntryIdent

	if metaIdent != nil {
		name := metaIdent.Name
		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(metaIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(metaIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}

	if entryIdent != nil {
		name := entryIdent.Name
		if _, alreadyDefined := localVars[name]; alreadyDefined && !c.shellLocalVars[name] {
			c.addError(entryIdent, text.FmtCannotShadowLocalVariable(name))
		} else if _, alreadyDefined := globalVars[name]; alreadyDefined {
			c.addError(entryIdent, text.FmtCannotShadowGlobalVariable(name))
		} else {
			localVars[name] = localVarInfo{}
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkReadonlyPatternExpr(node *ast.ReadonlyPatternExpression, parent ast.Node) ast.TraversalAction {
	ok := false
	switch p := parent.(type) {
	case *ast.FunctionParameter:
		ok = p.Type == node
	default:
	}

	if !ok {
		c.addError(node, text.MISPLACED_READONLY_PATTERN_EXPRESSION)
	}

	return ast.ContinueTraversal
}

func (c *checker) precheckTopLevelFuncDecl(stmt *ast.FunctionDeclaration, module ast.Node) {
	globalVars := c.getModGlobalVars(module)
	fnDecls := c.getModFunctionDecls(module)

	funcName, ok := stmt.Name.(*ast.IdentifierLiteral)
	if !ok {
		return
	}

	_, alreadyDeclared := fnDecls[funcName.Name]
	if alreadyDeclared {
		c.addError(stmt, text.FmtInvalidFnDeclAlreadyDeclared(funcName.Name))
		return
	}

	//Pre-declare the functions that don't capture locals.
	if len(stmt.Function.CaptureList) == 0 {
		globalVars[funcName.Name] = GlobalVarInfo{IsConst: true, FnExpr: stmt.Function}

		fns := c.data.functionsToDeclareEarly[module]
		if fns == nil {
			fns = new([]*ast.FunctionDeclaration)
			c.data.functionsToDeclareEarly[module] = fns
		}
		*fns = append(*fns, stmt)

		info := &fnDeclInfo{node: stmt, module: module}
		fnDecls[funcName.Name] = info
	}
}

func (c *checker) checkCallExpression(node *ast.CallExpression, scopeNode, closestModule ast.Node) ast.TraversalAction {

	return ast.ContinueTraversal
}

func (c *checker) checkFuncDecl(node *ast.FunctionDeclaration, parent, closestModule ast.Node) ast.TraversalAction {
	funcName, ok := node.Name.(*ast.IdentifierLiteral)
	if !ok {
		return ast.Prune
	}

	//Check location.

	switch parent.(type) {
	case *ast.Chunk, *ast.EmbeddedModule:
		fnDecls := c.getModFunctionDecls(closestModule)
		globalVars := c.getModGlobalVars(closestModule)
		localVars := c.getLocalVarsInScope(closestModule)

		if len(node.Function.CaptureList) == 0 {
			_, ok := fnDecls[funcName.Name]
			if !ok {
				c.addError(node, "function has no been pre-checked by the static checker")
				return ast.ContinueTraversal
			}

			_, ok = c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
			if !ok {
				c.data.firstForbiddenPosForGlobalElementDecls[closestModule] = node.Span.Start
			}
		} else {
			declInfo := &fnDeclInfo{node: node, module: closestModule}

			for _, captured := range node.Function.CaptureList {
				if ident, ok := captured.(*ast.IdentifierLiteral); ok {
					_, isLocal := localVars[ident.Name]
					_, isGlobal := globalVars[ident.Name]

					if isLocal {
						declInfo.capturedLocals = append(declInfo.capturedLocals, ident.Name)
					} else if !isGlobal {
						c.addError(node, text.FmtInvalidOrMisplacedFnDeclShouldBeAfterCapturedVarDeclaration(ident.Name))
					}
				}
			}

			fnDecls[funcName.Name] = declInfo
			globalVars[funcName.Name] = GlobalVarInfo{IsConst: true, FnExpr: node.Function}
		}

	case *ast.StructBody:
		//struct method
	default:
		c.addError(node, text.INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT)
		return ast.ContinueTraversal
	}

	return ast.ContinueTraversal
}

func (c *checker) checkFuncExpr(node *ast.FunctionExpression, closestModule ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	fnLocalVars := c.getLocalVarsInScope(node)

	//Check the capture list.

	for _, e := range node.CaptureList {
		ident, ok := e.(*ast.IdentifierLiteral)
		if !ok { //invalid (parsing error)
			continue
		}
		name := ident.Name

		//Check that the captured variable exists and is a local.

		if !c.varExists(name, ancestorChain) {
			c.addError(e, text.FmtVarIsNotDeclared(name))
		} else if c.doGlobalVarExist(name, closestModule) {
			c.addError(e, text.FmtCannotPassGlobalToFunction(name))
		} else if _, ok := fnLocalVars[name]; ok {
			c.addError(ident, text.FmtVarIsAlreadyCaptured(name))
			return ast.ContinueTraversal
		}

		fnLocalVars[name] = localVarInfo{}
	}

	//Check parameters.

	for _, p := range node.Parameters {
		paramNameIdent, ok := p.Var.(*ast.IdentifierLiteral)
		if !ok {
			continue
		}
		name := paramNameIdent.Name

		globalVariables := c.getModGlobalVars(closestModule)

		if _, alreadyDefined := globalVariables[name]; alreadyDefined {
			c.addError(p, text.FmtParameterCannotShadowGlobalVariable(name))
			return ast.ContinueTraversal
		}

		if _, alreadyDefined := fnLocalVars[name]; alreadyDefined {
			c.addError(p, text.FmtParameterAlreadyDeclared(name))
			return ast.ContinueTraversal
		}

		fnLocalVars[name] = localVarInfo{}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkFuncPatternExpr(node *ast.FunctionPatternExpression, closestModule ast.Node) ast.TraversalAction {
	fnLocalVars := c.getLocalVarsInScope(node)

	for _, p := range node.Parameters {
		if p.Var == nil {
			continue
		}

		paramNameIdent, ok := p.Var.(*ast.IdentifierLiteral)
		if !ok {
			continue
		}

		name := paramNameIdent.Name

		globalVariables := c.getModGlobalVars(closestModule)

		if _, alreadyDefined := globalVariables[name]; alreadyDefined {
			c.addError(p, text.FmtParameterCannotShadowGlobalVariable(name))
			return ast.ContinueTraversal
		}

		fnLocalVars[name] = localVarInfo{}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkReturnStatement(node *ast.ReturnStatement, ancestorChain []ast.Node) ast.TraversalAction {

	//Go up until we find a module, function body, or an invalid node.
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		ancestor := ancestorChain[i]

		if ast.IsTheTopLevel(ancestor) {
			return ast.ContinueTraversal //ok
		}

		switch ancestor.(type) {
		case *ast.FunctionExpression:
			return ast.ContinueTraversal //ok
		case *ast.IfStatement, *ast.ForStatement, *ast.WalkStatement,
			*ast.SwitchStatement, *ast.SwitchStatementCase, *ast.DefaultCaseWithBlock,
			*ast.MatchStatement, *ast.MatchStatementCase,
			*ast.SynchronizedBlockStatement, *ast.Block:
		default:
			c.addError(node, text.MISPLACED_RETURN_STATEMENT)
			return ast.ContinueTraversal //ok
		}
	}

	c.addError(node, text.MISPLACED_RETURN_STATEMENT)
	return ast.ContinueTraversal
}

func (c *checker) checkCoyieldStmt(node *ast.CoyieldStatement, ancestorChain []ast.Node) ast.TraversalAction {
	ok := c.checkInput.Module != nil && c.checkInput.Module.IsEmbedded()

	for i := len(ancestorChain) - 1; i >= 0; i-- {
		if !ast.IsScopeContainerNode(ancestorChain[i]) {
			continue
		}

		if ok && ancestorChain[i] != c.checkInput.Node {
			ok = false
			break
		}

		switch ancestorChain[i].(type) {
		case *ast.EmbeddedModule:
			ok = true
		}
		break
	}

	if !ok {
		c.addError(node, text.MISPLACE_COYIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES)
	}
	return ast.ContinueTraversal
}

func (c *checker) checkBreakStmt(node ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	stmtIndex := -1

	//we search for the closest switch, match or iterative statement/expression in the ancestor chain
loop0:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *ast.SwitchStatement, *ast.MatchStatement,
			*ast.ForStatement, *ast.ForExpression, *ast.WalkStatement, *ast.WalkExpression:
			stmtIndex = i
			break loop0
		}
	}

	if stmtIndex < 0 {
		c.addError(node, text.BREAK_STMTS_ONLY_ALLOWED_LOCATION)
		return ast.ContinueTraversal
	}

	for i := stmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *ast.IfStatement, *ast.SwitchStatementCase, *ast.MatchStatementCase, *ast.MatchStatement,
			*ast.DefaultCaseWithBlock, *ast.Block:
		default:
			c.addError(node, text.BREAK_STMTS_ONLY_ALLOWED_LOCATION)
			return ast.ContinueTraversal
		}

	}
	return ast.ContinueTraversal
}

func (c *checker) checkContinueStmt(node ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	iterativeStmtIndex := -1

	//we search for the closest iterative statement or expression in the ancestor chain
loop0:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *ast.ForStatement, *ast.WalkStatement,
			*ast.ForExpression, *ast.WalkExpression:
			iterativeStmtIndex = i
			break loop0
		}
	}

	if iterativeStmtIndex < 0 {
		c.addError(node, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT)
		return ast.ContinueTraversal
	}

	for i := iterativeStmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *ast.IfStatement, *ast.SwitchStatement, *ast.MatchStatement, *ast.SwitchStatementCase, *ast.MatchStatementCase,
			*ast.DefaultCaseWithBlock, *ast.Block:
		default:
			c.addError(node, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT)
			return ast.ContinueTraversal
		}

	}
	return ast.ContinueTraversal
}

func (c *checker) checkYieldStmt(node ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	iterativeStmtIndex := -1

	//we search for the last for or walk expressions in the ancestor chain
loop0:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *ast.ForExpression, *ast.WalkExpression:
			iterativeStmtIndex = i
			break loop0
		}
	}

	if iterativeStmtIndex < 0 {
		c.addError(node, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR)
		return ast.ContinueTraversal
	}

	for i := iterativeStmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *ast.IfStatement, *ast.SwitchStatement, *ast.SwitchStatementCase,
			*ast.MatchStatementCase, *ast.MatchStatement, *ast.Block,
			*ast.ForStatement, *ast.WalkStatement:
		default:
			c.addError(node, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR)
			return ast.ContinueTraversal
		}

	}
	return ast.ContinueTraversal
}

func (c *checker) checkPruneStmt(node *ast.PruneStatement, ancestorChain []ast.Node) ast.TraversalAction {
	walkStmtIndex := -1
	//we search for the last walk statement or expression in the ancestor chain
loop1:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *ast.WalkStatement, *ast.WalkExpression:
			walkStmtIndex = i
			break loop1
		}
	}

	if walkStmtIndex < 0 {
		c.addError(node, text.PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMTS_AND_EXPRS)
		return ast.ContinueTraversal
	}

	for i := walkStmtIndex + 1; i < len(ancestorChain); i++ {
		switch ancestorChain[i].(type) {
		case *ast.IfStatement, *ast.SwitchStatement, *ast.MatchStatement, *ast.Block, *ast.ForStatement:
		default:
			c.addError(node, text.PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMTS_AND_EXPRS)
			return ast.ContinueTraversal
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkSwitchStatement(node *ast.SwitchStatement, scopeNode, closestModule ast.Node) ast.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	c.store[node] = variablesBeforeStmt

	//default case uniqueness is checked by the parser.

	return ast.ContinueTraversal
}

func (c *checker) checkMatchStatement(node *ast.MatchStatement, scopeNode, closestModule ast.Node) ast.TraversalAction {
	variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
	c.store[node] = variablesBeforeStmt

	//default case uniqueness is checked by the parser.

	return ast.ContinueTraversal
}

func (c *checker) checkMatchCase(node *ast.MatchStatementCase, scopeNode, closestModule ast.Node) ast.TraversalAction {

	//define the variables named after groups if the literal is used as a case in a match statement

	if node.GroupMatchingVariable == nil {
		return ast.ContinueTraversal
	}

	variable := node.GroupMatchingVariable.(*ast.IdentifierLiteral)

	if _, alreadyDefined := c.getModGlobalVars(closestModule)[variable.Name]; alreadyDefined {
		c.addError(variable, text.FmtCannotShadowGlobalVariable(variable.Name))
		return ast.ContinueTraversal
	}

	localVars := c.getLocalVarsInScope(scopeNode)

	if info, alreadyDefined := localVars[variable.Name]; alreadyDefined && info != (localVarInfo{isGroupMatchingVar: true}) {
		c.addError(variable, text.FmtCannotShadowLocalVariable(variable.Name))
		return ast.ContinueTraversal
	}

	localVars[variable.Name] = localVarInfo{isGroupMatchingVar: true}

	return ast.ContinueTraversal
}

func (c *checker) checkVariable(node *ast.Variable, scopeNode ast.Node, ancestorChain []ast.Node, closestModule ast.Node) ast.TraversalAction {
	if len(node.Name) > MAX_NAME_BYTE_LEN {
		c.addError(node, text.FmtNameIsTooLong(node.Name))
		return ast.ContinueTraversal
	}

	if node.Name == "" {
		return ast.ContinueTraversal
	}

	globalVars := c.getModGlobalVars(closestModule)

	if globalVarInfo, exists := globalVars[node.Name]; exists {
		parent := ancestorChain[len(ancestorChain)-1]

		if len(node.Name) > MAX_NAME_BYTE_LEN {
			c.addError(node, text.FmtNameIsTooLong(node.Name))
			return ast.ContinueTraversal
		}

		if _, isAssignment := parent.(*ast.Assignment); isAssignment {
			return ast.ContinueTraversal
		}

		if _, isLazyExpr := scopeNode.(*ast.QuotedExpression); isLazyExpr {
			return ast.ContinueTraversal
		}

		if _, ok := scopeNode.(*ast.ExtendStatement); ok {
			c.addError(node, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES)
			return ast.ContinueTraversal
		}

		if _, ok := scopeNode.(*ast.StructDefinition); ok {
			c.addError(node, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS)
			return ast.ContinueTraversal
		}

		if !exists {
			c.addError(node, text.FmtGlobalVarIsNotDeclared(node.Name))
			return ast.ContinueTraversal
		}

		fnDecls := c.getModFunctionDecls(closestModule)

		if fnDecls[node.Name] != nil && fnDecls[node.Name].module == closestModule {
			//If the global variable is a function then no global declarations (patterns, global variables)
			//should be located after this reference.

			if _, ok := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]; !ok {
				topLevelStmt, ok := ast.FindClosestTopLevelStatement(node, ancestorChain)
				if !ok {
					panic(ErrUnreachable)
				}
				c.data.firstForbiddenPosForGlobalElementDecls[closestModule] = topLevelStmt.Base().Span.Start
			}
		}

		switch scope := scopeNode.(type) {
		case *ast.FunctionExpression:
			c.data.addFnCapturedGlobal(scope, node.Name, &globalVarInfo)
		case *ast.EmbeddedModule:
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

		case *ast.DynamicMappingEntry, *ast.StaticMappingEntry:
			mappingExpr := findClosest[*ast.MappingExpression](ancestorChain)
			c.data.addMappingCapturedGlobal(mappingExpr, node.Name)
		}

		return ast.ContinueTraversal
	}

	//Local variable

	if _, isLazyExpr := scopeNode.(*ast.QuotedExpression); isLazyExpr {
		return ast.ContinueTraversal
	}

	if _, ok := scopeNode.(*ast.ExtendStatement); ok {
		c.addError(node, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES)
		return ast.ContinueTraversal
	}

	if _, ok := scopeNode.(*ast.StructDefinition); ok {
		c.addError(node, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS)
		return ast.ContinueTraversal
	}

	variables := c.getLocalVarsInScope(scopeNode)
	_, exist := variables[node.Name]

	if !exist {
		c.addError(node, text.FmtVarIsNotDeclared(node.Name))
		return ast.ContinueTraversal
	}

	return ast.ContinueTraversal
}

func (c *checker) checkIdentifier(ident *ast.IdentifierLiteral, parent, scopeNode, closestModule ast.Node, ancestorChain []ast.Node) ast.TraversalAction {

	if len(ident.Name) > MAX_NAME_BYTE_LEN {
		c.addError(ident, text.FmtNameIsTooLong(ident.Name))
		return ast.ContinueTraversal
	}

	if _, ok := scopeNode.(*ast.QuotedExpression); ok {
		return ast.ContinueTraversal
	}

	//we check the parent to know if the identifier refers to a variable
	switch p := parent.(type) {
	case *ast.CallExpression:
		if p.CommandLikeSyntax && !ident.IncludedIn(p.Callee) {
			return ast.ContinueTraversal

		}
	case *ast.FunctionDeclaration:
		return ast.ContinueTraversal
	case *ast.LocalVariableDeclarator:
		if p.Left == ident {
			return ast.ContinueTraversal
		}
	case *ast.ObjectDestructurationProperty:
		return ast.ContinueTraversal
	case *ast.ObjectProperty:
		if p.Key == ident {
			return ast.ContinueTraversal
		}
	case *ast.ObjectPatternProperty:
		if p.Key == ident {
			return ast.ContinueTraversal

		}
	case *ast.ObjectMetaProperty:
		if p.Key == ident {
			return ast.ContinueTraversal

		}
	case *ast.StructDefinition:
		if p.Name == ident {
			return ast.ContinueTraversal

		}

	case *ast.StructFieldDefinition:
		if p.Name == ident {
			return ast.ContinueTraversal

		}
	case *ast.NewExpression:
		if p.Type == ident {
			return ast.ContinueTraversal

		}
	case *ast.StructFieldInitialization:
		if p.Name == ident {
			return ast.ContinueTraversal

		}
	case *ast.IdentifierMemberExpression:
		if ident != p.Left {
			return ast.ContinueTraversal

		}
	case *ast.PatternNamespaceMemberExpression:
		return ast.ContinueTraversal

	case *ast.DoubleColonExpression:
		if ident == p.Element {
			return ast.ContinueTraversal

		}
	case *ast.DynamicMappingEntry:
		if ident == p.KeyVar || ident == p.GroupMatchingVariable {
			return ast.ContinueTraversal

		}
	case *ast.ForStatement, *ast.ForExpression, *ast.WalkStatement, *ast.WalkExpression,
		*ast.ObjectLiteral, *ast.MemberExpression, *ast.QuantityLiteral, *ast.RateLiteral,

		*ast.KeyListExpression:
		return ast.ContinueTraversal

	case *ast.MarkupOpeningTag:
		if ident == p.Name {
			return ast.ContinueTraversal
		}
	case *ast.MarkupPatternOpeningTag:
		if ident == p.Name {
			return ast.ContinueTraversal
		}
	case *ast.MarkupClosingTag:
		if ident == p.Name {
			return ast.ContinueTraversal
		}
	case *ast.MarkupPatternClosingTag:
		if ident == p.Name {
			return ast.ContinueTraversal
		}
	case *ast.MarkupAttribute:
		if ident == p.Name {
			return ast.ContinueTraversal
		}
	case *ast.MarkupPatternAttribute:
		if ident == p.Name {
			return ast.ContinueTraversal
		}
	}

	if _, ok := scopeNode.(*ast.ExtendStatement); ok {
		c.addError(ident, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES)
		return ast.ContinueTraversal
	}

	if _, ok := scopeNode.(*ast.StructDefinition); ok {
		c.addError(ident, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS)
		return ast.ContinueTraversal
	}

	if !c.varExists(ident.Name, ancestorChain) {
		if ident.Name == "const" {
			c.addError(ident, text.VAR_CONST_NOT_DECLARED_IF_YOU_MEANT_TO_DECLARE_CONSTANTS_GLOBAL_CONST_DECLS_ONLY_SUPPORTED_AT_THE_START_OF_THE_MODULE)
		} else {
			c.addError(ident, text.FmtVarIsNotDeclared(ident.Name))
		}
		return ast.ContinueTraversal
	}

	// if the variable is a global in a function expression or in a mapping entry we capture it
	if c.doGlobalVarExist(ident.Name, closestModule) {
		fnDecls := c.getModFunctionDecls(closestModule)

		if fnDecls[ident.Name] != nil && fnDecls[ident.Name].module == closestModule {

			//If the identifier references a  function then no global declarations (patterns, global variables)
			//should be located after this reference.

			if _, ok := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]; !ok {
				topLevelStmt, ok := ast.FindClosestTopLevelStatement(ident, ancestorChain)
				if !ok {
					panic(ErrUnreachable)
				}
				c.data.firstForbiddenPosForGlobalElementDecls[closestModule] = topLevelStmt.Base().Span.Start
			}
		}

		globalVarInfo := c.getModGlobalVars(closestModule)[ident.Name]

		switch scope := scopeNode.(type) {
		case *ast.FunctionExpression:
			c.data.addFnCapturedGlobal(scope, ident.Name, &globalVarInfo)
		case *ast.EmbeddedModule:
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

		case *ast.DynamicMappingEntry, *ast.StaticMappingEntry:
			mappingExpr := findClosest[*ast.MappingExpression](ancestorChain)
			c.data.addMappingCapturedGlobal(mappingExpr, ident.Name)
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkSelfExprAndSendValExpr(node, parent ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	isSelfExpr := true

	var objectLiteral *ast.ObjectLiteral
	var misplacementErr = text.SELF_ACCESSIBILITY_EXPLANATION
	isInExtensionMethod := false
	inReceptionHandler := false
	isSelfInStructMethod := false

	switch node.(type) {
	case *ast.SendValueExpression:
		isSelfExpr = false
		misplacementErr = text.MISPLACED_SENDVAL_EXPR
	}

loop:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		if !ast.IsScopeContainerNode(ancestorChain[i]) {
			continue
		}

		switch a := ancestorChain[i].(type) {
		case *ast.InitializationBlock:
			switch i {
			case 0:
			default:
				switch ancestorChain[i-1].(type) {
				case *ast.ObjectMetaProperty:
					if i == 1 {
						c.addError(node, text.CANNOT_CHECK_OBJECT_METAPROP_WITHOUT_PARENT)
						break
					}
				}

				switch ancestor := ancestorChain[i-2].(type) {
				case *ast.ObjectLiteral:
					objectLiteral = ancestor
				default:
				}
			}
			break loop
		case *ast.FunctionExpression:
			//Determine if the function is the method of an object, extension or struct.

			j := i - 1

			if j == -1 {
				break loop
			}

			maybeInReceptionHandler := false

			if _, ok := ancestorChain[j].(*ast.ReceptionHandlerExpression); ok {
				j--
				maybeInReceptionHandler = true
			}

			switch ancestorChain[j].(type) {
			case *ast.ObjectProperty:
				if j == 0 {
					c.addError(node, text.CANNOT_CHECK_OBJECT_PROP_WITHOUT_PARENT)
					break loop
				}
				j--

				objLit, ok := ancestorChain[j].(*ast.ObjectLiteral)
				if ok && j-1 >= 0 {

					if maybeInReceptionHandler {
						inReceptionHandler = true
						objectLiteral = objLit
					}

					isInExtensionMethod =
						utils.Implements[*ast.ExtendStatement](ancestorChain[j-1]) &&
							ancestorChain[j-1].(*ast.ExtendStatement).Extension == objLit

					if isInExtensionMethod {
						objectLiteral = objLit
					}
				}
			case *ast.FunctionDeclaration:
				if j == 0 {
					c.addError(node, text.CANNOT_CHECK_STRUCT_METHOD_DEF_WITHOUT_PARENT)
					break loop
				}
				_, ok := ancestorChain[j-1].(*ast.StructBody)
				isSelfInStructMethod = ok && isSelfExpr
			}

			break loop
		case *ast.EmbeddedModule:
			c.addError(node, misplacementErr)
			return ast.ContinueTraversal
		case *ast.Chunk:
			//
		case *ast.ExtendStatement:
			if isSelfExpr && node.Base().IncludedIn(a.Extension) { //ok
				return ast.ContinueTraversal
			}
		}
	}

	if !isSelfInStructMethod {
		if objectLiteral == nil {
			c.addError(node, misplacementErr)
			return ast.ContinueTraversal
		}

		_ = inReceptionHandler

		propInfo := c.getPropertyInfo(objectLiteral)

		switch p := parent.(type) {
		case *ast.MemberExpression:
			if !propInfo.known[p.PropertyName.Name] && !isInExtensionMethod {
				c.addError(p, text.FmtObjectDoesNotHaveProp(p.PropertyName.Name))
			}
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkPatternDef(node *ast.PatternDefinition, parent, closestModule ast.Node, inPreinitBlock bool) ast.TraversalAction {
	switch parent.(type) {
	case *ast.Chunk, *ast.EmbeddedModule:
	default:
		if !inPreinitBlock {
			c.addError(node, text.MISPLACED_PATTERN_DEF_NOT_TOP_LEVEL_STMT)
			return ast.Prune
		}
	}

	firstForbiddenPos := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
	if firstForbiddenPos != 0 && node.Base().Span.Start >= firstForbiddenPos {
		c.addError(node, text.MISPLACED_PATTERN_DEF_AFTER_FN_DECL_OR_REF_TO_FN)
		return ast.Prune
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
	return ast.ContinueTraversal
}

func (c *checker) checkPatternNamespaceDefinition(node *ast.PatternNamespaceDefinition, parent, closestModule ast.Node, inPreinitBlock bool) ast.TraversalAction {
	switch parent.(type) {
	case *ast.Chunk, *ast.EmbeddedModule:
	default:
		if !inPreinitBlock {
			c.addError(node, text.MISPLACED_PATTERN_NS_DEF_NOT_TOP_LEVEL_STMT)
			return ast.Prune
		}
	}

	firstForbiddenPos := c.data.firstForbiddenPosForGlobalElementDecls[closestModule]
	if firstForbiddenPos != 0 && node.Base().Span.Start >= firstForbiddenPos {
		c.addError(node, text.MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN)
		return ast.Prune
	}

	namespaceName, ok := node.NamespaceName()

	if ok {
		namespaces := c.getModPatternNamespaces(closestModule)
		if _, alreadyDefined := namespaces[namespaceName]; alreadyDefined && !inPreinitBlock {
			c.addError(node, text.FmtPatternNamespaceAlreadyDeclared(namespaceName))
		} else {
			patterns := map[string]int{}
			namespaces[namespaceName] = patternNamespaceInfo{patterns: patterns}

			var properties []*ast.ObjectProperty
			switch right := node.Right.(type) {
			case *ast.ObjectLiteral:
				properties = right.Properties
			case *ast.RecordLiteral:
				properties = right.Properties
			}

			for _, prop := range properties {
				if prop.HasNoKey() {
					continue
				}
				patterns[prop.Name()] = 0
			}
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkPatternIdentifier(node *ast.PatternIdentifierLiteral, parent, closestModule ast.Node, ancestorChain []ast.Node) ast.TraversalAction {

	switch parent := parent.(type) {
	case *ast.OtherPropsExpr:
		if node.Name == parse.NO_OTHERPROPS_PATTERN_NAME {
			return ast.ContinueTraversal
		}
	case *ast.StructDefinition:
		if parent.Name == node {
			return ast.ContinueTraversal
		}
	case *ast.MarkupPatternOpeningTag:
		if parent.Name == node {
			return ast.ContinueTraversal
		}
	case *ast.MarkupPatternClosingTag:
		if parent.Name == node {
			return ast.ContinueTraversal
		}
	}

	//Check if struct type.
	stuctDefs := c.getModStructDefs(closestModule)
	_, ok := stuctDefs[node.Name]
	if ok {
		//Check that the node is not misplaced.
		errMsg := ""
		switch parent := parent.(type) {
		case *ast.PointerType, *ast.StructFieldDefinition, *ast.NewExpression:
			//ok
		case *ast.FunctionParameter:
			errMsg = text.STRUCT_TYPES_NOT_ALLOWED_AS_PARAMETER_TYPES
		case *ast.FunctionExpression:
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

		return ast.ContinueTraversal

	}

	//Ignore the check if the pattern identifier refers to a pattern that is not yet defined.

	for _, a := range ancestorChain {
		if def, ok := a.(*ast.PatternDefinition); ok && def.IsLazy {
			return ast.ContinueTraversal
		}
	}

	//Check that the pattern is declared.

	name := node.Name
	patterns := c.getModPatterns(closestModule)
	if _, ok := patterns[name]; !ok {
		errMsg := ""
		switch parent.(type) {
		case *ast.PointerType, *ast.NewExpression:
			errMsg = text.FmtStructTypeIsNotDefined(name)
		default:
			errMsg = text.FmtPatternIsNotDeclared(name)
		}
		c.addError(node, errMsg)
	}
	return ast.ContinueTraversal
}

func (c *checker) checkPatternNamespaceIdentifier(node *ast.PatternNamespaceIdentifierLiteral, closestModule ast.Node) ast.TraversalAction {
	namespaceName := node.Name
	namespaces := c.getModPatternNamespaces(closestModule)

	if _, alreadyDefined := namespaces[namespaceName]; !alreadyDefined {
		c.addError(node, text.FmtPatternNamespaceIsNotDeclared(namespaceName))
	}

	return ast.ContinueTraversal
}

func (c *checker) checkPatternNamespaceMember(node *ast.PatternNamespaceMemberExpression, closestModule ast.Node) ast.TraversalAction {

	namespaceName := node.Namespace.Name
	namespaces := c.getModPatternNamespaces(closestModule)

	info, alreadyDefined := namespaces[namespaceName]
	if !alreadyDefined {
		//No error is reported because this is already done by checkPatternNamespaceIdentifier.
		return ast.ContinueTraversal
	}

	memberName := node.MemberName.Name
	_, ok := info.patterns[memberName]
	if !ok {
		c.addError(node.MemberName, text.FmtPatternNamespaceDoesNotHaveMember(namespaceName, memberName))
	}

	return ast.ContinueTraversal
}

func (c *checker) checkRuntimeTypeCheckExpr(node *ast.RuntimeTypeCheckExpression, parent ast.Node) ast.TraversalAction {
	switch p := parent.(type) {
	case *ast.CallExpression:
		for _, arg := range p.Arguments {
			if node == arg {
				return ast.ContinueTraversal
			}
		}

		c.addError(node, text.MISPLACED_RUNTIME_TYPECHECK_EXPRESSION)
	default:
		c.addError(node, text.MISPLACED_RUNTIME_TYPECHECK_EXPRESSION)
	}
	return ast.ContinueTraversal
}

func (c *checker) checkNewExpr(node *ast.NewExpression) ast.TraversalAction {
	typ := node.Type
	switch typ.(type) {
	case *ast.PatternIdentifierLiteral:
		//ok, the identifier will be checked next
	//TODO: support slices
	case nil:
		return ast.ContinueTraversal
	default:
		c.addError(node.Type, text.A_STRUCT_TYPE_NAME_IS_EXPECTED)
		return ast.ContinueTraversal
	}
	return ast.ContinueTraversal
}

func (c *checker) checkStructInitLiteral(node *ast.StructInitializationLiteral) ast.TraversalAction {
	// look for duplicate field names
	fieldNames := make([]string, 0, len(node.Fields))

	for _, field := range node.Fields {
		fieldInit, ok := field.(*ast.StructFieldInitialization)
		if ok {
			name := fieldInit.Name.Name
			if slices.Contains(fieldNames, name) {
				c.addError(fieldInit.Name, text.FmtDuplicateFieldName(name))
			} else {
				fieldNames = append(fieldNames, name)
			}
		}
	}
	return ast.ContinueTraversal
}

func (c *checker) checkPointerType(node *ast.PointerType, parent ast.Node) ast.TraversalAction {
	patternIdent, ok := node.ValueType.(*ast.PatternIdentifierLiteral)
	if !ok {
		c.addError(node.ValueType, text.A_STRUCT_TYPE_IS_EXPECTED_AFTER_THE_STAR)
	} else {
		//Check that the node is not misplaced.
		switch parent := parent.(type) {
		case *ast.StructFieldDefinition, *ast.FunctionParameter:
			//ok
		case *ast.FunctionExpression:
			if node != parent.ReturnType {
				c.addError(node, text.MISPLACED_POINTER_TYPE)
			}
		case *ast.LocalVariableDeclarator:
			if node != parent.Type {
				c.addError(node, text.MISPLACED_POINTER_TYPE)
			}
		default:
			c.addError(node, text.MISPLACED_POINTER_TYPE)
		}

		if symbolic.IsNameOfBuiltinComptimeType(patternIdent.Name) {
			//do not check the pattern identifier.
			return ast.Prune
		}

	}
	return ast.ContinueTraversal
}

func (c *checker) checkTestSuiteExpr(node *ast.TestSuiteExpression, ancestorChain []ast.Node) ast.TraversalAction {
	hasSubsuiteStmt := false
	hasTestCaseStmt := false

	for _, stmt := range node.Module.Statements {
		switch stmt := stmt.(type) {
		case *ast.TestCaseExpression:
			if stmt.IsStatement {
				hasTestCaseStmt = true
			}
		case *ast.TestSuiteExpression:
			if stmt.IsStatement {
				hasSubsuiteStmt = true
			}
		}
	}

	if hasSubsuiteStmt && hasTestCaseStmt {
		for _, stmt := range node.Module.Statements {
			switch stmt := stmt.(type) {
			case *ast.TestCaseExpression:
				if stmt.IsStatement {
					c.addError(stmt, text.TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT)
				}
			case *ast.TestSuiteExpression:
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
			case *ast.EmbeddedModule:
				if i-1 <= 0 {
					break search_test_case
				}
				testCaseExpr, ok := ancestorChain[i-1].(*ast.TestCaseExpression)
				if ok && testCaseExpr.IsStatement {
					c.addError(node, text.TEST_SUITE_STMTS_NOT_ALLOWED_INSIDE_TEST_CASE_STMTS)
					break search_test_case
				}
			}
		}
	}

	return ast.ContinueTraversal
}

func (c *checker) checkTestCaseExpr(node *ast.TestCaseExpression, ancestorChain []ast.Node) ast.TraversalAction {
	inTestSuite := false

search_test_suite:
	for i := len(ancestorChain) - 1; i >= 0; i-- {
		switch ancestorChain[i].(type) {
		case *ast.EmbeddedModule:
			if i-1 <= 0 {
				break search_test_suite
			}
			testSuiteExpr, ok := ancestorChain[i-1].(*ast.TestSuiteExpression)
			if ok {
				inTestSuite = testSuiteExpr.Module == ancestorChain[i]
				break search_test_suite
			}
		}
	}

	if !inTestSuite && node.IsStatement && (c.currentModule == nil || c.currentModule.Kind != inoxmod.TestSuiteModule) {
		c.addError(node, text.TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES)
	}

	return ast.ContinueTraversal
}

func (c *checker) checkEmbeddedModule(node *ast.EmbeddedModule, parent, parentModule ast.Node, ancestorChain []ast.Node) ast.TraversalAction {
	globals := c.getModGlobalVars(node)
	patterns := c.getModPatterns(node)
	patternNamespaces := c.getModPatternNamespaces(node)

	parentModuleGlobals := c.getModGlobalVars(parentModule)
	parentModulePatterns := c.getModPatterns(parentModule)
	parentModulePatternNamespaces := c.getModPatternNamespaces(parentModule)

	switch parent.(type) {
	case *ast.TestSuiteExpression:
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

	case *ast.TestCaseExpression:
		globals[globalnames.CURRENT_TEST] = GlobalVarInfo{IsConst: true, IsStartConstant: true}

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

	return ast.ContinueTraversal
}

// checkSingleNode perform post checks on a single node.
func (c *checker) postCheckSingleNode(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) ast.TraversalAction {

	closestModule := findClosestModule(ancestorChain)
	_ = closestModule

	switch n := node.(type) {
	case *ast.ObjectLiteral:
		//manifest

		if utils.Implements[*ast.Manifest](parent) {
			if len(ancestorChain) < 3 {
				c.addError(parent, text.CANNOT_CHECK_MANIFEST_WITHOUT_PARENT)
				break
			}

			chunk := ancestorChain[len(ancestorChain)-2]
			isEmbeddedModule := utils.Implements[*ast.EmbeddedModule](chunk)

			if isEmbeddedModule {
				var moduleKind inoxmod.Kind
				switch ancestorChain[len(ancestorChain)-3].(type) {
				case *ast.SpawnExpression:
					moduleKind = inoxmod.UserLThreadModule
				case *ast.TestSuiteExpression:
					moduleKind = inoxmod.TestSuiteModule
				case *ast.TestCaseExpression:
					moduleKind = inoxmod.TestCaseModule
				default:
					panic(ErrUnreachable)
				}

				CheckManifestObject(ManifestStaticCheckArguments{
					ObjectLit:             n,
					IgnoreUnknownSections: true,
					ModuleKind:            moduleKind,
					OnError: func(n ast.Node, msg string) {
						c.addError(n, msg)
					},
				})
			} //else: the manifest of regular modules is already checked during the pre-init phase
		}
	case *ast.ForStatement, *ast.ForExpression, *ast.WalkStatement, *ast.WalkExpression:
		varsBefore := c.store[node].(map[string]localVarInfo)
		c.setScopeLocalVars(scopeNode, varsBefore)
	case *ast.SwitchStatement, *ast.MatchStatement:
		varsBefore, ok := c.store[node]
		if ok {
			c.setScopeLocalVars(scopeNode, varsBefore.(map[string]localVarInfo))
		}
	case *ast.PreinitStatement:
		if c.furthestPreinitStmt == n {
			c.furthestPreinitStmt = nil
		}
	case *ast.AssertionStatement:
		if c.furthestAssertionStmt == n {
			c.furthestAssertionStmt = nil
		}
	case *ast.MarkupPatternExpression:
		if c.furthestMarkupPatternExpr == n {
			c.furthestMarkupPatternExpr = nil
		}
	}
	return ast.ContinueTraversal
}

func checkVisibilityInitializationBlock(propInfo *propertyInfo, block *ast.InitializationBlock, onError func(n ast.Node, msg string)) {
	if len(block.Statements) != 1 || !utils.Implements[*ast.ObjectLiteral](block.Statements[0]) {
		onError(block, text.INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ)
		return
	}

	objLiteral := block.Statements[0].(*ast.ObjectLiteral)

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
			_, ok := prop.Value.(*ast.KeyListExpression)
			if !ok {
				onError(prop, text.VAL_SHOULD_BE_KEYLIST_LIT)
				return
			}
		case "visible_by":
			dict, ok := prop.Value.(*ast.DictionaryLiteral)
			if !ok {
				onError(prop, text.VAL_SHOULD_BE_DICT_LIT)
				return
			}

			for _, entry := range dict.Entries {
				switch keyNode := entry.Key.(type) {
				case *ast.UnambiguousIdentifierLiteral:
					switch keyNode.Name {
					case "self":
						_, ok := entry.Value.(*ast.KeyListExpression)
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
	properties []*ast.ObjectProperty,
	spreadElements []*ast.PropertySpreadElement,
	isObject bool,
	addError func(n ast.Node, msg string),
) (ast.TraversalAction, map[string]struct{}) {
	keys := map[string]struct{}{}
	hasElements := false

	// look for duplicate keys
	for _, prop := range properties {
		var k string

		if prop.Type != nil {
			addError(prop.Type, "type annotation of properties is not allowed")
		}

		switch n := prop.Key.(type) {
		case *ast.DoubleQuotedStringLiteral:
			k = n.Value
		case *ast.IdentifierLiteral:
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

		extractionExpr, isValid := element.Expr.(*ast.ExtractionExpression)
		if !isValid {
			continue
		}

		for _, key := range extractionExpr.Keys.Keys {
			name := key.(*ast.IdentifierLiteral).Name

			_, found := keys[name]
			if found {
				addError(key, text.FmtDuplicateKey(name))
				return ast.ContinueTraversal, nil
			}
			keys[name] = struct{}{}
		}
	}

	return ast.ContinueTraversal, keys
}

// CombineStaticCheckErrors combines static check errors into a single error with a multiline message.
func CombineStaticCheckErrors(errs ...*Error) error {

	goErrors := make([]error, len(errs))
	for i, e := range errs {
		goErrors[i] = e
	}
	return utils.CombineErrors(goErrors...)
}

type Error struct {
	Message        string
	LocatedMessage string
	Location       sourcecode.SourcePositionStack
}

func NewError(s string, location sourcecode.SourcePositionStack) *Error {
	return &Error{
		Message:        CHECK_ERR_PREFIX + s,
		LocatedMessage: CHECK_ERR_PREFIX + location.String() + s,
		Location:       location,
	}
}

func (err Error) Error() string {
	return err.LocatedMessage
}

func (err Error) MessageWithoutLocation() string {
	return err.Message
}

func (err Error) LocationStack() sourcecode.SourcePositionStack {
	return err.Location
}

type StaticCheckWarning struct {
	Message        string
	LocatedMessage string
	Location       sourcecode.SourcePositionStack
}

func NewStaticCheckWarning(s string, location sourcecode.SourcePositionStack) *StaticCheckWarning {
	return &StaticCheckWarning{
		Message:        CHECK_ERR_PREFIX + s,
		LocatedMessage: CHECK_ERR_PREFIX + location.String() + s,
		Location:       location,
	}
}

func (err StaticCheckWarning) MessageWithoutLocation() string {
	return err.Message
}

func (err StaticCheckWarning) LocationStack() sourcecode.SourcePositionStack {
	return err.Location
}

func SamePointer(a, b interface{}) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
