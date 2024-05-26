package ast

import (
	"fmt"
	"reflect"
	"runtime/debug"
)

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
		walk(n.BadValueElemIdent, node, ancestorChain, fn, afterFn)
		walk(n.IteratedValue, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *ForExpression:
		walk(n.KeyPattern, node, ancestorChain, fn, afterFn)
		walk(n.KeyIndexIdent, node, ancestorChain, fn, afterFn)
		walk(n.ValuePattern, node, ancestorChain, fn, afterFn)
		walk(n.ValueElemIdent, node, ancestorChain, fn, afterFn)
		walk(n.BadValueElemIdent, node, ancestorChain, fn, afterFn)
		walk(n.IteratedValue, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *WalkStatement:
		walk(n.Walked, node, ancestorChain, fn, afterFn)
		walk(n.MetaIdent, node, ancestorChain, fn, afterFn)
		walk(n.EntryIdent, node, ancestorChain, fn, afterFn)
		walk(n.BadEntryIdent, node, ancestorChain, fn, afterFn)
		walk(n.Body, node, ancestorChain, fn, afterFn)
	case *WalkExpression:
		walk(n.Walked, node, ancestorChain, fn, afterFn)
		walk(n.MetaIdent, node, ancestorChain, fn, afterFn)
		walk(n.EntryIdent, node, ancestorChain, fn, afterFn)
		walk(n.BadEntryIdent, node, ancestorChain, fn, afterFn)
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
