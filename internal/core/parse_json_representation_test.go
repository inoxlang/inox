package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJSONRepresentation(t *testing.T) {

	t.Run("simple values", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		//string
		v, err := ParseJSONRepresentation(ctx, `{"str__value":"a"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Str("a"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"a"`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Str("a"), v)
		}

		//boolean
		v, err = ParseJSONRepresentation(ctx, `{"bool__value":true}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, True, v)
		}

		v, err = ParseJSONRepresentation(ctx, `true`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, True, v)
		}

		//float
		v, err = ParseJSONRepresentation(ctx, `{"float__value":1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `1`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		//int
		v, err = ParseJSONRepresentation(ctx, `{"int__value":"1"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1"`, INT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		//line count
		v, err = ParseJSONRepresentation(ctx, `{"line-count__value":"1ln"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, LineCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1ln"`, LINECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, LineCount(1), v)
		}

		//byte count
		v, err = ParseJSONRepresentation(ctx, `{"byte-count__value":"1B"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1B"`, BYTECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteCount(1), v)
		}

		//rune count
		v, err = ParseJSONRepresentation(ctx, `{"rune-count__value":"1rn"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, RuneCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1rn"`, RUNECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, RuneCount(1), v)
		}

		//path
		v, err = ParseJSONRepresentation(ctx, `{"path__value":"/"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Path("/"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"/"`, PATH_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Path("/"), v)
		}

		//scheme
		v, err = ParseJSONRepresentation(ctx, `{"scheme__value":"https"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Scheme("https"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https"`, SCHEME_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Scheme("https"), v)
		}

		//host
		v, err = ParseJSONRepresentation(ctx, `{"host__value":"https://example.com"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Host("https://example.com"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com"`, HOST_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Host("https://example.com"), v)
		}

		//url
		v, err = ParseJSONRepresentation(ctx, `{"url__value":"https://example.com/"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, URL("https://example.com/"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com/"`, URL_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, URL("https://example.com/"), v)
		}

		//path pattern
		v, err = ParseJSONRepresentation(ctx, `{"path-pattern__value":"/..."}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, PathPattern("/..."), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"/..."`, PATHPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, PathPattern("/..."), v)
		}

		//host pattern
		v, err = ParseJSONRepresentation(ctx, `{"host-pattern__value":"https://*.com"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, HostPattern("https://*.com"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://*.com"`, HOSTPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, HostPattern("https://*.com"), v)
		}

		//url pattern
		v, err = ParseJSONRepresentation(ctx, `{"url-pattern__value":"https://example.com/..."}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, URLPattern("https://example.com/..."), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com/..."`, URLPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, URLPattern("https://example.com/..."), v)
		}
	})

	t.Run("object", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		//no pattern
		obj, err := ParseJSONRepresentation(ctx, `{"object__value":{}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap())
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"a":"1"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap())
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"_url_":"ldb://main/users/0"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap())
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
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap())
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":"1"}`, OBJECT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap())
		}

		//{a: int} pattern
		pattern := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":"1"}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1)}, obj.(*Object).ValueEntryMap())
		}

		//{a: {b: int}} pattern
		pattern = NewInexactObjectPattern(map[string]Pattern{"a": NewInexactObjectPattern(map[string]Pattern{"b": INT_PATTERN})})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":{}}`, pattern)
		if assert.ErrorContains(t, err, "failed to parse value of object property a: the following properties are missing: b") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":{"b": "1"}}`, pattern)
		if assert.NoError(t, err) {
			entries := obj.(*Object).ValueEntryMap()
			if assert.Contains(t, entries, "a") {
				assert.Equal(t, map[string]Value{"b": Int(1)}, entries["a"].(*Object).ValueEntryMap())
			}
		}
	})

	t.Run("record", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

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

		rec, err = ParseJSONRepresentation(ctx, `{"a":"1"}`, pattern)
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

		rec, err = ParseJSONRepresentation(ctx, `{"a":{"b": "1"}}`, pattern)
		if assert.NoError(t, err) {
			entries := rec.(*Record).ValueEntryMap()
			if assert.Contains(t, entries, "a") {
				assert.Equal(t, map[string]Value{"b": Int(1)}, entries["a"].(*Record).ValueEntryMap())
			}
		}
	})

	t.Run("list", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		//no pattern
		list, err := ParseJSONRepresentation(ctx, `{"list__value":[]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `{"list__value":["1"]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, list.(*List).GetOrBuildElements(ctx))
		}

		//%list patteren
		list, err = ParseJSONRepresentation(ctx, `{}`, LIST_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, LIST_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, list.(*List).GetOrBuildElements(ctx))
		}

		//[]int pattern
		pattern := NewListPatternOf(INT_PATTERN)

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, list.(*List).GetOrBuildElements(ctx))
		}

		//[] pattern
		pattern = NewListPattern([]Pattern{})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many elements, 0 elements were expected") {
			assert.Nil(t, list)
		}

		//[int] pattern
		pattern = NewListPattern([]Pattern{INT_PATTERN})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 1 elements were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, list.(*List).GetOrBuildElements(ctx))
		}

		//[int, int] pattern
		pattern = NewListPattern([]Pattern{INT_PATTERN, INT_PATTERN})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 2 elements were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 2 elements were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `["1", "2"]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1), Int(2)}, list.(*List).GetOrBuildElements(ctx))
		}

		//[[int]] pattern
		pattern = NewListPattern([]Pattern{NewListPattern([]Pattern{INT_PATTERN})})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 1 elements were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[["1"]]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{NewWrappedValueList(Int(1))}, list.(*List).GetOrBuildElements(ctx))
		}
	})

	t.Run("tuple", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		//no pattern
		tuple, err := ParseJSONRepresentation(ctx, `{"tuple__value":[]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `{"tuple__value":["1"]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//%tuple patteren
		tuple, err = ParseJSONRepresentation(ctx, `{}`, TUPLE_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, TUPLE_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[]int pattern
		pattern := NewTuplePatternOf(INT_PATTERN)

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[] pattern
		pattern = NewTuplePattern([]Pattern{})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many elements, 0 elements were expected") {
			assert.Nil(t, tuple)
		}

		//[int] pattern
		pattern = NewTuplePattern([]Pattern{INT_PATTERN})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 1 elements were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[int, int] pattern
		pattern = NewTuplePattern([]Pattern{INT_PATTERN, INT_PATTERN})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 2 elements were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 2 elements were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1", "2"]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1), Int(2)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[[int]] pattern
		pattern = NewTuplePattern([]Pattern{NewTuplePattern([]Pattern{INT_PATTERN})})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements, 1 elements were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[["1"]]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{NewTuple([]Serializable{Int(1)})}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}
	})

}
