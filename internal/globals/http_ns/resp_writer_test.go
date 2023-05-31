package http_ns

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aohorodnyk/mimeheader"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestHttpResponseWriter(t *testing.T) {

	list := core.NewWrappedValueList
	obj := core.NewObjectFromMap

	t.Run("WriteJSON()", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		testCases := []struct {
			value      core.Value
			outputJSON string
			ok         bool
		}{
			{
				value:      core.Int(1),
				outputJSON: `"1"`,
				ok:         true,
			},
			{
				value: obj(core.ValMap{
					"name": core.Str("foo"),
				}, ctx),
				outputJSON: `{"name":"foo"}`,
				ok:         true,
			},
			{
				value: list(obj(core.ValMap{
					"name": core.Str("foo"),
				}, ctx)),
				outputJSON: `[{"name":"foo"}]`,
				ok:         true,
			},
			{
				value:      core.Str("{}"),
				outputJSON: `{}`,
				ok:         true,
			},
			{
				value: core.Str("{"),
				ok:    false,
			},
			//TODO: add tests for Go values
		}

		for _, testCase := range testCases {
			t.Run(fmt.Sprint(testCase.value), func(t *testing.T) {
				ctx := core.NewContext(core.ContextConfig{})
				core.NewGlobalState(ctx)

				recorder := httptest.NewRecorder()
				resp := HttpResponseWriter{rw: recorder, acceptHeader: mimeheader.ParseAcceptHeader(core.JSON_CTYPE)}

				_, err := resp.WriteJSON(ctx, testCase.value)
				result := recorder.Result()

				if testCase.ok {
					assert.NoError(t, err)
					assert.Equal(t, testCase.outputJSON, recorder.Body.String())
					assert.Equal(t, core.JSON_CTYPE, result.Header.Get("Content-Type"))
				} else {
					assert.Error(t, err)
					assert.Empty(t, recorder.Body.Bytes())
					assert.Equal(t, "", result.Header.Get("Content-Type"))
				}
			})
		}

	})

	t.Run("SetCookie()", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		testCases := []struct {
			obj    *core.Object
			cookie *http.Cookie
			ok     bool
		}{
			{
				obj: core.NewObject(),
				ok:  false,
			},
			{
				obj: core.NewObjectFromMap(core.ValMap{"name": core.Str("mycookie")}, ctx),
				ok:  false,
			},
			{
				obj: core.NewObjectFromMap(core.ValMap{"value": core.Str("0")}, ctx),
				ok:  false,
			},
			{
				obj:    core.NewObjectFromMap(core.ValMap{"name": core.Str("mycookie"), "value": core.Str("0")}, ctx),
				ok:     true,
				cookie: &http.Cookie{Name: "mycookie", Value: "0"},
			},
			{
				obj: core.NewObjectFromMap(core.ValMap{"name": core.Str("mycookie"), "value": core.Str("0"), "domain": core.Str("localhost")}, ctx),
				ok:  false,
			},
			{
				obj:    core.NewObjectFromMap(core.ValMap{"name": core.Str("mycookie"), "value": core.Str("0"), "domain": core.Host("://localhost")}, ctx),
				ok:     true,
				cookie: &http.Cookie{Name: "mycookie", Value: "0", Domain: "localhost"},
			},
		}

		for _, testCase := range testCases {
			t.Run(fmt.Sprint(testCase.obj), func(t *testing.T) {
				ctx := core.NewContext(core.ContextConfig{})
				core.NewGlobalState(ctx)

				recorder := httptest.NewRecorder()
				resp := HttpResponseWriter{rw: recorder}

				err := resp.SetCookie(ctx, testCase.obj)
				cookies := recorder.Result().Cookies()

				if testCase.ok {
					assert.NoError(t, err)
					assert.Len(t, cookies, 1)
					cookie := cookies[0]
					cookie.Raw = "" //we do not compare some fields
					cookie.Secure = false
					cookie.HttpOnly = false
					cookie.SameSite = 0
					assert.Equal(t, testCase.cookie, cookie)
				} else {
					assert.Empty(t, cookies)
					assert.Error(t, err)
				}
			})
		}
	})
}
