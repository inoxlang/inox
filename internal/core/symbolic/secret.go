package symbolic

import (
	"fmt"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_SECRET_PATTERN = NewSecretPattern(ANY_STR_PATTERN)
	ANY_SECRET         = utils.Must(NewSecret(ANY_STR_LIKE, ANY_SECRET_PATTERN))

	_ = []Value{(*Secret)(nil)}
	_ = []Pattern{(*SecretPattern)(nil)}
)

// A Secret represents a symbolic Secret.
type Secret struct {
	pattern *SecretPattern
	value   Value
	SerializableMixin
}

func NewSecret(value Value, pattern *SecretPattern) (*Secret, error) {
	if !isAnyStringLike(value) && value.IsMutable() {
		return nil, fmt.Errorf("failed to create secret: value should be immutable: %T", value)
	}
	return &Secret{value: value}, nil
}

func (r *Secret) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Secret)
	return ok
}

func (r *Secret) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("secret")
}

func (r *Secret) WidestOfType() Value {
	return &Secret{value: ANY}
}

type SecretPattern struct {
	stringPattern StringPattern

	NotCallablePatternMixin
	SerializableMixin
}

// NewSecretPattern creates a SecretPattern from the given string pattern
func NewSecretPattern(patt StringPattern) *SecretPattern {
	return &SecretPattern{
		stringPattern: patt,
	}
}

func (pattern *SecretPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*SecretPattern)
	if !ok {
		return false
	}
	return pattern.stringPattern.Test(otherPattern.stringPattern, state)
}

func (pattern *SecretPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	secret, ok := v.(*Secret)
	if !ok {
		return false
	}

	return secret.pattern == pattern
}

func (pattern *SecretPattern) SymbolicValue() Value {
	return utils.Must(NewSecret(pattern.stringPattern.SymbolicValue(), pattern))
}

func (pattern *SecretPattern) HasUnderlyingPattern() bool {
	return pattern.stringPattern.HasUnderlyingPattern()
}

func (pattern *SecretPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("secret-pattern(")
	pattern.stringPattern.PrettyPrint(w.ZeroDepthIndent(), config)
	w.WriteString(")")
}

func (pattern *SecretPattern) WidestOfType() Value {
	return nil
}

func (pattern *SecretPattern) IteratorElementKey() Value {
	return ANY
}

func (pattern *SecretPattern) IteratorElementValue() Value {
	return ANY
}

func (pattern *SecretPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}
