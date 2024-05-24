package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestTraverse(t *testing.T) {

	//TODO: check visited values

	t.Run("integer", func(t *testing.T) {
		called := false

		err := Traverse(Int(1), func(v Value) (ast.TraversalAction, error) {
			called = true
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{})

		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("empty object", func(t *testing.T) {
		called := false

		err := Traverse(objFrom(nil), func(v Value) (ast.TraversalAction, error) {
			called = true
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{})

		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("object with an integer property : max depth = 0", func(t *testing.T) {
		callCount := 0

		err := Traverse(objFrom(ValMap{"n": Int(1)}), func(v Value) (ast.TraversalAction, error) {
			callCount++
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 0})

		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("object with an integer property : max depth = 1", func(t *testing.T) {
		callCount := 0

		err := Traverse(objFrom(ValMap{"n": Int(1)}), func(v Value) (ast.TraversalAction, error) {
			callCount++
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 1})

		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("object with a reference to itself", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		callCount := 0

		obj := &Object{}
		obj.SetProp(ctx, "self", obj)

		err := Traverse(obj, func(v Value) (ast.TraversalAction, error) {
			callCount++
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("empty record", func(t *testing.T) {
		called := false

		err := Traverse(objFrom(nil), func(v Value) (ast.TraversalAction, error) {
			called = true
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{})

		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("record with an integer property : max depth = 0", func(t *testing.T) {
		callCount := 0

		err := Traverse(NewRecordFromMap(ValMap{"n": Int(1)}), func(v Value) (ast.TraversalAction, error) {
			callCount++
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 0})

		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("record with an integer property : max depth = 1", func(t *testing.T) {
		callCount := 0

		err := Traverse(NewRecordFromMap(ValMap{"n": Int(1)}), func(v Value) (ast.TraversalAction, error) {
			callCount++
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 1})

		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("list with a reference to itself", func(t *testing.T) {
		callCount := 0

		list := &List{underlyingList: &ValueList{}}
		list.append(nil, list)

		err := Traverse(list, func(v Value) (ast.TraversalAction, error) {
			callCount++
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("treedata with only a root", func(t *testing.T) {
		treedata := &Treedata{Root: Int(1)}

		var visited []Value

		err := Traverse(treedata, func(v Value) (ast.TraversalAction, error) {
			visited = append(visited, v)
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		if assert.Len(t, visited, 2) {
			assert.Equal(t, []Value{treedata, treedata.Root}, visited)
		}
	})

	t.Run("treedata with a root and a childless entry", func(t *testing.T) {
		treedata := &Treedata{
			Root: Int(1),
			HiearchyEntries: []TreedataHiearchyEntry{
				{Value: Int(2)},
			},
		}

		var visited []Value

		err := Traverse(treedata, func(v Value) (ast.TraversalAction, error) {
			visited = append(visited, v)
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		if assert.Len(t, visited, 3) {
			assert.Equal(t, []Value{treedata, treedata.Root, Int(2)}, visited)
		}
	})

	t.Run("treedata with a root and a single-child entry", func(t *testing.T) {
		treedata := &Treedata{
			Root: Int(1),
			HiearchyEntries: []TreedataHiearchyEntry{
				{
					Value:    Int(2),
					Children: []TreedataHiearchyEntry{{Value: Int(3)}},
				},
			},
		}

		var visited []Value

		err := Traverse(treedata, func(v Value) (ast.TraversalAction, error) {
			visited = append(visited, v)
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		if assert.Len(t, visited, 4) {
			assert.Equal(t, []Value{treedata, treedata.Root, Int(2), Int(3)}, visited)
		}
	})

	t.Run("treedata with binary tree structure of depth 2", func(t *testing.T) {
		treedata := &Treedata{
			Root: Int(1),
			HiearchyEntries: []TreedataHiearchyEntry{
				{
					Value:    Int(2),
					Children: []TreedataHiearchyEntry{{Value: Int(3)}},
				},
				{
					Value:    Int(4),
					Children: []TreedataHiearchyEntry{{Value: Int(5)}},
				},
			},
		}

		var visited []Value

		err := Traverse(treedata, func(v Value) (ast.TraversalAction, error) {
			visited = append(visited, v)
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		if assert.Len(t, visited, 6) {
			assert.Equal(t, []Value{treedata, treedata.Root, Int(2), Int(3), Int(4), Int(5)}, visited)
		}
	})

	t.Run("pruning", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		callCount := 0

		v := NewWrappedValueList(
			objFrom(ValMap{
				"v": Int(1),
			}),
			objFrom(ValMap{
				"v": Int(2),
			}),
		)
		err := Traverse(v, func(v Value) (ast.TraversalAction, error) {
			callCount++
			if obj, ok := v.(*Object); ok && obj.Prop(ctx, "v") == Int(1) {
				return ast.Prune, nil
			}
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		assert.Equal(t, 4, callCount)
	})

	t.Run("stop", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		callCount := 0

		v := NewWrappedValueList(
			objFrom(ValMap{
				"v": Int(1),
			}),
			objFrom(ValMap{
				"v": Int(2),
			}),
		)
		err := Traverse(v, func(v Value) (ast.TraversalAction, error) {
			callCount++
			if obj, ok := v.(*Object); ok && obj.Prop(ctx, "v") == Int(1) {
				return ast.StopTraversal, nil
			}
			return ast.ContinueTraversal, nil
		}, TraversalConfiguration{MaxDepth: 10})

		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})
}
