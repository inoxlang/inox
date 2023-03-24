package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

type HttpRequest struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpRequest) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HttpRequest)
	return ok
}

func (r HttpRequest) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &HttpRequest{}
}

func (req *HttpRequest) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return &symbolic.GoFunction{}, false
}

func (req *HttpRequest) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "method":
		return &symbolic.String{}
	case "url":
		return &symbolic.URL{}
	case "path":
		return &symbolic.Path{}
	case "body":
		return &symbolic.Reader{}
	case "headers":
		return symbolic.NewAnyKeyRecord(symbolic.NewTupleOf(&symbolic.String{}))
	case "cookies":
		//TODO
		fallthrough
	default:
		return symbolic.GetGoMethodOrPanic(name, req)
	}
}

func (HttpRequest) PropertyNames() []string {
	return []string{"method", "url", "path", "body" /*"cookies"*/, "headers"}
}

func (r *HttpRequest) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *HttpRequest) IsWidenable() bool {
	return false
}

func (r *HttpRequest) String() string {
	return "http-request"
}

func (r *HttpRequest) WidestOfType() symbolic.SymbolicValue {
	return &HttpRequest{}
}
