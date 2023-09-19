package http_ns

import (
	"net/http"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

func TestFilesystemRouting(t *testing.T) {

	t.Run("GET /x should return the result of /routes/x.ix", func(t *testing.T) {
		runMappingTestCase(t,
			serverTestCase{
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
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

}
