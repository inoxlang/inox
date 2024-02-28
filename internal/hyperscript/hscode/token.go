package hscode

import "reflect"

type Token struct {
	Type  TokenType `json:"type"` //can be empty
	Value string    `json:"value"`

	Start int32 `json:"start"`
	End   int32 `json:"end"`

	Line   int32 `json:"line"`
	Column int32 `json:"column"`

	Op       bool
	Template bool //string template
}

func (t Token) IsZero() bool {
	return reflect.ValueOf(t).IsZero()
}

func (t Token) IsNotZero() bool {
	return !reflect.ValueOf(t).IsZero()
}

type TokenType string

const (
	PLUS        TokenType = "PLUS"
	MINUS       TokenType = "MINUS"
	MULTIPLY    TokenType = "MULTIPLY"
	DIVIDE      TokenType = "DIVIDE"
	PERIOD      TokenType = "PERIOD"
	ELLIPSIS    TokenType = "ELLIPSIS"
	BACKSLASH   TokenType = "BACKSLASH"
	COLON       TokenType = "COLON"
	PERCENT     TokenType = "PERCENT"
	PIPE        TokenType = "PIPE"
	EXCLAMATION TokenType = "EXCLAMATION"
	QUESTION    TokenType = "QUESTION"
	POUND       TokenType = "POUND"
	AMPERSAND   TokenType = "AMPERSAND"
	DOLLAR      TokenType = "DOLLAR"
	SEMI        TokenType = "SEMI"
	COMMA       TokenType = "COMMA"
	L_PAREN     TokenType = "L_PAREN"
	R_PAREN     TokenType = "R_PAREN"
	L_ANG       TokenType = "L_ANG"
	R_ANG       TokenType = "R_ANG"
	LTE_ANG     TokenType = "LTE_ANG"
	GTE_ANG     TokenType = "GTE_ANG"
	EQ          TokenType = "EQ"
	EQQ         TokenType = "EQQ"
	NEQ         TokenType = "NEQ"
	NEQQ        TokenType = "NEQQ"
	L_BRACE     TokenType = "L_BRACE"
	R_BRACE     TokenType = "R_BRACE"
	L_BRACKET   TokenType = "L_BRACKET"
	R_BRACKET   TokenType = "R_BRACKET"
	EQUALS      TokenType = "EQUALS"

	IDENTIFIER    TokenType = "IDENTIFIER"
	CLASS_REF     TokenType = "CLASS_REF"
	ATTRIBUTE_REF TokenType = "ATTRIBUTE_REF"
	ID_REF        TokenType = "ID_REF"
	STYLE_REF     TokenType = "STYLE_REF"
	NUMBER        TokenType = "NUMBER"
	STRING        TokenType = "STRING"
	WHITESPACE    TokenType = "WHITESPACE"
	RESERVED      TokenType = "RESERVED"
	EOF           TokenType = "EOF"
)
