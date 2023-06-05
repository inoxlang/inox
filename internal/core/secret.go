package core

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrSecretIsNotPEMEncoded = errors.New("secret is not PEM encoded")
	_                        = []Value{(*Secret)(nil)}
	_                        = []Pattern{(*SecretPattern)(nil)}
)

// A Secret represents a string such as a password, an API-Key or a PEM encoded key; a secret always return false when it is compared for equality.
type Secret struct {
	NoReprMixin
	NotClonableMixin

	value    StringLike
	pemBlock *pem.Block //not nil if value is PEM encoded

	pattern *SecretPattern
}

func (s *Secret) StringValue() StringLike {
	return s.value
}

func (s *Secret) String() string {
	return "secret(...)"
}

func (s *Secret) Format(f fmt.State, verb rune) {
	f.Write([]byte("secret(...)"))
}

func (s *Secret) DecodedPEM() (*pem.Block, error) {
	if s.pemBlock != nil {
		return s.pemBlock, nil
	}
	return nil, ErrSecretIsNotPEMEncoded
}

func (s *Secret) AssertIsPattern(secret *SecretPattern) {
	if s.pattern != secret {
		panic(fmt.Errorf("internal assertion failed, secret is not of the expected pattern"))
	}
}

type SecretPattern struct {
	NotCallablePatternMixin
	NoReprMixin
	NotClonableMixin

	stringPattern StringPattern
	pemEncoded    bool
}

func NewSecretPattern(stringPattern StringPattern, pem bool) *SecretPattern {
	return &SecretPattern{stringPattern: stringPattern, pemEncoded: pem}
}

func NewPEMRegexPattern(typeRegex string) StringPattern {
	return NewRegexPattern(
		"-----BEGIN " + typeRegex + "-----\\r?\\n" +
			"[a-zA-Z0-9+/\\n\\r]+={0,2}\\r?\\n" +
			"-----END " + typeRegex + "-----(\\r?\\n)?",
	)
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
	var block *pem.Block
	if pattern.pemEncoded {
		_block, rest := pem.Decode(utils.StringAsBytes(s))
		if len(bytes.TrimSpace(rest)) != 0 {
			return nil, errors.New("PEM encoded secret is followed by non space charaters")
		}
		block = _block
	}
	return &Secret{
		value:    val.(StringLike),
		pattern:  pattern,
		pemBlock: block,
	}, nil
}
