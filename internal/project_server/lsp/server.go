package lsp

import (
	"fmt"
	"net"
	"net/http"
	"reflect"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"

	"github.com/inoxlang/inox/internal/globals/http_ns"
)

type Server struct {
	Methods
	customMethods []*jsonrpc.MethodInfo
	rpcServer     *jsonrpc.Server
	ctx           *core.Context //same context as the JSON RPC server.

	//websocket mode
	ServerCertificate    string
	ServerCertificateKey string
	ServerHttpHandler    http.Handler
}

func NewServer(ctx *core.Context, opt *Options) *Server {
	s := &Server{
		ctx: ctx,
	}
	s.Opt = *opt
	s.rpcServer = jsonrpc.NewServer(ctx, opt.OnSession)
	s.ServerHttpHandler = opt.HttpHandler
	return s
}

func (s *Server) Context() *core.Context {
	return s.ctx
}

func (s *Server) Run() error {
	mtds := s.GetMethods()
	for _, m := range mtds {
		if m != nil {
			s.rpcServer.RegisterMethod(*m)
		}
	}

	for _, m := range s.customMethods {
		if m != nil {
			s.rpcServer.RegisterMethod(*m)
		}
	}

	return s.run()
}

func (s *Server) OnCustom(info jsonrpc.MethodInfo) {
	for _, m := range s.customMethods {
		if m.Name == info.Name {
			panic(fmt.Errorf("handler for method %s is already set", m.Name))
		}
	}
	s.customMethods = append(s.customMethods, &info)
}

func (s *Server) run() error {
	addr := s.Opt.Address
	netType := s.Opt.Network

	if s.Opt.MessageReaderWriter != nil {
		logs.Println("use custom message reader+writer.")

		s.rpcServer.MsgConnComeIn(s.Opt.MessageReaderWriter, func(session *jsonrpc.Session) {})
	} else if netType != "" {
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

	if err := s.ctx.CheckHasPermission(core.RawTcpPermission{
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

	wsServer, err := jsonrpc.NewJsonRpcWebsocketServer(s.ctx, jsonrpc.JsonRpcWebsocketServerConfig{
		Addr:              addr,
		RpcServer:         s.rpcServer,
		MaxWebsocketPerIp: s.Opt.MaxWebsocketPerIp,
	})
	if err != nil {
		return err
	}

	httpServer, err := http_ns.NewGolangHttpServer(s.ctx, http_ns.GolangHttpServerConfig{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				wsServer.HandleNew(w, r)
				return
			}
			if s.ServerHttpHandler != nil {
				s.ServerHttpHandler.ServeHTTP(w, r)
			}
		}),
		PemEncodedCert: s.ServerCertificate,
		PemEncodedKey:  s.ServerCertificateKey,
	})

	if err != nil {
		return err
	}

	wsServer.Logger().Info().Msg("start HTTPS server")
	err = httpServer.ListenAndServeTLS("", "")
	if err != nil {
		return fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	return nil
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
