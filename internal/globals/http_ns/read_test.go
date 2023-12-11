package http_ns

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestHttpGet(t *testing.T) {
	t.Parallel()

	makeServer := func() (*http.Server, core.URL) {
		var ADDR = "localhost:" + strconv.Itoa(int(port.Add(1)))
		var URL = core.URL("http://" + ADDR + "/")

		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//always ok
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server, URL
	}

	t.Run("missing permission", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer()
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
		t.Parallel()

		server, URL := makeServer()
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

func TestHttpRead(t *testing.T) {
	t.Parallel()

	makeServer := func() (*http.Server, core.URL) {
		var ADDR = "localhost:" + strconv.Itoa(int(port.Add(1)))
		var URL = core.URL("http://" + ADDR + "/")

		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Content-Type", mimeconsts.JSON_CTYPE)

				//note that there are 2 spaces after the colon.
				w.Write([]byte(`{"a":  1}`))
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server, URL
	}

	t.Run("missing permission", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer()
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Delete, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpRead(ctx, URL)

		notAllowedError := new(core.NotAllowedError)
		if !assert.ErrorAs(t, err, &notAllowedError) {
			return
		}

		assert.Equal(t, core.HttpPermission{Kind_: permkind.Read, Entity: URL}, notAllowedError.Permission)
		assert.Nil(t, resp)
	})

	t.Run("the request rate limit should be met", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer()
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

		_, err := HttpRead(ctx, URL)
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()

		_, err = HttpRead(ctx, URL)
		if !assert.NoError(t, err) {
			return
		}

		assert.WithinDuration(t, start.Add(time.Second), time.Now(), 100*time.Millisecond)
	})

	t.Run("by default the content should be parsed based on the Content-type header", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer()
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.SimpleRateLimit,
					Value: 100,
				},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		val, err := HttpRead(ctx, URL)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.IsType(t, (*core.Object)(nil), val) {
			return
		}

		obj := val.(*core.Object)
		assert.Equal(t, core.Float(1), obj.Prop(ctx, "a"))
	})

	t.Run("if a mimetype argument is passed the parsing should be based it", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer()
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.SimpleRateLimit,
					Value: 100,
				},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		val, err := HttpRead(ctx, URL, core.Mimetype(mimeconsts.PLAIN_TEXT_CTYPE))
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.Str(`{"a":  1}`), val)
	})
}
