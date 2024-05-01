package html_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	_html_symbolic "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
)

func (n *HTMLNode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	//TODO
	return _html_symbolic.ANY_HTML_NODE, nil
}
