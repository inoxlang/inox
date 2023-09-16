package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUdataWalker(t *testing.T) {

	t.Run("single node", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
		}

		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		assert.False(t, walker.Next(ctx))
	})

	t.Run("root + child", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(1),
				},
			},
		}

		//root
		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.False(t, walker.Next(ctx)) {
			return
		}
	})

	t.Run("root + two children", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(1),
				},
				{
					Value: Int(2),
				},
			},
		}

		//root
		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//child 1
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Key(ctx)) {
			return
		}

		//child 2
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.False(t, walker.Next(ctx)) {
			return
		}
	})

	t.Run("root + child + grand child", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(1),
					Children: []UDataHiearchyEntry{
						{
							Value: Int(2),
						},
					},
				},
			},
		}

		//root
		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Key(ctx)) {
			return
		}

		//grand child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.False(t, walker.Next(ctx)) {
			return
		}
	})

	t.Run("root + child + child with grandchild", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(1),
					Children: []UDataHiearchyEntry{
						{
							Value: Int(2),
						},
					},
				},
				{
					Value: Int(3),
				},
			},
		}

		//root
		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Key(ctx)) {
			return
		}

		//second child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Key(ctx)) {
			return
		}

		//grand child (child of first child)
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(3), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(3), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.False(t, walker.Next(ctx)) {
			return
		}
	})

	t.Run("root + child + child with grandchild", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(1),
				},
				{
					Value: Int(2),
					Children: []UDataHiearchyEntry{
						{
							Value: Int(3),
						},
					},
				},
			},
		}

		//root
		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Key(ctx)) {
			return
		}

		//second child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Key(ctx)) {
			return
		}

		//grand child (child of second child)
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(3), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(3), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.False(t, walker.Next(ctx)) {
			return
		}
	})

	t.Run("root + both children with grandchild", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		udata := &UData{
			Root: Int(0),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(1),
					Children: []UDataHiearchyEntry{
						{
							Value: Int(2),
						},
					},
				},
				{
					Value: Int(3),
					Children: []UDataHiearchyEntry{
						{
							Value: Int(4),
						},
					},
				},
			},
		}

		//root
		walker, err := udata.Walker(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(0), walker.Key(ctx)) {
			return
		}

		//child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(1), walker.Key(ctx)) {
			return
		}

		//grand child (child of first child)
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(2), walker.Key(ctx)) {
			return
		}

		//second child
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(3), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(3), walker.Key(ctx)) {
			return
		}

		//grand child (child of second child)
		if !assert.True(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.True(t, walker.Next(ctx)) {
			return
		}
		if !assert.Equal(t, Int(4), walker.Value(ctx)) {
			return
		}
		if !assert.Equal(t, Int(4), walker.Key(ctx)) {
			return
		}

		//end
		if !assert.False(t, walker.HasNext(ctx)) {
			return
		}
		if !assert.False(t, walker.Next(ctx)) {
			return
		}
	})
}
