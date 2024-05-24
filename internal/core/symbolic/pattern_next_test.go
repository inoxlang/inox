package symbolic

import (
	"testing"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestExactValuePatternWithExactValuePatternsRemoved(t *testing.T) {
	t.Run("concretizable value", func(t *testing.T) {
		p := utils.Must(NewExactValuePattern(INT_1))

		new, err := p.WithExactValuePatternsRemoved()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, getStatic(INT_1), new)
	})

	t.Run("non-concretizable value", func(t *testing.T) {
		p := utils.Must(NewExactValuePattern(ANY_INT))

		new, err := p.WithExactValuePatternsRemoved()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, getStatic(ANY_INT), new)
	})
}

func TestExactStringPatternWithExactValuePatternsRemoved(t *testing.T) {
	t.Run("concretizable value", func(t *testing.T) {
		p := NewExactStringPatternWithConcreteValue(NewString("a"))

		new, err := p.WithExactValuePatternsRemoved()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, getStatic(ANY_STRING), new)
	})

	t.Run("non-concretizable value", func(t *testing.T) {
		p := NewExactStringPatternWithRunTimeValue(NewRunTimeValue(ANY_STRING).asStrLike())

		new, err := p.WithExactValuePatternsRemoved()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, getStatic(ANY_STRING), new)
	})
}

func TesObjectPatternithExactValuePatternsRemoved(t *testing.T) {
	//TODO: add tests

	t.Run("property with concretizable value", func(t *testing.T) {
		p := NewInexactObjectPattern(map[string]Pattern{
			"a": NewExactStringPatternWithConcreteValue(NewString("a")),
		}, nil)

		new, err := p.WithExactValuePatternsRemoved()
		if !assert.NoError(t, err) {
			return
		}

		expected := NewInexactObjectPattern(map[string]Pattern{
			"a": getStatic(ANY_STRING),
		}, nil)

		assert.Equal(t, expected, new)
	})

	t.Run("property with a non-concretizable value", func(t *testing.T) {
		p := NewInexactObjectPattern(map[string]Pattern{
			"a": NewExactStringPatternWithRunTimeValue(NewRunTimeValue(ANY_STRING).asStrLike()),
		}, nil)

		new, err := p.WithExactValuePatternsRemoved()
		if !assert.NoError(t, err) {
			return
		}

		expected := NewInexactObjectPattern(map[string]Pattern{
			"a": getStatic(ANY_STRING),
		}, nil)

		assert.Equal(t, expected, new)
	})
}
