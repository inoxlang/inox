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

	t.Run("ReadNBytesAsHex", func(t *testing.T) {
		outputs := map[string]struct{}{}

		for i := 0; i < RAND_TESTS_COUNT; i++ {
			output := DefaultRandSource.ReadNBytesAsHex(10)
			if _, ok := outputs[output]; ok {
				t.Logf("outputs should not repeat: %s", output)
				t.FailNow()
			}
			outputs[output] = struct{}{}
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

func TestRegexPatternRandom(t *testing.T) {

	t.Run("2-elem character class", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		r := NewRegexPattern("[ab]")

		for i := 0; i < RAND_TESTS_COUNT; i++ {
			s := r.Random(ctx)
			assert.True(t, s.Equal(ctx, Str("a"), nil, 0) || s.Equal(ctx, Str("b"), nil, 0))
		}
	})

	t.Run("3-elem character class", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		r := NewRegexPattern("[abc]")

		for i := 0; i < RAND_TESTS_COUNT; i++ {
			s := r.Random(ctx)
			assert.True(t,
				s.Equal(ctx, Str("a"), nil, 0) ||
					s.Equal(ctx, Str("b"), nil, 0) ||
					s.Equal(ctx, Str("c"), nil, 0),
			)
		}
	})

	t.Run("2-elem alternate", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		r := NewRegexPattern("(a1|b2)")

		for i := 0; i < RAND_TESTS_COUNT; i++ {
			s := r.Random(ctx)
			assert.True(t, s.Equal(ctx, Str("a1"), nil, 0) || s.Equal(ctx, Str("b2"), nil, 0))
		}
	})

	t.Run("3-elem altenate", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		r := NewRegexPattern("(a1|b2|c3)")

		for i := 0; i < RAND_TESTS_COUNT; i++ {
			s := r.Random(ctx)
			assert.True(t,
				s.Equal(ctx, Str("a1"), nil, 0) ||
					s.Equal(ctx, Str("b2"), nil, 0) ||
					s.Equal(ctx, Str("c3"), nil, 0),
			)
		}
	})
}
