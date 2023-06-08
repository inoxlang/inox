package local_db_ns

import (
	"errors"
	"sync"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
)

const (
	SCHEMA_KEY = "/schema"

	MAIN_KV_FILE = "main.kv"
)

var (
	ErrInvalidDatabaseDirpath = errors.New("invalid database dir path")
	ErrDatabaseAlreadyOpen    = errors.New("database is already open")
	ErrCannotResolveDatabase  = errors.New("cannot resolve database")
	ErrCannotFindDatabaseHost = errors.New("cannot find corresponding host of database")
	ErrInvalidDatabaseHost    = errors.New("host of database is invalid")
	ErrInvalidPathKey         = errors.New("invalid path used as local database key")
	ErrDatabaseNotSupported   = errors.New("database is not supported")

	LOCAL_DB_PROPNAMES = []string{"close"}
)

// A LocalDatabase is a database thats stores data on the filesystem.
type LocalDatabase struct {
	host    Host
	dirPath core.Path
	mainKV  *SingleFileKV
	schema  *core.ObjectPattern
}

type LocalDatabaseConfig struct {
	Host     core.Host
	Path     core.Path
	InMemory bool
}

// openDatabase opens a local database, read, create & write permissions are required.
func openDatabase(ctx *Context, r core.ResourceName) (*LocalDatabase, error) {

	var pth Path

	switch resource := r.(type) {
	case Host:
		if resource.Scheme() != LDB_SCHEME {
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

	patt := PathPattern(pth.ToAbs(ctx.GetFileSystem()) + "...")

	for _, kind := range []core.PermissionKind{permkind.Read, permkind.Create, permkind.WriteStream} {
		perm := FilesystemPermission{Kind_: kind, Entity: patt}
		if err := ctx.CheckHasPermission(perm); err != nil {
			return nil, err
		}
	}

	host, ok := ctx.GetHostFromResolutionData(pth)
	if !ok {
		return nil, ErrCannotFindDatabaseHost
	}

	if host.Scheme() != LDB_SCHEME {
		return nil, ErrInvalidDatabaseHost
	}

	db, err := openLocalDatabaseWithConfig(ctx, LocalDatabaseConfig{
		Path: pth,
		Host: host,
	})

	return db, err
}

func openLocalDatabaseWithConfig(ctx *core.Context, config LocalDatabaseConfig) (*LocalDatabase, error) {
	mainKVPath := core.Path("")
	if config.InMemory {
		config.Path = ""
	} else {
		mainKVPath = config.Path.Join("./"+MAIN_KV_FILE, ctx.GetFileSystem())
	}

	kv, err := openKvWrapperNoPermCheck(KvStoreConfig{
		Host:     config.Host,
		Path:     mainKVPath,
		InMemory: config.InMemory,
	}, ctx.GetFileSystem())

	if err != nil {
		return nil, err
	}

	localDB := &LocalDatabase{
		host:    config.Host,
		dirPath: config.Path,
		mainKV:  kv,
	}

	return localDB, nil
}

func (ldb *LocalDatabase) Resource() core.SchemeHolder {
	return ldb.host
}

func (ldb *LocalDatabase) Close(ctx *core.Context) error {
	ldb.mainKV.close(ctx)
	return nil
}

func (ldb *LocalDatabase) Get(ctx *Context, key Path) (Value, Bool) {
	return ldb.mainKV.get(ctx, key, ldb)
}

func (ldb *LocalDatabase) Has(ctx *Context, key Path) Bool {
	return ldb.mainKV.has(ctx, key, ldb)
}

func (ldb *LocalDatabase) Set(ctx *Context, key Path, value Value) {
	ldb.mainKV.set(ctx, key, value, ldb)
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
	case "close":
		return core.WrapGoMethod(ldb.Close), true
	}
	return nil, false
}

func (ldb *LocalDatabase) PropertyNames(ctx *Context) []string {
	return LOCAL_DB_PROPNAMES
}

type databaseRegistry struct {
	lock          sync.Mutex
	resolutions   map[Host]Path
	openDatabases map[Path]*LocalDatabase
}
