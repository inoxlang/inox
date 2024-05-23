package ast_test

import (
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestWalk(t *testing.T) {

	t.Run("prune", func(t *testing.T) {
		chunk := parse.MustParseChunk("1")
		ast.Walk(chunk, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) (ast.TraversalAction, error) {
			switch node.(type) {
			case *ast.Chunk:
				return ast.Prune, nil
			default:
				t.Fatal("the traversal should get pruned on the Module")
			}
			return ast.ContinueTraversal, nil
		}, nil)
	})

	t.Run("stop", func(t *testing.T) {
		chunk := parse.MustParseChunk("1 2")
		ast.Walk(chunk, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) (ast.TraversalAction, error) {
			switch n := node.(type) {
			case *ast.IntLiteral:
				if n.Value == 2 {
					t.Fatal("the traversal should have stopped")
				}
				return ast.StopTraversal, nil
			}
			return ast.ContinueTraversal, nil
		}, nil)
	})

	t.Run("traversal", func(t *testing.T) {
		chunk := parse.MustParseChunk("import lib /lib.ix {}")
		callCount := 0

		err := ast.Walk(chunk, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) (ast.TraversalAction, error) {
			switch callCount {
			case 0:
				assert.IsType(t, (*ast.Chunk)(nil), node)
				assert.Equal(t, []ast.Node{nil}, ancestorChain)
			case 1:
				assert.IsType(t, (*ast.ImportStatement)(nil), node)
				assert.Equal(t, []ast.Node{nil, chunk}, ancestorChain)
			case 2:
				assert.IsType(t, (*ast.IdentifierLiteral)(nil), node)
				assert.Equal(t, []ast.Node{nil, chunk, chunk.Statements[0]}, ancestorChain)
			case 3:
				assert.IsType(t, (*ast.AbsolutePathLiteral)(nil), node)
				assert.Equal(t, []ast.Node{nil, chunk, chunk.Statements[0]}, ancestorChain)
			case 4:
				assert.IsType(t, (*ast.ObjectLiteral)(nil), node)
				assert.Equal(t, []ast.Node{nil, chunk, chunk.Statements[0]}, ancestorChain)
			}

			callCount++
			return ast.ContinueTraversal, nil
		}, nil)

		assert.NoError(t, err)
	})

}

func TestShiftNodeSpans(t *testing.T) {

	node := &ast.Chunk{
		NodeBase: ast.NodeBase{NodeSpan{Start: 0, End: 2}, nil, false},
		Statements: []ast.Node{
			&ast.IntLiteral{
				NodeBase: ast.NodeBase{NodeSpan{Start: 0, End: 1}, nil, false},
			},
		},
	}

	ast.ShiftNodeSpans(node, +2)
	assert.EqualValues(t, &ast.Chunk{
		NodeBase: ast.NodeBase{NodeSpan{2, 4}, nil, false},
		Statements: []ast.Node{
			&ast.IntLiteral{
				NodeBase: ast.NodeBase{NodeSpan{2, 3}, nil, false},
			},
		},
	}, node)

}

func TestFindNode(t *testing.T) {
	chunk := parse.MustParseChunk(`
		fn(arg %int){

		}
	`)

	node := ast.FindNode(chunk, (*ast.PatternIdentifierLiteral)(nil), nil)
	if !assert.NotNil(t, node) {
		return
	}
	assert.Equal(t, "int", node.Name)
}

func TestFindPreviousStatement(t *testing.T) {

	t.Run("previous statement for second statement in top level", func(t *testing.T) {
		chunk := parse.MustParseChunk(`
			1
			2
		`)

		node, chain := ast.FindNodeAndChain(chunk, (*ast.IntLiteral)(nil), func(number *ast.IntLiteral, isFirstFound bool, _ []ast.Node) bool {
			return number.Value == 2
		})

		stmt, ok := ast.FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*ast.IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*ast.IntLiteral).Value)
	})

	t.Run("previous statement for first statement in top level", func(t *testing.T) {
		chunk := parse.MustParseChunk(`
			1
		`)

		node, chain := ast.FindNodeAndChain(chunk, (*ast.IntLiteral)(nil), nil)

		stmt, ok := ast.FindPreviousStatement(node, chain)
		assert.False(t, ok)
		assert.Nil(t, stmt)
	})

	t.Run("previous statement for second statement in block", func(t *testing.T) {
		chunk := parse.MustParseChunk(`
			if true {
				1
				2
			}
		`)

		node, chain := ast.FindNodeAndChain(chunk, (*ast.IntLiteral)(nil), func(number *ast.IntLiteral, unique bool, _ []ast.Node) bool {
			return number.Value == 2
		})

		stmt, ok := ast.FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*ast.IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*ast.IntLiteral).Value)
	})

	t.Run("previous statement for first statement in block", func(t *testing.T) {
		chunk := parse.MustParseChunk(`
			1
			if true {
				2
			}
		`)

		node, chain := ast.FindNodeAndChain(chunk, (*ast.IntLiteral)(nil), func(number *ast.IntLiteral, unique bool, _ []ast.Node) bool {
			return number.Value == 2
		})

		stmt, ok := ast.FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*ast.IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*ast.IntLiteral).Value)
	})

	t.Run("previous statement for second statement in top level of embedded module", func(t *testing.T) {
		chunk := parse.MustParseChunk(`
			go {} do {
				1
				2
			}
		`)

		node, chain := ast.FindNodeAndChain(chunk, (*ast.IntLiteral)(nil), func(number *ast.IntLiteral, unique bool, _ []ast.Node) bool {
			return number.Value == 2
		})

		stmt, ok := ast.FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*ast.IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*ast.IntLiteral).Value)
	})

	t.Run("previous statement for first statement in top level of embedded module", func(t *testing.T) {
		chunk := parse.MustParseChunk(`
			1
			go {} do {
				2
			}
		`)

		node, chain := ast.FindNodeAndChain(chunk, (*ast.IntLiteral)(nil), nil)

		stmt, ok := ast.FindPreviousStatement(node, chain)
		assert.False(t, ok)
		assert.Nil(t, stmt)
	})

}

func TestFindClosestMaxDistance(t *testing.T) {

	t.Run("a maximum distance of zero should be ignored", func(t *testing.T) {
		ancestors := []ast.Node{(*ast.Chunk)(nil), (*ast.Manifest)(nil), (*ast.ObjectLiteral)(nil)}
		node, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.ObjectLiteral)(nil), 0)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[2], node)
		assert.Equal(t, 2, index)

		manifest, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.Manifest)(nil), 0)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[1], manifest)
		assert.Equal(t, 1, index)

		chunk, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.Chunk)(nil), 0)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[0], chunk)
		assert.Equal(t, 0, index)
	})

	t.Run("maximum distance of one", func(t *testing.T) {
		ancestors := []ast.Node{(*ast.Chunk)(nil), (*ast.Manifest)(nil), (*ast.ObjectLiteral)(nil)}

		objLit, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.ObjectLiteral)(nil), 1)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[2], objLit)
		assert.Equal(t, 2, index)

		manifest, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.Manifest)(nil), 1)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[1], manifest)
		assert.Equal(t, 1, index)

		chunk, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.Chunk)(nil), 1)

		if !assert.False(t, ok) {
			return
		}
		assert.Nil(t, chunk)
		assert.EqualValues(t, -1, index)
	})

	t.Run("maximum distance of two", func(t *testing.T) {
		ancestors := []ast.Node{(*ast.Chunk)(nil), (*ast.Manifest)(nil), (*ast.ObjectLiteral)(nil), (*ast.ObjectProperty)(nil)}

		objProp, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.ObjectProperty)(nil), 2)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[3], objProp)
		assert.Equal(t, 3, index)

		objLit, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.ObjectLiteral)(nil), 2)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[2], objLit)
		assert.Equal(t, 2, index)

		manifest, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.Manifest)(nil), 2)

		if !assert.True(t, ok) {
			return
		}
		assert.Same(t, ancestors[1], manifest)
		assert.Equal(t, 1, index)

		chunk, index, ok := ast.FindClosestMaxDistance(ancestors, (*ast.Chunk)(nil), 2)

		if !assert.False(t, ok) {
			return
		}
		assert.Nil(t, chunk)
		assert.EqualValues(t, -1, index)
	})
}
