package projectserver

import (
	"slices"

	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

// getAutoEditForChange determines a text edit to immediately apply based on a change made by the user.
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
			posRange := chunk.GetSourcePosition(span)

			edit = defines.TextEdit{
				Range:   rangeToLspRange(posRange),
				NewText: "/>",
			}
			hasEdit = true
			return
		} else { //add closing tag
			elemEnd := node.Base().Span.End
			afterOpeningElem := parse.NodeSpan{Start: elemEnd, End: elemEnd}
			posRange := chunk.GetSourcePosition(afterOpeningElem)

			edit = defines.TextEdit{
				Range:   rangeToLspRange(posRange),
				NewText: "</" + tagName.Name + ">",
			}
			hasEdit = true
			return
		}
	}

	hasEdit = false
	return
}
