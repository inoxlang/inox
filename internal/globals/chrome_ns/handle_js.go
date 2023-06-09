//go:build js

package chrome_ns

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
	html_ns "github.com/inoxlang/inox/internal/globals/html_ns"
)

var (
	ErrChromeHandleNotAvailable = errors.New("chrome handle not available")
)

func newHandle(ctx *core.Context) *Handle {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doEmulateViewPort(ctx *core.Context) error {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doNavigate(ctx *core.Context, u core.URL) error {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doWaitVisible(ctx *core.Context, s core.Str) error {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doClick(ctx *core.Context, s core.Str) error {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doScreensotPage(ctx *core.Context) (*core.ByteSlice, error) {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doScreenshot(ctx *core.Context, sel core.Str) (*core.ByteSlice, error) {
	panic(ErrChromeHandleNotAvailable)
}

func (h *Handle) doGetHTMLNode(ctx *core.Context, sel core.Str) (*html_ns.HTMLNode, error) {
	panic(ErrChromeHandleNotAvailable)
}
