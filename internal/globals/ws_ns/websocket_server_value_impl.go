package ws_ns

import "github.com/inoxlang/inox/internal/core"

//core.Value implementation for WebsocketServer.

func (s *WebsocketServer) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "upgrade":
		return core.WrapGoMethod(s.Upgrade), true
	case "close":
		return core.WrapGoMethod(s.Close), true
	}
	return nil, false
}

func (s *WebsocketServer) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*WebsocketServer) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*WebsocketServer) PropertyNames(ctx *core.Context) []string {
	return []string{"upgrade", "close"}
}
