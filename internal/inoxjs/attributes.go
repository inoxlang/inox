package inoxjs

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/parse"
)

const (
	CONDITIONAL_DISPLAY_ATTR_NAME = "x-if"
	FOR_LOOP_ATTR_NAME            = "x-for"

	//error messages
	INOXJS_ATTRS_SHOULD_HAVE_A_STRING_LIT_AS_VALUE = "inoxjs attributes should have a string literal as value"
	INVALID_VALUE_FOR_FOR_LOOP_ATTR                = "invalid value, valid examples: `:elem in :list`, `:elem in :list index :index`"
)

var (
	BUILTIN_INOXJS_ATTRIBUTE_NAMES = []string{CONDITIONAL_DISPLAY_ATTR_NAME, FOR_LOOP_ATTR_NAME}

	FOR_LOOP_ATTR_VALUE_PATTERN = //
	regexp.MustCompile(`^(:[a-zA-Z_][_a-zA-Z0-9]*)\s+in\s+([$:@]?[a-zA-Z_][-_a-zA-Z0-9]*?)(?:\s*|\s+index\s+(:[a-zA-Z_][_a-zA-Z0-9]*))$`)
)

func AnalyzeInoxJsAttributes(
	ctx context.Context,
	element *parse.MarkupElement,
	sourcedChunk *parse.ParsedChunkSource,
) (
	isComponent bool,
	introducedElementScopedVarNames []string,
	errors []Error,
	criticalErr error,
) {

	for _, attr := range element.Opening.Attributes {
		markupAttr, ok := attr.(*parse.MarkupAttribute)
		if !ok {
			continue
		}
		ident, ok := markupAttr.Name.(*parse.IdentifierLiteral)
		if !ok ||
			!slices.Contains(BUILTIN_INOXJS_ATTRIBUTE_NAMES, ident.Name) ||
			markupAttr.Value == nil {
			continue
		}

		attrName := ident.Name

		var attrValue string
		attrValueSpan := markupAttr.Value.Base().Span

		switch val := markupAttr.Value.(type) {
		case *parse.DoubleQuotedStringLiteral:
			attrValue = val.Value
		case *parse.MultilineStringLiteral:
			attrValue = val.Value
		default:
			location := sourcedChunk.GetSourcePosition(val.Base().Span)
			errors = append(errors, MakeError(INOXJS_ATTRS_SHOULD_HAVE_A_STRING_LIT_AS_VALUE, location))
			continue
		}

		switch ident.Name {
		case FOR_LOOP_ATTR_NAME:
			isComponent = true

			trimmedValue := strings.TrimSpace(attrValue)
			matchGroups := FOR_LOOP_ATTR_VALUE_PATTERN.FindAllStringSubmatch(trimmedValue, -1)

			if matchGroups == nil {
				errors = append(errors, MakeError(INVALID_VALUE_FOR_FOR_LOOP_ATTR, sourcedChunk.GetSourcePosition(attrValueSpan)))
				break
			}

			introducedElementScopedVarNames = append(introducedElementScopedVarNames, matchGroups[0][1])
		case CONDITIONAL_DISPLAY_ATTR_NAME:
			_, parsingErr, err := hsparse.ParseHyperScriptExpression(ctx, attrValue)
			if err != nil {
				criticalErr = fmt.Errorf("critical error while parsing Hyperscript in attribute %s: %w", attrName, err)
				return
			}

			if parsingErr != nil {
				codeStartIndex := attrValueSpan.Start + (1 /* " or ` */) + parsingErr.Token.Start
				codeEndIndex := attrValueSpan.Start + (1 /* " or ` */) + parsingErr.Token.End

				inoxjsError := MakeError(
					parsingErr.Message,
					//location
					sourcedChunk.GetSourcePosition(parse.NodeSpan{
						Start: codeStartIndex,
						End:   codeEndIndex,
					}),
				)
				inoxjsError.IsHyperscriptParsingError = true
				errors = append(errors, inoxjsError)
			}
		}
	}

	return
}
