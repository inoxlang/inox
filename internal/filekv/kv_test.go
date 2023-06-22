package filekv

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

//TODO: add equivalent tests for transactions

func TestKvSet(t *testing.T) {

	fls := newMemFilesystem()

	kv, err := OpenSingleFileKV(KvStoreConfig{
		Path:       "/data.kv",
		Filesystem: fls,
	})

	if !assert.NoError(t, err) {
		return
	}

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
	})

	//create item
	kv.Set(ctx, "/data", core.Int(1), kv)

	//check item exists
	val, ok, err := kv.Get(ctx, "/data", kv)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.True(t, bool(ok)) {
		return
	}

	assert.Equal(t, core.Int(1), val)
}

func TestKvInsert(t *testing.T) {

	t.Run("simple", func(t *testing.T) {
		fls := newMemFilesystem()

		kv, err := OpenSingleFileKV(KvStoreConfig{
			Path:       "/data.kv",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
		})

		//create item
		kv.Insert(ctx, "/data", core.Int(1), kv)

		//check item exists
		val, ok, err := kv.Get(ctx, "/data", kv)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, bool(ok)) {
			return
		}

		assert.Equal(t, core.Int(1), val)
	})

	t.Run("double insert", func(t *testing.T) {
		fls := newMemFilesystem()

		kv, err := OpenSingleFileKV(KvStoreConfig{
			Path:       "/data.kv",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
		})

		//first insert
		kv.Insert(ctx, "/data", core.Int(1), kv)

		//second insert
		func() {
			defer func() {
				e := recover()
				assert.ErrorIs(t, e.(error), ErrKeyAlreadyPresent)
			}()
			kv.Insert(ctx, "/data", core.Int(2), kv)
		}()

		//check item exists and has the original value
		val, ok, err := kv.Get(ctx, "/data", kv)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, bool(ok)) {
			return
		}

		assert.Equal(t, core.Int(1), val)
	})
}

func TestKvForEach(t *testing.T) {

	fls := newMemFilesystem()

	kv, err := OpenSingleFileKV(KvStoreConfig{
		Path:       "/data.kv",
		Filesystem: fls,
	})

	if !assert.NoError(t, err) {
		return
	}

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
	})

	//create item
	kv.Set(ctx, "/data", core.Int(1), kv)

	iterCount := 0
	err = kv.ForEach(ctx, func(key core.Path, getVal func() core.Value) error {

		assert.Equal(t, core.Path("/data"), key)
		assert.Equal(t, core.Int(1), getVal())

		iterCount++
		return nil
	}, kv)

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, 1, iterCount)
}

func TestKvDelete(t *testing.T) {

	fls := newMemFilesystem()

	kv, err := OpenSingleFileKV(KvStoreConfig{
		Path:       "/data.kv",
		Filesystem: fls,
	})

	if !assert.NoError(t, err) {
		return
	}

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
	})

	//create item
	kv.Set(ctx, "/data", core.Int(1), kv)

	//check item exists
	val, ok, err := kv.Get(ctx, "/data", kv)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.True(t, bool(ok)) {
		return
	}

	assert.Equal(t, core.Int(1), val)

	//delete item
	kv.Delete(ctx, "/data", kv)

	//check item no longer exists
	val, ok, err = kv.Get(ctx, "/data", kv)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.False(t, bool(ok)) {
		return
	}

	assert.Equal(t, core.Nil, val)
}
