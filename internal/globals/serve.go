package globals

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
)

func _serve(ctx *core.Context, resource core.ResourceName, args ...core.Value) error {
	server, err := http_ns.NewHttpsServer(ctx, resource.(core.Host))
	if err != nil {
		return err
	}

	server.WaitClosed(ctx)
	return nil
}
