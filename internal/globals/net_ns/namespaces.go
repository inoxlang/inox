package net_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	net_symbolic "github.com/inoxlang/inox/internal/globals/net_ns/symbolic"
	"github.com/inoxlang/inox/internal/help"
)

func init() {
	// register limits
	core.RegisterLimit(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimit, 0)

	// register symbolic version of Go Functions
	core.RegisterSymbolicGoFunctions([]any{
		tcpConnect, func(ctx *symbolic.Context, host *symbolic.Host) (*net_symbolic.TcpConn, *symbolic.Error) {
			return &net_symbolic.TcpConn{}, nil
		},
		dnsResolve, func(ctx *symbolic.Context, domain *symbolic.String, recordTypeName *symbolic.String) (*symbolic.List, *symbolic.Error) {
			return symbolic.NewListOf(symbolic.ANY_STRING), nil
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
