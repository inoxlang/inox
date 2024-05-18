//go:build unix

package localdb

// import (
// 	"path/filepath"
// 	"testing"
// 	"time"

// 	"github.com/inoxlang/inox/internal/core"
// 	"github.com/inoxlang/inox/internal/core/permbase"
// 	_ "github.com/inoxlang/inox/internal/globals/containers"
// 	"github.com/inoxlang/inox/internal/globals/containers/common"
// 	"github.com/inoxlang/inox/internal/globals/containers/setcoll"
// 	"github.com/inoxlang/inox/internal/globals/fs_ns"
// 	"github.com/inoxlang/inox/internal/project"
// 	"github.com/inoxlang/inox/internal/utils"
// 	"github.com/stretchr/testify/assert"
// )

// const MEM_FS_STORAGE_SIZE = 100_000_000

// func TestOpenDatabase(t *testing.T) {
// 	const HOST = core.Host("ldb://main")

// 	projectsDir := t.TempDir()

// 	registryCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
// 	defer registryCtx.CancelGracefully()
// 	projectRegistry, err := project.OpenRegistry(projectsDir, registryCtx)

// 	if !assert.NoError(t, err) {
// 		return
// 	}

// 	createProject := func(t *testing.T) (proj core.Project, memberAuthToken string) {
// 		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
// 			Name: "test-project",
// 		})

// 		if !assert.NoError(t, err) {
// 			t.FailNow()
// 			return
// 		}

// 		p, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
// 		if !assert.NoError(t, err) {
// 			t.FailNow()
// 		}
// 		return p, string(memberId)
// 	}

// 	t.Run("opening the same database is forbidden", func(t *testing.T) {
// 		project, memberAuthToken := createProject(t)

// 		//Open database

// 		ctxConfig := core.ContextConfig{
// 			HostDefinitions: map[core.Host]core.Value{
// 				core.Host("ldb://main"): HOST,
// 			},
// 		}

// 		ctx1 := core.NewContextWithEmptyState(ctxConfig, nil)
// 		state1 := ctx1.MustGetClosestState()
// 		state1.Project = project
// 		state1.MemberAuthToken = memberAuthToken

// 		_db, err := OpenDatabase(ctx1, HOST, false)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer _db.Close(ctx1)

// 		//Open the same database without closing.

// 		ctx2 := core.NewContextWithEmptyState(ctxConfig, nil)
// 		state2 := ctx2.MustGetClosestState()
// 		state2.Project = project
// 		state2.MemberAuthToken = memberAuthToken

// 		db, err := OpenDatabase(ctx2, HOST, false)
// 		if !assert.ErrorIs(t, err, ErrOpenDatabase) {
// 			return
// 		}
// 		assert.NotSame(t, db, _db)
// 	})

// 	t.Run("open same database sequentially (in-between closing)", func(t *testing.T) {
// 		project, memberAuthToken := createProject(t)

// 		//Open database

// 		ctxConfig := core.ContextConfig{
// 			HostDefinitions: map[core.Host]core.Value{
// 				core.Host("ldb://main"): HOST,
// 			},
// 		}

// 		ctx1 := core.NewContextWithEmptyState(ctxConfig, nil)
// 		state1 := ctx1.MustGetClosestState()
// 		state1.Project = project
// 		state1.MemberAuthToken = memberAuthToken

// 		_db, err := OpenDatabase(ctx1, HOST, false)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//Close database

// 		_db.Close(ctx1)

// 		ctx2 := core.NewContextWithEmptyState(ctxConfig, nil)
// 		state2 := ctx2.MustGetClosestState()
// 		state2.Project = project
// 		state2.MemberAuthToken = memberAuthToken

// 		//Open database

// 		db, err := OpenDatabase(ctx2, HOST, false)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		defer _db.Close(ctx1)

// 		assert.NotSame(t, db, _db)
// 	})

// 	t.Run("open same database in parallel should result in at least one error", func(t *testing.T) {
// 		project, memberAuthToken := createProject(t)

// 		ctxConfig := core.ContextConfig{
// 			HostDefinitions: map[core.Host]core.Value{
// 				core.Host("ldb://main"): HOST,
// 			},
// 		}

// 		db1Open := make(chan struct{})

// 		var ctx1, ctx2 *core.Context
// 		var db1, db2 *LocalDatabase

// 		defer func() {
// 			if db1 != nil {
// 				db1.Close(ctx1)
// 			}
// 			if db2 != nil {
// 				db2.Close(ctx2)
// 			}
// 		}()

// 		go func() {
// 			defer func() {
// 				db1Open <- struct{}{}
// 			}()

// 			//open database in first context
// 			ctx1 = core.NewContextWithEmptyState(ctxConfig, nil)
// 			state1 := ctx1.MustGetClosestState()
// 			state1.Project = project
// 			state1.MemberAuthToken = memberAuthToken

// 			_db1, err := OpenDatabase(ctx1, HOST, false)
// 			if !assert.NoError(t, err) {
// 				return
// 			}
// 			db1 = _db1
// 		}()

// 		select {
// 		case <-db1Open:
// 		case <-time.After(time.Second):
// 			assert.Fail(t, "timeout")
// 			return
// 		}

// 		//open same database in second context
// 		ctx2 = core.NewContextWithEmptyState(ctxConfig, nil)
// 		state2 := ctx2.MustGetClosestState()
// 		state2.Project = project
// 		state2.MemberAuthToken = memberAuthToken

// 		_db2, err := OpenDatabase(ctx2, HOST, false)
// 		if !assert.ErrorIs(t, err, ErrOpenDatabase) {
// 			return
// 		}
// 		db2 = _db2
// 	})

// 	t.Run("open same database in parallel with different access mode", func(t *testing.T) {
// 		project, memberAuthToken := createProject(t)

// 		ctxConfig := core.ContextConfig{
// 			HostDefinitions: map[core.Host]core.Value{
// 				core.Host("ldb://main"): HOST,
// 			},
// 		}

// 		db1Open := make(chan struct{})

// 		var ctx1, ctx2 *core.Context
// 		var db1, db2 *LocalDatabase

// 		defer func() {
// 			if db1 != nil {
// 				db1.Close(ctx1)
// 			}
// 			if db2 != nil {
// 				db2.Close(ctx2)
// 			}
// 		}()

// 		schema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "a",
// 				Pattern: core.INT_PATTERN,
// 			},
// 		})

// 		go func() {
// 			defer func() {
// 				db1Open <- struct{}{}
// 			}()

// 			//open database in first context
// 			ctx1 = core.NewContextWithEmptyState(ctxConfig, nil)
// 			state1 := ctx1.MustGetClosestState()
// 			state1.Project = project
// 			state1.MemberAuthToken = memberAuthToken

// 			ctx1.AddNamedPattern("int", core.INT_PATTERN)

// 			_db1, err := OpenDatabase(ctx1, HOST, false)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			//set schema
// 			_db1.UpdateSchema(ctx1, schema, core.MigrationOpHandlers{})

// 			db1 = _db1
// 		}()

// 		select {
// 		case <-db1Open:
// 		case <-time.After(time.Second):
// 			assert.Fail(t, "timeout")
// 			return
// 		}

// 		//open same database in second context but in restricted mode
// 		ctx2 = core.NewContextWithEmptyState(ctxConfig, nil)
// 		state2 := ctx2.MustGetClosestState()
// 		state2.Project = project
// 		state2.MemberAuthToken = memberAuthToken

// 		ctx2.AddNamedPattern("int", core.INT_PATTERN)

// 		_db2, err := OpenDatabase(ctx2, HOST, true /*restricted access*/)
// 		if !assert.NoError(t, err) {
// 			return
// 		}
// 		db2 = _db2
// 		if !assert.NotSame(t, db1, db2) {
// 			return
// 		}

// 		schemaVisibleByDB2 := db2.Schema()
// 		assert.True(t, schemaVisibleByDB2.Equal(ctx2, schema, map[uintptr]uintptr{}, 0))
// 	})

// 	t.Run("re-open with a schema", func(t *testing.T) {

// 		t.Run("top-level Set with URL-based uniqueness", func(t *testing.T) {
// 			project, memberAuthToken := createProject(t)

// 			ctxConfig := core.ContextConfig{
// 				HostDefinitions: map[core.Host]core.Value{
// 					core.Host("ldb://main"): HOST,
// 				},
// 			}

// 			ctx := core.NewContextWithEmptyState(ctxConfig, nil)
// 			ctx.AddNamedPattern("Set", setcoll.SET_PATTERN)
// 			ctx.AddNamedPattern("str", setcoll.SET_PATTERN)
// 			state1 := ctx.MustGetClosestState()
// 			state1.Project = project
// 			state1.MemberAuthToken = memberAuthToken

// 			db, err := OpenDatabase(ctx, HOST, false)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 				{
// 					Name:    "name",
// 					Pattern: core.STR_PATTERN,
// 				},
// 			})

// 			setPattern :=
// 				utils.Must(setcoll.SET_PATTERN.CallImpl(
// 					ctx,
// 					setcoll.SET_PATTERN,
// 					[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}),
// 				)

// 			schema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 			db.UpdateSchema(ctx, schema, core.MigrationOpHandlers{})

// 			err = db.Close(ctx)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			//re-open

// 			db, err = OpenDatabase(ctx, HOST, false)
// 			if !assert.NoError(t, err) {
// 				return
// 			}

// 			defer db.Close(ctx)

// 			entities := utils.Must(db.LoadTopLevelEntities(ctx))

// 			if !assert.Contains(t, entities, "users") {
// 				return
// 			}

// 			userSet := entities["users"]
// 			assert.IsType(t, (*setcoll.Set)(nil), userSet)
// 		})

// 	})
// }

// func TestLocalDatabase(t *testing.T) {

// 	const HOST = core.Host("ldb://main")

// 	setup := func(ctxHasTransaction bool) (*LocalDatabase, *core.Context, *core.Transaction) {
// 		//core.ResetResourceMap()
// 		osDir, _ := filepath.Abs(t.TempDir())
// 		osDir += "/"
// 		config := LocalDatabaseConfig{}
// 		project := project.NewDummyProject("proj", fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE))

// 		ctxConfig := core.ContextConfig{
// 			HostDefinitions: map[core.Host]core.Value{
// 				HOST: HOST,
// 			},
// 		}
// 		config.Host = HOST
// 		config.OsFsDir = core.DirPathFrom(osDir)

// 		ctx := core.NewContextWithEmptyState(ctxConfig, nil)
// 		ctx.MustGetClosestState().Project = project

// 		var tx *core.Transaction
// 		if ctxHasTransaction {
// 			tx = core.StartNewTransaction(ctx)
// 		}

// 		ldb, err := openLocalDatabaseWithConfig(ctx, config)
// 		assert.NoError(t, err)

// 		return ldb, ctx, tx
// 	}

// 	t.Run("context has a transaction", func(t *testing.T) {
// 		ctxHasTransactionFromTheSart := true

// 		t.Run("Get non existing", func(t *testing.T) {
// 			ldb, ctx, tx := setup(ctxHasTransactionFromTheSart)
// 			defer ldb.Close(ctx)

// 			v, ok := ldb.Get(ctx, core.Path("/a"))
// 			assert.False(t, bool(ok))
// 			assert.Equal(t, core.Nil, v)

// 			assert.NoError(t, tx.Rollback(ctx))
// 		})

// 		t.Run("Set -> Get -> commit", func(t *testing.T) {
// 			ldb, ctx, tx := setup(ctxHasTransactionFromTheSart)
// 			defer ldb.Close(ctx)

// 			key := core.Path("/a")
// 			//r := ldb.GetFullResourceName(key)
// 			ldb.Set(ctx, key, core.Int(1))
// 			// if !assert.False(t, core.TryAcquireConcreteResource(r)) {
// 			// 	return
// 			// }

// 			v, ok := ldb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)
// 			//assert.False(t, core.TryAcquireConcreteResource(r))

// 			// //we check that the database transaction is not commited yet
// 			// ldb.underlying.db.View(func(txn *Tx) error {
// 			// 	_, err := txn.Get(string(key))
// 			// 	assert.ErrorIs(t, err, errNotFound)
// 			// 	return nil
// 			// })

// 			assert.NoError(t, tx.Commit(ctx))
// 			// assert.True(t, core.TryAcquireConcreteResource(r))
// 			// core.ReleaseConcreteResource(r)

// 			//we check that the database transaction is commited
// 			otherCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
// 			v, ok, err := ldb.mainKV.Get(otherCtx, key, ldb)

// 			if !assert.NoError(t, err) {
// 				return
// 			}
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)
// 		})

// 		t.Run("Set -> rollback", func(t *testing.T) {
// 			ldb, ctx, tx := setup(ctxHasTransactionFromTheSart)
// 			defer ldb.Close(ctx)

// 			key := core.Path("/a")
// 			//r := ldb.GetFullResourceName(key)
// 			ldb.Set(ctx, key, core.Int(1))
// 			// if !assert.False(t, core.TryAcquireConcreteResource(r)) {
// 			// 	return
// 			// }

// 			v, ok := ldb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)

// 			// //we check that the database transaction is not commited yet
// 			// ldb.underlying.db.View(func(txn *Tx) error {
// 			// 	_, err := txn.Get(string(key))
// 			// 	assert.ErrorIs(t, err, errNotFound)
// 			// 	return nil
// 			// })

// 			assert.NoError(t, tx.Rollback(ctx))
// 			// assert.True(t, core.TryAcquireConcreteResource(r))
// 			// core.ReleaseConcreteResource(r)

// 			// //we check that the database transaction is not commited
// 			// ldb.underlying.db.View(func(txn *Tx) error {
// 			// 	_, err := txn.Get(string(key))
// 			// 	assert.ErrorIs(t, err, errNotFound)
// 			// 	return nil
// 			// })

// 			//same
// 			v, ok = ldb.Get(ctx, key)
// 			//assert.True(t, core.TryAcquireConcreteResource(r))
// 			//core.ReleaseConcreteResource(r)
// 			assert.Equal(t, core.Nil, v)
// 			assert.False(t, bool(ok))
// 		})

// 	})

// 	t.Run("context has no transaction", func(t *testing.T) {
// 		ctxHasTransactionFromTheSart := false

// 		t.Run("Get non existing", func(t *testing.T) {
// 			ldb, ctx, _ := setup(ctxHasTransactionFromTheSart)
// 			defer ldb.Close(ctx)

// 			v, ok := ldb.Get(ctx, core.Path("/a"))
// 			assert.False(t, bool(ok))
// 			assert.Equal(t, core.Nil, v)
// 		})

// 		t.Run("Set then Get", func(t *testing.T) {
// 			ldb, ctx, _ := setup(ctxHasTransactionFromTheSart)
// 			defer ldb.Close(ctx)

// 			key := core.Path("/a")
// 			ldb.Set(ctx, key, core.Int(1))

// 			v, ok := ldb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(1), v)

// 			//we check that the database transaction is commited
// 			otherCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

// 			v, ok, err := ldb.mainKV.Get(otherCtx, key, ldb)

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
// 			ldb, ctx, _ := setup(ctxHasTransactionFromTheSart)
// 			defer ldb.Close(ctx)

// 			//first call to Set
// 			key := core.Path("/a")
// 			ldb.Set(ctx, key, core.Int(1))

// 			//attach transaction
// 			core.StartNewTransaction(ctx)

// 			//second call to Set
// 			ldb.Set(ctx, key, core.Int(2))

// 			v, ok := ldb.Get(ctx, key)
// 			assert.True(t, bool(ok))
// 			assert.Equal(t, core.Int(2), v)
// 		})
// 	})
// }

// func TestUpdateSchema(t *testing.T) {
// 	HOST := core.Host("ldb://main")

// 	openDB := func(tempdir string, filesystem core.SnapshotableFilesystem) (*LocalDatabase, *core.Context, bool) {
// 		//core.ResetResourceMap()

// 		config := LocalDatabaseConfig{}

// 		dir, _ := filepath.Abs(tempdir)
// 		dir += "/"
// 		pattern := core.PathPattern(dir + "...")
// 		project := project.NewDummyProject("proj", filesystem)

// 		ctxConfig := core.ContextConfig{
// 			Permissions: []core.Permission{
// 				core.FilesystemPermission{Kind_: permbase.Read, Entity: pattern},
// 				core.FilesystemPermission{Kind_: permbase.Create, Entity: pattern},
// 				core.FilesystemPermission{Kind_: permbase.WriteStream, Entity: pattern},

// 				core.DatabasePermission{Kind_: permbase.Read, Entity: HOST},
// 				core.DatabasePermission{Kind_: permbase.Write, Entity: HOST},
// 			},
// 			HostDefinitions: map[core.Host]core.Value{
// 				HOST: HOST,
// 			},
// 			Filesystem: filesystem,
// 		}
// 		config.Host = HOST
// 		config.OsFsDir = core.DirPathFrom(filepath.Join(tempdir, "data"))

// 		ctx := core.NewContextWithEmptyState(ctxConfig, nil)
// 		ctx.AddNamedPattern("int", core.INT_PATTERN)
// 		ctx.AddNamedPattern("str", core.STR_PATTERN)
// 		ctx.AddNamedPattern("Set", setcoll.SET_PATTERN)
// 		ctx.MustGetClosestState().Project = project

// 		ldb, err := openLocalDatabaseWithConfig(ctx, config)
// 		if !assert.NoError(t, err) {
// 			return nil, nil, false
// 		}

// 		return ldb, ctx, true
// 	}

// 	t.Run("complex top-level entity", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		ldb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "name",
// 				Pattern: core.STR_PATTERN,
// 			},
// 		})

// 		setPattern :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx, setcoll.SET_PATTERN,
// 				[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}))

// 		schema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 		ldb.UpdateSchema(ctx, schema, core.MigrationOpHandlers{
// 			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))

// 		if !assert.Contains(t, topLevelValues, "users") {
// 			return
// 		}

// 		userSet := topLevelValues["users"]
// 		assert.IsType(t, (*setcoll.Set)(nil), userSet)
// 	})

// 	t.Run("call after TopLevelEntities() call", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		ldb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
// 		assert.Empty(t, topLevelValues)

// 		namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "name",
// 				Pattern: core.STR_PATTERN,
// 			},
// 		})

// 		setPattern :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx,
// 				setcoll.SET_PATTERN,
// 				[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}))

// 		schema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 		assert.PanicsWithError(t, core.ErrTopLevelEntitiesAlreadyLoaded.Error(), func() {
// 			ldb.UpdateSchema(ctx, schema, core.MigrationOpHandlers{})
// 		})
// 	})

// 	t.Run("updating with the same schema should be ignored", func(t *testing.T) {

// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		ldb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "name",
// 				Pattern: core.STR_PATTERN,
// 			},
// 		})

// 		setPattern :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx,
// 				setcoll.SET_PATTERN,
// 				[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := ldb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open

// 		ldb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		currentSchema := ldb.schema

// 		schemaCopy := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "users",
// 				Pattern: setPattern,
// 			},
// 		})

// 		ldb.UpdateSchema(ctx, schemaCopy, core.MigrationOpHandlers{})

// 		//should not have changed
// 		assert.Same(t, currentSchema, ldb.schema)
// 	})

// 	t.Run("top level entity removed during migration should not be present", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		ldb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "name",
// 				Pattern: core.STR_PATTERN,
// 			},
// 		})

// 		setPattern :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx,
// 				setcoll.SET_PATTERN,
// 				[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := ldb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema

// 		ldb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		nextSchema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{})

// 		ldb.UpdateSchema(ctx, nextSchema, core.MigrationOpHandlers{
// 			Deletions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": nil,
// 			},
// 		})

// 		assert.Same(t, nextSchema, ldb.schema)
// 		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
// 		assert.NotContains(t, topLevelValues, "users")
// 	})

// 	t.Run("top level entity added during migration should be present", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		ldb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "name",
// 				Pattern: core.STR_PATTERN,
// 			},
// 		})

// 		setPattern :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx,
// 				setcoll.SET_PATTERN,
// 				[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{})

// 		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := ldb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema

// 		ldb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		nextSchema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 		ldb.UpdateSchema(ctx, nextSchema, core.MigrationOpHandlers{
// 			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		assert.Same(t, nextSchema, ldb.schema)
// 		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
// 		assert.Contains(t, topLevelValues, "users")
// 	})

// 	t.Run("top level entity replacement added during migration should be present", func(t *testing.T) {
// 		tempdir := t.TempDir()
// 		fls := fs_ns.NewMemFilesystem(MEM_FS_STORAGE_SIZE)

// 		ldb, ctx, ok := openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}

// 		namedObjectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
// 			{
// 				Name:    "name",
// 				Pattern: core.STR_PATTERN,
// 			},
// 		})

// 		setPattern :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx,
// 				setcoll.SET_PATTERN,
// 				[]core.Serializable{namedObjectPattern, common.URL_UNIQUENESS_IDENT}),
// 			)

// 		initialSchema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{})

// 		ldb.UpdateSchema(ctx, initialSchema, core.MigrationOpHandlers{})

// 		err := ldb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema (initial Set type)

// 		ldb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		nextSchema1 := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern}})

// 		ldb.UpdateSchema(ctx, nextSchema1, core.MigrationOpHandlers{
// 			Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		assert.Same(t, nextSchema1, ldb.schema)
// 		topLevelValues := utils.Must(ldb.LoadTopLevelEntities(ctx))
// 		if !assert.Contains(t, topLevelValues, "users") {
// 			return
// 		}
// 		users := topLevelValues["users"].(*setcoll.Set)
// 		users.Add(ctx, core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, ctx))

// 		//make sure the updated Set has been saved
// 		s, _ := ldb.GetSerialized(ctx, "/users")
// 		if !assert.Contains(t, s, "foo") {
// 			return
// 		}

// 		err = ldb.Close(ctx)
// 		if !assert.NoError(t, err) {
// 			return
// 		}

// 		//re open with next schema (different Set type)

// 		ldb, ctx, ok = openDB(tempdir, fls)
// 		if !ok {
// 			return
// 		}
// 		defer ldb.Close(ctx)

// 		setPattern2 :=
// 			utils.Must(setcoll.SET_PATTERN.CallImpl(
// 				ctx,
// 				setcoll.SET_PATTERN,
// 				[]core.Serializable{core.INT_PATTERN, common.URL_UNIQUENESS_IDENT}),
// 			)

// 		nextSchema2 := core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "users", Pattern: setPattern2}})

// 		ldb.UpdateSchema(ctx, nextSchema2, core.MigrationOpHandlers{
// 			Replacements: map[core.PathPattern]*core.MigrationOpHandler{
// 				"/users": {
// 					InitialValue: core.NewWrappedValueList(),
// 				},
// 			},
// 		})

// 		assert.Same(t, nextSchema2, ldb.schema)
// 		topLevelValues = utils.Must(ldb.LoadTopLevelEntities(ctx))
// 		if !assert.Contains(t, topLevelValues, "users") {
// 			return
// 		}

// 		//make sure the updated Set has been saved
// 		s, _ = ldb.GetSerialized(ctx, "/users")
// 		if assert.Contains(t, s, "foo") {
// 			return
// 		}
// 	})
// }
