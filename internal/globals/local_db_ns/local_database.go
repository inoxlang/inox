package local_db_ns

import (
	"errors"
	"fmt"
	"sync"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/rs/zerolog"
)

const (
	SCHEMA_KEY = "/_schema_"

	MAIN_KV_FILE   = "main.kv"
	SCHEMA_KV_FILE = "schema.kv"
)

var (
	ErrInvalidDatabaseDirpath = errors.New("invalid database dir path")
	ErrDatabaseAlreadyOpen    = errors.New("database is already open")
	ErrCannotResolveDatabase  = errors.New("cannot resolve database")
	ErrCannotFindDatabaseHost = errors.New("cannot find corresponding host of database")
	ErrInvalidDatabaseHost    = errors.New("host of database is invalid")
	ErrInvalidPathKey         = errors.New("invalid path used as local database key")
	ErrDatabaseNotSupported   = errors.New("database is not supported")

	LOCAL_DB_PROPNAMES = []string{"update_schema", "close"}

	_ core.Database = (*LocalDatabase)(nil)
)

// A LocalDatabase is a database thats stores data on the filesystem.
type LocalDatabase struct {
	host     Host
	dirPath  core.Path
	mainKV   *SingleFileKV
	schemaKV *SingleFileKV
	schema   *core.ObjectPattern
	logger   zerolog.Logger
}

type LocalDatabaseConfig struct {
	Host       core.Host
	Path       core.Path
	InMemory   bool
	Restricted bool
}

// openDatabase opens a local database, read, create & write permissions are required.
func openDatabase(ctx *Context, r core.ResourceName, restrictedAccess bool) (*LocalDatabase, error) {

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
		Path:       pth,
		Host:       host,
		Restricted: restrictedAccess,
	})

	return db, err
}

func openLocalDatabaseWithConfig(ctx *core.Context, config LocalDatabaseConfig) (*LocalDatabase, error) {
	mainKVPath := core.Path("")
	schemaKVPath := core.Path("")

	fls := ctx.GetFileSystem()

	if config.InMemory {
		config.Path = ""
	} else {
		mainKVPath = config.Path.Join("./"+MAIN_KV_FILE, fls)
		schemaKVPath = config.Path.Join("./"+SCHEMA_KV_FILE, fls)
	}

	localDB := &LocalDatabase{
		host:    config.Host,
		dirPath: config.Path,
	}

	if !config.Restricted {
		mainKv, err := openSingleFileKV(KvStoreConfig{
			Host:       config.Host,
			Path:       mainKVPath,
			InMemory:   config.InMemory,
			Filesystem: fls,
		})

		if err != nil {
			return nil, err
		}

		localDB.mainKV = mainKv
	}

	schemaKv, err := openSingleFileKV(KvStoreConfig{
		Host:       config.Host,
		Path:       schemaKVPath,
		InMemory:   config.InMemory,
		Filesystem: fls,
	})

	if err != nil {
		return nil, err
	}

	schema, ok := schemaKv.get(ctx, "/", localDB)
	if ok {
		patt, ok := schema.(*ObjectPattern)
		if !ok {
			err := localDB.Close(ctx)
			if err != nil {
				return nil, fmt.Errorf("schema is present but is not an object pattern, close db: %w", err)
			}
			return nil, fmt.Errorf("schema is present but is not an object pattern, close db")
		}
		localDB.schema = patt
	} else {
		localDB.schema = core.NewInexactObjectPattern(map[string]core.Pattern{})
	}

	return localDB, nil
}

func (ldb *LocalDatabase) Resource() core.SchemeHolder {
	return ldb.host
}

func (ldb *LocalDatabase) TopLevelEntities() map[string]Value {
	return nil
}

func (ldb *LocalDatabase) Schema() *core.ObjectPattern {
	return ldb.schema
}

func (ldb *LocalDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern) error {
	ldb.schemaKV.set(ctx, "/", schema, ldb)
	ldb.schema = schema
	return nil
}

func (ldb *LocalDatabase) Close(ctx *core.Context) error {
	ldb.mainKV.close(ctx)
	ldb.schemaKV.close(ctx)
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
	case "update_schema":
		return core.WrapGoMethod(ldb.UpdateSchema), true
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
