package html_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	_html_symbolic "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
)

func (n *HTMLNode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return _html_symbolic.NewHTMLNode(), nil
}
