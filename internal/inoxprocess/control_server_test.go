package inoxprocess

import (
	"os/exec"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestControlServer(t *testing.T) {
	inoxBinaryPath, err := exec.LookPath("inox")
	if err != nil {
		t.Skipf("the inox binary has not been found: %s", err.Error())
	}

	ctx := core.NewContexWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.WebsocketPermission{Kind_: permkind.Provide},
		},
		Filesystem: fs_ns.NewMemFilesystem(10_000),
	}, nil)

	defer ctx.CancelGracefully()

	server, err := NewControlServer(ctx, ControlServerConfig{
		InoxBinaryPath: inoxBinaryPath,
		Port:           8310,
	})

	if !assert.NoError(t, err) {
		return
	}

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

	process, err := server.CreateControlledProcess(grantedPerms, nil)

	if !assert.NoError(t, err) {
		return
	}

	if process.cmd.Process != nil {
		defer process.cmd.Process.Kill()
	}
}
