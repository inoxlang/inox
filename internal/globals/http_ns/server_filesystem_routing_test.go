package http_ns

import (
	"net/http"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
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
					{acceptedContentType: core.PLAIN_TEXT_CTYPE, result: `hello`},
				},
			},
			createClient,
		)
	})

	t.Run("POST /x should return the result of /routes/x.ix", func(t *testing.T) {
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
						method:              "POST",
						header:              http.Header{"Content-Type": []string{core.PLAIN_TEXT_CTYPE}},
						acceptedContentType: core.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
				},
			},
			createClient,
		)
	})

	t.Run("POST /x should return the result of :/routes/POST-x.ix even if /routes/x.ix is present", func(t *testing.T) {
		runMappingTestCase(t,
			serverTestCase{
				input: `return {
						routing: /routes/
					}`,
				makeFilesystem: func() afs.Filesystem {
					fls := fs_ns.NewMemFilesystem(10_000)
					fls.MkdirAll("/routes", fs_ns.DEFAULT_DIR_FMODE)
					util.WriteFile(fls, "/routes/POST-x.ix", []byte(`
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
						method:              "POST",
						header:              http.Header{"Content-Type": []string{core.PLAIN_TEXT_CTYPE}},
						acceptedContentType: core.PLAIN_TEXT_CTYPE,
						result:              `hello`,
					},
				},
			},
			createClient,
		)
	})
}
