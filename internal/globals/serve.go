package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	_http "github.com/inoxlang/inox/internal/globals/http"
)

func _serve(ctx *core.Context, resource core.ResourceName, args ...core.Value) error {
	server, err := _http.NewHttpServer(ctx, resource)
	if err != nil {
		return err
	}

	server.WaitClosed(ctx)
	return nil
}
