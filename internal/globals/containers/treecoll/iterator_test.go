package treecoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestTreeIteration(t *testing.T) {

	t.Run("only root: nil children", func(t *testing.T) {
		tree := &Tree{}
		tree.root = &TreeNode{
			data: core.Int(0),
			tree: tree,
		}

		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		it := tree.Iterator(ctx, core.IteratorConfiguration{}).(*TreeIterator)

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))
		assert.Equal(t, tree.root, it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("only root: empty children", func(t *testing.T) {
		tree := &Tree{}
		tree.root = &TreeNode{
			data:     core.Int(0),
			tree:     tree,
			children: []*TreeNode{},
		}

		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		it := tree.Iterator(ctx, core.IteratorConfiguration{}).(*TreeIterator)

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))
		assert.Equal(t, tree.root, it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("root + leaf child", func(t *testing.T) {
		tree := &Tree{}
		tree.root = &TreeNode{
			data: core.Int(0),
			tree: tree,
			children: []*TreeNode{
				{
					data: core.Int(1),
					tree: tree,
				},
			},
		}

		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		it := tree.Iterator(ctx, core.IteratorConfiguration{}).(*TreeIterator)

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))
		assert.Equal(t, tree.root, it.Value(ctx))

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(1), it.Key(ctx))
		assert.Equal(t, (tree.root.children)[0], it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("complex", func(t *testing.T) {
		tree := &Tree{}
		tree.root = &TreeNode{
			data: core.Int(0),
			children: []*TreeNode{
				{
					data: core.Int(1),
					tree: tree,
				},
				{
					data: core.Int(2),
					tree: tree,
					children: []*TreeNode{
						{
							data: core.Int(3),
							tree: tree,
						},
					},
				},
			},
			tree: tree,
		}

		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		it := tree.Iterator(ctx, core.IteratorConfiguration{}).(*TreeIterator)

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))
		assert.Equal(t, tree.root, it.Value(ctx))

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(1), it.Key(ctx))
		assert.Equal(t, tree.root.children[0], it.Value(ctx))

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(2), it.Key(ctx))
		assert.Equal(t, tree.root.children[1], it.Value(ctx))

		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(3), it.Key(ctx))
		assert.Equal(t, tree.root.children[1].children[0], it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

}
