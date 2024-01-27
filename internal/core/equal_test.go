package core

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestEqualityCompareObjects(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("same object", func(t *testing.T) {
		obj := &Object{}
		assert.True(t, obj.Equal(ctx, obj, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal objects", func(t *testing.T) {
		o1 := objFrom(ValMap{"a": Int(1)})
		o2 := objFrom(ValMap{"a": Int(1)})

		assert.True(t, o1.Equal(ctx, o2, map[uintptr]uintptr{}, 0))
		assert.True(t, o2.Equal(ctx, o1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different objects : same keys but different values", func(t *testing.T) {
		o1 := objFrom(ValMap{"a": Int(1)})
		o2 := objFrom(ValMap{"a": Int(0)})

		assert.False(t, o1.Equal(ctx, o2, map[uintptr]uintptr{}, 0))
		assert.False(t, o2.Equal(ctx, o1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different objects : different keys", func(t *testing.T) {
		o1 := objFrom(ValMap{"a": Int(1)})
		o2 := objFrom(ValMap{"a": Int(1), "b": Int(2)})

		assert.False(t, o1.Equal(ctx, o2, map[uintptr]uintptr{}, 0))
		assert.False(t, o2.Equal(ctx, o1, map[uintptr]uintptr{}, 0))
	})

	t.Run("equal objects with a cycle", func(t *testing.T) {
		o1 := &Object{}
		o1.SetProp(ctx, "self", o1)

		o2 := &Object{}
		o2.SetProp(ctx, "self", o2)

		assert.True(t, o1.Equal(ctx, o1, map[uintptr]uintptr{}, 0))
		assert.True(t, o1.Equal(ctx, o2, map[uintptr]uintptr{}, 0))
		assert.True(t, o2.Equal(ctx, o1, map[uintptr]uintptr{}, 0))
	})

	t.Run("non-equal objects with a cycle", func(t *testing.T) {
		o1 := objFrom(ValMap{"a": Int(0)})
		o1.SetProp(ctx, "self", o1)

		o2 := objFrom(ValMap{"b": Int(1)})
		o2.SetProp(ctx, "self", o2)

		assert.False(t, o1.Equal(ctx, o2, map[uintptr]uintptr{}, 0))
		assert.False(t, o2.Equal(ctx, o1, map[uintptr]uintptr{}, 0))
	})
}

func TestEqualityCompareDictionaries(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("same dictionary", func(t *testing.T) {
		dict := &Dictionary{}
		assert.True(t, dict.Equal(ctx, dict, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal dictionaries", func(t *testing.T) {
		d1 := NewDictionary(map[string]Serializable{"\"a\"": Int(1)})
		d2 := NewDictionary(map[string]Serializable{"\"a\"": Int(1)})

		assert.True(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.True(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different dictionaries : same keys but different Serializables", func(t *testing.T) {
		d1 := NewDictionary(map[string]Serializable{"\"a\"": Int(0)})
		d2 := NewDictionary(map[string]Serializable{"\"a\"": Int(1)})

		assert.False(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.False(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different dictionaries : different keys", func(t *testing.T) {
		d1 := NewDictionary(map[string]Serializable{"\"a\"": Int(0)})
		d2 := NewDictionary(map[string]Serializable{"\"a\"": Int(1), "\"b\"": Int(2)})

		assert.False(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.False(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("equal dictionaries with a cycle", func(t *testing.T) {
		d1 := NewDictionary(map[string]Serializable{})
		d1.entries["\"self\""] = d1
		d1.keys["\"self\""] = String("self")

		d2 := NewDictionary(map[string]Serializable{})
		d2.entries["\"self\""] = d1
		d2.keys["\"self\""] = String("self")

		assert.True(t, d1.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
		assert.True(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.True(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("non-equal dictionaries with a cycle", func(t *testing.T) {
		d1 := NewDictionary(map[string]Serializable{"\"a\"": Int(0)})
		d1.entries["\"self\""] = d1
		d1.keys["\"self\""] = String("self")

		d2 := NewDictionary(map[string]Serializable{"\"a\"": Int(1)})
		d2.entries["\"self\""] = d1
		d2.keys["\"self\""] = String("self")

		assert.False(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.False(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})
}

func TestEqualityCompareValueLists(t *testing.T) {
	ctx := NewContext(ContextConfig{
		DoNotSpawnDoneGoroutine: true,
	})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("same list", func(t *testing.T) {
		s := &ValueList{elements: []Serializable{String("a")}}

		assert.True(t, s.Equal(ctx, s, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal lists", func(t *testing.T) {
		s1 := &ValueList{elements: []Serializable{String("a")}}
		s2 := &ValueList{elements: []Serializable{String("a")}}

		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different lists", func(t *testing.T) {
		s1 := &ValueList{elements: []Serializable{String("a")}}
		s2 := &ValueList{elements: []Serializable{String("a"), String("b")}}

		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("equal lists with a cycle", func(t *testing.T) {
		s1 := &ValueList{elements: []Serializable{Int(0)}}
		s2 := &ValueList{elements: []Serializable{Int(0)}}

		s1.elements[0] = s1
		s2.elements[0] = s2

		assert.True(t, s1.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("non-equal lists with a cycle", func(t *testing.T) {
		s1 := &ValueList{elements: []Serializable{Int(0), Int(1)}}
		s2 := &ValueList{elements: []Serializable{Int(0)}}

		s1.elements[0] = s1
		s2.elements[0] = s2

		assert.True(t, s1.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})
}

func TestEqualityCompareKeyLists(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	t.Run("same list", func(t *testing.T) {
		s := KeyList{"a"}

		assert.True(t, s.Equal(ctx, s, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal lists", func(t *testing.T) {
		s1 := KeyList{"a"}
		s2 := KeyList{"a"}

		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different key lists", func(t *testing.T) {
		s1 := KeyList{"a"}
		s2 := KeyList{"a", "b"}

		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two key lists with same value in different order", func(t *testing.T) {
		s1 := KeyList{"a", "b"}
		s2 := KeyList{"b", "a"}

		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})
}

func TestEqualityComparePaths(t *testing.T) {
	assert.True(t, Path("./").Equal(nil, Path("./"), map[uintptr]uintptr{}, 0))
	assert.False(t, Path("./").Equal(nil, Path("./a"), map[uintptr]uintptr{}, 0))
}

func TestEqualityComparePathPatterns(t *testing.T) {
	assert.True(t, PathPattern("./").Equal(nil, PathPattern("./"), map[uintptr]uintptr{}, 0))
	assert.False(t, PathPattern("./").Equal(nil, PathPattern("./a"), map[uintptr]uintptr{}, 0))
}

func TestEqualityCompareStrings(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	s := utils.Must(concatValues(ctx, []Value{
		String(strings.Repeat("a", 50)),
		String(strings.Repeat("a", 50)),
	}))
	assert.IsType(t, &StringConcatenation{}, s)

	t.Run("same string", func(t *testing.T) {
		s1 := String(strings.Repeat("a", 100))
		s2 := utils.Must(concatValues(ctx, []Value{
			String(strings.Repeat("a", 50)),
			String(strings.Repeat("a", 50)),
		}))

		assert.True(t, s1.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal strings", func(t *testing.T) {
		s1 := String(strings.Repeat("a", 100))
		s2 := utils.Must(concatValues(ctx, []Value{
			String(strings.Repeat("a", 50)),
			String(strings.Repeat("a", 50)),
		}))

		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different lists", func(t *testing.T) {
		s1 := String(strings.Repeat("a", 100))
		s2 := utils.Must(concatValues(ctx, []Value{
			String(strings.Repeat("a", 50)),
			String(strings.Repeat("a", 51)),
		}))

		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

}

func TestEqualityCompareOrderedPairs(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	intPairA := &OrderedPair{Int(1), Int(2)}
	intPairB := &OrderedPair{Int(2), Int(1)}

	assertEqualInoxValues(t, intPairA, intPairA, ctx)
	assertNotEqualInoxValues(t, intPairA, intPairB, ctx)
	assertNotEqualInoxValues(t, intPairB, intPairA, ctx)
}

func TestEqualityCompareULIDs(t *testing.T) {
	ulid1 := NewULID()
	ulid2 := NewULID()

	assertEqualInoxValues(t, ulid1, ulid1, nil)
	assertEqualInoxValues(t, ulid2, ulid2, nil)

	assertNotEqualInoxValues(t, ulid1, ulid2, nil)
	assertNotEqualInoxValues(t, ulid2, ulid1, nil)
}

func TestEqualityCompareUUIDs(t *testing.T) {
	firstUUID := NewUUIDv4()
	secondUUID := NewUUIDv4()

	assertEqualInoxValues(t, firstUUID, firstUUID, nil)
	assertEqualInoxValues(t, secondUUID, secondUUID, nil)

	assertNotEqualInoxValues(t, firstUUID, secondUUID, nil)
	assertNotEqualInoxValues(t, secondUUID, firstUUID, nil)
}

func TestEqualityCompareObjectPatterns(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("empty inexact", func(t *testing.T) {
		emptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{})
		emptyExact := NewExactObjectPattern([]ObjectPatternEntry{})
		notEmptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "x", Pattern: INT_PATTERN}})
		notEmptyExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "x", Pattern: INT_PATTERN}})

		assertEqualInoxValues(t, emptyInexact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, emptyInexact, emptyExact, ctx)
		assertNotEqualInoxValues(t, emptyInexact, notEmptyInexact, ctx)
		assertNotEqualInoxValues(t, emptyInexact, notEmptyExact, ctx)
	})

	t.Run("empty exact", func(t *testing.T) {
		emptyExact := NewExactObjectPattern([]ObjectPatternEntry{})
		emptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{})
		notEmptyExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "x", Pattern: INT_PATTERN}})
		notEmptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "x", Pattern: INT_PATTERN}})

		assertEqualInoxValues(t, emptyExact, emptyExact, ctx)
		assertNotEqualInoxValues(t, emptyExact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, emptyExact, notEmptyExact, ctx)
		assertNotEqualInoxValues(t, emptyExact, notEmptyInexact, ctx)
	})

	t.Run("single prop inexact", func(t *testing.T) {
		singlePropAInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		singlePropAExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		singlePropBInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}})
		emptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{})
		emptyExact := NewExactObjectPattern([]ObjectPatternEntry{})

		assertEqualInoxValues(t, singlePropAInexact, singlePropAInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, singlePropBInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, singlePropAExact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, emptyExact, ctx)
	})

	t.Run("single prop exact", func(t *testing.T) {
		singlePropAExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		singlePropBExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}})
		singlePropAInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		emptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{})
		emptyExact := NewExactObjectPattern([]ObjectPatternEntry{})

		assertEqualInoxValues(t, singlePropAExact, singlePropAExact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, singlePropBExact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, singlePropAInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, emptyExact, ctx)
	})
}

func TestEqualityCompareRecordPatterns(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)
	defer ctx.CancelGracefully()

	t.Run("empty inexact", func(t *testing.T) {
		emptyInexact := NewInexactRecordPattern([]RecordPatternEntry{})
		emptyExact := NewExactRecordPattern([]RecordPatternEntry{})
		notEmptyInexact := NewInexactRecordPattern([]RecordPatternEntry{{Name: "x", Pattern: INT_PATTERN}})
		notEmptyExact := NewExactRecordPattern([]RecordPatternEntry{{Name: "x", Pattern: INT_PATTERN}})

		assertEqualInoxValues(t, emptyInexact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, emptyInexact, emptyExact, ctx)
		assertNotEqualInoxValues(t, emptyInexact, notEmptyInexact, ctx)
		assertNotEqualInoxValues(t, emptyInexact, notEmptyExact, ctx)
	})

	t.Run("empty exact", func(t *testing.T) {
		emptyExact := NewExactRecordPattern([]RecordPatternEntry{})
		emptyInexact := NewInexactRecordPattern([]RecordPatternEntry{})
		notEmptyExact := NewExactRecordPattern([]RecordPatternEntry{{Name: "x", Pattern: INT_PATTERN}})
		notEmptyInexact := NewInexactRecordPattern([]RecordPatternEntry{{Name: "x", Pattern: INT_PATTERN}})

		assertEqualInoxValues(t, emptyExact, emptyExact, ctx)
		assertNotEqualInoxValues(t, emptyExact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, emptyExact, notEmptyExact, ctx)
		assertNotEqualInoxValues(t, emptyExact, notEmptyInexact, ctx)
	})

	t.Run("single prop inexact", func(t *testing.T) {
		singlePropAInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		singlePropAExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		singlePropBInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}})
		emptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{})
		emptyExact := NewExactObjectPattern([]ObjectPatternEntry{})

		assertEqualInoxValues(t, singlePropAInexact, singlePropAInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, singlePropBInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, singlePropAExact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAInexact, emptyExact, ctx)
	})

	t.Run("single prop exact", func(t *testing.T) {
		singlePropAExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		singlePropBExact := NewExactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}})
		singlePropAInexact := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})
		emptyInexact := NewInexactObjectPattern([]ObjectPatternEntry{})
		emptyExact := NewExactObjectPattern([]ObjectPatternEntry{})

		assertEqualInoxValues(t, singlePropAExact, singlePropAExact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, singlePropBExact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, singlePropAInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, emptyInexact, ctx)
		assertNotEqualInoxValues(t, singlePropAExact, emptyExact, ctx)
	})
}

func assertEqualInoxValues(t *testing.T, a, b Value, ctx *Context) bool {
	t.Helper()
	return assert.True(t, a.Equal(ctx, b, map[uintptr]uintptr{}, 0))
}

func assertNotEqualInoxValues(t *testing.T, a, b Value, ctx *Context) {
	t.Helper()
	assert.False(t, a.Equal(ctx, b, map[uintptr]uintptr{}, 0))
}
