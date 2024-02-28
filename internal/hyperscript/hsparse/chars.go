package hsparse

import "github.com/inoxlang/inox/internal/hyperscript/hscode"

// Utility functions
func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNumeric(c rune) bool {
	return c >= '0' && c <= '9'
}

func isWhitespace(c rune) bool {
	return c == ' ' || c == '\t' || isNewline(c)
}

func isNewline(c rune) bool {
	return c == '\r' || c == '\n'
}

func isValidCSSClassChar(c rune) bool {
	return isAlpha(c) || isNumeric(c) || c == '-' || c == '_' || c == ':'
}

func isValidCSSIDChar(c rune) bool {
	return isAlpha(c) || isNumeric(c) || c == '-' || c == '_' || c == ':'
}

func isIdentifierChar(c rune, dollarIsOp bool) bool {
	return c == '_' || (dollarIsOp && c == '$')
}

func isReservedChar(c rune) bool {
	return c == '`' || c == '^'
}

func isValidSingleQuoteStringStart(tokens []hscode.Token) bool {
	if len(tokens) > 0 {
		previousToken := tokens[len(tokens)-1]
		if previousToken.Type == "IDENTIFIER" || previousToken.Type == "CLASS_REF" || previousToken.Type == "ID_REF" {
			return false
		}
		if previousToken.Op && (previousToken.Value == ">" || previousToken.Value == ")") {
			return false
		}
	}
	return true
}
