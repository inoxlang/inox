package core

import (
	"sync"

	parse "github.com/inoxlang/inox/internal/parse"
)

var (
	staticallyCheckHostResolutionDataFnRegistry     = map[Scheme]StaticallyCheckHostResolutionDataFn{}
	staticallyCheckHostResolutionDataFnRegistryLock sync.Mutex
)

type StaticallyCheckHostResolutionDataFn func(optionalProject Project, node parse.Node) (errorMsg string)

func resetStaticallyCheckHostResolutionDataFnRegistry() {
	staticallyCheckHostResolutionDataFnRegistryLock.Lock()
	clear(staticallyCheckHostResolutionDataFnRegistry)
	staticallyCheckHostResolutionDataFnRegistryLock.Unlock()
}

func RegisterStaticallyCheckHostResolutionDataFn(scheme Scheme, fn StaticallyCheckHostResolutionDataFn) {
	staticallyCheckHostResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckHostResolutionDataFnRegistryLock.Unlock()

	_, ok := staticallyCheckHostResolutionDataFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	staticallyCheckHostResolutionDataFnRegistry[scheme] = fn
}
