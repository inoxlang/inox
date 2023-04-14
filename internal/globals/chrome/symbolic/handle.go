package internal

import (
	"errors"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

type Handle struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Handle) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Handle)
	return ok
}

func (r Handle) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Handle{}
}

func (r *Handle) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Handle) IsWidenable() bool {
	return false
}

func (r *Handle) String() string {
	return "%browser-handle"
}

func (r *Handle) WidestOfType() symbolic.SymbolicValue {
	return &Handle{}
}

func (h *Handle) Nav(ctx *symbolic.Context, u *symbolic.URL) *symbolic.Error {
	return nil
}

func (h *Handle) WaitVisible(ctx *symbolic.Context, s *symbolic.String) *symbolic.Error {
	return nil
}

func (h *Handle) Click(ctx *symbolic.Context, s *symbolic.String) *symbolic.Error {
	return nil
}

func (h *Handle) ScreenshotPage(ctx *symbolic.Context) (*symbolic.ByteSlice, *symbolic.Error) {
	return &symbolic.ByteSlice{}, nil
}

func (h *Handle) Screenshot(ctx *symbolic.Context, sel *symbolic.String) (*symbolic.ByteSlice, *symbolic.Error) {
	return &symbolic.ByteSlice{}, nil
}

func (h *Handle) Close(ctx *symbolic.Context) {
}

func (h *Handle) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, h)
}

func (h *Handle) WithExistingPropReplaced(name string, value symbolic.SymbolicValue) (symbolic.IProps, error) {
	return nil, errors.New(symbolic.FmtCannotAssignPropertyOf(h))
}

func (h *Handle) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "nav":
		return symbolic.WrapGoMethod(h.Nav), true
	case "waitVisible":
		return symbolic.WrapGoMethod(h.WaitVisible), true
	case "click":
		return symbolic.WrapGoMethod(h.Click), true
	case "screenshotPage":
		return symbolic.WrapGoMethod(h.ScreenshotPage), true
	case "screenshot":
		return symbolic.WrapGoMethod(h.Screenshot), true
	case "close":
		return symbolic.WrapGoMethod(h.Close), true
	}
	return nil, false
}

func (h *Handle) PropertyNames() []string {
	return []string{"nav", "waitVisible", "click", "screenshotPage", "screenshot", "close"}
}

func (h *Handle) IsMutable() bool {
	return true
}
