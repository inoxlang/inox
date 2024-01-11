package http_ns

import (
	"fmt"
	"sync"

	"github.com/inoxlang/inox/internal/core"
)

type preparedModules struct {
	entries    map[string] /*file path*/ *preparedModule
	lock       sync.Mutex
	creatorCtx *core.Context
}

func newPreparedModules(ctx *core.Context) *preparedModules {
	modules := &preparedModules{
		creatorCtx: ctx,
		entries:    map[string]*preparedModule{},
	}
	return modules
}

// prepareModulesInFolder prepares all the modules referenced by the API operations.
func (c *preparedModules) prepareFrom(api *API) error {

	c.lock.Lock()
	defer c.lock.Unlock()

	return api.ForEachHandlerModule(func(mod *core.Module) error {
		name := mod.MainChunk.Name()
		if name == "" || name[0] != '/' {
			return fmt.Errorf("name of module's main chunk should be an absolute path: %s", name)
		}

		fpath := name
		preparedModule := &preparedModule{
			fpath:  fpath,
			module: mod,
		}
		c.entries[fpath] = preparedModule
		return nil
	})
}

type preparedModule struct {
	fpath  string
	lock   sync.Mutex
	module *core.Module
}
