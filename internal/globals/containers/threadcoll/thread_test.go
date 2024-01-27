package threadcoll

import (
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestThreadAdd(t *testing.T) {
	const THREAD_URL = core.URL("ldb://main/threads/59595")
	var THREAD_DIR_URL_PATH = THREAD_URL.ToDirURL().Path()

	threadPattern := NewThreadPattern(ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

	t.Run("elements should be visible by the tx that added them and should be visible to all txs after their tx is commited", func(t *testing.T) {
		t.Run("one element added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, threadPattern)

			tx1 := core.StartNewTransaction(ctx1)
			core.StartNewTransaction(ctx2)

			//Add a message.

			message1 := core.NewObject()
			thread.Add(ctx1, message1)

			url, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url.Path()))
			}

			//Check that the message is visible from tx1's POV.

			list := thread.GetElementsBefore(ctx1, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Zero(t, list.Len())

			//Commit the first transaction.

			assert.NoError(t, tx1.Commit(ctx1))

			//Check that the message is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)
		})

		t.Run("two elements added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, threadPattern)

			tx1 := core.StartNewTransaction(ctx1)
			core.StartNewTransaction(ctx2)

			//Add a message.

			message1 := core.NewObject()
			message2 := core.NewObject()
			thread.Add(ctx1, message1)
			thread.Add(ctx1, message2)

			//Check URLs.

			url, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url.Path()))
			}

			url2, ok := message2.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url2.Path()))
			}

			//Check that the messages are visible from tx1's POV.

			list := thread.GetElementsBefore(ctx1, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)

			//Check that the messages are visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Zero(t, list.Len())

			//Commit the first transaction.

			assert.NoError(t, tx1.Commit(ctx1))

			//Check that the messages are visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)
		})

		t.Run("one element added by each tx", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, threadPattern)

			tx1 := core.StartNewTransaction(ctx1)
			core.StartNewTransaction(ctx2)

			//Add a message (tx1).

			message1 := core.NewObject()
			thread.Add(ctx1, message1)

			url1, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url1.Path()))
			}

			//Add a message (tx2).

			message2 := core.NewObject()
			thread.Add(ctx2, message2)

			url2, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url2.Path()))
			}

			//Check that only $message1 is visible from tx1's POV.

			list := thread.GetElementsBefore(ctx1, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that only $message2 is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2), list)

			//Commit the first transaction.

			assert.NoError(t, tx1.Commit(ctx1))

			//Check that both messages are visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)
		})

	})

	t.Run("elements should not be visible if their tx was roll backed", func(t *testing.T) {
		t.Run("elem added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, threadPattern)

			tx1 := core.StartNewTransaction(ctx1)
			core.StartNewTransaction(ctx2)

			//Add a message.

			message1 := core.NewObject()
			thread.Add(ctx1, message1)

			url, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url.Path()))
			}

			//Check that the message is visible from tx1's POV.

			list := thread.GetElementsBefore(ctx1, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Zero(t, list.Len())

			//Roll back the first transaction.

			assert.NoError(t, tx1.Rollback(ctx1))

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Zero(t, list.Len())
		})

		t.Run("two elements added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, threadPattern)

			tx1 := core.StartNewTransaction(ctx1)
			core.StartNewTransaction(ctx2)

			//Add a message.

			message1 := core.NewObject()
			message2 := core.NewObject()
			thread.Add(ctx1, message1)
			thread.Add(ctx1, message2)

			url1, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url1.Path()))
			}

			url2, ok := message2.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url2.Path()))
			}

			//Check that both messages are visible from tx1's POV.

			list := thread.GetElementsBefore(ctx1, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)

			//Check that no message is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Zero(t, list.Len())

			//Roll back the first transaction.

			assert.NoError(t, tx1.Rollback(ctx1))

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Zero(t, list.Len())
		})

		t.Run("one element added by each tx", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, threadPattern)

			tx1 := core.StartNewTransaction(ctx1)
			core.StartNewTransaction(ctx2)

			//Add a message (tx1).

			message1 := core.NewObject()
			thread.Add(ctx1, message1)

			url1, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url1.Path()))
			}

			//Add a message (tx2).

			message2 := core.NewObject()
			thread.Add(ctx2, message2)

			url2, ok := message1.URL()
			if assert.True(t, ok) {
				assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(url2.Path()))
			}

			//Check that only $message1 is visible from tx1's POV.

			list := thread.GetElementsBefore(ctx1, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that only $message2 is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2), list)

			//Roll back the first transaction.

			assert.NoError(t, tx1.Rollback(ctx1))

			//Check that only $message2 is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, core.MAX_ULID, 10)
			assert.Equal(t, core.NewWrappedValueList(message2), list)
		})
	})

	t.Run("Set should be persisted at end of successful transaction if .Add was called transactionnaly", func(t *testing.T) {
		ctx, storage := sharedThreadTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		storage.SetSerialized(ctx, THREAD_DIR_URL_PATH, `[]`)
		val, err := loadThread(ctx, core.FreeEntityLoadingParams{
			Key: THREAD_DIR_URL_PATH, Storage: storage, Pattern: threadPattern,
		})

		thread := val.(*MessageThread)
		thread.Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		message1 := core.NewObject()
		thread.Add(ctx, message1)

		msgURL, ok := message1.URL()
		if assert.True(t, ok) {
			assert.True(t, THREAD_DIR_URL_PATH.CanBeDirOfEntry(msgURL.Path()))
		}

		assert.True(t, bool(thread.Contains(ctx, message1)))
		values := core.IterateAllValuesOnly(ctx, thread.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{message1}, values)

		//Check that the Set is not persised

		persisted, err := loadThread(ctx, core.FreeEntityLoadingParams{
			Key: THREAD_DIR_URL_PATH, Storage: storage, Pattern: threadPattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, thread) //future-proofing the test
		elems := core.IterateAllValuesOnly(ctx, persisted.(*MessageThread).Iterator(ctx, core.IteratorConfiguration{}))
		if !assert.Empty(t, elems) {
			return
		}

		// Commit the transaction.

		assert.NoError(t, tx.Commit(ctx))

		//Check that the Set is persised

		persisted, err = loadThread(ctx, core.FreeEntityLoadingParams{
			Key: THREAD_DIR_URL_PATH, Storage: storage, Pattern: threadPattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, thread) //future-proofing the test
		elems = core.IterateAllValuesOnly(ctx, thread.Iterator(ctx, core.IteratorConfiguration{}))
		if !assert.Len(t, elems, 1) {
			return
		}

		u, ok := elems[0].(*core.Object).URL()
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, msgURL, u)
	})
}

func sharedThreadTestSetup(t *testing.T) (*core.Context, core.DataStore) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.DatabasePermission{
				Kind_:  permkind.Read,
				Entity: core.Host("ldb://main"),
			},
			core.DatabasePermission{
				Kind_:  permkind.Write,
				Entity: core.Host("ldb://main"),
			},
		},
	}, nil)
	kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "kv")),
	}))
	storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
	return ctx, storage
}

func sharedThreadTestSetup2(t *testing.T) (*core.Context, *core.Context, core.DataStore) {
	config := core.ContextConfig{
		Permissions: []core.Permission{
			core.DatabasePermission{
				Kind_:  permkind.Read,
				Entity: core.Host("ldb://main"),
			},
			core.DatabasePermission{
				Kind_:  permkind.Write,
				Entity: core.Host("ldb://main"),
			},
		},
	}

	ctx1 := core.NewContexWithEmptyState(config, nil)
	ctx2 := core.NewContexWithEmptyState(config, nil)

	kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "kv")),
	}))
	storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
	return ctx1, ctx2, storage
}
