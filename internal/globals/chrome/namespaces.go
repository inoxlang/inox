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
}

func NewChromeNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"Handle": core.ValOf(NewHandle),
	})
}
