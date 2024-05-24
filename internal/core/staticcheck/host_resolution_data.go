package staticcheck

import (
	"errors"
	"sync"

	"github.com/inoxlang/inox/internal/ast"
)

var (
	staticallyCheckHostDefinitionDataFnRegistry = map[Scheme]HostDefinitionCheckFn{}
	staticallyCheckHostDefinitionFnRegistryLock sync.Mutex
)

type HostDefinitionCheckFn func(node ast.Node) (errorMsg string)

func ResetHostDefinitionDataCheckFnRegistry() {
	staticallyCheckHostDefinitionFnRegistryLock.Lock()
	clear(staticallyCheckHostDefinitionDataFnRegistry)
	staticallyCheckHostDefinitionFnRegistryLock.Unlock()
}

func RegisterStaticallyCheckHostDefinitionFn(scheme Scheme, fn HostDefinitionCheckFn) {
	staticallyCheckHostDefinitionFnRegistryLock.Lock()
	defer staticallyCheckHostDefinitionFnRegistryLock.Unlock()

	_, ok := staticallyCheckHostDefinitionDataFnRegistry[scheme]
	if ok {
		panic(errors.New("non-unique registration of static checker for host definition data"))
	}

	staticallyCheckHostDefinitionDataFnRegistry[scheme] = fn
}
