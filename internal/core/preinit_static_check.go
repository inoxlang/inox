package core

import (
	"fmt"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/parse"
)

type preinitBlockCheckParams struct {
	node    *parse.PreinitStatement
	fls     afs.Filesystem
	onError func(n parse.Node, msg string)
	module  *Module
}

func checkPreinitBlock(args preinitBlockCheckParams) {
	parse.Walk(args.node.Block, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		case *parse.Block, *parse.IdentifierLiteral, *parse.Variable,
			parse.SimpleValueLiteral, *parse.URLExpression,
			*parse.IntegerRangeLiteral, *parse.FloatRangeLiteral,

			//patterns
			*parse.PatternDefinition, *parse.PatternIdentifierLiteral,
			*parse.PatternNamespaceDefinition, *parse.PatternConversionExpression,
			*parse.ComplexStringPatternPiece, *parse.PatternPieceElement,
			*parse.ObjectPatternLiteral, *parse.RecordPatternLiteral, *parse.ObjectPatternProperty,
			*parse.PatternCallExpression, *parse.PatternGroupName,
			*parse.PatternUnion, *parse.ListPatternLiteral, *parse.TuplePatternLiteral:

			//ok
		case *parse.InclusionImportStatement:
			includedChunk := args.module.InclusionStatementMap[n]

			checkPatternOnlyIncludedChunk(includedChunk.Node, args.onError)
		default:
			args.onError(n, fmt.Sprintf("%s: %T", staticcheck.ErrForbiddenNodeinPreinit, n))
			return parse.Prune, nil
		}

		return parse.ContinueTraversal, nil
	}, nil)
}

func checkPatternOnlyIncludedChunk(chunk *parse.Chunk, onError func(n parse.Node, msg string)) {
	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

		if node == chunk {
			return parse.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case *parse.IncludableChunkDescription,
			parse.SimpleValueLiteral, *parse.URLExpression,
			*parse.IntegerRangeLiteral, *parse.FloatRangeLiteral,

			//patterns
			*parse.PatternDefinition, *parse.PatternIdentifierLiteral,
			*parse.PatternNamespaceDefinition, *parse.PatternConversionExpression,
			*parse.ComplexStringPatternPiece, *parse.PatternPieceElement,
			*parse.ObjectPatternLiteral, *parse.RecordPatternLiteral, *parse.ObjectPatternProperty,
			*parse.PatternCallExpression, *parse.PatternGroupName,
			*parse.PatternUnion, *parse.ListPatternLiteral, *parse.TuplePatternLiteral:
		default:
			onError(n, fmt.Sprintf("%s: %T", text.FORBIDDEN_NODE_TYPE_IN_INCLUDABLE_CHUNK_IMPORTED_BY_PREINIT, n))
			return parse.Prune, nil
		}

		return parse.ContinueTraversal, nil
	}, nil)
}
