package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueList(t *testing.T) {

	t.Run("set", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := newValueList(Int(1))
		list.set(ctx, 0, Int(2))
		assert.Equal(t, []Serializable{Int(2)}, list.elements)
	})

	t.Run("setSlice", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newValueList(Int(1))
			list.SetSlice(ctx, 0, 1, newValueList(Int(2)))
			assert.Equal(t, []Serializable{Int(2)}, list.elements)
		})

		t.Run("several elements", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newValueList(Int(1), Int(2))
			list.SetSlice(ctx, 0, 2, newValueList(Int(3), Int(4)))
			assert.Equal(t, []Serializable{Int(3), Int(4)}, list.elements)
		})
	})

	t.Run("insertElement", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := newValueList(Int(1))
		list.insertElement(ctx, Int(2), 0)
		assert.Equal(t, []Serializable{Int(2), Int(1)}, list.elements)
	})

	t.Run("insertSequence", func(t *testing.T) {
		t.Run("at existing index", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newValueList(Int(1))
			list.insertSequence(ctx, newValueList(Int(2), Int(3)), 0)
			assert.Equal(t, []Serializable{Int(2), Int(3), Int(1)}, list.elements)
		})
		t.Run("at exclusive end", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newValueList(Int(1))
			list.insertSequence(ctx, newValueList(Int(2), Int(3)), 1)
			assert.Equal(t, []Serializable{Int(1), Int(2), Int(3)}, list.elements)
		})
	})

	t.Run("appendSequence", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := newValueList(Int(1))
		list.appendSequence(ctx, newValueList(Int(2), Int(3)))
		assert.Equal(t, []Serializable{Int(1), Int(2), Int(3)}, list.elements)
	})

	t.Run("removePosition", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := newValueList(Int(1))
		list.removePosition(ctx, 0)
		assert.Equal(t, []Serializable{}, list.elements)
	})

	t.Run("removePositionRange", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newValueList(Int(1))
			list.removePositionRange(ctx, NewIncludedEndIntRange(0, 0))
			assert.Equal(t, []Serializable{}, list.elements)
		})

		t.Run("several elements", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newValueList(Int(1), Int(2))
			list.removePositionRange(ctx, NewIncludedEndIntRange(0, 1))
			assert.Equal(t, []Serializable{}, list.elements)
		})
	})
}
