package net_ns

import (
	"fmt"
	"net"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/miekg/dns"
)

const (
	TCP_SIMUL_CONN_TOTAL_LIMIT_NAME = "tcp/simul-connection"

	DEFAULT_TCP_DIAL_TIMEOUT        = 10 * time.Second
	DEFAULT_TCP_KEEP_ALIVE_INTERVAL = 10 * time.Second
	DEFAULT_TCP_BUFF_SIZE           = 1 << 16

	OPTION_DOES_NOT_EXIST_FMT = "option '%s' does not exist"
)

func dnsResolve(ctx *core.Context, domain core.String, recordTypeName core.String) ([]core.String, error) {
	defaultConfig, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
	client := new(dns.Client)
	//TODO: reuse client ?

	msg := new(dns.Msg)
	var recordType uint16

	perm := core.DNSPermission{Kind_: permbase.Read, Domain: core.Host("://" + domain)}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	switch recordTypeName {
	case "A":
		recordType = dns.TypeA
	case "AAAA":
		recordType = dns.TypeAAAA
	case "CNAME":
		recordType = dns.TypeCNAME
	case "MX":
		recordType = dns.TypeMX
	default:
		return nil, fmt.Errorf("invalid DNS record type: '%s'", recordTypeName)
	}

	msg.SetQuestion(dns.Fqdn(string(domain)), recordType)
	msg.RecursionDesired = true

	r, _, err := client.Exchange(msg, net.JoinHostPort(defaultConfig.Servers[0], defaultConfig.Port))
	if r == nil {
		return nil, fmt.Errorf("dns: error: %s", err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns: failure: response code is %d", r.Rcode)
	}

	records := []core.String{}
	for _, rr := range r.Answer {
		records = append(records, core.String(rr.String()))
	}

	return records, nil
}

func tcpConnect(ctx *core.Context, host core.Host) (*TcpConn, error) {

	perm := core.RawTcpPermission{
		Kind_:  permbase.Read,
		Domain: host,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	ctx.Take(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

	addr, err := net.ResolveTCPAddr("tcp", host.WithoutScheme())
	if err != nil {
		ctx.GiveBack(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		return nil, err
	}

	dialer := net.Dialer{
		Timeout:   DEFAULT_TCP_DIAL_TIMEOUT,
		KeepAlive: DEFAULT_TCP_KEEP_ALIVE_INTERVAL,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr.String())
	if err != nil {
		ctx.GiveBack(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		return nil, err
	}

	return &TcpConn{
		initialCtx: ctx,
		conn:       conn.(*net.TCPConn),
		host:       host,
	}, nil
}
