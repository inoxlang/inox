package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	http_symbolic "github.com/inox-project/inox/internal/globals/http/symbolic"
)

func (serv *HttpServer) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpServer{}, nil
}

func (req *HttpRequest) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpRequest{}, nil
}

func (resp *HttpResponseWriter) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpResponseWriter{}, nil
}

func (resp *HttpResponse) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpResponse{}, nil
}

func (c *HttpClient) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.HttpClient{}, nil
}

func (evs *ServerSentEventSource) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &http_symbolic.ServerSentEventSource{}, nil
}
