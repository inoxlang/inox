package parse

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeAtSpan(t *testing.T) {

	t.Run("shallow", func(t *testing.T) {
		t.Run("", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "a = 1\na\nfn f(){}",
			}))

			//identifier on first line
			span := chunk.GetLineColumnSingeCharSpan(1, 1)
			node, ok := chunk.GetNodeAtSpan(span)
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &IdentifierLiteral{}, node)

			//identifier on second line
			span = chunk.GetLineColumnSingeCharSpan(2, 1)
			node, ok = chunk.GetNodeAtSpan(span)
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &IdentifierLiteral{}, node)
		})

		t.Run("empty span within an identifier", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "aaa",
			}))

			node, ok := chunk.GetNodeAtSpan(NodeSpan{1, 1})
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &IdentifierLiteral{}, node)
		})

		t.Run("span starting at exclusive end of node", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "aaa ",
			}))

			node, ok := chunk.GetNodeAtSpan(NodeSpan{3, 4})
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &Chunk{}, node)
		})
	})

	t.Run("deep", func(t *testing.T) {
		t.Run("", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "fn f(arg %int){}",
			}))

			span := chunk.GetLineColumnSingeCharSpan(1, 10)
			node, ok := chunk.GetNodeAtSpan(span)
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &PatternIdentifierLiteral{}, node)

			span = chunk.GetLineColumnSingeCharSpan(1, 13)
			node, ok = chunk.GetNodeAtSpan(span)
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &PatternIdentifierLiteral{}, node)
		})

		t.Run("span starting at exclusive end of node", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "html<div></div>",
			}))

			// elements: ... (div)[identifier] (>)[token of opening elem] ()[xml text] ...
			// the found node should be the XML opening element.

			node, ok := chunk.GetNodeAtSpan(NodeSpan{8, 9}) //the 'div' identifier ends at 8.
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &XMLOpeningElement{}, node)
		})

		t.Run("empty span within an identifier", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "html<div></div>",
			}))

			node, ok := chunk.GetNodeAtSpan(NodeSpan{7, 7}) //the 'div' identifier ends at 8.
			if !assert.True(t, ok) {
				return
			}
			if !assert.IsType(t, &IdentifierLiteral{}, node) {
				return
			}

			assert.Equal(t, "div", node.(*IdentifierLiteral).Name)
		})

		t.Run("empty span at a node with an empty span", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "html<div></div>",
			}))

			node, ok := chunk.GetNodeAtSpan(NodeSpan{9, 9}) //the XML text is empty and its position is 9
			if !assert.True(t, ok) {
				return
			}
			assert.IsType(t, &XMLElement{}, node)
		})
	})

}

func TestFindFirstStatementAndChainOnLine(t *testing.T) {
	t.Run("empty chunk", func(t *testing.T) {
		chunk := utils.Must(ParseChunkSource(InMemorySource{
			NameString: "test",
			CodeString: "",
		}))

		node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
		if !assert.False(t, found) {
			return
		}

		assert.Nil(t, node)
		assert.Nil(t, ancestors)
	})

	t.Run("single line chunk", func(t *testing.T) {
		t.Run("space", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: " ",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})
		t.Run("comment", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "# comment",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("single statement", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "1",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("single statement preceded by a simple space", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: " 1",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("single multi-node statement", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "f()",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*CallExpression)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("two statements", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "1; 2",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.True(t, found) {
				return
			}

			expectedNodes, expectedAncestors := FindNodesAndChains(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNodes[0], node)
			assert.Equal(t, expectedAncestors[0], ancestors)
		})
	})

	t.Run("two-line chunk: first one is empty", func(t *testing.T) {
		t.Run("second line is empty: nothing should be found in first line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line is empty: nothing should be found in second line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line is a space: nothing should be found in first line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n ",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line is a space: nothing should be found in second line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n ",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line is a comment: nothing should be found in first line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n# comment",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line is a comment: nothing should be found in second line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n# comment",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line has a single statement: nothing should be found in first line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n1",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(1)

			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("second line has a single statement: node should be found in second line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n1",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("second line has a single statement preceded by a space: node should be found in second line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n 1",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("second line has two statements: node should be found in second line", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "\n1; 2",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.True(t, found) {
				return
			}

			expectedNodes, expectedAncestors := FindNodesAndChains(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNodes[0], node)
			assert.Equal(t, expectedAncestors[0], ancestors)
		})
	})

	t.Run("block", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "if true {\n}",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.False(t, found) {
				return
			}

			assert.Nil(t, node)
			assert.Nil(t, ancestors)
		})

		t.Run("single statement", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "if true {\n1}",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("single statement preceded by a space", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "if true {\n 1}",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.True(t, found) {
				return
			}

			expectedNode, expectedAncestors := FindNodeAndChain(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNode, node)
			assert.Equal(t, expectedAncestors, ancestors)
		})

		t.Run("two statements", func(t *testing.T) {
			chunk := utils.Must(ParseChunkSource(InMemorySource{
				NameString: "test",
				CodeString: "if true {\n1; 2}",
			}))

			node, ancestors, found := chunk.FindFirstStatementAndChainOnLine(2)
			if !assert.True(t, found) {
				return
			}

			expectedNodes, expectedAncestors := FindNodesAndChains(chunk.Node, (*IntLiteral)(nil), nil)

			assert.Equal(t, expectedNodes[0], node)
			assert.Equal(t, expectedAncestors[0], ancestors)
		})
	})
}
