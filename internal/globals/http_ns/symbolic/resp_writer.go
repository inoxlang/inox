package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTTP_RESP_WRITER_PROPNAMES = []string{
		"write_text", "write_binary", "write_html", "write_json", "set_cookie", "write_headers", "write_error",
		"add_header", "set_status",
	}

	ANY_HTTP_RESP_WRITER = &ResponseWriter{}
)

type ResponseWriter struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *ResponseWriter) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*ResponseWriter)
	return ok
}

func (rw *ResponseWriter) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "write_text":
		return symbolic.WrapGoMethod(rw.WritePlainText), true
	case "write_binary":
		return symbolic.WrapGoMethod(rw.WriteBinary), true
	case "write_html":
		return symbolic.WrapGoMethod(rw.WriteHTML), true
	case "write_json":
		return symbolic.WrapGoMethod(rw.WriteJSON), true
	case "set_cookie":
		return symbolic.WrapGoMethod(rw.SetCookie), true
	case "set_status":
		return symbolic.WrapGoMethod(rw.SetStatus), true
	case "write_headers":
		return symbolic.WrapGoMethod(rw.WriteHeaders), true
	case "write_error":
		return symbolic.WrapGoMethod(rw.WriteError), true
	case "add_header":
		return symbolic.WrapGoMethod(rw.AddHeader), true
	default:
		return nil, false
	}
}

func (rw *ResponseWriter) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, rw)
}

func (*ResponseWriter) PropertyNames() []string {
	return HTTP_RESP_WRITER_PROPNAMES
}

func (r *ResponseWriter) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-response-writer")
}

func (r *ResponseWriter) WidestOfType() symbolic.Value {
	return ANY_HTTP_RESP_WRITER
}

func (rw *ResponseWriter) WritePlainText(ctx *symbolic.Context, v *symbolic.ByteSlice) (*symbolic.Int, *symbolic.Error) {
	return symbolic.ANY_INT, nil
}

func (rw *ResponseWriter) WriteBinary(ctx *symbolic.Context, v *symbolic.ByteSlice) (*symbolic.Int, *symbolic.Error) {
	return symbolic.ANY_INT, nil
}

func (rw *ResponseWriter) WriteHTML(ctx *symbolic.Context, v symbolic.Value) (*symbolic.Int, *symbolic.Error) {
	return symbolic.ANY_INT, nil
}

func (rw *ResponseWriter) WriteJSON(ctx *symbolic.Context, v symbolic.Serializable) (*symbolic.Int, *symbolic.Error) {
	return symbolic.ANY_INT, nil
}

func (rw *ResponseWriter) SetCookie(ctx *symbolic.Context, obj *symbolic.Object) *symbolic.Error {
	return nil
}

func (rw *ResponseWriter) SetStatus(ctx *symbolic.Context, status *StatusCode) {
}

func (rw *ResponseWriter) WriteHeaders(ctx *symbolic.Context, status *symbolic.OptionalParam[*StatusCode]) {
}

func (rw *ResponseWriter) WriteError(ctx *symbolic.Context, err *symbolic.Error, status *StatusCode) {
}

func (rw *ResponseWriter) AddHeader(ctx *symbolic.Context, k, v *symbolic.String) {
}

func (rw *ResponseWriter) Finish(ctx *symbolic.Context) {
}
