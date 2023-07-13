package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJSONRepresentation(t *testing.T) {
	t.Run("object", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		//no pattern
		obj, err := ParseJSONRepresentation(ctx, `{"obj__value":{}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap())
		}

		obj, err = ParseJSONRepresentation(ctx, `{"obj__value":{"a":"1"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap())
		}

		//%obj patteren
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
		rec, err := ParseJSONRepresentation(ctx, `{"rec__value":{}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, rec.(*Record).ValueEntryMap())
		}

		rec, err = ParseJSONRepresentation(ctx, `{"rec__value":{"a":"1"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, rec.(*Record).ValueEntryMap())
		}

		//%rec patteren
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
