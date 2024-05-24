package memds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectedGraphUniqueStringData(t *testing.T) {

	t.Run("GetNode with data", func(t *testing.T) {
		g := NewDirectedGraphUniqueString[string, int](ThreadUnsafe)
		node, err := g.GetNode(WithData, "A")
		if !assert.ErrorIs(t, err, ErrNodeNotFound) {
			return
		}
		assert.Zero(t, node)

		id := g.AddNode("A")

		//check that we can get the added node
		node, err = g.GetNode(WithData, "A")
		assert.NoError(t, err)
		assert.Equal(t, GraphNode[string]{
			Id:   id,
			Data: "A",
		}, node)

		g.RemoveNode(node.Id)

		//check that we cannot longer get the node
		node, err = g.GetNode(WithData, "A")
		if !assert.ErrorIs(t, err, ErrNodeNotFound) {
			return
		}
		assert.Zero(t, node)
	})

	t.Run("HasNode with data", func(t *testing.T) {
		g := NewDirectedGraphUniqueString[string, int](ThreadUnsafe)
		node, err := g.GetNode(WithData, "A")
		if !assert.ErrorIs(t, err, ErrNodeNotFound) {
			return
		}
		assert.Zero(t, node)

		g.AddNode("A")

		//check that we can get the added node
		ok, err := g.HasNode(WithData, "A")
		if !assert.NoError(t, err) {
			return
		}
		assert.True(t, ok)
		g.RemoveNode(node.Id)

		//check that we cannot longer get the node
		ok, err = g.HasNode(WithData, "A")
		if !assert.NoError(t, err) {
			return
		}
		assert.False(t, ok)
	})

	t.Run("adding two nodes with the same data is forbidden", func(t *testing.T) {
		g := NewDirectedGraphUniqueString[string, int](ThreadUnsafe)
		node, err := g.GetNode(WithData, "A")
		if !assert.ErrorIs(t, err, ErrNodeNotFound) {
			return
		}
		assert.Zero(t, node)

		g.AddNode("A")
		g.AddNode("B")
		assert.PanicsWithError(t, ErrNodeSameStringDataAlreadyInGraph.Error(), func() {
			g.AddNode("B")
		})
	})

}
