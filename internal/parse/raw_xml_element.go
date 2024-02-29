package parse

import (
	"context"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mimeconsts"
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
		for _, attr := range element.Opening.Attributes {
			attr, ok := attr.(*XMLAttribute)
			if !ok {
				continue
			}
			ident, ok = attr.Name.(*IdentifierLiteral)
			if !ok {
				continue
			}

			isHyperscript := false

			//<script h> element
			if ident.Name == inoxconsts.HYPERSCRIPT_SCRIPT_MARKER {
				isHyperscript = true
			}

			//<script type="text/hyperscript">
			if ident.Name == "type" {
				strLit, ok := attr.Value.(*QuotedStringLiteral)
				isHyperscript = ok && strLit.Value == mimeconsts.HYPERSCRIPT_CTYPE
			}

			if isHyperscript && p.parseHyperscript != nil {
				result, parsingErr, err := p.parseHyperscript(p.context, element.RawElementContent)
				if err != nil && element.Err == nil {
					//Only critical errors oare reported in element.Err.
					element.Err = &ParsingError{UnspecifiedParsingError, err.Error()}
				}
				if parsingErr != nil {
					element.RawElementParsingResult = parsingErr
				}
				if result != nil {
					element.RawElementParsingResult = result
				}
			}
		}
	case "style":
	}
}
