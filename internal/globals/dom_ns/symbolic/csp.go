package dom_ns

import (
	"bufio"
	"reflect"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type ContentSecurityPolicy struct {
	_ int
}

func NewCSP() *ContentSecurityPolicy {
	return &ContentSecurityPolicy{}
}

func (n *ContentSecurityPolicy) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*ContentSecurityPolicy)
	if !ok {
		return false
	}
	return true
}

func (n *ContentSecurityPolicy) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	clone := new(ContentSecurityPolicy)
	clones[reflect.ValueOf(n).Pointer()] = clone

	return clone
}

func (r *ContentSecurityPolicy) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *ContentSecurityPolicy) IsWidenable() bool {
	return false
}

func (r *ContentSecurityPolicy) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%content-security-policy")))
	return
}

func (r *ContentSecurityPolicy) WidestOfType() symbolic.SymbolicValue {
	return &ContentSecurityPolicy{}
}
