package lsp

import (
	"fmt"
	"net"
	"reflect"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/permkind"

	core "github.com/inoxlang/inox/internal/core"

	_net "github.com/inoxlang/inox/internal/globals/net_ns"
)

type Server struct {
	Methods
	rpcServer *jsonrpc.Server
	ctx       *core.Context
}

func NewServer(ctx *core.Context, opt *Options) *Server {
	s := &Server{
		ctx: ctx,
	}
	s.Opt = *opt
	s.rpcServer = jsonrpc.NewServer(opt.OnSession)
	return s
}

func (s *Server) Run() error {
	mtds := s.GetMethods()
	for _, m := range mtds {
		if m != nil {
			s.rpcServer.RegisterMethod(*m)
		}
	}

	return s.run()
}

func (s *Server) run() error {
	addr := s.Opt.Address
	netType := s.Opt.Network
	if netType != "" {
		switch netType {
		case "wss":
			return s.startWebsocketServer(addr)
		case "tcp":
			return s.startTcpServer(netType, addr)
		default:
			return fmt.Errorf("network type %s is not supported/allowed", netType)
		}
	} else {
		logs.Println("use stdio mode.")
		// use stdio mode

		var stdio jsonrpc.ReaderWriter
		if s.Opt.StdioInput != nil && s.Opt.StdioOutput != nil {
			stdio = &stdioReaderWriter{
				reader: s.Opt.StdioInput,
				writer: s.Opt.StdioOutput,
			}
		} else {
			stdio = NewStdio()
		}

		s.rpcServer.ConnComeIn(stdio)
	}
	return nil
}

func (s *Server) startTcpServer(netType string, addr string) error {
	if addr == "" {
		addr = "127.0.0.1:7998"
	}

	if err := s.ctx.CheckHasPermission(_net.RawTcpPermission{
		Kind_:  permkind.Provide,
		Domain: core.Host("://" + addr),
	}); err != nil {
		return err
	}

	logs.Printf("use socket mode: net: %s, addr: %s\n", netType, addr)
	listener, err := net.Listen(netType, addr)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go s.rpcServer.ConnComeIn(conn)
	}
}

func (s *Server) startWebsocketServer(addr string) error {
	if addr == "" {
		addr = "127.0.0.1:7998"
	}

	wsServer, err := NewJsonRpcWebsocketServer(s.ctx, addr, s.rpcServer)
	if err != nil {
		return err
	}

	return wsServer.Listen()
}

func wrapErrorToRespError(err interface{}, code int) error {
	if isNil(err) {
		return nil
	}
	if e, ok := err.(error); ok {
		return e
	}
	return jsonrpc.ResponseError{
		Code:    code,
		Message: fmt.Sprintf("%v", err),
		Data:    err,
	}
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return true
	}
	return false
}
