package internal

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

func (c *HttpClient) IsMutable() bool {
	return true
}

func (*ServerSentEventSource) IsMutable() bool {
	return true
}
