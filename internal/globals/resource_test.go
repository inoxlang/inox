package internal

import (
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	_http "github.com/inoxlang/inox/internal/globals/http"
	"github.com/stretchr/testify/assert"
)

const RESOURCE_TEST_HOST = core.Host("https://localhost:8080")

func setup(t *testing.T, handler func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest)) *core.Context {
	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.HttpPermission{Kind_: core.ReadPerm, Entity: core.URLPattern("https://localhost:8080/...")},
			core.HttpPermission{Kind_: core.ProvidePerm, Entity: RESOURCE_TEST_HOST},
		},
	})

	state := core.NewGlobalState(ctx)
	state.Out = os.Stdout
	state.Logger = log.New(state.Out, "", 0)

	_, err := _http.NewHttpServer(ctx, RESOURCE_TEST_HOST, core.WrapGoFunction(handler))

	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	return ctx
}

func TestCreateResource(t *testing.T) {

}

func TestReadResource(t *testing.T) {

	insecure := core.Option{Name: "insecure", Value: core.True}
	raw := core.Option{Name: "raw", Value: core.True}
	resource := core.URL(string(RESOURCE_TEST_HOST) + "/resource")

	t.Run("http", func(t *testing.T) {
		t.Run("resource not found", func(t *testing.T) {
			ctx := setup(t, func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
				rw.WriteStatus(ctx, http.StatusNotFound)
			})
			defer ctx.Cancel()

			res, err := _readResource(ctx, resource, core.NewObjectFromMap(core.ValMap{
				"0": insecure,
			}, ctx))

			if assert.Error(t, err) {
				assert.Nil(t, res)
			}
		})

		t.Run("existing resource", func(t *testing.T) {
			ctx := setup(t, func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
				rw.WriteJSON(ctx, core.True)
			})
			defer ctx.Cancel()

			res, err := _readResource(ctx, resource, core.NewObjectFromMap(core.ValMap{
				"0": insecure,
			}, ctx))

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.True, res)
		})

		t.Run("raw", func(t *testing.T) {
			ctx := setup(t, func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
				rw.WriteJSON(ctx, core.True)
			})
			defer ctx.Cancel()

			res, err := _readResource(ctx, resource, raw, core.NewObjectFromMap(core.ValMap{
				"0": insecure,
			}, ctx))

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewByteSlice([]byte("true"), false, core.JSON_CTYPE), res)
		})

		t.Run("an error should be returned if parsing required AND there is no parser for content type", func(t *testing.T) {
			ctx := setup(t, func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
				rw.WriteContentType("custom/type")
				rw.BodyWriter().Write([]byte("X;X"))
			})
			defer ctx.Cancel()

			res, err := _readResource(ctx, resource, core.NewObjectFromMap(core.ValMap{
				"0": insecure,
			}, ctx))

			assert.Nil(t, res)
			assert.ErrorIs(t, err, ErrContentTypeParserNotFound)
		})

		t.Run("an error should bot be returned if raw data is asked AND there is no parser for content type", func(t *testing.T) {
			ctx := setup(t, func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
				rw.WriteContentType("custom/type")
				rw.BodyWriter().Write([]byte("X;X"))
			})
			defer ctx.Cancel()

			res, err := _readResource(ctx, resource, raw, core.NewObjectFromMap(core.ValMap{
				"0": insecure,
			}, ctx))

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewByteSlice([]byte("X;X"), false, "custom/type"), res)
		})
	})

}

func TestGetResource(t *testing.T) {

	options := func(ctx *core.Context) *core.Object {
		return core.NewObjectFromMap(core.ValMap{
			"0": core.Option{Name: "insecure", Value: core.True},
		}, ctx)
	}

	t.Run("read IXON", func(t *testing.T) {
		ctx := setup(t, func(ctx *core.Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
			rw.WriteIXON(ctx, core.NewObjectFromMap(core.ValMap{"a": core.Int(1)}, ctx))
		})
		defer ctx.Cancel()

		resource := core.URL(string(RESOURCE_TEST_HOST) + "/resource")
		res, err := _getResource(ctx, resource, options(ctx))

		if !assert.NoError(t, err) {
			return
		}

		obj := core.NewObjectFromMap(core.ValMap{"a": core.Int(1)}, ctx)
		obj.SetURLOnce(ctx, resource)
		assert.Equal(t, obj, res)
	})
}
