package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExactValuePattern(t *testing.T) {
	patt := NewExactValuePattern(Int(2))

	assert.True(t, patt.Equal(nil, patt, nil, 0))
	assert.False(t, patt.Equal(nil, NewExactValuePattern(Float(2)), nil, 0))
}

func TestExactStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {
		patt := NewExactStringPattern(Str("ab"))
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
	singleOptionalProp := &ObjectPattern{
		entryPatterns:   map[string]Pattern{"a": INT_PATTERN},
		optionalEntries: map[string]struct{}{"a": {}},
		inexact:         false,
	}

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

	assert.True(t, singleOptionalProp.Test(ctx, objFrom(ValMap{})))
	assert.True(t, singleOptionalProp.Test(ctx, objFrom(ValMap{"a": Int(1)})))
	assert.False(t, singleOptionalProp.Test(ctx, objFrom(ValMap{"a": Str("")})))
	assert.False(t, singleOptionalProp.Test(ctx, objFrom(ValMap{"a": Int(1), "b": Int(2)})))
}

func TestRecordPattern(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	noProps := &RecordPattern{entryPatterns: map[string]Pattern{}, inexact: false}
	inexactNoProps := &RecordPattern{entryPatterns: map[string]Pattern{}, inexact: true}
	singleProp := &RecordPattern{entryPatterns: map[string]Pattern{"a": INT_PATTERN}, inexact: false}
	inexactSingleProp := &RecordPattern{entryPatterns: map[string]Pattern{"a": INT_PATTERN}, inexact: true}
	singleOptionalProp := &RecordPattern{
		entryPatterns:   map[string]Pattern{"a": INT_PATTERN},
		optionalEntries: map[string]struct{}{"a": {}},
		inexact:         false,
	}

	assert.True(t, noProps.Test(ctx, &Record{}))
	assert.False(t, noProps.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1)})))

	assert.True(t, inexactNoProps.Test(ctx, NewRecordFromMap(ValMap{})))
	assert.True(t, inexactNoProps.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.True(t, inexactNoProps.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))

	assert.False(t, singleProp.Test(ctx, NewRecordFromMap(ValMap{})))
	assert.True(t, singleProp.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.False(t, singleProp.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))

	assert.False(t, inexactSingleProp.Test(ctx, NewRecordFromMap(ValMap{})))
	assert.True(t, inexactSingleProp.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.True(t, inexactSingleProp.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))

	assert.True(t, singleOptionalProp.Test(ctx, NewRecordFromMap(ValMap{})))
	assert.True(t, singleOptionalProp.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1)})))
	assert.False(t, singleOptionalProp.Test(ctx, NewRecordFromMap(ValMap{"a": Str("")})))
	assert.False(t, singleOptionalProp.Test(ctx, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)})))
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
