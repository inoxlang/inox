package core

var (
	//CreateHttpClient is initialized by the internal/globals/http_ns package.
	CreateHttpClient func(insecure, saveCookies bool) (ProtocolClient, error) = func(insecure, saveCookies bool) (ProtocolClient, error) {
		panic(ErrNotImplemented)
	}
)

// A ProtocolClient represents a client for one or more protocols (e.g HTTP + HTTPS).
type ProtocolClient interface {
	Value

	Schemes() []Scheme

	//IsStateful should return true if the client has an internal state (e.g. cookies for HTTP clients).
	//IsStateful() can return true even for stateless protocols.
	IsStateful() bool

	//MayPurposefullySkipAuthentication should return true if the client may skip the authentication of the server,
	//even if it is supported. An example would be skipping certificate checks for HTTPS clients.
	MayPurposefullySkipAuthentication() bool
}
