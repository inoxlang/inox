package core

import (
	"runtime"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestParseJSONRepresentation(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 10)
	}

	t.Run("strings", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"str__value":"a"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Str("a"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"a"`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Str("a"), v)
		}

	})

	t.Run("booleans", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//boolean
		v, err := ParseJSONRepresentation(ctx, `{"bool__value":true}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, True, v)
		}

		v, err = ParseJSONRepresentation(ctx, `true`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, True, v)
		}

		v, err = ParseJSONRepresentation(ctx, `false`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, False, v)
		}
	})

	t.Run("integers", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"int__value":1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `1`, INT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		//if no schema is specified a float is expected
		v, err = ParseJSONRepresentation(ctx, `1`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		intPattern := NewIntRangePattern(IntRange{start: 0, end: 2, inclusiveEnd: true, step: 1}, -1)
		v, err = ParseJSONRepresentation(ctx, `1`, intPattern)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		_, err = ParseJSONRepresentation(ctx, `-1`, intPattern)
		if !assert.Error(t, err, ErrNotMatchingSchemaIntFound) {
			return
		}

		_, err = ParseJSONRepresentation(ctx, `3`, intPattern)
		if !assert.ErrorIs(t, err, ErrNotMatchingSchemaIntFound) {
			return
		}

		//int (as string)
		v, err = ParseJSONRepresentation(ctx, `{"int__value":"1"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1"`, INT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}
	})

	t.Run("floats", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"float__value":1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `1.0`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		floatPattern := NewFloatRangePattern(FloatRange{start: 0, end: 2, inclusiveEnd: true}, -1)
		v, err = ParseJSONRepresentation(ctx, `1`, floatPattern)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		_, err = ParseJSONRepresentation(ctx, `-1`, floatPattern)
		if !assert.Error(t, err, ErrNotMatchingSchemaFloatFound) {
			return
		}

		_, err = ParseJSONRepresentation(ctx, `3`, floatPattern)
		if !assert.ErrorIs(t, err, ErrNotMatchingSchemaFloatFound) {
			return
		}
	})

	t.Run("line counts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"line-count__value":"1ln"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, LineCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1ln"`, LINECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, LineCount(1), v)
		}
	})

	t.Run("byte counts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"byte-count__value":"1B"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1B"`, BYTECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteCount(1), v)
		}
	})

	t.Run("rune counts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"rune-count__value":"1rn"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, RuneCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1rn"`, RUNECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, RuneCount(1), v)
		}
	})

	t.Run("paths", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"path__value":"/"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Path("/"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"/"`, PATH_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Path("/"), v)
		}
	})

	t.Run("schemes", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//scheme
		v, err := ParseJSONRepresentation(ctx, `{"scheme__value":"https"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Scheme("https"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https"`, SCHEME_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Scheme("https"), v)
		}
	})

	t.Run("hosts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"host__value":"https://example.com"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Host("https://example.com"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com"`, HOST_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Host("https://example.com"), v)
		}
	})

	t.Run("urls", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"url__value":"https://example.com/"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, URL("https://example.com/"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com/"`, URL_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, URL("https://example.com/"), v)
		}
	})

	t.Run("path patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"path-pattern__value":"/..."}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, PathPattern("/..."), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"/..."`, PATHPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, PathPattern("/..."), v)
		}
	})

	t.Run("host patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"host-pattern__value":"https://*.com"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, HostPattern("https://*.com"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://*.com"`, HOSTPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, HostPattern("https://*.com"), v)
		}
	})

	t.Run("url patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"url-pattern__value":"https://example.com/..."}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, URLPattern("https://example.com/..."), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com/..."`, URLPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, URLPattern("https://example.com/..."), v)
		}
	})
	t.Run("objects", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		obj, err := ParseJSONRepresentation(ctx, `{"object__value":{}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"a":"1"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"_url_":"ldb://main/users/0"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap(nil))
		}

		url, ok := obj.(*Object).URL()
		if assert.True(t, ok) {
			assert.Equal(t, URL("ldb://main/users/0"), url)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"_x_":"0"}}`, nil)
		if assert.ErrorIs(t, err, ErrNonSupportedMetaProperty) {
			assert.Nil(t, obj)
		}

		//%object patteren
		obj, err = ParseJSONRepresentation(ctx, `{}`, OBJECT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":"1"}`, OBJECT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap(nil))
		}

		//{a: int} pattern
		pattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1)}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":1,"b":"c"}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1), "b": Str("c")}, obj.(*Object).ValueEntryMap(nil))
		}

		//{a: {b: int}} pattern
		pattern = NewInexactObjectPattern(map[string]Pattern{"a": NewInexactObjectPattern(map[string]Pattern{"b": INT_PATTERN})})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":{}}`, pattern)
		if assert.ErrorContains(t, err, "failed to parse value of object property \"a\": the following properties are missing: b") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":{"b": 1}}`, pattern)
		if assert.NoError(t, err) {
			entries := obj.(*Object).ValueEntryMap(nil)
			if assert.Contains(t, entries, "a") {
				assert.Equal(t, map[string]Value{"b": Int(1)}, entries["a"].(*Object).ValueEntryMap(nil))
			}
		}

		//{a: []int} pattern ([]int has a default value)
		pattern = NewInexactObjectPattern(map[string]Pattern{"a": NewListPatternOf(INT_PATTERN)})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern) //auto fix
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewWrappedValueList()}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":[1]}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewWrappedIntList(Int(1))}, obj.(*Object).ValueEntryMap(nil))
		}
	})

	t.Run("records", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		rec, err := ParseJSONRepresentation(ctx, `{"record__value":{}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, rec.(*Record).ValueEntryMap())
		}

		rec, err = ParseJSONRepresentation(ctx, `{"record__value":{"a":"1"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, rec.(*Record).ValueEntryMap())
		}

		rec, err = ParseJSONRepresentation(ctx, `{"record__value":{"_x_":"0"}}`, nil)
		if assert.ErrorIs(t, err, ErrNonSupportedMetaProperty) {
			assert.Nil(t, rec)
		}

		//%record patteren
		rec, err = ParseJSONRepresentation(ctx, `{}`, RECORD_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, rec.(*Record).ValueEntryMap())
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":"1"}`, RECORD_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, rec.(*Record).ValueEntryMap())
		}

		//{a: int} pattern
		pattern := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})

		rec, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, rec)
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1)}, rec.(*Record).ValueEntryMap())
		}

		//{a: {b: int}} pattern
		pattern = NewInexactRecordPattern(map[string]Pattern{"a": NewInexactRecordPattern(map[string]Pattern{"b": INT_PATTERN})})

		rec, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, rec)
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":{}}`, pattern)
		if assert.ErrorContains(t, err, "failed to parse value of record property a: the following properties are missing: b") {
			assert.Nil(t, rec)
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":{"b": 1}}`, pattern)
		if assert.NoError(t, err) {
			entries := rec.(*Record).ValueEntryMap()
			if assert.Contains(t, entries, "a") {
				assert.Equal(t, map[string]Value{"b": Int(1)}, entries["a"].(*Record).ValueEntryMap())
			}
		}

		//{a: #[]int} pattern (#[]int has a default value)
		pattern = NewInexactRecordPattern(map[string]Pattern{"a": NewTuplePatternOf(INT_PATTERN)})

		rec, err = ParseJSONRepresentation(ctx, `{}`, pattern) //auto fix
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewTuple(nil)}, rec.(*Record).ValueEntryMap())
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":[1]}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewTuple([]Serializable{Int(1)})}, rec.(*Record).ValueEntryMap())
		}
	})

	t.Run("lists", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		list, err := ParseJSONRepresentation(ctx, `{"list__value":[]}`, nil)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `{"list__value":["1"]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, list.(*List).GetOrBuildElements(ctx))
		}

		//%list patteren
		list, err = ParseJSONRepresentation(ctx, `[]`, LIST_PATTERN)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, LIST_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, list.(*List).GetOrBuildElements(ctx))
		}

		//[]int pattern
		pattern := NewListPatternOf(INT_PATTERN)

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*IntList)(nil), list.(*List).underlyingList)
		}

		list, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*IntList)(nil), list.(*List).underlyingList)
		}

		//[]bool pattern
		pattern = NewListPatternOf(BOOL_PATTERN)

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*BoolList)(nil), list.(*List).underlyingList)
		}

		list, err = ParseJSONRepresentation(ctx, `[true]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{True}, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*BoolList)(nil), list.(*List).underlyingList)
		}

		//[] pattern
		pattern = NewListPattern([]Pattern{})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.ErrorContains(t, err, "JSON array too many elements (1), at most 0 element(s) were expected") {
			assert.Nil(t, list)
		}

		//[int] pattern
		pattern = NewListPattern([]Pattern{INT_PATTERN})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `[1,2]`, pattern)
		if assert.ErrorContains(t, err, "JSON array too many elements (2), at most 1 element(s) were expected") {
			assert.Nil(t, list)
		}

		//[int, int] pattern
		pattern = NewListPattern([]Pattern{INT_PATTERN, INT_PATTERN})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (0), 2 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (1), 2 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[1, 2]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1), Int(2)}, list.(*List).GetOrBuildElements(ctx))
		}

		//[[int]] pattern
		pattern = NewListPattern([]Pattern{NewListPattern([]Pattern{INT_PATTERN})})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[[1]]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{NewWrappedValueList(Int(1))}, list.(*List).GetOrBuildElements(ctx))
		}
	})

	t.Run("tuples", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		tuple, err := ParseJSONRepresentation(ctx, `{"tuple__value":[]}`, nil)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `{"tuple__value":["1"]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//%tuple patteren
		tuple, err = ParseJSONRepresentation(ctx, `[]`, TUPLE_PATTERN)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, TUPLE_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[]int pattern
		pattern := NewTuplePatternOf(INT_PATTERN)

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[] pattern
		pattern = NewTuplePattern([]Pattern{})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many elements (1), 0 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		//[int] pattern
		pattern = NewTuplePattern([]Pattern{INT_PATTERN})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1,2]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many elements (2), 1 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		//[int, int] pattern
		pattern = NewTuplePattern([]Pattern{INT_PATTERN, INT_PATTERN})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (0), 2 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (1), 2 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1, 2]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1), Int(2)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[[int]] pattern
		pattern = NewTuplePattern([]Pattern{NewTuplePattern([]Pattern{INT_PATTERN})})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[[1]]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{NewTuple([]Serializable{Int(1)})}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}
	})

	t.Run("unions", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("integers & strings", func(t *testing.T) {
			pattern := NewUnionPattern([]Pattern{INT_PATTERN, STRLIKE_PATTERN}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			//priority to first pattern
			val, err = ParseJSONRepresentation(ctx, `"1"`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `" 1"`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, Str(" 1"), val)
			}
		})

		t.Run("integers", func(t *testing.T) {
			range1 := IntRange{start: 0, end: 2, inclusiveEnd: true, step: 1}
			range2 := IntRange{start: 1, end: 3, inclusiveEnd: true, step: 1}

			pattern1 := NewUnionPattern([]Pattern{NewIntRangePattern(range1, -1), NewIntRangePattern(range2, -1)}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `3`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(3), val)
			}

			_, err = ParseJSONRepresentation(ctx, `2`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(2), val)
			}
		})

		t.Run("objects", func(t *testing.T) {
			pattern := NewUnionPattern([]Pattern{
				NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN}),
				NewInexactObjectPattern(map[string]Pattern{"b": INT_PATTERN}),
			}, nil)

			val, err := ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1)}), val)
			}

			val, err = ParseJSONRepresentation(ctx, `{"b":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"b": Int(1)}), val)
			}

			_, err = ParseJSONRepresentation(ctx, `{"a":1,"b":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Int(1)}), val)
			}
		})

		t.Run("lists", func(t *testing.T) {
			pattern := NewUnionPattern([]Pattern{
				NewListPatternOf(INT_PATTERN),
				NewListPatternOf(BOOL_PATTERN),
			}, nil)

			val, err := ParseJSONRepresentation(ctx, `[]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedIntListFrom([]Int{}), val)
			}

			val, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedIntList(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `[true]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedBoolList(True), val)
			}
		})
	})

	t.Run("disjoint unions", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("integers & strings", func(t *testing.T) {
			pattern1 := NewUnionPattern([]Pattern{INT_PATTERN, STRLIKE_PATTERN}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			//priority to first pattern
			val, err = ParseJSONRepresentation(ctx, `"1"`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `" 1"`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Str(" 1"), val)
			}
		})

		t.Run("integers", func(t *testing.T) {
			range1 := IntRange{start: 0, end: 2, inclusiveEnd: true, step: 1}
			range2 := IntRange{start: 2, end: 3, inclusiveEnd: true, step: 1}

			pattern1 := NewDisjointUnionPattern([]Pattern{NewIntRangePattern(range1, -1), NewIntRangePattern(range2, -1)}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `3`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(3), val)
			}

			_, err = ParseJSONRepresentation(ctx, `2`, pattern1)
			assert.ErrorIs(t, err, ErrJsonNotMatchingSchema)
		})

		t.Run("objects", func(t *testing.T) {
			//no pattern
			val, err := ParseJSONRepresentation(ctx, `{"a":1,"b":1}`, nil)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Int(1)}), val)
			}

			pattern := NewDisjointUnionPattern([]Pattern{
				NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN}),
				NewInexactObjectPattern(map[string]Pattern{"b": INT_PATTERN}),
			}, nil)

			val, err = ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1)}), val)
			}

			val, err = ParseJSONRepresentation(ctx, `{"b":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"b": Int(1)}), val)
			}

			_, err = ParseJSONRepresentation(ctx, `{"a":1,"b":1}`, pattern)
			assert.ErrorIs(t, err, ErrJsonNotMatchingSchema)
		})

		t.Run("lists", func(t *testing.T) {
			//no pattern
			val, err := ParseJSONRepresentation(ctx, `[1, 2]`, nil)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedValueList(Int(1), Int(2)), val)
			}

			pattern := NewDisjointUnionPattern([]Pattern{
				NewListPatternOf(INT_PATTERN),
				NewListPatternOf(BOOL_PATTERN),
			}, nil)

			_, err = ParseJSONRepresentation(ctx, `[]`, pattern)
			assert.ErrorIs(t, err, ErrJsonNotMatchingSchema)

			val, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedIntList(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `[true]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedBoolList(True), val)
			}
		})
	})

	t.Run("exact values", func(t *testing.T) {

	})
}
