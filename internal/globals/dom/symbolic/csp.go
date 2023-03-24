package internal

import (
	"reflect"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
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

func (r *ContentSecurityPolicy) String() string {
	return "content-security-policy"
}

func (r *ContentSecurityPolicy) WidestOfType() symbolic.SymbolicValue {
	return &ContentSecurityPolicy{}
}
