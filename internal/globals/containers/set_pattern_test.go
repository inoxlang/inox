package containers

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestSetPattern(t *testing.T) {
	//
	patt, err := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, REPR_UNIQUENESS_IDENT})

	if assert.NoError(t, err) {
		uniqueness := UniquenessConstraint{
			Type: UniqueRepr,
		}

		expectedPattern := NewSetPattern(SetConfig{
			Element:    core.INT_PATTERN,
			Uniqueness: uniqueness,
		}, core.CallBasedPatternReprMixin{
			Callee: SET_PATTERN,
			Params: []core.Serializable{core.INT_PATTERN, uniqueness.ToValue()},
		})

		assert.Equal(t, expectedPattern, patt)
	}

	objectPattern := core.NewInexactObjectPattern(map[string]core.Pattern{
		"a": core.INT_PATTERN,
	})

	//
	patt, err = SET_PATTERN.Call([]core.Serializable{objectPattern, URL_UNIQUENESS_IDENT})

	if assert.NoError(t, err) {
		uniqueness := UniquenessConstraint{
			Type: UniqueURL,
		}

		expectedPattern := NewSetPattern(SetConfig{
			Element:    objectPattern,
			Uniqueness: uniqueness,
		}, core.CallBasedPatternReprMixin{
			Callee: SET_PATTERN,
			Params: []core.Serializable{objectPattern, uniqueness.ToValue()},
		})

		assert.Equal(t, expectedPattern, patt)
	}

	//
	patt, err = SET_PATTERN.Call([]core.Serializable{objectPattern, core.PropertyName("a")})

	if assert.NoError(t, err) {
		uniqueness := UniquenessConstraint{
			Type:         UniquePropertyValue,
			PropertyName: core.PropertyName("a"),
		}

		expectedPattern := NewSetPattern(SetConfig{
			Element:    objectPattern,
			Uniqueness: uniqueness,
		}, core.CallBasedPatternReprMixin{
			Callee: SET_PATTERN,
			Params: []core.Serializable{objectPattern, uniqueness.ToValue()},
		})

		assert.Equal(t, expectedPattern, patt)
	}
}
