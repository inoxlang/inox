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

type OpenDBFn func(ctx *Context, config DbOpenConfiguration) (Database, error)

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
