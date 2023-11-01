//go:build unix

package local_db

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

const MEM_FS_STORAGE_SIZE = 100_000_000

func TestOpenDatabase(t *testing.T) {

	t.Run("opening the same database is forbidden", func(t *testing.T) {
		dir, _ := filepath.Abs(t.TempDir())
		dir += "/"

		pattern := core.PathPattern(dir + "...")

		ctxConfig := core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: pattern},
			},
			HostResolutions: map[core.Host]core.Value{
				core.Host("ldb://main"): core.Path(dir),
			},
			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
		}

		ctx1 := core.NewContexWithEmptyState(ctxConfig, nil)

		_db, err := OpenDatabase(ctx1, core.Path(dir), false)
		if !assert.NoError(t, err) {
			return
		}
		defer _db.Close(ctx1)

		ctx2 := core.NewContexWithEmptyState(ctxConfig, nil)

		db, err := OpenDatabase(ctx2, core.Path(dir), false)
		if !assert.NoError(t, err) {
			return
		}
		assert.NotSame(t, db, _db)
	})

	t.Run("open same database sequentially (in-between closing)", func(t *testing.T) {
		dir, _ := filepath.Abs(t.TempDir())
		dir += "/"

		pattern := core.PathPattern(dir + "...")

		ctxConfig := core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: pattern},
			},
			HostResolutions: map[core.Host]core.Value{
				core.Host("ldb://main"): core.Path(dir),
			},
			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
		}

		ctx1 := core.NewContexWithEmptyState(ctxConfig, nil)

		_db, err := OpenDatabase(ctx1, core.Path(dir), false)
		if !assert.NoError(t, err) {
			return
		}
		_db.Close(ctx1)

		ctx2 := core.NewContexWithEmptyState(ctxConfig, nil)

		db, err := OpenDatabase(ctx2, core.Path(dir), false)
		if !assert.NoError(t, err) {
			return
		}
		defer _db.Close(ctx1)

		assert.NotSame(t, db, _db)
	})

	t.Run("open same database in parallel should result in at least one error", func(t *testing.T) {
		//TODO when implemented.

		t.SkipNow()

		dir, _ := filepath.Abs(t.TempDir())
		dir += "/"

		pattern := core.PathPattern(dir + "...")

		ctxConfig := core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: pattern},
			},
			HostResolutions: map[core.Host]core.Value{
				core.Host("ldb://main"): core.Path(dir),
			},
			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
		}

		wg := new(sync.WaitGroup)
		wg.Add(2)

		var ctx1, ctx2 *core.Context
		var db1, db2 *LocalDatabase

		defer func() {
			if db1 != nil {
				db1.Close(ctx1)
			}
			if db2 != nil {
				db2.Close(ctx2)
			}
		}()

		go func() {
			defer wg.Done()

			//open database in first context
			ctx1 = core.NewContexWithEmptyState(ctxConfig, nil)

			_db1, err := OpenDatabase(ctx1, core.Path(dir), false)
			if !assert.NoError(t, err) {
				return
			}
			db1 = _db1
		}()

		go func() {
			defer wg.Done()
			//open same database in second context
			ctx2 = core.NewContexWithEmptyState(ctxConfig, nil)

			_db2, err := OpenDatabase(ctx2, core.Path(dir), false)
			if !assert.NoError(t, err) {
				return
			}
			db2 = _db2
		}()
		wg.Wait()

		assert.Same(t, db1, db2)
	})

	t.Run("re-open with a schema", func(t *testing.T) {

		t.Run("top-level Set with URL-based uniqueness", func(t *testing.T) {
			dir, _ := filepath.Abs(t.TempDir())
			dir += "/"

			pattern := core.PathPattern(dir + "...")

			ctxConfig := core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Read, Entity: pattern},
					core.FilesystemPermission{Kind_: permkind.Create, Entity: pattern},
					core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: pattern},
				},
				HostResolutions: map[core.Host]core.Value{
					core.Host("ldb://main"): core.Path(dir),
				},
				Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
			}

			ctx := core.NewContexWithEmptyState(ctxConfig, nil)
			ctx.AddNamedPattern("Set", containers.SET_PATTERN)
			ctx.AddNamedPattern("str", containers.SET_PATTERN)

			db, err := OpenDatabase(ctx, core.Path(dir), false)
			if !assert.NoError(t, err) {
				return
			}

			setPattern :=
				utils.Must(containers.SET_PATTERN.CallImpl(
					containers.SET_PATTERN,
					[]core.Serializable{
						core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}), containers_common.URL_UNIQUENESS_IDENT,
					}),
				)

			schema := core.NewInexactObjectPattern(map[string]core.Pattern{
				"users": setPattern,
			})

			db.UpdateSchema(ctx, schema, core.MigrationOpHandlers{})

			err = db.Close(ctx)
			if !assert.NoError(t, err) {
				return
			}

			//re-open

			db, err = OpenDatabase(ctx, core.Path(dir), false)
			if !assert.NoError(t, err) {
				return
			}

			defer db.Close(ctx)

			entities := utils.Must(db.LoadTopLevelEntities(ctx))

			if !assert.Contains(t, entities, "users") {
				return
			}

			userSet := entities["users"]
			assert.IsType(t, (*containers.Set)(nil), userSet)
		})

	})
}

func TestLocalDatabase(t *testing.T) {

	for _, inMemory := range []bool{true, false} {

		name := "in_memory"
		HOST := core.Host("ldb://main")

		if !inMemory {
			name = "filesystem"
		}

		setup := func(ctxHasTransaction bool) (*LocalDatabase, *core.Context, *core.Transaction) {
			//core.ResetResourceMap()

			config := LocalDatabaseConfig{
				InMemory: inMemory,
			}

			ctxConfig := core.ContextConfig{}

			if !inMemory {
				dir, _ := filepath.Abs(t.TempDir())
				dir += "/"
				pattern := core.PathPattern(dir + "...")

				ctxConfig = core.ContextConfig{
					Permissions: []core.Permission{
						core.FilesystemPermission{Kind_: permkind.Read, Entity: pattern},
						core.FilesystemPermission{Kind_: permkind.Create, Entity: pattern},
						core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: pattern},
					},
					HostResolutions: map[core.Host]core.Value{
						HOST: core.Path(dir),
					},
					Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
				}
				config.Host = HOST
				config.Path = core.Path(dir)
			}

			ctx := core.NewContexWithEmptyState(ctxConfig, nil)

			var tx *core.Transaction
			if ctxHasTransaction {
				tx = core.StartNewTransaction(ctx)
			}

			ldb, err := openLocalDatabaseWithConfig(ctx, config)
			assert.NoError(t, err)

			return ldb, ctx, tx
		}

		t.Run(name, func(t *testing.T) {
			t.Run("context has a transaction", func(t *testing.T) {
				ctxHasTransactionFromTheSart := true

				t.Run("Get non existing", func(t *testing.T) {
					ldb, ctx, tx := setup(ctxHasTransactionFromTheSart)
					defer ldb.Close(ctx)

					v, ok := ldb.Get(ctx, core.Path("/a"))
					assert.False(t, bool(ok))
					assert.Equal(t, core.Nil, v)

					assert.NoError(t, tx.Rollback(ctx))
				})

				t.Run("Set -> Get -> commit", func(t *testing.T) {
					ldb, ctx, tx := setup(ctxHasTransactionFromTheSart)
					defer ldb.Close(ctx)

					key := core.Path("/a")
					//r := ldb.GetFullResourceName(key)
					ldb.Set(ctx, key, core.Int(1))
					// if !assert.False(t, core.TryAcquireConcreteResource(r)) {
					// 	return
					// }

					v, ok := ldb.Get(ctx, key)
					assert.True(t, bool(ok))
					assert.Equal(t, core.Int(1), v)
					//assert.False(t, core.TryAcquireConcreteResource(r))

					// //we check that the database transaction is not commited yet
					// ldb.underlying.db.View(func(txn *Tx) error {
					// 	_, err := txn.Get(string(key))
					// 	assert.ErrorIs(t, err, errNotFound)
					// 	return nil
					// })

					assert.NoError(t, tx.Commit(ctx))
					// assert.True(t, core.TryAcquireConcreteResource(r))
					// core.ReleaseConcreteResource(r)

					//we check that the database transaction is commited
					otherCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
					v, ok, err := ldb.mainKV.Get(otherCtx, key, ldb)

					if !assert.NoError(t, err) {
						return
					}
					assert.True(t, bool(ok))
					assert.Equal(t, core.Int(1), v)
				})

				t.Run("Set -> rollback", func(t *testing.T) {
					ldb, ctx, tx := setup(ctxHasTransactionFromTheSart)
					defer ldb.Close(ctx)

					key := core.Path("/a")
					//r := ldb.GetFullResourceName(key)
					ldb.Set(ctx, key, core.Int(1))
					// if !assert.False(t, core.TryAcquireConcreteResource(r)) {
					// 	return
					// }

					v, ok := ldb.Get(ctx, key)
					assert.True(t, bool(ok))
					assert.Equal(t, core.Int(1), v)

					// //we check that the database transaction is not commited yet
					// ldb.underlying.db.View(func(txn *Tx) error {
					// 	_, err := txn.Get(string(key))
					// 	assert.ErrorIs(t, err, errNotFound)
					// 	return nil
					// })

					assert.NoError(t, tx.Rollback(ctx))
					// assert.True(t, core.TryAcquireConcreteResource(r))
					// core.ReleaseConcreteResource(r)

					// //we check that the database transaction is not commited
					// ldb.underlying.db.View(func(txn *Tx) error {
					// 	_, err := txn.Get(string(key))
					// 	assert.ErrorIs(t, err, errNotFound)
					// 	return nil
					// })

					//same
					v, ok = ldb.Get(ctx, key)
					//assert.True(t, core.TryAcquireConcreteResource(r))
					//core.ReleaseConcreteResource(r)
					assert.Equal(t, core.Nil, v)
					assert.False(t, bool(ok))
				})

			})

			t.Run("context has no transaction", func(t *testing.T) {
				ctxHasTransactionFromTheSart := false

				t.Run("Get non existing", func(t *testing.T) {
					ldb, ctx, _ := setup(ctxHasTransactionFromTheSart)
					defer ldb.Close(ctx)

					v, ok := ldb.Get(ctx, core.Path("/a"))
					assert.False(t, bool(ok))
					assert.Equal(t, core.Nil, v)
				})

				t.Run("Set then Get", func(t *testing.T) {
					ldb, ctx, _ := setup(ctxHasTransactionFromTheSart)
					defer ldb.Close(ctx)

					key := core.Path("/a")
					ldb.Set(ctx, key, core.Int(1))

					v, ok := ldb.Get(ctx, key)
					assert.True(t, bool(ok))
					assert.Equal(t, core.Int(1), v)

					//we check that the database transaction is commited
					otherCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

					v, ok, err := ldb.mainKV.Get(otherCtx, key, ldb)

					if !assert.NoError(t, err) {
						return
					}
					assert.True(t, bool(ok))
					assert.Equal(t, core.Int(1), v)
				})
			})

			t.Run("context gets transaction in the middle of the execution", func(t *testing.T) {
				ctxHasTransactionFromTheSart := false

				t.Run("Set with no tx then set with tx", func(t *testing.T) {
					ldb, ctx, _ := setup(ctxHasTransactionFromTheSart)
					defer ldb.Close(ctx)

					//first call to Set
					key := core.Path("/a")
					ldb.Set(ctx, key, core.Int(1))

					//attach transaction
					core.StartNewTransaction(ctx)

					//second call to Set
					ldb.Set(ctx, key, core.Int(2))

					v, ok := ldb.Get(ctx, key)
					assert.True(t, bool(ok))
					assert.Equal(t, core.Int(2), v)
				})
			})
		})
	}
}

func TestUpdateSchema(t *testing.T) {
	HOST := core.Host("ldb://main")

	openDB := func(tempdir string, filesystem afs.Filesystem) (*LocalDatabase, *core.Context, bool) {
		//core.ResetResourceMap()

		config := LocalDatabaseConfig{}

		dir, _ := filepath.Abs(tempdir)
		dir += "/"
		pattern := core.PathPattern(dir + "...")

		ctxConfig := core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: pattern},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: pattern},

				core.DatabasePermission{Kind_: permkind.Read, Entity: HOST},
				core.DatabasePermission{Kind_: permkind.Write, Entity: HOST},
			},
			HostResolutions: map[core.Host]core.Value{
				HOST: core.Path(dir),
			},
			Filesystem: filesystem,
		}
		config.Host = HOST
		config.Path = core.Path(dir)

		ctx := core.NewContexWithEmptyState(ctxConfig, nil)
		ctx.AddNamedPattern("int", core.INT_PATTERN)
		ctx.AddNamedPattern("str", core.STR_PATTERN)
		ctx.AddNamedPattern("Set", containers.SET_PATTERN)

		ldb, err := openLocalDatabaseWithConfig(ctx, config)
		if !assert.NoError(t, err) {
			return nil, nil, false
		}

		return ldb, ctx, true
	}

	t.Run("complex top-level entity", func(t *testing.T) {
		tempdir := t.TempDir()
		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

		ldb, ctx, ok := openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		setPattern :=
			utils.Must(containers.SET_PATTERN.CallImpl(containers.SET_PATTERN,
				[]core.Serializable{core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}), containers_common.URL_UNIQUENESS_IDENT}))

		schema := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		ldb.UpdateSchema(ctx, schema, core.MigrationOpHandlers{
			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
				"/users": {
					InitialValue: core.NewWrappedValueList(),
				},
			},
		})

		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))

		if !assert.Contains(t, topLevelValues, "users") {
			return
		}

		userSet := topLevelValues["users"]
		assert.IsType(t, (*containers.Set)(nil), userSet)
	})

	t.Run("call after TopLevelEntities() call", func(t *testing.T) {
		tempdir := t.TempDir()
		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

		ldb, ctx, ok := openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
		assert.Empty(t, topLevelValues)

		setPattern :=
			utils.Must(containers.SET_PATTERN.CallImpl(
				containers.SET_PATTERN,
				[]core.Serializable{
					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}), containers_common.URL_UNIQUENESS_IDENT,
				}))

		schema := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		assert.PanicsWithError(t, core.ErrTopLevelEntitiesAlreadyLoaded.Error(), func() {
			ldb.UpdateSchema(ctx, schema, core.MigrationOpHandlers{})
		})
	})

	t.Run("updating with the same schema should be ignored", func(t *testing.T) {

		tempdir := t.TempDir()
		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

		ldb, ctx, ok := openDB(tempdir, fls)
		if !ok {
			return
		}

		setPattern :=
			utils.Must(containers.SET_PATTERN.CallImpl(
				containers.SET_PATTERN,
				[]core.Serializable{
					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
					containers_common.URL_UNIQUENESS_IDENT,
				}),
			)

		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

		err := ldb.Close(ctx)
		if !assert.NoError(t, err) {
			return
		}

		//re open

		ldb, ctx, ok = openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		currentSchema := ldb.schema

		schemaCopy := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		ldb.UpdateSchema(ctx, schemaCopy, core.MigrationOpHandlers{})

		//should not have changed
		assert.Same(t, currentSchema, ldb.schema)
	})

	t.Run("top level entity removed during migration should not be present", func(t *testing.T) {
		tempdir := t.TempDir()
		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

		ldb, ctx, ok := openDB(tempdir, fls)
		if !ok {
			return
		}

		setPattern :=
			utils.Must(containers.SET_PATTERN.CallImpl(
				containers.SET_PATTERN,
				[]core.Serializable{
					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
					containers_common.URL_UNIQUENESS_IDENT,
				}),
			)

		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

		err := ldb.Close(ctx)
		if !assert.NoError(t, err) {
			return
		}

		//re open with next schema

		ldb, ctx, ok = openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		nextSchema := core.NewInexactObjectPattern(map[string]core.Pattern{})

		ldb.UpdateSchema(ctx, nextSchema, core.MigrationOpHandlers{
			Deletions: map[core.PathPattern]*core.MigrationOpHandler{
				"/users": nil,
			},
		})

		assert.Same(t, nextSchema, ldb.schema)
		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
		assert.NotContains(t, topLevelValues, "users")
	})

	t.Run("top level entity added during migration should be present", func(t *testing.T) {
		tempdir := t.TempDir()
		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

		ldb, ctx, ok := openDB(tempdir, fls)
		if !ok {
			return
		}

		setPattern :=
			utils.Must(containers.SET_PATTERN.CallImpl(
				containers.SET_PATTERN,
				[]core.Serializable{
					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
					containers_common.URL_UNIQUENESS_IDENT,
				}),
			)

		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{})

		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

		err := ldb.Close(ctx)
		if !assert.NoError(t, err) {
			return
		}

		//re open with next schema

		ldb, ctx, ok = openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		nextSchema := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		ldb.UpdateSchema(ctx, nextSchema, core.MigrationOpHandlers{
			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
				"/users": {
					InitialValue: core.NewWrappedValueList(),
				},
			},
		})

		assert.Same(t, nextSchema, ldb.schema)
		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
		assert.Contains(t, topLevelValues, "users")
	})

	t.Run("top level entity replacement added during migration should be present", func(t *testing.T) {
		tempdir := t.TempDir()
		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

		ldb, ctx, ok := openDB(tempdir, fls)
		if !ok {
			return
		}

		setPattern :=
			utils.Must(containers.SET_PATTERN.CallImpl(
				containers.SET_PATTERN,
				[]core.Serializable{
					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
					containers_common.URL_UNIQUENESS_IDENT,
				}),
			)

		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{})

		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

		err := ldb.Close(ctx)
		if !assert.NoError(t, err) {
			return
		}

		//re open with next schema (initial Set type)

		ldb, ctx, ok = openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		nextSchema1 := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern,
		})

		ldb.UpdateSchema(ctx, nextSchema1, core.MigrationOpHandlers{
			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
				"/users": {
					InitialValue: core.NewWrappedValueList(),
				},
			},
		})

		assert.Same(t, nextSchema1, ldb.schema)
		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
		if !assert.Contains(t, topLevelValues, "users") {
			return
		}
		users := topLevelValues["users"].(*containers.Set)
		users.Add(ctx, core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, ctx))

		//make sure the updated Set has been saved
		s, _ := ldb.GetSerialized(ctx, "/users")
		if !assert.Contains(t, s, "foo") {
			return
		}

		//re open with next schema (different Set type)

		ldb, ctx, ok = openDB(tempdir, fls)
		if !ok {
			return
		}
		defer ldb.Close(ctx)

		setPattern2 :=
			utils.Must(containers.SET_PATTERN.CallImpl(
				containers.SET_PATTERN,
				[]core.Serializable{core.INT_PATTERN, containers_common.URL_UNIQUENESS_IDENT}),
			)

		nextSchema2 := core.NewInexactObjectPattern(map[string]core.Pattern{
			"users": setPattern2,
		})

		ldb.UpdateSchema(ctx, nextSchema2, core.MigrationOpHandlers{
			Replacements: map[core.PathPattern]*core.MigrationOpHandler{
				"/users": {
					InitialValue: core.NewWrappedValueList(),
				},
			},
		})

		assert.Same(t, nextSchema2, ldb.schema)
		topLevelValues = utils.Must(ldb.LoadTopLevelEntities(ctx))
		if !assert.Contains(t, topLevelValues, "users") {
			return
		}

		//make sure the updated Set has been saved
		s, _ = ldb.GetSerialized(ctx, "/users")
		if assert.Contains(t, s, "foo") {
			return
		}
	})
}
