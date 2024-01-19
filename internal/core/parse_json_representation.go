package core

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"slices"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/parse"
)

var (
	ErrJsonNotMatchingSchema         = errors.New("JSON is not matching schema")
	ErrTriedToParseJSONRepr          = errors.New("tried to parse json representation but failed")
	ErrNotMatchingSchemaIntFound     = errors.New("integer was found but it does not match the schema")
	ErrNotMatchingSchemaFloatFound   = errors.New("float was found but it does not match the schema")
	ErrJSONImpossibleToDetermineType = errors.New("impossible to determine type")
	ErrNonSupportedMetaProperty      = errors.New("non-supported meta property")
	ErrInvalidRuneRepresentation     = errors.New("invalid rune representation")
)

func ParseJSONRepresentation(ctx *Context, s string, pattern Pattern) (Serializable, error) {
	//TODO: add checks
	//TODO: return an error if there are duplicate keys.

	it := jsoniter.ParseString(jsoniter.ConfigDefault, s)
	return ParseNextJSONRepresentation(ctx, it, pattern, false)
}

func ParseNextJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (res Serializable, finalErr error) {
	defer func() {
		if finalErr != nil {
			res = nil
		}
	}()

	switch p := pattern.(type) {
	case nil:
		if it.WhatIsNext() == jsoniter.ObjectValue {
			//first we check if the object has the shape {"typename__value": <value>},
			//if it's not the case we call parseObjectJSONrepresentation.

			var value Serializable

			p := it.SkipAndReturnBytes()
			tempIterator := jsoniter.NewIterator(jsoniter.ConfigDefault)
			tempIterator.ResetBytes(slices.Clone(p))

			tempIterator.ReadObjectCB(func(it *jsoniter.Iterator, s string) bool {
				if value != nil || !strings.HasSuffix(s, JSON_UNTYPED_VALUE_SUFFIX) {
					return false
				}

				typename := strings.TrimSuffix(s, JSON_UNTYPED_VALUE_SUFFIX)

				pattern := DEFAULT_NAMED_PATTERNS[typename]
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
			float, err := number.Float64()
			if err != nil {
				return nil, err
			}
			return Float(float), nil
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
	case StringPattern:
		if it.WhatIsNext() != jsoniter.StringValue {
			if try {
				return nil, ErrTriedToParseJSONRepr
			}
			return nil, ErrJsonNotMatchingSchema
		}
		s := Str(it.ReadString())
		if !p.Test(ctx, s) {
			if try {
				return nil, ErrTriedToParseJSONRepr
			}
			return nil, ErrJsonNotMatchingSchema
		}
		return s, nil
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

		v, err := ParseNextJSONRepresentation(ctx, it, pattern, try)
		if err != nil {
			return nil, err
		}
		if !v.Equal(ctx, p.value, nil, 0) {
			if try {
				return nil, ErrTriedToParseJSONRepr
			}
			return nil, ErrJsonNotMatchingSchema
		}
		return v, nil
	case *DifferencePattern:
		//TODO: optimize by not copying the bytes
		bytes := it.SkipAndReturnBytes()
		tempIterator := jsoniter.NewIterator(jsoniter.ConfigDefault)
		defer tempIterator.ResetBytes(nil)

		tempIterator.ResetBytes(bytes)

		value, err := ParseNextJSONRepresentation(ctx, tempIterator, p.base, true)
		if err != nil {
			if try {
				return nil, ErrTriedToParseJSONRepr
			}
			return nil, err
		}

		tempIterator.Error = nil
		tempIterator.ResetBytes(bytes)

		_, err = ParseNextJSONRepresentation(ctx, tempIterator, p.removed, true)
		if err == nil {
			if try {
				return nil, ErrTriedToParseJSONRepr
			}
			return nil, ErrJsonNotMatchingSchema
		}

		return value, nil
	case *TypePattern:
		switch p {
		case SERIALIZABLE_PATTERN:
			return ParseNextJSONRepresentation(ctx, it, nil, try)
		case NIL_PATTERN:
			if it.WhatIsNext() != jsoniter.NilValue {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, ErrJsonNotMatchingSchema
			}
			return Nil, nil
		case STR_PATTERN, STRING_PATTERN:
			if it.WhatIsNext() != jsoniter.StringValue {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, ErrJsonNotMatchingSchema
			}
			return Str(it.ReadString()), nil
		case RUNE_PATTERN:
			if it.WhatIsNext() != jsoniter.StringValue {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, ErrJsonNotMatchingSchema
			}
			s := it.ReadString()
			if utf8.RuneCountInString(s) != 1 {
				return nil, ErrInvalidRuneRepresentation
			}
			return Rune(s[0]), nil
		case BOOL_PATTERN:
			if it.WhatIsNext() != jsoniter.BoolValue {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, ErrJsonNotMatchingSchema
			}
			return Bool(it.ReadBool()), nil
		case FLOAT_PATTERN:
			if it.WhatIsNext() != jsoniter.NumberValue {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
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
		case FREQUENCY_PATTERN:
			return parseFrequencyJSONRepresentation(ctx, it, nil, try)
		case BYTERATE_PATTERN:
			return parseByteRateJSONRepresentation(ctx, it, nil, try)
		case DURATION_PATTERN:
			return parseDurationJSONRepresentation(ctx, it, nil, try)
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
		case PROPNAME_PATTERN:
			return parsePropNameJSONRepresentation(ctx, it, try)
		case LONG_VALUEPATH_PATTERN:
			return parseLongValuePathJSONRepresentation(ctx, it, try)
		case NAMED_SEGMENT_PATH_PATTERN:
			return parseNamedSegmentPathPatternSONRepresentation(ctx, it, try)
		case TYPE_PATTERN_PATTERN:
			return parseTypePatternSONRepresentation(ctx, it, try)
		case OBJECT_PATTERN_PATTERN:
			return parseObjectPatternJSONRepresentation(ctx, it, try)
		case RECORD_PATTERN_PATTERN:
			return parseRecordPatternJSONRepresentation(ctx, it, try)
		case LIST_PATTERN_PATTERN:
			return parseListPatternJSONRepresentation(ctx, it, try)
		case TUPLE_PATTERN_PATTERN:
			return parseTuplePatternJSONRepresentation(ctx, it, try)
		case EXACT_VALUE_PATTERN_PATTERN:
			return parseExactValuePatternJSONRepresentation(ctx, it, try)
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

	var parseInTwoIterations = false

	var effectivePatternEntries ObjectPatternEntriesHelper

	if pattern != nil {
		if pattern.entries != nil {
			effectivePatternEntries = ObjectPatternEntriesHelper(pattern.entries)
		}

		for _, entry := range pattern.entries {
			if entry.Dependencies.Pattern != nil {
				//Parse in two iterations to update the effective pattern entries
				//according to present keys.
				parseInTwoIterations = true
				effectivePatternEntries = slices.Clone(pattern.entries)
				break
			}
		}
	}

	var currentIt = it

	if parseInTwoIterations {
		objectBytes := it.SkipAndReturnBytes()
		tempIt := jsoniter.NewIterator(jsoniter.ConfigDefault)
		tempIt.ResetBytes(objectBytes)

		//first iteration to know the keys
		tempIt.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
			if parse.IsMetadataKey(key) && key != URL_METADATA_KEY {
				finalErr = fmt.Errorf("%w: %s", ErrNonSupportedMetaProperty, key)
				return false
			}

			tempIt.Skip()
			if key == URL_METADATA_KEY {
				return true
			}

			patternEntry, _ := effectivePatternEntries.CompleteEntry(key)

			//We update the effective entry pattern based on the dependencies.
			if objPatt, ok := patternEntry.Dependencies.Pattern.(*ObjectPattern); ok {
				for _, conditionalEntry := range objPatt.entries {

					patternEntry, ok := effectivePatternEntries.CompleteEntry(conditionalEntry.Name)

					if ok {
						var effectivePattern Pattern
						if patternEntry.Pattern == SERIALIZABLE_PATTERN {
							effectivePattern = conditionalEntry.Pattern
						} else {
							effectivePattern = NewIntersectionPattern([]Pattern{patternEntry.Pattern, conditionalEntry.Pattern}, nil)
						}
						effectivePatternEntries = effectivePatternEntries.setEntry(conditionalEntry.Name, ObjectPatternEntry{
							Name:    conditionalEntry.Name,
							Pattern: effectivePattern,
						})
					} else {
						effectivePatternEntries = effectivePatternEntries.setEntry(conditionalEntry.Name, ObjectPatternEntry{
							Name:    conditionalEntry.Name,
							Pattern: conditionalEntry.Pattern,
						})
					}
				}
			}
			return true
		})

		if finalErr != nil {
			return nil, finalErr
		}

		if it.Error != nil && it.Error != io.EOF {
			return nil, it.Error
		}

		tempIt.Error = nil
		tempIt.ResetBytes(objectBytes)
		currentIt = tempIt
	}

	currentIt.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		if parse.IsMetadataKey(key) && key != URL_METADATA_KEY {
			finalErr = fmt.Errorf("%w: %s", ErrNonSupportedMetaProperty, key)
			return false
		}

		if key == URL_METADATA_KEY {
			obj.url = URL(currentIt.ReadString())
			return true
		}

		obj.keys = append(obj.keys, key)
		entryPattern, _, _ := effectivePatternEntries.Entry(key) //no issue if entryPattenrn is nil

		val, err := ParseNextJSONRepresentation(ctx, currentIt, entryPattern, try)
		if err != nil {
			finalErr = fmt.Errorf("failed to parse value of object property %q: %w", key, err)
			return false
		}
		obj.values = append(obj.values, val)
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	var missingRequiredProperties []string

	//If exact, check that there are no additional properties
	if pattern != nil && !pattern.inexact {
		for _, key := range obj.keys {
			if effectivePatternEntries.HasRequiredOrOptionalEntry(key) {
				continue
			}
			//Unexpected additional property.
			if try {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, fmt.Errorf("unexpected property %q (exact object pattern)", key)
			}
		}
	}

	//Auto fix & see what properties are missing.
	if pattern != nil {
		pattern.ForEachEntry(func(entry ObjectPatternEntry) error {
			if !entry.IsOptional && !slices.Contains(obj.keys, entry.Name) {

				//try auto fix
				defaultValPattern, ok := entry.Pattern.(DefaultValuePattern)
				if ok {
					defaultValue, err := defaultValPattern.DefaultValue(ctx)
					if err != nil {
						goto missing
					}
					obj.keys = append(obj.keys, entry.Name)
					obj.values = append(obj.values, defaultValue.(Serializable))
					return nil
				}

			missing:
				missingRequiredProperties = append(missingRequiredProperties, entry.Name)
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

	//check dependencies
	if pattern != nil && len(pattern.dependentKeys) > 0 {
		for _, propName := range obj.keys {
			entry, _ := effectivePatternEntries.CompleteEntry(propName)

			deps := entry.Dependencies

			//check required keys
			for _, requiredKey := range deps.RequiredKeys {
				ok := false
				for _, name := range obj.keys {
					if name == requiredKey {
						ok = true
						break
					}
				}

				if !ok {
					if try {
						return nil, ErrTriedToParseJSONRepr
					}
					return nil, fmt.Errorf("due to dependencies the following property is missing: %q", requiredKey)
				}
			}

			if deps.Pattern != nil && !deps.Pattern.Test(ctx, obj) {
				if try {
					return nil, ErrTriedToParseJSONRepr
				}
				return nil, fmt.Errorf("dependencies of property %q are not fulfilled", propName)
			}
		}
	}

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

	it.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		if parse.IsMetadataKey(key) {
			finalErr = fmt.Errorf("%w: %s", ErrNonSupportedMetaProperty, key)
			return false
		}
		rec.keys = append(rec.keys, key)

		var entryPattern Pattern
		if pattern != nil {
			entryPattern, _, _ = pattern.Entry(key)
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

	pattern.ForEachEntry(func(entry RecordPatternEntry) error {
		if !entry.IsOptional && !slices.Contains(rec.keys, entry.Name) {

			//try auto fix
			defaultValPattern, ok := entry.Pattern.(DefaultValuePattern)
			if ok {
				defaultValue, err := defaultValPattern.DefaultValue(ctx)
				if err != nil {
					goto missing
				}
				rec.keys = append(rec.keys, entry.Name)
				rec.values = append(rec.values, defaultValue.(Serializable))
				return nil
			}

		missing:
			missingRequiredProperties = append(missingRequiredProperties, entry.Name)
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
		s := it.ReadNumber()
		if strings.HasSuffix(string(s), ".0") {
			s = s[:len(s)-2]
		}
		n, err := strconv.ParseInt(string(s), 10, 64)

		if err != nil {
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

	if pattern != nil && !pattern.Test(ctx, integer) {
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
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, fmt.Errorf("failed to parse float: %w", it.Error)
	}

	if patt, ok := pattern.(*FloatRangePattern); ok && !patt.Test(ctx, float) {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
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
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, BYTE_COUNT_UNIT)

	if it.Error != nil && it.Error != io.EOF {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, fmt.Errorf("failed to parse byte count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
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
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, fmt.Errorf("invalid unit")
	}

	s = strings.TrimSuffix(s, RUNE_COUNT_UNIT)

	if it.Error != nil && it.Error != io.EOF {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, fmt.Errorf("failed to parse rune count: %w", it.Error)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, fmt.Errorf("failed to parse rune count: %w", err)
	}
	return RuneCount(i), nil
}

func parseFrequencyJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (_ Frequency, finalErr error) {
	if it.WhatIsNext() != jsoniter.NumberValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}

	freq := Frequency(it.ReadFloat64())

	err := freq.Validate()
	if err != nil {
		return 0, err
	}

	return Frequency(freq), nil
}

func parseByteRateJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (ByteRate, error) {
	if it.WhatIsNext() != jsoniter.NumberValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}

	number := it.ReadNumber()
	n, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to parse byte rate (integer): %w", err)
	}

	rate := ByteRate(n)
	err = rate.Validate()
	if err != nil {
		return 0, err
	}

	return rate, nil
}

func parseDurationJSONRepresentation(ctx *Context, it *jsoniter.Iterator, pattern Pattern, try bool) (Duration, error) {
	if it.WhatIsNext() != jsoniter.NumberValue {
		if try {
			return 0, ErrTriedToParseJSONRepr
		}
		return 0, ErrJsonNotMatchingSchema
	}

	d := Duration(it.ReadFloat64() * float64(time.Second))

	err := d.Validate()
	if err != nil {
		return 0, err
	}

	return d, nil
}

func parsePathJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ Path, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	pth := Path(it.ReadString())
	if err := pth.Validate(); err != nil {
		return "", err
	}
	return pth, nil
}

func parseSchemeJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ Scheme, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	scheme := Scheme(it.ReadString())
	if err := scheme.Validate(); err != nil {
		return "", err
	}
	return scheme, nil
}

func parseHostJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ Host, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	host := Host(it.ReadString())
	if err := host.Validate(); err != nil {
		return "", err
	}
	return host, nil
}

func parseURLJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ URL, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	url := URL(it.ReadString())
	if err := url.Validate(); err != nil {
		if err == ErrMissingURLSpecificFeature {
			//fix
			return url + "/", nil
		}
		return "", err
	}
	return url, nil
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

func parsePropNameJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ PropertyName, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return "", ErrTriedToParseJSONRepr
		}
		return "", ErrJsonNotMatchingSchema
	}

	name := PropertyName(it.ReadString())
	if err := name.Validate(); err != nil {
		return "", err
	}
	return name, nil
}

func parseLongValuePathJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *LongValuePath, finalErr error) {
	if it.WhatIsNext() != jsoniter.ArrayValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	var segments []ValuePathSegment
	index := -1

	it.ReadArrayCB(func(it *jsoniter.Iterator) bool {
		index++

		val, err := ParseNextJSONRepresentation(ctx, it, nil, try)
		if err != nil {
			if try {
				finalErr = ErrTriedToParseJSONRepr
				return false
			}

			finalErr = fmt.Errorf("failed to parse segment of long value path (index %d): %w", index, err)
			return false
		}
		segment, ok := val.(ValuePathSegment)
		if !ok {
			finalErr = fmt.Errorf("unexpected non-segment value in long value path (index %d)", index)
			return false
		}
		segments = append(segments, segment)
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	p := LongValuePath(segments)
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func parseNamedSegmentPathPatternSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *NamedSegmentPathPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	s := it.ReadString()
	//TODO: add checks
	//should the node spans be persisted ? The source file can change in the future ...

	node, _ := parse.ParseExpression(s)
	lit, ok := node.(*parse.NamedSegmentPathPatternLiteral)
	if !ok {
		return nil, errors.New("invalid JSON representation of a named segment path pattern")
	}
	return NewNamedSegmentPathPattern(lit), nil
}

func parseTypePatternSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *TypePattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.StringValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	s := it.ReadString()
	pattern, ok := DEFAULT_NAMED_PATTERNS[s]
	if !ok {
		return nil, fmt.Errorf("%q is not a default named pattern", s)
	}

	typePattern, ok := pattern.(*TypePattern)
	if !ok {
		return nil, fmt.Errorf("%q is not a type pattern", s)
	}

	return typePattern, nil
}

func parseObjectPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *ObjectPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	var inexact bool = true
	var entries []ObjectPatternEntry

	it.ReadObjectCB(func(it *jsoniter.Iterator, s string) bool {
		switch s {
		case SERIALIZED_OBJECT_PATTERN_INEXACT_KEY: //inexactness
			if it.WhatIsNext() != jsoniter.BoolValue {
				finalErr = errors.New("invalid representation of object pattern's inexactness")
				return false
			}
			inexact = it.ReadBool()
			return true
		case SERIALIZED_OBJECT_PATTERN_ENTRIES_KEY: //entries
			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of object pattern entries")
				return false
			}
			//read entries
			it.ReadObjectCB(func(it *jsoniter.Iterator, entryName string) bool {
				entry, err := parseObjectPatternEntryJSONRepresentation(ctx, it, entryName)
				if err != nil {
					finalErr = err
					return false
				}
				entries = append(entries, entry)
				return true
			})

			if finalErr != nil {
				return false
			}

			if it.Error == io.EOF {
				finalErr = errors.New("unterminated object pattern representation")
				return false
			}

			if it.Error != nil {
				return false
			}

			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in object pattern representation", s)
			return false
		}
	})

	if finalErr != nil {
		return nil, finalErr
	}

	if it.Error != nil && it.Error != io.EOF {
		return nil, it.Error
	}

	return NewObjectPattern(inexact, entries), nil
}

func parseObjectPatternEntryJSONRepresentation(ctx *Context, it *jsoniter.Iterator, name string) (entry ObjectPatternEntry, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		finalErr = fmt.Errorf("invalid representation of an object pattern entry (name: %q)", name)
		return
	}

	it.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		switch key {
		case SERIALIZED_OBJECT_PATTERN_ENTRY_PATTERN_KEY:
			pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = err
				return false
			}
			var ok bool
			entry.Pattern, ok = pattern.(Pattern)
			if !ok {
				finalErr = fmt.Errorf("invalid pattern for object pattern entry (entry name: %q)", name)
				return false
			}
			return true
		case SERIALIZED_OBJECT_PATTERN_ENTRY_IS_OPTIONAL_KEY:
			if it.WhatIsNext() != jsoniter.BoolValue {
				finalErr = fmt.Errorf("invalid optionality for object pattern entry (entry name: %q)", name)
				return false
			}
			entry.IsOptional = it.ReadBool()
			return true
		case SERIALIZED_OBJECT_PATTERN_ENTRY_REQ_KEYS_KEY:
			if it.WhatIsNext() != jsoniter.ArrayValue {
				finalErr = fmt.Errorf("invalid required key list for object pattern entry (entry name: %q)", name)
				return false
			}
			it.ReadArrayCB(func(it *jsoniter.Iterator) bool {
				if it.WhatIsNext() != jsoniter.StringValue {
					finalErr = fmt.Errorf("invalid required key for object pattern entry (entry name: %q)", name)
					return false
				}
				entry.Dependencies.RequiredKeys = append(entry.Dependencies.RequiredKeys, it.ReadString())
				return true
			})
			return true
		case SERIALIZED_OBJECT_PATTERN_ENTRY_REQ_PATTERN_KEY:
			pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = err
				return false
			}
			var ok bool
			entry.Dependencies.Pattern, ok = pattern.(Pattern)
			if !ok {
				finalErr = fmt.Errorf("invalid required pattern for object pattern entry (entry name: %q)", name)
				return false
			}
			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in object pattern entry representation", key)
			return false
		}
	})

	if finalErr != nil {
		entry = ObjectPatternEntry{}
		return
	}

	return
}

func parseRecordPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *RecordPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	var inexact bool = true
	var entries []RecordPatternEntry

	it.ReadObjectCB(func(it *jsoniter.Iterator, s string) bool {
		switch s {
		case SERIALIZED_RECORD_PATTERN_INEXACT_KEY: //inexactness
			if it.WhatIsNext() != jsoniter.BoolValue {
				finalErr = errors.New("invalid representation of record pattern's inexactness")
				return false
			}
			inexact = it.ReadBool()
			return true
		case SERIALIZED_RECORD_PATTERN_ENTRIES_KEY: //entries
			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of record pattern entries")
				return false
			}
			//read entries
			it.ReadObjectCB(func(it *jsoniter.Iterator, entryName string) bool {
				entry, err := parseRecordPatternEntryJSONRepresentation(ctx, it, entryName)
				if err != nil {
					finalErr = err
					return false
				}
				entries = append(entries, entry)
				return true
			})

			if finalErr != nil {
				return false
			}

			if it.Error == io.EOF {
				finalErr = errors.New("unterminated record pattern representation")
				return false
			}

			if it.Error != nil {
				return false
			}

			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in record pattern representation", s)
			return false
		}
	})

	if finalErr != nil {
		return nil, finalErr
	}

	if it.Error != nil && it.Error != io.EOF {
		return nil, it.Error
	}

	return NewRecordPattern(inexact, entries), nil
}

func parseRecordPatternEntryJSONRepresentation(ctx *Context, it *jsoniter.Iterator, name string) (entry RecordPatternEntry, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		finalErr = fmt.Errorf("invalid representation of an record pattern entry (name: %q)", name)
		return
	}

	it.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		switch key {
		case SERIALIZED_RECORD_PATTERN_ENTRY_PATTERN_KEY:
			pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = err
				return false
			}
			var ok bool
			entry.Pattern, ok = pattern.(Pattern)
			if !ok {
				finalErr = fmt.Errorf("invalid pattern for record pattern entry (entry name: %q)", name)
				return false
			}
			return true
		case SERIALIZED_RECORD_PATTERN_ENTRY_IS_OPTIONAL_KEY:
			if it.WhatIsNext() != jsoniter.BoolValue {
				finalErr = fmt.Errorf("invalid optionality for record pattern entry (entry name: %q)", name)
				return false
			}
			entry.IsOptional = it.ReadBool()
			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in record pattern entry representation", key)
			return false
		}
	})

	if finalErr != nil {
		entry = RecordPatternEntry{}
		return
	}

	return
}

func parseListPatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *ListPattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	var (
		generalElement Pattern

		hasElements bool
		elements    []Pattern

		hasMinCount bool
		minCount    int

		hasMaxCount bool
		maxCount    int
	)

	it.ReadObjectCB(func(it *jsoniter.Iterator, s string) bool {
		switch s {
		case SERIALIZED_LIST_PATTERN_MIN_COUNT_KEY: //min count
			if hasElements {
				finalErr = errors.New("list pattern's minimal element count should not be present since elements are specified")
				return false
			}

			if it.WhatIsNext() != jsoniter.NumberValue {
				finalErr = errors.New("invalid representation of list pattern's minimum element count")
				return false
			}
			hasMinCount = true
			minCount = int(it.ReadInt32())
			return it.Error == nil
		case SERIALIZED_LIST_PATTERN_MAX_COUNT_KEY: //max count
			if hasElements {
				finalErr = errors.New("list pattern's maximum element count should not be present since elements are specified")
				return false
			}

			if it.WhatIsNext() != jsoniter.NumberValue {
				finalErr = errors.New("invalid representation of list pattern's maximum element count")
				return false
			}
			hasMaxCount = true
			maxCount = int(it.ReadInt32())
			return it.Error == nil
		case SERIALIZED_LIST_PATTERN_ELEMENTS_KEY: //known length

			if generalElement != nil {
				finalErr = errors.New("list pattern's elements should not be present since a general element is specified")
				return false
			}

			if hasMinCount {
				finalErr = errors.New("list pattern's elements should not be present since a minimal element count is specified")
				return false
			}

			if hasMaxCount {
				finalErr = errors.New("list pattern's elements should not be present since a maximal element count is specified")
				return false
			}

			if it.WhatIsNext() != jsoniter.ArrayValue {
				finalErr = errors.New("invalid representation of list pattern's elements")
				return false
			}
			hasElements = true

			index := -1
			it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
				index++
				pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
				if err != nil {
					finalErr = fmt.Errorf("invalid pattern for list pattern's element at index %d: %w", index, err)
					return false
				}

				element, ok := pattern.(Pattern)
				if !ok {
					finalErr = fmt.Errorf("invalid pattern for list pattern's element at index %d", index)
					return false
				}

				elements = append(elements, element)
				return true
			})

			return true
		case SERIALIZED_LIST_PATTERN_ELEMENT_KEY: //general element
			if hasElements {
				finalErr = errors.New("list pattern's general element should not be present since elements are specified")
				return false
			}

			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of list pattern's general element")
				return false
			}

			pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = err
				return false
			}

			var ok bool
			generalElement, ok = pattern.(Pattern)
			if !ok {
				finalErr = errors.New("invalid pattern for list pattern's general element")
			}
			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in list pattern representation", s)
			return false
		}
	})

	if finalErr != nil {
		return nil, finalErr
	}

	if it.Error != nil && it.Error != io.EOF {
		return nil, it.Error
	}

	if hasElements {
		return NewListPattern(elements), nil
	}
	patt := NewListPatternOf(generalElement)

	if hasMaxCount {
		patt = patt.WithMinMaxElements(minCount, maxCount)
	} else if hasMinCount {
		patt = patt.WithMinElements(minCount)
	}
	return patt, nil
}

func parseTuplePatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *TuplePattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	var (
		generalElement Pattern

		hasElements bool
		elements    []Pattern
	)

	it.ReadObjectCB(func(it *jsoniter.Iterator, s string) bool {
		switch s {
		case SERIALIZED_TUPLE_PATTERN_ELEMENTS_KEY: //known length

			if generalElement != nil {
				finalErr = errors.New("tuple pattern's elements should not be present since a general element is specified")
				return false
			}

			if it.WhatIsNext() != jsoniter.ArrayValue {
				finalErr = errors.New("invalid representation of tuple pattern's elements")
				return false
			}
			hasElements = true

			index := -1
			it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
				index++
				pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
				if err != nil {
					finalErr = fmt.Errorf("invalid pattern for tuple pattern's element at index %d: %w", index, err)
					return false
				}

				element, ok := pattern.(Pattern)
				if !ok {
					finalErr = fmt.Errorf("invalid pattern for tuple pattern's element at index %d", index)
					return false
				}

				elements = append(elements, element)
				return true
			})

			return true
		case SERIALIZED_TUPLE_PATTERN_ELEMENT_KEY: //general element
			if hasElements {
				finalErr = errors.New("tuple pattern's general element should not be present since elements are specified")
				return false
			}

			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of tuple pattern's general element")
				return false
			}

			pattern, err := ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = err
				return false
			}

			var ok bool
			generalElement, ok = pattern.(Pattern)
			if !ok {
				finalErr = errors.New("invalid pattern for tuple pattern's general element")
			}
			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in tuple pattern representation", s)
			return false
		}
	})

	if finalErr != nil {
		return nil, finalErr
	}

	if it.Error != nil && it.Error != io.EOF {
		return nil, it.Error
	}

	if hasElements {
		return NewTuplePattern(elements), nil
	}

	return NewTuplePatternOf(generalElement), nil
}

func parseExactValuePatternJSONRepresentation(ctx *Context, it *jsoniter.Iterator, try bool) (_ *ExactValuePattern, finalErr error) {
	val, err := ParseNextJSONRepresentation(ctx, it, nil, try)
	if err != nil {
		return nil, err
	}
	return NewExactValuePattern(val), nil
}

func parseSameTypeListJSONRepr[T any](
	ctx *Context,
	it *jsoniter.Iterator,
	listPattern *ListPattern,
	try bool,
	finalErr *error,
) (elements []T) {
	index := 0
	containedElementFound := listPattern.containedElement == nil

	it.ReadArrayCB(func(it *jsoniter.Iterator) bool {
		val, err := ParseNextJSONRepresentation(ctx, it, listPattern.generalElementPattern, try)
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

		if !containedElementFound && listPattern.containedElement.Test(ctx, val) {
			containedElementFound = true
		}

		return true
	})

	if !containedElementFound {
		*finalErr = errors.New("JSON array has no element matching the contained element pattern")
	}
	return
}

func parseListJSONrepresentation(ctx *Context, it *jsoniter.Iterator, pattern *ListPattern, try bool) (in *List, finalErr error) {
	if it.WhatIsNext() != jsoniter.ArrayValue {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	checkLength := func(length int) bool {
		if pattern == nil {
			return true
		}
		if minCount := pattern.MinElementCount(); length < minCount {
			if try {
				finalErr = ErrTriedToParseJSONRepr
				return false
			}
			finalErr = fmt.Errorf("JSON array has not enough elements (%d), at least %d element(s) were expected", length, minCount)
			return false
		} else if maxCount := pattern.MaxElementCount(); length > maxCount {
			if try {
				finalErr = ErrTriedToParseJSONRepr
				return false
			}
			finalErr = fmt.Errorf("JSON array too many elements (%d), at most %d element(s) were expected", length, maxCount)
			return false
		}

		return true
	}

	index := 0

	if pattern == nil {
		var elements []Serializable

		it.ReadArrayCB(func(it *jsoniter.Iterator) bool {
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

		if !checkLength(len(elements)) {
			return
		}

		return NewWrappedValueList(elements...), nil
	}

	if pattern.elementPatterns != nil {
		containedElementFound := pattern.containedElement == nil
		var elements []Serializable

		it.ReadArrayCB(func(it *jsoniter.Iterator) bool {
			if index >= len(pattern.elementPatterns) {
				elements = append(elements, nil)
				index++
				return true
			}

			elementPattern, ok := pattern.ElementPatternAt(index)
			if !ok {
				finalErr = fmt.Errorf("JSON array has unexpected element at index %d", index)
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

			if !containedElementFound && pattern.containedElement.Test(ctx, val) {
				containedElementFound = true
			}

			return true
		})

		if finalErr != nil {
			return nil, finalErr
		}

		if pattern.elementPatterns != nil && len(elements) < len(pattern.elementPatterns) {
			if try {
				finalErr = ErrTriedToParseJSONRepr
				return
			}
			return nil, fmt.Errorf(
				"JSON array has too many or not enough elements (%d), %d element(s) were expected", len(elements), len(pattern.elementPatterns))
		}

		if !containedElementFound {
			if try {
				finalErr = ErrTriedToParseJSONRepr
				return
			}
			return nil, errors.New("JSON array has no element matching the contained element pattern")
		}

		if !checkLength(len(elements)) {
			return
		}

		return NewWrappedValueList(elements...), nil
	} else {
		generalElementPattern := pattern.generalElementPattern
		//TODO: store floats in a FloatList.

		if _, isIntRangePattern := generalElementPattern.(*IntRangePattern); isIntRangePattern || generalElementPattern == INT_PATTERN {
			elements := parseSameTypeListJSONRepr[Int](ctx, it, pattern, try, &finalErr)
			if finalErr != nil {
				return nil, finalErr
			}
			if !checkLength(len(elements)) {
				return
			}

			return NewWrappedIntListFrom(elements), nil
		} else if generalElementPattern == BOOL_PATTERN {
			elements := parseSameTypeListJSONRepr[Bool](ctx, it, pattern, try, &finalErr)
			if finalErr != nil {
				return nil, finalErr
			}
			if !checkLength(len(elements)) {
				return
			}
			return NewWrappedBoolList(elements...), nil
		} else if _, ok := generalElementPattern.(StringPattern); ok || generalElementPattern == STRING_PATTERN || generalElementPattern == STR_PATTERN {
			elements := parseSameTypeListJSONRepr[StringLike](ctx, it, pattern, try, &finalErr)
			if finalErr != nil {
				return nil, finalErr
			}
			if !checkLength(len(elements)) {
				return
			}
			return NewWrappedStringListFrom(elements), nil
		} //else
		elements := parseSameTypeListJSONRepr[Serializable](ctx, it, pattern, try, &finalErr)
		if finalErr != nil {
			return nil, finalErr
		}

		if !checkLength(len(elements)) {
			return
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
	it.ReadArrayCB(func(it *jsoniter.Iterator) bool {
		if pattern.elementPatterns != nil && index >= len(pattern.elementPatterns) {
			elements = append(elements, nil)
			index++
			return true
		}

		elementPattern, ok := pattern.ElementPatternAt(index)
		if !ok {
			finalErr = fmt.Errorf("JSON array has unexpected element at index %d", index)
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

	if pattern.elementPatterns != nil && len(elements) != len(pattern.elementPatterns) {
		if try {
			return nil, ErrTriedToParseJSONRepr
		}

		elemCount := len(elements)
		if elemCount < len(pattern.elementPatterns) {
			return nil, fmt.Errorf("JSON array has not enough elements (%d), %d element(s) were expected", elemCount, len(pattern.elementPatterns))
		} else {
			return nil, fmt.Errorf("JSON array has too many elements (%d), %d element(s) were expected", elemCount, len(pattern.elementPatterns))
		}
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
		if try {
			return nil, ErrTriedToParseJSONRepr
		}
		return nil, ErrJsonNotMatchingSchema
	}

	if err != nil {
		return nil, err
	}

	for _, otherCases := range pattern.cases[1:] {
		if !otherCases.Test(ctx, value) {
			if try {
				return nil, ErrTriedToParseJSONRepr
			}
			return nil, ErrJsonNotMatchingSchema
		}
	}
	return value, nil
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return (c >= '0' && c <= '9')
}

func countPrevBackslashes(s []byte, i int) int {
	index := i - 1
	count := 0
	for ; index >= 0; index-- {
		if s[index] == '\\' {
			count += 1
		} else {
			break
		}
	}

	return count
}
