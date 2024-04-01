package parse

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrMissingTokens = errors.New("missing tokens")
)

// A Node represents an immutable AST Node, all node types embed NodeBase that implements the Node interface
type Node interface {
	Base() NodeBase
	BasePtr() *NodeBase
	Kind() NodeKind
}

type NodeSpan struct {
	Start int32 `json:"start"` //0-indexed
	End   int32 `json:"end"`   //exclusive end, 0-indexed
}

func (s NodeSpan) HasPositionEndIncluded(i int32) bool {
	return i >= s.Start && i <= s.End
}

func (s NodeSpan) Len() int32 {
	return s.End - s.Start
}

// NodeBase implements Node interface
type NodeBase struct {
	Span            NodeSpan      `json:"span"`
	Err             *ParsingError `json:"error,omitempty"`
	IsParenthesized bool          //can be true even if the closing parenthesis is missing
}

func (base NodeBase) Base() NodeBase {
	return base
}

func (base *NodeBase) BasePtr() *NodeBase {
	return base
}

func (base *NodeBase) Kind() NodeKind {
	return UnspecifiedNodeKind
}

func (base NodeBase) IncludedIn(node Node) bool {
	return base.Span.Start >= node.Base().Span.Start && base.Span.End <= node.Base().Span.End
}

type NodeKind uint8

const (
	UnspecifiedNodeKind NodeKind = iota
	Expr
	Stmt
)

type SimpleValueLiteral interface {
	Node
	ValueString() string
}

type IIdentifierLiteral interface {
	Identifier() string
}

type GroupPatternLiteral interface {
	Node
	GroupNames() []string
}

var _ = []SimpleValueLiteral{
	(*DoubleQuotedStringLiteral)(nil), (*UnquotedStringLiteral)(nil), (*MultilineStringLiteral)(nil), (*IdentifierLiteral)(nil),
	(*UnambiguousIdentifierLiteral)(nil), (*IntLiteral)(nil), (*FloatLiteral)(nil),
	(*AbsolutePathLiteral)(nil), (*RelativePathLiteral)(nil), (*AbsolutePathPatternLiteral)(nil), (*RelativePathPatternLiteral)(nil),
	(*NamedSegmentPathPatternLiteral)(nil), (*RegularExpressionLiteral)(nil), (*BooleanLiteral)(nil), (*NilLiteral)(nil),
	(*HostLiteral)(nil), (*HostPatternLiteral)(nil), (*URLLiteral)(nil), (*URLPatternLiteral)(nil), (*PortLiteral)(nil),
	(*FlagLiteral)(nil),
}

var _ = []IIdentifierLiteral{UnambiguousIdentifierLiteral{}, IdentifierLiteral{}}

type InvalidURLPattern struct {
	NodeBase `json:"base:invalid-url-pattern"`
	Value    string
}

type InvalidURL struct {
	NodeBase `json:"base:invalid-url"`
	Value    string `json:"value"`
}

type InvalidAliasRelatedNode struct {
	NodeBase `json:"base:invalid-alias-related-pattern"`
	Raw      string `json:"raw"`
}

type InvalidPathPattern struct {
	NodeBase `json:"base:invalid-path-pattern"`
	Value    string `json:"value"`
}

type InvalidComplexStringPatternElement struct {
	NodeBase `json:"base:invalid-complex-string-pattern-elem"`
}

type InvalidObjectElement struct {
	NodeBase `json:"base:invalid-object-elem"`
}

type InvalidMemberLike struct {
	NodeBase `json:"base:invalid-member-like"`
	Left     Node `json:"left,omitempty"`
	Right    Node `json:"right,omitempty"` //can be nil
}

type MissingExpression struct {
	NodeBase `json:"base:missing-expr"`
}

type InvalidCSSselectorNode struct {
	NodeBase `json:"base:invalid-css-selector-node"`
}

type UnknownNode struct {
	NodeBase `json:"base:unknown-node"`
}

type Comment struct {
	NodeBase `json:"base:comment"`
	Raw      string
}

func IsScopeContainerNode(node Node) bool {
	switch node.(type) {
	case *Chunk, *EmbeddedModule, *FunctionExpression, *FunctionPatternExpression, *QuotedExpression,
		*InitializationBlock, *MappingExpression, *StaticMappingEntry, *DynamicMappingEntry, *TestSuiteExpression, *TestCaseExpression,
		*ExtendStatement,       //ExtendStatement being a scope container is not 100% incorrect
		*StructDefinition,      //same
		*LifetimejobExpression: // <-- remove ?
		return true
	default:
		return false
	}
}

func IsTheTopLevel(node Node) bool {
	switch node.(type) {
	case *Chunk, *EmbeddedModule:
		return true
	}
	return false
}

// Chunk represents the root node obtained when parsing an Inox chunk.
type Chunk struct {
	NodeBase                   `json:"base:chunk"`
	GlobalConstantDeclarations *GlobalConstantDeclarations `json:"globalConstDecls,omitempty"`    //nil if no const declarations at the top of the module
	Preinit                    *PreinitStatement           `json:"preinit,omitempty"`             //nil if no preinit block at the top of the module
	Manifest                   *Manifest                   `json:"manifest,omitempty"`            //nil if no manifest at the top of the module
	IncludableChunkDesc        *IncludableChunkDescription `json:"includableChunkDesc,omitempty"` //nil if no manifest at the top of the module
	Statements                 []Node                      `json:"statements,omitempty"`
	IsShellChunk               bool

	//mostly valueless tokens, sorted by position (ascending).
	//EmbeddedModule nodes hold references to subslices of .Tokens.
	Tokens []Token `json:"tokens,omitempty"`
}

type EmbeddedModule struct {
	NodeBase       `json:"base:embedded-module"`
	Manifest       *Manifest `json:"manifest,omitempty"` //can be nil
	Statements     []Node    `json:"statements,omitempty"`
	SingleCallExpr bool      `json:"isSingleCallExpr"`
	Tokens         []Token/*slice of the parent chunk .Tokens*/ `json:"tokens,omitempty"`
}

func (emod *EmbeddedModule) ToChunk() *Chunk {
	return &Chunk{
		NodeBase:   emod.NodeBase,
		Manifest:   emod.Manifest,
		Statements: emod.Statements,
		Tokens:     emod.Tokens,
	}
}

type Variable struct {
	NodeBase `json:"base:variable"`
	Name     string
}

func (v Variable) Str() string {
	return "$" + v.Name
}

func (Variable) Kind() NodeKind {
	return Expr
}

type MemberExpression struct {
	NodeBase     `json:"base:member-expr"`
	Left         Node
	PropertyName *IdentifierLiteral
	Optional     bool
}

func (MemberExpression) Kind() NodeKind {
	return Expr
}

type ComputedMemberExpression struct {
	NodeBase     `json:"base:invalid"`
	Left         Node
	PropertyName Node
	Optional     bool
}

func (ComputedMemberExpression) Kind() NodeKind {
	return Expr
}

type IdentifierMemberExpression struct {
	NodeBase      `json:"base:invalid-member-expr"`
	Left          *IdentifierLiteral
	PropertyNames []*IdentifierLiteral
}

func (IdentifierMemberExpression) Kind() NodeKind {
	return Expr
}

type IndexExpression struct {
	NodeBase `json:"base:index-expr"`
	Indexed  Node
	Index    Node
}

func (IndexExpression) Kind() NodeKind {
	return Expr
}

type SliceExpression struct {
	NodeBase   `json:"base:slice-expr"`
	Indexed    Node `json:"indexed"`
	StartIndex Node `json:"startIndex,omitempty"` //can be nil
	EndIndex   Node `json:"endIndex,omitempty"`   //can be nil
}

func (SliceExpression) Kind() NodeKind {
	return Expr
}

type DoubleColonExpression struct {
	NodeBase `json:"base:double-colon-expr"`
	Left     Node               `json:"left"`
	Element  *IdentifierLiteral `json:"element"`
}

func (DoubleColonExpression) Kind() NodeKind {
	return Expr
}

type KeyListExpression struct {
	NodeBase `json:"base:key-list-expr"`
	Keys     []Node `json:"keys"` //slice of *IdentifierLiteral if ok
}

func (expr KeyListExpression) Names() []*IdentifierLiteral {
	names := make([]*IdentifierLiteral, len(expr.Keys))

	for i, e := range expr.Keys {
		if ident, ok := e.(*IdentifierLiteral); ok {
			names[i] = ident
		} else {
			panic(errors.New("one of the element of key list is not an identifiers"))
		}
	}

	return names
}

func (KeyListExpression) Kind() NodeKind {
	return Expr
}

type BooleanConversionExpression struct {
	NodeBase `json:"base:bool-conv-expr"`
	Expr     Node
}

func (BooleanConversionExpression) Kind() NodeKind {
	return Expr
}

type BooleanLiteral struct {
	NodeBase `json:"base:bool-lit"`
	Value    bool
}

func (BooleanLiteral) Kind() NodeKind {
	return Expr
}

func (l BooleanLiteral) ValueString() string {
	if l.Value {
		return "true"
	}
	return "false"
}

type FlagLiteral struct {
	NodeBase   `json:"base:flag-lit"`
	SingleDash bool
	Name       string
	Raw        string
}

func (FlagLiteral) Kind() NodeKind {
	return Expr
}

func (l FlagLiteral) ValueString() string {
	return l.Raw
}

type OptionExpression struct {
	NodeBase   `json:"base:option-expr"`
	SingleDash bool
	Name       string
	Value      Node
}

func (OptionExpression) Kind() NodeKind {
	return Expr
}

type IntLiteral struct {
	NodeBase `json:"base:int-lit"`
	Raw      string
	Value    int64
}

func (l IntLiteral) IsHex() bool {
	return strings.HasPrefix(l.Raw, "0x")
}

func (l IntLiteral) IsOctal() bool {
	return strings.HasPrefix(l.Raw, "0o")
}

func (l IntLiteral) ValueString() string {
	return l.Raw
}

func (IntLiteral) Kind() NodeKind {
	return Expr
}

type FloatLiteral struct {
	NodeBase `json:"base:float-lit"`
	Raw      string
	Value    float64
}

func (l FloatLiteral) ValueString() string {
	return l.Raw
}

func (FloatLiteral) Kind() NodeKind {
	return Expr
}

type PortLiteral struct {
	NodeBase   `json:"base:port-lit"`
	Raw        string
	PortNumber uint16
	SchemeName string
}

func (l PortLiteral) ValueString() string {
	return l.Raw
}

func (PortLiteral) Kind() NodeKind {
	return Expr
}

type QuantityLiteral struct {
	NodeBase `json:"base:quantity-lit"`
	Raw      string
	Values   []float64
	Units    []string
}

func (l QuantityLiteral) ValueString() string {
	return l.Raw
}

func (QuantityLiteral) Kind() NodeKind {
	return Expr
}

type YearLiteral struct {
	NodeBase `json:"base:year-lit"`
	Raw      string
	Value    time.Time
}

func (l YearLiteral) ValueString() string {
	return l.Raw
}

func (YearLiteral) Kind() NodeKind {
	return Expr
}

type DateLiteral struct {
	NodeBase `json:"base:date-lit"`
	Raw      string
	Value    time.Time
}

func (l DateLiteral) ValueString() string {
	return l.Raw
}

func (DateLiteral) Kind() NodeKind {
	return Expr
}

type DateTimeLiteral struct {
	NodeBase `json:"base:datetime-lit"`
	Raw      string
	Value    time.Time
}

func (l DateTimeLiteral) ValueString() string {
	return l.Raw
}

func (DateTimeLiteral) Kind() NodeKind {
	return Expr
}

type RateLiteral struct {
	NodeBase `json:"base:rate-lit"`
	Values   []float64
	Units    []string
	DivUnit  string
	Raw      string
}

func (l RateLiteral) ValueString() string {
	return l.Raw
}

func (RateLiteral) Kind() NodeKind {
	return Expr
}

type RuneLiteral struct {
	NodeBase `json:"base:rune-lit"`
	Value    rune
}

func (l RuneLiteral) ValueString() string {
	return string(l.Value)
}

func (RuneLiteral) Kind() NodeKind {
	return Expr
}

type DoubleQuotedStringLiteral struct {
	NodeBase `json:"base:double-quoted-str-lit"`
	Raw      string
	Value    string
}

func (l DoubleQuotedStringLiteral) ValueString() string {
	return l.Value
}

func (DoubleQuotedStringLiteral) Kind() NodeKind {
	return Expr
}

type UnquotedStringLiteral struct {
	NodeBase `json:"base:unquoted-str-lit"`
	Raw      string
	Value    string
}

func (l UnquotedStringLiteral) ValueString() string {
	return l.Value
}

func (UnquotedStringLiteral) Kind() NodeKind {
	return Expr
}

type MultilineStringLiteral struct {
	NodeBase
	Raw            string
	Value          string
	IsUnterminated bool
}

func (l MultilineStringLiteral) ValueString() string {
	return l.Value
}

func (MultilineStringLiteral) Kind() NodeKind {
	return Expr
}

func (l MultilineStringLiteral) RawWithoutQuotes() string {
	raw := l.Raw[1:]

	isTerminated := !l.IsUnterminated
	if isTerminated {
		raw = raw[:len(raw)-1]
	}

	return raw
}

type StringTemplateLiteral struct {
	NodeBase
	Pattern Node   //*PatternIdentifierLiteral | *PatternNamespaceMemberExpression | nil
	Slices  []Node //StringTemplateSlice | StringTemplateInterpolation
}

func (lit *StringTemplateLiteral) HasInterpolations() bool {
	for _, slice := range lit.Slices {
		if _, ok := slice.(*StringTemplateInterpolation); ok {
			return true
		}
	}
	return false
}

func (StringTemplateLiteral) Kind() NodeKind {
	return Expr
}

type StringTemplateSlice struct {
	NodeBase
	Raw   string
	Value string
}

type StringTemplateInterpolation struct {
	NodeBase
	Type string // empty if not typed, examples of value: 'str', 'str.from' (without the quotes)
	Expr Node
}

type ByteSliceLiteral struct {
	NodeBase
	Raw   string
	Value []byte
}

func (l ByteSliceLiteral) ValueString() string {
	return string(l.Value)
}

func (ByteSliceLiteral) Kind() NodeKind {
	return Expr
}

type URLLiteral struct {
	NodeBase
	Value string
}

func (l URLLiteral) Scheme() (string, error) {
	u, err := url.Parse(l.Value)
	if err != nil {
		return "", err
	}

	return u.Scheme, nil
}

func (l URLLiteral) ValueString() string {
	return l.Value
}

func (URLLiteral) Kind() NodeKind {
	return Expr
}

type SchemeLiteral struct {
	NodeBase
	Name string
}

func (s SchemeLiteral) ValueString() string {
	return s.Name + "://"
}

func (SchemeLiteral) Kind() NodeKind {
	return Expr
}

type HostLiteral struct {
	NodeBase
	Value string
}

func (l HostLiteral) ValueString() string {
	return l.Value
}

func (HostLiteral) Kind() NodeKind {
	return Expr
}

type HostPatternLiteral struct {
	NodeBase
	Value string
	Raw   string
}

func (l HostPatternLiteral) ValueString() string {
	return l.Value
}

func (HostPatternLiteral) Kind() NodeKind {
	return Expr
}

type URLPatternLiteral struct {
	NodeBase
	Value string
	Raw   string
}

func (l URLPatternLiteral) ValueString() string {
	return l.Value
}

func (URLPatternLiteral) Kind() NodeKind {
	return Expr
}

type AbsolutePathLiteral struct {
	NodeBase
	Raw   string
	Value string
}

func (l AbsolutePathLiteral) ValueString() string {
	return l.Value
}

func (AbsolutePathLiteral) Kind() NodeKind {
	return Expr
}

type RelativePathLiteral struct {
	NodeBase
	Raw   string
	Value string
}

func (l RelativePathLiteral) ValueString() string {
	return l.Value
}

func (RelativePathLiteral) Kind() NodeKind {
	return Expr
}

type AbsolutePathPatternLiteral struct {
	NodeBase
	Raw        string
	Value      string //unprefixed path pattern (e.g. /* for a `%/*` literal)
	Unprefixed bool
}

func (l AbsolutePathPatternLiteral) ValueString() string {
	return l.Value
}

func (AbsolutePathPatternLiteral) Kind() NodeKind {
	return Expr
}

type RelativePathPatternLiteral struct {
	NodeBase
	Raw        string
	Value      string
	Unprefixed bool
}

func (l RelativePathPatternLiteral) ValueString() string {
	return l.Value
}

func (RelativePathPatternLiteral) Kind() NodeKind {
	return Expr
}

// TODO: rename
type NamedSegmentPathPatternLiteral struct {
	NodeBase
	Slices      []Node //PathPatternSlice | NamedPathSegment
	Raw         string
	StringValue string
}

func (l NamedSegmentPathPatternLiteral) ValueString() string {
	return l.StringValue
}

func (l NamedSegmentPathPatternLiteral) GroupNames() []string {
	var names []string
	for _, e := range l.Slices {
		if named, ok := e.(*NamedPathSegment); ok {
			names = append(names, named.Name)
		}
	}
	return names
}

func (NamedSegmentPathPatternLiteral) Kind() NodeKind {
	return Expr
}

type PathPatternExpression struct {
	NodeBase
	Slices []Node //PathPatternSlice | Variable
}

func (PathPatternExpression) Kind() NodeKind {
	return Expr
}

type RelativePathExpression struct {
	NodeBase
	Slices []Node
}

func (RelativePathExpression) Kind() NodeKind {
	return Expr
}

type AbsolutePathExpression struct {
	NodeBase
	Slices []Node
}

func (AbsolutePathExpression) Kind() NodeKind {
	return Expr
}

type RegularExpressionLiteral struct {
	NodeBase
	Value      string
	Raw        string
	Unprefixed bool
}

func (l RegularExpressionLiteral) ValueString() string {
	return l.Value
}

func (RegularExpressionLiteral) Kind() NodeKind {
	return Expr
}

type URLExpression struct {
	NodeBase
	Raw         string
	HostPart    Node
	Path        []Node
	QueryParams []Node
}

func (URLExpression) Kind() NodeKind {
	return Expr
}

type HostExpression struct {
	NodeBase
	Scheme *SchemeLiteral
	Host   Node
	Raw    string
}

func (HostExpression) Kind() NodeKind {
	return Expr
}

type URLQueryParameter struct {
	NodeBase
	Name  string
	Value []Node
}

func (URLQueryParameter) Kind() NodeKind {
	return Expr
}

type URLQueryParameterValueSlice struct {
	NodeBase
	Value string
}

func (URLQueryParameterValueSlice) Kind() NodeKind {
	return Expr
}

func (s URLQueryParameterValueSlice) ValueString() string {
	return s.Value
}

type PathSlice struct {
	NodeBase
	Value string
}

func (s PathSlice) ValueString() string {
	return s.Value
}

func (PathSlice) Kind() NodeKind {
	return Expr
}

type PathPatternSlice struct {
	NodeBase
	Value string
}

func (s PathPatternSlice) ValueString() string {
	return s.Value
}

func (PathPatternSlice) Kind() NodeKind {
	return Expr
}

type NamedPathSegment struct {
	NodeBase
	Name string
}

type NilLiteral struct {
	NodeBase
}

func (l NilLiteral) ValueString() string {
	return "nil"
}

func (NilLiteral) Kind() NodeKind {
	return Expr
}

type ObjectLiteral struct {
	NodeBase
	Properties     []*ObjectProperty
	MetaProperties []*ObjectMetaProperty
	SpreadElements []*PropertySpreadElement
}

func (objLit ObjectLiteral) PropValue(name string) (Node, bool) {
	for _, prop := range objLit.Properties {
		if prop.Key != nil && prop.Name() == name {
			return prop.Value, true
		}
	}

	return nil, false
}

func (objLit ObjectLiteral) HasNamedProp(name string) bool {
	for _, prop := range objLit.Properties {
		if prop.Key != nil && prop.Name() == name {
			return true
		}
	}

	return false
}

func (ObjectLiteral) Kind() NodeKind {
	return Expr
}

type ExtractionExpression struct {
	NodeBase
	Object Node
	Keys   *KeyListExpression
}

func (ExtractionExpression) Kind() NodeKind {
	return Expr
}

type PatternPropertySpreadElement struct {
	NodeBase
	Expr Node
}

type PropertySpreadElement struct {
	NodeBase
	Expr Node //should be an *ExtractionExpression if parsing is ok
}

type ObjectProperty struct {
	NodeBase
	Key   Node //can be nil
	Type  Node //can be nil
	Value Node
}

func (prop ObjectProperty) HasNoKey() bool {
	return prop.Key == nil
}

func (prop ObjectProperty) Name() string {
	switch v := prop.Key.(type) {
	case *IdentifierLiteral:
		return v.Name
	case *DoubleQuotedStringLiteral:
		return v.Value
	default:
		panic(fmt.Errorf("invalid key type %T", v))
	}
}

func (prop ObjectProperty) HasNameEqualTo(name string) bool {
	switch v := prop.Key.(type) {
	case *IdentifierLiteral:
		return v.Name == name
	case *DoubleQuotedStringLiteral:
		return v.Value == name
	default:
		return false
	}
}

type ObjectPatternProperty struct {
	NodeBase
	Key      Node //can be nil (error)
	Type     Node //can be nil
	Value    Node
	Optional bool
}

func (prop ObjectPatternProperty) Name() string {
	switch v := prop.Key.(type) {
	case *IdentifierLiteral:
		return v.Name
	case *DoubleQuotedStringLiteral:
		return v.Value
	default:
		panic(fmt.Errorf("invalid key type %T", v))
	}
}

type ObjectMetaProperty struct {
	NodeBase
	Key            Node
	Initialization *InitializationBlock
}

func (prop ObjectMetaProperty) Name() string {
	switch v := prop.Key.(type) {
	case *IdentifierLiteral:
		return v.Name
	case *DoubleQuotedStringLiteral:
		return v.Value
	default:
		panic(fmt.Errorf("invalid key type %T", v))
	}
}

type InitializationBlock struct {
	NodeBase
	Statements []Node
}

type RecordLiteral struct {
	NodeBase
	Properties     []*ObjectProperty
	SpreadElements []*PropertySpreadElement
}

func (RecordLiteral) Kind() NodeKind {
	return Expr
}

type ListLiteral struct {
	NodeBase
	TypeAnnotation Node //can be nil
	Elements       []Node
}

func (list *ListLiteral) HasSpreadElements() bool {
	for _, e := range list.Elements {
		if _, ok := e.(*ElementSpreadElement); ok {
			return true
		}
	}
	return false
}

func (ListLiteral) Kind() NodeKind {
	return Expr
}

type TupleLiteral struct {
	NodeBase
	TypeAnnotation Node //can be nil
	Elements       []Node
}

func (list *TupleLiteral) HasSpreadElements() bool {
	for _, e := range list.Elements {
		if _, ok := e.(*ElementSpreadElement); ok {
			return true
		}
	}
	return false
}

func (*TupleLiteral) Kind() NodeKind {
	return Expr
}

type ElementSpreadElement struct {
	NodeBase
	Expr Node
}

type DictionaryLiteral struct {
	NodeBase
	Entries []*DictionaryEntry
}

func (DictionaryLiteral) Kind() NodeKind {
	return Expr
}

type DictionaryEntry struct {
	NodeBase
	Key   Node
	Value Node
}

type IdentifierLiteral struct {
	NodeBase
	Name string
}

func (l IdentifierLiteral) ValueString() string {
	return "#" + l.Name
}

func (l IdentifierLiteral) Identifier() string {
	return l.Name
}

func (IdentifierLiteral) Kind() NodeKind {
	return Expr
}

type UnambiguousIdentifierLiteral struct {
	NodeBase
	Name string
}

func (l UnambiguousIdentifierLiteral) ValueString() string {
	return "#" + l.Name
}

func (l UnambiguousIdentifierLiteral) Identifier() string {
	return l.Name
}

func (UnambiguousIdentifierLiteral) Kind() NodeKind {
	return Expr
}

type PropertyNameLiteral struct {
	NodeBase
	Name string
}

func (l PropertyNameLiteral) ValueString() string {
	return "." + l.Name
}

func (PropertyNameLiteral) Kind() NodeKind {
	return Expr
}

type LongValuePathLiteral struct {
	NodeBase
	Segments []SimpleValueLiteral
}

func (l LongValuePathLiteral) ValueString() string {
	buf := bytes.Buffer{}

	for _, segment := range l.Segments {
		buf.WriteString(segment.ValueString())
	}

	return buf.String()
}

func (LongValuePathLiteral) Kind() NodeKind {
	return Expr
}

//TODO: add ValueIndexLiteral

type SelfExpression struct {
	NodeBase
}

func (SelfExpression) Kind() NodeKind {
	return Expr
}

type SendValueExpression struct {
	NodeBase
	Value    Node
	Receiver Node
}

func (SendValueExpression) Kind() NodeKind {
	return Expr
}

type PatternIdentifierLiteral struct {
	NodeBase
	Unprefixed bool
	Name       string
}

func (PatternIdentifierLiteral) Kind() NodeKind {
	return Expr
}

type PatternNamespaceIdentifierLiteral struct {
	NodeBase
	Unprefixed bool
	Name       string
}

func (PatternNamespaceIdentifierLiteral) Kind() NodeKind {
	return Expr
}

type OptionalPatternExpression struct {
	NodeBase
	Pattern Node
}

func (OptionalPatternExpression) Kind() NodeKind {
	return Expr
}

type ObjectPatternLiteral struct {
	NodeBase
	Properties      []*ObjectPatternProperty
	OtherProperties []*OtherPropsExpr
	SpreadElements  []*PatternPropertySpreadElement
}

func (ObjectPatternLiteral) Kind() NodeKind {
	return Expr
}

func (l ObjectPatternLiteral) Exact() bool {
	for _, p := range l.OtherProperties {
		if p.No {
			return true
		}
	}

	return false
}

type OtherPropsExpr struct {
	NodeBase
	No      bool
	Pattern Node
}

type ListPatternLiteral struct {
	NodeBase
	Elements       []Node
	GeneralElement Node //GeneralElement and Elements cannot be non-nil at the same time
}

func (ListPatternLiteral) Kind() NodeKind {
	return Expr
}

type RecordPatternLiteral struct {
	NodeBase
	Properties      []*ObjectPatternProperty
	OtherProperties []*OtherPropsExpr
	SpreadElements  []*PatternPropertySpreadElement
}

func (RecordPatternLiteral) Kind() NodeKind {
	return Expr
}

func (l RecordPatternLiteral) Exact() bool {
	for _, p := range l.OtherProperties {
		if p.No {
			return true
		}
	}

	return false
}

type TuplePatternLiteral struct {
	NodeBase
	Elements       []Node
	GeneralElement Node //GeneralElement and Elements cannot be non-nil at the same time
}

func (TuplePatternLiteral) Kind() NodeKind {
	return Expr
}

type OptionPatternLiteral struct {
	NodeBase
	SingleDash bool
	Name       string
	Value      Node
	Unprefixed bool
}

func (OptionPatternLiteral) Kind() NodeKind {
	return Expr
}

type GlobalConstantDeclarations struct {
	NodeBase
	Declarations []*GlobalConstantDeclaration
}

func (GlobalConstantDeclarations) Kind() NodeKind {
	return Stmt
}

type GlobalConstantDeclaration struct {
	NodeBase
	Left  Node //*IdentifierLiteral
	Right Node
}

func (decl GlobalConstantDeclaration) Ident() *IdentifierLiteral {
	return decl.Left.(*IdentifierLiteral)
}

func (GlobalConstantDeclaration) Kind() NodeKind {
	return Stmt
}

type LocalVariableDeclarations struct {
	NodeBase
	Declarations []*LocalVariableDeclaration
}

func (LocalVariableDeclarations) Kind() NodeKind {
	return Stmt
}

type LocalVariableDeclaration struct {
	NodeBase
	Left  Node
	Type  Node //can be nil
	Right Node
}

func (LocalVariableDeclaration) Kind() NodeKind {
	return Stmt
}

type GlobalVariableDeclarations struct {
	NodeBase
	Declarations []*GlobalVariableDeclaration
}

func (GlobalVariableDeclarations) Kind() NodeKind {
	return Stmt
}

type GlobalVariableDeclaration struct {
	NodeBase
	Left  Node
	Type  Node //can be nil
	Right Node
}

func (GlobalVariableDeclaration) Kind() NodeKind {
	return Stmt
}

type Assignment struct {
	NodeBase
	Left     Node
	Right    Node
	Operator AssignmentOperator
}

func (Assignment) Kind() NodeKind {
	return Stmt
}

type MultiAssignment struct {
	NodeBase
	Variables []Node
	Right     Node
	Nillable  bool
}

func (MultiAssignment) Kind() NodeKind {
	return Stmt
}

type CallExpression struct {
	NodeBase
	Callee            Node
	Arguments         []Node //can include a SpreadArgument
	Must              bool
	CommandLikeSyntax bool
}

func (CallExpression) Kind() NodeKind {
	return Expr
}

func (e CallExpression) IsCalleeNamed(name string) bool {
	switch callee := e.Callee.(type) {
	case *IdentifierLiteral:
		return callee.Name == name
	case *Variable:
		return callee.Name == name
	default:
		return false
	}
}

type SpreadArgument struct {
	NodeBase
	Expr Node
}

type IfStatement struct {
	NodeBase
	Test       Node
	Consequent *Block
	Alternate  Node //can be nil, *Block | *IfStatement
}

func (IfStatement) Kind() NodeKind {
	return Stmt
}

type IfExpression struct {
	NodeBase
	Test       Node
	Consequent Node
	Alternate  Node //can be nil
}

func (IfExpression) Kind() NodeKind {
	return Stmt
}

type ForStatement struct {
	NodeBase

	KeyIndexIdent *IdentifierLiteral //can be nil
	KeyPattern    Node               //can be nil

	ValueElemIdent *IdentifierLiteral //can be nil
	ValuePattern   Node               //can be nil

	Body          *Block
	Chunked       bool
	IteratedValue Node
}

func (ForStatement) Kind() NodeKind {
	return Stmt
}

type ForExpression struct {
	NodeBase

	KeyIndexIdent *IdentifierLiteral //can be nil
	KeyPattern    Node               //can be nil

	ValueElemIdent *IdentifierLiteral //can be nil
	ValuePattern   Node               //can be nil

	Body          Node //*Block or expression
	Chunked       bool
	IteratedValue Node
}

func (ForExpression) Kind() NodeKind {
	return Expr
}

type WalkStatement struct {
	NodeBase
	Walked     Node
	MetaIdent  *IdentifierLiteral
	EntryIdent *IdentifierLiteral
	Body       *Block
}

func (WalkStatement) Kind() NodeKind {
	return Stmt
}

type PruneStatement struct {
	NodeBase
}

func (PruneStatement) Kind() NodeKind {
	return Stmt
}

type Block struct {
	NodeBase
	Statements []Node
}

type SynchronizedBlockStatement struct {
	NodeBase
	SynchronizedValues []Node
	Block              *Block
}

func (SynchronizedBlockStatement) Kind() NodeKind {
	return Stmt
}

type ReturnStatement struct {
	NodeBase
	Expr Node //can be nil
}

func (ReturnStatement) Kind() NodeKind {
	return Stmt
}

type CoyieldStatement struct {
	NodeBase
	Expr Node //can be nil
}

func (CoyieldStatement) Kind() NodeKind {
	return Stmt
}

type YieldStatement struct {
	NodeBase
	Expr Node //can be nil
}

func (YieldStatement) Kind() NodeKind {
	return Stmt
}

type BreakStatement struct {
	NodeBase
	Label *IdentifierLiteral //can be nil
}

func (BreakStatement) Kind() NodeKind {
	return Stmt
}

type ContinueStatement struct {
	NodeBase
	Label *IdentifierLiteral //can be nil
}

func (ContinueStatement) Kind() NodeKind {
	return Stmt
}

type SwitchStatement struct {
	NodeBase
	Discriminant Node
	Cases        []*SwitchStatementCase
	DefaultCases []*DefaultCaseWithBlock
}

func (SwitchStatement) Kind() NodeKind {
	return Stmt
}

type SwitchStatementCase struct {
	NodeBase
	Values []Node
	Block  *Block
}

type MatchStatement struct {
	NodeBase
	Discriminant Node
	Cases        []*MatchStatementCase
	DefaultCases []*DefaultCaseWithBlock
}

func (MatchStatement) Kind() NodeKind {
	return Stmt
}

type MatchStatementCase struct {
	NodeBase
	Values                []Node
	GroupMatchingVariable Node //can be nil
	Block                 *Block
}

type DefaultCaseWithBlock struct {
	NodeBase
	Block *Block
}

type SwitchExpression struct {
	NodeBase
	Discriminant Node
	Cases        []*SwitchExpressionCase
	DefaultCases []*DefaultCaseWithResult
}

func (SwitchExpression) Kind() NodeKind {
	return Expr
}

type SwitchExpressionCase struct {
	NodeBase
	Values []Node
	Result Node
}

type MatchExpression struct {
	NodeBase
	Discriminant Node
	Cases        []*MatchExpressionCase
	DefaultCases []*DefaultCaseWithResult
}

func (MatchExpression) Kind() NodeKind {
	return Expr
}

type MatchExpressionCase struct {
	NodeBase
	Values                []Node
	GroupMatchingVariable Node //can be nil
	Result                Node
}

type DefaultCaseWithResult struct {
	NodeBase
	Result Node
}

type UnaryOperator int

const (
	BoolNegate UnaryOperator = iota
	NumberNegate
)

type BinaryOperator int

const (
	Add BinaryOperator = iota
	AddDot
	Sub
	SubDot
	Mul
	MulDot
	Div
	DivDot
	LessThan
	LessThanDot
	LessOrEqual
	LessOrEqualDot
	GreaterThan
	GreaterThanDot
	GreaterOrEqual
	GreaterOrEqualDot
	Equal
	NotEqual
	Is
	IsNot
	In
	NotIn
	Keyof
	Urlof
	Dot //unused, present for symmetry
	Range
	ExclEndRange
	And
	Or
	Match
	NotMatch
	Substrof
	SetDifference
	NilCoalescing
	PairComma
)

var BINARY_OPERATOR_STRINGS = [...]string{
	Add:               "+",
	AddDot:            "+.",
	Sub:               "-",
	SubDot:            "-.",
	Mul:               "*",
	MulDot:            "*.",
	Div:               "/",
	DivDot:            "/.",
	LessThan:          "<",
	LessThanDot:       "<.",
	LessOrEqual:       "<=",
	LessOrEqualDot:    "<=.",
	GreaterThan:       ">",
	GreaterThanDot:    ">.",
	GreaterOrEqual:    ">=",
	GreaterOrEqualDot: ">=.",
	Equal:             "==",
	NotEqual:          "!=",
	Is:                "is",
	IsNot:             "is-not",
	In:                "in",
	NotIn:             "not-in",
	Keyof:             "keyof",
	Urlof:             "urlof",
	Dot:               ".",
	Range:             "..",
	ExclEndRange:      "..<",
	And:               "and",
	Or:                "or",
	Match:             "match",
	NotMatch:          "not-match",
	Substrof:          "substrof",
	SetDifference:     "\\",
	NilCoalescing:     "??",
	PairComma:         ",",
}

func (operator BinaryOperator) String() string {
	if operator < 0 || int(operator) >= len(BINARY_OPERATOR_STRINGS) {
		return "(unknown operator)"
	}
	return BINARY_OPERATOR_STRINGS[int(operator)]
}

type AssignmentOperator int

const (
	Assign AssignmentOperator = iota
	PlusAssign
	MinusAssign
	MulAssign
	DivAssign
)

func (operator AssignmentOperator) Int() bool {
	switch operator {
	case PlusAssign, MinusAssign, MulAssign, DivAssign:
		return true
	}
	return false
}

type UnaryExpression struct {
	NodeBase
	Operator UnaryOperator
	Operand  Node
}

func (UnaryExpression) Kind() NodeKind {
	return Expr
}

type BinaryExpression struct {
	NodeBase
	Operator BinaryOperator
	Left     Node
	Right    Node
}

func (BinaryExpression) Kind() NodeKind {
	return Expr
}

type IntegerRangeLiteral struct {
	NodeBase
	LowerBound *IntLiteral
	UpperBound Node //can be nil
}

func (IntegerRangeLiteral) Kind() NodeKind {
	return Expr
}

type FloatRangeLiteral struct {
	NodeBase
	LowerBound *FloatLiteral
	UpperBound Node //can be nil
}

func (FloatRangeLiteral) Kind() NodeKind {
	return Expr
}

type QuantityRangeLiteral struct {
	NodeBase
	LowerBound *QuantityLiteral
	UpperBound Node //can be nil
}

func (QuantityRangeLiteral) Kind() NodeKind {
	return Expr
}

type UpperBoundRangeExpression struct {
	NodeBase
	UpperBound Node
}

func (UpperBoundRangeExpression) Kind() NodeKind {
	return Expr
}

type RuneRangeExpression struct {
	NodeBase
	Lower *RuneLiteral
	Upper *RuneLiteral
}

func (RuneRangeExpression) Kind() NodeKind {
	return Expr
}

type FunctionExpression struct {
	NodeBase
	CaptureList      []Node
	Parameters       []*FunctionParameter
	ReturnType       Node //can be nil
	IsVariadic       bool
	Body             Node
	IsBodyExpression bool
}

func (expr FunctionExpression) NonVariadicParamCount() int {
	if expr.IsVariadic {
		return max(0, len(expr.Parameters)-1)
	}

	return len(expr.Parameters)
}

func (expr FunctionExpression) VariadicParameter() *FunctionParameter {
	if !expr.IsVariadic {
		panic("cannot get variadic parameter of non-variadic function expression")
	}

	return expr.Parameters[len(expr.Parameters)-1]
}

func (expr FunctionExpression) SignatureInformation() (
	nonVariadicParamCount int, parameters []*FunctionParameter, variadicParam *FunctionParameter, returnType Node,
	isBodyExpr bool) {

	nonVariadicParamCount = expr.NonVariadicParamCount()
	parameters = expr.Parameters
	if expr.IsVariadic {
		variadicParam = expr.VariadicParameter()
	}
	returnType = expr.ReturnType
	isBodyExpr = expr.IsBodyExpression

	return
}

func (FunctionExpression) Kind() NodeKind {
	return Expr
}

type FunctionDeclaration struct {
	NodeBase
	Function *FunctionExpression
	Name     Node //*IdentifierLiteral | *UnquotedRegion
}

func (FunctionDeclaration) Kind() NodeKind {
	return Stmt
}

type FunctionParameter struct {
	NodeBase
	Var        Node //can be nil for function patterns, *IdentifierLiteral or *UnquotedRegion
	Type       Node //can be nil
	IsVariadic bool
}

type ReadonlyPatternExpression struct {
	NodeBase
	Pattern Node
}

type FunctionPatternExpression struct {
	NodeBase
	Value      Node
	Parameters []*FunctionParameter
	ReturnType Node //optional if .Body is present
	IsVariadic bool
}

func (expr FunctionPatternExpression) NonVariadicParamCount() int {
	if expr.IsVariadic {
		return max(0, len(expr.Parameters)-1)
	}

	return len(expr.Parameters)
}

func (expr FunctionPatternExpression) VariadicParameter() *FunctionParameter {
	if !expr.IsVariadic {
		panic("cannot get variadic parameter of non-variadic function pattern expression")
	}

	return expr.Parameters[len(expr.Parameters)-1]
}

func (expr FunctionPatternExpression) SignatureInformation() (
	nonVariadicParamCount int, parameters []*FunctionParameter, variadicParam *FunctionParameter, returnType Node,
	isBodyExpr bool) {

	nonVariadicParamCount = expr.NonVariadicParamCount()
	parameters = expr.Parameters
	if expr.IsVariadic {
		variadicParam = expr.VariadicParameter()
	}

	returnType = expr.ReturnType

	return
}

func (FunctionPatternExpression) Kind() NodeKind {
	return Expr
}

type StructDefinition struct {
	NodeBase
	Name Node //*PatternIdentifierLiteral
	Body *StructBody
}

func (d *StructDefinition) GetName() (string, bool) {
	ident, ok := d.Name.(*PatternIdentifierLiteral)
	if ok {
		return ident.Name, true
	}
	return "", false
}

type StructBody struct {
	NodeBase
	Definitions []Node //*StructFieldDefinition and *FunctionDeclaration
}

type StructFieldDefinition struct {
	NodeBase
	Name *IdentifierLiteral
	Type Node
}

type NewExpression struct {
	NodeBase
	Type           Node //*PatternIdentifierLiteral for structs
	Initialization Node
}

type StructInitializationLiteral struct {
	NodeBase
	Fields []Node //*StructFieldInitialization
}

type StructFieldInitialization struct {
	NodeBase
	Name  *IdentifierLiteral
	Value Node
}

type PointerType struct {
	NodeBase
	ValueType Node
}

type DereferenceExpression struct {
	NodeBase
	Pointer Node
}

type PatternConversionExpression struct {
	NodeBase
	Value Node
}

func (PatternConversionExpression) Kind() NodeKind {
	return Expr
}

type PreinitStatement struct {
	NodeBase
	Block *Block
}

type Manifest struct {
	NodeBase `json:"base:manifest"`
	Object   Node `json:"object,omitempty"`
}

type IncludableChunkDescription struct {
	NodeBase `json:"includable-file-desc"`
}

type PermissionDroppingStatement struct {
	NodeBase `json:"base:permDroppingStmt"`
	Object   *ObjectLiteral `json:"object,omitempty"`
}

func (PermissionDroppingStatement) Kind() NodeKind {
	return Stmt
}

type ImportStatement struct {
	NodeBase      `json:"base:importStmt"`
	Identifier    *IdentifierLiteral `json:"identifier,omitempty"`
	Source        Node               `json:"source,omitempty"` // *URLLiteral, *RelativePathLiteral, *AbsolutePathLiteral
	Configuration Node               `json:"configuration,omitempty"`
}

func (stmt *ImportStatement) SourceString() (string, bool) {
	switch src := stmt.Source.(type) {
	case *URLLiteral:
		return src.Value, true
	case *AbsolutePathLiteral:
		return src.Value, true
	case *RelativePathLiteral:
		return src.Value, true
	default:
		return "", false
	}
}

func (ImportStatement) Kind() NodeKind {
	return Stmt
}

type InclusionImportStatement struct {
	NodeBase `json:"base:inclusionImportStmt"`
	Source   Node `json:"source,omitempty"`
}

func (InclusionImportStatement) Kind() NodeKind {
	return Stmt
}

func (stmt *InclusionImportStatement) PathSource() (_ string, absolute bool) {
	switch n := stmt.Source.(type) {
	case *RelativePathLiteral:
		return n.Value, false
	case *AbsolutePathLiteral:
		return n.Value, true
	}
	panic(errors.New(".Source of InclusionImportStatement is not a *RelativePathLiteral nor *AbsolutePathLiteral"))
}

type QuotedExpression struct {
	NodeBase
	Expression Node
}

func (QuotedExpression) Kind() NodeKind {
	return Expr
}

type QuotedStatements struct {
	NodeBase
	Statements []Node
}

func (QuotedStatements) Kind() NodeKind {
	return Expr
}

type UnquotedRegion struct {
	NodeBase
	Spread     bool
	Expression Node
}

func (UnquotedRegion) Kind() NodeKind {
	return Expr
}

type DynamicMemberExpression struct {
	NodeBase
	Left         Node
	PropertyName *IdentifierLiteral
	Optional     bool
}

func (DynamicMemberExpression) Kind() NodeKind {
	return Expr
}

type SpawnExpression struct {
	NodeBase
	//GroupVar Node //can be nil
	//Globals            Node //*KeyListExpression or *ObjectLiteral
	Meta   Node // cae be nil
	Module *EmbeddedModule
	//GrantedPermissions *ObjectLiteral //nil if no "allow ...." in the spawn expression
}

func (SpawnExpression) Kind() NodeKind {
	return Expr
}

type MappingExpression struct {
	NodeBase
	Entries []Node
}

func (MappingExpression) Kind() NodeKind {
	return Expr
}

type StaticMappingEntry struct {
	NodeBase
	Key   Node
	Value Node
}

type DynamicMappingEntry struct {
	NodeBase
	Key                   Node
	KeyVar                Node
	GroupMatchingVariable Node // can be nil
	ValueComputation      Node
}

type ComputeExpression struct {
	NodeBase
	Arg Node
}

func (ComputeExpression) Kind() NodeKind {
	return Expr
}

type TreedataLiteral struct {
	NodeBase
	Root     Node
	Children []*TreedataEntry
}

func (TreedataLiteral) Kind() NodeKind {
	return Expr
}

type TreedataEntry struct {
	NodeBase
	Value    Node
	Children []*TreedataEntry
}

type TreedataPair struct {
	NodeBase
	Key   Node
	Value Node
}

func (TreedataPair) Kind() NodeKind {
	return Expr
}

type PipelineStatement struct {
	NodeBase
	Stages []*PipelineStage
}

func (PipelineStatement) Kind() NodeKind {
	return Stmt
}

type PipelineExpression struct {
	NodeBase
	Stages []*PipelineStage
}

func (PipelineExpression) Kind() NodeKind {
	return Expr
}

type PipelineStageKind int

const (
	NormalStage PipelineStageKind = iota
	ParallelStage
)

type PipelineStage struct {
	Kind PipelineStageKind
	Expr Node
}

type PatternDefinition struct {
	NodeBase
	Left   Node //*PatternIdentifierLiteral if valid
	Right  Node
	IsLazy bool
}

func (d PatternDefinition) PatternName() (string, bool) {
	if ident, ok := d.Left.(*PatternIdentifierLiteral); ok {
		return ident.Name, true
	}
	return "", false
}

func (PatternDefinition) Kind() NodeKind {
	return Stmt
}

type PatternNamespaceDefinition struct {
	NodeBase
	Left   Node //*PatternNamespaceIdentifierLiteral if valid
	Right  Node
	IsLazy bool
}

func (d PatternNamespaceDefinition) NamespaceName() (string, bool) {
	if ident, ok := d.Left.(*PatternNamespaceIdentifierLiteral); ok {
		return ident.Name, true
	}
	return "", false
}

func (PatternNamespaceDefinition) Kind() NodeKind {
	return Stmt
}

type PatternNamespaceMemberExpression struct {
	NodeBase
	Namespace  *PatternNamespaceIdentifierLiteral
	MemberName *IdentifierLiteral
}

func (PatternNamespaceMemberExpression) Kind() NodeKind {
	return Expr
}

type ComplexStringPatternPiece struct {
	NodeBase
	Unprefixed bool
	Elements   []*PatternPieceElement
}

func (p *ComplexStringPatternPiece) IsResolvableAtCheckTime() bool {
	yes := true

	Walk(p, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		switch node := node.(type) {
		case *ComplexStringPatternPiece, *PatternPieceElement, *PatternUnion, *PatternGroupName,
			*RuneLiteral, *RegularExpressionLiteral, *DoubleQuotedStringLiteral, *MultilineStringLiteral,
			*IntLiteral:
		case *RuneRangeExpression:
			yes = utils.Implements[*RuneLiteral](node.Lower) && (node.Upper == nil || utils.Implements[*RuneLiteral](node.Upper))
		case *IntegerRangeLiteral:
			yes = utils.Implements[*IntLiteral](node.LowerBound) && (node.UpperBound == nil || utils.Implements[*IntLiteral](node.UpperBound))
		default:
			yes = false
		}
		if !yes {
			return StopTraversal, nil
		}
		return ContinueTraversal, nil
	}, nil)

	return yes
}

type OcurrenceCountModifier int

const (
	ExactlyOneOcurrence OcurrenceCountModifier = iota
	AtLeastOneOcurrence
	ZeroOrMoreOcurrence
	OptionalOcurrence
	ExactOcurrence
)

type PatternGroupName struct {
	NodeBase
	Name string
}

type PatternPieceElement struct {
	NodeBase
	Ocurrence           OcurrenceCountModifier
	ExactOcurrenceCount int
	Expr                Node
	GroupName           *PatternGroupName
}

type PatternUnion struct {
	NodeBase
	Cases []Node
}

func (PatternUnion) Kind() NodeKind {
	return Expr
}

type PatternCallExpression struct {
	NodeBase
	Callee    Node //*PatternIdentifierLiteral | *PatternNamespaceMemberExpression
	Arguments []Node
}

func (PatternCallExpression) Kind() NodeKind {
	return Expr
}

type ConcatenationExpression struct {
	NodeBase
	Elements []Node
}

func (ConcatenationExpression) Kind() NodeKind {
	return Expr
}

type AssertionStatement struct {
	NodeBase
	Expr Node
}

func (AssertionStatement) Kind() NodeKind {
	return Stmt
}

type RuntimeTypeCheckExpression struct {
	NodeBase
	Expr Node
}

func (RuntimeTypeCheckExpression) Kind() NodeKind {
	return Expr
}

type TestSuiteExpression struct {
	NodeBase    `json:"base:test-suite-expr"`
	Meta        Node            `json:"meta,omitempty"`
	Module      *EmbeddedModule `json:"embeddedModule,omitempty"`
	IsStatement bool            `json:"isStatement"`
}

func (e TestSuiteExpression) Kind() NodeKind {
	if e.IsStatement {
		return Stmt
	}
	return Expr
}

type TestCaseExpression struct {
	NodeBase    `json:"base:test-case-expr"`
	Meta        Node            `json:"meta,omitempty"`
	Module      *EmbeddedModule `json:"embeddedModule,omitempty"`
	IsStatement bool            `json:"isStatement"`
}

func (e TestCaseExpression) Kind() NodeKind {
	if e.IsStatement {
		return Stmt
	}
	return Expr
}

type LifetimejobExpression struct {
	NodeBase
	Meta    Node
	Subject Node // can be nil
	Module  *EmbeddedModule
}

func (LifetimejobExpression) Kind() NodeKind {
	return Expr
}

type ReceptionHandlerExpression struct {
	NodeBase
	Pattern Node
	Handler Node
}

func (ReceptionHandlerExpression) Kind() NodeKind {
	return Expr
}

//CSS selectors & combinators

type CssSelectorExpression struct {
	NodeBase
	Elements []Node
}

func (CssSelectorExpression) Kind() NodeKind {
	return Expr
}

type CssCombinator struct {
	NodeBase
	Name string
}

type CssClassSelector struct {
	NodeBase
	Name string
}

type CssPseudoClassSelector struct {
	NodeBase
	Name      string
	Arguments []Node
}

type CssPseudoElementSelector struct {
	NodeBase
	Name string
}

type CssTypeSelector struct {
	NodeBase
	Name string
}

type CssIdSelector struct {
	NodeBase
	Name string
}

type CssAttributeSelector struct {
	NodeBase
	AttributeName *IdentifierLiteral
	Pattern       string
	Value         Node
}

type XMLExpression struct {
	NodeBase  `json:"base:xml-expr"`
	Namespace Node        `json:"namespace,omitempty"` //*IdentifierLiteral or nil, NOT an XML namespace
	Element   *XMLElement `json:"element"`
}

func (XMLExpression) Kind() NodeKind {
	return Expr
}

func (e XMLExpression) EffectiveNamespaceName() string {
	if e.Namespace == nil {
		return globalnames.HTML_NS
	}

	return e.Namespace.(*IdentifierLiteral).Name
}

type XMLElement struct {
	NodeBase                `json:"base:xml-elem"`
	Opening                 *XMLOpeningElement `json:"opening,omitempty"`
	Children                []Node             `json:"children,omitempty"`
	Closing                 *XMLClosingElement `json:"closing,omitempty"`           //nil if self-closed
	RawElementContent       string             `json:"rawElementContent,omitempty"` //set for script and style tags
	RawElementContentStart  int32              `json:"rawElementContentStart,omitempty"`
	RawElementContentEnd    int32              `json:"rawElementContentEnd,omitempty"`
	EstimatedRawElementType RawElementType     `json:"estimatedRawElementType,omitempty"`

	//The following field can be set only if parsing RawElementContent is supported (js, css, hyperscript).
	RawElementParsingResult any `json:"-"` //example: *hscode.ParsingResult|*hscode.ParsingError
}

type RawElementType string

const (
	JsScript          RawElementType = "js-script"
	HyperscriptScript RawElementType = "hyperscript-script"
	CssStyleElem      RawElementType = "css-style"
)

type XMLOpeningElement struct {
	NodeBase   `json:"base:xml-opening-elem"`
	Name       Node   `json:"name"`
	Attributes []Node `json:"attributes"` //*XMLAttribute | *HyperscriptAttributeShorthand
	SelfClosed bool   `json:"selfClosed"`
}

func (attr XMLOpeningElement) GetName() string {
	return attr.Name.(*IdentifierLiteral).Name
}

type XMLClosingElement struct {
	NodeBase `json:"base:xml-closing-elem"`
	Name     Node `json:"name"`
}

type XMLAttribute struct {
	NodeBase `json:"base:xml-attr"`
	Name     Node `json:"name"`
	Value    Node `json:"value,omitempty"`
}

func (attr XMLAttribute) GetName() string {
	return attr.Name.(*IdentifierLiteral).Name
}

func (attr XMLAttribute) ValueIfStringLiteral() string {
	switch val := attr.Value.(type) {
	case *DoubleQuotedStringLiteral:
		return val.Value
	case *MultilineStringLiteral:
		return val.Value
	default:
		return ""
	}
}

type HyperscriptAttributeShorthand struct {
	NodeBase `json:"base:hs-attr-shorthand"`

	Value string `json:"value"`

	IsUnterminated bool `json:"isUnterminated"`

	//The following fields can be set only if hyperscript parsing is supported.

	HyperscriptParsingResult *hscode.ParsingResult `json:"-"`
	HyperscriptParsingError  *hscode.ParsingError  `json:"-"`
}

type XMLText struct {
	NodeBase `json:"base:xml-text"`
	Raw      string `json:"raw"`
	Value    string `json:"value"`
}

type XMLInterpolation struct {
	NodeBase `json:"base:xml-interpolation"`
	Expr     Node `json:"expr"`
}

type ExtendStatement struct {
	NodeBase
	ExtendedPattern Node
	Extension       Node //*ObjectLiteral if coorecy
}

func (ExtendStatement) Kind() NodeKind {
	return Stmt
}

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

		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *PreinitStatement:
		walk(n.Block, node, ancestorChain, fn, afterFn)
	case *Manifest:
		walk(n.Object, node, ancestorChain, fn, afterFn)
	case *EmbeddedModule:
		walk(n.Manifest, node, ancestorChain, fn, afterFn)

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
	case *LocalVariableDeclaration:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
	case *GlobalVariableDeclarations:
		for _, decl := range n.Declarations {
			walk(decl, node, ancestorChain, fn, afterFn)
		}
	case *GlobalVariableDeclaration:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.Type, node, ancestorChain, fn, afterFn)
		walk(n.Right, node, ancestorChain, fn, afterFn)
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
		for _, stmt := range n.Statements {
			walk(stmt, node, ancestorChain, fn, afterFn)
		}
	case *UnquotedRegion:
		walk(n.Expression, node, ancestorChain, fn, afterFn)
	case *DynamicMemberExpression:
		walk(n.Left, node, ancestorChain, fn, afterFn)
		walk(n.PropertyName, node, ancestorChain, fn, afterFn)
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
	case *LifetimejobExpression:
		walk(n.Meta, node, ancestorChain, fn, afterFn)
		walk(n.Subject, node, ancestorChain, fn, afterFn)
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
	case *XMLExpression:
		walk(n.Namespace, node, ancestorChain, fn, afterFn)
		walk(n.Element, node, ancestorChain, fn, afterFn)
	case *XMLElement:
		walk(n.Opening, node, ancestorChain, fn, afterFn)
		for _, child := range n.Children {
			walk(child, node, ancestorChain, fn, afterFn)
		}
		walk(n.Closing, node, ancestorChain, fn, afterFn)
	case *XMLOpeningElement:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		for _, attr := range n.Attributes {
			walk(attr, node, ancestorChain, fn, afterFn)
		}
	case *XMLAttribute:
		walk(n.Name, node, ancestorChain, fn, afterFn)
		walk(n.Value, node, ancestorChain, fn, afterFn)
	case *XMLClosingElement:
		walk(n.Name, node, ancestorChain, fn, afterFn)
	case *XMLInterpolation:
		walk(n.Expr, node, ancestorChain, fn, afterFn)
	case *ExtendStatement:
		walk(n.ExtendedPattern, node, ancestorChain, fn, afterFn)
		walk(n.Extension, node, ancestorChain, fn, afterFn)
	case *LongValuePathLiteral:
		for _, segment := range n.Segments {
			walk(segment, node, ancestorChain, fn, afterFn)
		}
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

func FindLocalVarDeclWithName(root Node, name string) *LocalVariableDeclaration {
	return FindNode(root, (*LocalVariableDeclaration)(nil), func(n *LocalVariableDeclaration, isFirstFound bool, ancestors []Node) bool {
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
