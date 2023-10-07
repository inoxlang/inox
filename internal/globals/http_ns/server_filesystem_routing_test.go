package http_ns

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/local_db"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/stretchr/testify/assert"
)

func TestFilesystemRouting(t *testing.T) {

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

	t.Run("GET /x should return the result of /routes/GET-x.ix even if /routes/x.ix is present", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() afs.Filesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/GET-x.ix", []byte(`
							manifest {}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					util.WriteFile(fls, "/routes/x.ix", []byte(`
						manifest {}

						return "default"
					`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:              "GET",
						acceptedContentType: mimeconsts.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("GET /x should return the result of /routes/x/GET.ix even if /routes/x/index.ix is present", func(t *testing.T) {
		runServerTest(t,
			serverTestCase{
				input: `return {
						routing: {dynamic: /routes/}
					}`,
				makeFilesystem: func() afs.Filesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes/x", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/x/GET.ix", []byte(`
							manifest {}
	
							return "hello"
						`), fs_ns.DEFAULT_FILE_FMODE)

					util.WriteFile(fls, "/routes/x/index.ix", []byte(`
						manifest {}

						return "default"
					`), fs_ns.DEFAULT_FILE_FMODE)
					return fls
				},
				requests: []requestTestInfo{
					{
						method:              "GET",
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

	t.Run("method-aspecific handler module with %reader _body parameter should accept all methods", func(t *testing.T) {
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

	t.Run("an error should be returned if a method-aspecific handler module has a JSON body parameter", func(t *testing.T) {
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

	t.Run("method-aspecific handler module with %(#POST) _method parameter should only accept POST requests", func(t *testing.T) {
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

				localDb, err := local_db.OpenDatabase(gs.Ctx, host, false)
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
					core.INT_PATTERN,
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

		t.Run("GET /x with content-type: text/plain", func(t *testing.T) {
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

		t.Run("GET /x with content-type: */*", func(t *testing.T) {
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
	})

}
