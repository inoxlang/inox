package core

import (
	"encoding/base32"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	iconv "github.com/inoxlang/inox/internal/utils/intconv"
	"github.com/inoxlang/inox/internal/utils/pathutils"
)

var (
	ErrOwnerStateAlreadySet                            = errors.New("owner state already set")
	ErrOwnerStateNotSet                                = errors.New("owner state not set")
	ErrNameCollisionWithInitialDatabasePropertyName    = errors.New("name collision with initial database property name")
	ErrTopLevelEntityNamesShouldBeValidInoxIdentifiers = errors.New("top-level entity names should be valid Inox identifiers (e.g., users, client-names)")
	ErrTopLevelEntitiesAlreadyLoaded                   = errors.New("top-level entities already loaded")
	ErrDatabaseSchemaOnlyUpdatableByOwnerState         = errors.New("database schema can only be updated by owner state")
	ErrNoDatabaseSchemaUpdateExpected                  = errors.New("no database schema update is expected")
	ErrCurrentSchemaNotEqualToExpectedSchema           = errors.New("current schema not equal to expected schema")
	ErrNewSchemaNotEqualToExpectedSchema               = errors.New("new schema not equal to expected schema")
	ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed        = errors.New("database schema already updated or no longer allowed")
	ErrInvalidAccessSchemaNotUpdatedYet                = errors.New("access to database is not allowed because schema is not updated yet")
	ErrSchemaCannotBeUpdatedInDevMode                  = errors.New("schema cannot be updated in dev mode")
	ErrInvalidDatabaseDirpath                          = errors.New("invalid database dir path")
	ErrDatabaseAlreadyOpen                             = errors.New("database is already open")
	ErrDatabaseClosed                                  = errors.New("database is closed")
	ErrCannotResolveDatabase                           = errors.New("cannot resolve database")
	ErrCannotFindDatabaseHost                          = errors.New("cannot find corresponding host of database")
	ErrInvalidDatabaseHost                             = errors.New("host of database is invalid")
	ErrInvalidDBValuePropRetrieval                     = errors.New("invalid property retrieval: value should be serializablr and should not be that a method or dynamic value")

	DATABASE_PROPNAMES = []string{"update_schema", "close", "schema"}

	ElementKeyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

	_ Value = (*DatabaseIL)(nil)
)

type DatabaseIL struct {
	inner         Database
	initialSchema *ObjectPattern

	newSchema    *ObjectPattern //set on schema update or if there is an override (see DatabaseWrappingArgs).
	newSchemaSet atomic.Bool

	devMode              bool
	schemaUpdateExpected bool
	schemaUpdated        atomic.Bool
	schemaUpdateLock     sync.Mutex

	expectedSchema *ObjectPattern

	ownerState *GlobalState //optional, can be set later using .SetOwnerStateOnceAndLoadIfNecessary
	name       string

	propertyNames                     []string
	topLevelEntitiesLoaded            atomic.Bool
	topLevelEntities                  map[string]Serializable
	topLevelEntitiesAccessPermissions map[string]DatabasePermission
}

type DbOpenConfiguration struct {
	Resource       SchemeHolder
	ResolutionData Value
	FullAccess     bool
	Project        Project
}

func checkDatabaseSchema(pattern *ObjectPattern) error {
	return pattern.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		expr, ok := parse.ParseExpression(propName)
		_, isIdent := expr.(*parse.IdentifierLiteral)
		if !ok || !isIdent {
			return fmt.Errorf("invalid top-level entity name: %q, %w", propName, ErrTopLevelEntityNamesShouldBeValidInoxIdentifiers)
		}

		if !hasTypeLoadingFunction(propPattern) {
			return fmt.Errorf("invalid pattern for top level entity .%s: %w", propName, ErrNoLoadFreeEntityFnRegistered)
		}
		if isOptional {
			return fmt.Errorf("unexpected optional property .%s in schema", propName)
		}
		return nil
	})
}

type Database interface {
	Resource() SchemeHolder

	Schema() *ObjectPattern

	//UpdateSchema updates the schema and validates the content of the database,
	//this method should return ErrTopLevelEntitiesAlreadyLoaded if it is called after .TopLevelEntities.
	//The caller should always pass a schema whose ALL entry patterns have a loading function.
	UpdateSchema(ctx *Context, schema *ObjectPattern, migrationHandlers MigrationOpHandlers)

	LoadTopLevelEntities(ctx *Context) (map[string]Serializable, error)

	Close(ctx *Context) error
}

type DatabaseWrappingArgs struct {
	Name       string
	Inner      Database
	OwnerState *GlobalState //if nil the owner state should be set later by calling SetOwnerStateOnceAndLoadIfNecessary

	//If true the database is not loaded until the schema has been updated.
	//This field is unrelated to ExpectedSchema.
	ExpectedSchemaUpdate bool

	//If not nil the current schema is compared to the expected schema.
	//The comparison is performed after any schema update.
	//This field is unrelated to ExpectedSchemaUpdate.
	ExpectedSchema *ObjectPattern

	//Force the loading top level entities if there is not expected schema update.
	//This parameter has lower priority than DevMode.
	ForceLoadBeforeOwnerStateSet bool

	//In dev mode top level entities are never loaded, and a mismatch between
	//the current schema and the expected schema causes the expected schema to be used.
	DevMode bool
}

// WrapDatabase wraps a Database in a struct that implements Value.
// In dev mode if the current schema does not match ExpectedSchema a DatbaseIL is returned alongside the error.
func WrapDatabase(ctx *Context, args DatabaseWrappingArgs) (*DatabaseIL, error) {
	schema := args.Inner.Schema()

	propertyNames := slices.Clone(DATABASE_PROPNAMES)
	schema.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if utils.SliceContains(DATABASE_PROPNAMES, propName) {
			panic(fmt.Errorf("%w: %s", ErrNameCollisionWithInitialDatabasePropertyName, propName))
		}
		propertyNames = append(propertyNames, propName)
		return nil
	})

	db := &DatabaseIL{
		inner:                args.Inner,
		initialSchema:        schema,
		propertyNames:        propertyNames,
		schemaUpdateExpected: args.ExpectedSchemaUpdate,
		expectedSchema:       args.ExpectedSchema,
		ownerState:           args.OwnerState,
		name:                 args.Name,

		devMode: args.DevMode,
	}

	var errInDevMode error

	if !args.ExpectedSchemaUpdate {
		currentSchema := args.Inner.Schema()

		//compare the current schema with the expected schema.
		if args.ExpectedSchema != nil && !currentSchema.Equal(ctx, args.ExpectedSchema, map[uintptr]uintptr{}, 0) {
			if db.devMode {
				db.newSchema = args.ExpectedSchema
				db.newSchemaSet.Store(true)

				if db.ownerState != nil {
					db.AddOwnerStateTeardownCallback()
				}

				errInDevMode = ErrCurrentSchemaNotEqualToExpectedSchema
				goto return_
			}
			return nil, ErrCurrentSchemaNotEqualToExpectedSchema
		}

		if args.ForceLoadBeforeOwnerStateSet {
			topLevelEntities, err := args.Inner.LoadTopLevelEntities(ctx)
			if err != nil {
				return nil, err
			}
			db.topLevelEntities = topLevelEntities
			db.topLevelEntitiesLoaded.Store(true)
			db.setDatabasePermissions()
		}
	}

	if db.ownerState != nil {
		db.AddOwnerStateTeardownCallback()
	}

return_:
	return db, errInDevMode
}

func (db *DatabaseIL) TopLevelEntitiesLoaded() bool {
	return db.topLevelEntitiesLoaded.Load()
}

func (db *DatabaseIL) AddOwnerStateTeardownCallback() {
	db.ownerState.Ctx.OnGracefulTearDown(func(ctx *Context) (finalErr error) {
		defer func() {
			if e := recover(); e != nil {
				finalErr = fmt.Errorf("%w: %s", utils.ConvertPanicValueToError(e), string(debug.Stack()))
			}
		}()

		return db.inner.Close(ctx)
	})
}

func (db *DatabaseIL) SetOwnerStateOnceAndLoadIfNecessary(ctx *Context, state *GlobalState) error {
	if db.ownerState != nil {
		panic(ErrOwnerStateAlreadySet)
	}

	if db.topLevelEntities == nil && !db.schemaUpdateExpected && !db.devMode {
		topLevelEntities, err := db.inner.LoadTopLevelEntities(ctx)
		if err != nil {
			return err
		}
		db.topLevelEntities = topLevelEntities
		db.topLevelEntitiesLoaded.Store(true)
		db.setDatabasePermissions()
	}
	db.ownerState = state

	db.AddOwnerStateTeardownCallback()
	return nil
}

func (db *DatabaseIL) IsPermissionForThisDB(perm DatabasePermission) bool {
	return (DatabasePermission{
		Kind_:  perm.Kind_,
		Entity: db.inner.Resource(),
	}).Includes(perm)
}

func (db *DatabaseIL) setDatabasePermissions() {
	if db.topLevelEntitiesAccessPermissions != nil {
		panic(errors.New("access permissions already set"))
	}
	db.topLevelEntitiesAccessPermissions = map[string]DatabasePermission{}

	host, ok := db.inner.Resource().(Host)
	if !ok {
		panic(errors.New("only hosts are supported for now"))
	}

	for name := range db.topLevelEntities {
		db.topLevelEntitiesAccessPermissions[name] = DatabasePermission{
			Kind_:  permkind.Read,
			Entity: URL(string(host) + "/" + name),
		}
	}
}

func (db *DatabaseIL) Resource() SchemeHolder {
	return db.inner.Resource()
}

func (db *DatabaseIL) UpdateSchema(ctx *Context, nextSchema *ObjectPattern, migrations ...*Object) {
	if db.devMode {
		panic(ErrSchemaCannotBeUpdatedInDevMode)
	}

	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}

	if !db.schemaUpdateExpected {
		panic(ErrNoDatabaseSchemaUpdateExpected)
	}

	if ctx.GetClosestState() != db.ownerState {
		panic(ErrDatabaseSchemaOnlyUpdatableByOwnerState)
	}

	if db.schemaUpdated.Load() {
		panic(ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed)
	}

	if db.topLevelEntitiesLoaded.Load() {
		panic(ErrTopLevelEntitiesAlreadyLoaded)
	}

	//this check is also run during symbolic evaluation
	if err := checkDatabaseSchema(nextSchema); err != nil {
		panic(err)
	}

	if db.expectedSchema != nil && !nextSchema.Equal(ctx, db.expectedSchema, map[uintptr]uintptr{}, 0) {
		panic(ErrNewSchemaNotEqualToExpectedSchema)
	}

	db.schemaUpdateLock.Lock()
	defer db.schemaUpdateLock.Unlock()

	defer db.schemaUpdated.Store(true)

	err := nextSchema.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if !hasTypeLoadingFunction(propPattern) {
			return fmt.Errorf("failed to update schema: pattern of .%s has no loading function: %w", propName, ErrNoLoadFreeEntityFnRegistered)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	migrationOps, err := GetMigrationOperations(ctx, db.initialSchema, nextSchema, "/")
	if err != nil {
		panic(err)
	}

	var migrationHandlers MigrationOpHandlers

	//TODO: make sure concrete migration handlers match concretized symbolic expected migration handlers
	if len(migrationOps) > 0 {
		if len(migrations) != 1 {
			panic(ErrUnreachable)
		}

		migrationObject := migrations[0]
		migrationObject.ForEachEntry(func(k string, v Serializable) error {
			dict := v.(*Dictionary)

			switch k {
			case symbolic.DB_MIGRATION__DELETIONS_PROP_NAME:
				migrationHandlers.Deletions = map[PathPattern]*MigrationOpHandler{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					var handler *MigrationOpHandler

					switch val := v.(type) {
					case NilT:
					case *InoxFunction:
						handler = &MigrationOpHandler{Function: val}
					default:
						panic(parse.ErrUnreachable)
					}

					migrationHandlers.Deletions[key.(PathPattern)] = handler
					return nil
				})
			case symbolic.DB_MIGRATION__REPLACEMENTS_PROP_NAME:
				migrationHandlers.Replacements = map[PathPattern]*MigrationOpHandler{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					var handler *MigrationOpHandler

					switch val := v.(type) {
					case *InoxFunction:
						handler = &MigrationOpHandler{Function: val}
					case Serializable:
						handler = &MigrationOpHandler{InitialValue: val}
					}

					migrationHandlers.Replacements[key.(PathPattern)] = handler
					return nil
				})
			case symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME:
				migrationHandlers.Inclusions = map[PathPattern]*MigrationOpHandler{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					var handler *MigrationOpHandler

					switch val := v.(type) {
					case *InoxFunction:
						handler = &MigrationOpHandler{Function: val}
					case Serializable:
						handler = &MigrationOpHandler{InitialValue: val}
					}

					migrationHandlers.Inclusions[key.(PathPattern)] = handler
					return nil
				})
			case symbolic.DB_MIGRATION__INITIALIZATIONS_PROP_NAME:
				migrationHandlers.Initializations = map[PathPattern]*MigrationOpHandler{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					var handler *MigrationOpHandler

					switch val := v.(type) {
					case *InoxFunction:
						handler = &MigrationOpHandler{Function: val}
					case Serializable:
						handler = &MigrationOpHandler{InitialValue: val}
					}

					migrationHandlers.Initializations[key.(PathPattern)] = handler
					return nil
				})
			default:
				return fmt.Errorf("unexpected property '%s' in migration object", k)
			}
			return nil
		})
	}

	db.inner.UpdateSchema(ctx, nextSchema, migrationHandlers)
	db.topLevelEntities = utils.Must(db.inner.LoadTopLevelEntities(ctx))
	db.topLevelEntitiesLoaded.Store(true)
	db.newSchema = nextSchema
	db.newSchemaSet.Store(true)
	db.setDatabasePermissions()
}

// GetOrLoad retrieves an entity or value stored inside the database.
func (db *DatabaseIL) GetOrLoad(ctx *Context, path Path) (Serializable, error) {
	first := true
	var current Serializable

	err := symbolic.ValidatePathOfValueInDatabase(path)
	if err != nil {
		return nil, err
	}

	err = pathutils.ForEachAbsolutePathSegment(path, func(segment string, startIndex, endIndex int) (err error) {
		defer func() {
			if e := recover(); e != nil {
				err = fmt.Errorf("failed to get value or entity at %s: %w", path[:endIndex], utils.ConvertPanicValueToError(e))
			}
		}()

		if first {
			topLevelEntity, ok := db.topLevelEntities[segment]
			if !ok {
				return fmt.Errorf("top level entity /%s does not exist", segment)
			}
			current = topLevelEntity
			first = false
			return nil
		}

		if collection, ok := current.(Collection); ok {
			key, err := ElementKeyFrom(segment)
			if err != nil {
				return fmt.Errorf("invalid path segment %q: %w", segment, err)
			}

			elem, err := collection.GetElementByKey(key)
			if errors.Is(err, ErrCollectionElemNotFound) {
				return fmt.Errorf("there is no entity at %s: %w", path[:endIndex], err)
			}
			if err != nil {
				return fmt.Errorf("failed to retrieve the entity at %s: %w", path[:endIndex], err)
			}
			current = elem
		} else {
			indexable, ok := current.(Indexable)
			if ok {
				index, err := strconv.ParseInt(segment, 10, 32)
				if err != nil {
					goto not_an_index
				}
				intIndex := iconv.MustI64ToI(index)

				if intIndex >= indexable.Len() || intIndex < 0 {
					return fmt.Errorf("there is no element at %s: index is out of range", path[:endIndex])
				}
				current = indexable.At(ctx, intIndex).(Serializable)
				return nil
			}

		not_an_index:
			iprops, ok := current.(IProps)
			if !ok {
				return fmt.Errorf("there is no element at %s", path[:endIndex])
			}
			propVal, isSerializable := iprops.Prop(ctx, segment).(Serializable)
			if !isSerializable {
				return ErrInvalidDBValuePropRetrieval
			}
			switch propVal.(type) {
			case *InoxFunction, *DynamicValue:
				return ErrInvalidDBValuePropRetrieval
			}
			current = propVal
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	urlHolder, ok := current.(UrlHolder)
	if ok {
		if url, ok := urlHolder.URL(); ok && url.Path() != path {
			//TODO: allow URL holders without URLs ?
			//should we set their URL ? what is the performance impact ?

			return nil, fmt.Errorf("an entity has been found at %s but it's URL'path is not equal to the specified path", path)
		}
	}

	return current, err
}

func (db *DatabaseIL) Close(ctx *Context) error {
	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}
	return db.inner.Close(ctx)
}

func (db *DatabaseIL) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "update_schema":
		return WrapGoMethod(db.UpdateSchema), true
	case "close":
		return WrapGoMethod(db.Close), true
	}
	return nil, false
}

func (db *DatabaseIL) Prop(ctx *Context, name string) Value {
	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}

	switch name {
	case "schema":
		if db.newSchemaSet.Load() {
			return db.newSchema
		}
		return db.initialSchema
	case "update_schema", "close":
	default:
		if db.schemaUpdateExpected {
			if !db.schemaUpdated.Load() {
				panic(ErrInvalidAccessSchemaNotUpdatedYet)
			}
		}

		val, ok := db.topLevelEntities[name]
		if ok {
			perm := db.topLevelEntitiesAccessPermissions[name]
			if err := ctx.CheckHasPermission(perm); err != nil {
				panic(err)
			}
			return val
		}
	}

	method, ok := db.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, db))
	}
	return method
}

func (*DatabaseIL) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (db *DatabaseIL) PropertyNames(ctx *Context) []string {
	return db.propertyNames
}
