package http_ns

import (
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/localdb"
	"github.com/inoxlang/inox/internal/mimeconsts"
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
		defer core.SetDefaultScriptLimits(save)
		defer core.UnsetDefaultScriptLimits()
	} else {
		core.SetDefaultScriptLimits([]core.Limit{threadCountLimit})
		defer core.UnsetDefaultScriptLimits()
	}

	//set default request handling limits: cpuTimeLimit
	if core.AreDefaultRequestHandlingLimitsSet() {
		save := core.GetDefaultRequestHandlingLimits()
		core.UnsetDefaultRequestHandlingLimits()
		core.SetDefaultRequestHandlingLimits([]core.Limit{cpuTimeLimit})
		defer core.SetDefaultRequestHandlingLimits(save)
		defer core.UnsetDefaultRequestHandlingLimits()
	} else {
		core.SetDefaultRequestHandlingLimits([]core.Limit{cpuTimeLimit})
		defer core.UnsetDefaultRequestHandlingLimits()
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
		defer core.SetDefaultMaxRequestHandlerLimits(save)
		defer core.UnsetDefaultMaxRequestHandlerLimits()
	} else {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{maxCpuTimeLimit})
		defer core.UnsetDefaultMaxRequestHandlerLimits()
	}

	t.Run("GET /x.html should return the content of /static/x.html and the CSP header should be set", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {static: /static/}
					}`,
				makeFilesystem: func() afs.Filesystem {
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
							CSP_HEADER_NAME: []string{DEFAULT_CSP.String()},
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
			makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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

		//TODO: improve implementation in order for the assertion to pass with +1ms instead of the +5ms.
		assert.WithinDuration(t, start.Add(workDuration), end, cpuTime/10+5*time.Millisecond)
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
				makeFilesystem: func() afs.Filesystem {
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

		//TODO: improve implementation in order for the assertion to pass with +1ms instead of the +5ms.
		assert.WithinDuration(t, start.Add(workDuration), end, cpuTime/10+5*time.Millisecond)
	})

	t.Run("the handler modules should never be created with any of the default script limits", func(t *testing.T) {
		//In this test we spawn many lthreads to make sure the test has not be created with
		//the default script limits that we configured at the start of the test suite.

		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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
				makeFilesystem: func() afs.Filesystem {
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

	t.Run("request transaction should be commited or rollbacked after request", func(t *testing.T) {

		baseTest := serverTestCase{
			input: `
				manifest {
					permissions: {
						read: ldb://main
						write: ldb://main
					}
					host-resolution: :{
						ldb://main : /db/
					}
				}
				return {
					routing: {dynamic: /routes/}
				}
			`,
			finalizeState: func(gs *core.GlobalState) error {
				host := core.Host("ldb://main")
				dbDir := core.Path("/db/")

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

				setPattern, err := containers.SET_PATTERN.CallImpl(containers.SET_PATTERN, []core.Serializable{
					core.SERIALIZABLE_PATTERN,
					containers_common.REPR_UNIQUENESS_IDENT,
				})

				if err != nil {
					return err
				}
				db.UpdateSchema(gs.Ctx, core.NewExactObjectPattern(map[string]core.Pattern{
					"set": setPattern,
				}), core.NewObjectFromMapNoInit(core.ValMap{
					"inclusions": core.NewDictionary(core.ValMap{
						"%/set": core.NewWrappedValueList(),
					}),
				}))

				gs.Manifest = &core.Manifest{
					Databases: core.DatabaseConfigs{
						{
							Name:                 "main",
							Resource:             host,
							ResolutionData:       dbDir,
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

		t.Run("GET /x text/plain", func(t *testing.T) {
			test := baseTest
			test.makeFilesystem = func() afs.Filesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				fls.MkdirAll("/db", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/x.ix", []byte(`
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
					path:                "/x",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					result:              `added`,
				},
				{
					pause:               10 * time.Millisecond,
					path:                "/x",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					result:              `persisted`,
				},
			}

			runServerTest(t, test, createClient)
		})

		t.Run("GET /x */*", func(t *testing.T) {
			test := baseTest
			test.makeFilesystem = func() afs.Filesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				fls.MkdirAll("/db", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/x.ix", []byte(`
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
					path:                "/x",
					acceptedContentType: mimeconsts.ANY_CTYPE,
					result:              `added`,
				},
				{
					pause:               10 * time.Millisecond,
					path:                "/x",
					acceptedContentType: mimeconsts.ANY_CTYPE,
					result:              `persisted`,
				},
			}

			runServerTest(t, test, createClient)
		})

		t.Run("manually cancelled transaction: should not be commited", func(t *testing.T) {
			//TODO: make the test work with shorter pauses between requests

			test := baseTest
			test.makeFilesystem = func() afs.Filesystem {
				fls := fs_ns.NewMemFilesystem(10_000)
				fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
				fls.MkdirAll("/db", fs_ns.DEFAULT_DIR_FMODE)
				util.WriteFile(fls, "/routes/set.ix", []byte(`
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

				util.WriteFile(fls, "/routes/read.ix", []byte(`
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
				util.WriteFile(fls, "/routes/do-not-cancel.ix", []byte(`
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
					path:                "/set",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					status:              404,
				},
				{
					pause:               100 * time.Millisecond,
					path:                "/read",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					status:              200,
					result:              "not persisted",
				},
				{
					pause:               100 * time.Millisecond,
					path:                "/do-not-cancel",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					status:              200,
				},
				{
					pause:               100 * time.Millisecond,
					path:                "/read",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					status:              200,
					result:              "not persisted",
				},
				{
					pause:               100 * time.Millisecond,
					path:                "/set",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					status:              200,
					result:              "tx not cancelled",
				},
				{
					pause:               200 * time.Millisecond,
					path:                "/read",
					acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
					status:              200,
					result:              "persisted",
				},
			}
			runServerTest(t, test, createClient)
		})

	})

}
