package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	openDbFnRegistry     = map[Scheme]OpenDBFn{}
	openDbFnRegistryLock sync.Mutex

	staticallyCheckDbResolutionDataFnRegistry     = map[Scheme]StaticallyCheckDbResolutionDataFn{}
	staticallyCheckDbResolutionDataFnRegistryLock sync.Mutex

	ErrNonUniqueDbOpenFnRegistration                = errors.New("non unique open DB function registration")
	ErrNameCollisionWithInitialDatabasePropertyName = errors.New("name collision with initial database property name")
	ErrTopLevelEntitiesAlreadyLoaded                = errors.New("top-level entities already loaded")
	ErrDatabaseSchemaOnlyUpdatableByOwnerState      = errors.New("database schema can only be updated by owner state")
	ErrNoneDatabaseSchemaUpdateExpected             = errors.New("none database schema update is expected")
	ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed     = errors.New("database schema already updated or no longer allowed")
	ErrInvalidAccessSchemaNotUpdatedYet             = errors.New("access to database is not allowed because schema is not updated yet")

	DATABASE_PROPNAMES = []string{"update_schema", "close", "schema"}

	_ Value    = (*DatabaseIL)(nil)
	_ Database = (*FailedToOpenDatabase)(nil)
)

type DatabaseIL struct {
	inner                Database
	initialSchema        *ObjectPattern
	schemaUpdateExpected bool
	schemaUpdated        atomic.Bool
	schemaUpdateLock     sync.Mutex
	ownerState           *GlobalState

	propertyNames    []string
	topLevelEntities map[string]Serializable
}

type DbOpenConfiguration struct {
	Resource       SchemeHolder
	ResolutionData Value
	FullAccess     bool
}

type OpenDBFn func(ctx *Context, config DbOpenConfiguration) (Database, error)

type StaticallyCheckDbResolutionDataFn func(node parse.Node) (errorMsg string)

type Database interface {
	Resource() SchemeHolder

	Schema() *ObjectPattern

	//UpdateSchema updates the schema and validates the content of the database,
	//this method should return ErrTopLevelEntitiesAlreadyLoaded if it is called after .TopLevelEntities.
	//The caller should always pass a schema whose ALL entry patterns have a loading function.
	UpdateSchema(ctx *Context, schema *ObjectPattern)

	TopLevelEntities(ctx *Context) map[string]Serializable
	Close(ctx *Context) error
}

type DatabaseWrappingArgs struct {
	Inner                Database
	OwnerState           *GlobalState
	ExpectedSchemaUpdate bool
}

func WrapDatabase(ctx *Context, args DatabaseWrappingArgs) *DatabaseIL {
	schema := args.Inner.Schema()

	propertyNames := utils.CopySlice(DATABASE_PROPNAMES)
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
		ownerState:           args.OwnerState,
	}

	if !args.ExpectedSchemaUpdate {
		db.topLevelEntities = args.Inner.TopLevelEntities(ctx)
	}

	return db
}

func RegisterOpenDbFn(scheme Scheme, fn OpenDBFn) {
	openDbFnRegistryLock.Lock()
	defer openDbFnRegistryLock.Unlock()

	_, ok := openDbFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	openDbFnRegistry[scheme] = fn
}

func GetOpenDbFn(scheme Scheme) (OpenDBFn, bool) {
	openDbFnRegistryLock.Lock()
	defer openDbFnRegistryLock.Unlock()

	//TODO: prevent re-opening database (same resolution data)
	fn, ok := openDbFnRegistry[scheme]

	return fn, ok
}

func RegisterStaticallyCheckDbResolutionDataFn(scheme Scheme, fn StaticallyCheckDbResolutionDataFn) {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	_, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	staticallyCheckDbResolutionDataFnRegistry[scheme] = fn
}

func GetStaticallyCheckDbResolutionDataFn(scheme Scheme) (StaticallyCheckDbResolutionDataFn, bool) {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	fn, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]

	return fn, ok
}

func (db *DatabaseIL) Resource() SchemeHolder {
	return db.inner.Resource()
}

func (db *DatabaseIL) UpdateSchema(ctx *Context, schema *ObjectPattern) {
	if !db.schemaUpdateExpected {
		panic(ErrNoneDatabaseSchemaUpdateExpected)
	}

	if ctx.GetClosestState() != db.ownerState {
		panic(ErrDatabaseSchemaOnlyUpdatableByOwnerState)
	}

	db.schemaUpdateLock.Lock()
	defer db.schemaUpdateLock.Unlock()

	if db.schemaUpdated.Load() {
		panic(ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed)
	}

	defer db.schemaUpdated.Store(true)

	err := schema.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if !hasTypeLoadingFunction(propPattern) {
			return fmt.Errorf("failed to update schema: pattern of .%s has no loading function: %w", propName, ErrNoLoadInstanceFnRegistered)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	db.inner.UpdateSchema(ctx, schema)
	db.topLevelEntities = db.inner.TopLevelEntities(ctx)
}

func (db *DatabaseIL) Close(ctx *Context) error {
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
	switch name {
	case "schema":
		return db.initialSchema
	}

	if db.schemaUpdateExpected {
		if !db.schemaUpdated.Load() {
			panic(ErrInvalidAccessSchemaNotUpdatedYet)
		}
	}

	val, ok := db.topLevelEntities[name]
	if ok {
		return val
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

type FailedToOpenDatabase struct {
	resource SchemeHolder
}

func NewFailedToOpenDatabase(resource SchemeHolder) *FailedToOpenDatabase {
	return &FailedToOpenDatabase{resource: resource}
}

func (db *FailedToOpenDatabase) Resource() SchemeHolder {
	return db.resource
}

func (db *FailedToOpenDatabase) Schema() *ObjectPattern {
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *FailedToOpenDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern) {
	panic(ErrNotImplemented)
}

func (db *FailedToOpenDatabase) TopLevelEntities(_ *Context) map[string]Serializable {
	return nil
}

func (db *FailedToOpenDatabase) Close(ctx *Context) error {
	return ErrNotImplemented
}

type dummyDatabase struct {
	resource         SchemeHolder
	schemaUpdated    bool
	topLevelEntities map[string]Serializable
}

func (db *dummyDatabase) Resource() SchemeHolder {
	return db.resource
}

func (db *dummyDatabase) Schema() *ObjectPattern {
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *dummyDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern) {
	if db.schemaUpdated {
		panic(errors.New("schema already updated"))
	}
	db.schemaUpdated = true
}

func (db *dummyDatabase) TopLevelEntities(_ *Context) map[string]Serializable {
	return db.topLevelEntities
}

func (db *dummyDatabase) Close(ctx *Context) error {
	return ErrNotImplemented
}
