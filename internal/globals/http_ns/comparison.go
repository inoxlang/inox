package http_ns

import (
	"slices"

	"github.com/inoxlang/inox/internal/core"
)

func (s *HttpsServer) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherServer, ok := other.(*HttpsServer)
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

func (r *HttpResult) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResult, ok := other.(*HttpResult)
	return ok && r == otherResult
}

func (s Status) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStatus, ok := other.(Status)
	if !ok {
		return false
	}
	return s.code == otherStatus.code
}

func (c StatusCode) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCode, ok := other.(StatusCode)
	if !ok {
		return false
	}
	return c == otherCode
}

func (c *HttpClient) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherClient, ok := other.(*HttpClient)
	return ok && c == otherClient
}

func (evs *ServerSentEventSource) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSource, ok := other.(*ServerSentEventSource)
	return ok && evs == otherSource
}

func (c *ContentSecurityPolicy) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCSP, ok := other.(*ContentSecurityPolicy)
	if !ok {
		return false
	}
	if len(c.directives) != len(otherCSP.directives) {
		return false
	}
	for name, directive := range c.directives {
		otherDirective, ok := otherCSP.directives[name]
		if !ok || len(directive.values) != len(otherDirective.values) {
			return false
		}
		for i, val := range directive.values {
			if otherDirective.values[i].raw != val.raw {
				return false
			}
		}
	}
	return true
}

func (p *HttpRequestPattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*HttpRequestPattern)
	if !ok || !slices.Equal(p.methods, otherPattern.methods) {
		return false
	}

	return p.headers.Equal(ctx, otherPattern.headers, alreadyCompared, depth+1)
}
