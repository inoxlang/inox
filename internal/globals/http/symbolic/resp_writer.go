package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
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
	case "writePlainText":
		return symbolic.WrapGoMethod(resp.WritePlainText), true
	case "writeBinary":
		return symbolic.WrapGoMethod(resp.WriteBinary), true
	case "writeHTML":
		return symbolic.WrapGoMethod(resp.WriteHTML), true
	case "writeJSON":
		return symbolic.WrapGoMethod(resp.WriteJSON), true
	case "writeIXON":
		return symbolic.WrapGoMethod(resp.WriteIXON), true
	case "setCookie":
		return symbolic.WrapGoMethod(resp.SetCookie), true
	case "writeStatus":
		return symbolic.WrapGoMethod(resp.WriteStatus), true
	case "writeError":
		return symbolic.WrapGoMethod(resp.WriteError), true
	case "addHeader":
		return symbolic.WrapGoMethod(resp.AddHeader), true
	default:
		return &symbolic.GoFunction{}, false
	}
}

func (resp *HttpResponseWriter) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, resp)
}

func (*HttpResponseWriter) PropertyNames() []string {
	return []string{
		"writePlainText", "writeBinary", "writeHTML", "writeJSON", "writeIXON", "setCookie", "writeStatus", "addHeader",
		"finish",
	}
}

func (r *HttpResponseWriter) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *HttpResponseWriter) IsWidenable() bool {
	return false
}

func (r *HttpResponseWriter) String() string {
	return "http-response-writer"
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
