package hscode

import "github.com/inoxlang/inox/internal/sourcecode"

const (
	FILE_EXTENSION = "._hs"
)

type ParsingResult struct {
	//Node               Node    `json:"node"`
	NodeData           map[string]any `json:"nodeData"` //set by the JS-based parser. May be not set for perf reasons.
	Tokens             []Token        `json:"tokens"`
	TokensNoWhitespace []Token        `json:"tokensNoWhitespace"` //No tokens of type WHITESPACE (linefeeds tokens are still present).
}

type ParsingError struct {
	Message            string  `json:"message"`
	MessageAtToken     string  `json:"messageAtToken"`
	Token              Token   `json:"token"`
	Tokens             []Token `json:"tokens"`
	TokensNoWhitespace []Token `json:"tokensNoWhitespace"` //No tokens of type WHITESPACE (linefeeds tokens are still present).
}

func (e ParsingError) Error() string {
	return e.Message
}

func MakePositionFromParsingError(err *ParsingError, path string) sourcecode.PositionRange {
	token := err.Token
	return sourcecode.PositionRange{
		SourceName:  path,
		StartLine:   token.Line,
		EndLine:     token.Line,
		StartColumn: token.Column,
		EndColumn:   token.Column + token.End - token.Start,
		Span:        sourcecode.NodeSpan{Start: token.Start, End: token.End},
	}
}
