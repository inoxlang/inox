package parse

import (
	"unicode"
	"unicode/utf8"
)

type stringCut struct {
	BeforeIndex        string
	AfterIndex         string
	IsIndexAtStart     bool
	IsIndexAtEnd       bool
	HasSpaceAfterIndex bool
	IsStringEmpty      bool
}

func CutQuotedStringLiteral(index int32, n *QuotedStringLiteral) (cut stringCut, ok bool) {

	if n.Err != nil {
		return stringCut{}, false
	}

	//Note: $n could be an invalid string (error).

	isStringEmpty := n.Value == ""

	if index == n.Span.Start+1 { //"<here>string"
		return stringCut{
			BeforeIndex:        "",
			AfterIndex:         n.Value,
			IsIndexAtStart:     true,
			IsIndexAtEnd:       isStringEmpty,
			HasSpaceAfterIndex: n.Span.End != n.Span.Start+1 && isFirstRuneSpace(n.Value),
			IsStringEmpty:      isStringEmpty,
		}, true
	}

	if index == n.Span.End-1 { //"string<here>"
		return stringCut{
			BeforeIndex:    n.Value,
			AfterIndex:     "",
			IsIndexAtStart: isStringEmpty,
			IsIndexAtEnd:   true,
			IsStringEmpty:  isStringEmpty,
		}, true
	}

	relativeCursorPosition := index - n.Span.Start - 1
	runes := []rune(n.Raw[1 : len(n.Raw)-1])

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

	afterCursor := string(afterCursorBytes)

	return stringCut{
		BeforeIndex:        string(beforeCursorBytes),
		AfterIndex:         afterCursor,
		HasSpaceAfterIndex: isFirstRuneSpace(afterCursor),
	}, true
}

func isFirstRuneSpace(s string) bool {
	firstRune, _ := utf8.DecodeRuneInString(s)
	return unicode.IsSpace(firstRune)
}
