package http_ns

import (
	"github.com/inoxlang/inox/internal/core"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

func (c *Client) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "get_host_cookies":
		return core.WrapGoMethod(c.GetHostCookieObjects), true
	}
	return nil, false
}

func (c *Client) Prop(ctx *core.Context, name string) core.Value {
	method, ok := c.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, c))
	}
	return method

}

func (*Client) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Client) PropertyNames(ctx *core.Context) []string {
	return http_ns_symb.HTTP_CLIENT_PROPNAMES
}
