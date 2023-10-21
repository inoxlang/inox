package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	HTTP_CLIENT_PROPNAMES = []string{"get_host_cookies"}

	_ = []symbolic.ProtocolClient{(*HttpClient)(nil)}
)

type HttpClient struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (c *HttpClient) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpClient)
	return ok
}

func (c *HttpClient) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "get_host_cookies":
		return symbolic.WrapGoMethod(c.GetHostCookies), true
	}
	return nil, false
}

func (c *HttpClient) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, c)
}

func (*HttpClient) PropertyNames() []string {
	return HTTP_CLIENT_PROPNAMES
}

func (*HttpClient) Schemes() []string {
	return []string{"http", "https"}
}

func (c *HttpClient) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-client")
}

func (c *HttpClient) WidestOfType() symbolic.Value {
	return &HttpClient{}
}

func (c *HttpClient) GetHostCookies(h *symbolic.Host) *symbolic.List {
	return symbolic.NewListOf(NewCookieObject())
}
