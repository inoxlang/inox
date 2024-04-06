package staticcheck

import (
	"errors"
	"sync"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/parse"
)

var (
	staticallyCheckDbResolutionDataFnRegistry     = map[Scheme]DbResolutionDataCheckFn{}
	staticallyCheckDbResolutionDataFnRegistryLock sync.Mutex
)

type Scheme interface {
	IsDatabaseScheme() bool
}

type Host interface {
	inoxmod.ResourceName
	Name() string
}

type URL interface {
	inoxmod.ResourceName
	HasQueryOrFragment() bool
}

type Project interface {
}

type DbResolutionDataCheckFn func(node parse.Node, optProject Project) (errorMsg string)

func IsStaticallyCheckDBFunctionRegistered(scheme Scheme) bool {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	_, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]
	return ok
}

func ResetDbResolutionDataCheckFnRegistry() {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()
	clear(staticallyCheckDbResolutionDataFnRegistry)
}

func RegisterDbResolutionDataCheckFn(scheme Scheme, fn DbResolutionDataCheckFn) {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	_, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]
	if ok {
		panic(errors.New("non-unique registration of static checker for db resolution data"))
	}

	staticallyCheckDbResolutionDataFnRegistry[scheme] = fn
}

func GetStaticallyCheckDbResolutionDataFn(scheme Scheme) (DbResolutionDataCheckFn, bool) {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	fn, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]

	return fn, ok
}
