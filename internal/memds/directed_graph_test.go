package memds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectedGraph(t *testing.T) {

	t.Run("AddNode", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id := g.AddNode(3)
			assert.Equal(t, NodeId(0), id)

			//check that node0 has been created
			node, ok := g.Node(id)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 3, node.Data)
			assert.Equal(t, id, node.Id)

			data, ok := g.NodeData(id)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 3, data)

			//other checks
			assert.Zero(t, g.EdgeCount())
			assert.Empty(t, g.Edges())

			assert.Equal(t, 1, g.NodeCount())
			assert.Equal(t, []NodeId{id}, g.NodeIds())
		})

		t.Run("twice", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			id1 := g.AddNode(4)

			assert.Equal(t, NodeId(1), id1)
			assert.Equal(t, NodeId(0), id0)

			//check that node0 is still present

			node0, ok := g.Node(id0)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 3, node0.Data)
			assert.Equal(t, id0, node0.Id)

			data, ok := g.NodeData(id0)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 3, data)

			//check that node1 has been created
			node1, ok := g.Node(id1)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 4, node1.Data)
			assert.Equal(t, id1, node1.Id)

			data, ok = g.NodeData(id1)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 4, data)

			//other checks
			assert.Zero(t, g.EdgeCount())
			assert.Empty(t, g.Edges())

			assert.Equal(t, 2, g.NodeCount())
			assert.ElementsMatch(t, []NodeId{id0, id1}, g.NodeIds())
		})

		t.Run("twice with in-between removal", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			g.RemoveNode(id0)
			id1 := g.AddNode(4)

			assert.Equal(t, NodeId(0), id1)
			assert.Equal(t, id1, id0)

			//check that node1 has been created
			node1, ok := g.Node(id0)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 4, node1.Data)
			assert.Equal(t, id1, node1.Id)

			data, ok := g.NodeData(id0)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 4, data)

			//other checks
			assert.Zero(t, g.EdgeCount())
			assert.Empty(t, g.Edges())

			assert.Equal(t, 1, g.NodeCount())
			assert.Equal(t, []NodeId{id0}, g.NodeIds())
		})
	})

	t.Run("AddEdge", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			id1 := g.AddNode(4)

			g.SetEdge(id0, id1, 7)

			//check that the edge has been created
			if !assert.True(t, g.HasEdgeBetween(id0, id1)) {
				return
			}

			assert.Equal(t, int64(1), g.EdgeCount())
			assert.Equal(t, []GraphEdge[int]{
				{
					From: id0,
					To:   id1,
					Data: 3 + 4,
				},
			}, g.Edges())

			//check destination nodes
			destNodes := g.DestinationNodes(id0)
			if !assert.Len(t, destNodes, 1) {
				return
			}
			assert.Empty(t, g.DestinationNodes(id1))

			destNode := destNodes[0]
			assert.Equal(t, GraphNode[int]{
				Id:   id1,
				Data: 4,
			}, destNode)

			//check source nodes
			srcNodes := g.SourceNodes(id1)
			if !assert.Len(t, srcNodes, 1) {
				return
			}
			assert.Empty(t, g.SourceNodes(id0))

			srcNode := srcNodes[0]
			assert.Equal(t, GraphNode[int]{
				Id:   id0,
				Data: 3,
			}, srcNode)
		})

		t.Run("set edge twice but with different data", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			id1 := g.AddNode(4)

			g.SetEdge(id0, id1, 7)
			g.SetEdge(id0, id1, 8)

			//check that the edge has been created
			if !assert.True(t, g.HasEdgeBetween(id0, id1)) {
				return
			}

			assert.Equal(t, int64(1), g.EdgeCount())
			assert.Equal(t, []GraphEdge[int]{
				{
					From: id0,
					To:   id1,
					Data: 8,
				},
			}, g.Edges())

			//check destination nodes
			destNodes := g.DestinationNodes(id0)
			if !assert.Len(t, destNodes, 1) {
				return
			}
			assert.Empty(t, g.DestinationNodes(id1))

			destNode := destNodes[0]
			assert.Equal(t, GraphNode[int]{
				Id:   id1,
				Data: 4,
			}, destNode)

			//check source nodes
			srcNodes := g.SourceNodes(id1)
			if !assert.Len(t, srcNodes, 1) {
				return
			}
			assert.Empty(t, g.SourceNodes(id0))

			srcNode := srcNodes[0]
			assert.Equal(t, GraphNode[int]{
				Id:   id0,
				Data: 3,
			}, srcNode)
		})

		t.Run("inexisting source node", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			id1 := g.AddNode(4)

			g.RemoveNode(id0)

			assert.PanicsWithError(t, ErrSrcNodeNotExist.Error(), func() {
				g.SetEdge(id0, id1, 7)
			})

			//check that the edge is not present
			if !assert.False(t, g.HasEdgeBetween(id0, id1)) {
				return
			}

			assert.Zero(t, g.EdgeCount())
			assert.Empty(t, g.Edges())

			assert.Empty(t, g.DestinationNodes(id0))
			assert.Empty(t, g.SourceNodes(id1))
		})

		t.Run("inexisting destination node", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			id1 := g.AddNode(4)

			g.RemoveNode(id1)

			assert.PanicsWithError(t, ErrDestNodeNotExist.Error(), func() {
				g.SetEdge(id0, id1, 7)
			})

			//check that the edge is not present
			if !assert.False(t, g.HasEdgeBetween(id0, id1)) {
				return
			}

			assert.Zero(t, g.EdgeCount())
			assert.Empty(t, g.Edges())

			assert.Empty(t, g.DestinationNodes(id0))
			assert.Empty(t, g.SourceNodes(id1))
		})
	})

	t.Run("RemoveEdge", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			g := NewDirectedGraph[int, int](ThreadUnsafe)
			id0 := g.AddNode(3)
			id1 := g.AddNode(4)

			g.SetEdge(id0, id1, 7)
			g.RemoveEdge(id0, id1)

			//check that the edge is not present
			if !assert.False(t, g.HasEdgeBetween(id0, id1)) {
				return
			}

			assert.Zero(t, g.EdgeCount())
			assert.Empty(t, g.Edges())

			assert.Empty(t, g.DestinationNodes(id0))
			assert.Empty(t, g.SourceNodes(id1))
		})

	})
}
