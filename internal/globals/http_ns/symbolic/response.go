package http_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_RESP = &HttpResponse{}

	HTTP_RESPONSE_PROPNAMES = []string{"body", "status", "statusCode", "cookies"}
)

type HttpResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpResponse) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HttpResponse)
	return ok
}

func (resp *HttpResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *HttpResponse) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "body":
		return &symbolic.Reader{}
	case "status":
		return &symbolic.String{}
	case "statusCode":
		return &symbolic.Int{}
	case "cookies":
		return symbolic.NewListOf(NewCookieObject())
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*HttpResponse) PropertyNames() []string {
	return HTTP_RESPONSE_PROPNAMES
}

func (r *HttpResponse) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *HttpResponse) IsWidenable() bool {
	return false
}

func (r *HttpResponse) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%http-response")))
	return
}

func (r *HttpResponse) WidestOfType() symbolic.SymbolicValue {
	return &HttpResponse{}
}
