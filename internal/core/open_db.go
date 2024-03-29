package core

import (
	"errors"
	"sync"

	"github.com/inoxlang/inox/internal/parse"
)

var (
	openDbFnRegistry     = map[Scheme]OpenDBFn{}
	openDbFnRegistryLock sync.Mutex

	staticallyCheckDbResolutionDataFnRegistry     = map[Scheme]StaticallyCheckDbResolutionDataFn{}
	staticallyCheckDbResolutionDataFnRegistryLock sync.Mutex

	ErrNonUniqueDbOpenFnRegistration = errors.New("non unique open DB function registration")
)

type OpenDBFn func(ctx *Context, config DbOpenConfiguration) (Database, error)

type StaticallyCheckDbResolutionDataFn func(node parse.Node, optProject Project) (errorMsg string)

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

func IsStaticallyCheckDBFunctionRegistered(scheme Scheme) bool {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	_, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]
	return ok
}

func ResetStaticallyCheckDbResolutionDataFnRegistry() {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()
	clear(staticallyCheckDbResolutionDataFnRegistry)
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
