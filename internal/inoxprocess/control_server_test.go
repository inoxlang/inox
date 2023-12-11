package inoxprocess

import (
	"errors"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
)

func TestControlServer(t *testing.T) {
	RegisterTypesInGob()

	inoxBinaryPath, err := exec.LookPath("inox")

	setup := func() (*core.Context, *ControlServer) {
		if err != nil {
			t.Skipf("the inox binary has not been found: %s", err.Error())
		}

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: permkind.Provide},
			},
			Filesystem: fs_ns.NewMemFilesystem(10_000),
		}, /*os.Stdout*/ nil)

		server, err := NewControlServer(ctx, ControlServerConfig{
			InoxBinaryPath: inoxBinaryPath,
			Port:           8310,
		})

		if !assert.NoError(t, err) {
			return nil, nil
		}

		return ctx, server
	}

	t.Run("create process", func(t *testing.T) {
		ctx, server := setup()
		if server == nil {
			return
		}
		defer ctx.CancelGracefully()

		go func() {
			err := server.Start()
			if err != nil {
				t.Log(err)
			}
		}()

		time.Sleep(100 * time.Millisecond)

		grantedPerms := []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		}

		proc, err := server.CreateControlledProcess(grantedPerms, nil)

		if !assert.NoError(t, err) {
			return
		}

		exists := utils.Must(process.PidExists(int32(proc.cmd.Process.Pid)))
		if !assert.True(t, exists) {
			return
		}

		if proc.cmd.Process != nil {
			defer proc.cmd.Process.Kill()
		}
	})

	t.Run("stop process executing nothing", func(t *testing.T) {
		t.Skip("TO FIX")

		ctx, server := setup()
		if server == nil {
			return
		}
		defer ctx.CancelGracefully()

		go func() {
			err := server.Start()
			if !errors.Is(err, http.ErrServerClosed) {
				t.Log(err)
			}
		}()

		time.Sleep(100 * time.Millisecond)

		grantedPerms := []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		}

		proc, err := server.CreateControlledProcess(grantedPerms, nil)

		if !assert.NoError(t, err) {
			return
		}

		exists := utils.Must(process.PidExists(int32(proc.cmd.Process.Pid)))
		if !assert.True(t, exists) {
			return
		}

		go func() {
			//test timeout
			time.Sleep(time.Second)
			ctx.CancelGracefully()
		}()

		proc.Stop(ctx)

		time.Sleep(10 * time.Millisecond)
		exists = utils.Must(process.PidExists(int32(proc.cmd.Process.Pid)))
		assert.False(t, exists)
	})

	//TODO: test that an orphan inox process exits a short time after too many reconnection attemps
}
