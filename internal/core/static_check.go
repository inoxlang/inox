package internal

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	CHECK_ERR_PREFIX  = "check: "
	MAX_NAME_BYTE_LEN = 64
)

var (
	STATIC_CHECK_DATA_PROP_NAMES = []string{"errors"}
)

// StaticCheck performs various checks on an AST, like checking duplicate declarations and keys or checking that statements like return,
// break and continue are not misplaced. No type checks are performed.
func StaticCheck(input StaticCheckInput) (*StaticCheckData, error) {
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
		fnDecls:           make(map[parse.Node]map[string]int),
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
			fnData:      map[*parse.FunctionExpression]*FunctionStaticData{},
			mappingData: map[*parse.MappingExpression]*MappingStaticData{},
		},
	}

	err := checker.check(input.Node)
	if err != nil {
		return nil, err
	}
	return checker.data, combineStaticCheckErrors(checker.data.errors...)
}

// see Check function.
type checker struct {
	currentModule            *Module //can be nil
	chunk                    *parse.ParsedChunk
	inclusionImportStatement *parse.InclusionImportStatement // can be nil
	parentChecker            *checker                        //can be nil
	checkInput               StaticCheckInput

	//key: *parse.Chunk|*parse.EmbeddedModule
	fnDecls map[parse.Node]map[string]int

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

	store map[parse.Node]interface{}

	data *StaticCheckData
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

func (checker *checker) makeCheckingError(node parse.Node, s string) *StaticCheckError {
	location := checker.getSourcePositionStack(node)

	return NewStaticCheckError(s, location)
}

func (checker *checker) getSourcePositionStack(node parse.Node) parse.SourcePositionStack {
	var sourcePositionStack parse.SourcePositionStack

	if checker.parentChecker != nil {
		sourcePositionStack = checker.parentChecker.getSourcePositionStack(checker.inclusionImportStatement)
	}

	sourcePositionStack = append(sourcePositionStack, checker.chunk.GetSourcePosition(node.Base().Span))
	return sourcePositionStack
}

func (checker *checker) addError(node parse.Node, s string) {
	checker.data.errors = append(checker.data.errors, checker.makeCheckingError(node, s))
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

func (checker *checker) getModFunctionDecls(mod parse.Node) map[string]int {
	fns, ok := checker.fnDecls[mod]
	if !ok {
		fns = make(map[string]int)
		checker.fnDecls[mod] = fns
	}
	return fns
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

	//check that the node is allowed in assertion

	if closestAssertion != nil {
		switch n.(type) {
		case *parse.Variable, *parse.GlobalVariable, *parse.IdentifierLiteral, *parse.BinaryExpression,
			*parse.PatternIdentifierLiteral, *parse.ObjectPatternLiteral, *parse.ObjectProperty, *parse.ObjectPatternProperty,
			*parse.ListPatternLiteral, *parse.ObjectLiteral, *parse.ListLiteral, *parse.FunctionPatternExpression,
			*parse.PatternNamespaceIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
			*parse.OptionPatternLiteral, *parse.OptionalPatternExpression, *parse.MemberExpression, *parse.IdentifierMemberExpression:
		default:
			if !parse.NodeIsSimpleValueLiteral(n) {
				c.addError(n, fmtFollowingNodeTypeNotAllowedInAssertions(n))
			}
		}
	}

	//actually check the node

switch_:
	switch node := n.(type) {
	case *parse.IntegerRangeLiteral:
		if node.LowerBound.Value > node.UpperBound.Value {
			c.addError(n, LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND)
		}
	case *parse.QuantityLiteral:

		var prevMultiplier string
		var prevUnit string
		var prevDurationUnitValue time.Duration

		for partIndex := 0; partIndex < len(node.Values); partIndex++ {
			if node.Values[partIndex] < 0 {
				c.addError(n, ErrNegQuantityNotSupported.Error())
				return parse.Continue
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
				c.addError(node, fmtNonSupportedUnit(node.Units[0]))
				return parse.Continue
			}

			unit := node.Units[partIndex][i:]

			switch unit {
			case "x", "ln", "rn", "B":
				if partIndex != 0 || prevUnit != "" {
					c.addError(node, INVALID_QUANTITY)
					return parse.Continue
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
					c.addError(node, INVALID_QUANTITY)
					return parse.Continue
				}

				prevDurationUnitValue = durationUnitValue
				prevUnit = unit
			case "%":
				if partIndex != 0 || prevUnit != "" {
					c.addError(node, INVALID_QUANTITY)
					return parse.Continue
				}
				if i == 0 {
					prevUnit = unit
					break
				}
				fallthrough
			default:
				c.addError(node, fmtNonSupportedUnit(node.Units[0]))
				return parse.Continue
			}
		}

		_, err := evalQuantity(node.Values, node.Units)
		if err != nil {
			c.addError(node, err.Error())
		}

	case *parse.RateLiteral:

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
			case "x", "B":
				return parse.Continue
			}
		}
		c.addError(node, INVALID_RATE)
		return parse.Continue
	case *parse.URLLiteral:
		if strings.HasPrefix(node.Value, "mem://") && utils.Must(url.Parse(node.Value)).Host != MEM_HOSTNAME {
			c.addError(node, INVALID_MEM_HOST_ONLY_VALID_VALUE)
		}
	case *parse.HostLiteral:
		if strings.HasPrefix(node.Value, "mem://") && utils.Must(url.Parse(node.Value)).Host != MEM_HOSTNAME {
			c.addError(node, INVALID_MEM_HOST_ONLY_VALID_VALUE)
		}
	case *parse.ObjectLiteral:
		indexKey := 0
		keys := map[string]bool{}

		propInfo := c.getPropertyInfo(node)

		// look for duplicate keys
		for _, prop := range node.Properties {
			var k string

			var isExplicit bool

			switch n := prop.Key.(type) {
			case *parse.QuotedStringLiteral:
				k = n.Value
				isExplicit = true
			case *parse.IdentifierLiteral:
				k = n.Name
				isExplicit = true
			case nil:
				k = strconv.Itoa(indexKey)
				indexKey++
			}

			if len(k) > MAX_NAME_BYTE_LEN {
				c.addError(prop.Key, fmtNameIsTooLong(k))
			}

			if parse.IsMetadataKey(k) {
				c.addError(prop.Key, OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS)
			} else if prevIsExplicit, found := keys[k]; found {
				if isExplicit && !prevIsExplicit {
					c.addError(prop, fmtObjLitExplicityDeclaresPropWithImplicitKey(k))
				} else {
					c.addError(prop, fmtDuplicateKey(k))
				}
			}

			keys[k] = isExplicit
		}

		// also look for duplicate keys
		for _, element := range node.SpreadElements {

			for _, key := range element.Expr.(*parse.ExtractionExpression).Keys.Keys {
				name := key.(*parse.IdentifierLiteral).Name

				_, found := keys[name]
				if found {
					c.addError(key, fmtDuplicateKey(name))
					return parse.Continue
				}
				keys[name] = true
			}
		}

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
	case *parse.RecordLiteral:
		indexKey := 0
		keys := map[string]bool{}

		// look for duplicate keys
		for _, prop := range node.Properties {
			var k string

			var isExplicit bool

			switch n := prop.Key.(type) {
			case *parse.QuotedStringLiteral:
				k = n.Value
				isExplicit = true
			case *parse.IdentifierLiteral:
				k = n.Name
				isExplicit = true
			case nil:
				k = strconv.Itoa(indexKey)
				indexKey++
			}

			if len(k) > MAX_NAME_BYTE_LEN {
				c.addError(prop.Key, fmtNameIsTooLong(k))
				return parse.Continue
			}

			if parse.IsMetadataKey(k) {
				c.addError(prop.Key, OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS)
			} else if prevIsExplicit, found := keys[k]; found {
				if isExplicit && !prevIsExplicit {
					c.addError(prop, fmtRecLitExplicityDeclaresPropWithImplicitKey(k))
					return parse.Continue
				}
				c.addError(prop, fmtDuplicateKey(k))
				return parse.Continue
			}

			keys[k] = isExplicit
		}

		// also look for duplicate keys
		for _, element := range node.SpreadElements {

			for _, key := range element.Expr.(*parse.ExtractionExpression).Keys.Keys {
				name := key.(*parse.IdentifierLiteral).Name

				_, found := keys[name]
				if found {
					c.addError(key, fmtDuplicateKey(name))
					return parse.Continue
				}
				keys[name] = true
			}
		}

	case *parse.DictionaryLiteral:
		keys := map[string]bool{}

		// look for duplicate keys
		for _, entry := range node.Entries {

			keyRepr := entry.Key.(parse.SimpleValueLiteral).ValueString()

			if keys[keyRepr] {
				c.addError(entry.Key, fmtDuplicateDictKey(keyRepr))
			} else {
				keys[keyRepr] = true
			}
		}

	case *parse.SpawnExpression:

		var globals = make(map[string]globalVarInfo)
		var globalDescNode parse.Node

		//add constant globals
		parentModuleGlobals := c.getModGlobalVars(closestModule)
		for name, info := range parentModuleGlobals {
			if info.isStartConstant {
				globals[name] = info
			}
		}

		// add globals passed by user
		if obj, ok := node.Meta.(*parse.ObjectLiteral); ok {
			val, ok := obj.PropValue("globals")
			if ok {
				globalDescNode = val
			}
		} else if node.Meta != nil {
			c.addError(node.Meta, "only object literals are supported as meta's value")
		}

		switch desc := globalDescNode.(type) {
		case *parse.KeyListExpression:
			for _, ident := range desc.Keys {
				globVarName := ident.(*parse.IdentifierLiteral).Name
				if !c.doGlobalVarExist(globVarName, closestModule) {
					c.addError(globalDescNode, fmtCannotPassGlobalThatIsNotDeclaredToRoutine(globVarName))
				}
				globals[globVarName] = globalVarInfo{isConst: true}
			}
		case *parse.ObjectLiteral:
			for _, prop := range desc.Properties {
				globals[prop.Name()] = globalVarInfo{isConst: true}
			}
		case *parse.NilLiteral:
		case nil:
		default:
			c.addError(node, INVALID_SPAWN_GLOBALS_SHOULD_BE)
		}

		if node.Module != nil && node.Module.SingleCallExpr {
			calleeName := node.Module.Statements[0].(*parse.CallExpression).Callee.(*parse.IdentifierLiteral).Name
			globals[calleeName] = globalVarInfo{isConst: true}
		}

		embeddedModuleGlobals := c.getModGlobalVars(node.Module)

		for name, info := range globals {
			embeddedModuleGlobals[name] = info
		}
	case *parse.LifetimejobExpression:
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
			return parse.Continue
		}

		if prop, ok := parent.(*parse.ObjectProperty); !ok || !prop.HasImplicitKey() {
			c.addError(node, MISSING_LIFETIMEJOB_SUBJECT_PATTERN_NOT_AN_IMPLICIT_OBJ_PROP)
		}
	case *parse.ReceptionHandlerExpression:
		if prop, ok := parent.(*parse.ObjectProperty); !ok || !prop.HasImplicitKey() {
			c.addError(node, MISPLACED_RECEPTION_HANDLER_EXPRESSION)
		}

	case *parse.MappingExpression:

	case *parse.StaticMappingEntry:
		switch node.Key.(type) {
		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
		default:
			if !parse.NodeIsSimpleValueLiteral(node.Key) {
				c.addError(node.Key, INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS)
			}
		}

	case *parse.DynamicMappingEntry:
		switch node.Key.(type) {
		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
		default:
			if !parse.NodeIsSimpleValueLiteral(node.Key) {
				c.addError(node.Key, INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS)
			}
		}

		localVars := c.getLocalVarsInScope(node)
		varname := node.KeyVar.(*parse.IdentifierLiteral).Name
		localVars[varname] = localVarInfo{}

		if node.GroupMatchingVariable != nil {
			varname := node.GroupMatchingVariable.(*parse.IdentifierLiteral).Name
			localVars[varname] = localVarInfo{}
		}

	case *parse.ComputeExpression:

		if _, ok := scopeNode.(*parse.DynamicMappingEntry); !ok {
			c.addError(node, MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
		} else {
		ancestor_loop:
			for i := len(ancestorChain) - 1; i >= 0; i-- {
				ancestor := ancestorChain[i]
				if ancestor == scopeNode {
					break
				}

				switch a := ancestor.(type) {
				case *parse.StaticMappingEntry:
					c.addError(node, MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
					break ancestor_loop
				case *parse.DynamicMappingEntry:
					if a.Key == node || i < len(ancestorChain)-1 && ancestorChain[i+1] == a.Key {
						c.addError(node, MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY)
					}
					break ancestor_loop
				}
			}
		}

	case *parse.InclusionImportStatement:
		includedChunk := c.currentModule.InclusionStatementMap[node]

		globals := make(map[parse.Node]map[string]globalVarInfo)
		globals[includedChunk.Node] = map[string]globalVarInfo{}

		c.checkInput.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
			globals[includedChunk.Node][name] = globalVarInfo{isConst: isStartConstant}
			return nil
		})

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
			fnDecls:                  make(map[parse.Node]map[string]int),
			globalVars:               globals,
			localVars:                make(map[parse.Node]map[string]localVarInfo),
			properties:               make(map[*parse.ObjectLiteral]*propertyInfo),
			patterns:                 patterns,
			patternNamespaces:        patternNamespaces,
			currentModule:            c.currentModule,
			chunk:                    includedChunk.ParsedChunk,
			inclusionImportStatement: node,
			store:                    make(map[parse.Node]interface{}),
			data: &StaticCheckData{
				fnData:      map[*parse.FunctionExpression]*FunctionStaticData{},
				mappingData: map[*parse.MappingExpression]*MappingStaticData{},
			},
		}

		err := chunkChecker.check(includedChunk.Node)
		if err != nil {
			panic(err)
		}
		if len(chunkChecker.data.errors) != 0 {
			c.data.errors = append(c.data.errors, chunkChecker.data.errors...)
		}

		for k, v := range chunkChecker.data.fnData {
			c.data.fnData[k] = v
		}

		for k, v := range chunkChecker.data.mappingData {
			c.data.mappingData[k] = v
		}

		//include all global data & top level local variables
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
				c.addError(node, fmtCannotShadowGlobalVariable(k))
			} else {
				globalVars[k] = v
			}
		}

		for k, v := range chunkChecker.localVars[includedChunk.Node] {
			localVars := c.getLocalVarsInScope(closestModule)
			if _, ok := localVars[k]; ok {
				c.addError(node, fmtCannotShadowLocalVariable(k))
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
				c.addError(node, fmtPatternAlreadyDeclared(k))
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
				c.addError(node, fmtPatternNamespaceAlreadyDeclared(k))
			} else {
				namespaces[k] = v
			}
		}

		if v, ok := chunkChecker.store[includedChunk.Node]; ok {
			panic(fmt.Errorf("data stored for included chunk %#v : %#v", includedChunk.Node, v))
		}

	//ok
	case *parse.ImportStatement:
		name := node.Identifier.Name
		variables := c.getModGlobalVars(closestModule)

		_, alreadyUsed := variables[name]
		if alreadyUsed {
			c.addError(node, fmtInvalidImportStmtAlreadyDeclaredGlobal(name))
			return parse.Continue
		}
		variables[name] = globalVarInfo{isConst: true}
	case *parse.GlobalConstantDeclarations:
		globalVars := c.getModGlobalVars(closestModule)

		for _, decl := range node.Declarations {
			name := decl.Left.Name

			_, alreadyUsed := globalVars[name]
			if alreadyUsed {
				c.addError(decl, fmtInvalidConstDeclGlobalAlreadyDeclared(name))
				return parse.Continue
			}
			globalVars[name] = globalVarInfo{isConst: true}
		}
	case *parse.LocalVariableDeclarations:
		localVars := c.getLocalVarsInScope(scopeNode)

		for _, decl := range node.Declarations {
			name := decl.Left.(*parse.IdentifierLiteral).Name

			globalVariables := c.getModGlobalVars(closestModule)

			if _, alreadyDefined := globalVariables[name]; alreadyDefined {
				c.addError(decl, fmtCannotShadowGlobalVariable(name))
				return parse.Continue
			}

			_, alreadyUsed := localVars[name]
			if alreadyUsed {
				c.addError(decl, fmtInvalidLocalVarDeclAlreadyDeclared(name))
				return parse.Continue
			}
			localVars[name] = localVarInfo{}
		}
	case *parse.Assignment, *parse.MultiAssignment:
		var names []string

		if assignment, ok := n.(*parse.Assignment); ok {

			switch left := assignment.Left.(type) {

			case *parse.GlobalVariable:
				fns, ok := c.fnDecls[closestModule]
				if ok {
					_, alreadyUsed := fns[left.Name]
					if alreadyUsed {
						c.addError(node, fmtInvalidGlobalVarAssignmentNameIsFuncName(left.Name))
						return parse.Continue
					}
				}

				localVars := c.getLocalVarsInScope(scopeNode)

				if _, alreadyDefined := localVars[left.Name]; alreadyDefined {
					c.addError(node, fmtCannotShadowLocalVariable(left.Name))
					return parse.Continue
				}

				variables := c.getModGlobalVars(closestModule)

				varInfo, alreadyDefined := variables[left.Name]
				if alreadyDefined {
					if varInfo.isConst {
						c.addError(node, fmtInvalidGlobalVarAssignmentNameIsConstant(left.Name))
						return parse.Continue
					}
				} else {
					if assignment.Operator != parse.Assign {
						c.addError(node, fmtInvalidGlobalVarAssignmentVarDoesNotExist(left.Name))
					}
					variables[left.Name] = globalVarInfo{isConst: false}
				}

			case *parse.Variable:
				if left.Name == "" { //$
					c.addError(node, INVALID_ASSIGNMENT_ANONYMOUS_VAR_CANNOT_BE_ASSIGNED)
					return parse.Continue
				}

				globalVariables := c.getModGlobalVars(closestModule)

				if _, alreadyDefined := globalVariables[left.Name]; alreadyDefined {
					c.addError(node, fmtCannotShadowGlobalVariable(left.Name))
					return parse.Continue
				}

				localVars := c.getLocalVarsInScope(scopeNode)

				if _, alreadyDefined := localVars[left.Name]; !alreadyDefined && assignment.Operator != parse.Assign {
					c.addError(node, fmtInvalidVariableAssignmentVarDoesNotExist(left.Name))
				}

				names = append(names, left.Name)
			case *parse.IdentifierLiteral:
				globalVariables := c.getModGlobalVars(closestModule)

				if _, alreadyDefined := globalVariables[left.Name]; alreadyDefined {
					c.addError(node, fmtCannotShadowGlobalVariable(left.Name))
					return parse.Continue
				}

				localVars := c.getLocalVarsInScope(scopeNode)

				if _, alreadyDefined := localVars[left.Name]; !alreadyDefined && assignment.Operator != parse.Assign {
					c.addError(node, fmtInvalidVariableAssignmentVarDoesNotExist(left.Name))
				}

				names = append(names, left.Name)
			case *parse.IdentifierMemberExpression:

				for _, ident := range left.PropertyNames {
					if parse.IsMetadataKey(ident.Name) {
						c.addError(node, fmtInvalidMemberAssignmentCannotModifyMetaProperty(ident.Name))
					}
				}
			case *parse.MemberExpression:
				curr := left
				var ok bool
				for {
					if parse.IsMetadataKey(curr.PropertyName.Name) {
						c.addError(node, fmtInvalidMemberAssignmentCannotModifyMetaProperty(curr.PropertyName.Name))
						break
					}
					if curr, ok = curr.Left.(*parse.MemberExpression); !ok {
						break
					}
				}
			}
		} else {
			assignment := n.(*parse.MultiAssignment)

			for _, variable := range assignment.Variables {
				name := variable.(*parse.IdentifierLiteral).Name

				globalVariables := c.getModGlobalVars(closestModule)

				if _, alreadyDefined := globalVariables[name]; alreadyDefined {
					c.addError(node, fmtCannotShadowGlobalVariable(name))
				}

				names = append(names, name)
			}
		}

		for _, name := range names {
			variables := c.getLocalVarsInScope(scopeNode)
			variables[name] = localVarInfo{}
		}

	case *parse.ForStatement:
		variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
		variables := c.getLocalVarsInScope(scopeNode)

		c.store[node] = variablesBeforeStmt

		if node.KeyIndexIdent != nil {
			if _, alreadyDefined := variables[node.KeyIndexIdent.Name]; alreadyDefined &&
				!c.shellLocalVars[node.KeyIndexIdent.Name] {
				c.addError(node, fmtCannotShadowVariable(node.KeyIndexIdent.Name))
				return parse.Continue
			}
			variables[node.KeyIndexIdent.Name] = localVarInfo{}
		}

		if node.ValueElemIdent != nil {
			if _, alreadyDefined := variables[node.ValueElemIdent.Name]; alreadyDefined &&
				!c.shellLocalVars[node.ValueElemIdent.Name] {
				c.addError(node, fmtCannotShadowVariable(node.ValueElemIdent.Name))
				return parse.Continue
			}
			variables[node.ValueElemIdent.Name] = localVarInfo{}
		}

	case *parse.WalkStatement:
		variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
		variables := c.getLocalVarsInScope(scopeNode)

		c.store[node] = variablesBeforeStmt

		if node.EntryIdent != nil {
			if _, alreadyDefined := variables[node.EntryIdent.Name]; alreadyDefined &&
				!c.shellLocalVars[node.EntryIdent.Name] {
				c.addError(node, fmtCannotShadowVariable(node.EntryIdent.Name))
				return parse.Continue
			}
			variables[node.EntryIdent.Name] = localVarInfo{}
		}

	case *parse.FunctionDeclaration:

		switch parent.(type) {
		case *parse.Chunk, *parse.EmbeddedModule:
			fns := c.getModFunctionDecls(closestModule)
			globVars := c.getModGlobalVars(closestModule)

			_, alreadyDeclared := fns[node.Name.Name]
			if alreadyDeclared {
				c.addError(node, fmtInvalidFnDeclAlreadyDeclared(node.Name.Name))
				return parse.Continue
			}

			_, alreadyUsed := globVars[node.Name.Name]
			if alreadyUsed {
				c.addError(node, fmtInvalidFnDeclGlobVarExist(node.Name.Name))
				return parse.Continue
			}

			fns[node.Name.Name] = 0
			globVars[node.Name.Name] = globalVarInfo{isConst: true, fnExpr: node.Function}
		default:
			c.addError(node, INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT)
			return parse.Continue
		}
	case *parse.FunctionExpression:
		fnLocalVars := c.getLocalVarsInScope(node)

		//we check that the captured variable exists & is a local
		for _, e := range node.CaptureList {
			name := e.(*parse.IdentifierLiteral).Name

			if !c.varExists(name, ancestorChain) {
				c.addError(node, fmtVarIsNotDeclared(name))
			} else if c.doGlobalVarExist(name, closestModule) {
				c.addError(node, fmtCannotPassGlobalToFunction(name))
			}

			fnLocalVars[name] = localVarInfo{}
		}

		for _, p := range node.Parameters {
			name := p.Var.Name

			globalVariables := c.getModGlobalVars(closestModule)

			if _, alreadyDefined := globalVariables[name]; alreadyDefined {
				c.addError(p, fmtParameterCannotShadowGlobalVariable(name))
				return parse.Continue
			}

			fnLocalVars[name] = localVarInfo{}
		}
	case *parse.FunctionPatternExpression:
		fnLocalVars := c.getLocalVarsInScope(node)

		for _, p := range node.Parameters {
			if p.Var == nil {
				continue
			}

			name := p.Var.Name

			globalVariables := c.getModGlobalVars(closestModule)

			if _, alreadyDefined := globalVariables[name]; alreadyDefined {
				c.addError(p, fmtParameterCannotShadowGlobalVariable(name))
				return parse.Continue
			}

			fnLocalVars[name] = localVarInfo{}
		}

	case *parse.YieldStatement:
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
			c.addError(node, MISPLACE_YIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES)
		}
	case *parse.BreakStatement, *parse.ContinueStatement:
		iterativeStmtIndex := -1

		//we search for the last iterative statement in the ancestor chain
	loop0:
		for i := len(ancestorChain) - 1; i >= 0; i-- {
			switch ancestorChain[i].(type) {
			case *parse.ForStatement, *parse.WalkStatement:
				iterativeStmtIndex = i
				break loop0
			}
		}

		if iterativeStmtIndex < 0 {
			c.addError(node, INVALID_BREAK_OR_CONTINUE_STMT_SHOULD_BE_IN_A_FOR_OR_WALK_STMT)
			return parse.Continue
		}

		for i := iterativeStmtIndex + 1; i < len(ancestorChain); i++ {
			switch ancestorChain[i].(type) {
			case *parse.IfStatement, *parse.SwitchStatement, *parse.SwitchCase,
				*parse.MatchCase, *parse.MatchStatement, *parse.Block:
			default:
				c.addError(node, INVALID_BREAK_OR_CONTINUE_STMT_SHOULD_BE_IN_A_FOR_OR_WALK_STMT)
				return parse.Continue
			}
		}
	case *parse.PruneStatement:
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
			c.addError(node, INVALID_PRUNE_STMT_SHOULD_BE_IN_WALK_STMT)
			return parse.Continue
		}

		for i := walkStmtIndex + 1; i < len(ancestorChain); i++ {
			switch ancestorChain[i].(type) {
			case *parse.IfStatement, *parse.SwitchStatement, *parse.MatchStatement, *parse.Block, *parse.ForStatement:
			default:
				c.addError(node, INVALID_PRUNE_STMT_SHOULD_BE_IN_WALK_STMT)
				return parse.Continue
			}
		}
	case *parse.MatchStatement:
		variablesBeforeStmt := c.getScopeLocalVarsCopy(scopeNode)
		c.store[node] = variablesBeforeStmt
	case *parse.MatchCase:
		//define the variables named after groups if the literal is used as a case in a match statement

		if node.GroupMatchingVariable == nil {
			break
		}

		variable := node.GroupMatchingVariable.(*parse.IdentifierLiteral)

		if _, alreadyDefined := c.getModGlobalVars(closestModule)[variable.Name]; alreadyDefined {
			c.addError(variable, fmtCannotShadowGlobalVariable(variable.Name))
			return parse.Continue
		}

		localVars := c.getLocalVarsInScope(scopeNode)

		if info, alreadyDefined := localVars[variable.Name]; alreadyDefined && info != (localVarInfo{isGroupMatchingVar: true}) {
			c.addError(variable, fmtCannotShadowLocalVariable(variable.Name))
			return parse.Continue
		}

		localVars[variable.Name] = localVarInfo{isGroupMatchingVar: true}
	case *parse.Variable:
		if len(node.Name) > MAX_NAME_BYTE_LEN {
			c.addError(node, fmtNameIsTooLong(node.Name))
			return parse.Continue
		}

		if node.Name == "" {
			break
		}

		if _, isLazyExpr := scopeNode.(*parse.LazyExpression); isLazyExpr {
			break
		}

		variables := c.getLocalVarsInScope(scopeNode)
		_, exist := variables[node.Name]

		if !exist {
			c.addError(node, fmtLocalVarIsNotDeclared(node.Name))
			return parse.Continue
		}

	case *parse.GlobalVariable:
		if len(node.Name) > MAX_NAME_BYTE_LEN {
			c.addError(node, fmtNameIsTooLong(node.Name))
			return parse.Continue
		}

		if _, isAssignment := parent.(*parse.Assignment); isAssignment {
			if fnExpr, ok := scopeNode.(*parse.FunctionExpression); ok {
				c.data.addFnAssigningGlobal(fnExpr)
			}
			break
		}

		if _, isLazyExpr := scopeNode.(*parse.LazyExpression); isLazyExpr {
			break
		}
		globalVars := c.getModGlobalVars(closestModule)
		globalVarInfo, exist := globalVars[node.Name]

		if !exist {
			c.addError(node, fmtGlobalVarIsNotDeclared(node.Name))
			return parse.Continue
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

	case *parse.IdentifierLiteral:

		if len(node.Name) > MAX_NAME_BYTE_LEN {
			c.addError(node, fmtNameIsTooLong(node.Name))
			return parse.Continue
		}

		if _, ok := scopeNode.(*parse.LazyExpression); ok {
			break
		}

		//we check the parent to know if the identifier refers to a variable
		switch p := parent.(type) {
		case *parse.CallExpression:
			if p.CommandLikeSyntax && !node.IncludedIn(p.Callee) {
				break switch_
			}
		case *parse.ObjectProperty:
			if p.Key == node {
				break switch_
			}
		case *parse.ObjectPatternProperty:
			if p.Key == node {
				break switch_
			}
		case *parse.ObjectMetaProperty:
			if p.Key == node {
				break switch_
			}
		case *parse.IdentifierMemberExpression:
			if node != p.Left {
				break switch_
			}
		case *parse.DynamicMemberExpression:
			if node != p.Left {
				break switch_
			}
		case *parse.PatternNamespaceMemberExpression:
			break switch_
		case *parse.DynamicMappingEntry:
			if node == p.KeyVar || node == p.GroupMatchingVariable {
				break switch_
			}
		case *parse.ForStatement, *parse.WalkStatement, *parse.ObjectLiteral, *parse.FunctionDeclaration, *parse.MemberExpression, *parse.QuantityLiteral, *parse.RateLiteral,
			*parse.KeyListExpression:
			break switch_
		case *parse.XMLOpeningElement:
			if node == p.Name {
				break switch_
			}
		case *parse.XMLClosingElement:
			if node == p.Name {
				break switch_
			}
		case *parse.XMLAttribute:
			if node == p.Name {
				break switch_
			}
		}

		if !c.varExists(node.Name, ancestorChain) {
			c.addError(node, fmtVarIsNotDeclared(node.Name))
			return parse.Continue
		}

		// if the variable is a global in a function expression or in a mapping entry we capture it
		if c.doGlobalVarExist(node.Name, closestModule) {
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

	case *parse.SelfExpression, *parse.SendValueExpression, *parse.SupersysExpression:

		var objectLiteral *parse.ObjectLiteral

		var misplacementErr = SELF_ACCESSIBILITY_EXPLANATION
		switch node.(type) {
		case *parse.SendValueExpression:
			misplacementErr = MISPLACED_SENDVAL_EXPR
		case *parse.SupersysExpression:
			misplacementErr = MISPLACED_SUPERSYS_EXPR
		}

	loop:
		for i := len(ancestorChain) - 1; i >= 0; i-- {
			if !parse.IsScopeContainerNode(ancestorChain[i]) {
				continue
			}

			switch ancestorChain[i].(type) {
			case *parse.InitializationBlock:
				switch i {
				case 0:
				default:
					switch ancestorChain[i-1].(type) {
					case *parse.ObjectMetaProperty:
						if i == 1 {
							c.addError(node, CANNOT_CHECK_OBJECT_METAPROP_WITHOUT_PARENT)
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
				j := i - 1
				switch j {
				case -1:
				default:

					if _, ok := ancestorChain[j].(*parse.ReceptionHandlerExpression); ok {
						j--
					}

					switch ancestorChain[j].(type) {
					case *parse.ObjectProperty:
						if j == 0 {
							c.addError(node, CANNOT_CHECK_OBJECT_PROP_WITHOUT_PARENT)
							break loop
						}
						j--
						switch ancestor := ancestorChain[j].(type) {
						case *parse.ObjectLiteral:
							objectLiteral = ancestor
						default:
						}
					}

				}
				break loop
			case *parse.EmbeddedModule: //ok if lifetime job
				if i == 0 || !parse.NodeIs(ancestorChain[i-1], &parse.LifetimejobExpression{}) {
					c.addError(node, misplacementErr)
				}
				return parse.Continue
			case *parse.Chunk:
				if c.currentModule != nil && c.currentModule.ModuleKind == LifetimeJobModule { // ok
					return parse.Continue
				}
			}
		}

		if objectLiteral == nil {
			c.addError(node, misplacementErr)
			return parse.Continue
		}

		propInfo := c.getPropertyInfo(objectLiteral)

		switch p := parent.(type) {
		case *parse.MemberExpression:
			if !propInfo.known[p.PropertyName.Name] {
				c.addError(p, fmtObjectDoesNotHaveProp(p.PropertyName.Name))
			}
		}

	case *parse.PatternDefinition:
		patternName := node.Left.Name
		patterns := c.getModPatterns(closestModule)
		if _, alreadyDefined := patterns[patternName]; alreadyDefined {
			c.addError(node, fmtPatternAlreadyDeclared(patternName))
		} else {
			patterns[patternName] = 0
		}
	case *parse.PatternNamespaceDefinition:
		namespaceName := node.Left.Name
		namespaces := c.getModPatternNamespaces(closestModule)
		if _, alreadyDefined := namespaces[namespaceName]; alreadyDefined {
			c.addError(node, fmtPatternAlreadyDeclared(namespaceName))
		} else {
			namespaces[namespaceName] = 0
		}
	case *parse.PatternNamespaceIdentifierLiteral:
		namespaceName := node.Name
		namespaces := c.getModPatternNamespaces(closestModule)

		if _, alreadyDefined := namespaces[namespaceName]; !alreadyDefined {
			c.addError(node, fmtPatternNamespaceIsNotDeclared(namespaceName))
		}
	case *parse.PatternIdentifierLiteral:

		for _, a := range ancestorChain {
			if def, ok := a.(*parse.PatternDefinition); ok && def.IsLazy {
				break switch_
			}
		}

		patterns := c.getModPatterns(closestModule)
		if _, ok := patterns[node.Name]; !ok {
			c.addError(node, fmtPatternIsNotDeclared(node.Name))
		}
	case *parse.RuntimeTypeCheckExpression:
		switch p := parent.(type) {
		case *parse.CallExpression:
			for _, arg := range p.Arguments {
				if n == arg {
					break switch_ //ok
				}
			}

			c.addError(node, MISPLACED_RUNTIME_TYPECHECK_EXPRESSION)
		default:
			c.addError(node, MISPLACED_RUNTIME_TYPECHECK_EXPRESSION)
		}
	case *parse.DynamicMemberExpression:
		if node.Optional {
			c.addError(node, OPTIONAL_DYN_MEMB_EXPR_NOT_SUPPORTED_YET)
		}
	}

	return parse.Continue
}

// checkSingleNode perform post checks on a single node.
func (checker *checker) postCheckSingleNode(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) parse.TraversalAction {

	closestModule := findClosestModule(ancestorChain)
	_ = closestModule

	switch n := node.(type) {
	case *parse.ObjectLiteral:
		//manifest

		switch p := parent.(type) {
		case *parse.Manifest:
			checkManifestObject(n, false, func(n parse.Node, msg string) {
				checker.addError(n, msg)
			})
		case *parse.EmbeddedModule:
			if p.Manifest == nil || p.Manifest.Object != node {
				break
			}
			checkManifestObject(n, false, func(n parse.Node, msg string) {
				checker.addError(n, msg)
			})
		}
	case *parse.ForStatement, *parse.WalkStatement:
		varsBefore := checker.store[node].(map[string]localVarInfo)
		checker.setScopeLocalVars(scopeNode, varsBefore)
	case *parse.MatchStatement:
		varsBefore, ok := checker.store[node]
		if ok {
			checker.setScopeLocalVars(scopeNode, varsBefore.(map[string]localVarInfo))
		}
	}
	return parse.Continue
}

func checkManifestObject(objLit *parse.ObjectLiteral, ignoreUnknownSections bool, onError func(n parse.Node, msg string)) {
	parse.Walk(objLit, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		case *parse.ObjectLiteral:
			if len(n.SpreadElements) != 0 {
				onError(n, NO_SPREAD_IN_MANIFEST)
			}
		case *parse.ListLiteral:
			if n.HasSpreadElements() {
				onError(n, NO_SPREAD_IN_MANIFEST)
			}
		}

		return parse.Continue, nil
	}, nil)

	for _, p := range objLit.Properties {
		if p.HasImplicitKey() {
			onError(p, IMPLICIT_KEY_PROPS_NOT_ALLOWED_IN_MANIFEST)
			continue
		}

		switch p.Name() {
		case "permissions":
			if obj, ok := p.Value.(*parse.ObjectLiteral); ok {
				checkPermissionListingObject(obj, onError)
			} else {
				onError(p, PERMS_SECTION_SHOULD_BE_AN_OBJECT)
			}
		case "host_resolution":
			dict, ok := p.Value.(*parse.DictionaryLiteral)
			if !ok {
				onError(p, HOST_RESOL_SECTION_SHOULD_BE_A_DICT)
				continue
			}

			parse.Walk(dict, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if node == dict {
					return parse.Continue, nil
				}

				switch n := node.(type) {
				case *parse.DictionaryEntry, parse.SimpleValueLiteral, *parse.GlobalVariable:
				default:
					onError(n, fmtForbiddenNodeInHostResolutionSection(n))
				}

				return parse.Continue, nil
			}, nil)

		case "limits":
			obj, ok := p.Value.(*parse.ObjectLiteral)

			if !ok {
				onError(p, LIMITS_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			parse.Walk(obj, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if node == obj {
					return parse.Continue, nil
				}

				switch n := node.(type) {
				case *parse.ObjectProperty, parse.SimpleValueLiteral, *parse.GlobalVariable:
				default:
					onError(n, fmtForbiddenNodeInLimitsSection(n))
				}

				return parse.Continue, nil
			}, nil)
		case "env":
			patt, ok := p.Value.(*parse.ObjectPatternLiteral)

			if !ok {
				onError(p, ENV_SECTION_SHOULD_BE_AN_OBJECT_PATTERN)
				continue
			}

			parse.Walk(patt, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
				if node == patt {
					return parse.Continue, nil
				}

				switch n := node.(type) {
				case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
					*parse.ObjectPatternProperty, *parse.PatternCallExpression, parse.SimpleValueLiteral, *parse.GlobalVariable:
				default:
					onError(n, fmtForbiddenNodeInEnvSection(n))
				}

				return parse.Continue, nil
			}, nil)
		case "parameters":
			obj, ok := p.Value.(*parse.ObjectLiteral)

			if !ok {
				onError(p, PARAMS_SECTION_SHOULD_BE_AN_OBJECT)
				continue
			}

			checkParametersObject(obj, onError)
		default:
			if !ignoreUnknownSections {
				onError(p, fmtUnknownSectionOfManifest(p.Name()))
			}
		}
	}

}

func checkPermissionListingObject(objLit *parse.ObjectLiteral, onError func(n parse.Node, msg string)) {
	parse.Walk(objLit, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		case *parse.ObjectLiteral, *parse.ListLiteral, *parse.DictionaryLiteral, *parse.DictionaryEntry, *parse.ObjectProperty,
			parse.SimpleValueLiteral, *parse.GlobalVariable:
		default:
			onError(n, fmtForbiddenNodeInPermListing(n))
		}

		return parse.Continue, nil
	}, nil)

	for _, p := range objLit.Properties {
		if p.HasImplicitKey() {
			onError(p, IMPLICIT_KEY_PROPS_NOT_ALLOWED_IN_PERMS_SECTION)
			continue
		}

		if !permkind.IsPermissionKindName(p.Name()) {
			onError(p.Key, fmtNotValidPermissionKindName(p.Name()))
		}
	}
}

func checkParametersObject(objLit *parse.ObjectLiteral, onError func(n parse.Node, msg string)) {

	parse.Walk(objLit, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		if node == objLit {
			return parse.Continue, nil
		}

		switch n := node.(type) {
		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression,
			*parse.ObjectProperty, *parse.ObjectPatternProperty, *parse.ObjectLiteral, *parse.ListLiteral,
			*parse.OptionPatternLiteral, *parse.OptionExpression, *parse.ListPatternLiteral, *parse.OptionalPatternExpression,
			*parse.PatternCallExpression, parse.SimpleValueLiteral, *parse.GlobalVariable:
		default:
			onError(n, fmtForbiddenNodeInParametersSection(n))
		}

		return parse.Continue, nil
	}, nil)

	positionalParamsEnd := false

	for _, prop := range objLit.Properties {
		if !prop.HasImplicitKey() { // non positional parameter
			positionalParamsEnd = true

			propValue := prop.Value
			optionPattern, isOptionPattern := prop.Value.(*parse.OptionPatternLiteral)
			if isOptionPattern {
				propValue = optionPattern.Value
			}

			switch propVal := propValue.(type) {
			case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
				//ok
			case *parse.ObjectLiteral:
				if isOptionPattern {
					break
				}

				missingPropertyNames := []string{"pattern"}

				for _, paramDescProp := range propVal.Properties {
					if paramDescProp.HasImplicitKey() {
						continue
					}
					name := paramDescProp.Name()

					for i, name := range missingPropertyNames {
						if name == paramDescProp.Name() {
							missingPropertyNames[i] = ""
						}
					}

					switch name {
					case "pattern":
						switch paramDescProp.Value.(type) {
						case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
						default:
							onError(paramDescProp, "the .pattern of a non positional parameter should be a named pattern (%path, %str, ...)")
						}
					case "default":
					case "char-name":
						switch paramDescProp.Value.(type) {
						case *parse.RuneLiteral:
						default:
							onError(paramDescProp, "the .char-name of a non positional parameter should be a rune literal")
						}
					case "description":
						switch paramDescProp.Value.(type) {
						case *parse.QuotedStringLiteral, *parse.MultilineStringLiteral:
						default:
							onError(paramDescProp, "the .description of a non positional parameter should be a string literal")
						}
					}
				}

				missingPropertyNames = utils.FilterSlice(missingPropertyNames, func(s string) bool { return s != "" })
				if len(missingPropertyNames) > 0 {
					onError(prop, "missing properties in description of non positional parameter: "+strings.Join(missingPropertyNames, ", "))
				}
			default:
				onError(prop,
					"the description of a non positional parameter should be a named pattern (%path, %str, ...) or "+
						"an option pattern (%-o=%path, ...)",
				)
			}

		} else if positionalParamsEnd {
			onError(prop, "properties with an implicit key describe positional parameters, all implict key properties should be at the top of the 'parameters' section")
		} else { //positional parameter

			obj, ok := prop.Value.(*parse.ObjectLiteral)
			if !ok {
				onError(prop, "the description of a positional parameter should be an object")
				continue
			}

			missingPropertyNames := []string{"name", "pattern"}

			for _, paramDescProp := range obj.Properties {
				if paramDescProp.HasImplicitKey() {
					onError(paramDescProp, "the description of a positional parameter should not contain implicit keys")
					continue
				}

				propName := paramDescProp.Name()

				for i, name := range missingPropertyNames {
					if name == propName {
						missingPropertyNames[i] = ""
					}
				}

				switch propName {
				case "description":
					switch paramDescProp.Value.(type) {
					case *parse.QuotedStringLiteral, *parse.MultilineStringLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be a string literal")
					}
				case "rest":
					switch paramDescProp.Value.(type) {
					case *parse.BooleanLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be a string literal")
					}
				case "name":
					switch paramDescProp.Value.(type) {
					case *parse.UnambiguousIdentifierLiteral:
					default:
						onError(paramDescProp, "the .description property of a positional parameter should be an identifier (ex: #dir)")
					}
				case "pattern":
					switch paramDescProp.Value.(type) {
					case *parse.UnambiguousIdentifierLiteral:
					default:
						switch paramDescProp.Value.(type) {
						case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
						default:
							onError(paramDescProp, "the .pattern of a non positional parameter should be a named pattern (ex: %path, %str, ...)")
						}
					}
				}
			}

			missingPropertyNames = utils.FilterSlice(missingPropertyNames, func(s string) bool { return s != "" })
			if len(missingPropertyNames) > 0 {
				onError(prop, "missing properties in description of positional parameter: "+strings.Join(missingPropertyNames, ", "))
			}
			//TODO: check unique rest parameter
			_ = obj
		}
	}
}

func checkVisibilityInitializationBlock(propInfo *propertyInfo, block *parse.InitializationBlock, onError func(n parse.Node, msg string)) {
	if len(block.Statements) != 1 || !parse.NodeIs(block.Statements[0], &parse.ObjectLiteral{}) {
		onError(block, INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ)
		return
	}

	objLiteral := block.Statements[0].(*parse.ObjectLiteral)

	if len(objLiteral.MetaProperties) != 0 {
		onError(objLiteral, INVALID_VISIB_DESC_SHOULDNT_HAVE_METAPROPS)
	}

	for _, prop := range objLiteral.Properties {
		if prop.HasImplicitKey() {
			onError(objLiteral, INVALID_VISIB_DESC_SHOULDNT_HAVE_IMPLICIT_KEYS)
			return
		}

		switch prop.Name() {
		case "public":
			_, ok := prop.Value.(*parse.KeyListExpression)
			if !ok {
				onError(prop, VAL_SHOULD_BE_KEYLIST_LIT)
				return
			}
		case "visible_by":
			dict, ok := prop.Value.(*parse.DictionaryLiteral)
			if !ok {
				onError(prop, VAL_SHOULD_BE_DICT_LIT)
				return
			}

			for _, entry := range dict.Entries {
				switch keyNode := entry.Key.(type) {
				case *parse.UnambiguousIdentifierLiteral:
					switch keyNode.Name {
					case "self":
						_, ok := entry.Value.(*parse.KeyListExpression)
						if !ok {
							onError(entry, VAL_SHOULD_BE_KEYLIST_LIT)
							return
						}
					default:
						onError(entry, INVALID_VISIBILITY_DESC_KEY)
					}
				default:
					onError(entry, INVALID_VISIBILITY_DESC_KEY)
					return
				}
			}
		default:
			onError(prop, INVALID_VISIBILITY_DESC_KEY)
			return
		}
	}
}

// combineErrors combines errors into a single error with a multiline message.
func combineErrors(errs ...error) error {

	if len(errs) == 0 {
		return nil
	}

	finalErrBuff := bytes.NewBuffer(nil)

	for _, err := range errs {
		finalErrBuff.WriteString(err.Error())
		finalErrBuff.WriteRune('\n')
	}

	return errors.New(strings.TrimRight(finalErrBuff.String(), "\n"))
}

// combineParsingErrorValues combines errors into a single error with a multiline message.
func combineParsingErrorValues(errs []Error, positions []parse.SourcePositionRange) error {

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

	return combineErrors(goErrors...)
}

// combineStaticCheckErrors combines static check errors into a single error with a multiline message.
func combineStaticCheckErrors(errs ...*StaticCheckError) error {

	goErrors := make([]error, len(errs))
	for i, e := range errs {
		goErrors[i] = e
	}
	return combineErrors(goErrors...)
}

type StaticCheckInput struct {
	Node                   parse.Node
	Module                 *Module
	Chunk                  *parse.ParsedChunk
	ParentChecker          *checker
	Globals                GlobalVariables
	AdditionalGlobalConsts []string
	ShellLocalVars         map[string]Value
	Patterns               map[string]Pattern
	PatternNamespaces      map[string]*PatternNamespace
}

// A StaticCheckData is the immutable data produced by statically checking a module.
type StaticCheckData struct {
	NoReprMixin
	NotClonableMixin

	errors      []*StaticCheckError
	fnData      map[*parse.FunctionExpression]*FunctionStaticData
	mappingData map[*parse.MappingExpression]*MappingStaticData

	//.errors property accessible from scripts
	errorsPropSet atomic.Bool
	errorsProp    *Tuple
}

// Errors returns all errors in the code after a static check, the result should not be modified.
func (d *StaticCheckData) Errors() []*StaticCheckError {
	return d.errors
}

func (d *StaticCheckData) ErrorTuple() *Tuple {
	if d.errorsPropSet.CompareAndSwap(false, true) {
		errors := make([]Value, len(d.errors))
		for i, err := range d.errors {
			errors[i] = err.Err()
		}
		d.errorsProp = NewTuple(errors)
	}
	return d.errorsProp
}

func (d *StaticCheckData) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *StaticCheckData) Prop(ctx *Context, name string) Value {
	switch name {
	case "errors":
		return d.ErrorTuple()
	}

	method, ok := d.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, d))
	}
	return method
}

func (*StaticCheckData) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*StaticCheckData) PropertyNames(ctx *Context) []string {
	return STATIC_CHECK_DATA_PROP_NAMES
}

type FunctionStaticData struct {
	capturedGlobals []string
	assignGlobal    bool
}

type MappingStaticData struct {
	referencedGlobals []string
}

func (data *StaticCheckData) addFnCapturedGlobal(fnExpr *parse.FunctionExpression, name string, optionalInfo *globalVarInfo) {
	fnData := data.fnData[fnExpr]
	if fnData == nil {
		fnData = &FunctionStaticData{}
		data.fnData[fnExpr] = fnData
	}

	if !utils.SliceContains(fnData.capturedGlobals, name) {
		fnData.capturedGlobals = append(fnData.capturedGlobals, name)
	}

	if optionalInfo != nil && optionalInfo.fnExpr != nil {
		capturedGlobalFnData := data.GetFnData(optionalInfo.fnExpr)
		if capturedGlobalFnData != nil {
			for _, name := range capturedGlobalFnData.capturedGlobals {
				if utils.SliceContains(fnData.capturedGlobals, name) {
					continue
				}

				fnData.capturedGlobals = append(fnData.capturedGlobals, name)
			}
		}
	}
}

func (data *StaticCheckData) addMappingCapturedGlobal(expr *parse.MappingExpression, name string) {
	mappingData := data.mappingData[expr]
	if mappingData == nil {
		mappingData = &MappingStaticData{}
		data.mappingData[expr] = mappingData
	}

	if !utils.SliceContains(mappingData.referencedGlobals, name) {
		mappingData.referencedGlobals = append(mappingData.referencedGlobals, name)
	}
}

func (data *StaticCheckData) addFnAssigningGlobal(fnExpr *parse.FunctionExpression) {
	fnData := data.fnData[fnExpr]
	if fnData == nil {
		fnData = &FunctionStaticData{}
		data.fnData[fnExpr] = fnData
	}

	fnData.assignGlobal = true
}

func (data *StaticCheckData) GetFnData(fnExpr *parse.FunctionExpression) *FunctionStaticData {
	return data.fnData[fnExpr]
}

func (data *StaticCheckData) GetMappingData(expr *parse.MappingExpression) *MappingStaticData {
	return data.mappingData[expr]
}
