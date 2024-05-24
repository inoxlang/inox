package core

import (
	"runtime"
	"testing"
	"time"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestObjectWatcher(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 200, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
			CheckGoroutines:        true,
			GoroutineCount:         runtime.NumGoroutine(),
		})
	}

	t.Run("mutations", func(t *testing.T) {
		t.Run("watcher should be informed about new property", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			obj := NewObject()
			w := obj.Watcher(ctx, WatcherConfiguration{Filter: MUTATION_PATTERN}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				time.Sleep(time.Microsecond)
				obj.SetProp(ctx, "a", Int(1))
			}()

			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewAddPropMutation(ctx, "a", Int(1), ShallowWatching, "/a"), v)
			w.Stop()

			_, err = w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})

		t.Run("watcher should be informed about an existing property being set", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			obj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx)
			w := obj.Watcher(ctx, WatcherConfiguration{Filter: MUTATION_PATTERN}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				time.Sleep(time.Microsecond)
				obj.SetProp(ctx, "a", Int(2))
			}()

			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewUpdatePropMutation(ctx, "a", Int(2), ShallowWatching, "/a"), v)
			w.Stop()

			_, err = w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})

		t.Run("intermediate depth watcher should be informed about the shallow changes of a property value", func(t *testing.T) {
			t.Skip()
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			innerObj := NewObjectFromMap(ValMap{"b": Int(1)}, ctx)
			obj := NewObjectFromMap(ValMap{"a": innerObj}, ctx)
			w := obj.Watcher(ctx, WatcherConfiguration{Filter: MUTATION_PATTERN, Depth: IntermediateDepthWatching}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				time.Sleep(time.Microsecond)
				innerObj.SetProp(ctx, "a", Int(2))
			}()

			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewUpdatePropMutation(ctx, "b", Int(2), ShallowWatching, "/a/b"), v)
			w.Stop()

			_, err = w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})

		t.Run("shallow watcher should NOT be informed about the shallow changes of a property value", func(t *testing.T) {
			t.Skip()

			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			innerObj := NewObjectFromMap(ValMap{"b": Int(1)}, ctx)
			obj := NewObjectFromMap(ValMap{"a": innerObj}, ctx)
			w := obj.Watcher(ctx, WatcherConfiguration{Filter: MUTATION_PATTERN, Depth: ShallowWatching}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				time.Sleep(time.Microsecond)
				innerObj.SetProp(ctx, "a", Int(2))
			}()

			_, err := w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})
	})

	t.Run("received messages", func(t *testing.T) {
		t.Run("watcher should return a message after a message has been received", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			obj := NewObject()
			w := obj.Watcher(ctx, WatcherConfiguration{Filter: MSG_PATTERN}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				obj.ReceiveMessage(ctx, NewMessage(Int(1), nil))
			}()

			msg, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, Message{}, msg)
			assert.Equal(t, Int(1), msg.(Message).data)
			w.Stop()

			_, err = w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})

		t.Run("watcher should not return anything after the object has changed", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			obj := NewObject()
			w := obj.Watcher(ctx, WatcherConfiguration{Filter: MSG_PATTERN}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				obj.SetProp(ctx, "a", Int(1))
			}()

			msg, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.ErrorIs(t, err, ErrWatchTimeout) {
				return
			}

			assert.Nil(t, msg)
		})
	})

}

func TestDictionaryWatcher(t *testing.T) {

	t.Run("mutations", func(t *testing.T) {
		t.Run("watcher should be informed about new property", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			dict := NewDictionary(ValMap{})
			w := dict.Watcher(ctx, WatcherConfiguration{Filter: MUTATION_PATTERN}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				time.Sleep(time.Microsecond)
				dict.SetValue(ctx, String("a"), Int(1))
			}()

			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewAddEntryMutation(ctx, String("a"), Int(1), ShallowWatching, `/"a"`), v)
			w.Stop()

			_, err = w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})

		t.Run("watcher should be informed about an existing property being set", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			dict := NewDictionary(ValMap{`"a"`: Int(1)})
			w := dict.Watcher(ctx, WatcherConfiguration{Filter: MUTATION_PATTERN}).(*GenericWatcher)
			defer w.Stop()

			go func() {
				time.Sleep(time.Microsecond)
				dict.SetValue(ctx, String("a"), Int(2))
			}()

			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewUpdateEntryMutation(ctx, String("a"), Int(2), ShallowWatching, `/"a"`), v)
			w.Stop()

			_, err = w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})
	})
}

func TestJoinedWatchers(t *testing.T) {
	// TODO
}

func TestGenericWatcher(t *testing.T) {
	// TODO
}

func TestPeriodicWatcher(t *testing.T) {

	var PERIOD = 10 * PERIODIC_WATCHER_GOROUTINE_TICK_INTERVAL

	for i := 0; i < 5; i++ {

		t.Run("next value set once", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			w := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w.Stop()

			go func() {
				w.InformAboutAsync(ctx, Int(1))
			}()

			start := time.Now()
			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.True(t, time.Since(start) < 3*PERIOD) // TODO: ellapsed time should be < 2 PERIOD
			assert.Equal(t, Int(1), v)
		})

		t.Run("next value quickly set twice", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			w := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w.Stop()

			go func() {
				w.InformAboutAsync(ctx, Int(1))
				w.InformAboutAsync(ctx, Int(2))
			}()

			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			start := time.Now()
			assert.True(t, time.Since(start) < 2*PERIOD)
			assert.Equal(t, Int(2), v)
		})

		t.Run("next value set once: not matching additional filter", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			w := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w.Stop()

			go func() {
				w.InformAboutAsync(ctx, Int(1))
			}()

			v, err := w.WaitNext(ctx, STR_PATTERN, time.Second/5)
			assert.ErrorIs(t, err, ErrWatchTimeout)
			assert.Nil(t, v)
		})

		t.Run("watcher stopped while it is waiting next value", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			w := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w.Stop()

			go func() {
				time.Sleep(time.Millisecond)
				w.Stop()
			}()

			_, err := w.WaitNext(ctx, nil, time.Second)
			assert.ErrorIs(t, err, ErrStoppedWatcher)
		})

		t.Run("delay before watcher start waiting next value", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			w := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w.Stop()

			go func() {
				w.InformAboutAsync(ctx, Int(1))
			}()

			time.Sleep(10 * PERIOD)
			v, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, Int(1), v)
		})

		t.Run("delay before 2 watchers start to wait next value", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			w1 := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w1.Stop()

			w2 := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer w1.Stop()

			go func() {
				w1.InformAboutAsync(ctx, Int(1))
				w2.InformAboutAsync(ctx, Int(2))
			}()

			time.Sleep(10 * PERIOD)
			v, err := w1.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, Int(1), v)

			v, err = w2.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, Int(2), v)
		})

		t.Run("delay before watcher start to wait next value + IDLE watcher", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			idleWatcher := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer idleWatcher.Stop()

			watcher := NewPeriodicWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN}, PERIOD)
			defer idleWatcher.Stop()

			go func() {
				watcher.InformAboutAsync(ctx, Int(2))
			}()

			time.Sleep(10 * PERIOD)

			v, err := watcher.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, Int(2), v)
		})

		//TODO: add more tests
	}

}
