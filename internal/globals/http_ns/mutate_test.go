package http_ns

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/testconfig"
)

func TestHttpPost(t *testing.T) {
	testconfig.AllowParallelization(t)

	makeServer := func(checkReq func(*http.Request)) (*http.Server, core.URL) {
		var ADDR = "localhost:" + nextPort()
		var URL = core.URL("http://" + ADDR + "/")

		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//always ok

				if checkReq != nil {
					checkReq(r)
				}
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server, URL
	}

	t.Run("missing URL", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(nil)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, core.NewObject())
		if resp != nil {
			resp.wrapped.Body.Close()
		}

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("string provided instead of URL", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(nil)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, core.String(URL), core.NewObject())
		if resp != nil {
			resp.wrapped.Body.Close()
		}

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing body", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(nil)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, URL)
		if resp != nil {
			resp.wrapped.Body.Close()
		}

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing permission", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(nil)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
			},
			Limits: []core.Limit{},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, URL)
		if resp != nil {
			resp.wrapped.Body.Close()
		}

		assert.Error(t, err)
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.HttpPermission{Kind_: permbase.Write, Entity: URL}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, resp)
	})

	t.Run("if the body value is an object the Content-Type should be JSON", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		var cType atomic.Value
		var body atomic.Value

		server, URL := makeServer(func(r *http.Request) {
			cType.Store(r.Header.Get("Content-Type"))

			bytes, err := io.ReadAll(r.Body)
			if err == nil {
				body.Store(string(bytes))
			}
		})
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Write, Entity: URL},
			},
			Limits: []core.Limit{
				{Name: HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 1 * core.FREQ_LIMIT_SCALE},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, URL, core.NewObject())
		if resp != nil {
			resp.wrapped.Body.Close()
		}

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, mimeconsts.JSON_CTYPE, cType.Load())
		assert.Equal(t, `{}`, body.Load())
	})

	t.Run("if the body value is a list the Content-Type should be JSON", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		var cType atomic.Value
		var body atomic.Value

		server, URL := makeServer(func(r *http.Request) {
			cType.Store(r.Header.Get("Content-Type"))

			bytes, err := io.ReadAll(r.Body)
			if err == nil {
				body.Store(string(bytes))
			}
		})
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Write, Entity: URL},
			},
			Limits: []core.Limit{
				{Name: HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 1 * core.FREQ_LIMIT_SCALE},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpPost(ctx, URL, core.NewWrappedValueList())
		if resp != nil {
			resp.wrapped.Body.Close()
		}

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, mimeconsts.JSON_CTYPE, cType.Load())
		assert.Equal(t, `[]`, body.Load())
	})
}

func TestHttpDelete(t *testing.T) {
	testconfig.AllowParallelization(t)

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
		testconfig.AllowParallelization(t)

		server, URL := makeServer()
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
			},
			Limits: []core.Limit{},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpDelete(ctx, URL)
		assert.Error(t, err)
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.HttpPermission{Kind_: permbase.Delete, Entity: URL}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, resp)
	})
}

func TestServeFile(t *testing.T) {
	t.Run("missing read permission", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		recorder := httptest.NewRecorder()
		resp := &ResponseWriter{rw: recorder}
		req := &Request{}

		err := ServeFile(ctx, resp, req, core.Path("/x"))
		assert.IsType(t, &core.NotAllowedError{}, err)
	})
}
