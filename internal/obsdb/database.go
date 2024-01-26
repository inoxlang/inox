package obsdb

// import (
// 	"errors"
// 	"fmt"
// 	"reflect"
// 	"strings"
// 	"sync"

// 	"github.com/inoxlang/inox/internal/core"
// 	"github.com/inoxlang/inox/internal/filekv"
// 	"github.com/inoxlang/inox/internal/globals/s3_ns"
// 	"github.com/inoxlang/inox/internal/parse"
// 	"github.com/inoxlang/inox/internal/utils"
// )

// const (
// 	SCHEMA_KEY = "/_schema_"

// 	MAIN_KV_FILE   = "main.kv"
// 	SCHEMA_KV_FILE = "schema.kv"
// )

// var (
// 	registry = openDatabasesRegistry{
// 		databases: map[registryDatabaseId]*ObjectStorageDatabase{},
// 	}

// 	ErrDatabaseOnlyAvailableInProjects = errors.New("object storage databases are only available in projects")

// 	_ core.Database = (*ObjectStorageDatabase)(nil)
// )

// func init() {
// 	core.RegisterOpenDbFn(ODB_SCHEME, func(ctx *core.Context, config core.DbOpenConfiguration) (core.Database, error) {
// 		return openDatabase(ctx, config.Resource, !config.FullAccess, config.Project)
// 	})

// 	core.RegisterStaticallyCheckDbResolutionDataFn(ODB_SCHEME, func(node parse.Node, optProject core.Project) string {
// 		hostLit, ok := node.(*parse.HostLiteral)
// 		if !ok || !strings.HasPrefix(hostLit.Value, "s3://") {
// 			return "the resolution data of an object storage database should be a host literal with a s3:// scheme"
// 		}

// 		if optProject == nil || reflect.ValueOf(optProject).IsNil() {
// 			return ErrDatabaseOnlyAvailableInProjects.Error()
// 		}

// 		return ""
// 	})
// }

// // An ObjectStorageDatabase is a database thats stores data in an object storage & has an optional on-disk cache.
// type ObjectStorageDatabase struct {
// 	host, s3Host core.Host
// 	id           registryDatabaseId

// 	filesystem *s3_ns.S3Filesystem
// 	mainKV     *filekv.SingleFileKV
// 	schemaKV   *filekv.SingleFileKV
// 	schema     *core.ObjectPattern

// 	topLevelValues     map[string]core.Serializable
// 	topLevelValuesLock sync.Mutex
// }

// type ObjectStorageDatabaseConfig struct {
// 	Host       core.Host
// 	S3Host     core.Host
// 	Restricted bool
// 	Filesystem *s3_ns.S3Filesystem
// 	Project    core.Project
// }

// type registryDatabaseId string

// func getRegistryDatabaseId(projectId core.ProjectID, s3Host core.Host) registryDatabaseId {
// 	return registryDatabaseId(string(projectId) + "//" + string(s3Host))
// }

// // openDatabase opens a database, read, create & write permissions are required.
// func openDatabase(ctx *core.Context, r core.ResourceName, restrictedAccess bool, optProject core.Project) (*ObjectStorageDatabase, error) {
// 	var s3Host core.Host

// 	h, ok := r.(core.Host)
// 	if ok || h.Scheme() != ODB_SCHEME {
// 		data := ctx.GetHostResolutionData(h)
// 		host, ok := data.(core.Host)
// 		if !ok || host.Scheme() != "s3" {
// 			return nil, core.ErrCannotResolveDatabase
// 		}
// 		s3Host = host
// 	} else {
// 		return nil, core.ErrCannotResolveDatabase
// 	}

// 	if optProject != nil && reflect.ValueOf(optProject).IsNil() {
// 		optProject = nil
// 	}

// 	bucket, err := s3_ns.OpenBucket(ctx, s3Host, s3_ns.OpenBucketOptions{
// 		AllowGettingCredentialsFromProject: true,
// 		Project:                            optProject,
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to open bucket: %w", err)
// 	}
// 	s3fls := s3_ns.NewS3Filesystem(ctx, bucket)

// 	//TODO: check permissions
// 	db, err := openDatabaseWithConfig(ctx, ObjectStorageDatabaseConfig{
// 		S3Host:     s3Host,
// 		Host:       h,
// 		Restricted: restrictedAccess,
// 		Filesystem: s3fls,
// 		Project:    optProject,
// 	})

// 	return db, err
// }

// func openDatabaseWithConfig(ctx *core.Context, config ObjectStorageDatabaseConfig) (*ObjectStorageDatabase, error) {
// 	if config.Project == nil || reflect.ValueOf(config.Project).IsNil() {
// 		return nil, ErrDatabaseOnlyAvailableInProjects
// 	}

// 	registry.lock.Lock()

// 	dbId := getRegistryDatabaseId(config.Project.Id(), config.S3Host)
// 	//return an error if the database is already open in the same project
// 	{
// 		_, alreadyOpen := registry.databases[dbId]
// 		defer registry.lock.Unlock()
// 		if alreadyOpen {
// 			return nil, core.ErrDatabaseAlreadyOpen
// 		}
// 	}

// 	mainKVPath := core.Path("/" + MAIN_KV_FILE)
// 	schemaKVPath := core.Path("/" + SCHEMA_KV_FILE)

// 	odb := &ObjectStorageDatabase{
// 		host:       config.Host,
// 		s3Host:     config.S3Host,
// 		id:         dbId,
// 		filesystem: config.Filesystem,
// 	}

// 	if !config.Restricted {
// 		mainKv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
// 			Path:       mainKVPath,
// 			Filesystem: config.Filesystem,
// 		})

// 		if err != nil {
// 			return nil, err
// 		}

// 		odb.mainKV = mainKv
// 	} else {
// 		odb.mainKV, _ = filekv.OpenSingleFileKV(filekv.KvStoreConfig{
// 			InMemory: true,
// 		})
// 	}

// 	schemaKv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
// 		Path:       schemaKVPath,
// 		Filesystem: config.Filesystem,
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	odb.schemaKV = schemaKv

// 	schema, ok, err := schemaKv.Get(ctx, "/", odb)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read schema: %w", err)
// 	}
// 	if ok {
// 		patt, ok := schema.(*core.ObjectPattern)
// 		if !ok {
// 			err := odb.Close(ctx)
// 			if err != nil {
// 				return nil, fmt.Errorf("schema is present but is not an object pattern, close db: %w", err)
// 			}
// 			return nil, fmt.Errorf("schema is present but is not an object pattern, close db")
// 		}
// 		odb.schema = patt
// 	} else {
// 		odb.schema = core.NewInexactObjectPattern(map[string]core.Pattern{})
// 	}

// 	registry.databases[dbId] = odb

// 	return odb, nil
// }

// func (obsdb *ObjectStorageDatabase) Resource() core.SchemeHolder {
// 	return obsdb.host
// }

// func (obsdb *ObjectStorageDatabase) Schema() *core.ObjectPattern {
// 	return obsdb.schema
// }

// func (obsdb *ObjectStorageDatabase) BaseURL() core.URL {
// 	return core.URL(obsdb.host + "/")
// }

// func (obsdb *ObjectStorageDatabase) LoadTopLevelEntities(ctx *core.Context) (map[string]core.Serializable, error) {
// 	obsdb.topLevelValuesLock.Lock()
// 	defer obsdb.topLevelValuesLock.Unlock()

// 	if obsdb.topLevelValues != nil {
// 		return obsdb.topLevelValues, nil
// 	}

// 	err := obsdb.load(ctx, nil, core.MigrationOpHandlers{})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return obsdb.topLevelValues, nil
// }

// func (obsdb *ObjectStorageDatabase) UpdateSchema(ctx *core.Context, schema *core.ObjectPattern, handlers core.MigrationOpHandlers) {
// 	obsdb.topLevelValuesLock.Lock()
// 	defer obsdb.topLevelValuesLock.Unlock()

// 	if obsdb.topLevelValues != nil {
// 		panic(core.ErrTopLevelEntitiesAlreadyLoaded)
// 	}

// 	if obsdb.schema.Equal(ctx, schema, map[uintptr]uintptr{}, 0) {
// 		return
// 	}

// 	obsdb.load(ctx, schema, handlers)

// 	obsdb.schemaKV.Set(ctx, "/", schema, obsdb)
// 	obsdb.schema = schema
// }

// func (obsdb *ObjectStorageDatabase) load(ctx *core.Context, migrationNextPattern *core.ObjectPattern, handlers core.MigrationOpHandlers) error {
// 	obsdb.topLevelValues = make(map[string]core.Serializable, obsdb.schema.EntryCount())
// 	state := ctx.GetClosestState()

// 	err := obsdb.schema.ForEachEntry(func(propName string, propPattern core.Pattern, isOptional bool) error {
// 		path := core.PathFrom("/" + propName)
// 		args := core.InstanceLoadArgs{
// 			Pattern:      propPattern,
// 			Key:          path,
// 			Storage:      obsdb,
// 			AllowMissing: true,
// 		}

// 		//replacement or migration of the top-level entity
// 		if migrationNextPattern != nil {
// 			args.Migration = &core.InstanceMigrationArgs{
// 				MigrationHandlers: handlers.FilterByPrefix(path),
// 			}
// 			if propPattern, _, ok := migrationNextPattern.Entry(propName); ok {
// 				args.Migration.NextPattern = propPattern
// 			}
// 		}

// 		value, err := core.LoadInstance(ctx, args)
// 		if err != nil {
// 			return fmt.Errorf("failed to load %s: %w", path, err)
// 		}

// 		if !args.IsDeletion(ctx) {
// 			obsdb.topLevelValues[propName] = value
// 		}
// 		return nil
// 	})

// 	if err != nil {
// 		return err
// 	}

// 	if migrationNextPattern != nil {
// 		_handlers := handlers.FilterTopLevel()

// 		for pattern, handler := range _handlers.Inclusions {
// 			path := core.Path(pattern)
// 			propName := string(path[1:])

// 			var initialValue core.Value
// 			if handler.Function != nil {
// 				prevValue, ok := obsdb.topLevelValues[string(pattern)]
// 				if !ok {
// 					prevValue = core.Nil
// 				}
// 				replacement, err := handler.Function.Call(state, nil, []core.Value{prevValue}, nil)
// 				if err != nil {
// 					return fmt.Errorf("error during call of inclusion handler for %s: %w", pattern, err)
// 				}
// 				initialValue = replacement.(core.Serializable)
// 			} else {
// 				initialValue = handler.InitialValue
// 			}

// 			pattern_, _, ok := migrationNextPattern.Entry(propName)
// 			if !ok {
// 				panic(core.ErrUnreachable)
// 			}

// 			args := core.InstanceLoadArgs{
// 				Pattern:      pattern_,
// 				Key:          path,
// 				InitialValue: initialValue.(core.Serializable),
// 				Storage:      obsdb,
// 				AllowMissing: true,
// 			}
// 			value, err := core.LoadInstance(ctx, args)
// 			if err != nil {
// 				return fmt.Errorf("failed to load %s: %w", path, err)
// 			}
// 			obsdb.topLevelValues[propName] = value
// 		}
// 	}

// 	return nil
// }

// func (obsdb *ObjectStorageDatabase) Close(ctx *core.Context) error {
// 	if obsdb.mainKV != nil {
// 		obsdb.mainKV.Close(ctx)
// 	}
// 	obsdb.schemaKV.Close(ctx)
// 	registry.lock.Lock()
// 	delete(registry.databases, obsdb.id)
// 	registry.lock.Unlock()

// 	return nil
// }

// func (obsdb *ObjectStorageDatabase) RemoveAllObjects(ctx *core.Context) {
// 	obsdb.filesystem.RemoveAllObjects()
// }

// func (odb *ObjectStorageDatabase) Get(ctx *core.Context, key core.Path) (core.Value, core.Bool) {
// 	return utils.Must2(odb.mainKV.Get(ctx, key, odb))
// }

// func (odb *ObjectStorageDatabase) GetSerialized(ctx *core.Context, key core.Path) (string, bool) {
// 	s, ok := utils.Must2(odb.mainKV.GetSerialized(ctx, key, odb))
// 	return s, bool(ok)
// }

// func (odb *ObjectStorageDatabase) Has(ctx *core.Context, key core.Path) bool {
// 	return bool(odb.mainKV.Has(ctx, key, odb))
// }

// func (obsdb *ObjectStorageDatabase) Set(ctx *core.Context, key core.Path, value core.Serializable) {
// 	obsdb.mainKV.Set(ctx, key, value, obsdb)
// }

// func (odb *ObjectStorageDatabase) SetSerialized(ctx *core.Context, key core.Path, serialized string) {
// 	odb.mainKV.SetSerialized(ctx, key, serialized, odb)
// }

// func (obsdb *ObjectStorageDatabase) Insert(ctx *core.Context, key core.Path, value core.Serializable) {
// 	obsdb.mainKV.Insert(ctx, key, value, obsdb)
// }

// func (obsdb *ObjectStorageDatabase) InsertSerialized(ctx *core.Context, key core.Path, serialized string) {
// 	obsdb.mainKV.InsertSerialized(ctx, key, serialized, obsdb)
// }

// func (obsdb *ObjectStorageDatabase) Remove(ctx *core.Context, key core.Path) {
// 	obsdb.mainKV.Delete(ctx, key, obsdb)
// }

// type openDatabasesRegistry struct {
// 	lock      sync.Mutex
// 	databases map[registryDatabaseId]*ObjectStorageDatabase
// }
