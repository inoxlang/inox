package inoxjs

import (
	"context"
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	TEXT_INTERPOLATION_OPENING_DELIMITER = "(("
	TEXT_INTERPOLATION_CLOSING_DELIMITER = "))"
	CONDITIONAL_DISPLAY_ATTR_NAME        = "x-if"
	FOR_LOOP_ATTR_NAME                   = "x-for"
	INIT_COMPONENT_FN_NAME               = "initComponent"
)

func ContainsClientSideInterpolation(s string) bool {
	openingDelimIndex := strings.Index(s, TEXT_INTERPOLATION_OPENING_DELIMITER)
	if openingDelimIndex < 0 {
		return false
	}
	closingDelimIndex := strings.Index(s, TEXT_INTERPOLATION_CLOSING_DELIMITER)
	return closingDelimIndex > openingDelimIndex
}

type ClientSideInterpolation struct {
	Expression    string
	ParsingResult *hscode.ParsingResult
	ParsingError  *hscode.ParsingError
}

func ParseClientSideInterpolations(ctx context.Context, s string) (interpolations []ClientSideInterpolation, criticalErr error) {
	if len(s) <= 1 {
		return
	}

	runes := []rune(s)

	i := 1
	inInterpolation := false
	interpolationStart := -1

	//Find interpolations.

	for i < len(runes) {
		if !inInterpolation && runes[i] == '(' && runes[i-1] == '(' {
			i++
			inInterpolation = true
			interpolationStart = i
			continue
		}
		if inInterpolation && runes[i] == ')' && runes[i-1] == ')' {
			interpolations = append(interpolations, ClientSideInterpolation{
				Expression: string(runes[interpolationStart : i-1]),
			})
			interpolationStart = -1
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
