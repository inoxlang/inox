package core

import (
	"runtime"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
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
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("NewUnionPattern", func(t *testing.T) {
		patt := NewUnionPattern([]Pattern{INT_PATTERN, STR_PATTERN}, nil)
		assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN}, patt.Cases())

		t.Run("flattening", func(t *testing.T) {
			patt = NewUnionPattern([]Pattern{
				INT_PATTERN,
				NewUnionPattern([]Pattern{STR_PATTERN, BOOL_PATTERN}, nil),
			}, nil)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN}, patt.Cases())

			patt = NewUnionPattern([]Pattern{
				INT_PATTERN,
				NewDisjointUnionPattern([]Pattern{STR_PATTERN, BOOL_PATTERN}, nil),
			}, nil)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				NewDisjointUnionPattern([]Pattern{STR_PATTERN, BOOL_PATTERN}, nil),
			}, patt.Cases())

			patt = NewUnionPattern([]Pattern{
				INT_PATTERN,
				NewUnionPattern([]Pattern{
					STR_PATTERN,
					NewUnionPattern([]Pattern{BOOL_PATTERN, FLOAT_PATTERN}, nil),
				}, nil),
			}, nil)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN, FLOAT_PATTERN}, patt.Cases())

			patt = NewUnionPattern([]Pattern{
				INT_PATTERN,
				NewUnionPattern([]Pattern{
					STR_PATTERN,
					NewDisjointUnionPattern([]Pattern{BOOL_PATTERN, FLOAT_PATTERN}, nil),
				}, nil),
			}, nil)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				STR_PATTERN,
				NewDisjointUnionPattern([]Pattern{BOOL_PATTERN, FLOAT_PATTERN}, nil),
			}, patt.Cases())
		})

		t.Run("flattening disjoint cases", func(t *testing.T) {
			patt = NewDisjointUnionPattern([]Pattern{
				INT_PATTERN,
				NewDisjointUnionPattern([]Pattern{STR_PATTERN, BOOL_PATTERN}, nil),
			}, nil)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN}, patt.Cases())

			patt = NewDisjointUnionPattern([]Pattern{
				INT_PATTERN,
				NewUnionPattern([]Pattern{STR_PATTERN, BOOL_PATTERN}, nil),
			}, nil)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				NewUnionPattern([]Pattern{STR_PATTERN, BOOL_PATTERN}, nil),
			}, patt.Cases())

			patt = NewDisjointUnionPattern([]Pattern{
				INT_PATTERN,
				NewDisjointUnionPattern([]Pattern{
					STR_PATTERN,
					NewDisjointUnionPattern([]Pattern{BOOL_PATTERN, FLOAT_PATTERN}, nil),
				}, nil),
			}, nil)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN, FLOAT_PATTERN}, patt.Cases())

			patt = NewDisjointUnionPattern([]Pattern{
				INT_PATTERN,
				NewDisjointUnionPattern([]Pattern{
					STR_PATTERN,
					NewUnionPattern([]Pattern{BOOL_PATTERN, FLOAT_PATTERN}, nil),
				}, nil),
			}, nil)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				STR_PATTERN,
				NewUnionPattern([]Pattern{BOOL_PATTERN, FLOAT_PATTERN}, nil),
			}, patt.Cases())
		})
	})

	t.Run("Test()", func(t *testing.T) {
		patt := NewUnionPattern([]Pattern{
			NewInexactObjectPattern(map[string]Pattern{"a": NewExactValuePattern(Int(1))}),
			NewInexactObjectPattern(map[string]Pattern{"b": NewExactValuePattern(Int(2))}),
		}, nil)

		assert.True(t, patt.Test(ctx, NewObjectFromMapNoInit(ValMap{"a": Int(1)})))
		assert.True(t, patt.Test(ctx, NewObjectFromMapNoInit(ValMap{"b": Int(2)})))
		assert.True(t, patt.Test(ctx, NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Int(2)})))

		disjointPatt := NewDisjointUnionPattern([]Pattern{
			NewInexactObjectPattern(map[string]Pattern{"a": NewExactValuePattern(Int(1))}),
			NewInexactObjectPattern(map[string]Pattern{"b": NewExactValuePattern(Int(2))}),
		}, nil)

		assert.True(t, disjointPatt.Test(ctx, NewObjectFromMapNoInit(ValMap{"a": Int(1)})))
		assert.True(t, disjointPatt.Test(ctx, NewObjectFromMapNoInit(ValMap{"b": Int(2)})))
		assert.False(t, disjointPatt.Test(ctx, NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Int(2)})))
	})
}

func TestObjectPattern(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	noProps := NewExactObjectPattern(map[string]Pattern{})
	inexactNoProps := NewInexactObjectPattern(map[string]Pattern{})
	singleProp := NewExactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
	inexactSingleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
	singleOptionalProp := NewExactObjectPatternWithOptionalProps(
		map[string]Pattern{"a": INT_PATTERN},
		map[string]struct{}{"a": {}},
	)

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

	t.Run("constraint validations", func(t *testing.T) {
		{
			runtime.GC()
			startMemStats := new(runtime.MemStats)
			runtime.ReadMemStats(startMemStats)

			defer utils.AssertNoMemoryLeak(t, startMemStats, 10, utils.AssertNoMemoryLeakOptions{
				PreSleepDurationMillis: 100,
				CheckGoroutines:        true,
				GoroutineCount:         runtime.NumGoroutine(),
				MaxGoroutineCountDelta: 0,
			})
		}

		patternWithPropALessThan5 := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN}).WithConstraints(
			[]*ComplexPropertyConstraint{
				{
					Expr: parse.MustParseExpression("(self.a < 5)"),
				},
			},
		)

		ctx := NewContexWithEmptyState(ContextConfig{
			DoNotSpawnDoneGoroutine: true,
		}, nil)
		defer ctx.CancelGracefully()

		ok := patternWithPropALessThan5.Test(ctx, NewObjectFromMapNoInit(ValMap{
			"a": Int(1),
		}))

		if !assert.True(t, ok) {
			return
		}

		ok = patternWithPropALessThan5.Test(ctx, NewObjectFromMapNoInit(ValMap{
			"a": Int(5),
		}))

		if !assert.False(t, ok) {
			return
		}
	})

	t.Run("Entry", func(t *testing.T) {
		pattern, optional, yes := singleProp.Entry("a")

		if !assert.True(t, yes) {
			return
		}
		assert.Same(t, INT_PATTERN, pattern)
		assert.False(t, optional)

		pattern, optional, yes = singleOptionalProp.Entry("a")

		if !assert.True(t, yes) {
			return
		}
		assert.Same(t, INT_PATTERN, pattern)
		assert.True(t, optional)
	})
}

func TestRecordPattern(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

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

func TestIntRangePattern(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("0..100", func(t *testing.T) {
		patt := NewIncludedEndIntRangePattern(0, 100, -1)
		assert.True(t, patt.Test(ctx, Int(0)))
		assert.True(t, patt.Test(ctx, Int(1)))
		assert.True(t, patt.Test(ctx, Int(2)))
		assert.True(t, patt.Test(ctx, Int(3)))
		assert.True(t, patt.Test(ctx, Int(4)))
		assert.True(t, patt.Test(ctx, Int(6)))
		assert.True(t, patt.Test(ctx, Int(9)))
		assert.True(t, patt.Test(ctx, Int(99)))

		assert.False(t, patt.Test(ctx, Int(102)))
	})

	t.Run("0..100, multiple of 3", func(t *testing.T) {
		patt := NewIncludedEndIntRangePattern(0, 100, 3)
		assert.True(t, patt.Test(ctx, Int(0)))
		assert.True(t, patt.Test(ctx, Int(3)))
		assert.True(t, patt.Test(ctx, Int(6)))
		assert.True(t, patt.Test(ctx, Int(9)))
		assert.True(t, patt.Test(ctx, Int(99)))

		assert.False(t, patt.Test(ctx, Int(-1)))
		assert.False(t, patt.Test(ctx, Int(1)))
		assert.False(t, patt.Test(ctx, Int(2)))
		assert.False(t, patt.Test(ctx, Int(4)))
		assert.False(t, patt.Test(ctx, Int(102)))
	})
}

func TestFloatRangePattern(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("0.0..100.0", func(t *testing.T) {
		patt := NewFloatRangePattern(FloatRange{
			start: 0,
			end:   100,
		}, -1)
		assert.True(t, patt.Test(ctx, Float(0)))
		assert.True(t, patt.Test(ctx, Float(1)))
		assert.True(t, patt.Test(ctx, Float(2)))
		assert.True(t, patt.Test(ctx, Float(3)))
		assert.True(t, patt.Test(ctx, Float(4)))
		assert.True(t, patt.Test(ctx, Float(6)))
		assert.True(t, patt.Test(ctx, Float(9)))
		assert.True(t, patt.Test(ctx, Float(99)))

		assert.False(t, patt.Test(ctx, Float(102)))
	})

	t.Run("0.0..100.0, multiple of 3", func(t *testing.T) {
		patt := NewFloatRangePattern(FloatRange{
			start: 0,
			end:   100,
		}, 3)
		assert.True(t, patt.Test(ctx, Float(0)))
		assert.True(t, patt.Test(ctx, Float(3)))
		assert.True(t, patt.Test(ctx, Float(6)))
		assert.True(t, patt.Test(ctx, Float(9)))
		assert.True(t, patt.Test(ctx, Float(99)))

		assert.False(t, patt.Test(ctx, Float(-1)))
		assert.False(t, patt.Test(ctx, Float(1)))
		assert.False(t, patt.Test(ctx, Float(2)))
		assert.False(t, patt.Test(ctx, Float(4)))
		assert.False(t, patt.Test(ctx, Float(102)))
	})
}

func TestSimplifyIntersection(t *testing.T) {

	t.Run("object patterns", func(t *testing.T) {
		emptyExactObject := NewExactObjectPattern(map[string]Pattern{})
		emptyInexactObject := NewInexactObjectPattern(map[string]Pattern{})

		result := simplifyIntersection([]Pattern{OBJECT_PATTERN, emptyExactObject})
		assert.Same(t, emptyExactObject, result)

		result = simplifyIntersection([]Pattern{emptyExactObject, OBJECT_PATTERN})
		assert.Same(t, emptyExactObject, result)

		result = simplifyIntersection([]Pattern{OBJECT_PATTERN, emptyInexactObject})
		assert.Same(t, emptyInexactObject, result)

		result = simplifyIntersection([]Pattern{emptyInexactObject, OBJECT_PATTERN})
		assert.Same(t, emptyInexactObject, result)

		result = simplifyIntersection([]Pattern{OBJECT_PATTERN, emptyExactObject, emptyInexactObject})
		assert.Equal(t, NewIntersectionPattern([]Pattern{emptyExactObject, emptyInexactObject}, nil), result)

		result = simplifyIntersection([]Pattern{emptyExactObject, OBJECT_PATTERN, emptyInexactObject})
		assert.Equal(t, NewIntersectionPattern([]Pattern{emptyExactObject, emptyInexactObject}, nil), result)

		result = simplifyIntersection([]Pattern{emptyExactObject, emptyInexactObject, OBJECT_PATTERN})
		assert.Equal(t, NewIntersectionPattern([]Pattern{emptyExactObject, emptyInexactObject}, nil), result)
	})

}
