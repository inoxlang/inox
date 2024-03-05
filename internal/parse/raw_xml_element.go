package parse

import (
	"context"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
)

const (
	HYPERSCRIPT_PARSING_ERROR_PREFIX = "hyperscript: "
)

var (
	parseHyperscript ParseHyperscriptFn
)

type ParseHyperscriptFn func(ctx context.Context, input string) (*hscode.ParsingResult, *hscode.ParsingError, error)

func RegisterParseHypercript(fn ParseHyperscriptFn) {
	parseHyperscript = fn
}

func (p *parser) parseContentOfRawXMLElement(element *XMLElement) {

	ident, ok := element.Opening.Name.(*IdentifierLiteral)
	if !ok {
		return
	}

	if element.RawElementContent == "" {
		return
	}

	switch ident.Name {
	case "script":

		if element.EstimatedRawElementType == HyperscriptScript && p.parseHyperscript != nil {
			result, parsingErr, err := p.parseHyperscript(p.context, element.RawElementContent)
			if err != nil && element.Err == nil {
				//Only critical errors are reported in element.Err.
				element.Err = &ParsingError{UnspecifiedParsingError, HYPERSCRIPT_PARSING_ERROR_PREFIX + err.Error()}
			}
			if parsingErr != nil {
				element.RawElementParsingResult = parsingErr
			}
			if result != nil {
				element.RawElementParsingResult = result
			}
		}

	case "style":
	}
}
