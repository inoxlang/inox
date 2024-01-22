package mapcoll

import (
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

const (
	INT_1 = core.Int(1)
	INT_2 = core.Int(2)

	STRING_A = core.Str("a")
	STRING_B = core.Str("b")
)

func TestSharedUnpersistedMapSet(t *testing.T) {
	t.Run("Map should be updated at end of transaction if .Set was called transactionnaly", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		m := NewMapWithConfig(ctx, nil, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		})
		m.Set(ctx, INT_1, STRING_A)

		assert.NoError(t, tx.Commit(ctx))

		otherCtx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		assert.True(t, bool(m.Has(otherCtx, INT_1)))
	})

	//Tests with several transactions.

	t.Run("transactions should wait for the previous transaction to finish", func(t *testing.T) {
		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()
		tx1 := core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()
		core.StartNewTransaction(ctx2)

		m := NewMapWithConfig(ctx1, nil, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		})

		m.Share(ctx1.GetClosestState())

		m.Set(ctx1, INT_1, STRING_A)
		assert.True(t, bool(m.Has(ctx1, INT_1)))

		tx2Done := make(chan struct{})
		go func() { //second transaction
			m.Set(ctx2, INT_2, STRING_B)

			//since the first transaction should be finished,
			//the other element should have been added.
			assert.True(t, bool(m.Has(ctx2, INT_1)))
			assert.True(t, bool(m.Has(ctx2, INT_2)))
			tx2Done <- struct{}{}
		}()

		assert.NoError(t, tx1.Commit(ctx1))

		<-tx2Done
	})

	t.Run("writes in subsequent transactions", func(t *testing.T) {
		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()
		tx1 := core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()
		tx2 := core.StartNewTransaction(ctx2)

		ctx3 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx3.CancelGracefully()
		core.StartNewTransaction(ctx3)

		m := NewMapWithConfig(ctx1, nil, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		})
		m.Share(ctx1.GetClosestState())

		//First transaction.

		m.Set(ctx1, INT_1, STRING_A)
		if !assert.NoError(t, tx1.Commit(ctx1)) {
			return
		}

		//Second transaction.

		assert.True(t, bool(m.Has(ctx2, INT_1)))

		m.Set(ctx2, INT_2, STRING_B)
		if !assert.NoError(t, tx2.Commit(ctx2)) {
			return
		}

		//Third transaction.
		assert.True(t, bool(m.Has(ctx3, INT_1)))
		assert.True(t, bool(m.Has(ctx3, INT_2)))
	})

}

func TestSharedUnpersistedMapHas(t *testing.T) {

	//Tests with several transactions.

	t.Run("readonly transactions can read the Map in parallel", func(t *testing.T) {
		ctx1, ctx2, _ := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		readTx1 := core.StartNewReadonlyTransaction(ctx1)
		core.StartNewReadonlyTransaction(ctx2)

		entries := core.NewWrappedValueList(INT_1, STRING_A, INT_2, STRING_B)

		m := NewMapWithConfig(ctx1, entries, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		})
		m.Share(ctx1.GetClosestState())

		assert.True(t, bool(m.Has(ctx1, INT_1)))
		assert.True(t, bool(m.Has(ctx2, INT_1)))

		assert.True(t, bool(m.Has(ctx1, INT_1)))
		assert.True(t, bool(m.Has(ctx2, INT_1)))

		assert.NoError(t, readTx1.Commit(ctx1))
		assert.True(t, bool(m.Has(ctx2, INT_1)))
	})
}

func TestSharedUnpersistedMapRemove(t *testing.T) {
	//TODO
}

func sharedSetTestSetup(t *testing.T) (*core.Context, core.DataStore) {
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

func sharedSetTestSetup2(t *testing.T) (*core.Context, *core.Context, core.DataStore) {
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
