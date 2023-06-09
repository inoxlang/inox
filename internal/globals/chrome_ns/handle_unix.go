//go:build unix

package chrome_ns

import (
	"time"

	"github.com/chromedp/chromedp"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/html_ns"
)

func newHandle(ctx *core.Context) *Handle {
	logger := *ctx.Logger()
	logger = logger.With().Str(core.SOURCE_LOG_FIELD_NAME, SRC_PATH).Logger()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
		chromedp.UserDataDir(string(core.CreateTempdir("chrome", ctx.GetFileSystem()))),
		//chromedp.Headless,
	)

	allocCtx, cancelAllocCtx := chromedp.NewExecAllocator(ctx, opts...)

	chromedpCtx, cancel := chromedp.NewContext(
		allocCtx,
		chromedp.WithErrorf(func(s string, i ...interface{}) {
			logger.Warn().Msgf(s, i...)
		}),
		chromedp.WithLogf(func(s string, i ...interface{}) {
			logger.Info().Msgf(s, i...)
		}),
		//most chatty
		chromedp.WithDebugf(func(s string, i ...interface{}) {
			logger.Trace().Msgf(s, i...)
		}),

		//???: the previous options are not working if they are created as several chromedp.WithBrowserXXX calls in chromedp.WithBrowserOption(...)

		//chromedp.WithDebugf(ctx.GetState().Logger.Printf),
	)

	handle := &Handle{
		allocCtx:       allocCtx,
		cancelAllocCtx: cancelAllocCtx,

		chromedpContext:       chromedpCtx,
		cancelChromedpContext: cancel,
	}

	return handle
}

func (h *Handle) doEmulateViewPort(ctx *core.Context) error {
	if err := h.do(ctx, chromedp.EmulateViewport(1920, 1080)); err != nil {
		return err
	}
	return nil
}

func (h *Handle) doNavigate(ctx *core.Context, u core.URL) error {
	action := chromedp.Navigate(string(u))
	return h.do(ctx, action)
}

func (h *Handle) doWaitVisible(ctx *core.Context, s core.Str) error {
	action := chromedp.WaitVisible(string(s))
	return h.do(ctx, action)
}
func (h *Handle) doClick(ctx *core.Context, s core.Str) error {
	action := chromedp.Click(string(s))
	return h.do(ctx, action)
}

func (h *Handle) doScreensotPage(ctx *core.Context) (*core.ByteSlice, error) {

	var b []byte

	action := chromedp.CaptureScreenshot(&b)
	if err := h.do(ctx, action); err != nil {
		return nil, err
	}

	return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
}

func (h *Handle) doScreenshot(ctx *core.Context, sel core.Str) (*core.ByteSlice, error) {
	var b []byte

	action := chromedp.Screenshot(sel, &b)
	if err := h.do(ctx, action); err != nil {
		return nil, err
	}

	return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
}

func (h *Handle) doGetHTMLNode(ctx *core.Context, sel core.Str) (*html_ns.HTMLNode, error) {
	var htmlS string

	action := chromedp.OuterHTML(sel, &htmlS)
	if err := h.do(ctx, action); err != nil {
		return nil, err
	}

	return html_ns.ParseSingleNodeHTML(htmlS)
}

func (h *Handle) do(ctx *core.Context, action chromedp.Action) error {
	done := make(chan struct{})

	go func() {
		select {
		case <-done:
		case <-time.After(DEFAULT_SINGLE_ACTION_TIMEOUT):
			h.close()
		}
	}()

	defer func() {
		done <- struct{}{}
	}()

	return chromedp.Run(h.chromedpContext,
		action,
	)
}
