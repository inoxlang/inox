package core

type ProtocolClient interface {
	Value
	Schemes() []Scheme
}
