package core

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventSourceBase(t *testing.T) {

	t.Run("regular handler registration", func(t *testing.T) {
		t.Run("OnEvent()", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			callCount := 0

			event := NewEvent(nil, Int(1), DateTime(time.Now()))

			assert.NoError(t, evs.OnEvent(func(ev *Event) {
				callCount++
				assert.Same(t, event, ev)
			}))

			for _, handler := range evs.GetHandlers() {
				handler(event)
			}

			assert.EqualValues(t, 1, callCount)
		})

		t.Run("OnEvent() twice", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			callCount := 0

			event := NewEvent(nil, Int(1), DateTime(time.Now()))

			assert.NoError(t, evs.OnEvent(func(ev *Event) {
				callCount++
				assert.Same(t, event, ev)
			}))

			assert.NoError(t, evs.OnEvent(func(ev *Event) {
				callCount++
				assert.Same(t, event, ev)
			}))

			for _, handler := range evs.GetHandlers() {
				handler(event)
			}

			assert.EqualValues(t, 2, callCount)
		})

		t.Run("OnEvent() followed by RemoveAllHandlers()", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			callCount := 0

			assert.NoError(t, evs.OnEvent(func(ev *Event) {
				callCount++
			}))

			evs.RemoveAllHandlers()

			assert.Empty(t, evs.GetHandlers())
			assert.Zero(t, callCount)
		})
	})

	t.Run("IDLE handler configured to wait for the first (non-ignored) event", func(t *testing.T) {

		t.Run("no ignored events", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			var callCount atomic.Int64

			oldEventAge := 2 * IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL
			evs.OnIDLE(IdleEventSourceHandler{
				MinimumLastEventAge: oldEventAge,
				//wait for first event
				DontWaitForFirstEvent: false,
				Microtask: func() {
					callCount.Add(1)
				},
			})

			assert.Zero(t, callCount.Load())

			time.Sleep(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL/2)
			assert.Zero(t, callCount.Load())

			time.Sleep(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.Zero(t, callCount.Load())

			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(1), DateTime(time.Now()))
				handler(event)
			}
			assert.Zero(t, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.Zero(t, callCount.Load())

			//wait long enough for the event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 1, callCount.Load())
		})

		t.Run("ignore first event", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			var callCount atomic.Int64

			oldEventAge := 2 * IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL
			evs.OnIDLE(IdleEventSourceHandler{
				MinimumLastEventAge: oldEventAge,
				//wait for first event
				DontWaitForFirstEvent: false,
				Microtask: func() {
					callCount.Add(1)
				},
				IsIgnoredEvent: func(e *Event) bool {
					return e.Value().(Int) == 1
				},
			})

			assert.Zero(t, callCount.Load())

			time.Sleep(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL/2)
			assert.Zero(t, callCount.Load())

			time.Sleep(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.Zero(t, callCount.Load())

			//emit first event
			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(1), DateTime(time.Now()))
				handler(event)
			}
			assert.Zero(t, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.Zero(t, callCount.Load())

			//Wait long enough for the first event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			//The handler should not have been called because the event should have been ignored.
			assert.Zero(t, callCount.Load())

			//emit second event
			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(2), DateTime(time.Now()))
				handler(event)
			}
			assert.Zero(t, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.Zero(t, callCount.Load())

			//wait long enough for the second event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 1, callCount.Load())
		})

		t.Run("two spaced events", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			var callCount atomic.Int64

			oldEventAge := 2 * IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL
			evs.OnIDLE(IdleEventSourceHandler{
				MinimumLastEventAge: oldEventAge,
				//wait for first event
				DontWaitForFirstEvent: false,
				Microtask: func() {
					callCount.Add(1)
				},
				//No events is ignored.
			})

			assert.Zero(t, callCount.Load())

			time.Sleep(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL/2)
			assert.Zero(t, callCount.Load())

			time.Sleep(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.Zero(t, callCount.Load())

			//emit first event
			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(1), DateTime(time.Now()))
				handler(event)
			}
			assert.Zero(t, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.Zero(t, callCount.Load())

			//Wait long enough for the first event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			//The handler should not have been called because the event should have been ignored.
			assert.EqualValues(t, 1, callCount.Load())

			//emit second event
			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(2), DateTime(time.Now()))
				handler(event)
			}
			assert.EqualValues(t, 1, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.EqualValues(t, 1, callCount.Load())

			//wait long enough for the second event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 2, callCount.Load())
		})
	})

	t.Run("IDLE handler configured to not wait for the first event", func(t *testing.T) {
		t.Run("no ignored events", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			var callCount atomic.Int64

			oldEventAge := 2 * IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL
			evs.OnIDLE(IdleEventSourceHandler{
				MinimumLastEventAge:   oldEventAge,
				DontWaitForFirstEvent: true,
				Microtask: func() {
					callCount.Add(1)
				},
			})

			assert.Zero(t, callCount.Load())

			//wait long enough for at least one tick to have passed.
			time.Sleep(2*IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL/2)
			assert.EqualValues(t, 1, callCount.Load())

			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(1), DateTime(time.Now()))
				handler(event)
			}

			assert.EqualValues(t, 1, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.EqualValues(t, 1, callCount.Load())

			//wait long enough for the event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 2, callCount.Load())
		})

		t.Run("ignore first event", func(t *testing.T) {
			evs := &EventSourceBase{}
			defer evs.RemoveAllHandlers()
			var callCount atomic.Int64

			oldEventAge := 2 * IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL
			evs.OnIDLE(IdleEventSourceHandler{
				MinimumLastEventAge:   oldEventAge,
				DontWaitForFirstEvent: true,
				Microtask: func() {
					callCount.Add(1)
				},
				IsIgnoredEvent: func(e *Event) bool {
					return e.Value().(Int) == 1
				},
			})

			assert.Zero(t, callCount.Load())

			//wait long enough for at least one tick to have passed.
			time.Sleep(2 * IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 1, callCount.Load())

			//emit first event
			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(1), DateTime(time.Now()))
				handler(event)
			}
			assert.EqualValues(t, 1, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.EqualValues(t, 1, callCount.Load())

			//wait long enough for the first event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 1, callCount.Load())

			//emit second event
			for _, handler := range evs.GetHandlers() {
				event := NewEvent(nil, Int(2), DateTime(time.Now()))
				handler(event)
			}
			assert.EqualValues(t, 1, callCount.Load())

			time.Sleep(oldEventAge / 2)
			assert.EqualValues(t, 1, callCount.Load())

			//wait long enough for the second event to be old and the next tick to have passed.
			time.Sleep(oldEventAge/2 + IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
			assert.EqualValues(t, 2, callCount.Load())
		})
	})
}
