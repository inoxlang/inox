package http_ns

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/mimeconsts"
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
					Kind:  core.FrequencyLimit,
					Value: 1 * core.FREQ_LIMIT_SCALE,
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

		assert.WithinDuration(t, start.Add(time.Second), time.Now(), 150*time.Millisecond)
	})
}

func TestHttpRead(t *testing.T) {
	t.Parallel()

	type contentType int

	const (
		json contentType = iota
		jsonUtf8
	)

	const JSON_UTF8 = mimeconsts.JSON_CTYPE + "; charset=utf-8"

	makeServer := func(contentType contentType) (*http.Server, core.URL) {
		var ADDR = "localhost:" + strconv.Itoa(int(port.Add(1)))
		var URL = core.URL("http://" + ADDR + "/")

		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				switch contentType {
				case json:
					w.Header().Add("Content-Type", mimeconsts.JSON_CTYPE)
				case jsonUtf8:
					w.Header().Add("Content-Type", JSON_UTF8)
				}

				w.Write([]byte(`{"a":  1}`))
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server, URL
	}

	t.Run("missing permission", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer(json)
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

		server, URL := makeServer(json)
		defer server.Close()

		//create a context that allows up to one request per second
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.FrequencyLimit,
					Value: 1 * core.FREQ_LIMIT_SCALE,
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

		assert.WithinDuration(t, start.Add(time.Second), time.Now(), 150*time.Millisecond)
	})

	t.Run("by default the content should be parsed based on the Content-type header", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer(json)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.FrequencyLimit,
					Value: 100 * core.FREQ_LIMIT_SCALE,
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

	t.Run("by default the content should be parsed based on the Content-type header: with params", func(t *testing.T) {
		t.Parallel()

		server, URL := makeServer(jsonUtf8)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.FrequencyLimit,
					Value: 100 * core.FREQ_LIMIT_SCALE,
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

		server, URL := makeServer(json)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{
					Name:  HTTP_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.FrequencyLimit,
					Value: 100 * core.FREQ_LIMIT_SCALE,
				},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		val, err := HttpRead(ctx, URL, core.Mimetype(mimeconsts.PLAIN_TEXT_CTYPE))
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.String(`{"a":  1}`), val)
	})
}
