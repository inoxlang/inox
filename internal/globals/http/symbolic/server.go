package internal

import symbolic "github.com/inox-project/inox/internal/core/symbolic"

type HttpServer struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpServer) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HttpServer)
	return ok
}

func (r HttpServer) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &HttpServer{}
}

func (serv *HttpServer) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "wait_closed":
		return symbolic.WrapGoMethod(serv.wait_closed), true
	case "close":
		return symbolic.WrapGoMethod(serv.close), true
	}
	return &symbolic.GoFunction{}, false
}

func (s *HttpServer) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*HttpServer) PropertyNames() []string {
	return []string{"wait_closed", "close"}
}

func (serv *HttpServer) wait_closed(ctx *symbolic.Context) {
}

func (serv *HttpServer) close(ctx *symbolic.Context) {
}

func (r *HttpServer) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *HttpServer) IsWidenable() bool {
	return false
}

func (r *HttpServer) String() string {
	return "http-server"
}

func (r *HttpServer) WidestOfType() symbolic.SymbolicValue {
	return &HttpServer{}
}
