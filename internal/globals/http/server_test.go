package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	_cookiejar "net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	_dom "github.com/inoxlang/inox/internal/globals/dom"
	_fs "github.com/inoxlang/inox/internal/globals/fs"
	_html "github.com/inoxlang/inox/internal/globals/html"
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
)

func init() {
	port.Store(8080)
}

func TestHttpServerMissingProvidePermission(t *testing.T) {

	host := core.Host("https://localhost:8080")
	ctx := core.NewContext(core.ContextConfig{
		Filesystem: _fs.GetOsFilesystem(),
	})
	core.NewGlobalState(ctx)
	server, err := NewHttpServer(ctx, host)

	assert.IsType(t, core.NotAllowedError{}, err)
	assert.Equal(t, core.HttpPermission{Kind_: permkind.Provide, Entity: host}, err.(core.NotAllowedError).Permission)
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
				Filesystem: _fs.GetOsFilesystem(),
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
			req.Header.Add("Accept", core.JSON_CTYPE)

			client := createClient()
			resp, err := client.Do(req)
			assert.NoError(t, err)

			body := string(utils.Must(io.ReadAll(resp.Body)))
			//we check the response
			assert.Equal(t, `"1"`, body)
			assert.Equal(t, 200, resp.StatusCode)
		})
	}
	//TODO: add VM evaluation versions
	//TODO: test that CSP is s
}

func TestHttpServerMapping(t *testing.T) {

	testCases := map[string]serverTestCase{
		"string": {
			input: `return Mapping {
					%/... => "hello"
				}
				`,
			requests: []requestTestInfo{
				{acceptedContentType: core.PLAIN_TEXT_CTYPE, result: `hello`},
			},
		},
		"string: */* is accepted": {
			input: `return Mapping {
					%/... => "hello"
				}
				`,
			requests: []requestTestInfo{
				{acceptedContentType: core.ANY_CTYPE, result: `hello`},
			},
		},
		"bytes": {
			input: `return Mapping {
					%/... => 0d[65] # 'A'
				}
				`,
			requests: []requestTestInfo{{acceptedContentType: core.APP_OCTET_STREAM_CTYPE, result: `A`}},
		},
		"html node": {
			input: `return Mapping {
					%/... => html.div{}
				}`,
			requests: []requestTestInfo{
				{acceptedContentType: core.HTML_CTYPE, result: `<div></div>`},
			},
		},
		"html node: */* is accepted": {
			input: `return Mapping {
					%/... => html.div{}
				}`,
			requests: []requestTestInfo{
				{acceptedContentType: core.ANY_CTYPE, result: `<div></div>`},
			},
		},
		"handler": {
			input: `
					fn handle(rw %http.resp_writer, r %http.req){
						rw.write_json({ a: 1 })
					}
					return Mapping {
						%/... => handle
					}
				`,
			requests: []requestTestInfo{{acceptedContentType: core.JSON_CTYPE, result: `{"a":"1"}`}},
		},
		"handler accessing a global function": {
			input: `
					fn helper(rw %http.resp_writer, r %http.req){
						rw.write_json({ a: 1 })
					}
					fn handle(rw %http.resp_writer, r %http.req){
						helper(rw, r)
					}
					return Mapping {
						%/... => handle
					}
				`,
			requests: []requestTestInfo{{acceptedContentType: core.JSON_CTYPE, result: `{"a":"1"}`}},
		},
		"JSON for model": {
			input: `$$model = {a: 1}

				return Mapping {
					%/... => model
				}`,
			requests: []requestTestInfo{{acceptedContentType: core.JSON_CTYPE, result: `{"a":"1"}`}},
		},
		"JSON for model with sensitive data, no defined visibility": {
			input: `
				$$model = {
					a: 1
					password: "mypassword"
					e: foo@mail.com
				}

				return Mapping {
					%/... => model
				}`,
			requests: []requestTestInfo{{acceptedContentType: core.JSON_CTYPE, result: `{"a":"1"}`}},
		},
		"JSON for model with all fields set as public": {
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
			requests: []requestTestInfo{{acceptedContentType: core.JSON_CTYPE, result: `{"a":"1","e":"a@mail.com","password":"mypassword"}`}},
		},
		"IXON for model with no defined visibility": {
			input: ` $$model = {
					a: 1
					password: "mypassword"
					e: foo@mail.com
				}

				return Mapping {
					%/... => model
				}`,
			requests: []requestTestInfo{{acceptedContentType: core.IXON_CTYPE, result: `{"a":1}`}},
		},
		"IXON for model with all fields set as public": {
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
			requests: []requestTestInfo{{acceptedContentType: core.IXON_CTYPE, result: `{"a":1,"e":a@mail.com,"password":"mypassword"}`}},
		},

		"large binary stream: event stream request": {
			input: strings.Replace(`
					return Mapping {
						%/... => torstream(mkbytes(<size>))
					}`, "<size>", strconv.Itoa(int(10*DEFAULT_PUSHED_BYTESTREAM_CHUNK_SIZE_RANGE.InclusiveEnd())), 1),
			requests: []requestTestInfo{
				{
					acceptedContentType: core.EVENT_STREAM_CTYPE,
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
	}

	for name, testCase := range testCases {
		runMappingTestCase(t, name, testCase, createClient)
	}

	t.Run("reactive rendering", func(t *testing.T) {

		t.Skip()

		testCases := map[string]serverTestCase{
			"HTML for model": {
				input: `$$model = {
						render: fn() => dom.div{class:"a"}
					}
					return Mapping {
						/ => "hello"
						%/... => model
					}`,
				requests: []requestTestInfo{
					{acceptedContentType: core.PLAIN_TEXT_CTYPE, path: "/"}, // get session
					{
						pause:                         10 * time.Millisecond,
						acceptedContentType:           core.HTML_CTYPE,
						resultRegex:                   `<div class="a".*?></div>`,
						checkIdenticalParallelRequest: true,
					},
				},
			},
			"HTML for self updating model: 2 requests": {
				input: `
					$$model = {
						count: 1
						sleep: sleep
						render: fn() => dom.div{class:"a", self.<count}

						lifetimejob #increment {
							self.sleep(100ms)
							self.count = 2
						}
					}

					return Mapping {
						%/... => model
					}`,
				requests: []requestTestInfo{
					{acceptedContentType: core.HTML_CTYPE, resultRegex: `<div class="a".*?>1</div>`},
					{acceptedContentType: core.HTML_CTYPE, resultRegex: `<div class="a".*?>2</div>`, preDelay: time.Second / 2},
				},
			},
			"event stream request for a model with an invalid view": {
				input: `
						$$model = {
							count: 1
							sleep: sleep
							render: fn() => 1
						}

						return Mapping {
							%/... => model
						}`,
				requests: []requestTestInfo{
					{
						acceptedContentType: core.EVENT_STREAM_CTYPE,
						// no events because fail
					},
				},
			},
			"self updating model: event stream request": {
				input: ` $$model = {
						count: 1
						sleep: sleep
						render: fn() => dom.div{class:"a", self.<count}

						lifetimejob #increment {
							self.sleep(100ms)
							self.count += 1
						}
					}

					return Mapping {
						%/... => model
					}`,
				requests: []requestTestInfo{
					{acceptedContentType: core.HTML_CTYPE, resultRegex: `<div class="a".*?>1</div>`},
					{
						acceptedContentType: core.EVENT_STREAM_CTYPE,
						events: []*core.Event{
							(&ServerSentEvent{
								Data: []byte(`<div class="a".*?>2</div>`),
							}).ToEvent(),
						},
						preDelay: 10 * time.Millisecond,
					},
				},
			},
		}

		for name, testCase := range testCases {
			runMappingTestCase(t, name, testCase, createClient)
		}
	})
}

func setupAdvancedTestCase(t *testing.T, testCase serverTestCase) (*core.GlobalState, *core.Context, *parse.Chunk, core.Host, error) {
	host := core.Host("https://localhost:" + strconv.Itoa(int(port.Add(1))))

	// create state & context
	ctx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.HttpPermission{Kind_: permkind.Provide, Entity: host},
			core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
			core.GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
			core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
			core.RoutinePermission{Kind_: permkind.Create},
		},
		Filesystem: _fs.GetOsFilesystem(),
	})

	for k, v := range core.DEFAULT_NAMED_PATTERNS {
		ctx.AddNamedPattern(k, v)
	}

	for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
		ctx.AddPatternNamespace(k, v)
	}

	state := core.NewGlobalState(ctx, map[string]core.Value{
		"dom":   core.ValOf(_dom.NewDomNamespace()),
		"html":  core.ValOf(_html.NewHTMLNamespace()),
		"sleep": core.WrapGoFunction(core.Sleep),
		"torstream": core.WrapGoFunction(func(ctx *core.Context, v core.Value) core.ReadableStream {
			return core.ToReadableStream(ctx, v, core.ANYVAL_PATTERN)
		}),
		"mkbytes": core.WrapGoFunction(func(ctx *core.Context, size core.Int) *core.ByteSlice {
			return &core.ByteSlice{Bytes: make([]byte, size), IsDataMutable: true}
		}),
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

func runMappingTestCase(t *testing.T, name string, testCase serverTestCase, defaultCreateClient func() *http.Client) {

	state, ctx, chunk, host, err := setupAdvancedTestCase(t, testCase)
	if !assert.NoError(t, err) {
		return
	}

	// get mapping
	treeWalkState := core.NewTreeWalkStateWithGlobal(state)
	mapping, err := core.TreeWalkEval(chunk, treeWalkState)
	if !assert.NoError(t, err) {
		return
	}

	runAdvancedServerTestCase(t, name, testCase, defaultCreateClient, func() (*HttpServer, *core.Context, core.Host, error) {
		server, err := NewHttpServer(ctx, host, mapping)

		return server, ctx, host, err
	})
}

func runHandlingDescTestCase(t *testing.T, name string, testCase serverTestCase, defaultCreateClientFn func() *http.Client) {
	state, ctx, chunk, host, err := setupAdvancedTestCase(t, testCase)
	if !assert.NoError(t, err) {
		return
	}

	// get description
	treeWalkState := core.NewTreeWalkStateWithGlobal(state)
	desc, err := core.TreeWalkEval(chunk, treeWalkState)
	if !assert.NoError(t, err) {
		return
	}

	runAdvancedServerTestCase(t, name, testCase, defaultCreateClientFn, func() (*HttpServer, *core.Context, core.Host, error) {
		server, err := NewHttpServer(ctx, host, desc)

		return server, ctx, host, err
	})
}

func runAdvancedServerTestCase(
	t *testing.T, name string, testCase serverTestCase,
	defaultCreateClient func() *http.Client, setup func() (*HttpServer, *core.Context, core.Host, error),
) {

	t.Run(name, func(t *testing.T) {

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

			if info.acceptedContentType != core.EVENT_STREAM_CTYPE {
				// we send a request to the server
				req, _ := http.NewRequest("GET", url, nil)

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

			if info.acceptedContentType != core.EVENT_STREAM_CTYPE { //normal request
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
	})
}

type requestTestInfo struct {
	pause               time.Duration //like predelay but does not send next requests
	preDelay            time.Duration
	acceptedContentType core.Mimetype
	path                string

	result                        string // ignore if content type is event stream
	resultRegex                   string // ignore if content type is event stream
	checkIdenticalParallelRequest bool
	header                        http.Header

	events    []*core.Event
	err       error
	status    int //defaults to 200
	okayIf429 bool
}

type serverTestCase struct {
	input          string
	requests       []requestTestInfo
	outWriter      io.Writer
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
			Filesystem: _fs.GetOsFilesystem(),
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
