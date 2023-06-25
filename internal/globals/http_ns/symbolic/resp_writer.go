package http_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type HttpResponseWriter struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpResponseWriter) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HttpResponseWriter)
	return ok
}

func (r HttpResponseWriter) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &HttpResponseWriter{}
}

func (resp *HttpResponseWriter) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "write_text":
		return symbolic.WrapGoMethod(resp.WritePlainText), true
	case "write_binary":
		return symbolic.WrapGoMethod(resp.WriteBinary), true
	case "write_html":
		return symbolic.WrapGoMethod(resp.WriteHTML), true
	case "write_json":
		return symbolic.WrapGoMethod(resp.WriteJSON), true
	case "write_ixon":
		return symbolic.WrapGoMethod(resp.WriteIXON), true
	case "set_cookie":
		return symbolic.WrapGoMethod(resp.SetCookie), true
	case "write_status":
		return symbolic.WrapGoMethod(resp.WriteStatus), true
	case "write_error":
		return symbolic.WrapGoMethod(resp.WriteError), true
	case "add_header":
		return symbolic.WrapGoMethod(resp.AddHeader), true
	default:
		return nil, false
	}
}

func (resp *HttpResponseWriter) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, resp)
}

func (*HttpResponseWriter) PropertyNames() []string {
	return []string{
		"write_text", "write_binary", "write_html", "write_json", "write_ixon", "set_cookie", "write_status", "write_error", "add_header",
	}
}

func (r *HttpResponseWriter) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *HttpResponseWriter) IsWidenable() bool {
	return false
}

func (r *HttpResponseWriter) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%http-response-writer")))
}

func (r *HttpResponseWriter) WidestOfType() symbolic.SymbolicValue {
	return &HttpResponseWriter{}
}

func (resp *HttpResponseWriter) WritePlainText(ctx *symbolic.Context, v *symbolic.ByteSlice) (*symbolic.Int, *symbolic.Error) {
	return &symbolic.Int{}, nil
}

func (resp *HttpResponseWriter) WriteBinary(ctx *symbolic.Context, v *symbolic.ByteSlice) (*symbolic.Int, *symbolic.Error) {
	return &symbolic.Int{}, nil
}

func (resp *HttpResponseWriter) WriteHTML(ctx *symbolic.Context, v symbolic.SymbolicValue) (*symbolic.Int, *symbolic.Error) {
	return &symbolic.Int{}, nil
}

func (resp *HttpResponseWriter) WriteJSON(ctx *symbolic.Context, v symbolic.SymbolicValue) (*symbolic.Int, *symbolic.Error) {
	return &symbolic.Int{}, nil
}

func (resp *HttpResponseWriter) WriteIXON(ctx *symbolic.Context, v symbolic.SymbolicValue) *symbolic.Error {
	return nil
}

func (resp *HttpResponseWriter) SetCookie(ctx *symbolic.Context, obj *symbolic.Object) *symbolic.Error {
	return nil
}

func (resp *HttpResponseWriter) WriteStatus(ctx *symbolic.Context, status *symbolic.Int) {
}

func (resp *HttpResponseWriter) WriteError(ctx *symbolic.Context, err *symbolic.Error, status *symbolic.Int) {
}

func (resp *HttpResponseWriter) AddHeader(ctx *symbolic.Context, k, v *symbolic.String) {
}
func (resp *HttpResponseWriter) Finish(ctx *symbolic.Context) {
}
