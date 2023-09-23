package net_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	net_symbolic "github.com/inoxlang/inox/internal/globals/net_ns/symbolic"
	"github.com/inoxlang/inox/internal/help"
)

func init() {
	// register limits
	core.LimRegistry.RegisterLimit(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimit, 0)
	core.LimRegistry.RegisterLimit(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimit, 0)

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

	help.RegisterHelpValues(map[string]any{
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
