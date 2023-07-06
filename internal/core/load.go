package core

import (
	"errors"
	"reflect"
	"sync"
)

var (
	loadInstanceFnregistry     = map[reflect.Type] /* pattern type*/ LoadInstanceFn{}
	loadInstanceFnRegistryLock sync.Mutex

	ErrNonUniqueLoadInstanceFnRegistration = errors.New("non unique load instance function registration")
	ErrLoadingRequireTransaction           = errors.New("loading a value requires a transaction")
	ErrTransactionsNotSupportedYet         = errors.New("transactions not supported yet")
	ErrFailedToLoadNonExistingValue        = errors.New("failed to non-existing value")
)

type SerializedValueStorage interface {
	BaseURL() URL
	GetSerialized(ctx *Context, key Path) (string, bool)
	Has(ctx *Context, key Path) bool
	SetSerialized(ctx *Context, key Path, serialized string)
	InsertSerialized(ctx *Context, key Path, serialized string)
}

type LoadInstanceFn func(ctx *Context, key Path, storage SerializedValueStorage, pattern Pattern) (UrlHolder, error)

func RegisterLoadInstanceFn(patternType reflect.Type, fn LoadInstanceFn) {
	loadInstanceFnRegistryLock.Lock()
	defer loadInstanceFnRegistryLock.Unlock()

	_, ok := loadInstanceFnregistry[patternType]
	if ok {
		panic(ErrNonUniqueLoadInstanceFnRegistration)
	}

	loadInstanceFnregistry[patternType] = fn
}

func getLoadInstanceFn(typ reflect.Type) (LoadInstanceFn, bool) {
	loadInstanceFnRegistryLock.Lock()
	defer loadInstanceFnRegistryLock.Unlock()

	fn, ok := loadInstanceFnregistry[typ]

	return fn, ok
}
