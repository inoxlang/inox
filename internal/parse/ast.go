package parse

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
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

type MissingStatement struct {
	NodeBase    `json:"base:missing-statement"`
	Annotations *MetadataAnnotations `json:"annotations,omitempty"`
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
		*MetadataAnnotations,   //MetadataAnnotation being a scope container is not 100% incorrect
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
	RegionHeaders              []*AnnotatedRegionHeader    `json:"regionHeaders,omitempty"`
	Statements                 []Node                      `json:"statements,omitempty"`
	IsShellChunk               bool

	//mostly valueless tokens, sorted by position (ascending).
	//EmbeddedModule nodes hold references to subslices of .Tokens.
	Tokens []Token `json:"tokens,omitempty"`
}

type EmbeddedModule struct {
	NodeBase       `json:"base:embedded-module"`
	Manifest       *Manifest                `json:"manifest,omitempty"` //can be nil
	RegionHeaders  []*AnnotatedRegionHeader `json:"regionHeaders,omitempty"`
	Statements     []Node                   `json:"statements,omitempty"`
	SingleCallExpr bool                     `json:"isSingleCallExpr"`
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
	Value      string
	Raw        string
	Unprefixed bool
}

func (l HostPatternLiteral) ValueString() string {
	return l.Value
}

func (HostPatternLiteral) Kind() NodeKind {
	return Expr
}

type URLPatternLiteral struct {
	NodeBase
	Value      string
	Raw        string
	Unprefixed bool
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
	Key         Node //can be nil (error)
	Type        Node //can be nil
	Value       Node
	Annotations *MetadataAnnotations //can be nil
	Optional    bool
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

type MetaIdentifier struct {
	NodeBase
	Name string
}

func (MetaIdentifier) Kind() NodeKind {
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

func (e CallExpression) IsMetaCallee() bool {
	switch callee := e.Callee.(type) {
	case *MetaIdentifier:
		return true
	case *MemberExpression:
		return utils.Implements[*MetaIdentifier](callee.Left)
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
	RegionHeaders []*AnnotatedRegionHeader
	Statements    []Node
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
	Annotations *MetadataAnnotations //can be nil
	Function    *FunctionExpression
	Name        Node //*IdentifierLiteral | *UnquotedRegion
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
	RegionHeaders []*AnnotatedRegionHeader
	Statements    []Node
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

type SequencePatternQuantifier int

const (
	ExactlyOneOccurrence SequencePatternQuantifier = iota
	AtLeastOneOccurrence
	ZeroOrMoreOccurrences
	OptionalOccurrence
	ExactOccurrenceCount
)

type PatternGroupName struct {
	NodeBase
	Name string
}

type PatternPieceElement struct {
	NodeBase
	Quantifier          SequencePatternQuantifier
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
	Opening                 *XMLOpeningElement       `json:"opening,omitempty"`
	RegionHeaders           []*AnnotatedRegionHeader `json:"regionHeaders,omitempty"`
	Children                []Node                   `json:"children,omitempty"`
	Closing                 *XMLClosingElement       `json:"closing,omitempty"`           //nil if self-closed
	RawElementContent       string                   `json:"rawElementContent,omitempty"` //set for script and style tags
	RawElementContentStart  int32                    `json:"rawElementContentStart,omitempty"`
	RawElementContentEnd    int32                    `json:"rawElementContentEnd,omitempty"`
	EstimatedRawElementType RawElementType           `json:"estimatedRawElementType,omitempty"`

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
	Extension       Node //*ObjectLiteral if correct
}

func (ExtendStatement) Kind() NodeKind {
	return Stmt
}

type XMLPatternExpression struct {
	NodeBase `json:"base:xml-pattern-expr"`
	Element  *XMLPatternElement `json:"element"`
}

func (XMLPatternExpression) Kind() NodeKind {
	return Expr
}

type XMLPatternElement struct {
	NodeBase      `json:"base:xml-pattern-elem"`
	Quantifier    XMLPatternElementQuantifier `json:"quantifier"`
	Opening       *XMLPatternOpeningElement   `json:"opening,omitempty"`
	RegionHeaders []*AnnotatedRegionHeader    `json:"regionHeaders,omitempty"`
	Children      []Node                      `json:"children,omitempty"`
	Closing       *XMLPatternClosingElement   `json:"closing,omitempty"` //nil if self-closed

	RawElementContent       string         `json:"rawElementContent,omitempty"` //set for script and style tags
	RawElementContentStart  int32          `json:"rawElementContentStart,omitempty"`
	RawElementContentEnd    int32          `json:"rawElementContentEnd,omitempty"`
	EstimatedRawElementType RawElementType `json:"estimatedRawElementType,omitempty"`
}

type XMLPatternElementQuantifier int

const (
	OneXmlElement XMLPatternElementQuantifier = iota
	OptionalXmlElement
	ZeroOrMoreXmlElements
	OneOrMoreXmlElements
)

type XMLPatternOpeningElement struct {
	NodeBase   `json:"base:xml-pattern-opening-elem"`
	Name       Node   `json:"name"`
	Attributes []Node `json:"attributes"` //*XMLPatternAttribute
	SelfClosed bool   `json:"selfClosed"`
}

func (attr XMLPatternOpeningElement) GetName() string {
	return attr.Name.(*IdentifierLiteral).Name
}

type XMLPatternClosingElement struct {
	NodeBase `json:"base:xml-pattern-closing-elem"`
	Name     Node `json:"name"`
}

type XMLPatternAttribute struct {
	NodeBase `json:"base:xml-pattern-attr"`
	Name     Node `json:"name"`
	Value    Node `json:"value,omitempty"` //can be nil
}

type XMLPatternWildcard struct {
	NodeBase `json:"base:xml-pattern-wildcard"`
}

func (attr XMLPatternAttribute) GetName() string {
	return attr.Name.(*IdentifierLiteral).Name
}

func (attr XMLPatternAttribute) ValueIfStringLiteral() string {
	switch val := attr.Value.(type) {
	case *DoubleQuotedStringLiteral:
		return val.Value
	case *MultilineStringLiteral:
		return val.Value
	default:
		return ""
	}
}

type XMLPatternInterpolation struct {
	NodeBase `json:"base:xml-pattern-interpolation"`
	Expr     Node `json:"expr"`
}

type MetadataAnnotations struct {
	NodeBase
	Expressions []Node
}

type AnnotatedRegionHeader struct {
	NodeBase
	Text        *AnnotatedRegionHeaderText
	Annotations *MetadataAnnotations
}

type AnnotatedRegionHeaderText struct {
	NodeBase
	Raw   string
	Value string
}
