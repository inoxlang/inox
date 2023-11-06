package chrome_ns

import (
	"context"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/html_ns"
)

const (
	DEFAULT_SINGLE_ACTION_TIMEOUT = 15 * time.Second

	SRC_PATH = "/chrome"
)

var (
	HANDLE_PROPNAMES = []string{"nav", "wait_visible", "click", "screenshot_page", "html_node", "close"}
)

type Handle struct {
	allocCtx       context.Context
	cancelAllocCtx context.CancelFunc

	chromedpContext       context.Context
	cancelChromedpContext context.CancelFunc
}

func NewHandle(ctx *core.Context) (*Handle, error) {
	handle, err := newHandle(ctx)
	if err != nil {
		return nil, err
	}
	if err := handle.doEmulateViewPort(ctx); err != nil {
		return nil, err
	}

	return handle, nil
}

func (h *Handle) Nav(ctx *core.Context, u core.URL) error {
	return h.doNavigate(ctx, u)
}

func (h *Handle) WaitVisible(ctx *core.Context, s core.Str) error {
	return h.doWaitVisible(ctx, s)
}

func (h *Handle) Click(ctx *core.Context, s core.Str) error {
	return h.doClick(ctx, s)
}

func (h *Handle) ScreenshotPage(ctx *core.Context) (*core.ByteSlice, error) {
	return h.doScreensotPage(ctx)
}

func (h *Handle) Screenshot(ctx *core.Context, sel core.Str) (*core.ByteSlice, error) {
	return h.doScreenshot(ctx, sel)
}

func (h *Handle) HtmlNode(ctx *core.Context, sel core.Str) (*html_ns.HTMLNode, error) {
	return h.doGetHTMLNode(ctx, sel)
}

func (h *Handle) Close(ctx *core.Context) {
	h.close()
}

func (h *Handle) close() {
	h.cancelChromedpContext()
	h.cancelAllocCtx()
}

func (h *Handle) Prop(ctx *core.Context, name string) core.Value {
	method, ok := h.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, h))
	}
	return method
}

func (*Handle) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (h *Handle) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "nav":
		return core.WrapGoMethod(h.Nav), true
	case "wait_visible":
		return core.WrapGoMethod(h.WaitVisible), true
	case "click":
		return core.WrapGoMethod(h.Click), true
	case "screenshot":
		return core.WrapGoMethod(h.Screenshot), true
	case "screenshot_page":
		return core.WrapGoMethod(h.ScreenshotPage), true
	case "html_node":
		return core.WrapGoMethod(h.HtmlNode), true
	case "close":
		return core.WrapGoMethod(h.Close), true
	}
	return nil, false
}

func (h *Handle) PropertyNames(ctx *core.Context) []string {
	return HANDLE_PROPNAMES
}
