package internal

import (
	"bufio"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
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
	return nil, false
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

func (c *HttpClient) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%http-client")))
	return
}

func (c *HttpClient) WidestOfType() symbolic.SymbolicValue {
	return &HttpClient{}
}

func (c *HttpClient) GetHostCookies(h *symbolic.Host) *symbolic.List {
	return symbolic.NewListOf(NewCookieObject())
}
