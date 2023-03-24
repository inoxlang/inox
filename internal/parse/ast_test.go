package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalk(t *testing.T) {

	t.Run("prune", func(t *testing.T) {
		chunk := MustParseChunk("1")
		Walk(chunk, func(node, parent, scopeNode Node, n4 []Node, _ bool) (TraversalAction, error) {
			switch node.(type) {
			case *Chunk:
				return Prune, nil
			default:
				t.Fatal("the traversal should get pruned on the Module")
			}
			return Continue, nil
		}, nil)
	})

	t.Run("stop", func(t *testing.T) {
		chunk := MustParseChunk("1 2")
		Walk(chunk, func(node, parent, scopeNode Node, n4 []Node, _ bool) (TraversalAction, error) {
			switch n := node.(type) {
			case *IntLiteral:
				if n.Value == 2 {
					t.Fatal("the traversal should have stopped")
				}
				return StopTraversal, nil
			}
			return Continue, nil
		}, nil)
	})
}

func TestShiftNodeSpans(t *testing.T) {

	node := &Chunk{
		NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
		Statements: []Node{
			&IntLiteral{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
			},
		},
	}

	shiftNodeSpans(node, +2)
	assert.EqualValues(t, &Chunk{
		NodeBase: NodeBase{NodeSpan{2, 4}, nil, nil},
		Statements: []Node{
			&IntLiteral{
				NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
			},
		},
	}, node)

}
