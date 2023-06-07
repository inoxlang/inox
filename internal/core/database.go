package core

import (
	"errors"
	"sync"
)

var (
	openDbFnRegistry     = map[Scheme]OpenDBFn{}
	openDbFnRegistryLock sync.Mutex

	ErrNonUniqueDbOpenFnRegistration = errors.New("non unique open DB function registration")
)

type OpenDBFn func(ctx *Context, resource SchemeHolder, resolutionData Value) (Database, error)

type Database interface {
	Value
	Resource() ResourceName //url or host
	Close(ctx *Context) error
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

	fn, ok := openDbFnRegistry[scheme]

	return fn, ok
}
