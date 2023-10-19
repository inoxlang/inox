package in_mem_ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectedGraphHasCycle(t *testing.T) {

	t.Run("empty graph", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("single node, no edges", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("two nodes, no edges", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		g.AddNode("B")

		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("two nodes, A -> B", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		g.SetEdge(A, B, -1)

		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("two nodes, A -> B -> A", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, A, -1)

		assert.True(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, no edges", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		g.AddNode("A")
		g.AddNode("B")
		g.AddNode("C")

		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, A -> B -> C", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)

		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, A -> B -> C -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, A, -1)

		assert.True(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, A -> B -> C -> B (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(C, B, -1)

		assert.True(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, A -> (B & C)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)

		assert.False(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, A -> (B & C), B -> C", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(B, C, -1)
		g.SetEdge(A, C, -1)

		assert.False(t, g.HasCycleOrCircuit())
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

		assert.True(t, g.HasCycleOrCircuit())
	})

	t.Run("three nodes, A -> (B & C), C -> A (cycle)", func(t *testing.T) {
		g := NewDirectedGraph[string, int](ThreadUnsafe)
		A := g.AddNode("A")
		B := g.AddNode("B")
		C := g.AddNode("C")
		g.SetEdge(A, B, -1)
		g.SetEdge(A, C, -1)
		g.SetEdge(C, A, -1)

		assert.True(t, g.HasCycleOrCircuit())
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

		assert.True(t, g.HasCycleOrCircuit())
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

		assert.True(t, g.HasCycleOrCircuit())
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

		assert.True(t, g.HasCycleOrCircuit())
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

		assert.True(t, g.HasCycleOrCircuit())
	})
}
