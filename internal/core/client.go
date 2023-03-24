package internal

type ProtocolClient interface {
	Value
	Schemes() []Scheme
}
