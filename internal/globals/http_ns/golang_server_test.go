package http_ns

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestGolangHttpServer(t *testing.T) {

	t.Parallel()

	t.Run("self signed certificate should be regenerated if the file's age is greater than the validation duration", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000_000)
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Filesystem: fls,
		}, nil)
		defer ctx.CancelGracefully()

		addr := "localhost:" + nextPort()
		validityDuration := 300 * time.Millisecond

		server, err := NewGolangHttpServer(ctx, GolangHttpServerConfig{
			Addr:                           addr,
			Handler:                        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			SelfSignedCertValidityDuration: validityDuration,
			PersistCreatedLocalCert:        true,
		})
		if !assert.NoError(t, err) {
			return
		}

		prevCert, err := util.ReadFile(fls, "/"+SELF_SIGNED_CERT_FILEPATH)
		if !assert.NoError(t, err) {
			return
		}

		server.Close()

		//re opening the server immediately should not cause any change because the certificate should be reused.
		server, err = NewGolangHttpServer(ctx, GolangHttpServerConfig{
			Addr:                           addr,
			Handler:                        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			SelfSignedCertValidityDuration: validityDuration,
			PersistCreatedLocalCert:        true,
		})

		if !assert.NoError(t, err) {
			return
		}

		defer server.Close()

		currentCert, err := util.ReadFile(fls, "/"+SELF_SIGNED_CERT_FILEPATH)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, prevCert, currentCert)

		//re opening the server after a validityDuration delay should cause the certificate to be regenerated.

		time.Sleep(validityDuration)

		server, err = NewGolangHttpServer(ctx, GolangHttpServerConfig{
			Addr:                           addr,
			Handler:                        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			SelfSignedCertValidityDuration: validityDuration,
			PersistCreatedLocalCert:        true,
		})

		if !assert.NoError(t, err) {
			return
		}

		defer server.Close()

		currentCert, err = util.ReadFile(fls, "/"+SELF_SIGNED_CERT_FILEPATH)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotEqual(t, prevCert, currentCert)
	})

}
