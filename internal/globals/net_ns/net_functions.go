package net_ns

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/miekg/dns"
)

const (
	HTTP_UPLOAD_RATE_LIMIT_NAME     = "http/upload"
	WS_SIMUL_CONN_TOTAL_LIMIT_NAME  = "ws/simul-connection"
	TCP_SIMUL_CONN_TOTAL_LIMIT_NAME = "tcp/simul-connection"
	HTTP_REQUEST_RATE_LIMIT_NAME    = "http/request"

	DEFAULT_TCP_DIAL_TIMEOUT        = 10 * time.Second
	DEFAULT_TCP_KEEP_ALIVE_INTERVAL = 10 * time.Second
	DEFAULT_TCP_BUFF_SIZE           = 1 << 16

	DEFAULT_HTTP_CLIENT_TIMEOUT = 10 * time.Second

	OPTION_DOES_NOT_EXIST_FMT = "option '%s' does not exist"
)

func websocketConnect(ctx *Context, u URL, options ...Option) (*WebsocketConnection, error) {
	insecure := false

	for _, opt := range options {
		switch opt.Name {
		case "insecure":
			insecure = bool(opt.Value.(Bool))
		default:
			return nil, core.FmtErrInvalidOptionName(opt.Name)
		}
	}

	perm := WebsocketPermission{
		Kind_:    permkind.Read,
		Endpoint: u,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	ctx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: insecure,
	}

	c, _, err := dialer.Dial(string(u), nil)
	if err != nil {
		ctx.GiveBack(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		return nil, fmt.Errorf("dial: %s", err.Error())
	}

	return &WebsocketConnection{
		conn:           c,
		endpoint:       u,
		messageTimeout: DEFAULT_WS_MESSAGE_TIMEOUT,
		serverContext:  ctx,
	}, nil
}

func dnsResolve(ctx *Context, domain Str, recordTypeName Str) ([]Str, error) {
	defaultConfig, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
	client := new(dns.Client)
	//TODO: reuse client ?

	msg := new(dns.Msg)
	var recordType uint16

	perm := DNSPermission{Kind_: permkind.Read, Domain: Host("://" + domain)}
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

	records := []Str{}
	for _, rr := range r.Answer {
		records = append(records, Str(rr.String()))
	}

	return records, nil
}

func tcpConnect(ctx *Context, host Host) (*TcpConn, error) {

	perm := RawTcpPermission{
		Kind_:  permkind.Read,
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
