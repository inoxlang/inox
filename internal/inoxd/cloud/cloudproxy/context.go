package cloudproxy

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/permkind"
)

func createContext(host core.Host, proxyArgs CloudProxyArgs) (ctx, topCtx *core.Context) {
	perms := []core.Permission{
		core.WebsocketPermission{
			Kind_:    permkind.Provide,
			Endpoint: host,
		},
		core.WebsocketPermission{
			Kind_: permkind.Read,
		},
		core.WebsocketPermission{
			Kind_: permkind.Write,
		},
		core.FilesystemPermission{
			Kind_:  permkind.Read,
			Entity: core.PathFrom(proxyArgs.Config.AnonymousAccountDatabasePath),
		},
		core.FilesystemPermission{
			Kind_:  permkind.Write,
			Entity: core.PathFrom(proxyArgs.Config.AnonymousAccountDatabasePath),
		},
	}

	topCtx = core.NewContexWithEmptyState(core.ContextConfig{
		Filesystem:          fs_ns.GetOsFilesystem(),
		Permissions:         perms,
		ParentStdLibContext: proxyArgs.GoContext,
	}, proxyArgs.OutW)

	inoxprocess.RestrictProcessAccess(topCtx, inoxprocess.ProcessRestrictionConfig{
		ForceAllowDNS: true,
	})

	fls := fs_ns.NewMemFilesystem(1_000_000)
	ctx = core.NewContexWithEmptyState(core.ContextConfig{
		ParentContext: topCtx,
		Filesystem:    fls,
		Permissions:   perms,
	}, proxyArgs.OutW)

	return ctx, topCtx
}
