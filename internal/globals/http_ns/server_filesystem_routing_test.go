package http_ns

import (
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/globals/containers/setcoll"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/localdb"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/stretchr/testify/assert"
)

func TestFilesystemRouting(t *testing.T) {

	const cpuTime = 25 * time.Millisecond
	cpuTimeLimit, err := core.GetLimit(nil, core.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(cpuTime))
	if !assert.NoError(t, err) {
		return
	}

	threadCountLimit, err := core.GetLimit(nil, core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, core.Int(20))
	if !assert.NoError(t, err) {
		return
	}

	//we set the default script limits with a single limits: the thread count limit with a high value.
	if core.AreDefaultScriptLimitsSet() {
		save := core.GetDefaultScriptLimits()
		core.UnsetDefaultScriptLimits()
		core.SetDefaultScriptLimits([]core.Limit{threadCountLimit})

		t.Cleanup(func() {
			core.UnsetDefaultScriptLimits()
			core.SetDefaultScriptLimits(save)
		})

	} else {
		core.SetDefaultScriptLimits([]core.Limit{threadCountLimit})
		t.Cleanup(func() {
			core.UnsetDefaultScriptLimits()
		})
	}

	//set default request handling limits: cpuTimeLimit
	if core.AreDefaultRequestHandlingLimitsSet() {
		save := core.GetDefaultRequestHandlingLimits()
		core.UnsetDefaultRequestHandlingLimits()
		core.SetDefaultRequestHandlingLimits([]core.Limit{cpuTimeLimit})
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
			core.SetDefaultRequestHandlingLimits(save)
		})

	} else {
		core.SetDefaultRequestHandlingLimits([]core.Limit{cpuTimeLimit})
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
		})
	}

	//set default max request handler limits
	const maxCpuTime = 100 * time.Millisecond
	maxCpuTimeLimit, err := core.GetLimit(nil, core.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(maxCpuTime))
	if !assert.NoError(t, err) {
		return
	}

	if core.AreDefaultMaxRequestHandlerLimitsSet() {
		save := core.GetDefaultMaxRequestHandlerLimits()
		core.UnsetDefaultMaxRequestHandlerLimits()
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{maxCpuTimeLimit})
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
			core.SetDefaultMaxRequestHandlerLimits(save)
		})

	} else {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{maxCpuTimeLimit})
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		})
	}

	t.Run("GET /x.html should return the content of /static/x.html and the CSP header should be set", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {static: /static/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/static", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/static/x.html", []byte(`x`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x.html",
						acceptedContentType: mimeconsts.HTML_CTYPE,
						result:              `x`,
						header: http.Header{
							CSP_HEADER_NAME: []string{DEFAULT_CSP.HeaderValue(CSPHeaderValueParams{})},
						},
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x should return the result of /routes/x.ix", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x should return the result of /routes/x/index.ix", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes/x", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x/index.ix", []byte(`
							manifest {}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("method-aspecific handler /routes/x.ix with no _method parameter, no _body parameter and no JSON body parameters should only accept GET/HEAD requests", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {
								parameters: {}
							}
	
							return "HELLO"
						`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:              "GET",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              "HELLO",
					},
					{
						method:              "HEAD",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					},
					{
						method:      "POST",
						requestBody: `body1`,
						header:      http.Header{"Content-Type": []string{mimeconsts.PLAIN_TEXT_CTYPE}},

						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusBadRequest,
					},
					{
						method:              "DELETE",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusBadRequest,
					},
				},
			},
			createClient,
		)
	})

	t.Run("method-agnostic handler module with %reader _body parameter should accept all methods", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {
								parameters: {
									_body: %reader
								}
							}
	
							return mod-args._body.read_all!()
						`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:              "GET",
						acceptedContentType: mimeconsts.APP_OCTET_STREAM_CTYPE,
						result:              ``,
					},
					{
						method:      "POST",
						requestBody: `body1`,
						header:      http.Header{"Content-Type": []string{mimeconsts.PLAIN_TEXT_CTYPE}},

						acceptedContentType: mimeconsts.APP_OCTET_STREAM_CTYPE,
						result:              `body1`,
					},
					{
						method:      "PATCH",
						requestBody: `body2`,
						header:      http.Header{"Content-Type": []string{mimeconsts.PLAIN_TEXT_CTYPE}},

						acceptedContentType: mimeconsts.APP_OCTET_STREAM_CTYPE,
						result:              `body2`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("an error should be returned if a method-agnostic handler module has a JSON body parameter", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {
								parameters: {
									name: %str
								}
							}
	
							return concat "name is " mod-args.name
						`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:      "POST",
						requestBody: `{"name": "foo"}`,
						header:      http.Header{"Content-Type": []string{mimeconsts.JSON_CTYPE}},

						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusNotFound,
					},
				},
			},
			createClient,
		)
	})

	t.Run("method-specific handler module with parameters describing the body", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/POST-x.ix", []byte(`
							manifest {
								parameters: {
									name: %str
								}
							}
	
							return concat "name is " mod-args.name
						`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:      "POST",
						requestBody: `{"name": "foo"}`,
						header:      http.Header{"Content-Type": []string{mimeconsts.JSON_CTYPE}},

						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `name is foo`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("method-agnostic handler module with %(#POST) _method parameter should only accept POST requests", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {
								parameters: {
									_method: %(#POST)
								}
							}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:      "POST",
						requestBody: `{"name": "foo"}`,
						header:      http.Header{"Content-Type": []string{mimeconsts.JSON_CTYPE}},

						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
					{
						method:      "PATCH",
						requestBody: `{"name": "foo"}`,
						header:      http.Header{"Content-Type": []string{mimeconsts.JSON_CTYPE}},

						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusBadRequest,
					},
				},
			},
			createClient,
		)
	})

	t.Run("an error should be returned during server creation if there a checking error in the handler module", func(t *testing.T) {
		_, ctx, _, host, err := setupTestCase(t, serverTestCase{
			input: `return {
					routing: {dynamic: /routes/}
				}`,
			makeFilesystem: func() core.SnapshotableFilesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/x.ix", []byte(`
						manifest {}

						call_non_existing()

						return "hello"
					`), fs_ns.DEFAULT_FILE_FMODE)

				return fls
			},
		})
		if ctx != nil {
			defer ctx.CancelGracefully()
		}
		if !assert.NoError(t, err) {
			return
		}

		_, err = NewHttpsServer(ctx, host, core.NewObjectFromMapNoInit(core.ValMap{
			HANDLING_DESC_ROUTING_PROPNAME: core.NewObjectFromMapNoInit(core.ValMap{
				"dynamic": core.Path("/routes/"),
			}),
		}))
		assert.ErrorContains(t, err, "not declared")
	})

	t.Run("a status of 500 (internal error) should be returned if the handler defines a limit greater than the corresponding maximum limit", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {
								limits: {
									"`+core.EXECUTION_CPU_TIME_LIMIT_NAME+`": 1s
								}
							}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusInternalServerError,
					},
				},
			},
			createClient,
		)
	})

	t.Run("a status of 500 should be returned if the handler uses all its CPU time, subsequent requests should be ok", func(t *testing.T) {
		var start time.Time
		var end time.Time

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				avoidTestParallelization: true,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/compute.ix", []byte(`
							manifest {}

							a = 1
							for i in 1..1_000_000_000 {
								a += 1
							}
	
							return "end"
						`), fs_ns.DEFAULT_FILE_FMODE)
					util.WriteFile(fls, "/routes/no-compute.ix", []byte(`
						manifest {}
						return "end"
					`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						onStartSending: func() {
							start = time.Now()
						},
						onStatusReceived: func() {
							end = time.Now()
						},
						path:                "/compute",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusInternalServerError,
					},
					//subsequent requests should be ok
					{
						pause:               cpuTime,
						path:                "/no-compute",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
						result:              "end",
					},
					{
						path:                "/no-compute",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
						result:              "end",
					},
					{
						path:                "/no-compute",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
						result:              "end",
					},
				},
			},
			createClient,
		)

		assert.WithinDuration(t, start.Add(cpuTime), end, 10*time.Millisecond)
	})

	t.Run("a status of 200 should be returned if the handler has sleept for a duration greater than its CPU time", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
							sleep 1s
							return "end"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
				},
			},
			createClient,
		)
	})

	t.Run("a status of 200 should be returned if the handler has worked for a duration slightly shorter than its CPU time", func(t *testing.T) {
		var start time.Time
		var end time.Time

		workDuration := cpuTime - cpuTime/4
		workDurationString := strconv.Itoa(int(workDuration/time.Millisecond)) + "ms"

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				avoidTestParallelization: true,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
							do_cpu_bound_work(`+workDurationString+`)
							return "end"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						onStartSending: func() {
							start = time.Now()
						},
						onStatusReceived: func() {
							end = time.Now()
						},
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
				},
			},
			createClient,
		)

		assert.WithinDuration(t, start.Add(workDuration), end, cpuTime/3)
	})

	t.Run("a status of 200 should be returned if a few parallel handlers have each worked for a duration slightly shorter than their CPU time", func(t *testing.T) {
		var start time.Time
		var end time.Time

		workDuration := cpuTime - cpuTime/4
		workDurationString := strconv.Itoa(int(workDuration/time.Millisecond)) + "ms"

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				avoidTestParallelization: true,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
							do_cpu_bound_work(`+workDurationString+`)
							return "end"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						onStartSending: func() {
							start = time.Now()
						},
						onStatusReceived: func() {
							end = time.Now()
						},
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusOK,
					},
				},
			},
			createClient,
		)

		assert.WithinDuration(t, start.Add(workDuration), end, cpuTime/3)
	})

	t.Run("the handler modules should never be created with any of the default script limits", func(t *testing.T) {
		//In this test we spawn many lthreads to make sure the test has not be created with
		//the default script limits that we configured at the start of the test suite.

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}

							for 1..15 {
								go do {
									sleep 0.5s
								}
							}

							sleep 1s

							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusNotFound,
					},
				},
			},
			createClient,
		)
	})

	t.Run("a handler module should be updated each time its file is changed", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				avoidTestParallelization: true,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							return "hello 1"
						`), fs_ns.DEFAULT_FILE_FMODE)

					go func() {
						time.Sleep(100 * time.Millisecond)
						util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}

							return "hello 2"
						`), fs_ns.DEFAULT_FILE_FMODE)
					}()

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello 1`,
					},
					{
						pause:               200 * time.Millisecond, //wait for the file to be updated.
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello 2`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("an endpoint should be removed after the handler file is removed", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				avoidTestParallelization: true,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					go func() {
						time.Sleep(100 * time.Millisecond)
						fls.Remove("/routes/x.ix")
					}()

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
					{
						pause:  200 * time.Millisecond, //wait for the file to be removed.
						path:   "/x",
						status: http.StatusNotFound,
					},
				},
			},
			createClient,
		)
	})

	t.Run("an endpoint should be created after a handler file is added", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				avoidTestParallelization: true,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							return "hello from x"
						`), fs_ns.DEFAULT_FILE_FMODE)

					go func() {
						time.Sleep(100 * time.Millisecond)
						util.WriteFile(fls, "/routes/y.ix", []byte(`
							manifest {}

							return "hello from y"
						`), fs_ns.DEFAULT_FILE_FMODE)
					}()

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello from x`,
					},
					{
						pause:  200 * time.Millisecond, //wait for the new file to be added.
						path:   "/y",
						result: `hello from y`,
					},
					//the /x endpoint should still be present.
					{
						path:   "/x",
						result: `hello from x`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("a nonce should be added to all <script> elements", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							return html<html>
								<head>
									<script></script>
									<script src="/index.js"></script>
									<script></script>
								</head>
							</html>
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.HTML_CTYPE,
						checkResponse: func(t *testing.T, resp *http.Response, body string) (cont bool) {
							resultRegex := `.*<script nonce=".*?"></script>\s*<script src=.*? nonce=".*?"></script>\s*<script nonce=".*?"></script>.*`
							if !assert.Regexp(t, resultRegex, body) {
								return false
							}

							attrStart := `nonce="`
							nonceAttrIndex := strings.Index(body, attrStart)
							s := body[nonceAttrIndex+len(attrStart):]
							endIndex := strings.Index(s, `"`)
							nonceValue := s[:endIndex]

							cspHeaderValues := resp.Header[CSP_HEADER_NAME]

							if !assert.Len(t, cspHeaderValues, 1) {
								return false
							}

							return assert.Contains(t, cspHeaderValues[0], "script-src-elem 'self' 'nonce-"+nonceValue+"'")
						},
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET requests are not allowed to have effects", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							err = add_effect()

							if err? {
								return err.text
							}
							return "ok"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              core.ErrEffectsNotAllowedInReadonlyTransaction.Error(),
					},
				},
			},
			createClient,
		)
	})

	t.Run("HEAD requests are not allowed to have effects", func(t *testing.T) {
		//TODO: check that the error is ErrEffectsNotAllowedInReadonlyTransaction.
		// A specific status code or text, or a header could be set.

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}
	
							err = add_effect()

							if err? {
								cancel_exec()
								return err.text
							}
							return "ok"
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						method:              "HEAD",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusNotFound,
					},
				},
			},
			createClient,
		)
	})

	t.Run("returned status code should be used as the response's status", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}

							return statuses.UNAUTHORIZED
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						method:              "HEAD",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						status:              http.StatusUnauthorized,
					},
				},
			},
			createClient,
		)
	})

	t.Run("returned result should be used to make the response", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x.ix", []byte(`
							manifest {}

							return Result{
								status: statuses.UNAUTHORIZED
								body: "NOT.AUTHORIZED"
								headers: {
									X-Y: "a"
								}
							}
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                 "/x",
						method:               "GET",
						acceptedContentType:  mimeconsts.PLAIN_TEXT_CTYPE,
						result:               "NOT.AUTHORIZED",
						status:               http.StatusUnauthorized,
						expectedHeaderSubset: http.Header{"X-Y": []string{"a"}},
					},
				},
			},
			createClient,
		)
	})

	t.Run("request transaction should be commited or rollbacked after request", func(t *testing.T) {

		testconfig.AllowParallelization(t)

		baseTest := serverTestCase{
			input: `
				manifest {
					permissions: {
						read: ldb://main
						write: ldb://main
					}
				}
				return {
					routing: {dynamic: /routes/}
				}
			`,
			finalizeState: func(gs *core.GlobalState) error {
				host := core.Host("ldb://main")

				localDb, err := localdb.OpenDatabase(gs.Ctx, host, false)
				if err != nil {
					return err
				}
				db, err := core.WrapDatabase(gs.Ctx, core.DatabaseWrappingArgs{
					Inner:                localDb,
					OwnerState:           gs,
					Name:                 "main",
					ExpectedSchemaUpdate: true,
				})
				if err != nil {
					return err
				}
				gs.Databases = map[string]*core.DatabaseIL{
					"main": db,
				}

				setPattern, err := setcoll.SET_PATTERN.CallImpl(setcoll.SET_PATTERN, []core.Serializable{
					core.SERIALIZABLE_PATTERN,
					common.REPR_UNIQUENESS_IDENT,
				})

				if err != nil {
					return err
				}

				schema := core.NewExactObjectPattern([]core.ObjectPatternEntry{
					{Name: "set", Pattern: setPattern},
				})

				db.UpdateSchema(gs.Ctx, schema, core.NewObjectFromMapNoInit(core.ValMap{
					"inclusions": core.NewDictionary(core.ValMap{
						core.GetJSONRepresentation(core.PathPattern("/set"), gs.Ctx, nil): core.NewWrappedValueList(),
					}),
				}))

				gs.Manifest = &core.Manifest{
					Databases: core.DatabaseConfigs{
						{
							Name:                 "main",
							Resource:             host,
							ResolutionData:       core.Nil,
							ExpectedSchemaUpdate: false,
							Owned:                true,
							Provided:             db,
						},
					},
				}

				return nil
			},
		}

		if reflect.TypeOf(baseTest).Kind() != reflect.Struct {
			assert.Fail(t, "")
		}

		t.Run("POST /x text/plain", func(t *testing.T) {
			test := baseTest
			test.makeFilesystem = func() core.SnapshotableFilesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				fls.MkdirAll("/db", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/POST-x.ix", []byte(`
						manifest {
							databases: /main.ix
							permissions: {
								read: ldb://main
								write: ldb://main
							}
						}

						if dbs.main.set.has(2) {
							return "persisted"
						}
						dbs.main.set.add(2)

						return "added"
					`), fs_ns.DEFAULT_FILE_FMODE)

				return fls
			}
			test.requests = []requestTestInfo{
				{
					method:              "POST",
					path:                "/x",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					result:              `added`,
				},
				{
					pause:               10 * time.Millisecond,
					method:              "POST",
					path:                "/x",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					result:              `persisted`,
				},
			}

			runServerTest(t, test, createClient)
		})

		t.Run("POST /x */*", func(t *testing.T) {
			test := baseTest
			test.makeFilesystem = func() core.SnapshotableFilesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				fls.MkdirAll("/db", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/POST-x.ix", []byte(`
						manifest {
							databases: /main.ix
							permissions: {
								read: ldb://main
								write: ldb://main
							}
						}

						if dbs.main.set.has(2) {
							return "persisted"
						}
						dbs.main.set.add(2)

						return "added"
					`), fs_ns.DEFAULT_FILE_FMODE)

				return fls
			}
			test.requests = []requestTestInfo{
				{
					method:              "POST",
					path:                "/x",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					result:              `added`,
				},
				{
					pause:               10 * time.Millisecond,
					method:              "POST",
					path:                "/x",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					result:              `persisted`,
				},
			}

			runServerTest(t, test, createClient)
		})

		t.Run("manually cancelled transaction: should not be commited", func(t *testing.T) {
			//TODO: make the test work with shorter pauses between requests

			test := baseTest
			test.makeFilesystem = func() core.SnapshotableFilesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				fls.MkdirAll("/db", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/POST-set.ix", []byte(`
						manifest {
							databases: /main.ix
							permissions: {
								read: ldb://main
								write: ldb://main
							}
						}

						if dbs.main.set.has(2) {
							return "persisted"
						}

						dbs.main.set.add(2)
						sleep 0.1s

						if dbs.main.set.has("do not cancel") {
							return "tx not cancelled"
						}

						cancel_exec()
					`), fs_ns.DEFAULT_FILE_FMODE)

				util.WriteFile(fls, "/routes/POST-read.ix", []byte(`
						manifest {
							databases: /main.ix
							permissions: {
								read: ldb://main
								write: ldb://main
							}
						}

						if dbs.main.set.has(2) {
							return "persisted"
						}
						return "not persisted"
					`), fs_ns.DEFAULT_FILE_FMODE)
				util.WriteFile(fls, "/routes/POST-add-do-not-cancel.ix", []byte(`
					manifest {
						databases: /main.ix
						permissions: {
							read: ldb://main
							write: ldb://main
						}
					}

					dbs.main.set.add("do not cancel")
					return ""
				`), fs_ns.DEFAULT_FILE_FMODE)
				return fls
			}
			test.requests = []requestTestInfo{
				{
					method:              "POST",
					path:                "/set",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					status:              404,
				},
				{
					pause:               150 * time.Millisecond,
					method:              "POST",
					path:                "/read",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					status:              200,
					result:              "not persisted",
				},
				{
					pause:               150 * time.Millisecond,
					method:              "POST",
					path:                "/add-do-not-cancel",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					status:              200,
				},
				{
					pause:               150 * time.Millisecond,
					method:              "POST",
					path:                "/read",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					status:              200,
					result:              "not persisted",
				},
				{
					pause:               150 * time.Millisecond,
					method:              "POST",
					path:                "/set",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					status:              200,
					result:              "tx not cancelled",
				},
				{
					pause:               150 * time.Millisecond,
					method:              "POST",
					path:                "/read",
					contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
					acceptedContentType: mimeconsts.ANY_CTYPE,
					status:              200,
					result:              "persisted",
				},
			}
			runServerTest(t, test, createClient)
		})

	})

}
