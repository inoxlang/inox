package chrome_ns

import (
	"errors"

	"github.com/inoxlang/inox/internal/core/symbolic"
	html_ns "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HANDLE_PROPNAMES = []string{"nav", "wait_visible", "click", "screenshot_page", "screenshot", "html_node", "close"}
)

type Handle struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Handle) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Handle)
	return ok
}

func (r *Handle) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("browser-handle")
}

func (r *Handle) WidestOfType() symbolic.Value {
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

func (h *Handle) HtmlNode(ctx *symbolic.Context, sel *symbolic.String) (*html_ns.HTMLNode, *symbolic.Error) {
	return html_ns.NewHTMLNode(), nil
}

func (h *Handle) Close(ctx *symbolic.Context) {
}

func (h *Handle) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, h)
}

func (h *Handle) WithExistingPropReplaced(state *symbolic.State, name string, value symbolic.Value) (symbolic.IProps, error) {
	return nil, errors.New(symbolic.FmtCannotAssignPropertyOf(h))
}

func (h *Handle) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "nav":
		return symbolic.WrapGoMethod(h.Nav), true
	case "wait_visible":
		return symbolic.WrapGoMethod(h.WaitVisible), true
	case "click":
		return symbolic.WrapGoMethod(h.Click), true
	case "screenshot_page":
		return symbolic.WrapGoMethod(h.ScreenshotPage), true
	case "screenshot":
		return symbolic.WrapGoMethod(h.Screenshot), true
	case "html_node":
		return symbolic.WrapGoMethod(h.HtmlNode), true
	case "close":
		return symbolic.WrapGoMethod(h.Close), true
	}
	return nil, false
}

func (h *Handle) PropertyNames() []string {
	return HANDLE_PROPNAMES
}

func (h *Handle) IsMutable() bool {
	return true
}
