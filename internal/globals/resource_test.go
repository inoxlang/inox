package internal

import (
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
)

var port atomic.Int32

func init() {
	port.Store(55_000)
}

func getNextHost() core.Host {
	port := strconv.Itoa(int(port.Add(1)))
	return core.Host("https://localhost:" + port)
}

func TestCreateResource(t *testing.T) {
	//TODO
}

func TestReadResource(t *testing.T) {
	t.Parallel()

	insecure := core.Option{Name: "insecure", Value: core.True}
	raw := core.Option{Name: "raw", Value: core.True}

	t.Run("http", func(t *testing.T) {
		t.Parallel()

		t.Run("resource not found", func(t *testing.T) {
			t.Parallel()

			ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
				rw.WriteStatus(ctx, http.StatusNotFound)
			})
			defer ctx.CancelGracefully()

			res, err := _readResource(ctx, resource, insecure)

			if assert.Error(t, err) {
				assert.Nil(t, res)
			}
		})

		t.Run("existing resource", func(t *testing.T) {
			t.Parallel()

			ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
				rw.WriteJSON(ctx, core.True)
			})
			defer ctx.CancelGracefully()

			res, err := _readResource(ctx, resource, insecure)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.True, res)
		})

		t.Run("raw", func(t *testing.T) {
			t.Parallel()

			ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
				rw.WriteJSON(ctx, core.True)
			})
			defer ctx.CancelGracefully()

			res, err := _readResource(ctx, resource, raw, insecure)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewByteSlice([]byte("true"), false, mimeconsts.JSON_CTYPE), res)
		})

		t.Run("content type with parameters", func(t *testing.T) {
			t.Parallel()

			mimeType := utils.Must(core.MimeTypeFrom("application/json; charset=utf-8"))
			ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
				rw.WriteContentType(string(mimeType))
				rw.BodyWriter().Write([]byte("true"))
			})
			defer ctx.CancelGracefully()

			res, err := _readResource(ctx, resource, raw, insecure)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewByteSlice([]byte("true"), false, mimeconsts.JSON_CTYPE), res)
		})

		t.Run("an error should be returned if parsing required AND there is no parser for content type", func(t *testing.T) {
			t.Parallel()

			ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
				rw.WriteContentType("custom/type")
				rw.BodyWriter().Write([]byte("X;X"))
			})
			defer ctx.CancelGracefully()

			res, err := _readResource(ctx, resource, insecure)

			assert.Nil(t, res)
			assert.ErrorIs(t, err, core.ErrContentTypeParserNotFound)
		})

		t.Run("an error should bot be returned if raw data is asked AND there is no parser for content type", func(t *testing.T) {
			t.Parallel()

			ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
				rw.WriteContentType("custom/type")
				rw.BodyWriter().Write([]byte("X;X"))
			})
			defer ctx.CancelGracefully()

			res, err := _readResource(ctx, resource, raw, insecure)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewByteSlice([]byte("X;X"), false, "custom/type"), res)
		})
	})

}

func TestGetResource(t *testing.T) {
	t.Parallel()

	insecure := core.Option{Name: "insecure", Value: core.True}

	t.Run("read IXON", func(t *testing.T) {
		t.Parallel()

		ctx, resource := setup(t, func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest) {
			rw.WriteIXON(ctx, core.NewObjectFromMap(core.ValMap{"a": core.Int(1)}, ctx))
		})
		defer ctx.CancelGracefully()

		res, err := _getResource(ctx, resource, insecure)

		if !assert.NoError(t, err) {
			return
		}

		obj := core.NewObjectFromMap(core.ValMap{"a": core.Int(1)}, ctx)
		obj.SetURLOnce(ctx, resource)
		assert.Equal(t, obj, res)
	})
}

func setup(t *testing.T, handler func(ctx *core.Context, rw *http_ns.HttpResponseWriter, req *http_ns.HttpRequest)) (*core.Context, core.URL) {
	permissiveHttpReqLimit := core.MustMakeNotDecrementingLimit(http_ns.HTTP_REQUEST_RATE_LIMIT_NAME, 10_000)

	host := getNextHost()

	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.HttpPermission{Kind_: permkind.Read, Entity: core.URLPattern(host + "/...")},
			core.HttpPermission{Kind_: permkind.Provide, Entity: host},
		},
		Filesystem: fs_ns.GetOsFilesystem(),
		Limits:     []core.Limit{permissiveHttpReqLimit},
	})

	state := core.NewGlobalState(ctx)
	state.Out = os.Stdout
	state.Logger = zerolog.New(state.Out)

	_, err := http_ns.NewHttpsServer(ctx, host, core.WrapGoFunction(handler))

	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	return ctx, core.URL(string(host) + "/resource")
}
