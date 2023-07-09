package net_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	help_ns "github.com/inoxlang/inox/internal/globals/help_ns"
	net_symbolic "github.com/inoxlang/inox/internal/globals/net_ns/symbolic"
)

func init() {
	// register limitations
	core.LimRegistry.RegisterLimitation(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimitation, 0)
	core.LimRegistry.RegisterLimitation(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimitation, 0)

	// register symbolic version of Go Functions
	core.RegisterSymbolicGoFunctions([]any{
		tcpConnect, func(ctx *symbolic.Context, host *symbolic.Host) (*net_symbolic.TcpConn, *symbolic.Error) {
			return &net_symbolic.TcpConn{}, nil
		},
		websocketConnect, func(ctx *symbolic.Context, u *symbolic.URL, opts ...*symbolic.Option) (*net_symbolic.WebsocketConnection, *symbolic.Error) {
			return &net_symbolic.WebsocketConnection{}, nil
		},
		NewWebsocketServer, func(ctx *symbolic.Context) (*net_symbolic.WebsocketServer, *symbolic.Error) {
			return &net_symbolic.WebsocketServer{}, nil
		},
		dnsResolve, func(ctx *symbolic.Context, domain *symbolic.String, recordTypeName *symbolic.String) (*symbolic.List, *symbolic.Error) {
			return symbolic.NewListOf(&symbolic.String{}), nil
		},
	})

	help_ns.RegisterHelpValues(map[string]any{
		"dns.resolve": dnsResolve,
		"tcp.connect": tcpConnect,
	})
}

func NewTcpNamespace() *core.Namespace {
	return core.NewNamespace("tcp", map[string]core.Value{
		"connect": core.ValOf(tcpConnect),
	})
}

func NewDNSnamespace() *core.Namespace {
	f := func() (int, int) {
		return 1, 1
	}
	_, _ = f()
	return core.NewNamespace("dns", map[string]core.Value{
		"resolve": core.ValOf(dnsResolve),
	})
}

func NewWebsocketNamespace() *core.Namespace {
	return core.NewNamespace("ws", map[string]core.Value{
		"connect": core.ValOf(websocketConnect),
		"Server":  core.ValOf(NewWebsocketServer),
	})
}
