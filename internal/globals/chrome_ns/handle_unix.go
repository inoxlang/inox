//go:build unix

package chrome_ns

import (
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/oklog/ulid/v2"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	HANDLE_ID_HEADER = "X-Browser-Handle"
)

var (
	ErrPageFailedToLoad = errors.New("page failed to load")
)

func newHandle(ctx *core.Context) (*Handle, error) {
	if BROWSER_BINPATH == "" {
		return nil, errors.New("BROWSER_BINPATH is not set")
	}

	id := ulid.Make().String()

	//create a temporary directory to store user data
	tempDir := fs_ns.CreateDirInProcessTempDir("chrome")

	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		handleIdToContextLock.Lock()
		delete(handleIdToContext, id)
		handleIdToContextLock.Unlock()

		return fs_ns.DeleteDirInProcessTempDir(tempDir)
	})

	//start the shared proxy if necessary
	StartSharedProxy(ctx)

	logger := *ctx.Logger()
	logger = logger.With().Str(core.SOURCE_LOG_FIELD_NAME, SRC_PATH).Logger()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		//execution
		chromedp.UserDataDir(string(tempDir)),
		chromedp.ExecPath(BROWSER_BINPATH),

		//headless
		chromedp.DisableGPU,

		//network
		chromedp.ProxyServer(BROWSER_PROXY_ADDR),
		chromedp.IgnoreCertErrors,
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
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
		id:             id,
		allocCtx:       allocCtx,
		cancelAllocCtx: cancelAllocCtx,

		chromedpContext:       chromedpCtx,
		cancelChromedpContext: cancel,
	}

	//register the browser's context
	handleIdToContextLock.Lock()
	handleIdToContext[handle.id] = ctx
	handleIdToContextLock.Unlock()

	return handle, nil
}

func (h *Handle) doEmulateViewPort(ctx *core.Context) error {
	if err := h.do(ctx, chromedp.EmulateViewport(1920, 1080)); err != nil {
		return err
	}
	return nil
}

func (h *Handle) doNavigate(ctx *core.Context, u core.URL) error {
	setHeaders := network.SetExtraHTTPHeaders(network.Headers{
		HANDLE_ID_HEADER: h.id,
	})

	err := h.do(ctx, setHeaders)
	if err != nil {
		return err
	}

	netResp, err := h.doResponse(ctx, chromedp.Navigate(string(u)))

	if netResp != nil && netResp.Status >= 400 {
		return fmt.Errorf("%w: status %q (%d)", ErrPageFailedToLoad, netResp.StatusText, netResp.Status)
	}

	if err != nil {
		return fmt.Errorf("%w: %w", ErrPageFailedToLoad, err)
	}
	return nil
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
		defer utils.Recover()

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

func (h *Handle) doResponse(ctx *core.Context, action chromedp.Action) (*network.Response, error) {
	done := make(chan struct{})

	go func() {
		defer utils.Recover()

		select {
		case <-done:
		case <-time.After(DEFAULT_SINGLE_ACTION_TIMEOUT):
			h.close()
		}
	}()

	defer func() {
		done <- struct{}{}
	}()

	return chromedp.RunResponse(h.chromedpContext,
		action,
	)
}
