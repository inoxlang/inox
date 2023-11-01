package containers

import (
	"strconv"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
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

func TestSetIteration(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, nil, SetConfig{
			Element: core.ANYVAL_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.False(t, it.HasNext(ctx)) {
			return
		}

		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(1)), SetConfig{
			Element: core.ANYVAL_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(1), it.Value(ctx))
		assert.Equal(t, core.Str("1"), it.Key(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(1), core.Int(2)), SetConfig{
			Element: core.ANYVAL_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))

		if core.Int(1) == it.Value(ctx).(core.Int) {
			assert.Equal(t, core.Str("1"), it.Key(ctx))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(2), it.Value(ctx))
			assert.Equal(t, core.Str("2"), it.Key(ctx))
		} else {
			assert.Equal(t, core.Int(2), it.Value(ctx))
			assert.Equal(t, core.Str("2"), it.Key(ctx))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(1), it.Value(ctx))
			assert.Equal(t, core.Str("1"), it.Key(ctx))
		}

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))

	})

	t.Run("iteration in two goroutines", func(t *testing.T) {
		var elements []core.Serializable
		for i := 0; i < 100_000; i++ {
			elements = append(elements, core.Int(i))
		}
		tuple := core.NewTuple(elements)

		go func() {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			set := NewSetWithConfig(ctx, tuple, SetConfig{
				Element: core.ANYVAL_PATTERN,
				Uniqueness: containers_common.UniquenessConstraint{
					Type: containers_common.UniqueRepr,
				},
			})

			it := set.Iterator(ctx, core.IteratorConfiguration{})

			for it.HasNext(ctx) {
				it.Next(ctx)
				_ = it.Value(ctx)
			}
		}()

		time.Sleep(time.Microsecond)

		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, tuple, SetConfig{
			Element: core.ANYVAL_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})

		i := 0
		for it.HasNext(ctx) {
			assert.True(t, it.Next(ctx))
			val := core.Str(strconv.Itoa(int(it.Value(ctx).(core.Int))))
			assert.Equal(t, it.Key(ctx), val)
			i++
		}
		assert.Equal(t, 100_000, i)
	})

	t.Run("iteration as another goroutine modifies the Set", func(t *testing.T) {
		var elements []core.Serializable
		for i := 0; i < 100_000; i++ {
			elements = append(elements, core.Int(i))
		}
		tuple := core.NewTuple(elements)

		go func() {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			set := NewSetWithConfig(ctx, tuple, SetConfig{
				Element: core.ANYVAL_PATTERN,
				Uniqueness: containers_common.UniquenessConstraint{
					Type: containers_common.UniqueRepr,
				},
			})

			for i := 100_000; i < 1_000_000; i++ {
				set.Add(ctx, core.Int(i))
			}
		}()

		time.Sleep(time.Microsecond)

		for index := 0; index < 5; index++ {

			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			set := NewSetWithConfig(ctx, tuple, SetConfig{
				Element: core.ANYVAL_PATTERN,
				Uniqueness: containers_common.UniquenessConstraint{
					Type: containers_common.UniqueRepr,
				},
			})

			it := set.Iterator(ctx, core.IteratorConfiguration{})

			i := 0
			for it.HasNext(ctx) {
				if !assert.True(t, it.Next(ctx)) {
					return
				}
				val := core.Str(strconv.Itoa(int(it.Value(ctx).(core.Int))))
				if !assert.Equal(t, it.Key(ctx), val) {
					return
				}
				i++
			}
			if !assert.Equal(t, 100_000, i) {
				return
			}
		}
	})

}

func TestGraphIteration(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		graph := NewGraph(ctx, core.NewWrappedValueList(), core.NewWrappedValueList())

		it := graph.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.False(t, it.HasNext(ctx)) {
			return
		}

		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		graph := NewGraph(ctx, core.NewWrappedValueList(core.Str("1")), core.NewWrappedValueList())

		it := graph.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))

		v := it.Value(ctx)
		if !assert.IsType(t, (*GraphNode)(nil), v) {
			return
		}
		node := v.(*GraphNode)
		assert.Equal(t, &GraphNode{id: 0, graph: graph}, node)
		assert.Equal(t, core.Str("1"), node.Prop(ctx, "data"))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		graph := NewGraph(ctx, core.NewWrappedValueList(core.Str("1"), core.Str("2")), core.NewWrappedValueList())

		it := graph.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))

		v := it.Value(ctx)
		if !assert.IsType(t, (*GraphNode)(nil), v) {
			return
		}
		node := v.(*GraphNode)

		if node.id == 0 {
			assert.Equal(t, core.Str("1"), node.Prop(ctx, "data"))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(1), it.Key(ctx))

			v := it.Value(ctx)
			if !assert.IsType(t, (*GraphNode)(nil), v) {
				return
			}
			secondNode := v.(*GraphNode)
			assert.Equal(t, &GraphNode{id: 1, graph: graph}, secondNode)
			assert.Equal(t, core.Str("2"), secondNode.Prop(ctx, "data"))
		} else {
			assert.Equal(t, core.Str("2"), node.Prop(ctx, "data"))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(1), it.Key(ctx))

			v := it.Value(ctx)
			if !assert.IsType(t, (*GraphNode)(nil), v) {
				return
			}
			secondNode := v.(*GraphNode)
			assert.Equal(t, &GraphNode{id: 0, graph: graph}, secondNode)
			assert.Equal(t, core.Str("1"), secondNode.Prop(ctx, "data"))
		}

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("iteration in two goroutines", func(t *testing.T) {
		//TODO
	})

	t.Run("iteration as another goroutine modifies the Graph", func(t *testing.T) {
		//TODO
	})

}
