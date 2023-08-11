package local_db_ns

import (
	"errors"
	"fmt"
	"sync"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
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
	ErrDatabaseNotSupported   = errors.New("database is not supported")

	LOCAL_DB_PROPNAMES = []string{"update_schema", "close"}

	_ core.Database = (*LocalDatabase)(nil)
)

// A LocalDatabase is a database thats stores data on the filesystem.
type LocalDatabase struct {
	host     Host
	dirPath  core.Path
	mainKV   *filekv.SingleFileKV
	schemaKV *filekv.SingleFileKV
	schema   *core.ObjectPattern
	logger   zerolog.Logger

	topLevelValues     map[string]core.Serializable
	topLevelValuesLock sync.Mutex
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
		var err error
		pth, err = resource.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrCannotResolveDatabase
	}

	if !pth.IsDirPath() {
		return nil, ErrInvalidDatabaseDirpath
	}

	patt := PathPattern(pth + "...")

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
		mainKv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Path:       mainKVPath,
			InMemory:   config.InMemory,
			Filesystem: fls,
		})

		if err != nil {
			return nil, err
		}

		localDB.mainKV = mainKv
	} else {
		localDB.mainKV, _ = filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			InMemory: true,
		})
	}

	schemaKv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path:       schemaKVPath,
		InMemory:   config.InMemory,
		Filesystem: fls,
	})

	if err != nil {
		return nil, err
	}

	localDB.schemaKV = schemaKv

	schema, ok, err := schemaKv.Get(ctx, "/", localDB)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}
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

func (ldb *LocalDatabase) Schema() *core.ObjectPattern {
	return ldb.schema
}

func (ldb *LocalDatabase) BaseURL() core.URL {
	return core.URL(ldb.host + "/")
}

func (ldb *LocalDatabase) TopLevelEntities(ctx *core.Context) map[string]core.Serializable {
	ldb.topLevelValuesLock.Lock()
	defer ldb.topLevelValuesLock.Unlock()

	if ldb.topLevelValues != nil {
		return ldb.topLevelValues
	}

	ldb.load(ctx, nil, core.MigrationOpHandlers{})

	return ldb.topLevelValues
}

func (ldb *LocalDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers core.MigrationOpHandlers) {
	ldb.topLevelValuesLock.Lock()
	defer ldb.topLevelValuesLock.Unlock()

	if ldb.topLevelValues != nil {
		panic(core.ErrTopLevelEntitiesAlreadyLoaded)
	}

	if ldb.schema.Equal(ctx, schema, map[uintptr]uintptr{}, 0) {
		return
	}

	ldb.load(ctx, schema, handlers)

	ldb.schemaKV.Set(ctx, "/", schema, ldb)
	ldb.schema = schema
}

func (ldb *LocalDatabase) load(ctx *Context, migrationNextPattern *ObjectPattern, handlers core.MigrationOpHandlers) {
	ldb.topLevelValues = make(map[string]core.Serializable, ldb.schema.EntryCount())

	err := ldb.schema.ForEachEntry(func(propName string, propPattern core.Pattern, isOptional bool) error {
		path := core.PathFrom("/" + propName)
		args := core.InstanceLoadArgs{
			Pattern:      propPattern,
			Key:          path,
			Storage:      ldb,
			AllowMissing: true,
		}

		if migrationNextPattern != nil {
			args.Migration = &core.InstanceMigrationArgs{
				MigrationHandlers: handlers,
			}
			if propPattern, _, ok := migrationNextPattern.Entry(propName); ok {
				args.Migration.NextPattern = propPattern
			}
		}

		value, err := core.LoadInstance(ctx, args)
		if err != nil {
			return err
		}

		if !args.IsDeletion(ctx) {
			ldb.topLevelValues[propName] = value
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	if migrationNextPattern != nil {
		//load new top level entities
		err := migrationNextPattern.ForEachEntry(func(propName string, propPattern core.Pattern, isOptional bool) error {
			_, alreadyLoaded := ldb.topLevelValues[propName]
			if alreadyLoaded {
				return nil
			}

			path := core.PathFrom("/" + propName)
			args := core.InstanceLoadArgs{
				Pattern:      propPattern,
				Key:          path,
				Storage:      ldb,
				AllowMissing: true,
			}
			value, err := core.LoadInstance(ctx, args)
			if err != nil {
				return err
			}
			ldb.topLevelValues[propName] = value
			return nil
		})

		if err != nil {
			panic(err)
		}
	}

}

func (ldb *LocalDatabase) Close(ctx *core.Context) error {
	if ldb.mainKV != nil {
		ldb.mainKV.Close(ctx)
	}
	ldb.schemaKV.Close(ctx)
	return nil
}

func (ldb *LocalDatabase) Get(ctx *Context, key Path) (Value, Bool) {
	return utils.Must2(ldb.mainKV.Get(ctx, key, ldb))
}

func (ldb *LocalDatabase) GetSerialized(ctx *Context, key Path) (string, bool) {
	s, ok := utils.Must2(ldb.mainKV.GetSerialized(ctx, key, ldb))
	return s, bool(ok)
}

func (ldb *LocalDatabase) Has(ctx *Context, key Path) bool {
	return bool(ldb.mainKV.Has(ctx, key, ldb))
}

func (ldb *LocalDatabase) Set(ctx *Context, key Path, value core.Serializable) {
	ldb.mainKV.Set(ctx, key, value, ldb)
}

func (ldb *LocalDatabase) SetSerialized(ctx *Context, key Path, serialized string) {
	ldb.mainKV.SetSerialized(ctx, key, serialized, ldb)
}

func (ldb *LocalDatabase) Insert(ctx *Context, key Path, value core.Serializable) {
	ldb.mainKV.Insert(ctx, key, value, ldb)
}

func (ldb *LocalDatabase) InsertSerialized(ctx *Context, key Path, serialized string) {
	ldb.mainKV.InsertSerialized(ctx, key, serialized, ldb)
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
