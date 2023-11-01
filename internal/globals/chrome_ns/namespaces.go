package chrome_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	chrome_symbolic "github.com/inoxlang/inox/internal/globals/chrome_ns/symbolic"
	"github.com/inoxlang/inox/internal/help"
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		NewHandle, func(ctx *symbolic.Context) (*chrome_symbolic.Handle, *symbolic.Error) {
			return &chrome_symbolic.Handle{}, nil
		},
	})

	help.RegisterHelpValue(NewHandle, "chrome.Handle")

	//create an empty handle in order to get the address of each method.
	emptyHandle := Handle{}

	help.RegisterHelpValues(map[string]any{
		"chrome.Handle/nav":             emptyHandle.Nav,
		"chrome.Handle/wait_visible":    emptyHandle.WaitVisible,
		"chrome.Handle/click":           emptyHandle.Click,
		"chrome.Handle/screenshot":      emptyHandle.Screenshot,
		"chrome.Handle/screenshot_page": emptyHandle.ScreenshotPage,
		"chrome.Handle/html_node":       emptyHandle.HtmlNode,
		"chrome.Handle/close":           emptyHandle.Close,
	})
}

func NewChromeNamespace() *core.Namespace {
	return core.NewNamespace("chrome", map[string]core.Value{
		"Handle": core.ValOf(NewHandle),
	})
}
