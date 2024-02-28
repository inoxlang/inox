package hscode

import "reflect"

type Node interface {
	Base() NodeBase
}

type NodeBase struct {
	Type       NodeType `json:"type"`
	StartToken *Token   `json:"startToken,omitempty"`
	EndToken   *Token   `json:"endToken,omitempty"`
	IsFeature  bool     `json:"isFeature"`
}

func (n NodeBase) IsZero() bool {
	return reflect.ValueOf(n).IsZero()
}

func (n NodeBase) IsNotZero() bool {
	return !reflect.ValueOf(n).IsZero()
}

func (n NodeBase) StartPos() int32 {
	return n.StartToken.Start
}

func (n NodeBase) EndPos() int32 {
	return n.EndToken.End
}

func (n NodeBase) IncludedIn(other Node) bool {
	return n.StartPos() >= other.Base().StartPos() && n.EndPos() <= other.Base().EndPos()
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
	StyleLiteral                 NodeType = "styleLiteral"
	PseudopossessiveIts          NodeType = "pseudopossessiveIts"
	StyleRefValue                NodeType = "styleRefValue"
	Initial_literal              NodeType = "initial_literal"
	ClosestExpr                  NodeType = "closestExpr"

	OnFeature NodeType = "onFeature"

	SettleCmd NodeType = "settleCmd"
	AddCmd    NodeType = "addCmd"

	RemoveCommand     NodeType = "removeCommand"
	ToggleCommand     NodeType = "toggleCommand"
	HideCommand       NodeType = "hideCommand"
	ShowCommand       NodeType = "showCommand"
	TakeCommand       NodeType = "takeCommand"
	PutCommand        NodeType = "putCommand"
	TransitionCommand NodeType = "transitionCommand"
	MeasureCommand    NodeType = "measureCommand"
	GoCommand         NodeType = "goCommand"
	JsCommand         NodeType = "jsCommand"
	AsyncCommand      NodeType = "asyncCommand"
	TellCommand       NodeType = "tellCommand"
	WaitCommand       NodeType = "waitCommand"
	TriggerCommand    NodeType = "triggerCommand"
	ReturnCommand     NodeType = "returnCommand"
	ExitCommand       NodeType = "exitCommand"
	HaltCommand       NodeType = "haltCommand"
	LogCommand        NodeType = "logCommand"
	BeepCommand       NodeType = "beep!Command"
	ThrowCommand      NodeType = "throwCommand"
	CallCommand       NodeType = "callCommand"
	MakeCommand       NodeType = "makeCommand"
	GetCommand        NodeType = "getCommand"
	DefaultCommand    NodeType = "defaultCommand"
	SetCommand        NodeType = "setCommand"
	IfCommand         NodeType = "ifCommand"
	RepeatCommand     NodeType = "repeatCommand"
	ForCommand        NodeType = "forCommand"
	ContinueCommand   NodeType = "continueCommand"
	BreakCommand      NodeType = "breakCommand"
	AppendCommand     NodeType = "appendCommand"
	PickCommand       NodeType = "pickCommand"
	IncrementCommand  NodeType = "incrementCommand"
	DecrementCommand  NodeType = "decrementCommand"
	FetchCommand      NodeType = "fetchCommand"
)
