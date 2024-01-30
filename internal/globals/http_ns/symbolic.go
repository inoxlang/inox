package http_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

func (serv *HttpsServer) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &http_symbolic.HttpsServer{}, nil
}

func (req *Request) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &http_symbolic.Request{}, nil
}

func (resp *ResponseWriter) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &http_symbolic.ResponseWriter{}, nil
}

func (resp *Response) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &http_symbolic.Response{}, nil
}

func (res *Result) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return http_symbolic.ANY_RESULT, nil
}

func (s Status) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return http_symbolic.ANY_STATUS, nil
}

func (c StatusCode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return http_symbolic.ANY_STATUS_CODE, nil
}

func (c *Client) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &http_symbolic.Client{}, nil
}

func (evs *ServerSentEventSource) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &http_symbolic.ServerSentEventSource{}, nil
}

func (*ContentSecurityPolicy) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return http_symbolic.NewCSP(), nil
}

func (*RequestPattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, core.ErrNotImplementedYet
}
