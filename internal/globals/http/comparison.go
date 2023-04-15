package internal

import core "github.com/inoxlang/inox/internal/core"

func (s *HttpServer) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherServer, ok := other.(*HttpServer)
	return ok && s == otherServer
}

func (r *HttpRequest) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherReq, ok := other.(*HttpRequest)
	if !ok {
		return false
	}

	return r.Request() == otherReq.Request()
}

func (rw *HttpResponseWriter) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResp, ok := other.(*HttpResponseWriter)
	return ok && rw == otherResp
}

func (r *HttpResponse) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResp, ok := other.(*HttpResponse)
	return ok && r == otherResp
}

func (c *HttpClient) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherClient, ok := other.(*HttpClient)
	return ok && c == otherClient
}

func (evs *ServerSentEventSource) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSource, ok := other.(*ServerSentEventSource)
	return ok && evs == otherSource
}
