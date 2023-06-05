package core

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransaction(t *testing.T) {

	for _, method := range []string{"commit", "rollback"} {
		t.Run("after call to "+method+" all acquired resources should be released", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)

			resource := Path("/a" + strconv.Itoa(rand.Int()))
			tx.acquireResource(ctx, resource)

			//we check that the resource is already acquired
			wg := new(sync.WaitGroup)
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok := TryAcquireResource(resource)
				assert.False(t, ok)
			}()
			wg.Wait()

			//when transaction is commited or rollbacked all associated resources are supposed to be released
			if method == "commit" {
				tx.Commit(ctx)
			} else {
				tx.Rollback(ctx)
			}

			//we check that the resource is released
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok := TryAcquireResource(resource)
				if assert.True(t, ok) {
					ReleaseResource(resource)
				}

			}()
			wg.Wait()
		})
	}

	t.Run("once transaction is commited calling one of a transaction's methods is invalid", func(t *testing.T) {

		t.Run("Start", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.Start(ctx), ErrFinishedTransaction)
		})

		t.Run("Commit", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.Commit(ctx), ErrFinishedTransaction)
		})

		t.Run("Rollback", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.Rollback(ctx), ErrFinishedTransaction)
		})

		t.Run("SetValue", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			assert.ErrorIs(t, tx.SetValue(1, 2), ErrFinishedTransaction)
		})

		t.Run("GetValue", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Commit(ctx))

			v, err := tx.GetValue(1)
			assert.Nil(t, v)
			assert.ErrorIs(t, err, ErrFinishedTransaction)
		})
	})

	t.Run("once transaction is rollbacked calling one of a transaction's methods is invalid", func(t *testing.T) {

		t.Run("Start", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.Start(ctx), ErrFinishedTransaction)
		})

		t.Run("Commit", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.Commit(ctx), ErrFinishedTransaction)
		})

		t.Run("Rollback", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.Rollback(ctx), ErrFinishedTransaction)
		})

		t.Run("SetValue", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			assert.ErrorIs(t, tx.SetValue(1, 2), ErrFinishedTransaction)
		})

		t.Run("GetValue", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			tx := NewTransaction(ctx)
			tx.Start(ctx)
			assert.NoError(t, tx.Rollback(ctx))

			v, err := tx.GetValue(1)
			assert.Nil(t, v)
			assert.ErrorIs(t, err, ErrFinishedTransaction)
		})
	})

	t.Run("transaction timeout", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		tx := NewTransaction(ctx, Option{TX_TIMEOUT_OPTION_NAME, Duration(time.Millisecond)})
		tx.Start(ctx)

		time.Sleep(2 * time.Millisecond)
		assert.ErrorIs(t, tx.Commit(ctx), ErrFinishedTransaction)
	})
}
