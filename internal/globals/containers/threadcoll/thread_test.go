package threadcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestThreadAdd(t *testing.T) {
	const THREAD_URL = core.URL("ldb://main/threads/59595")
	var THREAD_DIR_URL_PATH = THREAD_URL.ToDirURL().Path()

	t.Run("elements should be visible by the tx that added them and should be visible to all txs after their tx is commited", func(t *testing.T) {
		t.Run("one element added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

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

			time := ctx1.Now()

			list := thread.GetElementsBefore(ctx1, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Zero(t, list.Len())

			//Commit the first transaction.

			assert.NoError(t, tx1.Commit(ctx1))

			//Check that the message is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, ctx2.Now(), 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)
		})

		t.Run("two elements added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

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

			time := ctx1.Now()

			list := thread.GetElementsBefore(ctx1, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)

			//Check that the messages are visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Zero(t, list.Len())

			//Commit the first transaction.

			assert.NoError(t, tx1.Commit(ctx1))

			//Check that the messages are visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, ctx2.Now(), 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)
		})

		t.Run("one element added by each tx", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

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

			time := ctx1.Now()

			list := thread.GetElementsBefore(ctx1, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that only $message2 is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message2), list)

			//Commit the first transaction.

			assert.NoError(t, tx1.Commit(ctx1))

			//Check that both messages are visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, ctx2.Now(), 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)
		})

	})

	t.Run("elements should not be visible if their tx was roll backed", func(t *testing.T) {
		t.Run("elem added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

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

			time := ctx1.Now()

			list := thread.GetElementsBefore(ctx1, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Zero(t, list.Len())

			//Roll back the first transaction.

			assert.NoError(t, tx1.Rollback(ctx1))

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Zero(t, list.Len())
		})

		t.Run("two elements added by tx1", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

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

			time := ctx1.Now()

			list := thread.GetElementsBefore(ctx1, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message2, message1), list)

			//Check that no message is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Zero(t, list.Len())

			//Roll back the first transaction.

			assert.NoError(t, tx1.Rollback(ctx1))

			//Check that the message is not visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Zero(t, list.Len())
		})

		t.Run("one element added by each tx", func(t *testing.T) {
			ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx1.CancelGracefully()

			ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx2.CancelGracefully()

			thread := newEmptyThread(ctx1, THREAD_URL, ThreadConfig{Element: core.EMPTY_INEXACT_OBJECT_PATTERN})

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

			time := ctx1.Now()

			list := thread.GetElementsBefore(ctx1, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message1), list)

			//Check that only $message2 is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, time, 10)
			assert.Equal(t, core.NewWrappedValueList(message2), list)

			//Roll back the first transaction.

			assert.NoError(t, tx1.Rollback(ctx1))

			//Check that only $message2 is visible from tx2's POV.

			list = thread.GetElementsBefore(ctx2, ctx2.Now(), 10)
			assert.Equal(t, core.NewWrappedValueList(message2), list)
		})
	})

}
