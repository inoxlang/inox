package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTTP_CLIENT_PROPNAMES = []string{"get_host_cookies"}

	_ = []symbolic.ProtocolClient{(*Client)(nil)}
)

type Client struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (c *Client) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Client)
	return ok
}

func (c *Client) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "get_host_cookies":
		return symbolic.WrapGoMethod(c.GetHostCookies), true
	}
	return nil, false
}

func (c *Client) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, c)
}

func (*Client) PropertyNames() []string {
	return HTTP_CLIENT_PROPNAMES
}

func (*Client) Schemes() []string {
	return []string{"http", "https"}
}

func (c *Client) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-client")
}

func (c *Client) WidestOfType() symbolic.Value {
	return &Client{}
}

func (c *Client) GetHostCookies(h *symbolic.Host) *symbolic.List {
	return symbolic.NewListOf(NewCookieObject())
}
