package parse

import (
	"strings"
	"unicode"

	"github.com/inoxlang/inox/internal/utils"
)

func HasPathLikeStart(s string) bool {
	if len(s) == 0 {
		return false
	}

	return s[0] == '/' || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../")
}

func containsNotEscapedBracket(s []rune) bool {
	for i, e := range s {
		if e == '{' {
			if utils.CountPrevBackslashes(s, int32(i))%2 == 0 {
				return true
			}
		}
	}
	return false
}

func containsNotEscapedDollar(s []rune) bool {
	for i, e := range s {
		if e == '$' {
			if utils.CountPrevBackslashes(s, int32(i))%2 == 0 {
				return true
			}
		}
	}
	return false
}

func IsForbiddenSpaceCharacter(r rune) bool {
	return unicode.IsSpace(r) && r != '\n' && !isSpaceNotLF(r)
}

func isValidEntryEnd(s []rune, i int32) bool {
	switch s[i] {
	case '\n', ',', '}':
	case '#':
		if i < len32(s)-1 && IsCommentFirstSpace(s[i+1]) {
			break
		}
		fallthrough
	default:
		return false
	}
	return true
}

func isNonIdentBinaryOperatorChar(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '\\', '>', '<', '?', '.', '!', '=':
		return true
	default:
		return false
	}
}

func isAcceptedReturnTypeStart(runes []rune, i int32) bool {
	switch runes[i] {
	case '%', '(', '[', '*':
		return true
	case '#':
		return i < len32(runes)-1 && (runes[i+1] == '{' || runes[i+1] == '[')
	default:
		return IsFirstIdentChar(runes[i])
	}
}

func isAcceptedFirstVariableTypeAnnotationChar(r rune) bool {
	return r == '%' || r == '#' || r == '*' || IsFirstIdentChar(r) || isOpeningDelim(r)
}

func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isAlphaOrUndescore(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

func isByteSliceBase(r rune) bool {
	switch r {
	case 'x', 'd', 'b':
		return true
	}
	return false
}

func isDecDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isOctalDigit(r rune) bool {
	return r >= '0' && r <= '7'
}

func IsIdentChar(r rune) bool {
	return isAlpha(r) || isDecDigit(r) || r == '-' || r == '_'
}

func IsFirstIdentChar(r rune) bool {
	return isAlpha(r) || r == '_'
}

func isInterpolationAllowedChar(r rune) bool {
	return IsIdentChar(r) || isDecDigit(r) || r == '[' || r == ']' || r == '.' || r == '$' || r == ':'
}

func isUnquotedStringChar(r rune) bool {
	return IsIdentChar(r) || r == '+' || r == '~' || r == '/' || r == '^' || r == '@' || r == '.' || r == '%'
}

func isValidUnquotedStringChar(runes []rune, i int32) bool {
	return isUnquotedStringChar(runes[i]) && (runes[i] != '/' || i == len32(runes)-1 || runes[i+1] != '>')
}

func isSpaceNotLF(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r'
}

func isEndOfLine(runes []rune, i int32) bool {
	if runes[i] == '\n' {
		return true
	}

	//eat carriage returns
	for ; i < len32(runes) && runes[i] == '\r'; i++ {

	}

	return i < len32(runes) && runes[i] == '\n'
}

func IsCommentFirstSpace(r rune) bool {
	return isSpaceNotLF(r)
}

func IsDelim(r rune) bool {
	switch r {
	case '{', '}', '[', ']', '(', ')', '\n', ',', ';', ':', '|':
		return true
	default:
		return false
	}
}

func isOpeningDelim(r rune) bool {
	switch r {
	case '{', '[', '(':
		return true
	default:
		return false
	}
}

func isUnpairedDelim(r rune) bool {
	switch r {
	case '\n', ',', ';', ':', '|':
		return true
	default:
		return false
	}
}

func isPairedDelim(r rune) bool {
	switch r {
	case '{', '}', '[', ']', '(', ')':
		return true
	default:
		return false
	}
}

func isClosingDelim(r rune) bool {
	switch r {
	case '}', ')', ']':
		return true
	default:
		return false
	}
}

func isUnpairedOrIsClosingDelim(r rune) bool {
	switch r {
	case '\n', ',', ';', ':', '=', ')', ']', '}', '|':
		return true
	default:
		return false
	}
}

func isNonSpaceCSSCombinator(r rune) bool {
	switch r {
	case '>', '~', '+':
		return true
	default:
		return false
	}
}
