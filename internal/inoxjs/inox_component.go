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
	CONDITIONAL_DISPLAY_ATTR_NAME   = "x-if"
	FOR_LOOP_ATTR_NAME              = "x-for"
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
	Expression                   string
	ParsingResult                *hscode.ParsingResult
	ParsingError                 *hscode.ParsingError
	StartRuneIndex, EndRuneIndex int32 //indexes in the encoded string.
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
	exprStart := int32(-1)

	//Find interpolations in the encoded string.
	var encodedStrInterpolationSpans [][2]int32

	for i < encodedStrRuneCount {
		if !inInterpolation && encodedStrRunes[i] == '(' && encodedStrRunes[i-1] == '(' {
			i++
			inInterpolation = true
			exprStart = i
			continue
		}
		if inInterpolation && encodedStrRunes[i] == ')' && encodedStrRunes[i-1] == ')' {
			encodedStrInterpolationSpans = append(encodedStrInterpolationSpans, [2]int32{exprStart - 2, i + 1})
			exprStart = -1
			inInterpolation = false

			i++
			continue
		}

		i++
	}

	i = 1
	inInterpolation = false
	exprStart = -1

	runes := []rune(str)
	runeCount := intconv.MustIToI32(len(runes))

	//Find interpolations in text.

	for i < runeCount {
		if !inInterpolation && runes[i] == '(' && runes[i-1] == '(' {
			i++
			inInterpolation = true
			exprStart = i
			continue
		}
		if inInterpolation && runes[i] == ')' && runes[i-1] == ')' {
			interpIndex := len(interpolations)
			if interpIndex >= len(encodedStrInterpolationSpans) {
				interpolations = nil
				criticalErr = errors.New("the encoded string does not match the decoded string")
				return
			}

			interpolations = append(interpolations, ClientSideInterpolation{
				Expression:     string(runes[exprStart : i-1]),
				StartRuneIndex: encodedStrInterpolationSpans[interpIndex][0],
				EndRuneIndex:   encodedStrInterpolationSpans[interpIndex][1],
			})
			exprStart = -1
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
