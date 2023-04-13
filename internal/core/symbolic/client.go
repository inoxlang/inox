package internal

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

func (r *AnyProtocolClient) String() string {
	return "%protocol-client"
}

func (r *AnyProtocolClient) WidestOfType() SymbolicValue {
	return &AnyProtocolClient{}
}
