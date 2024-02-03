package core

import (
	"sync"

	"github.com/inoxlang/inox/internal/parse"
)

var (
	staticallyCheckHostDefinitionDataFnRegistry = map[Scheme]StaticallyCheckHostDefinitionFn{}
	staticallyCheckHostDefinitionFnRegistryLock sync.Mutex
)

type StaticallyCheckHostDefinitionFn func(optionalProject Project, node parse.Node) (errorMsg string)

func resetStaticallyCheckHostDefinitionDataFnRegistry() {
	staticallyCheckHostDefinitionFnRegistryLock.Lock()
	clear(staticallyCheckHostDefinitionDataFnRegistry)
	staticallyCheckHostDefinitionFnRegistryLock.Unlock()
}

func RegisterStaticallyCheckHostDefinitionFn(scheme Scheme, fn StaticallyCheckHostDefinitionFn) {
	staticallyCheckHostDefinitionFnRegistryLock.Lock()
	defer staticallyCheckHostDefinitionFnRegistryLock.Unlock()

	_, ok := staticallyCheckHostDefinitionDataFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	staticallyCheckHostDefinitionDataFnRegistry[scheme] = fn
}
