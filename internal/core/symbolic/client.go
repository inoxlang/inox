package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_PROTOCOL_CLIENT = &AnyProtocolClient{}
)

// A ProtocolClient represents a symbolic ProtocolClient;
type ProtocolClient interface {
	Value
	Schemes() []string
}

// An AnyProtocolClient represents a symbolic Iterable we do not know the concrete type.
type AnyProtocolClient struct {
	_ int
}

func (r *AnyProtocolClient) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(ProtocolClient)

	return ok
}

func (*AnyProtocolClient) Schemes() []string {
	return nil
}

func (r *AnyProtocolClient) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("protocol-client")
}

func (r *AnyProtocolClient) WidestOfType() Value {
	return ANY_PROTOCOL_CLIENT
}
