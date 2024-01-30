package http_ns

func (serv *HttpsServer) IsMutable() bool {
	return true
}

func (req Request) IsMutable() bool {
	return false
}

func (resp *ResponseWriter) IsMutable() bool {
	return true
}

func (resp *Response) IsMutable() bool {
	return true
}

func (resp *Result) IsMutable() bool {
	return true
}

func (s *Status) IsMutable() bool {
	return false
}

func (c *StatusCode) IsMutable() bool {
	return false
}

func (c *Client) IsMutable() bool {
	return true
}

func (*ServerSentEventSource) IsMutable() bool {
	return true
}

func (*ContentSecurityPolicy) IsMutable() bool {
	return false
}

func (*RequestPattern) IsMutable() bool {
	return false
}
