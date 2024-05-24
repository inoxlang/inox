package core

import (
	"fmt"

	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/text"
)

type preinitBlockCheckParams struct {
	node    *ast.PreinitStatement
	onError func(n ast.Node, msg string)
	module  *Module
}

func checkPreinitBlock(args preinitBlockCheckParams) {
	ast.Walk(args.node.Block, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {
		switch n := node.(type) {
		case *ast.Block, *ast.IdentifierLiteral, *ast.Variable,
			ast.SimpleValueLiteral, *ast.URLExpression,
			*ast.IntegerRangeLiteral, *ast.FloatRangeLiteral,

			//patterns
			*ast.PatternDefinition, *ast.PatternIdentifierLiteral,
			*ast.PatternNamespaceDefinition, *ast.PatternConversionExpression,
			*ast.ComplexStringPatternPiece, *ast.PatternPieceElement,
			*ast.ObjectPatternLiteral, *ast.RecordPatternLiteral, *ast.ObjectPatternProperty,
			*ast.PatternCallExpression, *ast.PatternGroupName,
			*ast.PatternUnion, *ast.ListPatternLiteral, *ast.TuplePatternLiteral:

			//ok
		case *ast.InclusionImportStatement:
			includedChunk := args.module.InclusionStatementMap[n]

			checkPatternOnlyIncludedChunk(includedChunk.Node, args.onError)
		default:
			args.onError(n, fmt.Sprintf("%s: %T", staticcheck.ErrForbiddenNodeinPreinit, n))
			return ast.Prune, nil
		}

		return ast.ContinueTraversal, nil
	}, nil)
}

func checkPatternOnlyIncludedChunk(chunk *ast.Chunk, onError func(n ast.Node, msg string)) {
	ast.Walk(chunk, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, after bool) (ast.TraversalAction, error) {

		if node == chunk {
			return ast.ContinueTraversal, nil
		}

		switch n := node.(type) {
		case *ast.IncludableChunkDescription,
			ast.SimpleValueLiteral, *ast.URLExpression,
			*ast.IntegerRangeLiteral, *ast.FloatRangeLiteral,

			//patterns
			*ast.PatternDefinition, *ast.PatternIdentifierLiteral,
			*ast.PatternNamespaceDefinition, *ast.PatternConversionExpression,
			*ast.ComplexStringPatternPiece, *ast.PatternPieceElement,
			*ast.ObjectPatternLiteral, *ast.RecordPatternLiteral, *ast.ObjectPatternProperty,
			*ast.PatternCallExpression, *ast.PatternGroupName,
			*ast.PatternUnion, *ast.ListPatternLiteral, *ast.TuplePatternLiteral:
		default:
			onError(n, fmt.Sprintf("%s: %T", text.FORBIDDEN_NODE_TYPE_IN_INCLUDABLE_CHUNK_IMPORTED_BY_PREINIT, n))
			return ast.Prune, nil
		}

		return ast.ContinueTraversal, nil
	}, nil)
}
