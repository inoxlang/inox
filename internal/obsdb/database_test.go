package obsdb

// import (
// 	"math/rand"
// 	"os"
// 	"strconv"
// 	"sync"
// 	"testing"

// 	"github.com/inoxlang/inox/internal/afs"
// 	"github.com/inoxlang/inox/internal/core"
// 	"github.com/inoxlang/inox/internal/core/permkind"
// 	"github.com/inoxlang/inox/internal/globals/containers"
// 	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
// 	"github.com/inoxlang/inox/internal/globals/fs_ns"
// 	"github.com/inoxlang/inox/internal/utils"
// 	"github.com/stretchr/testify/assert"
// )

// const (
// 	MEM_FS_STORAGE_SIZE = 100_000_000
// 	DB_HOST             = core.Host("odb://main")
// )

// var (
// 	OS_DB_TEST_ACCESS_KEY_ENV_VARNAME = "OS_DB_TEST_ACCESS_KEY"
// 	OS_DB_TEST_ACCESS_KEY             = os.Getenv(OS_DB_TEST_ACCESS_KEY_ENV_VARNAME)
// 	OS_DB_TEST_SECRET_KEY             = os.Getenv("OS_DB_TEST_SECRET_KEY")
// 	OS_DB_TEST_ENDPOINT               = os.Getenv("OS_DB_TEST_ENDPOINT")

// 	S3_HOST_RESOLUTION_DATA = core.NewObjectFromMapNoInit(core.ValMap{
// 		"bucket":     core.Str("test"),
// 		"host":       core.Host(OS_DB_TEST_ENDPOINT),
// 		"access-key": core.Str(OS_DB_TEST_ACCESS_KEY),
// 		"secret-key": utils.Must(core.SECRET_STRING_PATTERN.NewSecret(
// 			core.NewContexWithEmptyState(core.ContextConfig{}, nil),
// 			OS_DB_TEST_SECRET_KEY,
// 		)),
// 		"provider": core.Str("cloudflare"),
// 	})
// )

// func TestOpenDatabase(t *testing.T) {
// 	if OS_DB_TEST_ACCESS_KEY == "" {
// 		t.SkipNow()
// 	}

// 	t.Run("opening the same database (host) is forbidden", func(t *testing.T) {
// 		s3Host := randS3Host()

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST: s3Host,
// 				s3Host:  S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 		}

// 		ctx1 := core.NewContexWithEmptyState(ctxConfig, nil)
// 		project := &testProject{id: core.RandomProjectID("odb-test")}

// 		_db, err := openDatabase(ctx1, DB_HOST, false, project)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer _db.RemoveAllObjects(ctx1)
// 		defer _db.Close(ctx1)

// 		ctx2 := core.NewContexWithEmptyState(ctxConfig, nil)

// 		db, err := openDatabase(ctx2, DB_HOST, false, project)
// 		if !assert.ErrorIs(t, err, core.ErrDatabaseAlreadyOpen) {
// 			return
// 		}
// 		assert.Nil(t, db, _db)
// 	})

// 	t.Run("opening the same database (host) in different projects is allowed", func(t *testing.T) {
// 		s3Host := randS3Host()

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST: s3Host,
// 				s3Host:  S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 		}

// 		ctx1 := core.NewContexWithEmptyState(ctxConfig, nil)
// 		project1 := &testProject{id: core.RandomProjectID("odb-test-p1")}
// 		project2 := &testProject{id: core.RandomProjectID("odb-test-p2")}

// 		_db, err := openDatabase(ctx1, DB_HOST, false, project1)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer _db.RemoveAllObjects(ctx1)
// 		defer _db.Close(ctx1)

// 		ctx2 := core.NewContexWithEmptyState(ctxConfig, nil)

// 		db, err := openDatabase(ctx2, DB_HOST, false, project2)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer db.RemoveAllObjects(ctx2)
// 		defer db.Close(ctx2)
// 		assert.NotSame(t, db, _db)
// 	})

// 	t.Run("opening a database with the same S3 host is forbidden", func(t *testing.T) {
// 		const DB_HOST2 = core.Host("odb://other")

// 		s3Host := randS3Host()

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST:  s3Host,
// 				DB_HOST2: s3Host,
// 				s3Host:   S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 		}

// 		ctx1 := core.NewContexWithEmptyState(ctxConfig, nil)
// 		project := &testProject{id: core.RandomProjectID("odb-test")}

// 		_db, err := openDatabase(ctx1, DB_HOST, false, project)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer _db.RemoveAllObjects(ctx1)
// 		defer _db.Close(ctx1)

// 		ctx2 := core.NewContexWithEmptyState(ctxConfig, nil)

// 		db, err := openDatabase(ctx2, DB_HOST2, false, project)
// 		if !assert.ErrorIs(t, err, core.ErrDatabaseAlreadyOpen) {
// 			return
// 		}
// 		assert.Nil(t, db, _db)
// 	})

// 	t.Run("opening a database with the same S3 host is forbidden", func(t *testing.T) {
// 		//TODO
// 	})

// 	t.Run("open same database sequentially (in-between closing)", func(t *testing.T) {
// 		s3Host := randS3Host()

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST: s3Host,
// 				s3Host:  S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 		}
// 		project := &testProject{id: core.RandomProjectID("odb-test")}

// 		ctx1 := core.NewContexWithEmptyState(ctxConfig, nil)

// 		_db, err := openDatabase(ctx1, DB_HOST, false, project)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		_db.Close(ctx1)

// 		ctx2 := core.NewContexWithEmptyState(ctxConfig, nil)

// 		db, err := openDatabase(ctx2, DB_HOST, false, project)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer _db.RemoveAllObjects(ctx1)
// 		defer _db.Close(ctx1)

// 		assert.NotSame(t, db, _db)
// 	})

// 	t.Run("open same database in parallel should result in at least one error", func(t *testing.T) {
// 		//TODO when implemented.

// 		t.SkipNow()
// 		s3Host := randS3Host()

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST: s3Host,
// 				s3Host:  S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 		}
// 		project := &testProject{id: core.RandomProjectID("odb-test")}

// 		wg := new(sync.WaitGroup)
// 		wg.Add(2)

// 		var ctx1, ctx2 *core.Context
// 		var db1, db2 *ObjectStorageDatabase

// 		defer func() {
// 			if db1 != nil {
// 				db1.Close(ctx1)
// 				db1.RemoveAllObjects(ctx1)
// 			}
// 			if db2 != nil {
// 				db2.Close(ctx2)
// 				db2.RemoveAllObjects(ctx1)
// 			}
// 		}()

// 		go func() {
// 			defer wg.Done()

// 			//open database in first context
// 			ctx1 = core.NewContexWithEmptyState(ctxConfig, nil)

// 			_db1, err := openDatabase(ctx1, DB_HOST, false, project)
// 			if err != nil {
// 				return
// 			}
// 			db1 = _db1
// 		}()

// 		go func() {
// 			defer wg.Done()
// 			//open same database in second context
// 			ctx2 = core.NewContexWithEmptyState(ctxConfig, nil)

// 			_db2, err := openDatabase(ctx2, DB_HOST, false, project)
// 			if err != nil {
// 				return
// 			}
// 			db2 = _db2
// 		}()
// 		wg.Wait()

// 		assert.False(t, (db1 == nil) == (db2 == nil))
// 	})

// 	t.Run("re-open with a schema", func(t *testing.T) {

// 		t.Run("top-level Set with URL-based uniqueness", func(t *testing.T) {
// 			s3Host := randS3Host()

// 			ctxConfig := core.ContextConfig{
// 				Permissions: []core.Permission{},
// 				HostResolutions: map[core.Host]core.Value{
// 					DB_HOST: s3Host,
// 					s3Host:  S3_HOST_RESOLUTION_DATA,
// 				},
// 				Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 			}
// 			project := &testProject{id: core.RandomProjectID("odb-test")}

// 			ctx := core.NewContexWithEmptyState(ctxConfig, nil)
// 			ctx.AddNamedPattern("Set", containers.SET_PATTERN)
// 			ctx.AddNamedPattern("str", containers.SET_PATTERN)

// 			db, err := openDatabase(ctx, DB_HOST, false, project)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			setPattern :=
// 				utils.Must(containers.SET_PATTERN.CallImpl(
// 					containers.SET_PATTERN,
// 					[]core.Serializable{
// 						core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}), containers_common.URL_UNIQUENESS_IDENT,
// 					}),
// 				)

// 			schema := core.NewInexactObjectPattern(map[string]core.Pattern{
// 				"users": setPattern,
// 			})

// 			db.UpdateSchema(ctx, schema, core.MigrationOpHandlers{})

// 			err = db.Close(ctx)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			//re-open

// 			db, err = openDatabase(ctx, DB_HOST, false, project)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			defer db.RemoveAllObjects(ctx)
// 			defer db.Close(ctx)

// 			entities := utils.Must(db.LoadTopLevelEntities(ctx))

// 			if !assert.Contains(t, entities, "users") {
// 				return
// 			}

// 			userSet := entities["users"]
// 			assert.IsType(t, (*containers.Set)(nil), userSet)
// 		})

// 	})
// }

// func TestDatabase(t *testing.T) {
// 	if OS_DB_TEST_ACCESS_KEY == "" {
// 		t.SkipNow()
// 	}

// 	setup := func(ctxHasTransaction bool) (*ObjectStorageDatabase, *core.Context, *core.Transaction) {
// 		s3Host := randS3Host()
// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST: s3Host,
// 				s3Host:  S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE),
// 		}
// 		project := &testProject{id: core.RandomProjectID("odb-test")}

// 		ctx := core.NewContexWithEmptyState(ctxConfig, nil)

// 		var tx *core.Transaction
// 		if ctxHasTransaction {
// 			tx = core.StartNewTransaction(ctx)
// 		}

// 		odb, err := openDatabase(ctx, DB_HOST, false, project)
// 		assert.NoError(t, err)

// 		return odb, ctx, tx
// 	}

// 	t.Run("context has a transaction", func(t *testing.T) {
// 		ctxHasTransactionFromTheSart := true

// 		t.Run("Get non existing", func(t *testing.T) {
// 			odb, ctx, tx := setup(ctxHasTransactionFromTheSart)
// 			if odb == nil {
// 				return
// 			}
// 			defer odb.RemoveAllObjects(ctx)
// 			defer odb.Close(ctx)

// 			v, ok := odb.Get(ctx, core.Path("/a"))
// 			assert.False(t, bool(ok))
// 			assert.Equal(t, core.Nil, v)

// 			assert.NoError(t, tx.Rollback(ctx))
// 		})

// 		t.Run("Set -> Get -> commit", func(t *testing.T) {
// 			odb, ctx, tx := setup(ctxHasTransactionFromTheSart)
// 			if odb == nil {
// 				return
// 			}
// 			defer odb.RemoveAllObjects(ctx)
// 			defer odb.Close(ctx)

// 			key := core.Path("/a")
// 			//r := odb.GetFullResourceName(key)
// 			odb.Set(ctx, key, core.Int(1))
// 			// if !assert.False(t, core.TryAcquireConcreteResource(r)) {
// 			// 	return
// 			// }

// 			v, ok := odb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)
// 			//assert.False(t, core.TryAcquireConcreteResource(r))

// 			// //we check that the database transaction is not commited yet
// 			// odb.underlying.db.View(func(txn *Tx) error {
// 			// 	_, err := txn.Get(string(key))
// 			// 	assert.ErrorIs(t, err, errNotFound)
// 			// 	return nil
// 			// })

// 			assert.NoError(t, tx.Commit(ctx))
// 			// assert.True(t, core.TryAcquireConcreteResource(r))
// 			// core.ReleaseConcreteResource(r)

// 			//we check that the database transaction is commited
// 			otherCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
// 			v, ok, err := odb.mainKV.Get(otherCtx, key, odb)

// 			if !assert.NoError(t, err) {
// 				return
// 			}
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)
// 		})

// 		t.Run("Set -> rollback", func(t *testing.T) {
// 			odb, ctx, tx := setup(ctxHasTransactionFromTheSart)
// 			if odb == nil {
// 				return
// 			}
// 			defer odb.RemoveAllObjects(ctx)
// 			defer odb.Close(ctx)

// 			key := core.Path("/a")
// 			//r := odb.GetFullResourceName(key)
// 			odb.Set(ctx, key, core.Int(1))
// 			// if !assert.False(t, core.TryAcquireConcreteResource(r)) {
// 			// 	return
// 			// }

// 			v, ok := odb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)

// 			// //we check that the database transaction is not commited yet
// 			// odb.underlying.db.View(func(txn *Tx) error {
// 			// 	_, err := txn.Get(string(key))
// 			// 	assert.ErrorIs(t, err, errNotFound)
// 			// 	return nil
// 			// })

// 			assert.NoError(t, tx.Rollback(ctx))
// 			// assert.True(t, core.TryAcquireConcreteResource(r))
// 			// core.ReleaseConcreteResource(r)

// 			// //we check that the database transaction is not commited
// 			// odb.underlying.db.View(func(txn *Tx) error {
// 			// 	_, err := txn.Get(string(key))
// 			// 	assert.ErrorIs(t, err, errNotFound)
// 			// 	return nil
// 			// })

// 			//same
// 			v, ok = odb.Get(ctx, key)
// 			//assert.True(t, core.TryAcquireConcreteResource(r))
// 			//core.ReleaseConcreteResource(r)
// 			assert.Equal(t, core.Nil, v)
// 			assert.False(t, bool(ok))
// 		})

// 	})

// 	t.Run("context has no transaction", func(t *testing.T) {
// 		ctxHasTransactionFromTheSart := false

// 		t.Run("Get non existing", func(t *testing.T) {
// 			odb, ctx, _ := setup(ctxHasTransactionFromTheSart)
// 			if odb == nil {
// 				return
// 			}
// 			defer odb.RemoveAllObjects(ctx)
// 			defer odb.Close(ctx)

// 			v, ok := odb.Get(ctx, core.Path("/a"))
// 			assert.False(t, bool(ok))
// 			assert.Equal(t, core.Nil, v)
// 		})

// 		t.Run("Set then Get", func(t *testing.T) {
// 			odb, ctx, _ := setup(ctxHasTransactionFromTheSart)
// 			if odb == nil {
// 				return
// 			}
// 			defer odb.RemoveAllObjects(ctx)
// 			defer odb.Close(ctx)

// 			key := core.Path("/a")
// 			odb.Set(ctx, key, core.Int(1))

// 			v, ok := odb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)

// 			//we check that the database transaction is commited
// 			otherCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

// 			v, ok, err := odb.mainKV.Get(otherCtx, key, odb)

// 			if !assert.NoError(t, err) {
// 				return
// 			}
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)
// 		})
// 	})

// 	t.Run("context gets transaction in the middle of the execution", func(t *testing.T) {
// 		ctxHasTransactionFromTheSart := false

// 		t.Run("Set with no tx then set with tx", func(t *testing.T) {
// 			odb, ctx, _ := setup(ctxHasTransactionFromTheSart)
// 			if odb == nil {
// 				return
// 			}
// 			defer odb.RemoveAllObjects(ctx)
// 			defer odb.Close(ctx)

// 			//first call to Set
// 			key := core.Path("/a")
// 			odb.Set(ctx, key, core.Int(1))

// 			//attach transaction
// 			core.StartNewTransaction(ctx)

// 			//second call to Set
// 			odb.Set(ctx, key, core.Int(2))

// 			v, ok := odb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(2), v)
// 		})
// 	})
// }

// func TestUpdateSchema(t *testing.T) {
// 	if OS_DB_TEST_ACCESS_KEY == "" {
// 		t.SkipNow()
// 	}

// 	openDB := func(tempdir string, filesystem afs.Filesystem) (*ObjectStorageDatabase, *core.Context, bool) {
// 		//core.ResetResourceMap()
// 		s3Host := randS3Host()

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{
// 				core.DatabasePermission{Kind_: permkind.Read, Entity: DB_HOST},
// 			},
// 			HostResolutions: map[core.Host]core.Value{
// 				DB_HOST: s3Host,
// 				s3Host:  S3_HOST_RESOLUTION_DATA,
// 			},
// 			Filesystem: filesystem,
// 		}
// 		project := &testProject{id: core.RandomProjectID("odb-test")}

// 		ctx := core.NewContexWithEmptyState(ctxConfig, nil)
// 		ctx.AddNamedPattern("int", core.INT_PATTERN)
// 		ctx.AddNamedPattern("str", core.STR_PATTERN)
// 		ctx.AddNamedPattern("Set", containers.SET_PATTERN)

// 		odb, err := openDatabase(ctx, DB_HOST, false, project)
// 		if !assert.NoError(t, err) {
// 			return nil, nil, false
// 		}

// 		return odb, ctx, true
// 	}

// 	t.Run("complex top-level entity", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		odb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		setPattern :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(containers.SET_PATTERN,
// 				[]core.Serializable{core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}), containers_common.URL_UNIQUENESS_IDENT}))

// 		schema := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		odb.UpdateSchema(ctx, schema, core.MigrationOpHandlers{
// 			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		topLevelValues := utils.Must(odb.LoadTopLevelEntities(ctx))

// 		if !assert.Contains(t, topLevelValues, "users") {
// 			return
// 		}

// 		userSet := topLevelValues["users"]
// 		assert.IsType(t, (*containers.Set)(nil), userSet)
// 	})

// 	t.Run("call after TopLevelEntities() call", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		odb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		topLevelValues := utils.Must(odb.LoadTopLevelEntities(ctx))
// 		assert.Empty(t, topLevelValues)

// 		setPattern :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(
// 				containers.SET_PATTERN,
// 				[]core.Serializable{
// 					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}), containers_common.URL_UNIQUENESS_IDENT,
// 				}))

// 		schema := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		assert.PanicsWithError(t, core.ErrTopLevelEntitiesAlreadyLoaded.Error(), func() {
// 			odb.UpdateSchema(ctx, schema, core.MigrationOpHandlers{})
// 		})
// 	})

// 	t.Run("updating with the same schema should be ignored", func(t *testing.T) {

// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		odb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		setPattern :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(
// 				containers.SET_PATTERN,
// 				[]core.Serializable{
// 					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
// 					containers_common.URL_UNIQUENESS_IDENT,
// 				}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		odb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := odb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open

// 		odb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		currentSchema := odb.schema

// 		schemaCopy := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		odb.UpdateSchema(ctx, schemaCopy, core.MigrationOpHandlers{})

// 		//should not have changed
// 		assert.Same(t, currentSchema, odb.schema)
// 	})

// 	t.Run("top level entity removed during migration should not be present", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		odb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		setPattern :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(
// 				containers.SET_PATTERN,
// 				[]core.Serializable{
// 					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
// 					containers_common.URL_UNIQUENESS_IDENT,
// 				}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		odb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := odb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema

// 		odb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		nextSchema := core.NewInexactObjectPattern(map[string]core.Pattern{})

// 		odb.UpdateSchema(ctx, nextSchema, core.MigrationOpHandlers{
// 			Deletions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": nil,
// 			},
// 		})

// 		assert.Same(t, nextSchema, odb.schema)
// 		topLevelValues := utils.Must(odb.LoadTopLevelEntities(ctx))
// 		assert.NotContains(t, topLevelValues, "users")
// 	})

// 	t.Run("top level entity added during migration should be present", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		odb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		setPattern :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(
// 				containers.SET_PATTERN,
// 				[]core.Serializable{
// 					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
// 					containers_common.URL_UNIQUENESS_IDENT,
// 				}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{})

// 		odb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := odb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema

// 		odb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		nextSchema := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		odb.UpdateSchema(ctx, nextSchema, core.MigrationOpHandlers{
// 			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		assert.Same(t, nextSchema, odb.schema)
// 		topLevelValues := utils.Must(odb.LoadTopLevelEntities(ctx))
// 		assert.Contains(t, topLevelValues, "users")
// 	})

// 	t.Run("top level entity replacement added during migration should be present", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		odb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		setPattern :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(
// 				containers.SET_PATTERN,
// 				[]core.Serializable{
// 					core.NewInexactObjectPattern(map[string]core.Pattern{"name": core.STR_PATTERN}),
// 					containers_common.URL_UNIQUENESS_IDENT,
// 				}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern(map[string]core.Pattern{})

// 		odb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := odb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema (initial Set type)

// 		odb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		nextSchema1 := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern,
// 		})

// 		odb.UpdateSchema(ctx, nextSchema1, core.MigrationOpHandlers{
// 			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		assert.Same(t, nextSchema1, odb.schema)
// 		topLevelValues := utils.Must(odb.LoadTopLevelEntities(ctx))
// 		if !assert.Contains(t, topLevelValues, "users") {
// 			return
// 		}
// 		users := topLevelValues["users"].(*containers.Set)
// 		users.Add(ctx, core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, ctx))

// 		//make sure the updated Set has been saved
// 		s, _ := odb.GetSerialized(ctx, "/users")
// 		if !assert.Contains(t, s, "foo") {
// 			return
// 		}

// 		//re open with next schema (different Set type)

// 		odb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer odb.RemoveAllObjects(ctx)
// 		defer odb.Close(ctx)

// 		setPattern2 :=
// 			utils.Must(containers.SET_PATTERN.CallImpl(
// 				containers.SET_PATTERN,
// 				[]core.Serializable{core.INT_PATTERN, containers_common.URL_UNIQUENESS_IDENT}),
// 			)

// 		nextSchema2 := core.NewInexactObjectPattern(map[string]core.Pattern{
// 			"users": setPattern2,
// 		})

// 		odb.UpdateSchema(ctx, nextSchema2, core.MigrationOpHandlers{
// 			Replacements: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		assert.Same(t, nextSchema2, odb.schema)
// 		topLevelValues = utils.Must(odb.LoadTopLevelEntities(ctx))
// 		if !assert.Contains(t, topLevelValues, "users") {
// 			return
// 		}

// 		//make sure the updated Set has been saved
// 		s, _ = odb.GetSerialized(ctx, "/users")
// 		if assert.Contains(t, s, "foo") {
// 			return
// 		}
// 	})
// }

// func randS3Host() core.Host {
// 	return core.Host("s3://bucket-" + strconv.Itoa(int(rand.Int31())))
// }

// type testProject struct {
// 	id core.ProjectID
// }

// func (p *testProject) Id() core.ProjectID {
// 	return p.id
// }

// func (*testProject) GetSecrets(ctx *core.Context) ([]core.ProjectSecret, error) {
// 	panic("unimplemented")
// }

// func (*testProject) ListSecrets(ctx *core.Context) ([]core.ProjectSecretInfo, error) {
// 	panic("unimplemented")
// }

// func (p *testProject) BaseImage() (core.Image, error) {
// 	return nil, core.ErrNotImplemented
// }

// func (p *testProject) Configuration() core.ProjectConfiguration {
// 	panic("unimplemented")
// }

// func (*testProject) GetS3CredentialsForBucket(
// 	ctx *core.Context,
// 	bucketName string,
// 	provider string,
// ) (accessKey string, secretKey string, _ core.Host, _ error) {
// 	panic(core.ErrNotImplemented)
// }

// func (*testProject) CanProvideS3Credentials(s3Provider string) (bool, error) {
// 	return false, nil
// }
