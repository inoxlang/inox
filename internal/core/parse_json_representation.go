package core

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"slices"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	parse "github.com/inoxlang/inox/internal/parse"
)

var (
	ErrJsonNotMatchingSchema         = errors.New("JSON is not matching schema")
	ErrTriedToParseJSONRepr          = errors.New("tried to parse json representation but failed")
	ErrNotMatchingSchemaIntFound     = errors.New("integer was found but it does not match the schema")
	ErrNotMatchingSchemaFloatFound   = errors.New("float was found but it does not match the schema")
	ErrJSONImpossibleToDetermineType = errors.New("impossible to determine type")
)

func ParseJSONRepresentation(ctx *Context, s string, pattern Pattern) (Serializable, error) {
	//TODO: add checks

	it := jsoniter.ParseString(jsoniter.ConfigDefault, s)
	return ParseNextJSONRepresentation(ctx, it, pattern, false)
}

func ParseNextJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ Serializable, finalErr error) {
	switch p := pattern.(type) {
	case nil:
		if it.WhatIsNext() == jsoniter.ObjectValue {
			//first we check if the object has the shape {"typename__value": <value>},
			//if it's not the case we call parseObjectJSONrepresentation.

			var value Serializable

			p := it.SkipAndReturnBytes()
			tempIterator := jsoniter.NewIterator(jsoniter.ConfigDefault)
			tempIterator.ResetBytes(slices.Clone(p))

			tempIterator.ReadObjectCB(func(i *jsoniter.Iterator, s string) bool {
				if value != nil || !strings.HasSuffix(s, JSON_UNTYPED_VALUE_SUFFIX) {
					return false
				}

				typename := strings.TrimSuffix(s, JSON_UNTYPED_VALUE_SUFFIX)

				pattern := getDefaultNamedPattern(typename)
				if pattern == nil {
					finalErr = fmt.Errorf("unknown typename: %s", typename)
					return false
				}

				value, finalErr = ParseNextJSONRepresentation(ctx, tempIterator, pattern, false)
				return finalErr == nil
			})

			if finalErr != nil && finalErr != ErrTriedToParseJSONRepr {
				return nil, finalErr
			}

			finalErr = nil

			if value != nil {
				return value, nil
			}

			tempIterator.ResetBytes(p)
			tempIterator.Error = nil

			return parseObjectJSONrepresentation(ctx, tempIterator, nil, false)
		}

		switch it.WhatIsNext() {
		case jsoniter.ArrayValue:
			return parseListJSONrepresentation(ctx, it, nil, false)
		case jsoniter.BoolValue:
			return Bool(it.ReadBool()), nil
		case jsoniter.StringValue:
			return Str(it.ReadString()), nil
		case jsoniter.NilValue:
			return Nil, nil
		case jsoniter.NumberValue:
			number := it.ReadNumber()
			if strings.Contains(number.String(), ".") {
				float, err := number.Float64()
				if err != nil {
					return nil, err
				}
				return Float(float), nil
			}
			integer, err := number.Int64()
			if err != nil {
				return nil, err
			}
			return Int(integer), nil
		}

	case *IntRangePattern:
		return parseIntegerJSONRepresentation(ctx, it, pattern, try)
	case *FloatRangePattern:
		return parseFloatJSONRepresentation(ctx, it, pattern, try)
	case *ObjectPattern:
		return parseObjectJSONrepresentation(ctx, it, p, try)
	case *RecordPattern:
		return parseRecordJSONrepresentation(ctx, it, p, try)
	case *ListPattern:
		return parseListJSONrepresentation(ctx, it, p, try)
	case *TuplePattern:
		return parseTupleJSONrepresentation(ctx, it, p, try)
	case *UnionPattern:
		return parseUnionJSONrepresentation(ctx, it, p, try)
	case *IntersectionPattern:
		return parseIntersectionJSONrepresentation(ctx, it, p, try)
	case *ExactValuePattern:
		var pattern Pattern

		switch p.value.(type) {
		case Int:
			pattern = INT_PATTERN
		case Float:
			pattern = FLOAT_PATTERN
		case Bool:
			pattern = BOOL_PATTERN
		case NilT:
			pattern = NIL_PATTERN
		case StringLike:
			pattern = STR_PATTERN
		default:
			return nil, errors.New("exact value patterns with a complex value are not supported yet")
		}

		v, err := ParseNextJSONRepresentation(ctx, it, pattern, false)
		if err != nil {
			return nil, err
		}
		if !v.Equal(ctx, p.value, nil, 0) {
			return nil, ErrJsonNotMatchingSchema
		}
		return v, nil
	case *TypePattern:
		switch p {
		case SERIALIZABLE_PATTERN:
			return ParseNextJSONRepresentation(ctx, it, nil, try)
		case NIL_PATTERN:
			if it.WhatIsNext() != jsoniter.NilValue {
				return nil, ErrJsonNotMatchingSchema
			}
			return Nil, nil
		case STR_PATTERN, STRLIKE_PATTERN:
			if it.WhatIsNext() != jsoniter.StringValue {
				return nil, ErrJsonNotMatchingSchema
			}
			return Str(it.ReadString()), nil
		case BOOL_PATTERN:
			if it.WhatIsNext() != jsoniter.BoolValue {
				return nil, ErrJsonNotMatchingSchema
			}
			return Bool(it.ReadBool()), nil
		case FLOAT_PATTERN:
			if it.WhatIsNext() != jsoniter.NumberValue {
				return nil, ErrJsonNotMatchingSchema
			}
			return Float(it.ReadFloat64()), nil
		case OBJECT_PATTERN:
			return parseObjectJSONrepresentation(ctx, it, EMPTY_INEXACT_OBJECT_PATTERN, try)
		case RECORD_PATTERN:
			return parseRecordJSONrepresentation(ctx, it, EMPTY_INEXACT_RECORD_PATTERN, try)
		case LIST_PATTERN:
			return parseListJSONrepresentation(ctx, it, ANY_ELEM_LIST_PATTERN, try)
		case TUPLE_PATTERN:
			return parseTupleJSONrepresentation(ctx, it, ANY_ELEM_TUPLE_PATTERN, try)
		case INT_PATTERN:
			return parseIntegerJSONRepresentation(ctx, it, p, try)
		case LINECOUNT_PATTERN:
			return parseLineCountJSONRepresentation(ctx, it, nil, try)
		case BYTECOUNT_PATTERN:
			return parseByteCountJSONRepresentation(ctx, it, nil, try)
		case RUNECOUNT_PATTERN:
			return parseRuneCountJSONRepresentation(ctx, it, nil, try)
		case SIMPLERATE_PATTERN:
			//TODO
		case BYTERATE_PATTERN:
			//TODO
		case PATH_PATTERN:
			return parsePathJSONRepresentation(ctx, it, try)
		case SCHEME_PATTERN:
			return parseSchemeJSONRepresentation(ctx, it, try)
		case HOST_PATTERN:
			return parseHostJSONRepresentation(ctx, it, try)
		case URL_PATTERN:
			return parseURLJSONRepresentation(ctx, it, try)
		case PATHPATTERN_PATTERN:
			return parsePathPatternJSONRepresentation(ctx, it, try)
		case HOSTPATTERN_PATTERN:
			return parseHostPatternJSONRepresentation(ctx, it, try)
		case URLPATTERN_PATTERN:
			return parseURLPatternJSONRepresentation(ctx, it, try)
		case ANYVAL_PATTERN, SERIALIZABLE_PATTERN:
			return ParseNextJSONRepresentation(ctx, it, nil, try)
		default:
			return nil, fmt.Errorf("%q type pattern is not a core pattern", p.Name)
		}
	}

	return nil, ErrJSONImpossibleToDetermineType
}

func parseObjectJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *ObjectPattern, try bool) (_ *Object, finalErr error) {
	obj := &Object{}

	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

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

		val, err := ParseNextJSONRepresentation(ctx, it, entryPattern, try)
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

	//auto fix & see what properties are missing
	if pattern != nil {
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
	}

	if len(missingRequiredProperties) > 0 {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
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

func parseRecordJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *RecordPattern, try bool) (_ *Record, finalErr error) {
	rec := &Record{}

	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

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

		val, err := ParseNextJSONRepresentation(ctx, it, entryPattern, try)
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
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, fmt.Errorf("the following properties are missing: %s", strings.Join(missingRequiredProperties, ", "))
	}

	rec.sortProps()

	return rec, nil
}

func parseIntegerJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ Int, finalErr error) {
	var integer Int
	switch it.WhatIsNext() {
	case jsoniter.NumberValue:
		n := it.ReadInt64()
		if it.Error != nil && it.Error != io.EOF {
			if try {
				return 0, ErrTriedToParseJSONRepr
			}
			return 0, fmt.Errorf("failed to parse integer: %w", it.Error)
		}
		integer = Int(n)
	case jsoniter.StringValue:
		//representation of integers as a JSON string is only allowed if an integer is expected.
		if pattern != nil {
			s := it.ReadString()
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				if try {
					return 0, ErrTriedToParseJSONRepr
				}
				return 0, fmt.Errorf("failed to parse integer: %w", err)
			}
			integer = Int(n)
			break
		}
		fallthrough
	default:
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}

	if patt, ok := pattern.(*IntRangePattern); ok && !patt.Test(ctx, integer) {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrNotMatchingSchemaIntFound
	}
	return integer, nil
}

func parseFloatJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ Float, finalErr error) {
	if it.WhatIsNext() != jsoniter.NumberValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}
	float := Float(it.ReadFloat64())

	if it.Error != nil && it.Error != io.EOF {
		return 0, fmt.Errorf("failed to parse float: %w", it.Error)
	}

	if patt, ok := pattern.(*FloatRangePattern); ok && !patt.Test(ctx, float) {
		return 0, ErrNotMatchingSchemaFloatFound
	}
	return float, nil
}

func parseLineCountJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ LineCount, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}
	s := it.ReadString()

	if !strings.HasSuffix(s, LINE_COUNT_UNIT) {
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, LINE_COUNT_UNIT)

	if it.Error != nil && it.Error != io.EOF {
		return 0, fmt.Errorf("failed to parse line count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse line count: %w", err)
	}
	return LineCount(i), nil
}

func parseByteCountJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ ByteCount, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}
	s := it.ReadString()

	if !strings.HasSuffix(s, BYTE_COUNT_UNIT) {
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, BYTE_COUNT_UNIT)

	if it.Error != nil && it.Error != io.EOF {
		return 0, fmt.Errorf("failed to parse byte count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse byte count: %w", err)
	}
	return ByteCount(i), nil
}

func parseRuneCountJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ RuneCount, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}

	s := it.ReadString()

	if !strings.HasSuffix(s, RUNE_COUNT_UNIT) {
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, RUNE_COUNT_UNIT)

	if it.Error != nil && it.Error != io.EOF {
		return 0, fmt.Errorf("failed to parse rune count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse rune count: %w", err)
	}
	return RuneCount(i), nil
}

func parsePathJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ Path, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return Path(it.ReadString()), nil
}

func parseSchemeJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ Scheme, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return Scheme(it.ReadString()), nil
}

func parseHostJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ Host, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return Host(it.ReadString()), nil
}

func parseURLJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ URL, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return URL(it.ReadString()), nil
}

func parsePathPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ PathPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return PathPattern(it.ReadString()), nil
}

func parseHostPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ HostPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return HostPattern(it.ReadString()), nil
}

func parseURLPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ URLPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	return URLPattern(it.ReadString()), nil
}

func parseSameTypeListJSONRepr[T any](ctx *Context, it *jsoniter.Iterator, elementPattern Pattern, try bool, finalErr *error) (elements []T) {
	index := 0
	it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
		val, err := ParseNextJSONRepresentation(ctx, it, elementPattern, try)
		if err != nil {
			if try {
				*finalErr = ErrTriedToParseJSONRepr
				return false
			}

			*finalErr = fmt.Errorf("failed to parse element %d of array: %w", index, err)
			return false
		}
		elements = append(elements, val.(T))
		index++
		return true
	})
	return
}

func parseListJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *ListPattern, try bool) (_ *List, finalErr error) {
	if it.WhatIsNext() != jsoniter.ArrayValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	index := 0

	if pattern == nil {
		var elements []Serializable

		it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
			val, err := ParseNextJSONRepresentation(ctx, it, nil, try)
			if err != nil {
				if try {
					finalErr = ErrTriedToParseJSONRepr
					return false
				}
				finalErr = fmt.Errorf("failed to parse element %d of array: %w", index, err)
				return false
			}
			elements = append(elements, val)
			index++
			return true
		})

		return NewWrappedValueList(elements...), nil
	}

	if pattern.elementPatterns != nil {
		var elements []Serializable

		it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
			elementPattern, ok := pattern.ElementPatternAt(index)
			if !ok {
				finalErr = fmt.Errorf("JSON array has too many elements, %d elements were expected", len(pattern.elementPatterns))
				return false
			}

			val, err := ParseNextJSONRepresentation(ctx, it, elementPattern, try)
			if err != nil {
				if try {
					finalErr = ErrTriedToParseJSONRepr
					return false
				}
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
	} else {
		generalElementPattern := pattern.generalElementPattern
		if _, isIntRangePattern := generalElementPattern.(*IntRangePattern); isIntRangePattern || generalElementPattern == INT_PATTERN {
			elements := parseSameTypeListJSONRepr[Int](ctx, it, generalElementPattern, try, &finalErr)
			if finalErr != nil {
				return nil, finalErr
			}
			return NewWrappedIntListFrom(elements), nil
		} else if generalElementPattern == BOOL_PATTERN {
			elements := parseSameTypeListJSONRepr[Bool](ctx, it, generalElementPattern, try, &finalErr)
			if finalErr != nil {
				return nil, finalErr
			}
			return NewWrappedBoolList(elements...), nil
		} else if _, ok := generalElementPattern.(StringPattern); ok || generalElementPattern == STRLIKE_PATTERN || generalElementPattern == STR_PATTERN {
			elements := parseSameTypeListJSONRepr[StringLike](ctx, it, generalElementPattern, try, &finalErr)
			if finalErr != nil {
				return nil, finalErr
			}

			return NewWrappedStringListFrom(elements), nil
		} //else
		elements := parseSameTypeListJSONRepr[Serializable](ctx, it, generalElementPattern, try, &finalErr)
		if finalErr != nil {
			return nil, finalErr
		}

		return NewWrappedValueListFrom(elements), nil
	}
}

func parseTupleJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *TuplePattern, try bool) (_ *Tuple, finalErr error) {
	if it.WhatIsNext() != jsoniter.ArrayValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	var elements []Serializable
	index := 0
	it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
		elementPattern, ok := pattern.ElementPatternAt(index)
		if !ok {
			finalErr = fmt.Errorf("JSON array has too many elements, %d elements were expected", len(pattern.elementPatterns))
			return false
		}

		val, err := ParseNextJSONRepresentation(ctx, it, elementPattern, try)
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

func parseUnionJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *UnionPattern, try bool) (_ Serializable, finalErr error) {
	//TODO: optimize by not copying the bytes
	p := it.SkipAndReturnBytes()
	tempIterator := jsoniter.NewIterator(jsoniter.ConfigDefault)
	defer tempIterator.ResetBytes(nil)

	var (
		value                  Serializable
		firstMatchingCaseIndex int = -1
		err                    error
	)

	for i, unionCase := range pattern.cases {
		tempIterator.ResetBytes(slices.Clone(p))
		tempIterator.Error = nil

		value, err = ParseNextJSONRepresentation(ctx, tempIterator, unionCase, true)
		if errors.Is(err, ErrTriedToParseJSONRepr) {
			continue
		}
		if err != nil {
			return nil, err
		}

		firstMatchingCaseIndex = i
		break
	}

	if firstMatchingCaseIndex < 0 {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	//check that value does not match any other other case
	if pattern.disjoint && firstMatchingCaseIndex != len(pattern.cases)-1 {
		for _, unionCase := range pattern.cases[firstMatchingCaseIndex+1:] {
			if unionCase.Test(ctx, value) {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, ErrJsonNotMatchingSchema
			}
		}
	}

	return value, nil
}

func parseIntersectionJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *IntersectionPattern, try bool) (_ Serializable, finalErr error) {
	firstCase := pattern.cases[0]
	value, err := ParseNextJSONRepresentation(ctx, it, firstCase, try)

	if try && errors.Is(err, ErrJsonNotMatchingSchema) {
		return nil, ErrJsonNotMatchingSchema
	}

	if err != nil {
		return nil, err
	}

	for _, otherCases := range pattern.cases[1:] {
		if !otherCases.Test(ctx, value) {
			return nil, ErrJsonNotMatchingSchema
		}
	}
	return value, nil
}
