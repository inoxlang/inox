package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

// A RandomnessSource represents a symbolic RandomnessSource.
type RandomnessSource struct {
	UnassignablePropsMixin
	_ int
}

func (r *RandomnessSource) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *RandomnessSource:
		return true
	default:
		return false
	}
}

func (r *RandomnessSource) Start(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Commit(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Rollback(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Prop(name string) SymbolicValue {
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (r *RandomnessSource) PropertyNames() []string {
	return nil
}

func (r *RandomnessSource) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (r *RandomnessSource) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%random-source")))
	return
}

func (r *RandomnessSource) WidestOfType() SymbolicValue {
	return &RandomnessSource{}
}
