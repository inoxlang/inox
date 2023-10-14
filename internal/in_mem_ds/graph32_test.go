package in_mem_ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGraph32(t *testing.T) {

	ok := t.Run("AddNode", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			var g Graph32[int]

			node0 := g.AddNode(2)
			assert.Equal(t, 2, node0.data)
			assert.Equal(t, NodeId(0), node0.Id())
			assert.Equal(t, 1, g.Size())
			assert.True(t, g.HasNodeOfId(node0.Id()))

			{

				node, ok := g.NodeOfId(node0.Id())
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, node0, node)
			}

			node1 := g.AddNode(3)
			assert.Equal(t, 3, node1.data)
			assert.Equal(t, NodeId(1), node1.Id())
			assert.Equal(t, 2, g.Size())
			assert.True(t, g.HasNodeOfId(node1.Id()))
			assert.True(t, g.HasNodeOfId(node0.Id()))

			{

				node, ok := g.NodeOfId(node1.Id())
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, node1, node)
			}

		})

		t.Run("three nodes", func(t *testing.T) {
			var g Graph32[int]

			g.AddNode(3)
			g.AddNode(4)
			g.AddNode(5)

			assert.Equal(t, 3, g.Size())
		})

		t.Run("full graph", func(t *testing.T) {
			var g Graph32[int]

			for i := 0; i < g.Capacity(); i++ {
				g.AddNode(i + 3)
			}

			assert.Equal(t, g.Capacity(), g.Size())
		})
	})

	if !ok {
		return
	}

	t.Run("AddEdge", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			var g Graph32[int]

			node0 := g.AddNode(2)
			node1 := g.AddNode(3)

			g.AddEdge(node0.Id(), node1.Id())

			if !assert.True(t, g.HasEdgeFromTo(node0.Id(), node1.Id())) {
				return
			}

			assert.False(t, g.HasEdgeFromTo(node1.Id(), node0.Id()))
		})

		t.Run("repeat", func(t *testing.T) {
			var g Graph32[int]

			node0 := g.AddNode(2)
			node1 := g.AddNode(3)

			g.AddEdge(node0.Id(), node1.Id())
			g.AddEdge(node0.Id(), node1.Id())

			if !assert.True(t, g.HasEdgeFromTo(node0.Id(), node1.Id())) {
				return
			}

			assert.False(t, g.HasEdgeFromTo(node1.Id(), node0.Id()))
		})

		t.Run("adding an edge pointing back to the origin node is allowed", func(t *testing.T) {
			var g Graph32[int]

			node0 := g.AddNode(2)

			g.AddEdge(node0.Id(), node0.Id())

			if !assert.True(t, g.HasEdgeFromTo(node0.Id(), node0.Id())) {
				return
			}

			//test iteration
			it := g.IteratorFrom(node0.Id())

			if !assert.True(t, it.Next()) {
				return
			}
			assert.Equal(t, node0, it.Node())
			assert.False(t, it.Next())

			//test iteration: directly reachable nodes
			it2 := g.IteratorDirectlyReachableNodes(node0.Id())
			if !assert.True(t, it2.Next()) {
				return
			}
			assert.Equal(t, node0, it2.Node())
			assert.False(t, it2.Next())
		})

		t.Run("A -> B -> C", func(t *testing.T) {
			var g Graph32[int]

			node0 := g.AddNode(2)
			node1 := g.AddNode(3)
			node2 := g.AddNode(4)

			g.AddEdge(node0.Id(), node1.Id())
			g.AddEdge(node1.Id(), node2.Id())

			if !assert.True(t, g.HasEdgeFromTo(node0.Id(), node1.Id())) {
				return
			}

			if !assert.True(t, g.HasEdgeFromTo(node1.Id(), node2.Id())) {
				return
			}

			assert.False(t, g.HasEdgeFromTo(node0.Id(), node2.Id()))

			//test iteration
			it := g.IteratorFrom(node1.Id())

			if !assert.True(t, it.Next()) {
				return
			}
			assert.Equal(t, node2, it.Node())
			assert.False(t, it.Next())

			//test iteration: directly reachable nodes
			it2 := g.IteratorDirectlyReachableNodes(node0.Id())

			if !assert.True(t, it2.Next()) {
				return
			}
			assert.Equal(t, node1, it2.Node())
			assert.False(t, it2.Next())
		})

		t.Run("A -> B -> C -> A", func(t *testing.T) {
			var g Graph32[int]

			node0 := g.AddNode(2)
			node1 := g.AddNode(3)
			node2 := g.AddNode(4)
			node3 := g.AddNode(5)

			g.AddEdge(node0.Id(), node1.Id())
			g.AddEdge(node1.Id(), node2.Id())
			g.AddEdge(node2.Id(), node3.Id())
			g.AddEdge(node3.Id(), node0.Id())

			if assert.True(t, g.HasEdgeFromTo(node0.Id(), node1.Id())) {
				return
			}

			if !assert.True(t, g.HasEdgeFromTo(node1.Id(), node2.Id())) {
				return
			}

			assert.True(t, g.HasEdgeFromTo(node0.Id(), node2.Id()))

			//test iteration
			it := g.IteratorFrom(node1.Id())

			if !assert.True(t, it.Next()) {
				return
			}
			assert.Equal(t, node2, it.Node())

			if !assert.True(t, it.Next()) {
				return
			}
			assert.Equal(t, node3, it.Node())

			if !assert.True(t, it.Next()) {
				return
			}
			assert.Equal(t, node2, it.Node())
			assert.False(t, it.Next())

			//test iteration: directly reachable nodes
			it2 := g.IteratorDirectlyReachableNodes(node0.Id())

			if !assert.True(t, it2.Next()) {
				return
			}
			assert.Equal(t, node1, it2.Node())
			assert.False(t, it2.Next())
		})
	})
}
