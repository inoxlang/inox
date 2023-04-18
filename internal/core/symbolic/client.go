package internal

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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

func (r *AnyProtocolClient) Test(v SymbolicValue) bool {
	_, ok := v.(ProtocolClient)

	return ok
}

func (*AnyProtocolClient) Schemes() []string {
	return nil
}

func (r *AnyProtocolClient) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyProtocolClient) IsWidenable() bool {
	return false
}

func (r *AnyProtocolClient) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%protocol-client")))
	return
}

func (r *AnyProtocolClient) WidestOfType() SymbolicValue {
	return &AnyProtocolClient{}
}
