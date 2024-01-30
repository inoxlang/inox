package net_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	WEBSOCKET_SERVER_PROPNAMES = []string{"upgrade", "close"}
)

type WebsocketServer struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (s *WebsocketServer) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*WebsocketServer)
	return ok
}

func (s *WebsocketServer) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "upgrade":
		return symbolic.WrapGoMethod(s.Upgrade), true
	case "close":
		return symbolic.WrapGoMethod(s.Close), true
	}
	return nil, false
}

func (s *WebsocketServer) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*WebsocketServer) PropertyNames() []string {
	return WEBSOCKET_SERVER_PROPNAMES
}

func (s *WebsocketServer) Upgrade(ctx *symbolic.Context, rw *http_symbolic.ResponseWriter, req *http_symbolic.Request) (*WebsocketConnection, *symbolic.Error) {
	return &WebsocketConnection{}, nil
}

func (s *WebsocketServer) Close(ctx *symbolic.Context) *symbolic.Error {
	return nil
}

func (s *WebsocketServer) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("websocket-server")
}

func (s *WebsocketServer) WidestOfType() symbolic.Value {
	return &WebsocketServer{}
}
