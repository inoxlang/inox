package internal

import (
	"errors"
	"log"
	"sync"

	badger "github.com/dgraph-io/badger/v3"
	core "github.com/inox-project/inox/internal/core"
	internal "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

var (
	ErrInvalidDatabaseDirpath = errors.New("invalid database dir path")
	ErrDatabaseAlreadyOpen    = errors.New("database is already open")
	ErrCannotResolveDatabase  = errors.New("cannot resolve database")
	ErrDatabaseClosed         = errors.New("database is closed")
	ErrCannotFindDatabaseHost = errors.New("cannot find corresponding host of database")
	ErrInvalidDatabaseHost    = errors.New("host of database is invalid")
	ErrInvalidPathKey         = errors.New("invalid path used as local database key")

	dbRegistry = databaseRegistry{
		resolutions:   map[internal.Host]internal.Path{},
		openDatabases: map[internal.Path]*LocalDatabase{},
	}
)

func init() {
	core.RegisterSymbolicGoFunction(openDatabase, func(ctx *symbolic.Context, r symbolic.ResourceName) (*SymbolicLocalDatabase, *symbolic.Error) {
		return &SymbolicLocalDatabase{}, nil
	})
}

func NewLocalDbNamespace() *Record {
	return core.NewRecordFromMap(core.ValMap{
		//
		"open": internal.ValOf(openDatabase),
	})
}

// openDatabase opens a local database, read, create & write permissions are required.
func openDatabase(ctx *Context, r ResourceName) (*LocalDatabase, error) {

	var pth Path

	switch resource := r.(type) {
	case Host:
		if resource.Scheme() != "ldb" {
			return nil, ErrCannotResolveDatabase
		}
		data, ok := ctx.GetHostResolutionData(resource).(Path)
		if !ok {
			return nil, ErrCannotResolveDatabase
		}
		pth = data
	case Path:
		pth = resource
	default:
		return nil, ErrCannotResolveDatabase
	}

	if !pth.IsDirPath() {
		return nil, ErrInvalidDatabaseDirpath
	}

	patt := PathPattern(pth.ToAbs() + "...")

	for _, kind := range []core.PermissionKind{core.ReadPerm, core.CreatePerm, core.WritePerm} {
		perm := FilesystemPermission{Kind_: kind, Entity: patt}
		if err := ctx.CheckHasPermission(perm); err != nil {
			return nil, err
		}
	}

	host, ok := ctx.GetHostFromResolutionData(pth)
	if !ok {
		return nil, ErrCannotFindDatabaseHost
	}

	if host.Scheme() != "ldb" {
		return nil, ErrInvalidDatabaseHost
	}

	dbRegistry.lock.Lock()

	db, ok := dbRegistry.openDatabases[pth]
	if ok {
		dbRegistry.lock.Unlock()
		log.Println("db already exists !")
		return db, nil
	}

	defer func() {
		dbRegistry.lock.Unlock()
	}()
	db, err := NewLocalDatabase(LocalDatabaseConfig{Path: pth, Host: host})
	if err == nil {
		dbRegistry.openDatabases[pth] = db
		return db, nil
	}

	return nil, err
}

// A LocalDatabase is a database thats stores data on the filesystem.
type LocalDatabase struct {
	lock sync.RWMutex
	host Host
	path Path
	db   *badger.DB
}

type LocalDatabaseConfig struct {
	Host     Host
	Path     Path
	InMemory bool
}

func NewLocalDatabase(config LocalDatabaseConfig) (*LocalDatabase, error) {

	if config.InMemory {
		config.Path = ""
	}

	opts := badger.DefaultOptions(string(config.Path))

	if config.InMemory {
		opts = opts.WithInMemory(true)
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	localDB := &LocalDatabase{
		path: config.Path,
		db:   db,
	}

	return localDB, nil
}

func (ldb *LocalDatabase) Close() {
	ldb.db.Close()
	if ldb.db.IsClosed() {
		dbRegistry.lock.Lock()
		defer dbRegistry.lock.Unlock()

		delete(dbRegistry.openDatabases, ldb.path)
	}
}

func (ldb *LocalDatabase) Get(ctx *Context, key Path) (Value, Bool) {
	if ldb.db.IsClosed() {
		panic(ErrDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	var (
		r          = ldb.GetFullResourceName(key)
		valueFound = core.True
		val        Value
		b          []byte
		tx         = ctx.GetTx()
	)

	if tx == nil {
		err := ldb.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				valueFound = core.False
				return nil
			} else if err != nil {
				return err
			}

			_ = item.Value(func(val []byte) error {
				b = val
				return nil
			})

			val, err = internal.ParseRepr(ctx, b)
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := ldb.getCreateDatabaseTxn(tx)

		item, err := dbtx.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			valueFound = core.False
		} else if err != nil {
			panic(err)
		} else {
			_ = item.Value(func(val []byte) error {
				b = val
				return nil
			})

			val, err = internal.ParseRepr(ctx, b)

			if err != nil {
				panic(err)
			}
		}
	}

	if valueFound {
		if err := ctx.AcquireResource(r); err != nil {
			panic(err)
		}
	}

	if val == nil {
		val = core.Nil
	}

	return val, valueFound
}

func (ldb *LocalDatabase) Has(ctx *Context, key Path) Bool {
	if ldb.db.IsClosed() {
		panic(ErrDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	var (
		valueFound = core.True
		tx         = ctx.GetTx()
	)

	if tx == nil {
		err := ldb.db.View(func(txn *badger.Txn) error {
			_, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				valueFound = core.False
				return nil
			}
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := ldb.getCreateDatabaseTxn(tx)

		_, err := dbtx.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			valueFound = core.False
		} else if err != nil {
			panic(err)
		}

	}

	return valueFound
}

func (ldb *LocalDatabase) Set(ctx *Context, key Path, value Value) {

	if ldb.db.IsClosed() {
		panic(ErrDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	tx := ctx.GetTx()
	r := ldb.GetFullResourceName(key)

	if tx == nil {
		err := ldb.db.Update(func(txn *badger.Txn) error {
			repr := core.GetRepresentation(value, ctx)
			return txn.Set([]byte(key), []byte(repr))
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := ldb.getCreateDatabaseTxn(tx)

		repr := core.GetRepresentation(value, ctx)
		if err := dbtx.Set([]byte(key), []byte(repr)); err != nil {
			panic(err)
		}

	}

	if err := ctx.AcquireResource(r); err != nil {
		panic(err)
	}
}

func (ldb *LocalDatabase) getCreateDatabaseTxn(tx *core.Transaction) *badger.Txn {
	v, err := tx.GetValue(ldb)
	if err != nil {
		panic(err)
	}
	dbtx, ok := v.(*badger.Txn)

	if !ok {
		dbtx = ldb.db.NewTransaction(true)
		if err = tx.SetValue(ldb, dbtx); err != nil {
			panic(err)
		}

		if err = tx.OnEnd(ldb, makeTxEndcallbackFn(dbtx)); err != nil {
			panic(err)
		}
	}
	return dbtx
}

func (ldb *LocalDatabase) GetFullResourceName(pth Path) ResourceName {
	return URL(string(ldb.host) + string(pth))
}

func (ldb *LocalDatabase) Prop(ctx *core.Context, name string) Value {
	method, ok := ldb.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, ldb))
	}
	return method
}

func (*LocalDatabase) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (ldb *LocalDatabase) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "get":
		return core.WrapGoMethod(ldb.Get), true
	case "has":
		return core.WrapGoMethod(ldb.Has), true
	case "set":
		return core.WrapGoMethod(ldb.Set), true
	case "close":
		return core.WrapGoMethod(ldb.Close), true
	}
	return nil, false
}

func (ldb *LocalDatabase) PropertyNames(ctx *Context) []string {
	return []string{"get", "has", "set", "close"}
}

type databaseRegistry struct {
	lock          sync.Mutex
	resolutions   map[Host]Path
	openDatabases map[Path]*LocalDatabase
}

func makeTxEndcallbackFn(dbtx *badger.Txn) func(t *internal.Transaction, success bool) {
	return func(t *internal.Transaction, success bool) {
		defer dbtx.Discard()
		if success {
			dbtx.Commit()
		}
	}
}
