package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExactValuePattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {
		patt := &ExactValuePattern{value: Str("ab")}
		assert.Equal(t, IntRange{
			Start:        2,
			End:          2,
			inclusiveEnd: true,
			Step:         1,
		}, patt.LengthRange())
	})

}

func TestUnionPattern(t *testing.T) {

}

func TestObjectPattern(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	noProps := &ObjectPattern{entryPatterns: map[string]Pattern{}, inexact: false}
	inexactNoProps := &ObjectPattern{entryPatterns: map[string]Pattern{}, inexact: true}
	singleProp := &ObjectPattern{entryPatterns: map[string]Pattern{"a": INT_PATTERN}, inexact: false}
	inexactSingleProp := &ObjectPattern{entryPatterns: map[string]Pattern{"a": INT_PATTERN}, inexact: true}

	assert.True(t, noProps.Test(ctx, objFrom(ValMap{})))
	assert.False(t, noProps.Test(ctx, objFrom(ValMap{"a": Int(1)})))

	assert.True(t, inexactNoProps.Test(ctx, objFrom(ValMap{})))
	assert.True(t, inexactNoProps.Test(ctx, objFrom(ValMap{"a": Int(1)})))
	assert.True(t, inexactNoProps.Test(ctx, objFrom(ValMap{"a": Int(1), "b": Int(2)})))

	assert.False(t, singleProp.Test(ctx, objFrom(ValMap{})))
	assert.True(t, singleProp.Test(ctx, objFrom(ValMap{"a": Int(1)})))
	assert.False(t, singleProp.Test(ctx, objFrom(ValMap{"a": Int(1), "b": Int(2)})))

	assert.False(t, inexactSingleProp.Test(ctx, objFrom(ValMap{})))
	assert.True(t, inexactSingleProp.Test(ctx, objFrom(ValMap{"a": Int(1)})))
	assert.True(t, inexactSingleProp.Test(ctx, objFrom(ValMap{"a": Int(1), "b": Int(2)})))
}

func TestRecordPattern(t *testing.T) {
	noProps := &RecordPattern{entryPatterns: map[string]Pattern{}, inexact: false}
	inexactNoProps := &RecordPattern{entryPatterns: map[string]Pattern{}, inexact: true}
	singleProp := &RecordPattern{entryPatterns: map[string]Pattern{"a": INT_PATTERN}, inexact: false}
	inexactSingleProp := &RecordPattern{entryPatterns: map[string]Pattern{"a": INT_PATTERN}, inexact: true}

	assert.True(t, noProps.Test(nil, &Record{}))
	assert.False(t, noProps.Test(nil, NewRecordFromMap(ValMap{"a": Int(1)})))

	assert.True(t, inexactNoProps.Test(nil, NewRecordFromMap(ValMap{})))
	assert.True(t, inexactNoProps.Test(nil, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.True(t, inexactNoProps.Test(nil, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))

	assert.False(t, singleProp.Test(nil, NewRecordFromMap(ValMap{})))
	assert.True(t, singleProp.Test(nil, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.False(t, singleProp.Test(nil, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))

	assert.False(t, inexactSingleProp.Test(nil, NewRecordFromMap(ValMap{})))
	assert.True(t, inexactSingleProp.Test(nil, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.True(t, inexactSingleProp.Test(nil, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))

}

func TestListPattern(t *testing.T) {

	t.Run("Test(nil,)", func(t *testing.T) {
		//TODO
	})

}

func TestDifferencePattern(t *testing.T) {

	t.Run("Test(nil,)", func(t *testing.T) {
		//TODO
	})

}
