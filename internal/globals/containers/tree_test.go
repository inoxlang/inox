package containers

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCreateTree(t *testing.T) {

	//TODO: add tests on trees with lifetimejobs

	//whitebox testing

	t.Run("root only", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		tree := NewTree(ctx, &core.UData{Root: core.Str("root")})

		assert.NotNil(t, tree.root)
		root := tree.root
		assert.Equal(t, core.Str("root"), root.data)
		assert.Empty(t, root.children)
	})

	t.Run("root with a single child", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		tree := NewTree(ctx, &core.UData{
			Root: core.Str("root"),
			HiearchyEntries: []core.UDataHiearchyEntry{
				{Value: core.Str("child")},
			},
		})

		assert.NotNil(t, tree.root)
		root := tree.root
		assert.Equal(t, core.Str("root"), root.data)
		assert.Len(t, root.children, 1)

		child := root.children[0]
		assert.Equal(t, core.Str("child"), child.data)
		assert.Empty(t, child.children)
	})

	t.Run("root with two leaves", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		tree := NewTree(ctx, &core.UData{
			Root: core.Str("root"),
			HiearchyEntries: []core.UDataHiearchyEntry{
				{Value: core.Str("child1")},
				{Value: core.Str("child2")},
			},
		})

		assert.NotNil(t, tree.root)
		root := tree.root
		assert.Equal(t, core.Str("root"), root.data)
		assert.Len(t, root.children, 2)

		child1 := root.children[0]
		assert.Equal(t, core.Str("child1"), child1.data)
		assert.Empty(t, child1.children)

		child2 := root.children[1]
		assert.Equal(t, core.Str("child2"), child2.data)
		assert.Empty(t, child2.children)
	})

	t.Run("root with a leaf + a non leaf child", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		tree := NewTree(ctx, &core.UData{
			Root: core.Str("root"),
			HiearchyEntries: []core.UDataHiearchyEntry{
				{Value: core.Str("child1")},
				{
					Value: core.Str("child2"),
					Children: []core.UDataHiearchyEntry{
						{Value: core.Str("leaf")},
					},
				},
			},
		})

		assert.NotNil(t, tree.root)
		root := tree.root
		assert.Equal(t, core.Str("root"), root.data)
		assert.Len(t, root.children, 2)

		child1 := root.children[0]
		assert.Equal(t, core.Str("child1"), child1.data)
		assert.Empty(t, child1.children)

		child2 := root.children[1]
		assert.Equal(t, core.Str("child2"), child2.data)
		assert.Len(t, child2.children, 1)

		leaf := child2.children[0]
		assert.Equal(t, core.Str("leaf"), leaf.data)
		assert.Empty(t, leaf.children)
	})

	t.Run("depth 2", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		tree := NewTree(ctx, &core.UData{
			Root: core.Str("root"),
			HiearchyEntries: []core.UDataHiearchyEntry{
				{Value: core.Str("child1")},
				{
					Value: core.Str("child2"),
					Children: []core.UDataHiearchyEntry{
						{Value: core.Str("leaf1")},
					},
				},
				{
					Value: core.Str("child3"),
					Children: []core.UDataHiearchyEntry{
						{
							Value: core.Str("grandchild1"),
							Children: []core.UDataHiearchyEntry{
								{Value: core.Str("leaf2")},
							},
						},
						{Value: core.Str("grandchild2")},
					},
				},
			},
		})

		assert.NotNil(t, tree.root)
		root := tree.root
		assert.Equal(t, core.Str("root"), root.data)
		assert.Len(t, root.children, 3)

		child1 := root.children[0]
		assert.Equal(t, core.Str("child1"), child1.data)
		assert.Empty(t, child1.children)

		child2 := root.children[1]
		{
			assert.Equal(t, core.Str("child2"), child2.data)
			assert.Len(t, child2.children, 1)

			leaf := child2.children[0]
			assert.Equal(t, core.Str("leaf1"), leaf.data)
			assert.Empty(t, leaf.children)
		}

		child3 := root.children[2]
		{
			assert.Equal(t, core.Str("child3"), child3.data)
			assert.Len(t, child3.children, 2)

			grandchild1 := child3.children[0]
			{
				assert.Equal(t, core.Str("grandchild1"), grandchild1.data)
				assert.Len(t, grandchild1.children, 1)

				leaf := grandchild1.children[0]
				assert.Equal(t, core.Str("leaf2"), leaf.data)
				assert.Empty(t, leaf.children)
			}

			grandchild2 := child3.children[1]
			assert.Equal(t, core.Str("grandchild2"), grandchild2.data)
			assert.Empty(t, grandchild2.children)
		}
	})

	t.Run("depth 3", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		tree := NewTree(ctx, &core.UData{
			Root: core.Str("root"),
			HiearchyEntries: []core.UDataHiearchyEntry{
				{Value: core.Str("child1")},
				{
					Value: core.Str("child2"),
					Children: []core.UDataHiearchyEntry{
						{Value: core.Str("leaf1")},
					},
				},
				{
					Value: core.Str("child3"),
					Children: []core.UDataHiearchyEntry{
						{
							Value: core.Str("grandchild1"),
							Children: []core.UDataHiearchyEntry{
								{Value: core.Str("leaf2")},
								{Value: core.Str("leaf3")},
								{
									Value: core.Str("greatgrandchild1"),
									Children: []core.UDataHiearchyEntry{
										{Value: core.Str("leaf4")},
										{Value: core.Str("leaf5")},
									},
								},
							},
						},
						{
							Value: core.Str("grandchild2"),
							Children: []core.UDataHiearchyEntry{
								{Value: core.Str("leaf6")},
								{Value: core.Str("leaf7")},
							},
						},
					},
				},
			},
		})

		assert.NotNil(t, tree.root)
		root := tree.root
		assert.Equal(t, core.Str("root"), root.data)
		assert.Len(t, root.children, 3)

		child1 := root.children[0]
		assert.Equal(t, core.Str("child1"), child1.data)
		assert.Empty(t, child1.children)

		child2 := root.children[1]
		{
			assert.Equal(t, core.Str("child2"), child2.data)
			assert.Len(t, child2.children, 1)

			leaf := child2.children[0]
			assert.Equal(t, core.Str("leaf1"), leaf.data)
			assert.Empty(t, leaf.children)
		}

		child3 := root.children[2]
		{
			assert.Equal(t, core.Str("child3"), child3.data)
			assert.Len(t, child3.children, 2)

			grandchild1 := child3.children[0]
			{
				assert.Equal(t, core.Str("grandchild1"), grandchild1.data)
				assert.Len(t, grandchild1.children, 3)

				leaf2 := grandchild1.children[0]
				assert.Equal(t, core.Str("leaf2"), leaf2.data)
				assert.Empty(t, leaf2.children)

				leaf3 := grandchild1.children[1]
				assert.Equal(t, core.Str("leaf3"), leaf3.data)
				assert.Empty(t, leaf3.children)

				greatgrandchild1 := grandchild1.children[2]
				{
					assert.Equal(t, core.Str("greatgrandchild1"), greatgrandchild1.data)
					assert.Len(t, greatgrandchild1.children, 2)

					leaf4 := greatgrandchild1.children[0]
					assert.Equal(t, core.Str("leaf4"), leaf4.data)
					assert.Empty(t, leaf4.children)

					leaf5 := greatgrandchild1.children[1]
					assert.Equal(t, core.Str("leaf5"), leaf5.data)
					assert.Empty(t, leaf5.children)
				}
			}

			grandchild2 := child3.children[1]
			{
				assert.Equal(t, core.Str("grandchild2"), grandchild2.data)
				assert.Len(t, grandchild2.children, 2)

				leaf6 := grandchild2.children[0]
				assert.Equal(t, core.Str("leaf6"), leaf6.data)
				assert.Empty(t, leaf6.children)

				leaf7 := grandchild2.children[1]
				assert.Equal(t, core.Str("leaf7"), leaf7.data)
				assert.Empty(t, leaf7.children)
			}
		}
	})
}

func TestTreeNode(t *testing.T) {

	//TODO: add tests on shared tree nodes

	ctx := core.NewContext(core.ContextConfig{})
	core.NewGlobalState(ctx)

	tree := NewTree(ctx, &core.UData{
		Root: core.Str("root"),
		HiearchyEntries: []core.UDataHiearchyEntry{
			{Value: core.Str("child1")},
			{
				Value: core.Str("child2"),
				Children: []core.UDataHiearchyEntry{
					{Value: core.Str("child3")},
				},
			},
		},
	})

	t.Run("iterator", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		it := tree.root.Iterator(ctx, core.IteratorConfiguration{})

		//first element should be root
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))
		root := it.Value(ctx)
		assert.Equal(t, tree.root, root)

		//second element: first root's child
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(1), it.Key(ctx))
		child1 := it.Value(ctx)
		assert.Equal(t, tree.root.children[0], child1)

		//third element: second root's child
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(2), it.Key(ctx))
		child2 := it.Value(ctx)
		assert.Equal(t, tree.root.children[1], child2)

		//fourth element: child of second root's child
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(3), it.Key(ctx))
		leaf := it.Value(ctx)
		assert.Equal(t, tree.root.children[1].children[0], leaf)

		assert.False(t, it.HasNext(ctx))
	})

	t.Run("children iterator", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		field := tree.root.Prop(ctx, "children")

		it := field.(core.Iterator)

		//first element
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))

		assert.Equal(t, core.Int(0), it.Key(ctx))
		child1 := it.Value(ctx)
		assert.Equal(t, tree.root.children[0], child1)

		//second element
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))

		assert.Equal(t, core.Int(1), it.Key(ctx))
		child2 := it.Value(ctx)
		assert.Equal(t, tree.root.children[1], child2)

		//
		assert.False(t, it.HasNext(ctx))
	})

}
