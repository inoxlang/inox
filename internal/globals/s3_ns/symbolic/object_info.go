package s3_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

type ObjectInfo struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func (r *ObjectInfo) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*ObjectInfo)
	return ok
}

func (resp *ObjectInfo) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *ObjectInfo) Prop(name string) symbolic.Value {
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

func (r *ObjectInfo) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("object-info")
}

func (r *ObjectInfo) WidestOfType() symbolic.Value {
	return &ObjectInfo{}
}
