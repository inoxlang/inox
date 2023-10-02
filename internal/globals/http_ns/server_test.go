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

	"slices"

	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/rs/zerolog"
	"golang.org/x/net/publicsuffix"

	parse "github.com/inoxlang/inox/internal/parse"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

const (
	IDENTIDAL_SECONDARY_REQ_COUNT = 4
	REQ_TIMEOUT                   = 1 * time.Second
)

var (
	anyErr = errors.New("any")

	port = atomic.Int32{}

	toStr func(ctx *core.Context, arg core.Value) core.StringLike
)

func init() {
	port.Store(8080)
	if !default_state.IsDefaultScriptLimitsSet() {
		default_state.SetDefaultScriptLimits([]core.Limit{})
	}

	if default_state.NewDefaultContext == nil {
		default_state.SetNewDefaultContext(func(config default_state.DefaultContextConfig) (*core.Context, error) {
			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
					core.GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
					core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
					core.LThreadPermission{Kind_: permkind.Create},
					core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				},
				ParentContext: config.ParentContext,
			})

			for k, v := range core.DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(k, v)
			}

			for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
				ctx.AddPatternNamespace(k, v)
			}

			return ctx, nil
		})

		default_state.SetNewDefaultGlobalStateFn(func(ctx *core.Context, conf default_state.DefaultGlobalStateConfig) (*core.GlobalState, error) {
			return core.NewGlobalState(ctx), nil
		})
	}

	toStr := func(ctx *core.Context, arg core.Value) core.StringLike {
		switch a := arg.(type) {
		case core.Bool:
			if a {
				return core.Str("true")
			}
			return core.Str("false")
		case core.Integral:
			return core.Str(core.Stringify(a, ctx))
		case core.StringLike:
			return a
		case *core.ByteSlice:
			return core.Str(a.Bytes) //TODO: panic if invalid characters ?
		case *core.RuneSlice:
			return core.Str(a.ElementsDoNotModify())
		case core.ResourceName:
			return core.Str(a.ResourceName())
		default:
			panic(fmt.Errorf("cannot convert value of type %T to string", a))
		}
	}

	core.RegisterSymbolicGoFunction(toStr, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) symbolic.StringLike {
		return symbolic.ANY_STR_LIKE
	})

}

func TestHttpServerMissingProvidePermission(t *testing.T) {

	host := core.Host("https://localhost:8080")
	ctx := core.NewContext(core.ContextConfig{
		Filesystem: fs_ns.GetOsFilesystem(),
	})
	core.NewGlobalState(ctx)
	server, err := NewHttpServer(ctx, host)

	assert.IsType(t, &core.NotAllowedError{}, err)
	assert.Equal(t, core.HttpPermission{Kind_: permkind.Provide, Entity: host}, err.(*core.NotAllowedError).Permission)
	assert.Nil(t, server)
}

func TestHttpServerUserHandler(t *testing.T) {

	//TODO: rework test & add case where handler access a global

	host := core.Host("https://localhost:8080")

	code := "fn(rw, r){ rw.write_json(1) }"
	nodeFn, compiledFn, module := createHandlers(t, code)

	for i, handler := range []*core.InoxFunction{nodeFn, compiledFn} {
		name := "node handler"
		if i == 1 {
			name = "compiled function handler"
			t.Skip()
		}

		t.Run(name, func(t *testing.T) {

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.HttpPermission{Kind_: permkind.Provide, Entity: host},
				},
				Filesystem: fs_ns.GetOsFilesystem(),
			})
			state := core.NewGlobalState(ctx)
			state.Module = module
			state.Logger = zerolog.New(io.Discard)

			server, err := NewHttpServer(ctx, host, handler)
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
					%/... => html.div{}
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
					%/... => html.div{}
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
					%/... => html.div{}
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
					e: foo@mail.com
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
					e: a@mail.com

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

	t.Run("IXON of model with no defined visibility", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: ` $$model = {
					a: 1
					password: "mypassword"
					e: foo@mail.com
				}

				return Mapping {
					%/... => model
				}`,
				requests: []requestTestInfo{
					{acceptedContentType: mimeconsts.IXON_CTYPE, result: `{"a":1}`},
				},
			},
			createClient,
		)
	})

	t.Run("IXON of model with all fields set as public", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `$$model = {
					a: 1
					password: "mypassword"
					e: a@mail.com

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
					{acceptedContentType: mimeconsts.IXON_CTYPE, result: `{"a":1,"e":a@mail.com,"password":"mypassword"}`},
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

	var fls afs.Filesystem = fs_ns.GetOsFilesystem()

	if testCase.makeFilesystem != nil {
		fls = testCase.makeFilesystem()
	}

	// create state & context
	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.HttpPermission{Kind_: permkind.Provide, Entity: host},
			core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
			core.GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
			core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
			core.LThreadPermission{Kind_: permkind.Create},
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		},
		Filesystem: fls,
	})

	for k, v := range core.DEFAULT_NAMED_PATTERNS {
		ctx.AddNamedPattern(k, v)
	}

	for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
		ctx.AddPatternNamespace(k, v)
	}

	state := core.NewGlobalState(ctx, map[string]core.Value{
		"html":  core.ValOf(html_ns.NewHTMLNamespace()),
		"sleep": core.WrapGoFunction(core.Sleep),
		"torstream": core.WrapGoFunction(func(ctx *core.Context, v core.Value) core.ReadableStream {
			return core.ToReadableStream(ctx, v, core.ANYVAL_PATTERN)
		}),
		"mkbytes": core.WrapGoFunction(func(ctx *core.Context, size core.Int) *core.ByteSlice {
			return &core.ByteSlice{Bytes: make([]byte, size), IsDataMutable: true}
		}),
		"tostr": core.WrapGoFunction(toStr),
	})

	// create logger
	out := testCase.outWriter
	if out == nil {
		out = io.Discard
	}
	state.Logger = zerolog.New(out)
	state.Out = out

	// create module
	chunk := parse.MustParseChunk(testCase.input)
	state.Module = &core.Module{
		MainChunk: parse.NewParsedChunk(chunk, parse.InMemorySource{
			NameString: "code",
			CodeString: testCase.input,
		}),
	}

	staticData, err := core.StaticCheck(core.StaticCheckInput{
		State:             state,
		Node:              state.Module.MainChunk.Node,
		Module:            state.Module,
		Chunk:             state.Module.MainChunk,
		Globals:           state.Globals,
		Patterns:          state.Ctx.GetNamedPatterns(),
		PatternNamespaces: state.Ctx.GetPatternNamespaces(),
	})
	if !assert.NoError(t, err) {
		return nil, nil, nil, "", err
	}
	state.StaticCheckData = staticData
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

	runAdvancedServerTest(t, testCase, defaultCreateClient, func() (*HttpServer, *core.Context, core.Host, error) {
		server, err := NewHttpServer(ctx, host, handler)

		return server, ctx, host, err
	})
}

func runAdvancedServerTest(
	t *testing.T, testCase serverTestCase,
	defaultCreateClient func() *http.Client, setup func() (*HttpServer, *core.Context, core.Host, error),
) {

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
				if slices.Contains(METHODS_WITH_NO_BODY, method) {
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

			for k, values := range info.header {
				for _, val := range values {
					req.Header.Add(k, val)
				}
			}

			resp, err := client.Do(req)

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

		} else if info.err != anyErr {
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

			//check response

			if info.status == 0 {
				if info.okayIf429 && resp.StatusCode == 429 {
					goto check_body
				}
				if !assert.Equal(t, 200, resp.StatusCode) {
					return
				}
			} else {
				if !assert.Equal(t, info.status, resp.StatusCode, "request"+strconv.Itoa(i)) {
					return
				}
			}
		check_body:

			body := string(utils.Must(io.ReadAll(resp.Body)))

			switch {
			case info.result != "":
				if !assert.Equal(t, info.result, body) {
					return
				}
			case info.resultRegex != "":
				if !assert.Regexp(t, info.resultRegex, body) {
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
	pause               time.Duration //like predelay but does not send next requests
	preDelay            time.Duration
	acceptedContentType core.Mimetype
	path                string
	method              string
	header              http.Header
	requestBody         string

	//expected
	result                        string // ignore if content type is event stream
	resultRegex                   string // ignore if content type is event stream
	checkIdenticalParallelRequest bool
	events                        []*core.Event
	err                           error
	status                        int //defaults to 200
	okayIf429                     bool
}

type serverTestCase struct {
	input          string
	requests       []requestTestInfo
	outWriter      io.Writer
	makeFilesystem func() afs.Filesystem
	createClientFn func() func() *http.Client
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

	core.Compile(core.CompilationInput{
		Mod: module,
		Context: core.NewContext(core.ContextConfig{
			Filesystem: fs_ns.GetOsFilesystem(),
		}),
		StaticCheckData: staticCheckData,
	})
	bytecode := module.Bytecode
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
