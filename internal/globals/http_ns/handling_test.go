package http_ns

import (
	"net/http"
	"os"
	"reflect"
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

func TestHandling(t *testing.T) {
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
	registerDefaultRequestLimits(t, cpuTimeLimit)

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

	t.Run("result with no value and status OK", func(t *testing.T) {
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
								status: statuses.OK
							}
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						method:              "GET",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              "",
						status:              http.StatusOK,
					},
				},
			},
			createClient,
		)
	})

	t.Run("result having headers", func(t *testing.T) {
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
								headers: {
									A: ["1", "2"]
									B: "3"
								}
							}
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/x",
						method:              "GET",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              "",
						header: http.Header{
							"A": []string{"1", "2"},
							"B": []string{"3"},
						},
						status: http.StatusOK,
					},
				},
			},
			createClient,
		)
	})

	t.Run("session in returned result should be stored and a cookie should be sent", func(t *testing.T) {

		runServerTest(t,
			serverTestCase{
				input: `
					manifest {
						permissions: {
							read: ldb://main
							write: ldb://main
						}
					}
					return {
						routing: {dynamic: /routes/}
						sessions: {
							collection: dbs.main.sessions
						}
					}
				`,
				additionalGlobalConstsForStaticChecks: []string{"dbs"},
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
						core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: SESSION_ID_PROPNAME, Pattern: core.STR_PATTERN}}),
						core.PropertyName(SESSION_ID_PROPNAME),
					})

					if err != nil {
						return err
					}

					schema := core.NewExactObjectPattern([]core.ObjectPatternEntry{
						{Name: "sessions", Pattern: setPattern},
					})

					db.UpdateSchema(gs.Ctx, schema, core.NewObjectFromMapNoInit(core.ValMap{
						"inclusions": core.NewDictionary(core.ValMap{
							core.GetJSONRepresentation(core.PathPattern("/sessions"), gs.Ctx, nil): core.NewWrappedValueList(),
						}),
					}))

					gs.Globals.Set("dbs", core.NewMutableEntriesNamespace("dbs", map[string]core.Value{
						"main": db,
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
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/POST-sessions.ix", []byte(`
							manifest {
								databases: /main.ix
								permissions: {
									read: ldb://main
								}
							}

							return Result{
								session: {
									id: "85216e5c138b662924f5831df3a55cc8"
								}
								body: ""
							}
						`), fs_ns.DEFAULT_FILE_FMODE)

					util.WriteFile(fls, "/routes/GET-sessions.ix", []byte(`
						manifest {
							databases: /main.ix
							permissions: {
								read: ldb://main
							}
						}

						session = ctx_data(/session)
						if !(session match {id: string}){
							return "session not in context data"
						}

						assert (session match {id: string})
						session-id = session.id
						
						var ids str = concat session-id ":"
						for stored-session in dbs.main.sessions {
							ids = concat ids stored-session.id ";"
						}

						return ids
					`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/sessions",
						method:              "POST",
						contentType:         mimeconsts.PLAIN_TEXT_CTYPE,
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						expectedCookieValues: map[string]string{
							DEFAULT_SESSION_ID_COOKIE_NAME: "85216e5c138b662924f5831df3a55cc8",
						},
					},
					{
						pause:  10 * time.Millisecond,
						path:   "/sessions",
						method: "GET",
						header: http.Header{
							"Cookie": []string{DEFAULT_SESSION_ID_COOKIE_NAME + "=85216e5c138b662924f5831df3a55cc8"},
						},
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              "85216e5c138b662924f5831df3a55cc8:85216e5c138b662924f5831df3a55cc8;",
					},
				},
			},
			createClient,
		)
	})

	t.Run("each path parameter should have a corresponding entry in the context's data", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				outWriter: os.Stdout,
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() core.SnapshotableFilesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes/users/:user-id", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/users/:user-id/GET.ix", []byte(`
							manifest {}

							return ctx_data(/path-params/user-id)
						`), fs_ns.DEFAULT_FILE_FMODE)

					return fls
				},
				requests: []requestTestInfo{
					{
						path:                "/users/456",
						method:              "GET",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              "456",
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
