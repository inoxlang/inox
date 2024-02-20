package threadcoll

import (
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestPersistLoadThread(t *testing.T) {
	const THREAD_URL = core.URL("ldb://main/threads/58585")
	const THREAD_PATH = core.Path("/threads/58585")

	setup := func() (*core.Context, core.DataStore) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("empty", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewThreadPattern(ThreadConfig{})
		thread := newEmptyThread(ctx, THREAD_URL, pattern)

		//persist
		{
			persistThread(ctx, thread, THREAD_PATH, storage)

			serialized, ok := storage.GetSerialized(ctx, THREAD_PATH)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "[]", serialized)
		}

		loadedThread, err := loadThread(ctx, core.FreeEntityLoadingParams{
			Key: THREAD_PATH, Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, loadedThread) {
			return
		}

		//thread should be shared
		assert.True(t, loadedThread.(*MessageThread).IsShared())
	})

	t.Run("single element", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewThreadPattern(ThreadConfig{})
		thread := newEmptyThread(ctx, THREAD_URL, pattern)

		obj := core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(1)})
		thread.Add(ctx, obj)

		elemURL, ok := obj.URL()
		if !assert.True(t, ok) {
			return
		}

		elemULID := utils.Must(getElementIDFromURL(elemURL))
		elemKey := core.ElementKey(elemULID.String())

		//persist
		{
			persistThread(ctx, thread, THREAD_PATH, storage)

			serialized, ok := storage.GetSerialized(ctx, THREAD_PATH)
			if !assert.True(t, ok) {
				return
			}
			assert.Regexp(t, `\[\{.*?\}\]`, serialized)
		}

		loaded, err := loadThread(ctx, core.FreeEntityLoadingParams{
			Key: THREAD_PATH, Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedThread := loaded.(*MessageThread)

		//thread should be shared
		if !assert.True(t, loadedThread.IsShared()) {
			return
		}

		//check elements
		elem, err := loadedThread.GetElementByKey(ctx, elemKey)
		if assert.NoError(t, err) {
			assert.Equal(t, obj.EntryMap(ctx), elem.(*core.Object).EntryMap(ctx))
		}

		//TODO
		// elemKey := loadedMap.getElementPathKeyFromKey(`{"int__value":1}`)
		// elem, err := loadedMap.GetElementByKey(ctx, elemKey)
		// if !assert.NoError(t, err, core.ErrCollectionElemNotFound) {
		// 	return
		// }
		//assert.Equal(t, INT_1, elem)
	})

	t.Run("two elements", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewThreadPattern(ThreadConfig{})
		thread := newEmptyThread(ctx, THREAD_URL, pattern)

		obj1 := core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(1)})
		obj2 := core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(2)})

		thread.Add(ctx, obj1)
		thread.Add(ctx, obj2)

		//Check that obj1 has now a URL.
		elemURL1, ok1 := obj1.URL()
		if !assert.True(t, ok1) {
			return
		}

		elemULID1 := utils.Must(getElementIDFromURL(elemURL1))
		elemKey1 := core.ElementKey(elemULID1.String())

		elemURL2, ok2 := obj2.URL()
		if !assert.True(t, ok2) {
			return
		}

		//Check that obj1 has now a URL.
		elemULID2 := utils.Must(getElementIDFromURL(elemURL2))
		elemKey2 := core.ElementKey(elemULID2.String())

		//persist
		{
			persistThread(ctx, thread, THREAD_PATH, storage)

			serialized, ok := storage.GetSerialized(ctx, THREAD_PATH)
			if !assert.True(t, ok) {
				return
			}
			assert.Regexp(t, `\[\{.*?\},\{.*?\}\]`, serialized)
		}

		loaded, err := loadThread(ctx, core.FreeEntityLoadingParams{
			Key: THREAD_PATH, Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedThread := loaded.(*MessageThread)

		//thread should be shared
		if !assert.True(t, loadedThread.IsShared()) {
			return
		}

		//check elements
		elem1, err := loadedThread.GetElementByKey(ctx, elemKey1)
		if assert.NoError(t, err) {
			assert.Equal(t, obj1.EntryMap(ctx), elem1.(*core.Object).EntryMap(ctx))
		}
		elem2, err := loadedThread.GetElementByKey(ctx, elemKey2)
		if assert.NoError(t, err) {
			assert.Equal(t, obj2.EntryMap(ctx), elem2.(*core.Object).EntryMap(ctx))
		}
	})

	t.Run("migration: deletion", func(t *testing.T) {
		//TODO
	})

	t.Run("migration: replacement", func(t *testing.T) {
		//TODO
	})
}

func TestSetMigrate(t *testing.T) {

	//TODO
}
