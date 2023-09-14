package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"slices"

	parse "github.com/inoxlang/inox/internal/parse"
	jsoniter "github.com/json-iterator/go"
)

var (
	IncorrectJSONRepresentation = errors.New("incorrect JSON representation")
)

func ParseJSONRepresentation(ctx *Context, s string, pattern Pattern) (Serializable, error) {
	//TODO: add checks

	it := jsoniter.ParseString(jsoniter.ConfigCompatibleWithStandardLibrary, s)
	return ParseNextJSONRepresentation(ctx, it, pattern)
}

func ParseNextJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern) (_ Serializable, finalErr error) {
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

				value, finalErr = ParseNextJSONRepresentation(ctx, it, pattern)
				return finalErr == nil
			})

			if value == nil {
				finalErr = errors.New("impossible to determine type")
			}

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
		case jsoniter.NumberValue:
			return Float(v.ToFloat64()), nil
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
			return ParseNextJSONRepresentation(ctx, it, nil)
		case STR_PATTERN:
			if it.WhatIsNext() != jsoniter.StringValue {
				return nil, IncorrectJSONRepresentation
			}
			return Str(it.ReadString()), nil
		case BOOL_PATTERN:
			if it.WhatIsNext() != jsoniter.BoolValue {
				return nil, IncorrectJSONRepresentation
			}
			return Bool(it.ReadBool()), nil
		case FLOAT_PATTERN:
			if it.WhatIsNext() != jsoniter.NumberValue {
				return nil, IncorrectJSONRepresentation
			}
			return Float(it.ReadFloat64()), nil
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
		case LINECOUNT_PATTERN:
			return parseLineCountJSONRepresentation(ctx, it, nil)
		case BYTECOUNT_PATTERN:
			return parseByteCountJSONRepresentation(ctx, it, nil)
		case RUNECOUNT_PATTERN:
			return parseRuneCountJSONRepresentation(ctx, it, nil)
		case SIMPLERATE_PATTERN:
			//TODO
		case BYTERATE_PATTERN:
			//TODO
		case PATH_PATTERN:
			return parsePathJSONRepresentation(ctx, it)
		case SCHEME_PATTERN:
			return parseSchemeJSONRepresentation(ctx, it)
		case HOST_PATTERN:
			return parseHostJSONRepresentation(ctx, it)
		case URL_PATTERN:
			return parseURLJSONRepresentation(ctx, it)
		case PATHPATTERN_PATTERN:
			return parsePathPatternJSONRepresentation(ctx, it)
		case HOSTPATTERN_PATTERN:
			return parseHostPatternJSONRepresentation(ctx, it)
		case URLPATTERN_PATTERN:
			return parseURLPatternJSONRepresentation(ctx, it)
		default:
			return nil, fmt.Errorf("%q type pattern is not a core pattern", p.Name)
		}
	}

	return nil, errors.New("impossible to determine type")
}

func parseObjectJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *ObjectPattern) (_ *Object, finalErr error) {
	obj := &Object{}
	it.ReadObjectCB(func(i *jsoniter.Iterator, key string) bool {
		if parse.IsMetadataKey(key) && key != URL_METADATA_KEY {
			finalErr = fmt.Errorf("%w: %s", ErrNonSupportedMetaProperty, key)
			return false
		}

		if key == URL_METADATA_KEY {
			obj.url = URL(it.ReadString())
			return true
		}

		obj.keys = append(obj.keys, key)

		var entryPattern Pattern
		if pattern != nil {
			entryPattern = pattern.entryPatterns[key]
		}

		val, err := ParseNextJSONRepresentation(ctx, it, entryPattern)
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

			//try auto fix
			defaultValPattern, ok := propPattern.(DefaultValuePattern)
			if ok {
				defaultValue, err := defaultValPattern.DefaultValue(ctx)
				if err != nil {
					goto missing
				}
				obj.keys = append(obj.keys, propName)
				obj.values = append(obj.values, defaultValue.(Serializable))
				return nil
			}

		missing:
			missingRequiredProperties = append(missingRequiredProperties, propName)
		}
		return nil
	})

	if len(missingRequiredProperties) > 0 {
		return nil, fmt.Errorf("the following properties are missing: %s", strings.Join(missingRequiredProperties, ", "))
	}

	obj.sortProps()
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
		if parse.IsMetadataKey(key) {
			finalErr = fmt.Errorf("%w: %s", ErrNonSupportedMetaProperty, key)
			return false
		}
		rec.keys = append(rec.keys, key)

		var entryPattern Pattern
		if pattern != nil {
			entryPattern = pattern.entryPatterns[key]
		}

		val, err := ParseNextJSONRepresentation(ctx, it, entryPattern)
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

			//try auto fix
			defaultValPattern, ok := propPattern.(DefaultValuePattern)
			if ok {
				defaultValue, err := defaultValPattern.DefaultValue(ctx)
				if err != nil {
					goto missing
				}
				rec.keys = append(rec.keys, propName)
				rec.values = append(rec.values, defaultValue.(Serializable))
				return nil
			}

		missing:
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

func parseLineCountJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern) (_ LineCount, finalErr error) {
	s := it.ReadString()

	if !strings.HasSuffix(s, LINE_COUNT_UNIT) {
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, LINE_COUNT_UNIT)

	if it.Error != nil {
		return 0, fmt.Errorf("failed to parse line count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse line count: %w", err)
	}
	return LineCount(i), nil
}

func parseByteCountJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern) (_ ByteCount, finalErr error) {
	s := it.ReadString()

	if !strings.HasSuffix(s, BYTE_COUNT_UNIT) {
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, BYTE_COUNT_UNIT)

	if it.Error != nil {
		return 0, fmt.Errorf("failed to parse byte count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse byte count: %w", err)
	}
	return ByteCount(i), nil
}

func parseRuneCountJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern) (_ RuneCount, finalErr error) {
	s := it.ReadString()

	if !strings.HasSuffix(s, RUNE_COUNT_UNIT) {
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, RUNE_COUNT_UNIT)

	if it.Error != nil {
		return 0, fmt.Errorf("failed to parse rune count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse rune count: %w", err)
	}
	return RuneCount(i), nil
}

func parsePathJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ Path, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return Path(it.ReadString()), nil
}

func parseSchemeJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ Scheme, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return Scheme(it.ReadString()), nil
}

func parseHostJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ Host, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return Host(it.ReadString()), nil
}

func parseURLJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ URL, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return URL(it.ReadString()), nil
}

func parsePathPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ PathPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return PathPattern(it.ReadString()), nil
}

func parseHostPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ HostPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return HostPattern(it.ReadString()), nil
}

func parseURLPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator) (_ URLPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		return "", IncorrectJSONRepresentation
	}

	return URLPattern(it.ReadString()), nil
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

		val, err := ParseNextJSONRepresentation(ctx, it, elementPattern)
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

		val, err := ParseNextJSONRepresentation(ctx, it, elementPattern)
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
