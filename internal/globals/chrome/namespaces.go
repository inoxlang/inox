package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	chrome_symbolic "github.com/inoxlang/inox/internal/globals/chrome/symbolic"
	help "github.com/inoxlang/inox/internal/globals/help"
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

func NewChromeNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"Handle": core.ValOf(NewHandle),
	})
}
