package http_ns

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
)

func TestHttpPost(t *testing.T) {

	const ADDR = "localhost:8080"
	const URL = core.URL("http://" + ADDR + "/")

	makeServer := func() *http.Server {
		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//always ok
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server
	}

	t.Run("missing URL", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, core.NewObject())
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("string provided instead of URL", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, core.Str(URL), core.NewObject())
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing body", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, URL)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing permission", func(t *testing.T) {
		server := makeServer()
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, URL)
		assert.Error(t, err)
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.HttpPermission{Kind_: permkind.Write, Entity: URL}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, resp)
	})
}

func TestHttpDelete(t *testing.T) {

	const ADDR = "localhost:8080"
	const URL = core.URL("http://" + ADDR + "/")

	makeServer := func() *http.Server {
		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//always ok
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server
	}

	t.Run("missing permission", func(t *testing.T) {
		server := makeServer()
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpDelete(ctx, URL)
		assert.Error(t, err)
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.HttpPermission{Kind_: permkind.Delete, Entity: URL}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, resp)
	})
}

func TestServeFile(t *testing.T) {
	t.Run("missing read permission", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		recorder := httptest.NewRecorder()
		resp := &HttpResponseWriter{rw: recorder}
		req := &HttpRequest{}

		err := serveFile(ctx, resp, req, core.Path("/x"))
		assert.IsType(t, &core.NotAllowedError{}, err)
	})
}
