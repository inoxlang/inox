package cloudproxy

import (
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
)

func createContexts(host core.Host, proxyArgs CloudProxyArgs) (ctx, topCtx *core.Context) {
	databaseDir := core.DirPathFrom(filepath.Dir(proxyArgs.Config.AnonymousAccountDatabasePath))
	databaseDirPattern := databaseDir.ToPrefixPattern()

	perms := []core.Permission{
		core.WebsocketPermission{
			Kind_:    permbase.Provide,
			Endpoint: host,
		},
		core.WebsocketPermission{
			Kind_: permbase.Read,
		},
		core.WebsocketPermission{
			Kind_: permbase.Write,
		},
		core.FilesystemPermission{
			Kind_:  permbase.Read,
			Entity: databaseDirPattern,
		},
		core.FilesystemPermission{
			Kind_:  permbase.Write,
			Entity: databaseDirPattern,
		},
	}

	topCtx = core.NewContextWithEmptyState(core.ContextConfig{
		Filesystem:          proxyArgs.Filesystem,
		Permissions:         perms,
		ParentStdLibContext: proxyArgs.GoContext,
	}, proxyArgs.OutW)

	ctx = core.NewContextWithEmptyState(core.ContextConfig{
		ParentContext: topCtx,
		Permissions:   perms,
	}, proxyArgs.OutW)

	return ctx, topCtx
}
