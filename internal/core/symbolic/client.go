package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_PROTOCOL_CLIENT = &AnyProtocolClient{}
)

// A ProtocolClient represents a symbolic ProtocolClient;
type ProtocolClient interface {
	SymbolicValue
	Schemes() []string
}

// An AnyProtocolClient represents a symbolic Iterable we do not know the concrete type.
type AnyProtocolClient struct {
	_ int
}

func (r *AnyProtocolClient) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(ProtocolClient)

	return ok
}

func (*AnyProtocolClient) Schemes() []string {
	return nil
}

func (r *AnyProtocolClient) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%protocol-client")))
}

func (r *AnyProtocolClient) WidestOfType() SymbolicValue {
	return ANY_PROTOCOL_CLIENT
}
