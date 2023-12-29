package core

import (
	"runtime"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTransaction(t *testing.T) {
	{
		runtime.GC()
		defer utils.AssertNoGoroutineLeak(t, runtime.NumGoroutine())
	}

	// for _, method := range []string{"commit", "rollback"} {
	// 	t.Run("after call to "+method+" all acquired resources should be released", func(t *testing.T) {
	// 		ctx := NewContext(ContextConfig{})
	// 		tx := newTransaction(ctx, false)
	// 		tx.Start(ctx)

	// 		resource := Path("/a" + strconv.Itoa(rand.Int()))
	// 		tx.acquireResource(ctx, resource)

	// 		//we check that the resource is already acquired
	// 		wg := new(sync.WaitGroup)
	// 		wg.Add(1)
	// 		go func() {
	// 			defer wg.Done()
	// 			ok := TryAcquireConcreteResource(resource)
	// 			assert.False(t, ok)
	// 		}()
	// 		wg.Wait()

	// 		//when transaction is commited or rollbacked all associated resources are supposed to be released
	// 		if method == "commit" {
	// 			tx.Commit(ctx)
	// 		} else {
	// 			tx.Rollback(ctx)
	// 		}

	// 		//we check that the resource is released
	// 		wg.Add(1)
	// 		go func() {
	// 			defer wg.Done()
	// 			ok := TryAcquireConcreteResource(resource)
	// 			if assert.True(t, ok) {
	// 				ReleaseConcreteResource(resource)
	// 			}

	// 		}()
	// 		wg.Wait()
	// 	})
	// }

	t.Run("once transaction is commited calling one of a transaction's methods is invalid", func(t *testing.T) {

		t.Run("Start", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.Start(ctx), ErrFinishedTransaction)
		})

		t.Run("Commit", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.Commit(ctx), ErrFinishedTransaction)
		})

		t.Run("Rollback", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.Rollback(ctx), ErrFinishedTransaction)
		})

	})

	t.Run("once transaction is rollbacked calling one of a transaction's methods is invalid", func(t *testing.T) {

		t.Run("Start", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.Start(ctx), ErrFinishedTransaction)
		})

		t.Run("Commit", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.Commit(ctx), ErrFinishedTransaction)
		})

		t.Run("Rollback", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.Rollback(ctx), ErrFinishedTransaction)
		})

	})

	t.Run("transaction timeout", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		NewGlobalState(ctx)
		tx := newTransaction(ctx, false, Option{TX_TIMEOUT_OPTION_NAME, Duration(time.Millisecond)})
		tx.Start(ctx)

		time.Sleep(2 * time.Millisecond)
		assert.ErrorIs(t, tx.Commit(ctx), ErrFinishedTransaction)
	})

	t.Run("during a commit a panic with an integer value in a callback function should not prevent other callbacks to be called", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		NewGlobalState(ctx)
		tx := newTransaction(ctx, false)
		tx.Start(ctx)

		callCount := 0

		tx.OnEnd(1, func(tx *Transaction, success bool) {
			assert.True(t, success)
			callCount++
		})

		tx.OnEnd(2, func(tx *Transaction, success bool) {
			assert.True(t, success)
			callCount++
			panic(123)
		})

		tx.OnEnd(3, func(tx *Transaction, success bool) {
			assert.True(t, success)
			callCount++
		})

		time.Sleep(2 * time.Millisecond)
		err := tx.Commit(ctx)

		if !assert.Equal(t, 3, callCount) {
			return
		}

		if !assert.ErrorContains(t, err, "callback errors") {
			return
		}

		assert.ErrorContains(t, err, "123")
	})

	t.Run("during a rollback a panic with an integer value in a callback function should not prevent other callbacks to be called", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		NewGlobalState(ctx)
		tx := newTransaction(ctx, false)
		tx.Start(ctx)

		callCount := 0

		tx.OnEnd(1, func(tx *Transaction, success bool) {
			assert.False(t, success)
			callCount++
		})

		tx.OnEnd(2, func(tx *Transaction, success bool) {
			assert.False(t, success)
			callCount++
			panic(123)
		})

		tx.OnEnd(3, func(tx *Transaction, success bool) {
			assert.False(t, success)
			callCount++
		})

		time.Sleep(2 * time.Millisecond)
		err := tx.Rollback(ctx)

		if !assert.Equal(t, 3, callCount) {
			return
		}

		if !assert.ErrorContains(t, err, "callback errors") {
			return
		}

		assert.ErrorContains(t, err, "123")
	})
}
