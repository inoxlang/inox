package http_ns

import (
	"testing"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

func TestFileServer(t *testing.T) {

	t.Run("missing http permission", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			},
		})
		defer ctx.CancelGracefully()

		server, err := NewFileServer(ctx, core.Host("https://localhost:9090"), core.Path("./"))
		if !assert.Error(t, err) {
			return
		}

		var notAllowedError *core.NotAllowedError
		if !assert.ErrorAs(t, err, &notAllowedError) {
			return
		}

		assert.IsType(t, core.HttpPermission{}, notAllowedError.Permission)
		assert.Nil(t, server)
	})

	t.Run("missing filesystem permission", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.HttpPermission{Kind_: permkind.Provide, Entity: core.Host("https://localhost:9090")},
			},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer ctx.CancelGracefully()

		server, err := NewFileServer(ctx, core.Host("https://localhost:9090"), core.Path("./"))
		if !assert.Error(t, err) {
			return
		}

		var notAllowedError *core.NotAllowedError
		if !assert.ErrorAs(t, err, &notAllowedError) {
			return
		}

		assert.IsType(t, core.FilesystemPermission{}, notAllowedError.Permission)
		assert.Nil(t, server)
	})

}
