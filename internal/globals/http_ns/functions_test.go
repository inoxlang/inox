package http_ns

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
)

func TestHttpClient(t *testing.T) {

	permissiveHttpReqLimit := core.MustMakeNotDecrementingLimit(HTTP_REQUEST_RATE_LIMIT_NAME, 10_000)

	const ADDR = "localhost:8080"
	const URL = core.URL("http://" + ADDR + "/")
	url_, _ := url.Parse(string(URL))

	makeServer := func() *http.Server {
		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cookie := http.Cookie{Name: "k", Value: "1"}
				http.SetCookie(w, &cookie)
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server
	}

	t.Run("if cookies are disabled the cookie jar should be empty", func(t *testing.T) {

		server := makeServer()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{permissiveHttpReqLimit},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()
		defer server.Close()

		client, err := NewClient(ctx, core.NewObjectFromMap(core.ValMap{"save-cookies": core.False}, ctx))
		assert.NoError(t, err)

		_, err = HttpGet(ctx, URL, core.Option{Name: "client", Value: client})

		if !assert.NoError(t, err) {
			t.FailNow()
		}

		assert.Nil(t, client.options.Jar)
	})

	t.Run("if cookies are enabled the cookie jar should not be empty", func(t *testing.T) {

		server := makeServer()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{permissiveHttpReqLimit},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()
		defer server.Close()

		client, err := NewClient(ctx, core.NewObjectFromMap(core.ValMap{"save-cookies": core.True}, ctx))
		assert.NoError(t, err)

		_, err = HttpGet(ctx, URL, core.Option{Name: "client", Value: client})
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		cookies := client.options.Jar.Cookies(url_)
		assert.NotEmpty(t, cookies)
	})

	t.Run("set cookies should be sent", func(t *testing.T) {

		server := makeServer()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{permissiveHttpReqLimit},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()
		defer server.Close()

		client, err := NewClient(ctx, core.NewObjectFromMap(core.ValMap{"save-cookies": core.True}, ctx))
		assert.NoError(t, err)

		resp, err := HttpGet(ctx, URL, core.Option{Name: "client", Value: client})

		if !assert.NoError(t, err) {
			t.FailNow()
		}

		assert.NotEmpty(t, client.options.Jar.Cookies(url_))
		assert.NotEmpty(t, resp.wrapped.Cookies())
	})
}

func TestHttpGet(t *testing.T) {
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
				core.HttpPermission{Kind_: permkind.Delete, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpGet(ctx, URL)
		assert.Error(t, err)
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.HttpPermission{Kind_: permkind.Read, Entity: URL}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, resp)
	})

	t.Run("the request rate limit should be met", func(t *testing.T) {
		server := makeServer()
		defer server.Close()

		//create a context that allows up to one request per second
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.SimpleRateLimit,
					Value: 1,
				},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		_, err := HttpGet(ctx, URL)
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()

		_, err = HttpGet(ctx, URL)
		if !assert.NoError(t, err) {
			return
		}

		assert.WithinDuration(t, start.Add(time.Second), time.Now(), 100*time.Millisecond)
	})
}

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
		assert.Equal(t, core.HttpPermission{Kind_: permkind.Create, Entity: URL}, err.(*core.NotAllowedError).Permission)
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
