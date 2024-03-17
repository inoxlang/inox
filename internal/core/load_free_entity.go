package core

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	loadFreeEntityFnRegistry     = map[ /*pattern type*/ reflect.Type]LoadSelfManagedEntityFn{}
	loadFreeEntityFnRegistryLock sync.Mutex

	ErrNonUniqueLoadFreeEntityFnRegistration          = errors.New("non unique loading function registration")
	ErrNonUniqueGetSymbolicInitialFactoryRegistration = errors.New("non unique symbolic initial value factory registration")
	ErrNoLoadFreeEntityFnRegistered                   = errors.New("no loading function registered for given type")
	ErrLoadingRequireTransaction                      = errors.New("loading a value requires a transaction")
	ErrTransactionsNotSupportedYet                    = errors.New("transactions not supported yet")
	ErrFailedToLoadNonExistingValue                   = errors.New("failed to load non-existing value")

	ErrInvalidInitialValue = errors.New("invalid initial value")
)

func init() {
	resetLoadFreeEntityFnRegistry()
}

func resetLoadFreeEntityFnRegistry() {
	loadFreeEntityFnRegistryLock.Lock()
	clear(loadFreeEntityFnRegistry)
	loadFreeEntityFnRegistryLock.Unlock()

	RegisterLoadFreeEntityFn(OBJECT_PATTERN_TYPE, loadFreeObject)
}

type DataStore interface {
	BaseURL() URL
	GetSerialized(ctx *Context, key Path) (string, bool)
	Has(ctx *Context, key Path) bool
	SetSerialized(ctx *Context, key Path, serialized string)
	InsertSerialized(ctx *Context, key Path, serialized string)
}

type FreeEntityLoadingParams struct {
	Key          Path
	Storage      DataStore
	Pattern      Pattern
	InitialValue Serializable
	AllowMissing bool //if true the loading function is allowed to return an empty/default value matching the pattern
	Migration    *FreeEntityMigrationArgs
}

func (a FreeEntityLoadingParams) IsDeletion(ctx *Context) bool {
	if a.Migration == nil {
		return false
	}

	for pathPattern := range a.Migration.MigrationHandlers.Deletions {
		if pathPattern.Test(ctx, a.Key) {
			return true
		}
	}
	return false
}

type FreeEntityMigrationArgs struct {
	NextPattern       Pattern //can be nil
	MigrationHandlers MigrationOpHandlers
}

// LoadSelfManagedEntityFn should load a self-managed entity and should call the corresponding migration handlers.
// In the case of a deletion (nil, nil) should be returned.
// If the entity changes due to a migration this function should call LoadSelfManagedEntityFn
// with the new value passed in .InitialValue.
type LoadSelfManagedEntityFn func(ctx *Context, args FreeEntityLoadingParams) (UrlHolder, error)

func RegisterLoadFreeEntityFn(patternType reflect.Type, fn LoadSelfManagedEntityFn) {
	loadFreeEntityFnRegistryLock.Lock()
	defer loadFreeEntityFnRegistryLock.Unlock()

	_, ok := loadFreeEntityFnRegistry[patternType]
	if ok {
		panic(ErrNonUniqueLoadFreeEntityFnRegistration)
	}

	loadFreeEntityFnRegistry[patternType] = fn
}

func hasTypeLoadingFunction(pattern Pattern) bool {
	loadFreeEntityFnRegistryLock.Lock()
	defer loadFreeEntityFnRegistryLock.Unlock()

	_, ok := loadFreeEntityFnRegistry[reflect.TypeOf(pattern)]
	return ok
}

// See documentation of LoadFreeEntityFn.
func LoadFreeEntity(ctx *Context, args FreeEntityLoadingParams) (UrlHolder, error) {
	loadFreeEntityFnRegistryLock.Lock()

	patternType := reflect.TypeOf(args.Pattern)
	fn, ok := loadFreeEntityFnRegistry[patternType]
	loadFreeEntityFnRegistryLock.Unlock()

	if !ok {
		panic(ErrNoLoadFreeEntityFnRegistered)
	}

	if args.Key[len(args.Key)-1] == '/' {
		return nil, errors.New("free entity's key should not end with '/'")
	}

	return fn(ctx, args)
}

func loadFreeObject(ctx *Context, args FreeEntityLoadingParams) (UrlHolder, error) {
	path := args.Key
	pattern := args.Pattern
	storage := args.Storage

	objectPattern := pattern.(*ObjectPattern)

	object, ok := args.InitialValue.(*Object)
	if !ok && args.InitialValue != nil {
		return nil, fmt.Errorf("%w: an object is expected", ErrInvalidInitialValue)
	}

	if object == nil {
		serialized, ok := storage.GetSerialized(ctx, path)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrFailedToLoadNonExistingValue, path)
		}

		parsed, err := ParseJSONRepresentation(ctx, serialized, objectPattern)

		if err != nil {
			return nil, err
		}

		object, ok = parsed.(*Object)
		if !ok {
			return nil, fmt.Errorf("an object was expected")
		}
	} else { //initial value
		_, hasURL := object.URL()
		if hasURL {
			return nil, errors.New("initial object should not have a URL")
		}
	}

	_, hasURL := object.URL()
	if !hasURL {
		object.SetURLOnce(ctx, args.Storage.BaseURL().AppendAbsolutePath(path))
	}

	if args.InitialValue != nil {
		storage.SetSerialized(ctx, path, GetJSONRepresentation(object, ctx, pattern))
		if args.Migration != nil {
			panic(ErrUnreachable)
		}
	}

	//we perform the migration before adding mutation handlers for obvious reasons
	if args.Migration != nil {
		next, err := object.Migrate(ctx, args.Key, args.Migration)
		if err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}

		if args.IsDeletion(ctx) {
			return nil, nil
		}

		nextObject, ok := next.(*Object)
		if !ok || object != nextObject {
			return LoadFreeEntity(ctx, FreeEntityLoadingParams{
				Key:          args.Key,
				Storage:      args.Storage,
				Pattern:      args.Migration.NextPattern,
				InitialValue: next.(Serializable),
				AllowMissing: false,
				Migration:    nil,
			})
		}

		if args.Migration.NextPattern == nil {
			return nil, fmt.Errorf("missing next pattern for %s", path)
		}
		pattern = args.Migration.NextPattern
		updatedRepr := GetJSONRepresentation(object, ctx, pattern)
		storage.SetSerialized(ctx, path, updatedRepr)
	}

	object.Share(ctx.MustGetClosestState())

	//add mutation handlers
	object.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
		registerAgain = true
		updatedRepr := GetJSONRepresentation(object, ctx, pattern)
		storage.SetSerialized(ctx, path, updatedRepr)
		return
	}, MutationWatchingConfiguration{
		Depth: DeepWatching,
	})

	return object, nil
}

type TestValueStorage struct {
	BaseURL_ URL
	Data     map[Path]string
}

func (s *TestValueStorage) BaseURL() URL {
	return s.BaseURL_
}

func (s *TestValueStorage) GetSerialized(ctx *Context, key Path) (string, bool) {
	v, ok := s.Data[key]
	return v, ok
}

func (s *TestValueStorage) Has(ctx *Context, key Path) bool {
	_, ok := s.Data[key]
	return ok
}

func (s *TestValueStorage) InsertSerialized(ctx *Context, key Path, serialized string) {
	_, ok := s.Data[key]
	if !ok {
		panic(errors.New("already present"))
	}
	s.Data[key] = serialized
}

func (s *TestValueStorage) SetSerialized(ctx *Context, key Path, serialized string) {
	s.Data[key] = serialized
}
