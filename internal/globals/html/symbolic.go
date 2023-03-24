package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	_html_symbolic "github.com/inox-project/inox/internal/globals/html/symbolic"
)

func (n HTMLNode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return _html_symbolic.NewHTMLNode(), nil
}
