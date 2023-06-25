package dom_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type View struct {
	model symbolic.SymbolicValue
}

func NewDomView(model symbolic.SymbolicValue) *View {
	return &View{model: model}
}

func (n *View) Test(v symbolic.SymbolicValue) bool {
	otherView, ok := v.(*View)
	if !ok {
		return false
	}

	return n.model.Test(otherView.model)
}

func (r *View) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *View) IsWidenable() bool {
	return false
}

func (r *View) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%dom-view")))
	return
}

func (r *View) WidestOfType() symbolic.SymbolicValue {
	return &View{}
}
