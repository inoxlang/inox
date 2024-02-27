package hscode

type Node struct {
	Type NodeType `json:"type"`

	Root         *Node `json:"root,omitempty"`
	Expression   *Node `json:"expression,omitempty"`
	Expr         *Node `json:"expr,omitempty"`
	Attribute    *Node `json:"attribute,omitempty"`
	From         *Node `json:"from,omitempty"`
	To           *Node `json:"to,omitempty"`
	FirstIndex   *Node `json:"firstIndex,omitempty"`
	SecondIndex  *Node `json:"secondIndex,omitempty"`
	Lhs          *Node `json:"lhs,omitempty"`
	Rhs          *Node `json:"rhs,omitempty"`
	Value        *Node `json:"value,omitempty"`
	Target       *Node `json:"target,omitempty"`
	AttributeRef *Node `json:"attributeRef,omitempty"`

	InElt     *Node `json:"inElt,omitempty"`
	WithinElt *Node `json:"withinElt,omitempty"`

	Args           []Node `json:"args,omitempty"`
	ArgExressions  []Node `json:"argExressions,omitempty"`
	ArgExpressions []Node `json:"argExpressions,omitempty"`
	Values         []Node `json:"values,omitempty"`
	Features       []Node `json:"features,omitempty"`

	Fields []Field `json:"field,omitempty"`

	Token       *Token `json:"token,omitempty"`
	NumberToken *Token `json:"numberToken,omitempty"`
	Prop        *Token `json:"prop,omitempty"`

	Name     string `json:"name,omitempty"`
	Key      string `json:"key,omitempty"`
	CSS      string `json:"css,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Operator string `json:"operator,omitempty"`
	JsSource string `json:"jsSource,omitempty"`

	TypeName string `json:"typeName,omitempty"`

	ExposedFunctionNames []string `json:"exposedFunctionNames,omitempty"`
	DotOrColonPath       []string `json:"dotOrColonPath,omitempty"`

	Time any `json:"time,omitempty"`

	IsFeature     bool `json:"isFeature"`
	ParentSearch  bool `json:"parentSearch,omitempty"`
	ForwardSearch bool `json:"forwardSearch,omitempty"`
	InSearch      bool `json:"inSearch,omitempty"`
	Wrapping      bool `json:"wrapping,omitempty"`
	NullOk        bool `json:"nullOk,omitempty"`
}

type Field struct {
	Name  Token
	Value Node
}

type NodeType string

const (
	HyperscriptProgram           NodeType = "hyperscript"
	EmptyCommandListCommand      NodeType = "emptyCommandListCommand"
	UnlessStatementModifier      NodeType = "unlessStatementModifier"
	ImplicitReturn               NodeType = "implicitReturn"
	StringLiteral                NodeType = "string"
	NakedStringLiteral           NodeType = "nakedString"
	NumberLiteral                NodeType = "number"
	IdRef                        NodeType = "idRef"
	IdRefTemplate                NodeType = "idRefTemplate"
	ClassRef                     NodeType = "classRef"
	ClassRefTemplate             NodeType = "classRefTemplate"
	QueryRef                     NodeType = "queryRef"
	AttributeRef                 NodeType = "attributeRef"
	StyleRef                     NodeType = "styleRef"
	ComputedStyleRef             NodeType = "computedStyleRef"
	ObjectKey                    NodeType = "objectKey"
	ObjectLiteral                NodeType = "objectLiteral"
	NamedArgumentList            NodeType = "namedArgumentList"
	Symbol                       NodeType = "symbol"
	ImplicitMeTarget             NodeType = "implicitMeTarget"
	Boolean                      NodeType = "boolean"
	Null                         NodeType = "null"
	ArrayLiteral                 NodeType = "arrayLiteral"
	BlockLiteral                 NodeType = "blockLiteral"
	PropertyAccess               NodeType = "propertyAccess"
	OfExpression                 NodeType = "ofExpression"
	Possessive                   NodeType = "possessive"
	InExpression                 NodeType = "inExpression"
	AsExpression                 NodeType = "asExpression"
	FunctionCall                 NodeType = "functionCall"
	AttributeRefAccess           NodeType = "attributeRefAccess"
	ArrayIndex                   NodeType = "arrayIndex"
	StringPostfix                NodeType = "stringPostfix"
	TimeExpression               NodeType = "timeExpression"
	TypeCheck                    NodeType = "typeCheck"
	LogicalNot                   NodeType = "logicalNot"
	NoExpression                 NodeType = "noExpression"
	NegativeNumber               NodeType = "negativeNumber"
	BeepExpression               NodeType = "beepExpression"
	RelativePositionalExpression NodeType = "relativePositionalExpression"
	PositionalExpression         NodeType = "positionalExpression"
	MathOperator                 NodeType = "mathOperator"
	MathExpression               NodeType = "mathExpression"
	ComparisonOperator           NodeType = "comparisonOperator"
	LogicalOperator              NodeType = "logicalOperator"
	AsyncExpression              NodeType = "asyncExpression"
	JsBody                       NodeType = "jsBody"
	WaitCmd                      NodeType = "waitCmd"
	DotOrColonPath               NodeType = "dotOrColonPath"
	PseudoCommand                NodeType = "pseudoCommand"
	WaitATick                    NodeType = "waitATick"
	ImplicitIncrementOp          NodeType = "implicitIncrementOp"
	SettleCmd                    NodeType = "settleCmd"
	AddCmd                       NodeType = "addCmd"
	StyleLiteral                 NodeType = "styleLiteral"
	PseudopossessiveIts          NodeType = "pseudopossessiveIts"
	StyleRefValue                NodeType = "styleRefValue"
	Initial_literal              NodeType = "initial_literal"
	ClosestExpr                  NodeType = "closestExpr"
)
