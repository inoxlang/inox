package project_server

import (
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"golang.org/x/exp/slices"
)

var ()

func getAutoEditForChange(documentText string, replacement string, rangeStart, rangeExclusiveEnd int32) (edit defines.TextEdit, hasEdit bool) {
	chunk, _ := parse.ParseChunkSource(parse.InMemorySource{
		NameString: "script",
		CodeString: string(documentText),
	})

	if chunk == nil {
		return
	}

	switch {
	case replacement == ">":
		node, _, found := chunk.GetNodeAndChainAtSpan(parse.NodeSpan{Start: rangeStart, End: rangeStart + 1})
		if !found {
			return
		}

		elem, ok := node.(*parse.XMLOpeningElement)
		if !ok {
			return
		}

		tagName, ok := elem.Name.(*parse.IdentifierLiteral)
		if !ok {
			return
		}

		//if void tag turn it into a self-closing tag since void tags are not supported.
		if slices.Contains(html_ns.VOID_HTML_TAG_NAMES, tagName.Name) {
			elemEnd := node.Base().Span.End
			span := parse.NodeSpan{Start: elemEnd - 1, End: elemEnd}
			line, col := chunk.GetSpanLineColumn(span)

			edit = defines.TextEdit{
				Range: rangeToLspRange(parse.SourcePositionRange{
					SourceName:  "", //not used
					Span:        span,
					StartLine:   line,
					StartColumn: col,
				}),
				NewText: "/>",
			}
			hasEdit = true
			return
		} else { //add closing tag
			elemEnd := node.Base().Span.End
			afterOpeningElem := parse.NodeSpan{Start: elemEnd, End: elemEnd}
			line, col := chunk.GetSpanLineColumn(afterOpeningElem)

			edit = defines.TextEdit{
				Range: rangeToLspRange(parse.SourcePositionRange{
					SourceName:  "", //not used
					Span:        afterOpeningElem,
					StartLine:   line,
					StartColumn: col,
				}),
				NewText: "</" + tagName.Name + ">",
			}
			hasEdit = true
			return
		}
	}

	hasEdit = false
	return
}
