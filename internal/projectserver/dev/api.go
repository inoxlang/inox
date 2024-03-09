package dev

import (
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

	_, err := proxy.dbNoLock()
	if err != nil {
		return nil, err
	}

	a.session.dbProxies[nameS] = proxy

	return proxy, nil
}
