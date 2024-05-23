package parse

import (
	"unicode"
	"unicode/utf8"

	"github.com/inoxlang/inox/internal/ast"
)

type stringCut struct {
	BeforeIndex         string
	AfterIndex          string
	IsIndexAtStart      bool
	IsIndexAtEnd        bool
	HasSpaceBeforeIndex bool
	HasSpaceAfterIndex  bool
	IsStringEmpty       bool
}

func CutQuotedStringLiteral(index int32, n ast.SimpleValueLiteral) (cut stringCut, ok bool) {

	var value string
	var raw string

	switch n := n.(type) {
	case *ast.DoubleQuotedStringLiteral:
		value = n.ValueString()
		raw = n.Raw
	case *ast.MultilineStringLiteral:
		value = n.ValueString()
		raw = n.Raw
	default:
		return stringCut{}, false
	}

	base := n.Base()
	if base.Err != nil {
		return stringCut{}, false
	}

	//Do not cut if the index is outside the string.
	if index <= base.Span.Start || index >= base.Span.End {
		return stringCut{}, false
	}

	//Note: $n could be an invalid string (error).

	isStringEmpty := value == ""

	if index == base.Span.Start+1 { //"<here>string"
		return stringCut{
			BeforeIndex:        "",
			AfterIndex:         value,
			IsIndexAtStart:     true,
			IsIndexAtEnd:       isStringEmpty,
			HasSpaceAfterIndex: !isStringEmpty && isFirstRuneSpace(value),
			IsStringEmpty:      isStringEmpty,
		}, true
	}

	if index == base.Span.End-1 { //"string<here>"
		return stringCut{
			BeforeIndex:         value,
			AfterIndex:          "",
			IsIndexAtStart:      isStringEmpty,
			IsIndexAtEnd:        true,
			IsStringEmpty:       isStringEmpty,
			HasSpaceBeforeIndex: !isStringEmpty && isLastRuneSpace(value),
		}, true
	}

	relativeCursorPosition := index - base.Span.Start - 1
	runes := []rune(raw[1 : len(raw)-1])

	beforeCursorRunes := runes[:relativeCursorPosition]
	afterCursorRunes := runes[relativeCursorPosition:]

	beforeCursorBytes, ok := DecodeJsonStringBytesNoQuotes([]byte(string(beforeCursorRunes)))
	if !ok {
		return stringCut{}, false
	}

	afterCursorBytes, ok := DecodeJsonStringBytesNoQuotes([]byte(string(afterCursorRunes)))
	if !ok {
		return stringCut{}, false
	}

	beforeCursor := string(beforeCursorBytes)
	afterCursor := string(afterCursorBytes)

	return stringCut{
		BeforeIndex:         beforeCursor,
		AfterIndex:          afterCursor,
		HasSpaceBeforeIndex: isLastRuneSpace(beforeCursor),
		HasSpaceAfterIndex:  isFirstRuneSpace(afterCursor),
	}, true
}

func isFirstRuneSpace(s string) bool {
	firstRune, _ := utf8.DecodeRuneInString(s)
	return unicode.IsSpace(firstRune)
}

func isLastRuneSpace(s string) bool {
	firstRune, _ := utf8.DecodeLastRuneInString(s)
	return unicode.IsSpace(firstRune)
}
