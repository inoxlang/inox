package localdb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/inoxlang/inox/internal/buntdb"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	SCHEMA_KEY = "/_schema_"

	DB_KV_FILE   = "db.bbolt"
	META_KV_FILE = "meta.buntdb"
	LDB_SCHEME   = core.Scheme("ldb")

	OS_DB_DIR = 0700
)

var (
	LOCAL_DB_PROPNAMES = []string{"update_schema", "close"}

	ErrOpenDatabase = errors.New("database is already open by the current process or another one")

	_ core.Database = (*LocalDatabase)(nil)
)

func init() {
	core.RegisterOpenDbFn(LDB_SCHEME, func(ctx *core.Context, config core.DbOpenConfiguration) (core.Database, error) {
		return OpenDatabase(ctx, config.Resource, !config.FullAccess)
	})

	checkResolutionData := func(node parse.Node, _ core.Project) (errMsg string) {
		_, ok := node.(*parse.NilLiteral)
		if !ok {
			return "the resolution data of a local database should be nil"
		}

		return ""
	}
	core.RegisterStaticallyCheckDbResolutionDataFn(LDB_SCHEME, checkResolutionData)
	core.RegisterStaticallyCheckHostResolutionDataFn(LDB_SCHEME, func(optionalProject core.Project, node parse.Node) (errorMsg string) {
		return checkResolutionData(node, nil)
	})
}

// A LocalDatabase is a database thats stores data on a filesystem.
type LocalDatabase struct {
	host    core.Host
	osFsDir core.Path
	mainKV  *filekv.SingleFileKV
	metaKV  *buntdb.DB
	schema  *core.ObjectPattern
	logger  zerolog.Logger

	topLevelValues     map[string]core.Serializable
	topLevelValuesLock sync.Mutex
}

type LocalDatabaseConfig struct {
	OsFsDir    core.Path
	Host       core.Host
	InMemory   bool
	Restricted bool
}

// OpenDatabase opens a local database, read, create & write permissions are required.
func OpenDatabase(ctx *core.Context, r core.ResourceName, restrictedAccess bool) (*LocalDatabase, error) {

	var host core.Host
	switch resource := r.(type) {
	case core.Host:
		if resource.Scheme() != LDB_SCHEME {
			return nil, core.ErrCannotResolveDatabase
		}
		switch data := ctx.GetHostResolutionData(resource).(type) {
		case core.Host:
			//no data

			host = data
		case nil, core.NilT:
			host = resource
		default:
			//local databases do not require resolution data
			return nil, core.ErrCannotResolveDatabase
		}
	default:
		return nil, core.ErrCannotResolveDatabase
	}

	if host.Scheme() != LDB_SCHEME || host.ExplicitPort() >= 0 {
		return nil, core.ErrInvalidDatabaseHost
	}

	project := ctx.GetClosestState().Project
	if project == nil || reflect.ValueOf(project).IsZero() {
		return nil, errors.New("local databases are only supported in project mode")
	}
	dbsDir := project.DevDatabasesDirOnOsFs()

	db, err := openLocalDatabaseWithConfig(ctx, LocalDatabaseConfig{
		OsFsDir:    core.DirPathFrom(filepath.Join(dbsDir, host.Name())),
		Host:       host,
		Restricted: restrictedAccess,
	})

	return db, err
}

func openLocalDatabaseWithConfig(ctx *core.Context, config LocalDatabaseConfig) (*LocalDatabase, error) {
	osFs := fs_ns.GetOsFilesystem()
	mainKVPath := config.OsFsDir.Join(core.Path("./"+DB_KV_FILE), osFs)
	metaKVPath := config.OsFsDir.Join(core.Path("./"+META_KV_FILE), osFs)

	localDB := &LocalDatabase{
		host:    config.Host,
		osFsDir: config.OsFsDir,
	}

	//create the directory for the database
	osFsDir := config.OsFsDir.UnderlyingString()

	_, err := os.Stat(osFsDir)
	if os.IsNotExist(err) {
		err = os.Mkdir(osFsDir, OS_DB_DIR)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory for database %q: %w", config.Host, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to check if the directory of the %q database exist: %w", config.Host, err)
	}

	if !config.Restricted {
		mainKv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Path: mainKVPath,
		})

		if err != nil {
			if errors.Is(err, filekv.ErrOpenKvStore) {
				return nil, ErrOpenDatabase
			}
			return nil, err
		}

		localDB.mainKV = mainKv

		//open meta KV

		metaKV, err := buntdb.OpenBuntDBNoPermCheck(metaKVPath.UnderlyingString(), fs_ns.GetOsFilesystem())
		if err != nil {
			return nil, err
		}

		localDB.metaKV = metaKV
	} else {
		//in restricted mode we load the meta KV data inside an in-memory KV

		content, err := os.ReadFile(string(metaKVPath))
		if os.IsNotExist(err) {
			err = nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to read the content of the local database's meta file: %w", err)
		}

		metaKV, err := buntdb.OpenBuntDBNoPermCheck(":memory:", nil)
		if err != nil {
			return nil, err
		}

		localDB.metaKV = metaKV

		err = metaKV.Load(bytes.NewReader(content))

		// The db file is allowed to have ended mid-command.

		if err != nil && errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, err
		}
	}

	//get schema
	var serializedSchema string
	err = localDB.metaKV.View(func(tx *buntdb.Tx) error {
		serialized, err := tx.Get(SCHEMA_KEY, true)
		if err != nil {
			return err
		}
		serializedSchema = serialized
		return nil
	})

	schemaFound := err == nil

	if err != nil && !errors.Is(err, buntdb.ErrNotFound) {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	if schemaFound {
		schema, err := core.ParseRepr(ctx, []byte(serializedSchema))

		if err != nil {
			return nil, fmt.Errorf("failed to parse database schema: %w", err)
		}

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

	//load data and perform migrations

	if err := ldb.load(ctx, schema, handlers); err != nil {
		panic(err)
	}

	// store the new schema

	repr := string(core.GetRepresentation(schema, ctx))

	err := ldb.metaKV.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(SCHEMA_KEY, repr, nil)
		return err
	})
	if err != nil {
		panic(err)
	}
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
	ldb.metaKV.Close()
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
