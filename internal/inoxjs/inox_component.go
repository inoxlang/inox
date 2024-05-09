package inoxjs

import (
	"context"
	"errors"
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/intconv"
)

const (
	INTERPOLATION_OPENING_DELIMITER = "(("
	INTERPOLATION_CLOSING_DELIMITER = "))"
	INIT_COMPONENT_FN_NAME          = "initComponent"
)



func ContainsClientSideInterpolation(s string) bool {
	openingDelimIndex := strings.Index(s, INTERPOLATION_OPENING_DELIMITER)
	if openingDelimIndex < 0 {
		return false
	}
	closingDelimIndex := strings.Index(s, INTERPOLATION_CLOSING_DELIMITER)
	return closingDelimIndex > openingDelimIndex
}

type ClientSideInterpolation struct {
	Expression          string //includes leading and trailing space
	ParsingResult       *hscode.ParsingResult
	ParsingError        *hscode.ParsingError
	StartRuneIndex      int32 //index of the opening delimiter in the encoded string.
	EndRuneIndex        int32
	InnerStartRuneIndex int32
	InnerEndRuneIndex   int32
}

// ParseClientSideInterpolations parses the client side interpolations in a string, the second parameter is used to determine the
// span of the interpolation.
func ParseClientSideInterpolations(ctx context.Context, str, encoded string) (interpolations []ClientSideInterpolation, criticalErr error) {
	if len(str) <= 1 {
		return
	}

	if strings.Count(str, INTERPOLATION_OPENING_DELIMITER) != strings.Count(encoded, INTERPOLATION_OPENING_DELIMITER) ||
		strings.Count(str, INTERPOLATION_CLOSING_DELIMITER) != strings.Count(encoded, INTERPOLATION_CLOSING_DELIMITER) {
		criticalErr = errors.New("the encoded string containing the interpolations should not contain encoded '((' or '))' sequences")
		return
	}

	encodedStrRunes := []rune(encoded)
	encodedStrRuneCount := intconv.MustIToI32(len(encodedStrRunes))

	i := int32(1)
	inInterpolation := false
	innerStart := int32(-1)

	//Find interpolations in the encoded string.
	var encodedStrInterpolationSpans [][2]int32

	for i < encodedStrRuneCount {
		if !inInterpolation && encodedStrRunes[i] == '(' && encodedStrRunes[i-1] == '(' {
			i++
			inInterpolation = true
			innerStart = i
			continue
		}
		if inInterpolation && encodedStrRunes[i] == ')' && encodedStrRunes[i-1] == ')' {
			encodedStrInterpolationSpans = append(encodedStrInterpolationSpans, [2]int32{innerStart - 2, i + 1})
			innerStart = -1
			inInterpolation = false

			i++
			continue
		}

		i++
	}

	i = 1
	inInterpolation = false
	innerStart = -1

	runes := []rune(str)
	runeCount := intconv.MustIToI32(len(runes))

	//Find interpolations in text.

	for i < runeCount {
		if !inInterpolation && runes[i] == '(' && runes[i-1] == '(' {
			i++
			inInterpolation = true
			innerStart = i
			continue
		}
		if inInterpolation && runes[i] == ')' && runes[i-1] == ')' {
			interpIndex := len(interpolations)
			if interpIndex >= len(encodedStrInterpolationSpans) {
				interpolations = nil
				criticalErr = errors.New("the encoded string does not match the decoded string")
				return
			}

			openinDelimIndex := encodedStrInterpolationSpans[interpIndex][0]     //index in encoded
			closingDelimEndIndex := encodedStrInterpolationSpans[interpIndex][1] //index in encoded

			interpolations = append(interpolations, ClientSideInterpolation{
				Expression:     string(runes[innerStart : i-1]),
				StartRuneIndex: openinDelimIndex,
				EndRuneIndex:   closingDelimEndIndex,

				InnerStartRuneIndex: openinDelimIndex + int32(len(INTERPOLATION_OPENING_DELIMITER)),
				InnerEndRuneIndex:   closingDelimEndIndex - int32(len(INTERPOLATION_CLOSING_DELIMITER)),
			})
			innerStart = -1
			inInterpolation = false

			i++
			continue
		}

		i++
	}

	//Parse Hyperscript expressions.

	for interpIndex, interp := range interpolations {
		if utils.IsContextDone(ctx) {
			interpolations = nil
			criticalErr = ctx.Err()
			return
		}

		result, parsingErr, err := hsparse.ParseHyperScriptExpression(ctx, interp.Expression)
		if err != nil {
			criticalErr = err
			return
		}

		interp.ParsingResult = result
		interp.ParsingError = parsingErr
		interpolations[interpIndex] = interp
	}

	return
}
