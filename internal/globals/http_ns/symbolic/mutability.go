package http_ns

func (serv *HttpServer) IsMutable() bool {
	return true
}

func (req HttpRequest) IsMutable() bool {
	return false
}

func (resp *HttpResponseWriter) IsMutable() bool {
	return true
}

func (resp *HttpResponse) IsMutable() bool {
	return true
}

func (resp *HttpResult) IsMutable() bool {
	return true
}

func (s *Status) IsMutable() bool {
	return false
}

func (c *StatusCode) IsMutable() bool {
	return false
}

func (c *HttpClient) IsMutable() bool {
	return true
}

func (*ServerSentEventSource) IsMutable() bool {
	return true
}

func (*ContentSecurityPolicy) IsMutable() bool {
	return false
}

func (*HttpRequestPattern) IsMutable() bool {
	return false
}
