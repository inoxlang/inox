package http_ns

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
)

// NewFileServer returns an HttpServer that uses Go's http.FileServer(dir) to handle requests
func NewFileServer(ctx *core.Context, args ...core.Value) (*HttpServer, error) {
	var addr string
	var dir core.Path

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			if addr != "" {
				return nil, errors.New("address already provided")
			}
			parsed, _ := url.Parse(string(v))
			addr = parsed.Host

			perm := core.HttpPermission{Kind_: permkind.Provide, Entity: v}
			if err := ctx.CheckHasPermission(perm); err != nil {
				return nil, err
			}
		case core.Path:
			if !v.IsDirPath() {
				return nil, errors.New("the directory path should end with '/'")
			}
			var err error
			dir, err = v.ToAbs(ctx.GetFileSystem())
			if err != nil {
				return nil, err
			}

			perm := core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(string(dir) + "...")}
			if err := ctx.CheckHasPermission(perm); err != nil {
				return nil, err
			}
		default:
		}
	}

	if addr == "" {
		return nil, errors.New("no address provided")
	}

	if dir == "" {
		return nil, errors.New("no (directory) path required")
	}

	server, err := NewGolangHttpServer(ctx, GolangHttpServerConfig{
		Addr:    addr,
		Handler: http.FileServer(http.Dir(dir)),
	})
	if err != nil {
		return nil, err
	}

	endChan := make(chan struct{}, 1)

	go func() {
		log.Println(server.ListenAndServeTLS("", ""))
		endChan <- struct{}{}
	}()

	time.Sleep(5 * time.Millisecond)
	return &HttpServer{
		wrappedServer: server,
		endChan:       endChan,
	}, nil
}

func serveFile(ctx *core.Context, rw *HttpResponseWriter, r *HttpRequest, pth core.Path) error {
	{
		var err error
		pth, err = pth.ToAbs(ctx.GetFileSystem())
		if err != nil {
			return err
		}
	}

	perm := core.FilesystemPermission{Kind_: permkind.Read, Entity: pth}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	http.ServeFile(rw.rw, r.Request(), string(pth))
	return nil
}
