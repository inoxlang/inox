package http_ns

import (
	"net/http"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"github.com/stretchr/testify/assert"
)

func TestHttpRequestPattern(t *testing.T) {
	testconfig.AllowParallelization(t)

	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("creation", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("no argument", func(t *testing.T) {
			pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.Call(ctx, []core.Serializable{})
			if !assert.Error(t, err) {
				return
			}
			assert.Nil(t, pattern)
		})

		t.Run("argument of invalid type", func(t *testing.T) {
			pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.Call(ctx, []core.Serializable{core.Int(1)})
			if !assert.Error(t, err) {
				return
			}
			assert.Nil(t, pattern)
		})

		t.Run("empty description", func(t *testing.T) {
			pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.Call(ctx, []core.Serializable{core.NewInexactObjectPattern(nil)})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &RequestPattern{
				methods: nil,
				headers: core.NewInexactRecordPattern(nil),
			}, pattern)
		})

		t.Run("description with valid method", func(t *testing.T) {
			description := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{
					Name:    "method",
					Pattern: core.NewExactValuePattern(core.Identifier("GET")),
				},
			})

			pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.Call(ctx, []core.Serializable{description})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &RequestPattern{
				methods: []string{"GET"},
				headers: core.NewInexactRecordPattern(nil),
			}, pattern)
		})

		t.Run("description with invalid type for .method", func(t *testing.T) {
			description := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{
					Name:    "method",
					Pattern: core.NewExactValuePattern(core.String("GET_")),
				},
			})

			assert.Panics(t, func() {
				CALLABLE_HTTP_REQUEST_PATTERN.Call(ctx, []core.Serializable{description})
			})
		})
	})

	t.Run("Test", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		description := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
			{
				Name:    "method",
				Pattern: core.NewExactValuePattern(core.Identifier("GET")),
			},
		})

		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.Call(ctx, []core.Serializable{description})
		if !assert.NoError(t, err) {
			return
		}

		stdReq := utils.Must(http.NewRequest("GET", "https://example.com/", nil))
		stdReq.Header.Add("Content-Type", "application/json")
		req := utils.Must(NewServerSideRequest(stdReq, zerolog.Logger{}, nil))
		assert.True(t, pattern.Test(ctx, req))

		stdReq = utils.Must(http.NewRequest("POST", "https://example.com/", nil))
		stdReq.Header.Add("Content-Type", "application/json")
		req = utils.Must(NewServerSideRequest(stdReq, zerolog.Logger{}, nil))
		assert.False(t, pattern.Test(ctx, req))
	})
}

func TestCreationOfSymbolicHttpRequestPattern(t *testing.T) {
	testconfig.AllowParallelization(t)
	ctx := &symbolic.Context{}

	t.Run("no argument", func(t *testing.T) {
		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{}, nil)
		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, pattern)
	})

	t.Run("argument of invalid type", func(t *testing.T) {
		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{symbolic.NewInt(1)}, nil)
		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, pattern)
	})

	t.Run("empty description", func(t *testing.T) {
		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{symbolic.NewInexactObjectPattern(nil, nil)}, nil)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, http_symbolic.ANY_REQUEST_PATTERN, pattern)
	})

	t.Run("description with valid method", func(t *testing.T) {
		description := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{
			"method": utils.Must(symbolic.NewExactValuePattern(symbolic.NewIdentifier("GET"))),
		}, nil)

		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{description}, nil)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, http_symbolic.ANY_REQUEST_PATTERN, pattern)
	})

	t.Run("description with invalid type for .method", func(t *testing.T) {
		description := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{
			"method": utils.Must(symbolic.NewExactValuePattern(symbolic.NewString("GET_"))),
		}, nil)

		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{description}, nil)
		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, pattern)
	})

	t.Run("description with unknown-value idenfier for .method", func(t *testing.T) {
		description := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{
			"method": utils.Must(symbolic.NewExactValuePattern(symbolic.ANY_IDENTIFIER)),
		}, nil)

		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{description}, nil)
		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, pattern)
	})

	t.Run("description with invalid union for .method", func(t *testing.T) {
		description := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{
			"method": utils.Must(symbolic.NewUnionPattern([]symbolic.Pattern{
				utils.Must(symbolic.NewExactValuePattern(symbolic.NewString("GET"))),
				utils.Must(symbolic.NewExactValuePattern(symbolic.NewString("POST_"))),
			}, false)),
		}, nil)

		pattern, err := CALLABLE_HTTP_REQUEST_PATTERN.SymbolicCallImpl(ctx, []symbolic.Value{description}, nil)
		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, pattern)
	})
}
