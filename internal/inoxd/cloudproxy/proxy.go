package cloudproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/project_server"
)

const (
	CLOUD_PROXY_SUBCMD_NAME = "cloud-proxy"
)

func Run(outW, errW io.Writer) error {
	addr := "localhost:" + project_server.DEFAULT_PROJECT_SERVER_PORT
	host := core.Host("wss://" + addr)

	ctx := createContext(host, outW, errW)
	wsServer, err := net_ns.NewWebsocketServer(ctx)

	if err != nil {
		return err
	}

	httpServer, err := http_ns.NewGolangHttpServer(ctx, http_ns.GolangHttpServerConfig{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				_, err := wsServer.UpgradeGoValues(w, r, allowConnection)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}
		}),
	})

	if err != nil {
		return err
	}

	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return httpServer.Shutdown(ctx)
	})

	ctx.OnDone(func(timeoutCtx context.Context) error {
		return httpServer.Close()
	})

	fmt.Fprintf(outW, "start cloud proxy HTTPS server listening on %s\n", addr)

	err = httpServer.ListenAndServeTLS("", "")
	if err != nil {
		return fmt.Errorf("failed to create HTTPS server for cloud proxy: %w", err)
	}

	return nil
}

func createContext(host core.Host, outW, errW io.Writer) *core.Context {
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
	}

	topCtx := core.NewContexWithEmptyState(core.ContextConfig{
		Filesystem:  fs_ns.GetOsFilesystem(),
		Permissions: perms,
	}, outW)
	defer topCtx.CancelGracefully()

	inoxprocess.RestrictProcessAccess(topCtx, inoxprocess.ProcessRestrictionConfig{
		ForceAllowDNS: true,
	})

	fls := fs_ns.NewMemFilesystem(1_000_000)
	ctx := core.NewContexWithEmptyState(core.ContextConfig{
		ParentContext: topCtx,
		Filesystem:    fls,
		Permissions:   perms,
	}, outW)

	return ctx
}

func allowConnection(remoteAddrPort nettypes.RemoteAddrWithPort, remoteAddr nettypes.RemoteIpAddr, currentConns []*net_ns.WebsocketConnection) error {
	return nil
}
