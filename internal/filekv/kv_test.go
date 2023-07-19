package filekv

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

//TODO: add equivalent tests for transactions

func TestKvSet(t *testing.T) {

	for _, serialized := range []string{"not_serialized", "serialized"} {
		t.Run(serialized, func(t *testing.T) {
			for _, txCase := range []string{"no_tx", "tx", "finished_tx"} {
				t.Run(txCase, func(t *testing.T) {

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

					var tx *core.Transaction
					if txCase != "no_tx" {
						tx = core.StartNewTransaction(ctx)
						if txCase == "finished_tx" {
							utils.PanicIfErr(tx.Commit(ctx))
						}
					}

					//create item
					if serialized == "not_serialized" {
						kv.Set(ctx, "/data", core.Int(1), kv)
					} else {
						kv.SetSerialized(ctx, "/data", "1", kv)
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
						kv.db.View(func(tx *Tx) error {
							val, err := tx.Get("/data")
							if assert.NoError(t, err) {
								assert.Equal(t, "1", val)
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
							kv.db.View(func(tx *Tx) error {
								val, err := tx.Get("/data")
								if assert.NoError(t, err) {
									assert.Equal(t, "1", val)
								}
								return nil
							})
						}
					}
				})
			}
		})
	}

}

func TestKvGetSerialized(t *testing.T) {

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
	val, ok, err := kv.GetSerialized(ctx, "/data", kv)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.True(t, bool(ok)) {
		return
	}

	assert.Equal(t, "1", val)
}

func TestKvInsert(t *testing.T) {

	for _, serialized := range []string{"not_serialized", "serialized"} {
		t.Run(serialized, func(t *testing.T) {
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
				if serialized == "not_serialized" {
					kv.Insert(ctx, "/data", core.Int(1), kv)
				} else {
					kv.InsertSerialized(ctx, "/data", "1", kv)
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
				if serialized == "not_serialized" {
					kv.Insert(ctx, "/data", core.Int(1), kv)
				} else {
					kv.InsertSerialized(ctx, "/data", "1", kv)
				}

				//second insert
				func() {
					defer func() {
						e := recover()
						assert.ErrorIs(t, e.(error), ErrKeyAlreadyPresent)
					}()
					if serialized == "not_serialized" {
						kv.Insert(ctx, "/data", core.Int(1), kv)
					} else {
						kv.InsertSerialized(ctx, "/data", "2", kv)
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
		})
	}

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
