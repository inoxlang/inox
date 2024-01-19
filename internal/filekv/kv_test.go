package filekv

import (
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/bbolt"
)

//TODO: add equivalent tests for transactions

func TestKvSet(t *testing.T) {
	t.Parallel()

	t.Run("SetSerialized", func(t *testing.T) {
		testKvSet(t, true)
	})

	t.Run("Set", func(t *testing.T) {
		testKvSet(t, false)
	})
}

func testKvSet(t *testing.T, UseSetSerialized bool) {
	for _, txCase := range []string{"no_tx", "tx", "finished_tx"} {
		txCase := txCase
		t.Run(txCase, func(t *testing.T) {
			t.Parallel()

			kv, err := OpenSingleFileKV(KvStoreConfig{
				Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
			})
			defer ctx.CancelGracefully()

			var tx *core.Transaction
			if txCase != "no_tx" {
				tx = core.StartNewTransaction(ctx)
				if txCase == "finished_tx" {
					utils.PanicIfErr(tx.Commit(ctx))
				}
			}

			repr := core.GetJSONRepresentation(core.Int(1), ctx, nil)

			//create item
			if UseSetSerialized {
				kv.SetSerialized(ctx, "/data", repr, kv)
			} else {
				kv.Set(ctx, "/data", core.Int(1), kv)
			}

			//check item exists
			val, ok, err := kv.Get(ctx, "/data", kv)

			if !assert.NoError(t, err) {
				return
			}

			if !assert.True(t, bool(ok)) {
				return
			}

			assert.Equal(t, core.Int(1), val)

			//check item is persisted
			if txCase != "tx" {
				kv.db.View(func(tx *bbolt.Tx) error {
					val := tx.Bucket(BBOLT_DATA_BUCKET).Get([]byte("/data"))
					if val != nil {
						assert.Equal(t, repr, string(val))
					}
					return nil
				})
			}

			if txCase == "tx" {
				utils.PanicIfErr(tx.Commit(ctx))

				//check item exists
				val, ok, err := kv.Get(ctx, "/data", kv)

				if !assert.NoError(t, err) {
					return
				}

				if !assert.True(t, bool(ok)) {
					return
				}

				assert.Equal(t, core.Int(1), val)

				//check item is persisted
				if txCase != "tx" {
					kv.db.View(func(tx *bbolt.Tx) error {
						val := tx.Bucket(BBOLT_DATA_BUCKET).Get([]byte("/data"))
						if val != nil {
							assert.Equal(t, "1", string(val))
						}
						return nil
					})
				}
			}
		})
	}
}

func TestKvGetSerialized(t *testing.T) {
	t.Parallel()

	kv, err := OpenSingleFileKV(KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
	})

	if !assert.NoError(t, err) {
		return
	}

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
	})
	defer ctx.CancelGracefully()

	//create item
	kv.Set(ctx, "/data", core.Int(1), kv)

	//check item exists
	val, ok, err := kv.GetSerialized(ctx, "/data", kv)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.True(t, bool(ok)) {
		return
	}

	repr := core.GetJSONRepresentation(core.Int(1), ctx, nil)
	assert.Equal(t, repr, val)
}

func TestKvInsert(t *testing.T) {
	t.Run("InsertSerialized", func(t *testing.T) {
		testKvInsert(t, true)
	})

	t.Run("Insert", func(t *testing.T) {
		testKvInsert(t, false)
	})
}

func testKvInsert(t *testing.T, UseInsertSerialized bool) {
	t.Parallel()

	t.Run("simple", func(t *testing.T) {

		kv, err := OpenSingleFileKV(KvStoreConfig{
			Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
		})

		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
		})
		defer ctx.CancelGracefully()

		//create item
		if UseInsertSerialized {
			repr := core.GetJSONRepresentation(core.Int(1), ctx, nil)
			kv.InsertSerialized(ctx, "/data", repr, kv)
		} else {
			kv.Insert(ctx, "/data", core.Int(1), kv)
		}

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
		kv, err := OpenSingleFileKV(KvStoreConfig{
			Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
		})

		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
		})
		defer ctx.CancelGracefully()

		//first insert
		if UseInsertSerialized {
			repr := core.GetJSONRepresentation(core.Int(1), ctx, nil)
			kv.InsertSerialized(ctx, "/data", repr, kv)
		} else {
			kv.Insert(ctx, "/data", core.Int(1), kv)
		}

		//second insert
		func() {
			defer func() {
				e := recover()
				assert.ErrorIs(t, e.(error), ErrKeyAlreadyPresent)
			}()
			if UseInsertSerialized {
				repr := core.GetJSONRepresentation(core.Int(2), ctx, nil)
				kv.InsertSerialized(ctx, "/data", repr, kv)
			} else {
				kv.Insert(ctx, "/data", core.Int(2), kv)
			}
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
	t.Parallel()

	kv, err := OpenSingleFileKV(KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
	})

	if !assert.NoError(t, err) {
		return
	}

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
	})
	defer ctx.CancelGracefully()

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
	t.Parallel()

	kv, err := OpenSingleFileKV(KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
	})

	if !assert.NoError(t, err) {
		return
	}

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
	})
	defer ctx.CancelGracefully()

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
