package devtools

import (
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/core"
)

var (
	_ = core.DevAPI((*API)(nil))
)

// An API is used internally by an Inox web application to provide development tools.
// This type implements the core.Value interface.
type API struct {
	instance *Instance
}

func (a *API) DevAPI__() {

}

func (a *API) getDB(ctx *core.Context, name core.String) (*dbProxy, error) {
	a.instance.lock.Lock()
	defer a.instance.lock.Unlock()

	nameS := string(name)

	proxy, ok := a.instance.dbProxies[nameS]
	if ok {
		return proxy, nil
	}

	proxy = newDBProxy(nameS, a.instance)

	lockSession := false
	_, err := proxy.dbNoLock(lockSession)
	if err != nil {
		return nil, err
	}

	a.instance.dbProxies[nameS] = proxy

	return proxy, nil
}

func (a *API) getDatabaseNames(_ *core.Context) *core.List {
	a.instance.lock.Lock()
	defer a.instance.lock.Unlock()

	names := map[string]struct{}{}

	for name := range a.instance.dbProxies {
		names[name] = struct{}{}
	}

	for name := range a.instance.runningProgramDatabases {
		names[name] = struct{}{}
	}

	var nameSlice []core.StringLike
	for name := range names {
		nameSlice = append(nameSlice, core.String(name))
	}

	slices.SortFunc(nameSlice, func(a, b core.StringLike) int {
		return strings.Compare(a.GetOrBuildString(), b.GetOrBuildString())
	})

	return core.NewStringLikeFrom(nameSlice)
}
