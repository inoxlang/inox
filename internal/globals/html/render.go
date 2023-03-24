package internal

import (
	"bytes"
	"io"

	core "github.com/inox-project/inox/internal/core"
	"golang.org/x/net/html"
)

func (n *HTMLNode) Render(ctx *core.Context, w io.Writer, config core.RenderingInput) (int, error) {
	if !n.IsRecursivelyRenderable(ctx, config) {
		return 0, core.ErrNotRenderable
	}

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

func Render(ctx *core.Context, v core.Value) *core.ByteSlice {
	buf := bytes.NewBuffer(nil)
	_, err := v.(core.Renderable).Render(ctx, buf, core.RenderingInput{Mime: core.HTML_CTYPE})
	if err != nil {
		panic(err)
	}
	return &core.ByteSlice{Bytes: buf.Bytes(), IsDataMutable: true}
}

func RenderToString(ctx *core.Context, v core.Value) core.Str {
	return core.Str(Render(ctx, v).Bytes)
}
