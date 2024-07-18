package symbolic

import pprint "github.com/inoxlang/inox/internal/prettyprint"

type Opaque struct {
	_ int
}

func (o *Opaque) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return true
}

func (o *Opaque) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("opaque")
	return
}

func (o *Opaque) WidestOfType() Value {
	return OPAQUE
}
