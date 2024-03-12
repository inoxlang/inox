package dev

import (
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/core"
)

var (
	_ = core.DevAPI((*API)(nil))
)

type API struct {
	session *Session
}

func (a *API) DevAPI__() {

}

func (a *API) getDB(ctx *core.Context, name core.String) (*dbProxy, error) {
	a.session.lock.Lock()
	defer a.session.lock.Unlock()

	nameS := string(name)

	proxy, ok := a.session.dbProxies[nameS]
	if ok {
		return proxy, nil
	}

	proxy = newDBProxy(nameS, a.session)

	lockSession := false
	_, err := proxy.dbNoLock(lockSession)
	if err != nil {
		return nil, err
	}

	a.session.dbProxies[nameS] = proxy

	return proxy, nil
}

func (a *API) getDatabaseNames(_ *core.Context) *core.List {
	a.session.lock.Lock()
	defer a.session.lock.Unlock()

	names := map[string]struct{}{}

	for name := range a.session.dbProxies {
		names[name] = struct{}{}
	}

	for name := range a.session.runningProgramDatabases {
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
