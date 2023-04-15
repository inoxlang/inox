package internal

import (
	"path/filepath"
	"sync"
	"testing"

	badger "github.com/dgraph-io/badger/v3"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestLocalDatabase(t *testing.T) {

	t.Run("context has a transaction", func(t *testing.T) {

		create := func() (*LocalDatabase, *Context, *core.Transaction) {
			core.ResetResourceMap()
			ldb, err := NewLocalDatabase(LocalDatabaseConfig{InMemory: true})
			assert.NoError(t, err)

			ctx := core.NewContext(core.ContextConfig{})
			tx := core.StartNewTransaction(ctx)
			return ldb, ctx, tx
		}

		t.Run("Get non existing", func(t *testing.T) {
			ldb, ctx, tx := create()
			defer ldb.Close()

			v, ok := ldb.Get(ctx, Path("/a"))
			assert.False(t, bool(ok))
			assert.Equal(t, core.Nil, v)

			assert.NoError(t, tx.Rollback(ctx))
		})

		t.Run("Set -> Get -> commit", func(t *testing.T) {
			ldb, ctx, tx := create()
			defer ldb.Close()

			key := Path("/a")
			r := ldb.GetFullResourceName(key)
			ldb.Set(ctx, key, Int(1))
			assert.False(t, core.TryAcquireResource(r))

			v, ok := ldb.Get(ctx, key)
			assert.True(t, bool(ok))
			assert.Equal(t, Int(1), v)
			assert.False(t, core.TryAcquireResource(r))

			//we check that the database transaction is not commited yet
			ldb.db.View(func(txn *badger.Txn) error {
				_, err := txn.Get([]byte(key))
				assert.ErrorIs(t, err, badger.ErrKeyNotFound)
				return nil
			})

			assert.NoError(t, tx.Commit(ctx))
			assert.True(t, core.TryAcquireResource(r))
			core.ReleaseResource(r)

			//we check that the database transaction is commited
			ldb.db.View(func(txn *badger.Txn) error {
				item, err := txn.Get([]byte(key))
				assert.NoError(t, err)

				item.Value(func(val []byte) error {
					v, err := core.ParseRepr(ctx, val)
					assert.NoError(t, err)
					assert.Equal(t, Int(1), v)
					return nil
				})
				return nil
			})
		})

		t.Run("Set -> rollback", func(t *testing.T) {
			ldb, ctx, tx := create()
			defer ldb.Close()

			key := Path("/a")
			r := ldb.GetFullResourceName(key)
			ldb.Set(ctx, key, Int(1))
			assert.False(t, core.TryAcquireResource(r))

			v, ok := ldb.Get(ctx, key)
			assert.True(t, bool(ok))
			assert.Equal(t, Int(1), v)

			//we check that the database transaction is not commited yet
			ldb.db.View(func(txn *badger.Txn) error {
				_, err := txn.Get([]byte(key))
				assert.ErrorIs(t, err, badger.ErrKeyNotFound)
				return nil
			})

			assert.NoError(t, tx.Rollback(ctx))
			assert.True(t, core.TryAcquireResource(r))
			core.ReleaseResource(r)

			//we check that the database transaction is not commited
			ldb.db.View(func(txn *badger.Txn) error {
				_, err := txn.Get([]byte(key))
				assert.ErrorIs(t, err, badger.ErrKeyNotFound)
				return nil
			})

			//same
			v, ok = ldb.Get(ctx, key)
			assert.True(t, core.TryAcquireResource(r))
			core.ReleaseResource(r)
			assert.Equal(t, core.Nil, v)
			assert.False(t, bool(ok))
		})

	})

	t.Run("context has no transaction", func(t *testing.T) {

		create := func() (*LocalDatabase, *Context) {
			core.ResetResourceMap()
			ldb, err := NewLocalDatabase(LocalDatabaseConfig{InMemory: true})
			assert.NoError(t, err)

			ctx := core.NewContext(core.ContextConfig{})
			return ldb, ctx
		}

		t.Run("Get non existing", func(t *testing.T) {
			ldb, ctx := create()
			defer ldb.Close()

			v, ok := ldb.Get(ctx, Path("/a"))
			assert.False(t, bool(ok))
			assert.Equal(t, core.Nil, v)
		})

		t.Run("Set then Get", func(t *testing.T) {
			ldb, ctx := create()
			defer ldb.Close()

			key := Path("/a")
			ldb.Set(ctx, key, Int(1))

			v, ok := ldb.Get(ctx, key)
			assert.True(t, bool(ok))
			assert.Equal(t, Int(1), v)

			//we check that the database transaction is commited
			ldb.db.View(func(txn *badger.Txn) error {
				item, err := txn.Get([]byte(key))
				assert.NoError(t, err)

				item.Value(func(val []byte) error {
					v, err := core.ParseRepr(ctx, val)
					assert.NoError(t, err)
					assert.Equal(t, Int(1), v)
					return nil
				})
				return nil
			})
		})
	})

	t.Run("context gets transaction in the middle of the execution", func(t *testing.T) {

		create := func() (*LocalDatabase, *Context) {
			core.ResetResourceMap()
			ldb, err := NewLocalDatabase(LocalDatabaseConfig{InMemory: true})
			assert.NoError(t, err)

			ctx := core.NewContext(core.ContextConfig{})
			return ldb, ctx
		}

		t.Run("Set with no tx then set with tx", func(t *testing.T) {
			ldb, ctx := create()
			defer ldb.Close()

			//first call to Set
			key := Path("/a")
			ldb.Set(ctx, key, Int(1))

			//attach transaction
			core.StartNewTransaction(ctx)

			//second call to Set
			ldb.Set(ctx, key, Int(2))

			v, ok := ldb.Get(ctx, key)
			assert.True(t, bool(ok))
			assert.Equal(t, Int(2), v)
		})
	})
}

func TestOpenDatabase(t *testing.T) {

	t.Run("open same database sequentially", func(t *testing.T) {
		dir, _ := filepath.Abs(t.TempDir())
		dir += "/"

		pattern := core.PathPattern(dir + "...")

		ctxConfig := core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: core.ReadPerm, Entity: pattern},
				core.FilesystemPermission{Kind_: core.CreatePerm, Entity: pattern},
				core.FilesystemPermission{Kind_: core.WritePerm, Entity: pattern},
			},
			HostResolutions: map[core.Host]core.Value{
				core.Host("ldb://main"): core.Path(dir),
			},
		}

		ctx1 := core.NewContext(ctxConfig)
		_db, err := openDatabase(ctx1, core.Path(dir))
		if !assert.NoError(t, err) {
			return
		}
		defer _db.Close()

		ctx2 := core.NewContext(ctxConfig)
		db, err := openDatabase(ctx2, core.Path(dir))
		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, db, _db)
	})

	t.Run("open same database in parallel", func(t *testing.T) {
		dir, _ := filepath.Abs(t.TempDir())
		dir += "/"

		pattern := core.PathPattern(dir + "...")

		ctxConfig := core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: core.ReadPerm, Entity: pattern},
				core.FilesystemPermission{Kind_: core.CreatePerm, Entity: pattern},
				core.FilesystemPermission{Kind_: core.WritePerm, Entity: pattern},
			},
			HostResolutions: map[core.Host]core.Value{
				core.Host("ldb://main"): core.Path(dir),
			},
		}

		wg := new(sync.WaitGroup)
		wg.Add(2)

		var db, _db *LocalDatabase

		go func() {
			defer wg.Done()

			//open database in first context
			ctx1 := core.NewContext(ctxConfig)
			_db, err := openDatabase(ctx1, core.Path(dir))
			if !assert.NoError(t, err) {
				return
			}
			db = _db

			defer func() {
				db.Close()
			}()
		}()

		go func() {
			defer wg.Done()
			//open same database in second context
			ctx2 := core.NewContext(ctxConfig)
			db, err := openDatabase(ctx2, core.Path(dir))
			if !assert.NoError(t, err) {
				return
			}
			_db = db
		}()
		wg.Wait()

		assert.Same(t, db, _db)
	})
}
