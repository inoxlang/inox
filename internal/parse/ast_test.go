package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalk(t *testing.T) {

	t.Run("prune", func(t *testing.T) {
		chunk := MustParseChunk("1")
		Walk(chunk, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
			switch node.(type) {
			case *Chunk:
				return Prune, nil
			default:
				t.Fatal("the traversal should get pruned on the Module")
			}
			return ContinueTraversal, nil
		}, nil)
	})

	t.Run("stop", func(t *testing.T) {
		chunk := MustParseChunk("1 2")
		Walk(chunk, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
			switch n := node.(type) {
			case *IntLiteral:
				if n.Value == 2 {
					t.Fatal("the traversal should have stopped")
				}
				return StopTraversal, nil
			}
			return ContinueTraversal, nil
		}, nil)
	})

	t.Run("traversal", func(t *testing.T) {
		chunk := MustParseChunk("import lib /lib.ix {}")
		callCount := 0

		err := Walk(chunk, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
			switch callCount {
			case 0:
				assert.IsType(t, (*Chunk)(nil), node)
				assert.Equal(t, []Node{nil}, ancestorChain)
			case 1:
				assert.IsType(t, (*ImportStatement)(nil), node)
				assert.Equal(t, []Node{nil, chunk}, ancestorChain)
			case 2:
				assert.IsType(t, (*IdentifierLiteral)(nil), node)
				assert.Equal(t, []Node{nil, chunk, chunk.Statements[0]}, ancestorChain)
			case 3:
				assert.IsType(t, (*AbsolutePathLiteral)(nil), node)
				assert.Equal(t, []Node{nil, chunk, chunk.Statements[0]}, ancestorChain)
			case 4:
				assert.IsType(t, (*ObjectLiteral)(nil), node)
				assert.Equal(t, []Node{nil, chunk, chunk.Statements[0]}, ancestorChain)
			}

			callCount++
			return ContinueTraversal, nil
		}, nil)

		assert.NoError(t, err)
	})

}

func TestShiftNodeSpans(t *testing.T) {

	node := &Chunk{
		NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
		Statements: []Node{
			&IntLiteral{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
			},
		},
	}

	shiftNodeSpans(node, +2)
	assert.EqualValues(t, &Chunk{
		NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
		Statements: []Node{
			&IntLiteral{
				NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
			},
		},
	}, node)

}

func TestFindNode(t *testing.T) {
	chunk := MustParseChunk(`
		fn(arg %int){

		}
	`)

	node := FindNode(chunk, (*PatternIdentifierLiteral)(nil), nil)
	if !assert.NotNil(t, node) {
		return
	}
	assert.Equal(t, "int", node.Name)
}

func TestFindPreviousStatement(t *testing.T) {

	t.Run("previous statement for second statement in top level", func(t *testing.T) {
		chunk := MustParseChunk(`
			1
			2
		`)

		node, chain := FindNodeAndChain(chunk, (*IntLiteral)(nil), func(number *IntLiteral, unique bool) bool {
			return number.Value == 2
		})

		stmt, ok := FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*IntLiteral).Value)
	})

	t.Run("previous statement for first statement in top level", func(t *testing.T) {
		chunk := MustParseChunk(`
			1
		`)

		node, chain := FindNodeAndChain(chunk, (*IntLiteral)(nil), nil)

		stmt, ok := FindPreviousStatement(node, chain)
		assert.False(t, ok)
		assert.Nil(t, stmt)
	})

	t.Run("previous statement for second statement in block", func(t *testing.T) {
		chunk := MustParseChunk(`
			if true {
				1
				2
			}
		`)

		node, chain := FindNodeAndChain(chunk, (*IntLiteral)(nil), func(number *IntLiteral, unique bool) bool {
			return number.Value == 2
		})

		stmt, ok := FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*IntLiteral).Value)
	})

	t.Run("previous statement for first statement in block", func(t *testing.T) {
		chunk := MustParseChunk(`
			1
			if true {
				2
			}
		`)

		node, chain := FindNodeAndChain(chunk, (*IntLiteral)(nil), func(number *IntLiteral, unique bool) bool {
			return number.Value == 2
		})

		stmt, ok := FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*IntLiteral).Value)
	})

	t.Run("previous statement for second statement in top level of embedded module", func(t *testing.T) {
		chunk := MustParseChunk(`
			go {} do {
				1
				2
			}
		`)

		node, chain := FindNodeAndChain(chunk, (*IntLiteral)(nil), func(number *IntLiteral, unique bool) bool {
			return number.Value == 2
		})

		stmt, ok := FindPreviousStatement(node, chain)
		if !assert.True(t, ok) {
			return
		}

		assert.IsType(t, (*IntLiteral)(nil), stmt)

		assert.Equal(t, int64(1), stmt.(*IntLiteral).Value)
	})

	t.Run("previous statement for first statement in top level of embedded module", func(t *testing.T) {
		chunk := MustParseChunk(`
			1
			go {} do {
				2
			}
		`)

		node, chain := FindNodeAndChain(chunk, (*IntLiteral)(nil), nil)

		stmt, ok := FindPreviousStatement(node, chain)
		assert.False(t, ok)
		assert.Nil(t, stmt)
	})

}
