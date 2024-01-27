package s3_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
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
		return symbolic.ANY_STRING
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*ObjectInfo) PropertyNames() []string {
	return []string{"key"}
}

func (r *ObjectInfo) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("object-info")
}

func (r *ObjectInfo) WidestOfType() symbolic.Value {
	return &ObjectInfo{}
}
