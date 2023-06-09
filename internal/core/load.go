package core

import (
	"errors"
	"reflect"
	"sync"
)

var (
	loadInstanceFnregistry     = map[reflect.Type] /* pattern type*/ LoadInstanceFn{}
	loadInstanceFnRegistryLock sync.Mutex

	ErrNonUniqueLoadInstanceFnRegistration = errors.New("non unique loading function registration")
	ErrNoLoadInstanceFnRegistered          = errors.New("no loading function registered for given type")
	ErrLoadingRequireTransaction           = errors.New("loading a value requires a transaction")
	ErrTransactionsNotSupportedYet         = errors.New("transactions not supported yet")
	ErrFailedToLoadNonExistingValue        = errors.New("failed to load non-existing value")
)

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
	AllowMissing bool
}

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

	return fn(ctx, args)
}
