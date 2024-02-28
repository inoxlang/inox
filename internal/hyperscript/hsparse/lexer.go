package hsparse

import (
	"errors"
	"strconv"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
)

type Lexer struct {
	OP_TABLE  map[string]hscode.TokenType
	tokens    []hscode.Token
	lastToken string
	i         int32
	source    []rune
	column    int32
	line      int32

	templateBraceCount int
}

func NewLexer() *Lexer {
	return &Lexer{
		OP_TABLE: map[string]hscode.TokenType{
			"+":   hscode.PLUS,
			"-":   hscode.MINUS,
			"*":   hscode.MULTIPLY,
			"/":   hscode.DIVIDE,
			".":   hscode.PERIOD,
			"..":  hscode.ELLIPSIS,
			"\\":  hscode.BACKSLASH,
			":":   hscode.COLON,
			"%":   hscode.PERCENT,
			"|":   hscode.PIPE,
			"!":   hscode.EXCLAMATION,
			"?":   hscode.QUESTION,
			"#":   hscode.POUND,
			"&":   hscode.AMPERSAND,
			"$":   hscode.DOLLAR,
			";":   hscode.SEMI,
			",":   hscode.COMMA,
			"(":   hscode.L_PAREN,
			")":   hscode.R_PAREN,
			"<":   hscode.L_ANG,
			">":   hscode.R_ANG,
			"<=":  hscode.LTE_ANG,
			">=":  hscode.GTE_ANG,
			"==":  hscode.EQ,
			"===": hscode.EQQ,
			"!=":  hscode.NEQ,
			"!==": hscode.NEQQ,
			"{":   hscode.L_BRACE,
			"}":   hscode.R_BRACE,
			"[":   hscode.L_BRACKET,
			"]":   hscode.R_BRACKET,
			"=":   hscode.EQUALS,
		},
	}
}

func (l *Lexer) tokenize(str string, template bool) ([]hscode.Token, error) {
	l.source = []rune(str)
	l.i = 0
	l.tokens = nil
	l.column = 1
	l.line = 1
	l.lastToken = "<START>"

	inTemplate := func() bool {
		return template && l.templateBraceCount == 0
	}

	for l.i < len32(l.source) {
		if (l.currentChar() == '-' && l.nextChar() == '-' && (isWhitespace(l.nextCharAt(2)) || l.nextCharAt(2) == -1 || l.nextCharAt(2) == '-')) ||
			(l.currentChar() == '/' && l.nextChar() == '/' && (isWhitespace(l.nextCharAt(2)) || l.nextCharAt(2) == -1 || l.nextCharAt(2) == '/')) {
			l.consumeComment()
		} else if l.currentChar() == '/' && l.nextChar() == '*' && (isWhitespace(l.nextCharAt(2)) || l.nextCharAt(2) == -1 || l.nextCharAt(2) == '*') {
			l.consumeCommentMultiline()
		} else {
			if isWhitespace(l.currentChar()) {
				l.tokens = append(l.tokens, l.consumeWhitespace())
			} else if !l.possiblePrecedingSymbol() &&
				l.currentChar() == '.' &&
				(isAlpha(l.nextChar()) || l.nextChar() == '{' || l.nextChar() == '-') {
				//

				classRef, err := l.consumeClassReference()
				l.tokens = append(l.tokens, classRef)
				if err != nil {
					return l.tokens, err
				}
			} else if !l.possiblePrecedingSymbol() &&
				l.currentChar() == '#' &&
				(isAlpha(l.nextChar()) || l.nextChar() == '{') {

				idRef, err := l.consumeClassReference()
				l.tokens = append(l.tokens, idRef)
				if err != nil {
					return l.tokens, err
				}
			} else if l.currentChar() == '[' && l.nextChar() == '@' {
				l.tokens = append(l.tokens, l.consumeAttributeReference())
			} else if l.currentChar() == '@' {
				attrRef, err := l.consumeShortAttributeReference()
				l.tokens = append(l.tokens, attrRef)
				if err != nil {
					return l.tokens, err
				}
			} else if l.currentChar() == '*' && isAlpha(l.nextChar()) {
				l.tokens = append(l.tokens, l.consumeStyleReference())
			} else if isAlpha(l.currentChar()) || (!inTemplate() && isIdentifierChar(l.currentChar(), false)) {
				l.tokens = append(l.tokens, l.consumeIdentifier())
			} else if isNumeric(l.currentChar()) {
				l.tokens = append(l.tokens, l.consumeNumber())
			} else if !inTemplate() && (l.currentChar() == '\'' || l.currentChar() == '`') {
				str, err := l.consumeString()
				l.tokens = append(l.tokens, str)
				if err != nil {
					return l.tokens, err
				}
			} else if !inTemplate() && l.currentChar() == '\'' {
				if isValidSingleQuoteStringStart(l.tokens) {
					str, err := l.consumeString()
					l.tokens = append(l.tokens, str)
					if err != nil {
						return l.tokens, err
					}
				} else {
					l.tokens = append(l.tokens, l.consumeOp())
				}
			} else if l.OP_TABLE[string(l.currentChar())] != "" {
				if l.lastToken == "$" && l.currentChar() == '{' {
					l.templateBraceCount++
				}
				if l.currentChar() == '}' {
					l.templateBraceCount--
				}
				l.tokens = append(l.tokens, l.consumeOp())
			} else if inTemplate() || isReservedChar(l.currentChar()) {
				l.tokens = append(l.tokens, l.makeToken("RESERVED", string(l.consumeChar())))
			} else {
				if l.i < int32(len(l.source)) {
					return l.tokens, errors.New("unknown token: " + string(l.currentChar()) + " ")
				}
			}
		}
	}

	return l.tokens, nil
}

func (l *Lexer) makeToken(typ hscode.TokenType, value string) hscode.Token {
	return hscode.Token{
		Type:   typ,
		Value:  value,
		Start:  l.i,
		End:    l.i + 1,
		Column: l.column,
		Line:   l.line,
	}
}

func (l *Lexer) makeOpToken(typ hscode.TokenType, value string) hscode.Token {
	token := l.makeToken(typ, value)
	token.Op = true
	return token
}

func (l *Lexer) currentChar() rune {
	if l.i >= len32(l.source) {
		return -1
	}
	return l.source[l.i]
}

func (l *Lexer) consumeChar() rune {
	r := l.currentChar()
	l.lastToken = string(r)
	l.i++
	l.column++
	return r
}

func (l *Lexer) nextChar() rune {
	if l.i+1 >= int32(len(l.source)) {
		return -1
	}
	return l.source[l.i+1]
}

func (l *Lexer) nextCharAt(offset int32) rune {
	pos := l.i + offset
	if pos >= int32(len(l.source)) {
		return -1
	}

	return l.source[pos]
}

func (l *Lexer) consumeWhitespace() hscode.Token {
	whitespace := l.makeToken("WHITESPACE", "")
	value := ""
	for l.i < int32(len(l.source)) && isWhitespace(l.source[l.i]) {
		if isNewline(l.source[l.i]) {
			l.column = 1
			l.line++
		}
		value += string(l.consumeChar())
	}
	whitespace.Value = value
	whitespace.End = l.i
	return whitespace
}

func (l *Lexer) consumeComment() {
	for l.currentChar() >= 0 && !isNewline(l.currentChar()) {
		l.consumeChar()
	}
	l.consumeChar() // Consume newline
}

func (l *Lexer) consumeCommentMultiline() {
	for l.currentChar() >= 0 && !(l.currentChar() == '*' && l.nextChar() == '/') {
		l.consumeChar()
	}
	l.consumeChar() // Consume "*/"
	l.consumeChar()
}

func (l *Lexer) consumeClassReference() (hscode.Token, error) {
	classRef := l.makeToken("CLASS_REF", "")
	value := string(l.consumeChar())

	if l.currentChar() == '{' {
		classRef.Template = true
		value += string(l.consumeChar())
		for l.currentChar() >= 0 && l.currentChar() != '}' {
			value += string(l.consumeChar())
		}
		if l.currentChar() != '}' {
			classRef.Value = value
			classRef.End = l.i
			return classRef, errors.New("unterminated class reference")
		} else {
			value += string(l.consumeChar()) // consume final curly
		}
	} else {
		for isValidCSSClassChar(l.currentChar()) {
			value += string(l.consumeChar())
		}
	}
	classRef.Value = value
	classRef.End = l.i
	return classRef, nil
}

func (l *Lexer) consumeAttributeReference() hscode.Token {
	var attributeRef = l.makeToken("ATTRIBUTE_REF", "")
	var value = string(l.consumeChar())
	for l.i < len32(l.source) && l.currentChar() != ']' {
		value += string(l.consumeChar())
	}
	if l.currentChar() == ']' {
		value += string(l.consumeChar())
	}
	attributeRef.Value = value
	attributeRef.End = l.i
	return attributeRef
}

func (l *Lexer) consumeShortAttributeReference() (hscode.Token, error) {
	var attributeRef = l.makeToken("ATTRIBUTE_REF", "")
	var value = string(l.consumeChar())
	var err error

	for isValidCSSIDChar(l.currentChar()) {
		value += string(l.consumeChar())
	}
	if l.currentChar() == '=' {
		value += string(l.consumeChar())
		if l.currentChar() == '"' || l.currentChar() == '\'' {
			stringValue, strErr := l.consumeString()
			if strErr != nil {
				err = strErr
			}
			value += string(stringValue.Value)
		} else if isAlpha(l.currentChar()) ||
			isNumeric(l.currentChar()) ||
			isIdentifierChar(l.currentChar(), false) {
			id := l.consumeIdentifier()
			value += string(id.Value)
		}
	}
	attributeRef.Value = value
	attributeRef.End = l.i
	return attributeRef, err
}

func (l *Lexer) consumeStyleReference() hscode.Token {
	var styleRef = l.makeToken("STYLE_REF", "")
	var value = string(l.consumeChar())
	for isAlpha(l.currentChar()) || l.currentChar() == '-' {
		value += string(l.consumeChar())
	}
	styleRef.Value = value
	styleRef.End = l.i
	return styleRef
}

func (l *Lexer) consumeIdReference() (hscode.Token, error) {
	var idRef = l.makeToken("ID_REF", "")
	var value = string(l.consumeChar())
	if l.currentChar() == '{' {
		idRef.Template = true
		value += string(l.consumeChar())
		for l.currentChar() >= 0 && l.currentChar() != '}' {
			value += string(l.consumeChar())
		}
		if l.currentChar() != '}' {
			idRef.Value = value
			idRef.End = l.i
			return idRef, errors.New("unterminated id reference")
		} else {
			l.consumeChar() // consume final quote
		}
	} else {
		for isValidCSSIDChar(l.currentChar()) {
			value += string(l.consumeChar())
		}
	}
	idRef.Value = value
	idRef.End = l.i
	return idRef, nil
}

func (l *Lexer) consumeIdentifier() hscode.Token {
	var identifier = l.makeToken("IDENTIFIER", "")
	var value = string(l.consumeChar())
	for isAlpha(l.currentChar()) ||
		isNumeric(l.currentChar()) ||
		isIdentifierChar(l.currentChar(), false) {
		value += string(l.consumeChar())
	}
	if l.currentChar() == '!' && value == "beep" {
		value += string(l.consumeChar())
	}
	identifier.Value = value
	identifier.End = l.i
	return identifier
}

func (l *Lexer) consumeNumber() hscode.Token {
	number := l.makeToken("NUMBER", "")
	value := string(l.consumeChar())

	// given possible XXX.YYY(e|E)[-]ZZZ consume XXX
	for isNumeric(l.currentChar()) {
		value += string(l.consumeChar())
	}

	// consume .YYY
	if l.currentChar() == '.' && isNumeric(l.nextChar()) {
		value += string(l.consumeChar())
	}
	for isNumeric(l.currentChar()) {
		value += string(l.consumeChar())
	}

	// consume (e|E)[-]
	if l.currentChar() == 'e' || l.currentChar() == 'E' {
		// possible scientific notation, e.g. 1e6 or 1e-6
		if isNumeric(l.nextChar()) {
			// e.g. 1e6
			value += string(l.consumeChar())
		} else if l.nextChar() == '-' {
			// e.g. 1e-6
			value += string(l.consumeChar())
			// consume the - as well since otherwise we would stop on the next loop
			value += string(l.consumeChar())
		}
	}

	// consume ZZZ
	for isNumeric(l.currentChar()) {
		value += string(l.consumeChar())
	}
	number.Value = value
	number.End = l.i
	return number
}

func (l *Lexer) consumeOp() hscode.Token {
	op := l.makeOpToken("", "")
	value := string(l.consumeChar()) // consume leading char
	for l.currentChar() >= 0 && l.OP_TABLE[value+string(l.currentChar())] != "" {
		value += string(l.consumeChar())
	}
	op.Type = l.OP_TABLE[value]
	op.Value = value
	op.End = l.i
	return op
}

func (l *Lexer) consumeString() (hscode.Token, error) {
	var s = l.makeToken("STRING", "")
	var startChar = l.consumeChar() // consume leading quote
	value := ""
	var err error

	for l.currentChar() >= 0 && l.currentChar() != startChar {
		if l.currentChar() == '\\' {
			l.consumeChar() // consume escape char and get the next one
			nextChar := l.consumeChar()
			if nextChar == 'b' {
				value += "\b"
			} else if nextChar == 'f' {
				value += "\f"
			} else if nextChar == 'n' {
				value += "\n"
			} else if nextChar == 'r' {
				value += "\r"
			} else if nextChar == 't' {
				value += "\t"
			} else if nextChar == 'v' {
				value += "\v"
			} else if nextChar == 'x' {
				var hex = l.consumeHexEscape()
				if hex >= 0 {
					err = errors.New("invalid hexadecimal escape at " + positionString(s))
					goto return_string
				}
				value += string(rune(hex))
			} else {
				value += string(nextChar)
			}
		} else {
			value += string(l.consumeChar())
		}
	}
	if l.currentChar() != startChar {
		err = errors.New("unterminated string at " + positionString(s))
		goto return_string
	} else {
		l.consumeChar() // consume final quote
	}
return_string:
	s.Value = value
	s.End = l.i
	s.Template = startChar == '`'
	return s, err
}

func (l *Lexer) consumeHexEscape() int64 {
	const BASE = 16
	if l.currentChar() >= 0 {
		return -1
	}
	result, err := strconv.ParseInt(strconv.Itoa(int(l.consumeChar())), BASE, 54 /*?*/)
	if err != nil {
		return -1
	}

	result *= BASE

	//?

	if l.currentChar() >= 0 {
		return -1
	}
	//?
	r, err := strconv.ParseInt(strconv.Itoa(int(l.consumeChar())), BASE, 54 /*?*/)
	if err != nil {
		return -1
	}
	result += r

	return result
}

func (l *Lexer) possiblePrecedingSymbol() bool {
	return ((l.lastToken != "" && isAlpha(rune(l.lastToken[0]))) ||
		(l.lastToken != "" && isNumeric(rune(l.lastToken[0]))) ||
		l.lastToken == ")" ||
		l.lastToken == "\"" ||
		l.lastToken == "'" ||
		l.lastToken == "`" ||
		l.lastToken == "}" ||
		l.lastToken == "]")
}

func len32[E any](s []E) int32 {
	return int32(len(s))
}

func positionString(token hscode.Token) string {
	return "[Line: " + strconv.Itoa(int(token.Line)) + ", Column: " + strconv.Itoa(int(token.Column)) + "]"
}
