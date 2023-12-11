package http_ns

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

// NewFileServer returns an HttpServer that uses Go's http.FileServer(dir) to handle requests
func NewFileServer(ctx *core.Context, args ...core.Value) (*HttpsServer, error) {
	var addr string
	var dir core.Path
	fls := ctx.GetFileSystem()

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			if v.Scheme() != "https" {
				return nil, fmt.Errorf("invalid scheme '%s', only https is supported", v.Scheme())
			}

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
		Addr:                    addr,
		PersistCreatedLocalCert: true,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isDirURL := r.URL.Path[len(r.URL.Path)-1] == '/'
			filesystemPath := fls.Join(dir.UnderlyingString(), r.URL.Path)

			if isDirURL {
				//send HTML

				entries, err := fls.ReadDir(filesystemPath)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}

				w.Header().Add("Content-Type", mimeconsts.HTML_CTYPE)
				w.WriteHeader(200)
				w.Write(utils.StringAsBytes(`<html><head></head><body><pre>`))
				for _, entry := range entries {
					name := entry.Name()
					fmt.Fprintf(w, `<a href="%s">%s</a>`+"\n", name, name)
				}

				w.Write(utils.StringAsBytes(`</pre></body></html>`))
				return
			}
			//send content of the file

			err := serveFileNativeRequest(ctx, w, r, core.PathFrom(filesystemPath))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
		}),
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
	return &HttpsServer{
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
	return serveFileNativeRequest(ctx, rw.rw, r.Request(), pth)
}

func serveFileNativeRequest(ctx *core.Context, rw http.ResponseWriter, r *http.Request, pth core.Path) error {
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

	modTime := stat.ModTime()

	//TODO: pass a custom response writer that logs error and return a 404 status code.
	http.ServeContent(rw, r, string(pth), modTime, f)
	return nil
}
