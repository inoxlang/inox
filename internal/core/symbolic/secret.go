package symbolic

import (
	"bufio"
	"fmt"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_SECRET_PATTERN = NewSecretPattern(ANY_STR_PATTERN)
	ANY_SECRET         = utils.Must(NewSecret(ANY_STR_LIKE, ANY_SECRET_PATTERN))

	_ = []SymbolicValue{(*Secret)(nil)}
	_ = []Pattern{(*SecretPattern)(nil)}
)

// A Secret represents a symbolic Secret.
type Secret struct {
	pattern *SecretPattern
	value   SymbolicValue
	SerializableMixin
}

func NewSecret(value SymbolicValue, pattern *SecretPattern) (*Secret, error) {
	if !isAnyStringLike(value) && value.IsMutable() {
		return nil, fmt.Errorf("failed to create secret: value should be immutable: %T", value)
	}
	return &Secret{value: value}, nil
}

func (r *Secret) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Secret)
	return ok
}

func (r *Secret) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%secret")))
}

func (r *Secret) WidestOfType() SymbolicValue {
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

func (pattern *SecretPattern) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*SecretPattern)
	if !ok {
		return false
	}
	return pattern.stringPattern.Test(otherPattern.stringPattern, state)
}

func (pattern *SecretPattern) TestValue(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	secret, ok := v.(*Secret)
	if !ok {
		return false
	}

	return secret.pattern == pattern
}

func (pattern *SecretPattern) SymbolicValue() SymbolicValue {
	return utils.Must(NewSecret(pattern.stringPattern.SymbolicValue(), pattern))
}

func (pattern *SecretPattern) HasUnderlyingPattern() bool {
	return pattern.stringPattern.HasUnderlyingPattern()
}

func (pattern *SecretPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%secret-pattern(")))
	pattern.stringPattern.PrettyPrint(w, config, 0, 0)
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (pattern *SecretPattern) WidestOfType() SymbolicValue {
	return nil
}

func (pattern *SecretPattern) IteratorElementKey() SymbolicValue {
	return ANY
}

func (pattern *SecretPattern) IteratorElementValue() SymbolicValue {
	return ANY
}

func (pattern *SecretPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}
