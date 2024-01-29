package http_ns

import (
	"crypto/tls"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGolangHttpServer(t *testing.T) {

	testconfig.AllowParallelization(t)

	t.Run("self signed certificate should be regenerated if the file's age is greater than the validation duration", func(t *testing.T) {
		testconfig.AllowParallelization(t)

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

		prevCert, err := util.ReadFile(fls, "/"+RELATIVE_SELF_SIGNED_CERT_FILEPATH)
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

		currentCert, err := util.ReadFile(fls, "/"+RELATIVE_SELF_SIGNED_CERT_FILEPATH)
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

		currentCert, err = util.ReadFile(fls, "/"+RELATIVE_SELF_SIGNED_CERT_FILEPATH)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotEqual(t, prevCert, currentCert)
	})

	t.Run("listen on 0.0.0.0:port, allow generation of a self signed cert", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		fls := fs_ns.NewMemFilesystem(1_000_000)
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Filesystem: fls,
		}, nil)
		defer ctx.CancelGracefully()

		port := nextPort()
		addr := ":" + port
		validityDuration := 300 * time.Millisecond
		randomString := core.DefaultRandSource.ReadNBytesAsHex(4)

		server, err := NewGolangHttpServer(ctx, GolangHttpServerConfig{
			Addr: addr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(randomString))
			}),
			SelfSignedCertValidityDuration:           validityDuration,
			AllowSelfSignedCertCreationEvenIfExposed: true,
		})
		if !assert.NoError(t, err) {
			return
		}
		go func() {
			defer utils.Recover()
			server.ListenAndServeTLS("", "")
		}()
		defer server.Close()

		time.Sleep(10 * time.Millisecond)

		ips := utils.Must(netaddr.GetGlobalUnicastIPs())
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		for _, ip := range ips {
			resp, err := client.Get("https://" + ip.String() + ":" + port)
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if err != nil {
				assert.Fail(t, err.Error())
			}

			body := utils.Must(io.ReadAll(resp.Body))
			assert.Equal(t, randomString, string(body))
		}
	})
}
