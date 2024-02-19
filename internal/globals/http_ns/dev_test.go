package http_ns

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTargetServerCreationAndDevServerForwarding(t *testing.T) {
	registerDefaultRequestLimits(t)
	registerDefaultMaxRequestHandlerLimits(t)

	//This test suite is not parallelized because we reuse one of the dev port across all tests.

	t.Run("base case", func(t *testing.T) {
		port := inoxconsts.DEV_PORT_1
		host := core.Host("https://localhost:" + port)
		url := string(host) + "/"

		fls := fs_ns.NewMemFilesystem(1_000_000)

		rootCtx := core.NewContexWithEmptyState(core.ContextConfig{
			Filesystem:  fls,
			Permissions: []core.Permission{core.HttpPermission{Kind_: permkind.Provide, Entity: host}},
		}, nil)

		defer rootCtx.CancelGracefully()

		err := StartDevServer(rootCtx, DevServerConfig{
			Port:                port,
			DevServersDir:       "/",
			BindToAllInterfaces: false,
		})

		if !assert.NoError(t, err) {
			return
		}

		//Wait for the dev server to start.

		time.Sleep(50 * time.Millisecond)

		targetServerCtx := core.NewContexWithEmptyState(core.ContextConfig{
			ParentContext: rootCtx,
			Permissions:   rootCtx.GetGrantedPermissions(),
		}, nil)

		sessionKey := RandomDevSessionKey()
		targetServerCtx.PutUserData(CTX_DATA_KEY_FOR_DEV_SESSION_KEY, core.String(sessionKey))

		//Create a server that should return NO_HANDLER_PLACEHOLDER_MESSAGE.
		server, err := NewHttpsServer(targetServerCtx, host)
		if !assert.NoError(t, err) {
			return
		}

		//Wait for the server to start.

		time.Sleep(50 * time.Millisecond)

		//Check that the target server is registered.

		devServer, ok := GetDevServer(port)
		if !assert.True(t, ok) {
			return
		}

		targetServer, ok := devServer.GetTargetServer(sessionKey)
		if !assert.True(t, ok) {
			return
		}

		if !assert.Same(t, server, targetServer) {
			return
		}

		//Check that requests with the session key header are forwarded.

		req, err := http.NewRequest("GET", url, nil)
		if !assert.NoError(t, err) {
			return
		}

		req.Header.Add(inoxconsts.DEV_SESSION_KEY_HEADER, string(sessionKey))

		client := newInsecureGolangHttpClient()

		resp, err := client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}

		if !assert.NoError(t, err) {
			return
		}

		assert.EqualValues(t, 200, resp.StatusCode)

		body := utils.Must(io.ReadAll(resp.Body))
		assert.Equal(t, NO_HANDLER_PLACEHOLDER_MESSAGE, string(body))

		//Check that requests without the session key header are not forwarded.

		req, err = http.NewRequest("GET", url, nil)
		if !assert.NoError(t, err) {
			return
		}

		resp, err = client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}

		if !assert.NoError(t, err) {
			return
		}

		assert.EqualValues(t, 400, resp.StatusCode)

		body = utils.Must(io.ReadAll(resp.Body))
		assert.NotEqual(t, NO_HANDLER_PLACEHOLDER_MESSAGE, string(body))

		//The dev server should not be running after the context cancellation.

		rootCtx.CancelGracefully()

		time.Sleep(50 * time.Millisecond)

		_, ok = GetDevServer(port)
		assert.False(t, ok)
	})

}
