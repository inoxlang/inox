package http_ns

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	_cookiejar "net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"slices"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	http_ns "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
	"github.com/inoxlang/inox/internal/mimeconsts"
	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/rs/zerolog"
	"golang.org/x/net/publicsuffix"

	parse "github.com/inoxlang/inox/internal/parse"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

const (
	IDENTIDAL_SECONDARY_REQ_COUNT = 4
	REQ_TIMEOUT                   = 2 * time.Second
)

var (
	errAny = errors.New("any")
	port   = atomic.Int32{}
)

func nextPort() string {
	return strconv.Itoa(int(port.Add(1)))
}

func init() {
	port.Store(10_000)
	if !core.AreDefaultScriptLimitsSet() {
		core.SetDefaultScriptLimits([]core.Limit{})
	}

	if core.NewDefaultContext == nil {
		core.SetNewDefaultContext(func(config core.DefaultContextConfig) (*core.Context, error) {

			if len(config.OwnedDatabases) != 0 {
				panic(errors.New("not supported"))
			}

			permissions := []core.Permission{
				core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				core.GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				core.LThreadPermission{Kind_: permkind.Create},
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			}

			permissions = append(permissions, config.Permissions...)

			ctx := core.NewContext(core.ContextConfig{
				Permissions:          permissions,
				ForbiddenPermissions: config.ForbiddenPermissions,
				HostResolutions:      config.HostResolutions,
				ParentContext:        config.ParentContext,
			})

			for k, v := range core.DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(k, v)
			}

			for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
				ctx.AddPatternNamespace(k, v)
			}

			return ctx, nil
		})

		core.SetNewDefaultGlobalStateFn(func(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {
			state := core.NewGlobalState(ctx, map[string]core.Value{
				"html":              core.ValOf(html_ns.NewHTMLNamespace()),
				"sleep":             core.WrapGoFunction(core.Sleep),
				"torstream":         core.WrapGoFunction(toRstream),
				"mkbytes":           core.WrapGoFunction(mkBytes),
				"tostr":             core.WrapGoFunction(toStr),
				"cancel_exec":       core.WrapGoFunction(cancelExec),
				"do_cpu_bound_work": core.WrapGoFunction(doCpuBoundWork),
				"add_effect":        core.WrapGoFunction(addEffect),
				"EmailAddress":      core.WrapGoFunction(makeEmailAddress),
				"statuses":          STATUS_NAMESPACE,
				"Status":            core.WrapGoFunction(makeStatus),
				"Result":            core.WrapGoFunction(NewResult),
				"ctx_data":          core.WrapGoFunction(_ctx_data),
			})

			return state, nil
		})
	}

	core.RegisterSymbolicGoFunction(toStr, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.StringLike {
		return symbolic.ANY_STR_LIKE
	})

	core.RegisterSymbolicGoFunction(cancelExec, func(ctx *symbolic.Context) {})
	core.RegisterSymbolicGoFunction(doCpuBoundWork, func(ctx *symbolic.Context, _ *symbolic.Duration) {})
	core.RegisterSymbolicGoFunction(addEffect, func(ctx *symbolic.Context) *symbolic.Error { return nil })

	core.RegisterSymbolicGoFunction(mkBytes, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.ByteSlice {
		return symbolic.ANY_BYTE_SLICE
	})
	core.RegisterSymbolicGoFunction(makeEmailAddress, func(ctx *symbolic.Context, s symbolic.StringLike) *symbolic.EmailAddress {
		return symbolic.ANY_EMAIL_ADDR
	})
	core.RegisterSymbolicGoFunction(makeStatus, func(ctx *symbolic.Context, s *http_ns.StatusCode) *http_ns.Status {
		return http_ns.ANY_STATUS
	})

	core.RegisterSymbolicGoFunction(toRstream, func(ctx *symbolic.Context, v symbolic.Value) *symbolic.ReadableStream {
		return symbolic.NewReadableStream(symbolic.ANY)
	})

	core.RegisterSymbolicGoFunction(_ctx_data, func(ctx *symbolic.Context, path *symbolic.Path) symbolic.Value {
		return symbolic.ANY
	})

	if !core.IsSymbolicEquivalentOfGoFunctionRegistered(core.Sleep) {
		core.RegisterSymbolicGoFunction(core.Sleep, func(ctx *symbolic.Context, _ *symbolic.Duration) {

		})
	}
}

func TestHttpServerMissingProvidePermission(t *testing.T) {
	testconfig.AllowParallelization(t)

	if !core.AreDefaultRequestHandlingLimitsSet() {
		core.SetDefaultRequestHandlingLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
		})
	}

	if !core.AreDefaultMaxRequestHandlerLimitsSet() {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		})
	}

	host := core.Host("https://localhost:" + nextPort())
	ctx := core.NewContext(core.ContextConfig{
		Filesystem: fs_ns.GetOsFilesystem(),
	})
	core.NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	server, err := NewHttpsServer(ctx, host)

	assert.IsType(t, &core.NotAllowedError{}, err)
	assert.Equal(t, core.HttpPermission{Kind_: permkind.Provide, Entity: host}, err.(*core.NotAllowedError).Permission)
	assert.Nil(t, server)
}

func TestHttpServerWithoutHandler(t *testing.T) {

	if !core.AreDefaultRequestHandlingLimitsSet() {
		core.SetDefaultRequestHandlingLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
		})
	}

	if !core.AreDefaultMaxRequestHandlerLimitsSet() {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		})
	}

	t.Run("provided host is https://localhost:port", func(t *testing.T) {
		host := core.Host("https://localhost:" + nextPort())

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Provide, Entity: host},
			},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		state := core.NewGlobalState(ctx)
		state.Logger = zerolog.New(io.Discard)
		state.OutputFieldsInitialized.Store(true)

		server, err := NewHttpsServer(ctx, host)
		if server != nil {
			defer server.Close(ctx)
		}

		assert.NotNil(t, server)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, host, server.ListeningAddr())

		//we send a request to the server
		req, _ := http.NewRequest("GET", string(host)+"/x", nil)
		req.Header.Add("Accept", mimeconsts.JSON_CTYPE)

		client := createClient()
		resp, err := client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		assert.NoError(t, err)
	})

	t.Run("if the provided host is https://0.0.0.0:port and we are not in project mode the effective address should be localhost:port", func(t *testing.T) {
		port := nextPort()
		host := core.Host("https://0.0.0.0:" + port)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Provide, Entity: host},
			},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		state := core.NewGlobalState(ctx)
		state.Logger = zerolog.New(io.Discard)
		state.OutputFieldsInitialized.Store(true)

		server, err := NewHttpsServer(ctx, host)
		if server != nil {
			defer server.Close(ctx)
		}

		assert.NotNil(t, server)
		if !assert.NoError(t, err) {
			return
		}
		assert.EqualValues(t, "https://localhost:"+port, server.ListeningAddr())

		globalUnicastIps, err := netaddr.GetGlobalUnicastIPs()
		if err != nil {
			server.serverLogger.Err(err).Send()
			return
		}

		client := createClient()

		//check that the server is not listening on any global unicast  interfaces

		for _, ip := range globalUnicastIps {
			req, _ := http.NewRequest("GET", "https://"+ip.String()+":"+port+"/x", nil)
			resp, err := client.Do(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			assert.Error(t, err)
		}

		//check that the server is listening on localhost
		req, _ := http.NewRequest("GET", "https://localhost:"+port+"/x", nil)
		resp, err := client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		assert.NoError(t, err)
	})

	t.Run("if the provided host is https://0.0.0.0:port and the project does not allow exposing web servers the effective address should be localhost:port", func(t *testing.T) {
		port := nextPort()
		host := core.Host("https://0.0.0.0:" + port)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Provide, Entity: host},
			},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		state := core.NewGlobalState(ctx)
		state.Logger = zerolog.New(io.Discard)
		state.OutputFieldsInitialized.Store(true)
		state.Project = project.NewDummyProjectWithConfig("proj", fs_ns.NewMemFilesystem(10_000), project.ProjectConfiguration{
			ExposeWebServers: false,
		})

		server, err := NewHttpsServer(ctx, host)
		if server != nil {
			defer server.Close(ctx)
		}

		assert.NotNil(t, server)
		if !assert.NoError(t, err) {
			return
		}
		assert.EqualValues(t, "https://localhost:"+port, server.ListeningAddr())

		globalUnicastIps, err := netaddr.GetGlobalUnicastIPs()
		if err != nil {
			server.serverLogger.Err(err).Send()
			return
		}

		client := createClient()

		//check that the server is not listening on any global unicast  interfaces

		for _, ip := range globalUnicastIps {
			req, _ := http.NewRequest("GET", "https://"+ip.String()+":"+port+"/x", nil)
			resp, err := client.Do(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			assert.Error(t, err)
		}

		//check that the server is listening on localhost
		req, _ := http.NewRequest("GET", "https://localhost:"+port+"/x", nil)
		resp, err := client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		assert.NoError(t, err)
	})

	t.Run("if the provided host is https://0.0.0.0:port and the project allow exposing web servers the server should listen on all interfaces", func(t *testing.T) {
		port := nextPort()
		host := core.Host("https://0.0.0.0:" + port)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Provide, Entity: host},
			},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		state := core.NewGlobalState(ctx)
		state.Logger = zerolog.New(io.Discard)
		state.OutputFieldsInitialized.Store(true)
		state.Project = project.NewDummyProjectWithConfig("proj", fs_ns.NewMemFilesystem(10_000), project.ProjectConfiguration{
			ExposeWebServers: true,
		})

		server, err := NewHttpsServer(ctx, host)
		if server != nil {
			defer server.Close(ctx)
		}

		assert.NotNil(t, server)
		if !assert.NoError(t, err) {
			return
		}
		assert.EqualValues(t, "https://0.0.0.0:"+port, server.ListeningAddr())

		globalUnicastIps, err := netaddr.GetGlobalUnicastIPs()
		if err != nil {
			server.serverLogger.Err(err).Send()
			return
		}

		client := createClient()

		//check that the server is listening on all global unicast interfaces.

		for _, ip := range globalUnicastIps {
			req, _ := http.NewRequest("GET", "https://"+ip.String()+":"+port+"/x", nil)
			resp, err := client.Do(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			assert.NoError(t, err)
		}

		//check that the server is listening on localhost.
		req, _ := http.NewRequest("GET", "https://localhost:"+port+"/x", nil)
		resp, err := client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		assert.NoError(t, err)
	})
}

func TestHttpServerUserHandler(t *testing.T) {

	if !core.AreDefaultRequestHandlingLimitsSet() {
		core.SetDefaultRequestHandlingLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
		})
	}

	if !core.AreDefaultMaxRequestHandlerLimitsSet() {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		})
	}

	//TODO: rework test & add case where handler access a global

	code := "fn(rw, r){ rw.write_json(1) }"
	nodeFn, compiledFn, module := createHandlers(t, code)

	for i, handler := range []*core.InoxFunction{nodeFn, compiledFn} {
		name := "node handler"
		if i == 1 {
			name = "compiled function handler"
			t.Skip()
		}

		t.Run(name, func(t *testing.T) {
			host := core.Host("https://localhost:" + nextPort())

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.HttpPermission{Kind_: permkind.Provide, Entity: host},
				},
				Filesystem: fs_ns.GetOsFilesystem(),
			})
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx)
			state.Module = module
			state.Logger = zerolog.New(io.Discard)

			server, err := NewHttpsServer(ctx, host, handler)
			if server != nil {
				defer server.Close(ctx)
				time.Sleep(time.Millisecond)
			}

			assert.NoError(t, err)
			assert.NotNil(t, server)

			//we send a request to the server
			req, _ := http.NewRequest("GET", string(host)+"/x", nil)
			req.Header.Add("Accept", mimeconsts.JSON_CTYPE)

			client := createClient()
			resp, err := client.Do(req)
			assert.NoError(t, err)

			body := string(utils.Must(io.ReadAll(resp.Body)))
			//we check the response
			assert.Equal(t, `{"int__value":1}`, body)
			assert.Equal(t, 200, resp.StatusCode)
		})
	}
	//TODO: add VM evaluation versions
	//TODO: test that CSP is s
}

func TestHttpServerMapping(t *testing.T) {

	if !core.AreDefaultRequestHandlingLimitsSet() {
		core.SetDefaultRequestHandlingLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
		})
	}

	if !core.AreDefaultMaxRequestHandlerLimitsSet() {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{})
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		})
	}

	t.Run("CUSTOMMETHOD /x", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
							%/... => "hello"
						}
						`,
				requests: []requestTestInfo{
					{
						method:              "CUSTOMMETHOD",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusBadRequest,
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x: string result", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
							%/... => "hello"
						}
						`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE, result: `hello`},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x */* is accepted: string result", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
					%/... => "hello"
				}
				`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.ANY_CTYPE, result: `hello`},
				},
			},
			createClient,
		)
	})

	t.Run("POST /x: string result", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
							%/... => "hello"
						}
						`,
				requests: []requestTestInfo{
					{
						method:              "POST",
						header:              http.Header{"Content-Type": []string{mimeconsts.PLAIN_TEXT_CTYPE}},
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x: bytes result", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
					%/... => 0d[65] # 'A'
				}
				`,
				requests: []requestTestInfo{{acceptedContentType: mimeconsts.APP_OCTET_STREAM_CTYPE, result: `A`}},
			},
			createClient,
		)
	})

	t.Run("GET /x: html node", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
					%/... => html<div></div>
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.HTML_CTYPE, result: `<div></div>`},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x */* is accepted: html node", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
					%/... => html<div></div>
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.ANY_CTYPE, result: `<div></div>`},
				},
			},
			createClient,
		)
	})

	t.Run("POST /x: html node", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
					%/... => html<div></div>
				}`,
				requests: []requestTestInfo{
					{
						method:              "POST",
						header:              http.Header{"Content-Type": []string{mimeconsts.PLAIN_TEXT_CTYPE}},
						acceptedContentType: mimeconsts.HTML_CTYPE,
						result:              `<div></div>`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x: handler", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `
					fn handle(rw %http.resp-writer, r %http.req){
						rw.write_json({ a: 1 })
					}
					return Mapping {
						%/... => handle
					}
				`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.JSON_CTYPE, result: `{"object__value":{"a":{"int__value":1}}}`},
				},
			},
			createClient)
	})

	t.Run("GET /x JSON is accepted: nil", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
				%/... => nil
			}
			`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.JSON_CTYPE, result: "null"},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x */* is accepted: nil", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
				%/... => nil
			}
			`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.ANY_CTYPE, status: 404},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x JSON is accepted: notfound identifier", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
			%/... => #notfound
		}
		`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.JSON_CTYPE, status: http.StatusNotFound},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x */* is accepted: notfound identifier", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return Mapping {
			%/... => #notfound
		}
		`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.ANY_CTYPE, status: http.StatusNotFound},
				},
			},
			createClient,
		)
	})

	t.Run("handler accessing a global function", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `
					fn helper(rw %http.resp-writer, r %http.req){
						rw.write_json({ a: 1 })
					}
					fn handle(rw %http.resp-writer, r %http.req){
						helper(rw, r)
					}
					return Mapping {
						%/... => handle
					}
				`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.JSON_CTYPE, result: `{"object__value":{"a":{"int__value":1}}}`},
				},
			},
			createClient,
		)
	})

	t.Run("JSON of model", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `$$model = {a: 1}

				return Mapping {
					%/... => model
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.JSON_CTYPE, result: `{"object__value":{"a":{"int__value":1}}}`},
				},
			},
			createClient,
		)
	})

	t.Run("JSON of model with sensitive data, no defined visibility", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `
				$$model = {
					a: 1
					password: "mypassword"
					e: EmailAddress("foo@mail.com")
				}

				return Mapping {
					%/... => model
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.JSON_CTYPE, result: `{"object__value":{"a":{"int__value":1}}}`},
				},
			},
			createClient,
		)
	})

	t.Run("JSON of model with all fields set as public", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `$$model = {
					a: 1
					password: "mypassword"
					e: EmailAddress("a@mail.com")

					_visibility_ {
						{
							public: .{a, password, e}
						}
					}
				}

				return Mapping {
					%/... => model
				}`,
				requests: []requestTestInfo{
					{
						acceptedContentType: mimeconsts.JSON_CTYPE,
						result:              `{"object__value":{"a":{"int__value":1},"e":{"emailaddr__value":"a@mail.com"},"password":"mypassword"}}`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("large binary stream: event stream request", func(t *testing.T) {

		runServerTest(t,

			serverTestCase{
				input: strings.Replace(`
					return Mapping {
						%/... => torstream(mkbytes(<size>))
					}`, "<size>", strconv.Itoa(int(10*DEFAULT_PUSHED_BYTESTREAM_CHUNK_SIZE_RANGE.InclusiveEnd())), 1),
				requests: []requestTestInfo{
					{
						acceptedContentType: mimeconsts.EVENT_STREAM_CTYPE,
						events: func() []*core.Event {
							chunkMaxSize := DEFAULT_PUSHED_BYTESTREAM_CHUNK_SIZE_RANGE.InclusiveEnd()
							size := int(10 * chunkMaxSize)

							b := bytes.Repeat([]byte{0}, size)
							encoded := []byte(hex.EncodeToString(b))
							encodedSize := 2 * size
							// 10 chunks of equal size
							encodedDataChunkSize := encodedSize / 10

							var events []*core.Event

							for i := 0; i < 10; i++ {
								events = append(events, (&ServerSentEvent{
									Data: []byte(encoded[i*encodedDataChunkSize : (i+1)*encodedDataChunkSize]),
								}).ToEvent())
							}

							return events
						}(),
					},
				},
			},
			createClient,
		)
	})

}

func setupTestCase(t *testing.T, testCase serverTestCase) (*core.GlobalState, *core.Context, *parse.Chunk, core.Host, error) {
	host := core.Host("https://localhost:" + strconv.Itoa(int(port.Add(1))))

	var fls core.SnapshotableFilesystem = fs_ns.NewMemFilesystem(1_000_000)

	if testCase.makeFilesystem != nil {
		fls = testCase.makeFilesystem()
	}

	perms := []core.Permission{
		core.HttpPermission{Kind_: permkind.Provide, Entity: host},
		core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
		core.GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
		core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
		core.LThreadPermission{Kind_: permkind.Create},
		core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
	}

	utils.PanicIfErr(util.WriteFile(fls, "/main.ix", []byte(testCase.input), 0700))

	// create module
	chunk := parse.MustParseChunk(testCase.input)
	module := &core.Module{
		MainChunk: parse.NewParsedChunkSource(chunk, parse.SourceFile{
			NameString:  "/main.ix",
			Resource:    "/main.ix",
			ResourceDir: "/",
			CodeString:  testCase.input,
		}),
		ManifestTemplate: chunk.Manifest,
	}

	manifest, _, _, err := module.PreInit(core.PreinitArgs{
		AddDefaultPermissions: true,
	})

	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("preinit error: %w", err)
	}

	// create state & context
	ctx := core.NewContext(core.ContextConfig{
		Permissions:     append(perms, manifest.RequiredPermissions...),
		HostResolutions: manifest.HostResolutions,
		Filesystem:      fls,
	})

	for k, v := range core.DEFAULT_NAMED_PATTERNS {
		ctx.AddNamedPattern(k, v)
	}

	for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
		ctx.AddPatternNamespace(k, v)
	}

	state := core.NewGlobalState(ctx, map[string]core.Value{
		"html":         core.ValOf(html_ns.NewHTMLNamespace()),
		"sleep":        core.WrapGoFunction(core.Sleep),
		"torstream":    core.WrapGoFunction(toRstream),
		"mkbytes":      core.WrapGoFunction(mkBytes),
		"tostr":        core.WrapGoFunction(toStr),
		"cancel_exec":  core.WrapGoFunction(cancelExec),
		"EmailAddress": core.WrapGoFunction(makeEmailAddress),
		"Status":       core.WrapGoFunction(makeStatus),
		"ctx_data":     core.WrapGoFunction(_ctx_data),
	})

	state.Module = module
	state.Project = project.NewDummyProject("proj", fls)

	// create logger
	out := testCase.outWriter
	if out == nil {
		out = io.Discard
	}
	state.Logger = zerolog.New(out)
	state.Out = out

	if testCase.outWriter != nil {
		state.LogLevels = core.NewLogLevels(core.LogLevelsInitialization{
			DefaultLevel:            zerolog.DebugLevel,
			EnableInternalDebugLogs: true,
		})
	}

	staticData, err := core.StaticCheck(core.StaticCheckInput{
		State:                  state,
		Node:                   state.Module.MainChunk.Node,
		Module:                 state.Module,
		Chunk:                  state.Module.MainChunk,
		Globals:                state.Globals,
		Patterns:               state.Ctx.GetNamedPatterns(),
		PatternNamespaces:      state.Ctx.GetPatternNamespaces(),
		AdditionalGlobalConsts: testCase.additionalGlobalConstsForStaticChecks,
	})
	if !assert.NoError(t, err) {
		return nil, nil, nil, "", err
	}
	state.StaticCheckData = staticData

	if testCase.finalizeState != nil {
		err := testCase.finalizeState(state)
		if err != nil {
			ctx.CancelGracefully()
			return nil, nil, nil, "", err
		}
	}

	return state, ctx, chunk, host, nil
}

func runServerTest(t *testing.T, testCase serverTestCase, defaultCreateClient func() *http.Client) {

	state, ctx, chunk, host, err := setupTestCase(t, testCase)
	if !assert.NoError(t, err) {
		return
	}
	defer ctx.CancelGracefully()

	treeWalkState := core.NewTreeWalkStateWithGlobal(state)
	handler, err := core.TreeWalkEval(chunk, treeWalkState)
	if !assert.NoError(t, err) {
		return
	}

	runAdvancedServerTest(t, testCase, defaultCreateClient, func() (*HttpsServer, *core.Context, core.Host, error) {
		server, err := NewHttpsServer(ctx, host, handler)

		return server, ctx, host, err
	})
}

// important note: runAdvancedServerTest calls .Parallel() if testCase.avoidTestParallelization is false.
func runAdvancedServerTest(
	t *testing.T, testCase serverTestCase,
	defaultCreateClient func() *http.Client, setup func() (*HttpsServer, *core.Context, core.Host, error),
) {

	if !testCase.avoidTestParallelization {
		testconfig.AllowParallelization(t)
	}
	server, ctx, host, err := setup()
	if !assert.NoError(t, err) {
		return
	}

	defer server.Close(ctx)
	time.Sleep(time.Millisecond)

	//send requests
	createClient := defaultCreateClient
	if testCase.createClientFn != nil {
		createClient = testCase.createClientFn()
	}
	client := createClient()

	ctx.SetProtocolClientForHost(host, NewHttpClientFromPreExistingClient(client, true))

	responseLock := sync.Mutex{}
	responses := make([]*http.Response, len(testCase.requests))
	responseErrors := make([]error, len(testCase.requests))

	secondaryRequestResponses := make([][]*http.Response, len(testCase.requests))
	secondaryRequestResponseErrors := make([][]error, len(testCase.requests))

	receivedEvents := make([][]*core.Event, len(testCase.requests))

	wg := new(sync.WaitGroup)
	wg.Add(len(testCase.requests))

	sendReq := func(i int, info requestTestInfo, isPrimary bool, secondaryReqIndex int) {
		defer wg.Done()

		if info.preDelay != 0 {
			time.Sleep(info.preDelay)
		}

		url := string(host)
		if info.path == "" {
			url += "/x"
		} else {
			url += info.path
		}

		if info.acceptedContentType != mimeconsts.EVENT_STREAM_CTYPE {
			method := "GET"
			if info.method != "" {
				method = info.method
			}

			var body io.Reader
			if info.requestBody != "" {
				if slices.Contains(spec.METHODS_WITH_NO_BODY, method) {
					assert.Fail(t, fmt.Sprintf("body provided but method is %s", method))
					return
				}
				body = strings.NewReader(info.requestBody)
			}

			// we send a request to the server
			req, _ := http.NewRequest(method, url, body)

			if info.acceptedContentType != "" {
				req.Header.Add("Accept", string(info.acceptedContentType))
			}

			if info.contentType != "" {
				req.Header.Set("Content-Type", string(info.contentType))
			}

			for k, values := range info.header {
				for _, val := range values {
					req.Header.Add(k, val)
				}
			}
			if info.onStartSending != nil {
				info.onStartSending()
			}
			resp, err := client.Do(req)
			if info.onStatusReceived != nil {
				info.onStatusReceived()
			}

			responseLock.Lock()
			if isPrimary {
				responses[i], responseErrors[i] = resp, err
			} else {
				secondaryRequestResponses[i][secondaryReqIndex], secondaryRequestResponseErrors[i][secondaryReqIndex] =
					resp, err
			}
			responseLock.Unlock()
		} else {
			evs, err := NewEventSource(ctx, core.URL(url))
			if err != nil {
				responseErrors[i] = err
				return
			} else {
				evs.OnEvent(func(event *core.Event) {
					responseLock.Lock()
					receivedEvents[i] = append(receivedEvents[i], event)
					responseLock.Unlock()
				})
				<-time.After(time.Duration(len(info.events)) * 300 * time.Millisecond)
				evs.Close()
			}
		}

	}

	//send requests, add specified delays
	for i, req := range testCase.requests {
		if req.pause != 0 {
			time.Sleep(req.pause)
		}
		if req.checkIdenticalParallelRequest {
			wg.Add(IDENTIDAL_SECONDARY_REQ_COUNT)

			secondaryRequestResponses[i] = make([]*http.Response, IDENTIDAL_SECONDARY_REQ_COUNT)
			secondaryRequestResponseErrors[i] = make([]error, IDENTIDAL_SECONDARY_REQ_COUNT)

			go sendReq(i, req, true, -1)
			for j := 0; j < IDENTIDAL_SECONDARY_REQ_COUNT; j++ {
				go sendReq(i, req, false, j)
			}
		} else {
			go sendReq(i, req, true, -1)
		}
	}

	wg.Wait()
	responseLock.Lock() //prevent ininteresting race conditions

	server.Close(ctx)

	//check responses
	for i, info := range testCase.requests {
		resp := responses[i]
		err := responseErrors[i]

		if info.err == nil {
			if !assert.NoError(t, err) {
				return
			}

		} else if info.err != errAny {
			assert.ErrorIs(t, err, info.err)
		} else if !assert.Error(t, err) {
			return
		}

		if info.acceptedContentType != mimeconsts.EVENT_STREAM_CTYPE { //normal request
			if info.err != nil {
				if info.checkIdenticalParallelRequest {
					for _, secondaryErr := range secondaryRequestResponseErrors[i] {
						assert.ErrorIs(t, secondaryErr, info.err, "(secondary request)")
					}
				}
				continue
			}

			//check status

			reqInfo := "request" + strconv.Itoa(i) + " " + info.method + " " + info.path

			if info.status == 0 {
				if info.okayIf429 && resp.StatusCode == 429 {
					goto check_body
				}
				if !assert.Equal(t, 200, resp.StatusCode, reqInfo) {
					return
				}
			} else {
				if !assert.Equal(t, info.status, resp.StatusCode, reqInfo) {
					body, err := io.ReadAll(resp.Body)
					if err == nil {
						var logArg any = body
						if utf8.Valid(body) {
							logArg = string(body)
						}
						t.Log("body content is", logArg)
					}
					return
				}
			}

			//check headers

			for headerName, expectedValues := range info.expectedHeaderSubset {
				assert.Equal(t, expectedValues, resp.Header.Values(headerName))
			}

			//check cookies
			{
				cookies := resp.Cookies()
			check_cookies:
				for cookieName, expectedValue := range info.expectedCookieValues {
					for _, cookie := range cookies {
						if cookie.Name == cookieName {
							if !assert.Equal(t, expectedValue, cookie.Value, "cookie "+cookieName) {
								return
							}
							continue check_cookies
						}
					}
					assert.Fail(t, "failed to find cookie "+cookieName)
					return
				}
			}

		check_body:

			body := string(utils.Must(io.ReadAll(resp.Body)))

			switch {
			case info.result != "":
				if !assert.Equal(t, info.result, body, reqInfo) {
					return
				}
			case info.resultRegex != "":
				if !assert.Regexp(t, info.resultRegex, body, reqInfo) {
					return
				}
			case info.checkResponse != nil:
				if !info.checkResponse(t, resp, body) {
					return
				}
			default:
				continue
			}

			if info.checkIdenticalParallelRequest {
				for index, secondaryResp := range secondaryRequestResponses[i] {
					secondaryBody := string(utils.Must(io.ReadAll(secondaryResp.Body)))
					if !assert.Equal(t, body, secondaryBody, "secondary body should be equal to primary body, secondary request "+strconv.Itoa(index)) {
						return
					}
				}
			}

		} else { //check events
			assert.Len(t, receivedEvents[i], len(info.events))
		}
	}
}

type requestTestInfo struct {
	pause               time.Duration //like predelay but does not send current & next requests
	preDelay            time.Duration
	contentType         core.Mimetype
	acceptedContentType core.Mimetype
	path                string
	method              string
	header              http.Header
	requestBody         string

	//expected
	result                        string                                                           // ignored if .resultRegex or .checkResponse is set or of content type is event stream
	resultRegex                   string                                                           // ignored if .result or .checkResponse is set or if content type is event stream
	checkResponse                 func(t *testing.T, resp *http.Response, body string) (cont bool) //ignored if .result or .resultRegex is set
	checkIdenticalParallelRequest bool
	events                        []*core.Event
	err                           error
	status                        int //defaults to 200
	expectedHeaderSubset          http.Header
	expectedCookieValues          map[string]string
	okayIf429                     bool

	onStartSending   func()
	onStatusReceived func()
}

type serverTestCase struct {
	input                                 string
	requests                              []requestTestInfo
	outWriter                             io.Writer
	makeFilesystem                        func() core.SnapshotableFilesystem
	additionalGlobalConstsForStaticChecks []string
	finalizeState                         func(*core.GlobalState) error
	createClientFn                        func() func() *http.Client
	avoidTestParallelization              bool
}

func createHandlers(t *testing.T, code string) (*core.InoxFunction, *core.InoxFunction, *core.Module) {
	chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
		NameString: "server-test",
		CodeString: code,
	}))
	module := &core.Module{
		MainChunk: chunk,
	}

	staticCheckData, err := core.StaticCheck(core.StaticCheckInput{
		State:  core.NewGlobalState(core.NewContext(core.ContextConfig{})),
		Node:   chunk.Node,
		Module: module,
		Chunk:  module.MainChunk,
	})

	if !assert.NoError(t, err) {
		panic(err)
	}

	nodeFunction := &core.InoxFunction{Node: parse.FindNode(chunk.Node, (*parse.FunctionExpression)(nil), nil)}

	bytecode, _ := core.Compile(core.CompilationInput{
		Mod: module,
		Context: core.NewContext(core.ContextConfig{
			Filesystem: fs_ns.GetOsFilesystem(),
		}),
		StaticCheckData: staticCheckData,
	})
	consts := bytecode.Constants()
	compiledFunction := consts[len(consts)-1].(*core.InoxFunction)
	return nodeFunction, compiledFunction, module
}

func createClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: REQ_TIMEOUT,
		Jar:     utils.Must(_cookiejar.New(&_cookiejar.Options{PublicSuffixList: publicsuffix.List})),
	}
}

func cancelExec(ctx *core.Context) {
	ctx.CancelGracefully()
}

func doCpuBoundWork(ctx *core.Context, duration core.Duration) {
	deadline := time.Now().Add(time.Duration(duration))

	for {
		if time.Since(deadline) >= time.Microsecond {
			break
		}
	}
}

func addEffect(ctx *core.Context) error {
	tx := ctx.GetTx()
	if tx == nil {
		return errors.New("a transaction was expected")
	}
	return tx.AddEffect(ctx, &dummyEffect{})
}

func mkBytes(ctx *core.Context, size core.Int) *core.ByteSlice {
	return core.NewMutableByteSlice(make([]byte, size), "")
}

func makeEmailAddress(ctx *core.Context, s core.StringLike) core.EmailAddress {
	return utils.Must(core.NormalizeEmailAddress(s.GetOrBuildString()))
}

func makeStatus(ctx *core.Context, code StatusCode) Status {
	return utils.Must(MakeStatus(code))
}

func toRstream(ctx *core.Context, v core.Value) core.ReadableStream {
	return core.ToReadableStream(ctx, v, core.ANYVAL_PATTERN)
}

func toStr(ctx *core.Context, arg core.Value) core.StringLike {
	switch a := arg.(type) {
	case core.Bool:
		if a {
			return core.String("true")
		}
		return core.String("false")
	case core.Integral:
		return core.String(core.Stringify(a, ctx))
	case core.StringLike:
		return a
	case *core.ByteSlice:
		return core.String(a.UnderlyingBytes()) //TODO: panic if invalid characters ?
	case *core.RuneSlice:
		return core.String(a.ElementsDoNotModify())
	case core.ResourceName:
		return core.String(a.ResourceName())
	default:
		panic(fmt.Errorf("cannot convert value of type %T to string", a))
	}
}

var _ = core.Effect((*dummyEffect)(nil))

type dummyEffect struct {
}

func (*dummyEffect) Apply(*core.Context) error {
	panic("unimplemented")
}

func (*dummyEffect) IsApplied() bool {
	panic("unimplemented")
}

func (*dummyEffect) IsApplying() bool {
	panic("unimplemented")
}

func (*dummyEffect) PermissionKind() permkind.PermissionKind {
	panic("unimplemented")
}

func (*dummyEffect) Resources() []core.ResourceName {
	panic("unimplemented")
}

func (*dummyEffect) Reversability(*core.Context) core.Reversability {
	panic("unimplemented")
}

func (*dummyEffect) Reverse(*core.Context) error {
	panic("unimplemented")
}

func _ctx_data(ctx *core.Context, path core.Path) core.Value {
	data := ctx.ResolveUserData(path)
	if data == nil {
		data = core.Nil
	}
	return data
}
