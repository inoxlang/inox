package memds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectedGraphLongestPath(t *testing.T) {

	t.Run("empty graph", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		if !assert.Equal(t, 0, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 0, len)
		assert.Empty(t, 0, path)
	})

	t.Run("single node, no edges", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		if !assert.Equal(t, 0, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 0, len)
		assert.Empty(t, 0, path)
	})

	t.Run("two nodes, no edges", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		g.AddNode("B")

		if !assert.Equal(t, 0, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 0, len)
		assert.Empty(t, 0, path)
	})

	t.Run("two nodes, A -> B", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		g.SetEdge(A, B, -1)

		if !assert.Equal(t, 1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 1, len)
		assert.Equal(t, []NodeId{A, B}, path)
	})

	t.Run("two nodes, A -> B -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("two nodes, A and B, the C node was added and directly removed", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		g.AddNode("B")
		C := g.AddNode("C")
		g.RemoveNode(C)

		if !assert.Equal(t, 0, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 0, len)
		assert.Empty(t, path)
	})

	t.Run("two nodes, A -> C, the node B was removed and then node C was added", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.RemoveNode(B)

		g.SetEdge(A, C, -1)

		if !assert.Equal(t, 1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 1, len)
		assert.Equal(t, []NodeId{A, C}, path)
	})

	t.Run("three nodes, no edges", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		g.AddNode("B")
		g.AddNode("C")

		if !assert.Equal(t, 0, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 0, len)
		assert.Empty(t, path)
	})

	t.Run("three nodes, A -> B", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		_ = g.AddNode("C")
		g.SetEdge(A, B, -1)

		if !assert.Equal(t, 1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 1, len)
		assert.Equal(t, []NodeId{A, B}, path)
	})

	t.Run("three nodes, A -> C", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		_ = g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, C, -1)

		if !assert.Equal(t, 1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 1, len)
		assert.Equal(t, []NodeId{A, C}, path)
	})

	t.Run("three nodes, A -> B -> C", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)

		if !assert.Equal(t, 2, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 2, len)
		assert.Equal(t, []NodeId{A, B, C}, path)
	})

	t.Run("three nodes, A -> B -> C -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("three nodes, A -> B -> C -> B (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, B, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("three nodes, A -> (B & C)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)

		if !assert.Equal(t, 1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 1, len)
		if path[1] == B {
			assert.Equal(t, []NodeId{A, B}, path)
		} else {
			assert.Equal(t, []NodeId{A, C}, path)
		}
	})

	t.Run("three nodes, A -> (B & C), B -> C", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(A, C, -1)

		//A -> B -> C
		if !assert.Equal(t, 2, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 2, len)
		assert.Equal(t, []NodeId{A, B, C}, path)
	})

	t.Run("three nodes, A -> (B & C), B -> C, C -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("three nodes, A -> (B & C), C -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("three nodes, A -> (B & C), B -> C, C -> B (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, B, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("three nodes, A -> (B & C), B -> C, C -> (B & A) (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, B, -1)
		g.SetEdge(C, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("four nodes, A -> B -> C -> D", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")

		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, D, -1)

		if !assert.Equal(t, 3, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 3, len)
		assert.Equal(t, []NodeId{A, B, C, D}, path)
	})

	t.Run("four nodes, A -> B  C -> D", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")

		g.SetEdge(A, B, -1)
		g.SetEdge(C, D, -1)

		if !assert.Equal(t, 1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 1, len)
		if path[0] == A {
			assert.Equal(t, []NodeId{A, B}, path)
		} else {
			assert.Equal(t, []NodeId{C, D}, path)
		}
	})

	t.Run("four nodes, A -> B -> C", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		_ = g.AddNode("D")

		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)

		//A -> B -> C
		if !assert.Equal(t, 2, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 2, len)
		assert.Equal(t, []NodeId{A, B, C}, path)
	})

	t.Run("four nodes, A -> (B & C), B -> D, C -> D, D -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")

		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(B, D, -1)
		g.SetEdge(C, D, -1)
		g.SetEdge(D, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("four nodes, A -> (B & C), C -> D, D -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")

		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, D, -1)
		g.SetEdge(D, A, -1)

		if !assert.Equal(t, -1, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, -1, len)
		assert.Empty(t, path)
	})

	t.Run("five nodes, A -> (B & C), C -> D, B -> E", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")
		E := g.AddNode("E")

		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, D, -1)
		g.SetEdge(B, E, -1)

		//A -> C -> D or A -> B -> E
		if !assert.Equal(t, 2, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 2, len)
		if path[1] == B {
			assert.Equal(t, []NodeId{A, B, E}, path)
		} else {
			assert.Equal(t, []NodeId{A, C, D}, path)
		}
	})

	t.Run("six nodes, A -> (B & C), C -> D, D -> E, B -> F", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")
		E := g.AddNode("E")
		F := g.AddNode("F")

		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, D, -1)
		g.SetEdge(D, E, -1)
		g.SetEdge(B, F, -1)

		//A -> C -> D -> E
		if !assert.Equal(t, 3, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 3, len)
		assert.Equal(t, []NodeId{A, C, D, E}, path)
	})

	t.Run("[thread safe] six nodes, A -> (B & C), C -> D, D -> E, B -> F", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadSafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		D := g.AddNode("D")
		E := g.AddNode("E")
		F := g.AddNode("F")

		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, D, -1)
		g.SetEdge(D, E, -1)
		g.SetEdge(B, F, -1)

		//A -> C -> D -> E
		if !assert.Equal(t, 3, g.LongestPathLen()) {
			return
		}

		path, len := g.LongestPath()
		assert.Equal(t, 3, len)
		assert.Equal(t, []NodeId{A, C, D, E}, path)
	})
}
