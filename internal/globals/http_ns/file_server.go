package http_ns

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/inoxlang/inox/internal/compressarch"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/mimeconsts"
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

	fileCompressor := compressarch.NewFileCompressor()
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

			err := serveFile(fileServingParams{
				ctx:            ctx,
				rw:             w,
				r:              r,
				pth:            core.PathFrom(filesystemPath),
				fileCompressor: fileCompressor,
			})
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

// ServeFile is a thin wrapper around serveFileNativeRequest.
func ServeFile(ctx *core.Context, rw *ResponseWriter, r *Request, pth core.Path) error {
	return serveFile(fileServingParams{
		ctx: ctx,
		rw:  rw.DetachRespWriter(),
		r:   r.request,
		pth: pth,
	})
}
