package html_ns

import (
	"bytes"
	"io"

	"github.com/inoxlang/inox/internal/core"
	"golang.org/x/net/html"
)

func (n *HTMLNode) Render(ctx *core.Context, w io.Writer) (int, error) {
	if n.render != nil {
		return w.Write(n.render)
	}

	buf := bytes.NewBuffer(nil)
	err := html.Render(buf, n.node)
	if err != nil {
		return 0, err
	}
	n.render = buf.Bytes()
	i64, err := buf.WriteTo(w)
	return int(i64), err
}

func Render(ctx *core.Context, n *HTMLNode) *core.ByteSlice {
	buf := bytes.NewBuffer(nil)
	n.Render(ctx, buf)

	return core.NewMutableByteSlice(buf.Bytes(), "")
}

func RenderToString(ctx *core.Context, n *HTMLNode) core.String {
	return core.String(Render(ctx, n).UnderlyingBytes())
}
