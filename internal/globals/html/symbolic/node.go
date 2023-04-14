package internal

import (
	"reflect"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

type HTMLNode struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func NewHTMLNode() *HTMLNode {
	return &HTMLNode{}
}

func (n *HTMLNode) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HTMLNode)
	if !ok {
		return false
	}

	return true
}

func (n *HTMLNode) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	clone := new(HTMLNode)
	clones[reflect.ValueOf(n).Pointer()] = clone

	return clone
}

func (n *HTMLNode) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "firstChild":
		return NewHTMLNode()
	case "data":
		return &symbolic.String{}
	default:
		method, ok := n.GetGoMethod(name)
		if !ok {
			panic(symbolic.FormatErrPropertyDoesNotExist(name, n))
		}
		return method
	}
}

func (n *HTMLNode) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (n *HTMLNode) PropertyNames() []string {
	return []string{"firstChild", "data"}
}

func (r *HTMLNode) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *HTMLNode) IsWidenable() bool {
	return false
}

func (r *HTMLNode) String() string {
	return "%html-node"
}

func (r *HTMLNode) WidestOfType() symbolic.SymbolicValue {
	return &HTMLNode{}
}
