package http_ns

import (
	"crypto/tls"
	"net/http"
	_cookiejar "net/http/cookiejar"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/publicsuffix"
)

func TestHttpServerHandlingDescription(t *testing.T) {

	t.Run("handling description", func(t *testing.T) {

		t.Run("routing only", func(t *testing.T) {
			runHandlingDescTestCase(t, serverTestCase{
				input: `return {
					routing: Mapping {
						%/... => "hello"
					}
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
				},
			}, createClient)
		})

		t.Run("empty middleware list", func(t *testing.T) {
			runHandlingDescTestCase(t, serverTestCase{
				input: `return {
					middlewares: []
					routing: Mapping {
						%/... => "hello"
					}
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
				},
			}, createClient)
		})

		t.Run("a middleware filtering based on path", func(t *testing.T) {
			runHandlingDescTestCase(t, serverTestCase{
				input: ` return {
					middlewares: [
						Mapping {
							/a => #notfound
							/b => #continue
						}
					]
					routing: Mapping {
						/a => "a"
						/b => "b"
					}
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, path: "/a", status: 404},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, path: "/b", result: `b`, status: 200},
				},
			}, createClient)
		})

		t.Run("filesystem routing", func(t *testing.T) {
			runHandlingDescTestCase(t, serverTestCase{
				input: `return {
					routing: /routes/
				}`,
				makeFilesystem: func() afs.Filesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
						manifest {}

						return "hello"
					`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
				},
			}, createClient)
		})
	})

	t.Run("certificate & key", func(t *testing.T) {

		testCase := serverTestCase{
			input: `return {
				routing: Mapping {
					%/... => "hello"
				}
			}`,
			requests: []requestTestInfo{
				{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
			},
		}

		//create state & context

		state, ctx, chunk, host, err := setupAdvancedTestCase(t, testCase)
		if !assert.NoError(t, err) {
			return
		}

		// make handling description
		treeWalkState := core.NewTreeWalkStateWithGlobal(state)
		desc, err := core.TreeWalkEval(chunk, treeWalkState)
		if !assert.NoError(t, err) {
			return
		}

		cert, key, err := generateSelfSignedCertAndKeyValues(ctx)
		if !assert.NoError(t, err) {
			return
		}

		obj := desc.(*core.Object)
		obj.SetProp(ctx, HANDLING_DESC_CERTIFICATE_PROPNAME, cert)
		obj.SetProp(ctx, HANDLING_DESC_KEY_PROPNAME, key)

		//run the test

		runAdvancedServerTestCase(t, testCase, createClient, func() (*HttpServer, *core.Context, core.Host, error) {
			server, err := NewHttpServer(ctx, host, desc)

			return server, ctx, host, err
		})
	})

	t.Run("rate limiting", func(t *testing.T) {
		const HELLO = `
			return {
				routing: Mapping {
					%/... => "hello"
				}
			}`

		const MINI_PAUSE = 10 * time.Millisecond

		//improve + add new tests

		t.Run("server should block burst: same client (HTTP2)", func(t *testing.T) {

			runHandlingDescTestCase(t, serverTestCase{
				input: HELLO,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true},
				},
				createClientFn: func() func() *http.Client {
					client := &http.Client{
						Transport: &http.Transport{
							TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
							ForceAttemptHTTP2: true,
						},
						Timeout: REQ_TIMEOUT,
						Jar:     utils.Must(_cookiejar.New(&_cookiejar.Options{PublicSuffixList: publicsuffix.List})),
					}

					return utils.Ret(client)
				},
			}, createClient)
		})

		t.Run("server should block burst : same client (HTTP2) + small pause beween requests", func(t *testing.T) {

			runHandlingDescTestCase(t, serverTestCase{
				input: HELLO,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true, pause: MINI_PAUSE},
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`, okayIf429: true, pause: MINI_PAUSE},
				},
				createClientFn: func() func() *http.Client {
					client := &http.Client{
						Transport: &http.Transport{
							TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
							ForceAttemptHTTP2: true,
						},
						Timeout: REQ_TIMEOUT,
						Jar:     utils.Must(_cookiejar.New(&_cookiejar.Options{PublicSuffixList: publicsuffix.List})),
					}

					return utils.Ret(client)
				},
			}, createClient)
		})

	})
}
