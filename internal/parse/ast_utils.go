package parse

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"slices"
)

// NodeIsStringLiteral returns true if and only if node is of one of the following types:
// *QuotedStringLiteral, *UnquotedStringLiteral, *StringTemplateLiteral, *MultilineStringLiteral
func NodeIsStringLiteral(node Node) bool {
	switch node.(type) {
	case *DoubleQuotedStringLiteral, *UnquotedStringLiteral, *StringTemplateLiteral, *MultilineStringLiteral:
		return true
	}
	return false
}

func NodeIsSimpleValueLiteral(node Node) bool {
	_, ok := node.(SimpleValueLiteral)
	return ok
}

func NodeIsPattern(node Node) bool {
	switch node.(type) {
	case *PatternCallExpression,
		*ListPatternLiteral, *TuplePatternLiteral,
		*ObjectPatternLiteral, *RecordPatternLiteral,
		*DictionaryPatternLiteral,
		*PatternIdentifierLiteral, *PatternNamespaceMemberExpression,
		*ComplexStringPatternPiece, //not 100% correct since it can be included in another *ComplexStringPatternPiece,
		*PatternConversionExpression,
		*PatternUnion,
		*PathPatternExpression, *AbsolutePathPatternLiteral, *RelativePathPatternLiteral,
		*URLPatternLiteral, *HostPatternLiteral, *OptionalPatternExpression,
		*OptionPatternLiteral, *FunctionPatternExpression, *NamedSegmentPathPatternLiteral, *ReadonlyPatternExpression,
		*RegularExpressionLiteral:
		return true
	}
	return false
}

func NodeIs[T Node](node Node, typ T) bool {
	return reflect.TypeOf(typ) == reflect.TypeOf(node)
}

// shifts the span of all nodes in node by offset
func shiftNodeSpans(node Node, offset int32) {
	ancestorChain := make([]Node, 0)

	walk(node, nil, &ancestorChain, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
		node.BasePtr().Span.Start += offset
		node.BasePtr().Span.End += offset

		// tokens := node.BasePtr().Tokens
		// for i, token := range tokens {
		// 	token.Span.Start += offset
		// 	token.Span.End += offset
		// 	tokens[i] = token
		// }
		return ContinueTraversal, nil
	}, nil)
}

type TraversalAction int
type TraversalOrder int

const (
	ContinueTraversal TraversalAction = iota
	Prune
	StopTraversal
)

type NodeHandler = func(node Node, parent Node, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error)

// This functions performs a pre-order traversal on an AST (depth first).
// postHandle is called on a node after all its descendants have been visited.
func Walk(node Node, handle, postHandle NodeHandler) (err error) {
	defer func() {
		v := recover()

		switch val := v.(type) {
		case error:
			err = fmt.Errorf("%s:%w", debug.Stack(), val)
		case nil:
		case TraversalAction:
		default:
			panic(v)
		}
	}()

	ancestorChain := make([]Node, 0)
	walk(node, nil, &ancestorChain, handle, postHandle)
	return
}

func walk(node, parent Node, ancestorChain *[]Node, fn, afterFn NodeHandler) {

	if node == nil || reflect.ValueOf(node).IsNil() {
		return
	}

	if ancestorChain != nil {
		*ancestorChain = append((*ancestorChain), parent)
		defer func() {
			*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
		}()
	}

	var scopeNode = parent
	for _, a := range *ancestorChain {
		if IsScopeContainerNode(a) {
			scopeNode = a
		}
	}

	if fn != nil {
		action, err := fn(node, parent, scopeNode, *ancestorChain, false)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopTraversal:
			panic(StopTraversal)
		case Prune:
			return
		}
	}

	switch n := node.(type) {
	case *Chunk:
		walk(n.GlobalConstantDeclarations, node, ancestorChain, fn, afterFn)
		walk(n.IncludableChunkDesc, node, ancestorChain, fn, afterFn)
		walk(n.Preinit, node, ancestorChain, fn, afterFn)
		walk(n.Manifest, node, ancestorChain, fn, afterFn)

		for _, stmt := range n.RegionHeaders {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *PreinitStatement:
		walk(n.Block, node, ancestorChain, fn, afterFn)
	case *Manifest:
		walk(n.Object, node, ancestorChain, fn, afterFn)
	case *EmbeddedModule:
		walk(n.Manifest, node, ancestorChain, fn, afterFn)

		for _, stmt := range n.RegionHeaders {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *OptionExpression:
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *PermissionDroppingStatement:
		walk(n.Object, node, ancestorChain, fn, afterFn)
	case *ImportStatement:
		walk(n.Identifier, node, ancestorChain, fn, afterFn)
		walk(n.Source, node, ancestorChain, fn, afterFn)
		walk(n.Configuration, node, ancestorChain, fn, afterFn)
	case *InclusionImportStatement:
		walk(n.Source, node, ancestorChain, fn, afterFn)
	case *SpawnExpression:
		walk(n.Meta, node, ancestorChain, fn, afterFn)
		walk(n.Module, node, ancestorChain, fn, afterFn)
	case *MappingExpression:
		for _, entry := range n.Entries {
			walk(entry, node, ancestorChain, fn, afterFn)
		}
	case *StaticMappingEntry:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *DynamicMappingEntry:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.KeyVar, node, ancestorChain, fn, afterFn)
		walk(n.GroupMatchingVariable, node, ancestorChain, fn, afterFn)
		walk(n.ValueComputation, node, ancestorChain, fn, afterFn)
	case *ComputeExpression:
		walk(n.Arg, node, ancestorChain, fn, afterFn)
	case *TreedataLiteral:
		walk(n.Root, node, ancestorChain, fn, afterFn)

		for _, entry := range n.Children {
			walk(entry, node, ancestorChain, fn, afterFn)
		}
	case *TreedataEntry:
		walk(n.Value, node, ancestorChain, fn, afterFn)
		for _, entry := range n.Children {
			walk(entry, node, ancestorChain, fn, afterFn)
		}
	case *TreedataPair:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *ListLiteral:
		walk(n.TypeAnnotation, node, ancestorChain, fn, afterFn)
		for _, element := range n.Elements {
			walk(element, node, ancestorChain, fn, afterFn)
		}
	case *TupleLiteral:
		walk(n.TypeAnnotation, node, ancestorChain, fn, afterFn)
		for _, element := range n.Elements {
			walk(element, node, ancestorChain, fn, afterFn)
		}
	case *ElementSpreadElement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *OptionPatternLiteral:
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *Block:
		for _, stmt := range n.RegionHeaders {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *SynchronizedBlockStatement:
		for _, val := range n.SynchronizedValues {
			walk(val, node, ancestorChain, fn, afterFn)
		}
		walk(n.Block, node, ancestorChain, fn, afterFn)
	case *InitializationBlock:
		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *FunctionDeclaration:
		walk(n.Annotations, node, ancestorChain, fn, afterFn)
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Function, node, ancestorChain, fn, afterFn)
	case *FunctionExpression:
		for _, e := range n.CaptureList {
			walk(e, node, ancestorChain, fn, afterFn)
		}

		for _, p := range n.Parameters {
			walk(p, node, ancestorChain, fn, afterFn)
		}

		walk(n.ReturnType, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *FunctionPatternExpression:
		for _, p := range n.Parameters {
			walk(p, node, ancestorChain, fn, afterFn)
		}

		walk(n.ReturnType, node, ancestorChain, fn, afterFn)
	case *ReadonlyPatternExpression:
		walk(n.Pattern, node, ancestorChain, fn, afterFn)
	case *FunctionParameter:
		walk(n.Var, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
	case *StructDefinition:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *StructBody:
		for _, def := range n.Definitions {
			walk(def, node, ancestorChain, fn, afterFn)
		}
	case *StructFieldDefinition:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
	case *NewExpression:
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Initialization, node, ancestorChain, fn, afterFn)
	case *StructInitializationLiteral:
		for _, fieldInit := range n.Fields {
			walk(fieldInit, node, ancestorChain, fn, afterFn)
		}
	case *StructFieldInitialization:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *PointerType:
		walk(n.ValueType, node, ancestorChain, fn, afterFn)
	case *DereferenceExpression:
		walk(n.Pointer, node, ancestorChain, fn, afterFn)
	case *PatternConversionExpression:
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *GlobalConstantDeclarations:
		for _, decl := range n.Declarations {
			walk(decl, node, ancestorChain, fn, afterFn)
		}
	case *GlobalConstantDeclaration:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *LocalVariableDeclarations:
		for _, decl := range n.Declarations {
			walk(decl, node, ancestorChain, fn, afterFn)
		}
	case *LocalVariableDeclarator:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *GlobalVariableDeclarations:
		for _, decl := range n.Declarations {
			walk(decl, node, ancestorChain, fn, afterFn)
		}
	case *GlobalVariableDeclarator:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *ObjectDestructuration:
		for _, prop := range n.Properties {
			walk(prop, node, ancestorChain, fn, afterFn)
		}
	case *ObjectDestructurationProperty:
		walk(n.PropertyName, node, ancestorChain, fn, afterFn)
		walk(n.NewName, node, ancestorChain, fn, afterFn)
	case *ObjectLiteral:
		for _, prop := range n.Properties {
			walk(prop, node, ancestorChain, fn, afterFn)
		}
		for _, prop := range n.MetaProperties {
			walk(prop, node, ancestorChain, fn, afterFn)
		}
		for _, el := range n.SpreadElements {
			walk(el, node, ancestorChain, fn, afterFn)
		}
	case *RecordLiteral:
		for _, prop := range n.Properties {
			walk(prop, node, ancestorChain, fn, afterFn)
		}
		for _, el := range n.SpreadElements {
			walk(el, node, ancestorChain, fn, afterFn)
		}
	case *ObjectProperty:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *ObjectPatternProperty:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
		walk(n.Annotations, node, ancestorChain, fn, afterFn)
	case *ObjectMetaProperty:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Initialization, node, ancestorChain, fn, afterFn)
	case *PropertySpreadElement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *PatternPropertySpreadElement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *OptionalPatternExpression:
		walk(n.Pattern, node, ancestorChain, fn, afterFn)
	case *ObjectPatternLiteral:
		for _, prop := range n.Properties {
			walk(prop, node, ancestorChain, fn, afterFn)
		}
		for _, el := range n.SpreadElements {
			walk(el, node, ancestorChain, fn, afterFn)
		}
		for _, otherProps := range n.OtherProperties {
			walk(otherProps, node, ancestorChain, fn, afterFn)
		}
	case *OtherPropsExpr:
		walk(n.Pattern, node, ancestorChain, fn, afterFn)
	case *DictionaryPatternLiteral:
		for _, entry := range n.Entries {
			walk(entry, node, ancestorChain, fn, afterFn)
		}
	case *DictionaryPatternEntry:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *ListPatternLiteral:
		for _, elem := range n.Elements {
			walk(elem, node, ancestorChain, fn, afterFn)
		}
		walk(n.GeneralElement, node, ancestorChain, fn, afterFn)
	case *RecordPatternLiteral:
		for _, prop := range n.Properties {
			walk(prop, node, ancestorChain, fn, afterFn)
		}
		for _, el := range n.SpreadElements {
			walk(el, node, ancestorChain, fn, afterFn)
		}
		for _, otherProps := range n.OtherProperties {
			walk(otherProps, node, ancestorChain, fn, afterFn)
		}
	case *TuplePatternLiteral:
		for _, elem := range n.Elements {
			walk(elem, node, ancestorChain, fn, afterFn)
		}
		walk(n.GeneralElement, node, ancestorChain, fn, afterFn)
	case *DictionaryLiteral:
		for _, entry := range n.Entries {
			walk(entry, node, ancestorChain, fn, afterFn)
		}
	case *DictionaryEntry:
		walk(n.Key, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *MemberExpression:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.PropertyName, node, ancestorChain, fn, afterFn)
	case *ComputedMemberExpression:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.PropertyName, node, ancestorChain, fn, afterFn)
	case *ExtractionExpression:
		walk(n.Object, node, ancestorChain, fn, afterFn)
		walk(n.Keys, node, ancestorChain, fn, afterFn)
	case *IndexExpression:
		walk(n.Indexed, node, ancestorChain, fn, afterFn)
		walk(n.Index, node, ancestorChain, fn, afterFn)
	case *SliceExpression:
		walk(n.Indexed, node, ancestorChain, fn, afterFn)
		walk(n.StartIndex, node, ancestorChain, fn, afterFn)
		walk(n.EndIndex, node, ancestorChain, fn, afterFn)
	case *DoubleColonExpression:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Element, node, ancestorChain, fn, afterFn)
	case *IdentifierMemberExpression:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		for _, p := range n.PropertyNames {
			walk(p, node, ancestorChain, fn, afterFn)
		}
	case *KeyListExpression:
		for _, key := range n.Keys {
			walk(key, node, ancestorChain, fn, afterFn)
		}
	case *BooleanConversionExpression:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *Assignment:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *MultiAssignment:
		for _, vr := range n.Variables {
			walk(vr, node, ancestorChain, fn, afterFn)
		}
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *CallExpression:
		walk(n.Callee, node, ancestorChain, fn, afterFn)
		for _, arg := range n.Arguments {
			walk(arg, node, ancestorChain, fn, afterFn)
		}
	case *SpreadArgument:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *IfStatement:
		walk(n.Test, node, ancestorChain, fn, afterFn)
		walk(n.Consequent, node, ancestorChain, fn, afterFn)
		walk(n.Alternate, node, ancestorChain, fn, afterFn)
	case *IfExpression:
		walk(n.Test, node, ancestorChain, fn, afterFn)
		walk(n.Consequent, node, ancestorChain, fn, afterFn)
		walk(n.Alternate, node, ancestorChain, fn, afterFn)
	case *ForStatement:
		walk(n.KeyPattern, node, ancestorChain, fn, afterFn)
		walk(n.KeyIndexIdent, node, ancestorChain, fn, afterFn)
		walk(n.ValuePattern, node, ancestorChain, fn, afterFn)
		walk(n.ValueElemIdent, node, ancestorChain, fn, afterFn)
		walk(n.IteratedValue, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *ForExpression:
		walk(n.KeyPattern, node, ancestorChain, fn, afterFn)
		walk(n.KeyIndexIdent, node, ancestorChain, fn, afterFn)
		walk(n.ValuePattern, node, ancestorChain, fn, afterFn)
		walk(n.ValueElemIdent, node, ancestorChain, fn, afterFn)
		walk(n.IteratedValue, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *WalkStatement:
		walk(n.Walked, node, ancestorChain, fn, afterFn)
		walk(n.MetaIdent, node, ancestorChain, fn, afterFn)
		walk(n.EntryIdent, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *ReturnStatement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *CoyieldStatement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *BreakStatement:
		walk(n.Label, node, ancestorChain, fn, afterFn)
	case *ContinueStatement:
		walk(n.Label, node, ancestorChain, fn, afterFn)
	case *YieldStatement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *SwitchStatement:
		walk(n.Discriminant, node, ancestorChain, fn, afterFn)
		for _, switchCase := range n.Cases {
			walk(switchCase, node, ancestorChain, fn, afterFn)
		}
		for _, defaultCase := range n.DefaultCases {
			walk(defaultCase, node, ancestorChain, fn, afterFn)
		}
	case *SwitchStatementCase:
		for _, val := range n.Values {
			walk(val, node, ancestorChain, fn, afterFn)
		}
		walk(n.Block, node, ancestorChain, fn, afterFn)
	case *MatchStatement:
		walk(n.Discriminant, node, ancestorChain, fn, afterFn)
		for _, matchCase := range n.Cases {
			walk(matchCase, node, ancestorChain, fn, afterFn)
		}
		for _, defaultCase := range n.DefaultCases {
			walk(defaultCase, node, ancestorChain, fn, afterFn)
		}
	case *MatchStatementCase:
		walk(n.GroupMatchingVariable, node, ancestorChain, fn, afterFn)
		for _, val := range n.Values {
			walk(val, node, ancestorChain, fn, afterFn)
		}
		walk(n.Block, node, ancestorChain, fn, afterFn)
	case *DefaultCaseWithBlock:
		walk(n.Block, node, ancestorChain, fn, afterFn)
	case *SwitchExpression:
		walk(n.Discriminant, node, ancestorChain, fn, afterFn)
		for _, switchCase := range n.Cases {
			walk(switchCase, node, ancestorChain, fn, afterFn)
		}
		for _, defaultCase := range n.DefaultCases {
			walk(defaultCase, node, ancestorChain, fn, afterFn)
		}
	case *SwitchExpressionCase:
		for _, val := range n.Values {
			walk(val, node, ancestorChain, fn, afterFn)
		}
		walk(n.Result, node, ancestorChain, fn, afterFn)
	case *MatchExpression:
		walk(n.Discriminant, node, ancestorChain, fn, afterFn)
		for _, switchCase := range n.Cases {
			walk(switchCase, node, ancestorChain, fn, afterFn)
		}
		for _, defaultCase := range n.DefaultCases {
			walk(defaultCase, node, ancestorChain, fn, afterFn)
		}
	case *MatchExpressionCase:
		for _, val := range n.Values {
			walk(val, node, ancestorChain, fn, afterFn)
		}
		walk(n.Result, node, ancestorChain, fn, afterFn)
	case *DefaultCaseWithResult:
		walk(n.Result, node, ancestorChain, fn, afterFn)
	case *QuotedExpression:
		walk(n.Expression, node, ancestorChain, fn, afterFn)
	case *QuotedStatements:
		for _, stmt := range n.RegionHeaders {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *UnquotedRegion:
		walk(n.Expression, node, ancestorChain, fn, afterFn)
	case *UnaryExpression:
		walk(n.Operand, node, ancestorChain, fn, afterFn)
	case *BinaryExpression:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *UpperBoundRangeExpression:
		walk(n.UpperBound, node, ancestorChain, fn, afterFn)
	case *IntegerRangeLiteral:
		walk(n.LowerBound, node, ancestorChain, fn, afterFn)
		walk(n.UpperBound, node, ancestorChain, fn, afterFn)
	case *FloatRangeLiteral:
		walk(n.LowerBound, node, ancestorChain, fn, afterFn)
		walk(n.UpperBound, node, ancestorChain, fn, afterFn)
	case *QuantityRangeLiteral:
		walk(n.LowerBound, node, ancestorChain, fn, afterFn)
		walk(n.UpperBound, node, ancestorChain, fn, afterFn)
	case *RuneRangeExpression:
		walk(n.Lower, node, ancestorChain, fn, afterFn)
		walk(n.Upper, node, ancestorChain, fn, afterFn)
	case *StringTemplateLiteral:
		walk(n.Pattern, node, ancestorChain, fn, afterFn)
		for _, e := range n.Slices {
			walk(e, node, ancestorChain, fn, afterFn)
		}
	case *StringTemplateInterpolation:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *NamedSegmentPathPatternLiteral:
		for _, e := range n.Slices {
			walk(e, node, ancestorChain, fn, afterFn)
		}
	case *PathPatternExpression:
		for _, e := range n.Slices {
			walk(e, node, ancestorChain, fn, afterFn)
		}
	case *AbsolutePathExpression:
		for _, e := range n.Slices {
			walk(e, node, ancestorChain, fn, afterFn)
		}
	case *RelativePathExpression:
		for _, e := range n.Slices {
			walk(e, node, ancestorChain, fn, afterFn)
		}
	case *URLExpression:
		walk(n.HostPart, node, ancestorChain, fn, afterFn)
		for _, pathNode := range n.Path {
			walk(pathNode, node, ancestorChain, fn, afterFn)
		}
		for _, param := range n.QueryParams {
			walk(param, node, ancestorChain, fn, afterFn)
		}
	case *HostExpression:
		walk(n.Scheme, node, ancestorChain, fn, afterFn)
		walk(n.Host, node, ancestorChain, fn, afterFn)
	case *URLQueryParameter:
		for _, val := range n.Value {
			walk(val, node, ancestorChain, fn, afterFn)
		}
	case *PipelineStatement:
		for _, stage := range n.Stages {
			walk(stage.Expr, node, ancestorChain, fn, afterFn)
		}
	case *PipelineExpression:
		for _, stage := range n.Stages {
			walk(stage.Expr, node, ancestorChain, fn, afterFn)
		}
	case *PatternDefinition:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *PatternNamespaceDefinition:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *PatternNamespaceMemberExpression:
		walk(n.Namespace, node, ancestorChain, fn, afterFn)
		walk(n.MemberName, node, ancestorChain, fn, afterFn)
	case *ComplexStringPatternPiece:
		for _, element := range n.Elements {
			walk(element.GroupName, node, ancestorChain, fn, afterFn)
			walk(element, node, ancestorChain, fn, afterFn)
		}
	case *PatternPieceElement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *PatternUnion:
		for _, case_ := range n.Cases {
			walk(case_, node, ancestorChain, fn, afterFn)
		}
	case *PatternCallExpression:
		walk(n.Callee, node, ancestorChain, fn, afterFn)
		for _, arg := range n.Arguments {
			walk(arg, node, ancestorChain, fn, afterFn)
		}

	case *ConcatenationExpression:
		for _, el := range n.Elements {
			walk(el, node, ancestorChain, fn, afterFn)
		}
	case *AssertionStatement:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *RuntimeTypeCheckExpression:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *TestSuiteExpression:
		walk(n.Meta, node, ancestorChain, fn, afterFn)
		walk(n.Module, node, ancestorChain, fn, afterFn)
	case *TestCaseExpression:
		walk(n.Meta, node, ancestorChain, fn, afterFn)
		walk(n.Module, node, ancestorChain, fn, afterFn)
	case *ReceptionHandlerExpression:
		walk(n.Pattern, node, ancestorChain, fn, afterFn)
		walk(n.Handler, node, ancestorChain, fn, afterFn)
	case *SendValueExpression:
		walk(n.Value, node, ancestorChain, fn, afterFn)
		walk(n.Receiver, node, ancestorChain, fn, afterFn)
	case *CssSelectorExpression:
		for _, el := range n.Elements {
			walk(el, node, ancestorChain, fn, afterFn)
		}
	case *CssAttributeSelector:
		walk(n.AttributeName, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *MarkupExpression:
		walk(n.Namespace, node, ancestorChain, fn, afterFn)
		walk(n.Element, node, ancestorChain, fn, afterFn)
	case *MarkupElement:
		walk(n.Opening, node, ancestorChain, fn, afterFn)
		for _, header := range n.RegionHeaders {
			walk(header, node, ancestorChain, fn, afterFn)
		}
		for _, child := range n.Children {
			walk(child, node, ancestorChain, fn, afterFn)
		}
		walk(n.Closing, node, ancestorChain, fn, afterFn)
	case *MarkupOpeningTag:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		for _, attr := range n.Attributes {
			walk(attr, node, ancestorChain, fn, afterFn)
		}
	case *MarkupAttribute:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *MarkupClosingTag:
		walk(n.Name, node, ancestorChain, fn, afterFn)
	case *MarkupInterpolation:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *MarkupPatternExpression:
		walk(n.Element, node, ancestorChain, fn, afterFn)
	case *MarkupPatternElement:
		walk(n.Opening, node, ancestorChain, fn, afterFn)
		for _, header := range n.RegionHeaders {
			walk(header, node, ancestorChain, fn, afterFn)
		}
		for _, child := range n.Children {
			walk(child, node, ancestorChain, fn, afterFn)
		}
		walk(n.Closing, node, ancestorChain, fn, afterFn)
	case *MarkupPatternOpeningTag:
		walk(n.Name, node, ancestorChain, fn, afterFn)

		for _, attr := range n.Attributes {
			walk(attr, node, ancestorChain, fn, afterFn)
		}
	case *MarkupPatternAttribute:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
	case *MarkupPatternClosingTag:
		walk(n.Name, node, ancestorChain, fn, afterFn)
	case *MarkupPatternInterpolation:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *ExtendStatement:
		walk(n.ExtendedPattern, node, ancestorChain, fn, afterFn)
		walk(n.Extension, node, ancestorChain, fn, afterFn)
	case *MetadataAnnotations:
		for _, expr := range n.Expressions {
			walk(expr, node, ancestorChain, fn, afterFn)
		}
	case *AnnotatedRegionHeader:
		walk(n.Text, node, ancestorChain, fn, afterFn)
		walk(n.Annotations, node, ancestorChain, fn, afterFn)
	case *LongValuePathLiteral:
		for _, segment := range n.Segments {
			walk(segment, node, ancestorChain, fn, afterFn)
		}
	case *MissingStatement:
		walk(n.Annotations, node, ancestorChain, fn, afterFn)
	}

	if afterFn != nil {
		action, err := afterFn(node, parent, scopeNode, *ancestorChain, true)

		if err != nil {
			panic(err)
		}

		switch action {
		case StopTraversal:
			panic(StopTraversal)
		}
	}
}

func CountNodes(n Node) (count int) {
	Walk(n, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		count += 1
		return ContinueTraversal, nil
	}, nil)

	return
}

func FindNodeWithSpan(root Node, searchedNodeSpan NodeSpan) (n Node, found bool) {
	Walk(root, func(node, _, _ Node, _ []Node, _ bool) (TraversalAction, error) {

		nodeSpan := node.Base().Span
		if searchedNodeSpan.End < nodeSpan.Start || searchedNodeSpan.Start >= nodeSpan.End {
			return Prune, nil
		}

		if searchedNodeSpan == nodeSpan {
			n = node
			found = true
			return StopTraversal, nil
		}
		return ContinueTraversal, nil
	}, nil)

	return
}

func FindNodes[T Node](root Node, typ T, handle func(n T) bool) []T {
	n, _ := FindNodesAndChains(root, typ, handle)
	return n
}

func FindNodesAndChains[T Node](root Node, typ T, handle func(n T) bool) ([]T, [][]Node) {
	searchedType := reflect.TypeOf(typ)
	var found []T
	var ancestors [][]Node

	Walk(root, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if reflect.TypeOf(node) == searchedType {
			if handle == nil || handle(node.(T)) {
				found = append(found, node.(T))
				ancestors = append(ancestors, slices.Clone(ancestorChain))
			}
		}
		return ContinueTraversal, nil
	}, nil)

	return found, ancestors
}

// FindNodeAndChain walks over an AST node and returns the first node of type $typ for which $handle returns true.
// If $handle is nil only the type is checked.
func FindNode[T Node](root Node, typ T, handle func(n T, isFirstFound bool, ancestors []Node) bool) T {
	n, _ := FindNodeAndChain(root, typ, handle)
	return n
}

// FindNodeAndChain walks over an AST node and returns the first node of type $typ.
func FindFirstNode[T Node](root Node, typ T) T {
	n, _ := FindNodeAndChain(root, typ, func(n T, isUnique bool, ancestors []Node) bool {
		return isUnique
	})
	return n
}

// FindNodeAndChain walks over an AST node and returns the first node of type $typ (and its ancestors) for which $handle returns true.
// If $handle is nil only the type is checked.
func FindNodeAndChain[T Node](root Node, typ T, handle func(n T, isFirstFound bool, ancestors []Node) bool) (T, []Node) {
	searchedType := reflect.TypeOf(typ)
	isUnique := true

	var found T
	var _ancestorChain []Node

	Walk(root, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if reflect.TypeOf(node) == searchedType {
			if handle == nil || handle(node.(T), isUnique, _ancestorChain) {
				found = node.(T)
				isUnique = false
				_ancestorChain = ancestorChain
			}
		}
		return ContinueTraversal, nil
	}, nil)

	return found, _ancestorChain
}

// FindClosest searches for an ancestor node of type typ starting from the parent node (last ancestor).
func FindClosest[T Node](ancestorChain []Node, typ T) (node T, index int, ok bool) {
	return FindClosestMaxDistance[T](ancestorChain, typ, -1)
}

// FindClosestMaxDistance searches for an ancestor node of type typ starting from the parent node (last ancestor),
// maxDistance is the maximum distance from the parent node. A negative or zero maxDistance is ignored.
func FindClosestMaxDistance[T Node](ancestorChain []Node, typ T, maxDistance int) (node T, index int, ok bool) {
	searchedType := reflect.TypeOf(typ)

	lastI := 0
	if maxDistance > 0 {
		lastI = max(0, len(ancestorChain)-maxDistance-1)
	}

	for i := len(ancestorChain) - 1; i >= lastI; i-- {
		n := ancestorChain[i]
		if reflect.TypeOf(n) == searchedType {
			return n.(T), i, true
		}
	}

	return reflect.Zero(searchedType).Interface().(T), -1, false
}

// FindClosestTopLevelStatement returns the deepest top level statement among node and its ancestors.
func FindClosestTopLevelStatement(node Node, ancestorChain []Node) (Node, bool) {
	if len(ancestorChain) == 0 {
		return nil, false
	}
	parent := ancestorChain[len(ancestorChain)-1]

	if IsTheTopLevel(parent) {
		return node, true
	}

	for i := len(ancestorChain) - 1; i >= 1; i-- {
		if IsTheTopLevel(ancestorChain[i-1]) {
			return ancestorChain[i], true
		}
	}
	return nil, false
}

func FindPreviousStatement(n Node, ancestorChain []Node) (stmt Node, ok bool) {
	stmt, _, ok = FindPreviousStatementAndChain(n, ancestorChain, true)
	return
}

func FindPreviousStatementAndChain(n Node, ancestorChain []Node, climbBlocks bool) (stmt Node, chain []Node, ok bool) {
	if len(ancestorChain) == 0 || IsScopeContainerNode(n) {
		return nil, nil, false
	}

	p := ancestorChain[len(ancestorChain)-1]
	switch parent := p.(type) {
	case *Block:
		for i, stmt := range parent.Statements {
			if stmt == n {
				if i == 0 {
					if !climbBlocks {
						return nil, nil, false
					}
					return FindPreviousStatementAndChain(parent, ancestorChain[:len(ancestorChain)-1], climbBlocks)
				}
				return parent.Statements[i-1], ancestorChain, true
			}
		}
		if !climbBlocks {
			return nil, nil, false
		}
	case *Chunk:
		for i, stmt := range parent.Statements {
			if stmt == n {
				if i == 0 {
					return nil, nil, false
				}
				return parent.Statements[i-1], ancestorChain, true
			}
		}
	case *EmbeddedModule:
		for i, stmt := range parent.Statements {
			if stmt == n {
				if i == 0 {
					return nil, nil, false
				}
				return parent.Statements[i-1], ancestorChain, true
			}
		}
	}
	return FindPreviousStatementAndChain(p, ancestorChain[:len(ancestorChain)-1], climbBlocks)
}

func FindIdentWithName(root Node, name string) Node {
	return FindNode(root, (*IdentifierLiteral)(nil), func(n *IdentifierLiteral, isFirstFound bool, ancestors []Node) bool {
		return n.Name == name
	})
}

func FindLocalVarWithName(root Node, name string) Node {
	return FindNode(root, (*Variable)(nil), func(n *Variable, isFirstFound bool, ancestors []Node) bool {
		return n.Name == name
	})
}

func FindPatternIdentWithName(root Node, name string) Node {
	return FindNode(root, (*PatternIdentifierLiteral)(nil), func(n *PatternIdentifierLiteral, isFirstFound bool, ancestors []Node) bool {
		return n.Name == name
	})
}

func FindIntLiteralWithValue(root Node, value int64) *IntLiteral {
	return FindNode(root, (*IntLiteral)(nil), func(n *IntLiteral, isFirstFound bool, ancestors []Node) bool {
		return n.Value == value
	})
}

func FindObjPropWithName(root Node, name string) *ObjectProperty {
	return FindNode(root, (*ObjectProperty)(nil), func(n *ObjectProperty, isFirstFound bool, ancestors []Node) bool {
		return !n.HasNoKey() && n.Name() == name
	})
}

func FindLocalVarDeclWithName(root Node, name string) *LocalVariableDeclarator {
	return FindNode(root, (*LocalVariableDeclarator)(nil), func(n *LocalVariableDeclarator, isFirstFound bool, ancestors []Node) bool {
		ident, ok := n.Left.(*IdentifierLiteral)
		return ok && ident.Name == name
	})
}

func IsIdentLiteralWithName(n Node, name string) bool {
	ident, ok := n.(*IdentifierLiteral)
	return ok && ident.Name == name
}

func IsIdentMemberExprWithNames(n Node, name string, propNames ...string) bool {
	memberExpr, ok := n.(*IdentifierMemberExpression)
	if !ok || len(propNames) != len(memberExpr.PropertyNames) {
		return false
	}
	for i, propName := range propNames {
		if memberExpr.PropertyNames[i] == nil || memberExpr.PropertyNames[i].Name != propName {
			return false
		}
	}
	return true
}

func HasErrorAtAnyDepth(n Node) bool {
	err := false
	Walk(n, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if node.Base().Err != nil {
			err = true
			return StopTraversal, nil
		}
		return ContinueTraversal, nil
	}, nil)

	return err
}

func GetTreeView(n Node, chunk *Chunk) string {
	var buf = bytes.NewBuffer(nil)

	Walk(n, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		depth := len(ancestorChain)

		buf.Write(bytes.Repeat([]byte{' ', ' '}, depth))
		buf.WriteString(reflect.TypeOf(node).Elem().Name())

		if !NodeIsSimpleValueLiteral(node) {
			buf.WriteString("{ ")
			for _, tok := range GetTokens(node, chunk, false) {

				switch tok.Type {
				case UNEXPECTED_CHAR:
					buf.WriteString("(unexpected)`")
					if tok.Raw == "\n" {
						buf.WriteString("\\n")
					} else {
						buf.WriteString(tok.Str())
					}
				case NEWLINE:
					buf.WriteString(" `")
					buf.WriteString("\\n")
				default:
					buf.WriteString(" `")
					buf.WriteString(tok.Str())
				}
				buf.WriteString("` ")
			}
			buf.WriteByte('\n')
		} else {
			buf.WriteByte('\n')
		}

		return ContinueTraversal, nil
	}, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if !NodeIsSimpleValueLiteral(node) {
			depth := len(ancestorChain)
			buf.Write(bytes.Repeat([]byte{' ', ' '}, depth))
			buf.WriteString("}\n")
		}
		return ContinueTraversal, nil
	})

	return buf.String()
}

func GetInteriorSpan(node Node, chunk *Chunk) (interiorSpan NodeSpan, err error) {
	switch node.(type) {
	case *ObjectLiteral:
		return getInteriorSpan(node, chunk, OPENING_CURLY_BRACKET, CLOSING_CURLY_BRACKET)
	case *RecordLiteral:
		return getInteriorSpan(node, chunk, OPENING_RECORD_BRACKET, CLOSING_CURLY_BRACKET)
	case *DictionaryLiteral:
		return getInteriorSpan(node, chunk, OPENING_DICTIONARY_BRACKET, CLOSING_CURLY_BRACKET)
	}
	err = errors.New("not supported yet")
	return
}

// GetInteriorSpan returns the span of the "interior" of nodes such as blocks, objects or lists.
// the fist token matching the opening token is taken as the starting token (the span starts just after the token),
// the last token matching the closingToken is as taken as the ending token (the span ends just before this token).
func getInteriorSpan(node Node, chunk *Chunk, openingToken, closingToken TokenType) (interiorSpan NodeSpan, err error) {
	tokens := GetTokens(node, chunk, false)
	if len(tokens) == 0 {
		err = ErrMissingTokens
		return
	}

	interiorSpan = NodeSpan{Start: -1, End: -1}

	for _, token := range tokens {
		switch {
		case token.Type == openingToken && interiorSpan.Start < 0:
			interiorSpan.Start = token.Span.Start + 1
		case token.Type == closingToken:
			interiorSpan.End = token.Span.Start
		}
	}

	if interiorSpan.Start == -1 || interiorSpan.End == -1 {
		interiorSpan = NodeSpan{Start: -1, End: -1}
		err = ErrMissingTokens
		return
	}

	return
}

// DetermineActiveParameterIndex determines the index of the function parameter,
// it returns -1 if the index cannot be determined or if $ancestors does not contain
// a *Chunk. $callExprIndex should be -1 if $nodeAtSpan is $callExpr.
func DetermineActiveParameterIndex(
	cursorSpan NodeSpan,
	nodeAtSpan Node,
	callExpr *CallExpression,
	callExprIndex int,
	ancestors []Node,
) int {
	var argNode Node

	//Find the chunk in ancestors.

	var chunk *Chunk

	for _, ancestor := range ancestors {
		if c, ok := ancestor.(*Chunk); ok {
			chunk = c
			break
		}
	}

	if chunk == nil {
		return -1
	}

	if callExpr == ancestors[len(ancestors)-1] {
		if nodeAtSpan != callExpr {
			argNode = nodeAtSpan
		}
	} else if callExprIndex >= 0 {
		argNode = ancestors[callExprIndex+1]
	}

	if argNode != nil {
		for i, n := range callExpr.Arguments {
			if n == argNode {
				return i
			}
		}
		return -1
	} else if len(callExpr.Arguments) > 0 { //find the argument on the left of the cursor
		activeParamIndex := -1
		for i, currentArgNode := range callExpr.Arguments {
			currentArgEnd := currentArgNode.Base().Span.End

			if cursorSpan.Start >= currentArgEnd {
				activeParamIndex = i

				// increment argNodeIndex if the cursor is after a comma located after the current argument.
				for _, token := range GetTokens(callExpr, chunk, false) {
					if token.Type == COMMA && token.Span.Start >= currentArgEnd && cursorSpan.Start >= token.Span.End {
						activeParamIndex++
						break
					}
				}
			}
		}
		return activeParamIndex
	} else {
		return 0
	}
}
