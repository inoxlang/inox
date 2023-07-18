package s3_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type ObjectInfo struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func (r *ObjectInfo) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*ObjectInfo)
	return ok
}

func (resp *ObjectInfo) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *ObjectInfo) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "key":
		return &symbolic.String{}
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*ObjectInfo) PropertyNames() []string {
	return []string{"key"}
}

func (r *ObjectInfo) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *ObjectInfo) IsWidenable() bool {
	return false
}

func (r *ObjectInfo) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%object-info")))
	return
}

func (r *ObjectInfo) WidestOfType() symbolic.SymbolicValue {
	return &ObjectInfo{}
}
