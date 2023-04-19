package internal

import "fmt"

var (
	_ = []Value{(*Secret)(nil)}

	_ = []Pattern{(*SecretPattern)(nil)}
)

// A Secret represents a string such as a password or an API-Key, a secret always return false when it is compared for equality.
type Secret struct {
	NoReprMixin
	NotClonableMixin

	value   StringLike
	pattern *SecretPattern
}

func (s *Secret) Value() StringLike {
	return s.value
}

func (s *Secret) String() string {
	return "secret(...)"
}

func (s *Secret) Format(f fmt.State, verb rune) {
	f.Write([]byte("secret(...)"))
}

type SecretPattern struct {
	NotCallablePatternMixin
	NoReprMixin
	NotClonableMixin

	stringPattern StringPattern
}

func NewSecretPattern(stringPattern StringPattern) *SecretPattern {
	return &SecretPattern{stringPattern: stringPattern}
}

// Test returns true if the pattern of the secret is p, the content of the secret is not verified.
func (p *SecretPattern) Test(ctx *Context, v Value) bool {
	secret, ok := v.(*Secret)
	if !ok {
		return false
	}

	return secret.pattern == p
}

func (pattern *SecretPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (pattern *SecretPattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplemented)
}

func (pattern *SecretPattern) NewSecret(ctx *Context, s string) (*Secret, error) {
	val, err := pattern.stringPattern.Parse(ctx, s)
	if err != nil {
		return nil, err
	}
	return &Secret{value: val.(StringLike), pattern: pattern}, nil
}
