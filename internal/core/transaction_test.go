package core

import (
	"runtime"
	"testing"
	"time"

	utils "github.com/inoxlang/inox/internal/utils/common"
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

	t.Run("during the commit phase calling one of a transaction's methods is invalid", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		tx := newTransaction(ctx, false)
		tx.Start(ctx)

		tx.OnEnd(1, func(tx *Transaction, success bool) {
			assert.ErrorIs(t, tx.OnEnd(ctx, func(tx *Transaction, success bool) {}), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.AddEffect(ctx, nil), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.Commit(ctx), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.Rollback(ctx), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.Start(ctx), ErrFinishingTransaction)

			assert.False(t, tx.IsFinished())
			assert.True(t, tx.IsFinishing())
		})
		assert.NoError(t, tx.Commit(ctx))
	})

	t.Run("during the rollback phase calling one of a transaction's methods is invalid", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		tx := newTransaction(ctx, false)
		tx.Start(ctx)

		tx.OnEnd(1, func(tx *Transaction, success bool) {
			assert.ErrorIs(t, tx.OnEnd(ctx, func(tx *Transaction, success bool) {}), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.AddEffect(ctx, nil), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.Commit(ctx), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.Rollback(ctx), ErrFinishingTransaction)
			assert.ErrorIs(t, tx.Start(ctx), ErrFinishingTransaction)

			assert.False(t, tx.IsFinished())
			assert.True(t, tx.IsFinishing())
		})
		assert.NoError(t, tx.Rollback(ctx))
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

	t.Run("once transaction is finished the IsFinished should return true and Finished() should be closed", func(t *testing.T) {

		t.Run("after Commit", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)

			select {
			case <-tx.Finished():
				assert.Fail(t, "tx should not considered finished")
				return
			default:
			}

			if tx.IsFinished() {
				assert.Fail(t, "tx should not considered finished")
				return
			}

			assert.NoError(t, tx.Commit(ctx))

			select {
			case <-tx.Finished():
			default:
				assert.FailNow(t, "tx should be considered finished")
				return
			}

			if !tx.IsFinished() {
				assert.FailNow(t, "tx should be considered finished")
				return
			}

		})

		t.Run("after Rollback", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			tx := newTransaction(ctx, false)
			tx.Start(ctx)

			select {
			case <-tx.Finished():
				assert.Fail(t, "tx should not considered finished")
				return
			default:
			}

			if tx.IsFinished() {
				assert.Fail(t, "tx should not considered finished")
				return
			}

			assert.NoError(t, tx.Rollback(ctx))

			select {
			case <-tx.Finished():
			default:
				assert.FailNow(t, "tx should be considered finished")
				return
			}

			if !tx.IsFinished() {
				assert.FailNow(t, "tx should be considered finished")
				return
			}

		})

	})

	t.Run("gracefully cancelling the context should cause the transaction to roll back", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		tx := newTransaction(ctx, false)
		tx.Start(ctx)

		called := make(chan struct{}, 1)
		assert.NoError(t, tx.OnEnd(0, func(tx *Transaction, success bool) {
			called <- struct{}{}
			assert.False(t, success)
		}))

		go ctx.CancelGracefully()

		select {
		case <-called:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("ungracefully cancelling the context should cause the transaction to roll back", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		tx := newTransaction(ctx, false)
		tx.Start(ctx)

		called := make(chan struct{}, 1)
		assert.NoError(t, tx.OnEnd(0, func(tx *Transaction, success bool) {
			called <- struct{}{}
			assert.False(t, success)
		}))

		go ctx.CancelUngracefully()

		select {
		case <-called:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}
