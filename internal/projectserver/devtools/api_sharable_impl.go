package devtools

import "github.com/inoxlang/inox/internal/core"

func (a *API) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (a *API) IsShared() bool {
	return true
}

func (a *API) Share(originState *core.GlobalState) {
	//no-op
}

func (a *API) SmartLock(*core.GlobalState) {
	//no-op
}

func (a *API) SmartUnlock(*core.GlobalState) {
	//no-op
}
