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

func (r *ObjectInfo) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%object-info")))
	return
}

func (r *ObjectInfo) WidestOfType() symbolic.Value {
	return &ObjectInfo{}
}
