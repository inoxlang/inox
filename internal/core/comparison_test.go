package core

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCompareObjects(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

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

func TestCompareDictionaries(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	t.Run("same dictionary", func(t *testing.T) {
		dict := &Dictionary{}
		assert.True(t, dict.Equal(ctx, dict, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal dictionaries", func(t *testing.T) {
		d1 := NewDictionary(map[string]Value{"\"a\"": Int(1)})
		d2 := NewDictionary(map[string]Value{"\"a\"": Int(1)})

		assert.True(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.True(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different dictionaries : same keys but different values", func(t *testing.T) {
		d1 := NewDictionary(map[string]Value{"\"a\"": Int(0)})
		d2 := NewDictionary(map[string]Value{"\"a\"": Int(1)})

		assert.False(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.False(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different dictionaries : different keys", func(t *testing.T) {
		d1 := NewDictionary(map[string]Value{"\"a\"": Int(0)})
		d2 := NewDictionary(map[string]Value{"\"a\"": Int(1), "\"b\"": Int(2)})

		assert.False(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.False(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("equal dictionaries with a cycle", func(t *testing.T) {
		d1 := NewDictionary(map[string]Value{})
		d1.Entries["\"self\""] = d1
		d1.Keys["\"self\""] = Str("self")

		d2 := NewDictionary(map[string]Value{})
		d2.Entries["\"self\""] = d1
		d2.Keys["\"self\""] = Str("self")

		assert.True(t, d1.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
		assert.True(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.True(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})

	t.Run("non-equal dictionaries with a cycle", func(t *testing.T) {
		d1 := NewDictionary(map[string]Value{"\"a\"": Int(0)})
		d1.Entries["\"self\""] = d1
		d1.Keys["\"self\""] = Str("self")

		d2 := NewDictionary(map[string]Value{"\"a\"": Int(1)})
		d2.Entries["\"self\""] = d1
		d2.Keys["\"self\""] = Str("self")

		assert.False(t, d1.Equal(ctx, d2, map[uintptr]uintptr{}, 0))
		assert.False(t, d2.Equal(ctx, d1, map[uintptr]uintptr{}, 0))
	})
}

func TestCompareValueLists(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	t.Run("same list", func(t *testing.T) {
		s := &ValueList{elements: []Value{Str("a")}}

		assert.True(t, s.Equal(ctx, s, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal lists", func(t *testing.T) {
		s1 := &ValueList{elements: []Value{Str("a")}}
		s2 := &ValueList{elements: []Value{Str("a")}}

		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different lists", func(t *testing.T) {
		s1 := &ValueList{elements: []Value{Str("a")}}
		s2 := &ValueList{elements: []Value{Str("a"), Str("b")}}

		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("equal lists with a cycle", func(t *testing.T) {
		s1 := &ValueList{elements: []Value{Int(0)}}
		s2 := &ValueList{elements: []Value{Int(0)}}

		s1.elements[0] = s1
		s2.elements[0] = s2

		assert.True(t, s1.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("non-equal lists with a cycle", func(t *testing.T) {
		s1 := &ValueList{elements: []Value{Int(0), Int(1)}}
		s2 := &ValueList{elements: []Value{Int(0)}}

		s1.elements[0] = s1
		s2.elements[0] = s2

		assert.True(t, s1.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})
}

func TestCompareKeyLists(t *testing.T) {
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

func TestComparePaths(t *testing.T) {
	assert.True(t, Path("./").Equal(nil, Path("./"), map[uintptr]uintptr{}, 0))
	assert.False(t, Path("./").Equal(nil, Path("./a"), map[uintptr]uintptr{}, 0))
}

func TestComparePathPatterns(t *testing.T) {
	assert.True(t, PathPattern("./").Equal(nil, PathPattern("./"), map[uintptr]uintptr{}, 0))
	assert.False(t, PathPattern("./").Equal(nil, PathPattern("./a"), map[uintptr]uintptr{}, 0))
}

func TestCompareStrings(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	s := utils.Must(concatValues(ctx, []Value{
		Str(strings.Repeat("a", 50)),
		Str(strings.Repeat("a", 50)),
	}))
	assert.IsType(t, &StringConcatenation{}, s)

	t.Run("same string", func(t *testing.T) {
		s1 := Str(strings.Repeat("a", 100))
		s2 := utils.Must(concatValues(ctx, []Value{
			Str(strings.Repeat("a", 50)),
			Str(strings.Repeat("a", 50)),
		}))

		assert.True(t, s1.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
	})

	t.Run("two equal strings", func(t *testing.T) {
		s1 := Str(strings.Repeat("a", 100))
		s2 := utils.Must(concatValues(ctx, []Value{
			Str(strings.Repeat("a", 50)),
			Str(strings.Repeat("a", 50)),
		}))

		assert.True(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.True(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

	t.Run("two different lists", func(t *testing.T) {
		s1 := Str(strings.Repeat("a", 100))
		s2 := utils.Must(concatValues(ctx, []Value{
			Str(strings.Repeat("a", 50)),
			Str(strings.Repeat("a", 51)),
		}))

		assert.False(t, s1.Equal(ctx, s2, map[uintptr]uintptr{}, 0))
		assert.False(t, s2.Equal(ctx, s1, map[uintptr]uintptr{}, 0))
	})

}
