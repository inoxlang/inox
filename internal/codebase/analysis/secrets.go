package analysis

import (
	"github.com/inoxlang/inox/internal/parse"
)

var (
	HTTP_HEADERS_WITH_SECRETS = []string{"x-api-key", "authorization"}
)

func findHarcodedSecretsInInoxFile(inoxChunk *parse.Chunk, result *Result) {

	parse.Walk(inoxChunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		switch node := node.(type) {
		case *parse.ObjectProperty:
			if node.HasImplicitKey() {
				break
			}
			//name := node.Name()
			//lowercaseName := strings.ToLower(name)

			//isHttpHeaderWithSecret := slices.Contains(HTTP_HEADERS_WITH_SECRETS, lowercaseName)

		}

		return parse.ContinueTraversal, nil
	}, nil)
}
