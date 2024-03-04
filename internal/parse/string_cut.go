package parse

import (
	"unicode"
	"unicode/utf8"
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

func CutQuotedStringLiteral(index int32, n *QuotedStringLiteral) (cut stringCut, ok bool) {

	if n.Err != nil {
		return stringCut{}, false
	}

	//Do not cut if the index is outside the string.
	if index <= n.Span.Start || index >= n.Span.End {
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
			HasSpaceAfterIndex: !isStringEmpty && isFirstRuneSpace(n.Value),
			IsStringEmpty:      isStringEmpty,
		}, true
	}

	if index == n.Span.End-1 { //"string<here>"
		return stringCut{
			BeforeIndex:         n.Value,
			AfterIndex:          "",
			IsIndexAtStart:      isStringEmpty,
			IsIndexAtEnd:        true,
			IsStringEmpty:       isStringEmpty,
			HasSpaceBeforeIndex: !isStringEmpty && isLastRuneSpace(n.Value),
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
