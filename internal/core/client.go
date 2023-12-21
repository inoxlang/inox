package core

// A ProtocolClient represents a client for one or more protocols such as HTTP, HTTPS.
type ProtocolClient interface {
	Value
	Schemes() []Scheme
}
