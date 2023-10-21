package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
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

func (r *ContentSecurityPolicy) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("content-security-policy")
}

func (r *ContentSecurityPolicy) WidestOfType() symbolic.Value {
	return &ContentSecurityPolicy{}
}
