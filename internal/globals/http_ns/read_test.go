package http_ns

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/stretchr/testify/assert"
)

func TestHttpGet(t *testing.T) {
	testconfig.AllowParallelization(t)

	makeServer := func(checkReq func(*http.Request, http.ResponseWriter)) (*http.Server, core.URL) {
		var ADDR = "localhost:" + nextPort()
		var URL = core.URL("http://" + ADDR + "/")

		server := &http.Server{
			Addr: ADDR,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//always ok

				if checkReq != nil {
					checkReq(r, w)
				}
			}),
		}

		go server.ListenAndServe()
		time.Sleep(time.Millisecond)
		return server, URL
	}

	t.Run("missing permission", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(nil)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Delete, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpGet(ctx, URL)
		assert.Error(t, err)
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.HttpPermission{Kind_: permbase.Read, Entity: URL}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, resp)
	})

	t.Run("the request rate limit should be met", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(nil)
		defer server.Close()

		//create a context that allows up to one request per second
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
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

	t.Run("the response's body should be closed when the context is done", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		var sentByteCount atomic.Int32

		server, URL := makeServer(func(r *http.Request, w http.ResponseWriter) {
			byteSlice := bytes.Repeat([]byte{'a'}, 100)

			for {
				select {
				case <-r.Context().Done():
					return
				default:
					w.Write(byteSlice)
					sentByteCount.Add(int32(len(byteSlice)))
					time.Sleep(25 * time.Millisecond)
				}
			}
		})
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
			},
			Limits: []core.Limit{
				{Name: HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 1 * core.FREQ_LIMIT_SCALE},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpGet(ctx, URL)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NoError(t, err) {
			return
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			ctx.CancelGracefully()
		}()

		bytes, err := io.ReadAll(resp.Body(ctx))

		if !assert.ErrorIs(t, err, context.Canceled) {
			return
		}

		maxReceivedCount := int(sentByteCount.Load())
		assert.InDelta(t, maxReceivedCount, len(bytes), float64(maxReceivedCount/8))
	})
}

func TestHttpRead(t *testing.T) {
	testconfig.AllowParallelization(t)

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
		testconfig.AllowParallelization(t)

		server, URL := makeServer(json)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Delete, Entity: URL},
			},
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		resp, err := HttpRead(ctx, URL)

		notAllowedError := new(core.NotAllowedError)
		if !assert.ErrorAs(t, err, &notAllowedError) {
			return
		}

		assert.Equal(t, core.HttpPermission{Kind_: permbase.Read, Entity: URL}, notAllowedError.Permission)
		assert.Nil(t, resp)
	})

	t.Run("the request rate limit should be met", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		server, URL := makeServer(json)
		defer server.Close()

		//create a context that allows up to one request per second
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
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
		testconfig.AllowParallelization(t)

		server, URL := makeServer(json)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
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
		testconfig.AllowParallelization(t)

		server, URL := makeServer(jsonUtf8)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
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
		testconfig.AllowParallelization(t)

		server, URL := makeServer(json)
		defer server.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permbase.Read, Entity: URL},
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
