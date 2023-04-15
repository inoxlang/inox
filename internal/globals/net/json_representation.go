package internal

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
)

func (*WebsocketConnection) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (conn *WebsocketConnection) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*TcpConn) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (conn *TcpConn) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}
