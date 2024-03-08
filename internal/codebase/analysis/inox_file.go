package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
)

func analyzeInoxFile(path string, chunk *parse.Chunk, result *Result) {

	var state inoxFileAnalysisState

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		switch node := node.(type) {
		case *parse.XMLAttribute:
			analyzeXmlAttribute(node, &state, result)
		case *parse.HyperscriptAttributeShorthand:
			analyzeHyperscriptAtributeShortand(node, &state, result)
		case *parse.XMLElement:
			analyzeXmlElement(node, &state, result)
		case *parse.XMLText:
			if strings.Contains(node.Value, inoxjs.TEXT_INTERPOLATION_OPENING_DELIMITER) {
				result.IsInoxComponentLibUsed = true
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)

}

type inoxFileAnalysisState struct {
}
