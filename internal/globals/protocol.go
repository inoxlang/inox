package globals

import "github.com/inoxlang/inox/internal/core"

func setClientForURL(ctx *core.Context, u core.URL, client core.ProtocolClient) error {
	return ctx.SetProtocolClientForURL(u, client)
}

func setClientForHost(ctx *core.Context, h core.Host, client core.ProtocolClient) error {
	return ctx.SetProtocolClientForHost(h, client)
}
