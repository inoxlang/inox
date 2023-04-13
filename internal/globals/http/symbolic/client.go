package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

var (
	_ = []symbolic.ProtocolClient{&HttpClient{}}
)

type HttpClient struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (c *HttpClient) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HttpClient)
	return ok
}

func (c HttpClient) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &HttpClient{}
}

func (c *HttpClient) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "get_host_cookies":
		return symbolic.WrapGoMethod(c.GetHostCookies), true
	}
	return &symbolic.GoFunction{}, false
}

func (c *HttpClient) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, c)
}

func (*HttpClient) PropertyNames() []string {
	return []string{"wait_closed", "close", "get_host_cookies"}
}

func (*HttpClient) Schemes() []string {
	return []string{"http", "https"}
}

func (c *HttpClient) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (c *HttpClient) IsWidenable() bool {
	return false
}

func (c *HttpClient) String() string {
	return "%http-client"
}

func (c *HttpClient) WidestOfType() symbolic.SymbolicValue {
	return &HttpClient{}
}

func (c *HttpClient) GetHostCookies(h *symbolic.Host) *symbolic.List {
	return symbolic.NewListOf(NewCookieObject())
}
