package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSmartLock(t *testing.T) {
	t.Run("base case", func(t *testing.T) {
		state1 := NewGlobalState(NewContext(ContextConfig{}))
		defer state1.Ctx.CancelGracefully()
		state2 := NewGlobalState(NewContext(ContextConfig{}))
		defer state2.Ctx.CancelGracefully()

		embedder := NewObject()
		var lock SmartLock
		lock.Share(state1, func() {})

		heldByState2 := make(chan struct{})
		lock.Lock(state1, embedder)

		go func() {
			lock.Lock(state2, embedder)
			heldByState2 <- struct{}{}
			lock.Unlock(state2, embedder)
		}()

		time.Sleep(10 * time.Millisecond)

		select {
		case <-heldByState2:
			assert.FailNow(t, "the lock should be held by state1")
		default:
		}

		lock.Unlock(state1, embedder)

		select {
		case <-heldByState2:
			//ok
		case <-time.After(10 * time.Millisecond):
			assert.FailNow(t, "timeout: the lock should be held by state2")
		}

		//Lock again with state1.

		time.Sleep(10 * time.Millisecond)

		heldByState1 := make(chan struct{})

		go func() {
			lock.Lock(state1, embedder)
			heldByState1 <- struct{}{}
		}()

		select {
		case <-heldByState1:
			//ok
		case <-time.After(10 * time.Millisecond):
			assert.FailNow(t, "timeout: the lock should be held by state1")
		}
	})

	t.Run("the context of the holder should be cancelled if another module wants to acquire the lock after SMART_LOCK_HOLD_TIMEOUT has ellapsed", func(t *testing.T) {
		state1 := NewGlobalState(NewContext(ContextConfig{}))
		defer state1.Ctx.CancelGracefully()
		state2 := NewGlobalState(NewContext(ContextConfig{}))
		defer state2.Ctx.CancelGracefully()

		embedder := NewObject()
		var lock SmartLock
		lock.Share(state1, func() {})

		state1Panic := make(chan any)

		go func() {
			defer func() {
				state1Panic <- recover()
			}()
			lock.Lock(state1, embedder)
			<-state1.Ctx.Done()
			panic(state1.Ctx.Err())
		}()

		time.Sleep(SMART_LOCK_HOLD_TIMEOUT)

		select {
		case e := <-state1Panic:
			assert.FailNow(t, "the module of state1 should not have panicked since no module is waiting to acquire the lock", e)
		default:
		}

		start := time.Now()
		lock.Lock(state2, embedder)

		time.Sleep(time.Millisecond)

		assert.WithinDuration(t, start.Add(5*time.Millisecond), time.Now(), 5*time.Millisecond)

		select {
		case e := <-state1Panic:
			assert.ErrorIs(t, e.(error), context.Canceled)
		default:
			assert.FailNow(t, "the module of state1 should already have panicked since the waiting module should have cancelled its context")
		}
	})

	t.Run("if another module immediately wants to acquire the lock, the context of the holder should be cancelled after SMART_LOCK_HOLD_TIMEOUT", func(t *testing.T) {
		state1 := NewGlobalState(NewContext(ContextConfig{}))
		defer state1.Ctx.CancelGracefully()
		state2 := NewGlobalState(NewContext(ContextConfig{}))
		defer state2.Ctx.CancelGracefully()

		embedder := NewObject()
		var lock SmartLock
		lock.Share(state1, func() {})

		state1Panic := make(chan any)

		go func() {
			defer func() {
				state1Panic <- recover()
			}()
			lock.Lock(state1, embedder)
			<-state1.Ctx.Done()
			panic(state1.Ctx.Err())
		}()

		startWaiting := make(chan struct{})
		go func() {
			startWaiting <- struct{}{}
			lock.Lock(state2, embedder)
		}()

		<-startWaiting

		start := time.Now()

		select {
		case e := <-state1Panic:
			assert.Greater(t, time.Since(start), SMART_LOCK_HOLD_TIMEOUT-time.Millisecond)
			assert.ErrorIs(t, e.(error), context.Canceled)
		case <-time.After(2 * SMART_LOCK_HOLD_TIMEOUT):
			select {
			case <-state1.Ctx.Done():
				assert.FailNow(t, "?")
			default:
				assert.FailNow(t, "the module of state1 should already have panicked since the waiting module should have cancelled its context")
			}
		}
	})
}
