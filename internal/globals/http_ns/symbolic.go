package http_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

func (serv *HttpServer) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpServer{}, nil
}

func (req *HttpRequest) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpRequest{}, nil
}

func (resp *HttpResponseWriter) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpResponseWriter{}, nil
}

func (resp *HttpResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpResponse{}, nil
}

func (c *HttpClient) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpClient{}, nil
}

func (evs *ServerSentEventSource) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.ServerSentEventSource{}, nil
}

func (*ContentSecurityPolicy) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return http_symbolic.NewCSP(), nil
}
