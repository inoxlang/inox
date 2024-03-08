package scan

import (
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/scan"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/parse"
)

type Configuration struct {
	TopDirectories []string
	MaxFileSize    int64 //defaults to codebasescan.DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast           bool  //if true the scan will be faster but will use more CPU and memory.
	InoxChunkCache *parse.ChunkCache
}

type ScanResult struct {
	RequiredDefinitions []hsgen.Definition
}

func ScanCodebase(ctx *core.Context, fls afs.Filesystem, config Configuration) (ScanResult, error) {

	var requiredDefinitions []hsgen.Definition

	handleFile := func(path, fileContent string, n *parse.Chunk) error {
		parse.Walk(n, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
			var tokens []hscode.Token

			switch node := node.(type) {
			case *parse.HyperscriptAttributeShorthand:
				if node.HyperscriptParsingResult != nil {
					tokens = append(tokens, node.HyperscriptParsingResult.Tokens...)
				} else if node.HyperscriptParsingError != nil {
					tokens = append(tokens, node.HyperscriptParsingError.Tokens...)
				}
			case *parse.XMLElement:
				if node.EstimatedRawElementType == parse.HyperscriptScript {
					result, ok := node.RawElementParsingResult.(*hscode.ParsingResult)
					if ok {
						tokens = append(tokens, result.Tokens...)
					} else if err, ok := node.RawElementParsingResult.(*hscode.ParsingError); ok {
						tokens = append(tokens, err.Tokens...)
					}
				}
			}

			for _, token := range tokens {
				if token.Type == hscode.IDENTIFIER {
					if hsgen.IsBuiltinFeatureName(token.Value) || hsgen.IsBuiltinCommandName(token.Value) {
						def, ok := hsgen.GetBuiltinDefinition(token.Value)
						if ok {
							requiredDefinitions = append(requiredDefinitions, def)
						}
					}
				}
			}

			return parse.ContinueTraversal, nil
		}, nil)

		return nil
	}

	err := scan.ScanCodebase(ctx, fls, scan.Configuration{
		TopDirectories:     []string{"/"},
		ChunkCache:         config.InoxChunkCache,
		FileHandlers:       []scan.FileHandler{handleFile},
		FileParsingTimeout: 50 * time.Millisecond,
	})

	if err != nil {
		return ScanResult{}, nil
	}

	return ScanResult{
		RequiredDefinitions: requiredDefinitions,
	}, nil
}
