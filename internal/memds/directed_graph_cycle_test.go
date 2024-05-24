package memds

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDirectedGraphHasCycle(t *testing.T) {

	for i := 0; i < 10; i++ {
		if !t.Run("empty graph", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("single node, no edges", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			g.AddNode("A")
			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("two nodes, no edges", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			g.AddNode("A")
			g.AddNode("B")

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("two nodes, A -> B", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			g.SetEdge(A, B, -1)

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("two nodes, A -> B -> A", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			g.SetEdge(A, B, -1)
			g.SetEdge(B, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, no edges", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			g.AddNode("A")
			g.AddNode("B")
			g.AddNode("C")

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> B -> C", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(B, C, -1)

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> B -> C -> A (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(C, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> B -> C -> B (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(C, B, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> (B & C)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> (B & C), B -> C", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(A, C, -1)

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> (B & C), B -> C, C -> A (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(C, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> (B & C), C -> A (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(C, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> (B & C), B -> C, C -> B (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(C, B, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("three nodes, A -> (B & C), B -> C, C -> (B & A) (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(C, B, -1)
			g.SetEdge(C, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("four nodes, A -> (B & C), B -> C, C -> D", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			D := g.AddNode("D")

			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(B, C, -1)
			g.SetEdge(C, D, -1)

			assert.False(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("four nodes, A -> (B & C), B -> D, C -> D, D -> A (cycle)", func(t *testing.T) {
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

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("four nodes, A -> (B & C), C -> D, D -> A (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadUnsafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			D := g.AddNode("D")

			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(C, D, -1)
			g.SetEdge(D, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

		if !t.Run("[thread safe] four nodes, A -> (B & C), C -> D, D -> A (cycle)", func(t *testing.T) {
			g := NewDirectedGraph[string, int](ThreadSafe)
			A := g.AddNode("A")
			B := g.AddNode("B")
			C := g.AddNode("C")
			D := g.AddNode("D")

			g.SetEdge(A, B, -1)
			g.SetEdge(A, C, -1)
			g.SetEdge(C, D, -1)
			g.SetEdge(D, A, -1)

			assert.True(t, g.HasCycle())
		}) {
			return
		}

	}
}

func TestBigDirectedGraphHasCycle(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	const N = 1_000
	src := rand.NewSource(58649401344244)
	random := rand.New(src)

	//Create a graph with no cycles so that HasCycleOrCircuit() does no return early.

	g := NewDirectedGraph[string, int](ThreadUnsafe)

	//Add N nodes.
	for i := 0; i < N; i++ {
		g.AddNode("")
	}

	//Create N random edges.
	for i := 0; i < N; i++ {

		for {
			srcNodeID := NodeId(-1)
			destNodeID := NodeId(-1)

			//Find two random nodes that are distinct.
			for srcNodeID == destNodeID || g.HasEdgeBetween(srcNodeID, destNodeID) {
				srcNodeID = NodeId(random.Int63n(N))
				destNodeID = NodeId(random.Int63n(N))
			}

			g.SetEdge(srcNodeID, destNodeID, 0)

			if g.HasCycle() {
				//Roll back
				g.RemoveEdge(srcNodeID, destNodeID)
				continue
			}

			//There is no cycle, we can generate the next edge.
			break
		}
	}

	start := time.Now()

	if g.HasCycle() {
		t.FailNow()
	}

	fmt.Println(time.Since(start))
}
