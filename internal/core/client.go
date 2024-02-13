package core

var (
	//CreateHttpClient is initialized by the internal/globals/http_ns package.
	CreateHttpClient func(insecure, saveCookies bool) (ProtocolClient, error) = func(insecure, saveCookies bool) (ProtocolClient, error) {
		panic(ErrNotImplemented)
	}
)

// A ProtocolClient represents a client for one or more protocols such as HTTP, HTTPS.
type ProtocolClient interface {
	Value
	Schemes() []Scheme
}
