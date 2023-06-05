package core

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

const RAND_TESTS_COUNT = 5

func TestRandomnessSource(t *testing.T) {

	t.Run("RandUint64Range", func(t *testing.T) {
		for i := 0; i < RAND_TESTS_COUNT; i++ {
			assert.NotPanics(t, func() {
				DefaultRandSource.RandUint64Range(0, math.MaxUint64)
			})
			assert.Zero(t, DefaultRandSource.RandUint64Range(0, 0))
			assert.Equal(t, uint64(1), DefaultRandSource.RandUint64Range(1, 1))
			assert.Equal(t, uint64(math.MaxUint64), DefaultRandSource.RandUint64Range(math.MaxUint64, math.MaxUint64))
		}
	})

}

func TestObjectPatternRandom(t *testing.T) {
	for i := 0; i < RAND_TESTS_COUNT; i++ {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		emptyObj := &ObjectPattern{
			entryPatterns: map[string]Pattern{},
		}

		assert.Equal(t, NewObject(), emptyObj.Random(ctx))

		singlePropObj := &ObjectPattern{
			entryPatterns: map[string]Pattern{
				"a": &ExactValuePattern{value: Int(1)},
			},
		}

		assert.Equal(t, NewObjectFromMap(ValMap{"a": Int(1)}, ctx), singlePropObj.Random(ctx))

		twoPropObj := &ObjectPattern{
			entryPatterns: map[string]Pattern{
				"a": &ExactValuePattern{value: Int(1)},
				"b": &ExactValuePattern{value: Int(2)},
			},
		}

		assert.Equal(t, NewObjectFromMap(ValMap{
			"a": Int(1),
			"b": Int(2),
		}, ctx), twoPropObj.Random(ctx))
	}
}
