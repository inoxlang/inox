package local_db

import (
	"fmt"
	"strings"
	"sync"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	SCHEMA_KEY = "/_schema_"

	MAIN_KV_FILE   = "main.kv"
	SCHEMA_KV_FILE = "schema.kv"
	LDB_SCHEME     = core.Scheme("ldb")
)

var (
	LOCAL_DB_PROPNAMES = []string{"update_schema", "close"}

	_ core.Database = (*LocalDatabase)(nil)
)

func init() {
	core.RegisterOpenDbFn(LDB_SCHEME, func(ctx *core.Context, config core.DbOpenConfiguration) (core.Database, error) {
		return OpenDatabase(ctx, config.Resource, !config.FullAccess)
	})

	checkResolutionData := func(node parse.Node, _ core.Project) (errMsg string) {
		pathLit, ok := node.(*parse.AbsolutePathLiteral)
		if !ok || !strings.HasSuffix(pathLit.Value, "/") {
			return "the resolution data of a local database should be an absolute directory path (it should end with '/')"
		}

		return ""
	}
	core.RegisterStaticallyCheckDbResolutionDataFn(LDB_SCHEME, checkResolutionData)
	core.RegisterStaticallyCheckHostResolutionDataFn(LDB_SCHEME, func(optionalProject core.Project, node parse.Node) (errorMsg string) {
		return checkResolutionData(node, nil)
	})
}

// A LocalDatabase is a database thats stores data on the filesystem.
type LocalDatabase struct {
	host     core.Host
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

// OpenDatabase opens a local database, read, create & write permissions are required.
func OpenDatabase(ctx *core.Context, r core.ResourceName, restrictedAccess bool) (*LocalDatabase, error) {

	var pth core.Path

	switch resource := r.(type) {
	case core.Host:
		if resource.Scheme() != LDB_SCHEME {
			return nil, core.ErrCannotResolveDatabase
		}
		data, ok := ctx.GetHostResolutionData(resource).(core.Path)
		if !ok {
			return nil, core.ErrCannotResolveDatabase
		}
		pth = data
	case core.Path:
		var err error
		pth, err = resource.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return nil, err
		}
	default:
		return nil, core.ErrCannotResolveDatabase
	}

	if !pth.IsDirPath() {
		return nil, core.ErrInvalidDatabaseDirpath
	}

	patt := core.PathPattern(pth + "...")

	for _, kind := range []core.PermissionKind{permkind.Read, permkind.Create, permkind.WriteStream} {
		perm := core.FilesystemPermission{Kind_: kind, Entity: patt}
		if err := ctx.CheckHasPermission(perm); err != nil {
			return nil, err
		}
	}

	host, ok := ctx.GetHostFromResolutionData(pth)
	if !ok {
		return nil, core.ErrCannotFindDatabaseHost
	}

	if host.Scheme() != LDB_SCHEME {
		return nil, core.ErrInvalidDatabaseHost
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
		patt, ok := schema.(*core.ObjectPattern)
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

func (ldb *LocalDatabase) LoadTopLevelEntities(ctx *core.Context) (map[string]core.Serializable, error) {
	ldb.topLevelValuesLock.Lock()
	defer ldb.topLevelValuesLock.Unlock()

	if ldb.topLevelValues != nil {
		return ldb.topLevelValues, nil
	}

	err := ldb.load(ctx, nil, core.MigrationOpHandlers{})
	if err != nil {
		return nil, err
	}

	return ldb.topLevelValues, nil
}

func (ldb *LocalDatabase) UpdateSchema(ctx *core.Context, schema *core.ObjectPattern, handlers core.MigrationOpHandlers) {
	ldb.topLevelValuesLock.Lock()
	defer ldb.topLevelValuesLock.Unlock()

	if ldb.topLevelValues != nil {
		panic(core.ErrTopLevelEntitiesAlreadyLoaded)
	}

	if ldb.schema.Equal(ctx, schema, map[uintptr]uintptr{}, 0) {
		return
	}

	if err := ldb.load(ctx, schema, handlers); err != nil {
		panic(err)
	}

	ldb.schemaKV.Set(ctx, "/", schema, ldb)
	ldb.schema = schema
}

func (ldb *LocalDatabase) load(ctx *core.Context, migrationNextPattern *core.ObjectPattern, handlers core.MigrationOpHandlers) error {
	ldb.topLevelValues = make(map[string]core.Serializable, ldb.schema.EntryCount())
	state := ctx.GetClosestState()

	err := ldb.schema.ForEachEntry(func(propName string, propPattern core.Pattern, isOptional bool) error {
		path := core.PathFrom("/" + propName)
		args := core.InstanceLoadArgs{
			Pattern:      propPattern,
			Key:          path,
			Storage:      ldb,
			AllowMissing: true,
		}

		//replacement or migration of the top-level entity
		if migrationNextPattern != nil {
			args.Migration = &core.InstanceMigrationArgs{
				MigrationHandlers: handlers.FilterByPrefix(path),
			}
			if propPattern, _, ok := migrationNextPattern.Entry(propName); ok {
				args.Migration.NextPattern = propPattern
			}
		}

		value, err := core.LoadInstance(ctx, args)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", path, err)
		}

		if !args.IsDeletion(ctx) {
			ldb.topLevelValues[propName] = value
		}
		return nil
	})

	if err != nil {
		return err
	}

	if migrationNextPattern != nil {
		_handlers := handlers.FilterTopLevel()

		for pattern, handler := range _handlers.Inclusions {
			path := core.Path(pattern)
			propName := string(path[1:])

			var initialValue core.Value
			if handler.Function != nil {
				prevValue, ok := ldb.topLevelValues[string(pattern)]
				if !ok {
					prevValue = core.Nil
				}
				replacement, err := handler.Function.Call(state, nil, []core.Value{prevValue}, nil)
				if err != nil {
					return fmt.Errorf("error during call of inclusion handler for %s: %w", pattern, err)
				}
				initialValue = replacement.(core.Serializable)
			} else {
				initialValue = handler.InitialValue
			}

			pattern_, _, ok := migrationNextPattern.Entry(propName)
			if !ok {
				panic(core.ErrUnreachable)
			}

			args := core.InstanceLoadArgs{
				Pattern:      pattern_,
				Key:          path,
				InitialValue: initialValue.(core.Serializable),
				Storage:      ldb,
				AllowMissing: true,
			}
			value, err := core.LoadInstance(ctx, args)
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", path, err)
			}
			ldb.topLevelValues[propName] = value
		}
	}

	return nil
}

func (ldb *LocalDatabase) Close(ctx *core.Context) error {
	if ldb.mainKV != nil {
		ldb.mainKV.Close(ctx)
	}
	ldb.schemaKV.Close(ctx)
	return nil
}

func (ldb *LocalDatabase) Get(ctx *core.Context, key core.Path) (core.Value, core.Bool) {
	return utils.Must2(ldb.mainKV.Get(ctx, key, ldb))
}

func (ldb *LocalDatabase) GetSerialized(ctx *core.Context, key core.Path) (string, bool) {
	s, ok := utils.Must2(ldb.mainKV.GetSerialized(ctx, key, ldb))
	return s, bool(ok)
}

func (ldb *LocalDatabase) Has(ctx *core.Context, key core.Path) bool {
	return bool(ldb.mainKV.Has(ctx, key, ldb))
}

func (ldb *LocalDatabase) Set(ctx *core.Context, key core.Path, value core.Serializable) {
	ldb.mainKV.Set(ctx, key, value, ldb)
}

func (ldb *LocalDatabase) SetSerialized(ctx *core.Context, key core.Path, serialized string) {
	ldb.mainKV.SetSerialized(ctx, key, serialized, ldb)
}

func (ldb *LocalDatabase) Insert(ctx *core.Context, key core.Path, value core.Serializable) {
	ldb.mainKV.Insert(ctx, key, value, ldb)
}

func (ldb *LocalDatabase) InsertSerialized(ctx *core.Context, key core.Path, serialized string) {
	ldb.mainKV.InsertSerialized(ctx, key, serialized, ldb)
}

func (ldb *LocalDatabase) Remove(ctx *core.Context, key core.Path) {
	ldb.mainKV.Delete(ctx, key, ldb)
}

type databaseRegistry struct {
	lock          sync.Mutex
	resolutions   map[core.Host]core.Path
	openDatabases map[core.Path]*LocalDatabase
}
