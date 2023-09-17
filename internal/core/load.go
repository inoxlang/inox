package core

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	loadInstanceFnregistry     = map[reflect.Type] /*pattern type*/ LoadInstanceFn{}
	loadInstanceFnRegistryLock sync.Mutex

	ErrNonUniqueLoadInstanceFnRegistration            = errors.New("non unique loading function registration")
	ErrNonUniqueGetSymbolicInitialFactoryRegistration = errors.New("non unique symbolic initial value factory registration")
	ErrNoLoadInstanceFnRegistered                     = errors.New("no loading function registered for given type")
	ErrLoadingRequireTransaction                      = errors.New("loading a value requires a transaction")
	ErrTransactionsNotSupportedYet                    = errors.New("transactions not supported yet")
	ErrFailedToLoadNonExistingValue                   = errors.New("failed to load non-existing value")

	ErrInvalidInitialValue = errors.New("invalid initial value")
)

func init() {
	resetLoadInstanceFnRegistry()
}

func resetLoadInstanceFnRegistry() {
	loadInstanceFnRegistryLock.Lock()
	clear(loadInstanceFnregistry)
	loadInstanceFnRegistryLock.Unlock()

	RegisterLoadInstanceFn(OBJECT_PATTERN_TYPE, loadObject)
}

type SerializedValueStorage interface {
	BaseURL() URL
	GetSerialized(ctx *Context, key Path) (string, bool)
	Has(ctx *Context, key Path) bool
	SetSerialized(ctx *Context, key Path, serialized string)
	InsertSerialized(ctx *Context, key Path, serialized string)
}

type InstanceLoadArgs struct {
	Key          Path
	Storage      SerializedValueStorage
	Pattern      Pattern
	InitialValue Serializable
	AllowMissing bool //if true the loading function is allowed to return an empty/default value matching the pattern
	Migration    *InstanceMigrationArgs
}

func (a InstanceLoadArgs) IsDeletion(ctx *Context) bool {
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

type InstanceMigrationArgs struct {
	NextPattern       Pattern //can be nil
	MigrationHandlers MigrationOpHandlers
}

// LoadInstanceFn should load the associated value & call the corresponding migration handlers, in the case
// of a deletion (nil, nil) should be returned.
// If the value changes due to a migration this function should call LoadInstance with the new value passed in .InitialValue.
type LoadInstanceFn func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error)

func RegisterLoadInstanceFn(patternType reflect.Type, fn LoadInstanceFn) {
	loadInstanceFnRegistryLock.Lock()
	defer loadInstanceFnRegistryLock.Unlock()

	_, ok := loadInstanceFnregistry[patternType]
	if ok {
		panic(ErrNonUniqueLoadInstanceFnRegistration)
	}

	loadInstanceFnregistry[patternType] = fn
}

func hasTypeLoadingFunction(pattern Pattern) bool {
	loadInstanceFnRegistryLock.Lock()
	defer loadInstanceFnRegistryLock.Unlock()

	_, ok := loadInstanceFnregistry[reflect.TypeOf(pattern)]
	return ok
}

func LoadInstance(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
	loadInstanceFnRegistryLock.Lock()

	patternType := reflect.TypeOf(args.Pattern)
	fn, ok := loadInstanceFnregistry[patternType]
	loadInstanceFnRegistryLock.Unlock()

	if !ok {
		panic(ErrNoLoadInstanceFnRegistered)
	}

	if args.Key[len(args.Key)-1] == '/' {
		return nil, errors.New("instance key should not end with '/'")
	}

	return fn(ctx, args)
}

func loadObject(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
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
			return LoadInstance(ctx, InstanceLoadArgs{
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

	object.Share(ctx.GetClosestState())

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
