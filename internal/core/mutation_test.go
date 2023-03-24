package internal

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestObjectOnMutation(t *testing.T) {
	t.Run("callback microtask should be called after additional property is set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{}, ctx)
		called := atomic.Bool{}

		_, err := obj.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			called.Store(true)

			assert.Equal(t, NewAddPropMutation(ctx, "a", Int(1), ShallowWatching, "/a"), mutation)
			return true
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NoError(t, obj.SetProp(ctx, "a", Int(1))) {
			return
		}

		assert.True(t, called.Load())
	})

	t.Run("callback microtask should be called when an existing property is set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx)
		called := atomic.Bool{}

		_, err := obj.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			called.Store(true)

			assert.Equal(t, NewUpdatePropMutation(ctx, "a", Int(2), ShallowWatching, "/a"), mutation)
			return true
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NoError(t, obj.SetProp(ctx, "a", Int(2))) {
			return
		}

		assert.True(t, called.Load())
	})

	t.Run("callback microtask should be called after value of property has a shallow change", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		innerObj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx)
		obj := NewObjectFromMap(ValMap{"inner": innerObj}, ctx)
		called := atomic.Bool{}

		_, err := obj.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			called.Store(true)

			assert.Equal(t, NewUpdatePropMutation(ctx, "a", Int(2), IntermediateDepthWatching, "/inner/a"), mutation)
			return true
		}, MutationWatchingConfiguration{Depth: IntermediateDepthWatching})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NoError(t, innerObj.SetProp(ctx, "a", Int(2))) {
			return
		}

		assert.True(t, called.Load())
	})

	t.Run("callback microtask should be NOT called after additional property is set if callback has been removed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{}, ctx)
		called := atomic.Bool{}

		handle, err := obj.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			called.Store(true)
			return true
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if !assert.NoError(t, err) {
			return
		}

		obj.RemoveMutationCallback(ctx, handle)

		if !assert.NoError(t, obj.SetProp(ctx, "a", Int(1))) {
			return
		}

		assert.False(t, called.Load())
	})
}

func TestListOnMutation(t *testing.T) {

	t.Run("microtask should be called when an element is inserted", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		list := NewWrappedValueList()
		called := atomic.Bool{}

		_, err := list.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, NewInsertElemAtIndexMutation(ctx, 0, Int(1), ShallowWatching, "/0"), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		time.Sleep(time.Microsecond)
		list.insertElement(ctx, Int(1), 0)

		assert.True(t, called.Load())
		assert.Equal(t, []Value{Int(1)}, list.GetOrBuildElements(ctx))
	})

	t.Run("microtask should be called when an element is set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		list := NewWrappedValueList(Int(1))
		called := atomic.Bool{}

		_, err := list.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, NewSetElemAtIndexMutation(ctx, 0, Int(2), ShallowWatching, "/0"), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		time.Sleep(time.Microsecond)
		list.set(ctx, 0, Int(2))

		assert.True(t, called.Load())
		assert.Equal(t, []Value{Int(2)}, list.GetOrBuildElements(ctx))
	})

	t.Run("dynamic map invocation: microtask should NOT be called when an element is inserted if callback has been removed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		list := NewWrappedValueList()
		called := atomic.Bool{}

		handle, err := list.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		list.RemoveMutationCallback(ctx, handle)
		list.insertElement(ctx, Int(1), 0)

		assert.False(t, called.Load())
	})
}

func TestRuneSliceOnMutation(t *testing.T) {

	t.Run("microtask should be called when an element is inserted", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		slice := NewRuneSlice(nil)
		called := atomic.Bool{}

		_, err := slice.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, NewInsertElemAtIndexMutation(ctx, 0, Rune('a'), ShallowWatching, "/0"), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		time.Sleep(time.Microsecond)
		slice.insertElement(ctx, Rune('a'), 0)

		assert.True(t, called.Load())
		assert.Equal(t, []rune("a"), slice.elements)
	})

	t.Run("microtask should be called when an element is set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		slice := NewRuneSlice([]rune("a"))
		called := atomic.Bool{}

		_, err := slice.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, NewSetElemAtIndexMutation(ctx, 0, Rune('b'), ShallowWatching, "/0"), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		time.Sleep(time.Microsecond)
		slice.set(ctx, 0, Rune('b'))

		assert.True(t, called.Load())
		assert.Equal(t, []rune("b"), slice.elements)
	})

	t.Run("dynamic map invocation: microtask should NOT be called when an element is inserted if callback has been removed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		slice := NewWrappedValueList()
		called := atomic.Bool{}

		handle, err := slice.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		slice.RemoveMutationCallback(ctx, handle)
		slice.insertElement(ctx, Rune('a'), 0)

		assert.False(t, called.Load())
	})
}

func TestDynamicMemberOnMutation(t *testing.T) {

	t.Run("dynamic member of object: microtask should be called when member is set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{"int": Int(1)}, ctx)
		dyn, _ := NewDynamicMemberValue(ctx, obj, "int")
		called := atomic.Bool{}

		_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, NewUnspecifiedMutation(ShallowWatching, ""), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(time.Microsecond)
			obj.SetProp(ctx, "int", Int(2))
		}()

		time.Sleep(10 * time.Millisecond)

		assert.True(t, called.Load())
	})

	t.Run("dynamic member of dynamic value: microtask should be called when member is set "+
		"and dyn member should resolve to new value", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		innerObj := NewObjectFromMap(ValMap{"int": Int(1)}, ctx)
		obj := NewObjectFromMap(ValMap{"innerObj": innerObj}, ctx)

		dyn0, _ := NewDynamicMemberValue(ctx, obj, "innerObj")
		dyn, _ := NewDynamicMemberValue(ctx, dyn0, "int")

		called := atomic.Bool{}

		_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)
			assert.Equal(t, NewUnspecifiedMutation(ShallowWatching, ""), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(time.Microsecond)
			innerObj.SetProp(ctx, "int", Int(2))
		}()

		time.Sleep(10 * time.Millisecond)

		assert.True(t, called.Load())
		assert.Equal(t, Int(2), dyn.Resolve(ctx))
	})

	t.Run("dynamic member of dynamic value: microtask should be called when dynamic value changes"+
		"and dyn member should resolve to member of new value", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		innerObj := NewObjectFromMap(ValMap{"int": Int(1)}, ctx)
		obj := NewObjectFromMap(ValMap{"innerObj": innerObj}, ctx)

		dyn0, _ := NewDynamicMemberValue(ctx, obj, "innerObj")
		dyn, _ := NewDynamicMemberValue(ctx, dyn0, "int")

		called := atomic.Bool{}

		_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, NewUnspecifiedMutation(ShallowWatching, ""), mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(time.Microsecond)
			obj.SetProp(ctx, "innerObj", NewObjectFromMap(ValMap{"int": Int(2)}, ctx))
		}()

		time.Sleep(10 * time.Millisecond)

		assert.True(t, called.Load())
		assert.Equal(t, Int(2), dyn.Resolve(ctx))
	})

	t.Run("dynamic member of object: microtask should be NOT called when member is set if callback has been removed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{"int": Int(1)}, ctx)
		dyn, _ := NewDynamicMemberValue(ctx, obj, "int")
		called := atomic.Bool{}

		handle, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			called.Store(true)
			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(time.Microsecond)
			dyn.RemoveMutationCallback(ctx, handle)
			obj.SetProp(ctx, "int", Int(2))
		}()

		time.Sleep(10 * time.Millisecond)

		assert.False(t, called.Load())
	})

	t.Run("dynamic member of object: microtask should be NOT called when member is set if callback has been removed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{"int": Int(1)}, ctx)
		dyn, _ := NewDynamicMemberValue(ctx, obj, "int")
		called := atomic.Bool{}

		handle, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			called.Store(true)
			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(time.Microsecond)
			dyn.RemoveMutationCallback(ctx, handle)
			obj.SetProp(ctx, "int", Int(2))
		}()

		time.Sleep(10 * time.Millisecond)

		assert.False(t, called.Load())
	})
}

func TestDynamicMapInvocationOnMutation(t *testing.T) {

	t.Run("dynamic map invocation: microtask should be called when an element is inserted", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		list := NewWrappedValueList()
		dyn, _ := NewDynamicMapInvocation(ctx, list, PropertyName("a"))
		called := atomic.Bool{}

		_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, Mutation{
				Kind:  UnspecifiedMutation,
				Depth: ShallowWatching,
			}, mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		time.Sleep(time.Microsecond)
		list.insertElement(ctx, objFrom(ValMap{"a": Int(1)}), 0)

		assert.True(t, called.Load())
		assert.Equal(t, NewWrappedValueList(Int(1)), dyn.Resolve(ctx))
	})

	t.Run("dynamic map invocation: microtask should be called when an element is set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		list := NewWrappedValueList(objFrom(ValMap{"a": Int(1)}))
		dyn, _ := NewDynamicMapInvocation(ctx, list, PropertyName("a"))
		called := atomic.Bool{}

		_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, Mutation{
				Kind:  UnspecifiedMutation,
				Depth: ShallowWatching,
			}, mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		time.Sleep(time.Microsecond)
		list.set(ctx, 0, objFrom(ValMap{"a": Int(2)}))

		assert.True(t, called.Load())
		assert.Equal(t, NewWrappedValueList(Int(2)), dyn.Resolve(ctx))
	})

	t.Run("dynamic map invocation: microtask should NOT be called when an element is inserted if callback has been removed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		list := NewWrappedValueList()
		dyn, _ := NewDynamicMapInvocation(ctx, list, PropertyName("a"))
		called := atomic.Bool{}

		handle, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		// we modify the list in the same goroutine since List is not sharable
		dyn.RemoveMutationCallback(ctx, handle)
		list.insertElement(ctx, objFrom(ValMap{"a": Int(1)}), 0)

		assert.False(t, called.Load())
	})
}

func TestDynamicIfnOnMutation(t *testing.T) {

	t.Run("dynamic map invocation: microtask should be called when an element is inserted", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(ValMap{"condition": False}, ctx)
		cond, _ := NewDynamicMemberValue(ctx, obj, "condition")

		dyn := NewDynamicIf(ctx, cond, Int(1), Int(2))
		assert.Equal(t, Int(2), dyn.Resolve(ctx))

		called := atomic.Bool{}

		_, err := dyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			if !assert.False(t, called.Load()) {
				return
			}
			called.Store(true)

			assert.Equal(t, Mutation{
				Kind:  UnspecifiedMutation,
				Depth: ShallowWatching,
			}, mutation)

			return true
		}, MutationWatchingConfiguration{})

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(time.Microsecond)
			obj.SetProp(ctx, "condition", True)
		}()

		time.Sleep(time.Millisecond)

		assert.True(t, called.Load())
		assert.Equal(t, Int(1), dyn.Resolve(ctx))
	})

}
