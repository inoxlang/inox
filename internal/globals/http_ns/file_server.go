package http_ns

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
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
		defer func() {
			recover()
			endChan <- struct{}{}
		}()

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			ctx.Logger().Err(err).Send()
		}
	}()

	time.Sleep(5 * time.Millisecond)
	return &HttpServer{
		wrappedServer: server,
		endChan:       endChan,
	}, nil
}

// serveFile opens the file with the given path and calls http.ServeContent,
// an error is returned in the following cases:
// - permission error (read perm is required).
// - failure to open the file.
// - failure to get creation & modification time from file info.
// Note that errors can still happen during the call to http.ServeContent but they are written to the *http.ResponseWriter.
func serveFile(ctx *core.Context, rw *HttpResponseWriter, r *HttpRequest, pth core.Path) error {

	fls := ctx.GetFileSystem()
	{
		var err error
		pth, err = pth.ToAbs(fls)
		if err != nil {
			return err
		}
	}

	perm := core.FilesystemPermission{Kind_: permkind.Read, Entity: pth}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	f, err := fls.Open(string(pth))
	if err != nil {
		return err
	}

	//TODO: add implementation of afs.StatCapable to all file types in fs_ns.

	stat, err := core.FileStat(f, fls)
	if err != nil {
		return err
	}

	_, modif, err := fs_ns.GetCreationAndModifTime(stat)
	if err != nil {
		return err
	}

	//TODO: pass a custom response writer that logs error and return a 404 status code.
	http.ServeContent(rw.rw, r.Request(), string(pth), modif, f)
	return nil
}
