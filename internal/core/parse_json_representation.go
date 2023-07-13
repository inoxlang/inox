package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/exp/slices"
)

func ParseJSONRepresentation(ctx *Context, s string, pattern Pattern) (Serializable, error) {
	//TODO: add checks

	it := jsoniter.ParseString(jsoniter.ConfigCompatibleWithStandardLibrary, s)
	return parseJSONRepresentation(ctx, it, pattern)
}

func parseJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern) (_ Serializable, finalErr error) {
	switch p := pattern.(type) {
	case nil:
		if it.WhatIsNext() == jsoniter.ObjectValue {
			var value Serializable

			it.ReadObjectCB(func(i *jsoniter.Iterator, s string) bool {
				if value != nil || !strings.HasSuffix(s, JSON_UNTYPED_VALUE_SUFFIX) {
					finalErr = errors.New("impossible to determine type")
					return false
				}

				typename := strings.TrimSuffix(s, JSON_UNTYPED_VALUE_SUFFIX)

				pattern := getDefaultNamedPattern(typename)
				if pattern == nil {
					finalErr = fmt.Errorf("unknown typename: %s", typename)
					return false
				}

				value, finalErr = parseJSONRepresentation(ctx, it, pattern)
				return finalErr == nil
			})

			if finalErr != nil {
				return nil, finalErr
			}
			return value, nil
		}

		v := it.ReadAny()

		switch v.ValueType() {
		case jsoniter.BoolValue:
			return Bool(v.ToBool()), nil
		case jsoniter.StringValue:
			return Str(v.ToString()), nil
		case jsoniter.NilValue:
			return Nil, nil
		}

	case *IntRangePattern:
		return parseIntegerJSONRepresentation(ctx, it, pattern)
	case *ObjectPattern:
		return parseObjectJSONrepresentation(ctx, it, p)
	case *RecordPattern:
		return parseRecordJSONrepresentation(ctx, it, p)
	case *ListPattern:
		return parseListJSONrepresentation(ctx, it, p)
	case *TuplePattern:
		return parseTupleJSONrepresentation(ctx, it, p)
	case *TypePattern:
		switch p {
		case SERIALIZABLE_PATTERN:
			return parseJSONRepresentation(ctx, it, nil)
		case OBJECT_PATTERN:
			return parseObjectJSONrepresentation(ctx, it, EMPTY_INEXACT_OBJECT_PATTERN)
		case RECORD_PATTERN:
			return parseRecordJSONrepresentation(ctx, it, EMPTY_INEXACT_RECORD_PATTERN)
		case LIST_PATTERN:
			return parseListJSONrepresentation(ctx, it, ANY_ELEM_LIST_PATTERN)
		case TUPLE_PATTERN:
			return parseTupleJSONrepresentation(ctx, it, ANY_ELEM_TUPLE_PATTERN)
		case INT_PATTERN:
			return parseIntegerJSONRepresentation(ctx, it, nil)
		}
	}

	return nil, errors.New("impossible to determine type")
}

func parseObjectJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *ObjectPattern) (_ *Object, finalErr error) {
	obj := &Object{}
	it.ReadObjectCB(func(i *jsoniter.Iterator, key string) bool {
		obj.keys = append(obj.keys, key)

		var entryPattern Pattern
		if pattern != nil {
			entryPattern = pattern.entryPatterns[key]
		}

		val, err := parseJSONRepresentation(ctx, it, entryPattern)
		if err != nil {
			finalErr = fmt.Errorf("failed to parse value of object property %s: %w", key, err)
			return false
		}
		obj.values = append(obj.values, val)
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	var missingRequiredProperties []string

	pattern.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if !isOptional && !slices.Contains(obj.keys, propName) {
			missingRequiredProperties = append(missingRequiredProperties, propName)
		}
		return nil
	})

	if len(missingRequiredProperties) > 0 {
		return nil, fmt.Errorf("the following properties are missing: %s", strings.Join(missingRequiredProperties, ", "))
	}

	obj.sortProps()
	obj.initPartList(ctx)
	// add handlers before because jobs can mutate the object
	if err := obj.addMessageHandlers(ctx); err != nil {
		return nil, err
	}
	if err := obj.instantiateLifetimeJobs(ctx); err != nil {
		return nil, err
	}

	return obj, nil
}

func parseRecordJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *RecordPattern) (_ *Record, finalErr error) {
	rec := &Record{}
	it.ReadObjectCB(func(i *jsoniter.Iterator, key string) bool {
		rec.keys = append(rec.keys, key)

		var entryPattern Pattern
		if pattern != nil {
			entryPattern = pattern.entryPatterns[key]
		}

		val, err := parseJSONRepresentation(ctx, it, entryPattern)
		if err != nil {
			finalErr = fmt.Errorf("failed to parse value of record property %s: %w", key, err)
			return false
		}
		rec.values = append(rec.values, val)
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	var missingRequiredProperties []string

	pattern.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if !isOptional && !slices.Contains(rec.keys, propName) {
			missingRequiredProperties = append(missingRequiredProperties, propName)
		}
		return nil
	})

	if len(missingRequiredProperties) > 0 {
		return nil, fmt.Errorf("the following properties are missing: %s", strings.Join(missingRequiredProperties, ", "))
	}

	rec.sortProps()

	return rec, nil
}

func parseIntegerJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern) (_ Int, finalErr error) {
	s := it.ReadString()
	if it.Error != nil {
		return 0, fmt.Errorf("failed to parse integer: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse integer: %w", err)
	}
	return Int(i), nil
}

func parseListJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *ListPattern) (_ *List, finalErr error) {
	var elements []Serializable
	index := 0
	it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
		elementPattern, ok := pattern.ElementPatternAt(index)
		if !ok {
			finalErr = fmt.Errorf("JSON array has too many elements, %d elements were expected", len(pattern.elementPatterns))
			return false
		}

		val, err := parseJSONRepresentation(ctx, it, elementPattern)
		if err != nil {
			finalErr = fmt.Errorf("failed to parse element %d of array: %w", index, err)
			return false
		}
		elements = append(elements, val)
		index++
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	if pattern.elementPatterns != nil && len(elements) < len(pattern.elementPatterns) {
		return nil, fmt.Errorf("JSON array has not enough elements, %d elements were expected", len(pattern.elementPatterns))
	}

	return NewWrappedValueList(elements...), nil
}

func parseTupleJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *TuplePattern) (_ *Tuple, finalErr error) {
	var elements []Serializable
	index := 0
	it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
		elementPattern, ok := pattern.ElementPatternAt(index)
		if !ok {
			finalErr = fmt.Errorf("JSON array has too many elements, %d elements were expected", len(pattern.elementPatterns))
			return false
		}

		val, err := parseJSONRepresentation(ctx, it, elementPattern)
		if err != nil {
			finalErr = fmt.Errorf("failed to parse element %d of array: %w", index, err)
			return false
		}
		elements = append(elements, val)
		index++
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	if pattern.elementPatterns != nil && len(elements) < len(pattern.elementPatterns) {
		return nil, fmt.Errorf("JSON array has not enough elements, %d elements were expected", len(pattern.elementPatterns))
	}

	return NewTuple(elements), nil
}
