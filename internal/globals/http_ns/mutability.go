package http_ns

func (serv *HttpsServer) IsMutable() bool {
	return true
}

func (req *HttpRequest) IsMutable() bool {
	// only mutation is when .headers record is created, creation/retrieval is protected by a lock

	return false
}

func (resp *HttpResponseWriter) IsMutable() bool {
	return true
}

func (resp *HttpResponse) IsMutable() bool {
	return true
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
