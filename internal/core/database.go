package core

import (
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	openDbFnRegistry     = map[Scheme]OpenDBFn{}
	openDbFnRegistryLock sync.Mutex

	staticallyCheckDbResolutionDataFnRegistry     = map[Scheme]StaticallyCheckDbResolutionDataFn{}
	staticallyCheckDbResolutionDataFnRegistryLock sync.Mutex

	ErrNonUniqueDbOpenFnRegistration                   = errors.New("non unique open DB function registration")
	ErrOwnerStateAlreadySet                            = errors.New("owner state already set")
	ErrOwnerStateNotSet                                = errors.New("owner state not set")
	ErrNameCollisionWithInitialDatabasePropertyName    = errors.New("name collision with initial database property name")
	ErrTopLevelEntityNamesShouldBeValidInoxIdentifiers = errors.New("top-level entity names should be valid Inox identifiers (e.g., users, client-names)")
	ErrTopLevelEntitiesAlreadyLoaded                   = errors.New("top-level entities already loaded")
	ErrDatabaseSchemaOnlyUpdatableByOwnerState         = errors.New("database schema can only be updated by owner state")
	ErrNoDatabaseSchemaUpdateExpected                  = errors.New("no database schema update is expected")
	ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed        = errors.New("database schema already updated or no longer allowed")
	ErrInvalidAccessSchemaNotUpdatedYet                = errors.New("access to database is not allowed because schema is not updated yet")

	DATABASE_PROPNAMES = []string{"update_schema", "close", "schema"}

	ElementKeyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

	_ Value    = (*DatabaseIL)(nil)
	_ Database = (*FailedToOpenDatabase)(nil)
)

type DatabaseIL struct {
	inner                Database
	initialSchema        *ObjectPattern
	schemaUpdateExpected bool
	schemaUpdated        atomic.Bool
	schemaUpdateLock     sync.Mutex
	ownerState           *GlobalState //optional, can be set later using .SetOwnerStateOnce

	propertyNames    []string
	topLevelEntities map[string]Serializable
}

type DbOpenConfiguration struct {
	Resource       SchemeHolder
	ResolutionData Value
	FullAccess     bool
}

func checkDatabaseSchema(pattern *ObjectPattern) error {
	return pattern.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		expr, ok := parse.ParseExpression(propName)
		_, isIdent := expr.(*parse.IdentifierLiteral)
		if !ok || !isIdent {
			return fmt.Errorf("invalid top-level entity name: %q, %w", propName, ErrTopLevelEntityNamesShouldBeValidInoxIdentifiers)
		}

		if !hasTypeLoadingFunction(propPattern) {
			return fmt.Errorf("invalid pattern for top level entity .%s: %w", propName, ErrNoLoadInstanceFnRegistered)
		}
		return nil
	})
}

// An element key is a a string that:
// is at most 100-character long
// is not empty
// can only contain identifier chars (parse.IsIdentChar)
type ElementKey string

func ElementKeyFrom(key string) (ElementKey, error) {
	fmtErr := func(msg string) error {
		return fmt.Errorf("provided key %q is not a valid element key: %s", key, msg)
	}
	if len(key) == 0 {
		return "", fmtErr("empty")
	}

	if len(key) > 100 {
		return "", fmtErr("too long")
	}

	for _, r := range key {
		if !parse.IsIdentChar(r) {
			return "", fmtErr("invalid char found")
		}
	}
	return ElementKey(key), nil
}

func MustElementKeyFrom(key string) ElementKey {
	return utils.Must(ElementKeyFrom(key))
}

type OpenDBFn func(ctx *Context, config DbOpenConfiguration) (Database, error)

type StaticallyCheckDbResolutionDataFn func(node parse.Node) (errorMsg string)

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
	Inner                Database
	OwnerState           *GlobalState
	ExpectedSchemaUpdate bool

	//force the loading top level entities if there is not expected schema update
	ForceLoadBeforeOwnerStateSet bool
}

func WrapDatabase(ctx *Context, args DatabaseWrappingArgs) (*DatabaseIL, error) {
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

	if !args.ExpectedSchemaUpdate && args.ForceLoadBeforeOwnerStateSet {
		topLevelEntities, err := args.Inner.LoadTopLevelEntities(ctx)
		if err != nil {
			return nil, err
		}
		db.topLevelEntities = topLevelEntities
	}
	return db, nil
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

func (db *DatabaseIL) SetOwnerStateOnceAndLoadIfNecessary(ctx *Context, state *GlobalState) error {
	if db.ownerState != nil {
		panic(ErrOwnerStateAlreadySet)
	}
	if db.topLevelEntities == nil {
		topLevelEntities, err := db.inner.LoadTopLevelEntities(ctx)
		if err != nil {
			return err
		}
		db.topLevelEntities = topLevelEntities
	}
	db.ownerState = state
	return nil
}

func (db *DatabaseIL) Resource() SchemeHolder {
	return db.inner.Resource()
}

type MigrationOpHandlers struct {
	Deletions       map[PathPattern]*MigrationOpHandler //handler can be nil
	Inclusions      map[PathPattern]*MigrationOpHandler
	Replacements    map[PathPattern]*MigrationOpHandler
	Initializations map[PathPattern]*MigrationOpHandler
}

func (handlers MigrationOpHandlers) FilterTopLevel() MigrationOpHandlers {
	filtered := MigrationOpHandlers{}

	for pattern, handler := range handlers.Deletions {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Deletions == nil {
			filtered.Deletions = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Deletions[pattern] = handler
	}

	for pattern, handler := range handlers.Inclusions {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Inclusions == nil {
			filtered.Inclusions = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Inclusions[pattern] = handler
	}

	for pattern, handler := range handlers.Replacements {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Replacements == nil {
			filtered.Replacements = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Replacements[pattern] = handler
	}

	for pattern, handler := range handlers.Initializations {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Initializations == nil {
			filtered.Initializations = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Initializations[pattern] = handler
	}

	return filtered
}

func (handlers MigrationOpHandlers) FilterByPrefix(path Path) MigrationOpHandlers {
	filtered := MigrationOpHandlers{}

	prefix := string(path)
	prefixSlash := string(prefix)
	prefixNoSlash := string(prefix)

	if prefixSlash[len(prefixSlash)-1] != '/' {
		prefixSlash += "/"
	} else if prefixNoSlash != "/" {
		prefixNoSlash = prefixNoSlash[:len(prefixNoSlash)-1]
	}

	// if prefix is /users:
	// /users will match
	// /users/x will match
	// /users-x will not match
	matchedBy := func(pattern PathPattern) bool {
		if pattern.IsPrefixPattern() {
			panic(ErrUnreachable)
		}

		patternString := string(pattern)

		prefixPattern := patternString
		//remove trailing slash
		if prefixPattern != "/" && prefixPattern[len(prefixPattern)-1] == '/' {
			prefixPattern = prefixPattern[:len(prefixPattern)-1]
		}

		slashCount := strings.Count(prefixNoSlash, "/")
		patternSlashCount := strings.Count(prefixPattern, "/")

		if patternSlashCount < slashCount {
			return false
		}

		for i := 0; i < patternSlashCount-slashCount; i++ {
			index := strings.LastIndex(prefixPattern, "/")
			prefixPattern = prefixPattern[:index]
		}

		if prefixNoSlash == prefixPattern || strings.HasPrefix(prefixPattern, prefixSlash) {
			return true
		}
		return PathPattern(prefixPattern).Test(nil, path)
	}

	for pattern, handler := range handlers.Deletions {

		if matchedBy(pattern) {
			if filtered.Deletions == nil {
				filtered.Deletions = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Deletions[pattern] = handler
		}
	}

	for pattern, handler := range handlers.Inclusions {
		if matchedBy(pattern) {
			if filtered.Inclusions == nil {
				filtered.Inclusions = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Inclusions[pattern] = handler
		}
	}

	for pattern, handler := range handlers.Replacements {
		if matchedBy(pattern) {
			if filtered.Replacements == nil {
				filtered.Replacements = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Replacements[pattern] = handler
		}
	}

	for pattern, handler := range handlers.Initializations {
		if matchedBy(pattern) {
			if filtered.Initializations == nil {
				filtered.Initializations = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Initializations[pattern] = handler
		}
	}

	return filtered
}

type MigrationOpHandler struct {
	//ignored if InitialValue is set
	Function     *InoxFunction
	InitialValue Serializable
}

func (h MigrationOpHandler) GetResult(ctx *Context, state *GlobalState) Value {
	if h.Function != nil {
		return utils.Must(h.Function.Call(state, nil, []Value{}, nil))
	} else {
		return utils.Must(RepresentationBasedClone(ctx, h.InitialValue))
	}
}

func (db *DatabaseIL) UpdateSchema(ctx *Context, nextSchema *ObjectPattern, migrations ...*Object) {
	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}

	if !db.schemaUpdateExpected {
		panic(ErrNoDatabaseSchemaUpdateExpected)
	}

	if ctx.GetClosestState() != db.ownerState {
		panic(ErrDatabaseSchemaOnlyUpdatableByOwnerState)
	}

	//this check is also run during symbolic evaluation
	if err := checkDatabaseSchema(nextSchema); err != nil {
		panic(err)
	}

	db.schemaUpdateLock.Lock()
	defer db.schemaUpdateLock.Unlock()

	if db.schemaUpdated.Load() {
		panic(ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed)
	}

	defer db.schemaUpdated.Store(true)

	err := nextSchema.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if !hasTypeLoadingFunction(propPattern) {
			return fmt.Errorf("failed to update schema: pattern of .%s has no loading function: %w", propName, ErrNoLoadInstanceFnRegistered)
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

func (db *FailedToOpenDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationOpHandlers) {
	panic(ErrNotImplemented)
}

func (db *FailedToOpenDatabase) LoadTopLevelEntities(_ *Context) (map[string]Serializable, error) {
	return nil, nil
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

func (db *dummyDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationOpHandlers) {
	if db.schemaUpdated {
		panic(errors.New("schema already updated"))
	}
	db.schemaUpdated = true

	state := ctx.GetClosestState()

	if len(handlers.Deletions)+len(handlers.Initializations)+len(handlers.Replacements) > 0 {
		panic(errors.New("only inclusion handlers are supported"))
	}

	for pattern, handler := range handlers.Inclusions {
		if strings.Count(string(pattern), "/") != 1 {
			panic(errors.New("only shallow inclusion handlers are supported"))
		}
		result := handler.GetResult(ctx, state)
		db.topLevelEntities[string(pattern[1:])] = result.(Serializable)
	}
}

func (db *dummyDatabase) LoadTopLevelEntities(_ *Context) (map[string]Serializable, error) {
	return db.topLevelEntities, nil
}

func (db *dummyDatabase) Close(ctx *Context) error {
	return ErrNotImplemented
}
