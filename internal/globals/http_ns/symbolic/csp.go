package http_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_CSP = NewCSP()
)

type ContentSecurityPolicy struct {
	_ int
	symbolic.SerializableMixin
}

func NewCSP() *ContentSecurityPolicy {
	return &ContentSecurityPolicy{}
}

func (n *ContentSecurityPolicy) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*ContentSecurityPolicy)
	if !ok {
		return false
	}
	return true
}

func (r *ContentSecurityPolicy) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%content-security-policy")))
	return
}

func (r *ContentSecurityPolicy) WidestOfType() symbolic.Value {
	return &ContentSecurityPolicy{}
}
