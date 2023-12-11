package http_ns

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestHttpClient(t *testing.T) {
	t.Parallel()

	permissiveHttpReqLimit := core.MustMakeNotDecrementingLimit(HTTP_REQUEST_RATE_LIMIT_NAME, 10_000)

	makeServer := func() (*http.Server, core.URL) {
		var ADDR = "localhost:" + strconv.Itoa(int(port.Add(1)))
		var URL = core.URL("http://" + ADDR + "/")

		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cookie := http.Cookie{Name: "k", Value: "1"}
				http.SetCookie(w, &cookie)
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server, URL
	}

	t.Run("if cookies are disabled the cookie jar should be empty", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer()
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
		t.Parallel()

		server, URL := makeServer()
		url_, _ := url.Parse(string(URL))

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
		t.Parallel()

		server, URL := makeServer()
		url_, _ := url.Parse(string(URL))

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
