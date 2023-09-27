package core

import (
	"errors"
	"math"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var (
	ErrInvalidOrUnsupportedJsonSchema = errors.New("invalid or unsupported JSON Schema")

	JSON_SCHEMA_TYPE_TO_PATTERN = map[string]Pattern{
		"string":  STRLIKE_PATTERN,
		"number":  FLOAT_PATTERN,
		"integer": INT_PATTERN,
		"object":  OBJECT_PATTERN,
		"array":   LIST_PATTERN,
		"boolean": BOOL_PATTERN,
		"null":    NIL_PATTERN,
	}
)

func init() {
	//remove loader for file: URIs
	clear(jsonschema.Loaders)
}

func ConvertJsonSchemaToPattern(schemaBytes string) (Pattern, error) {
	compiler := jsonschema.NewCompiler()
	url := "schema.json"

	if err := compiler.AddResource(url, strings.NewReader(schemaBytes)); err != nil {
		return nil, err
	}

	schema, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}

	return convertJsonSchemaToPattern(schema, nil, false)
}

func convertJsonSchemaToPattern(schema *jsonschema.Schema, baseSchema *jsonschema.Schema, ignoreSpecial bool) (Pattern, error) {
	//baseSchema is the parent schema of children of not, allOf, anyOf, oneOf, it is nil in other cases.
	//We choose to ignore

	if len(schema.PatternProperties) > 0 {
		return nil, errors.New("pattern properties not supported yet")
	}

	// any or never
	if schema.Always != nil {
		if *schema.Always {
			return ANYVAL_PATTERN, nil
		}
		return NEVER_PATTERN, nil
	}

	if !ignoreSpecial {
		if schema.Not != nil {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true)
			if err != nil {
				return nil, err
			}

			negation, err := convertJsonSchemaToPattern(schema.Not, schema, false)
			if err != nil {
				return nil, err
			}

			return NewDifferencePattern(basePattern, negation), nil
		}

		if len(schema.AllOf) > 0 {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true)
			if err != nil {
				return nil, err
			}

			intersectionCases := []Pattern{basePattern}

			for _, t := range schema.AllOf {
				case_, err := convertJsonSchemaToPattern(t, schema, false)
				if err != nil {
					return nil, err
				}

				intersectionCases = append(intersectionCases, case_)
			}

			return NewIntersectionPattern(intersectionCases, nil), nil
		}

		if len(schema.AnyOf) > 0 {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true)
			if err != nil {
				return nil, err
			}

			var unionCases []Pattern
			for _, t := range schema.AnyOf {
				case_, err := convertJsonSchemaToPattern(t, schema, false)
				if err != nil {
					return nil, err
				}

				unionCases = append(unionCases, case_)
			}

			unionPattern := NewUnionPattern(unionCases, nil)

			if basePattern == ANYVAL_PATTERN {
				return unionPattern, nil
			}
			return NewIntersectionPattern([]Pattern{basePattern, unionPattern}, nil), nil
		}

		if len(schema.OneOf) > 0 {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true)
			if err != nil {
				return nil, err
			}

			var disjointUnionCases []Pattern
			for _, t := range schema.OneOf {
				case_, err := convertJsonSchemaToPattern(t, schema, false)
				if err != nil {
					return nil, err
				}

				disjointUnionCases = append(disjointUnionCases, case_)
			}

			disjointUnionPattern := NewDisjointUnionPattern(disjointUnionCases, nil)

			if basePattern == ANYVAL_PATTERN {
				return disjointUnionPattern, nil
			}

			return NewIntersectionPattern([]Pattern{basePattern, disjointUnionPattern}, nil), nil
		}

		if schema.If != nil || schema.Then != nil || schema.Else != nil {
			return nil, errors.New("conditional schemas are not supported yet")
		}
	}

	if len(schema.Constant) > 0 {
		constant := schema.Constant[0]
		value := ConvertJSONValToInoxVal(constant, true)
		return NewExactValuePattern(value), nil
	}

	if len(schema.Enum) > 0 {
		unionCases := make([]Pattern, len(schema.Enum))
		for i, jsonVal := range schema.Enum {
			unionCases[i] = NewExactValuePattern(ConvertJSONValToInoxVal(jsonVal, false))
		}

		return NewUnionPattern(unionCases, nil), nil
	}

	allowNumber := false
	allowInteger := false
	allowNull := false
	allowBoolean := false
	allowString := false
	allowObject := false
	allowArray := false

	var unionCases []Pattern

	switch len(schema.Types) {
	case 0:
		if schema.Maximum != nil || schema.Minimum != nil || schema.MultipleOf != nil {
			allowNumber = true
		}

		if schema.Pattern != nil || schema.Format != "" || schema.MinLength != -1 || schema.MaxLength != -1 {
			//TODO: .Format is a type agnostic keyword
			allowString = true
		}

		//note: schema.ContainsEval true for objects, why ?

		if schema.AdditionalItems != nil || schema.Items != nil || schema.MinItems >= 0 ||
			schema.MaxItems >= 0 || schema.PrefixItems != nil || schema.Contains != nil ||
			schema.MinContains != 1 || schema.MaxContains >= 0 ||
			schema.UnevaluatedItems != nil {
			allowArray = true
		}

		if schema.MinProperties >= 0 || schema.MaxProperties >= 0 || schema.Required != nil ||
			schema.Properties != nil || schema.PropertyNames != nil || schema.RegexProperties ||
			schema.PatternProperties != nil || schema.AdditionalProperties != nil ||
			schema.Dependencies != nil || schema.DependentRequired != nil {
			allowObject = true
		}
	default:
		for _, typename := range schema.Types {
			switch typename {
			case "number":
				allowNumber = true
			case "integer":
				allowInteger = true
			case "null":
				allowNull = true
			case "boolean":
				allowBoolean = true
			case "string":
				allowString = true
			case "array":
				allowArray = true
			case "object":
				allowObject = true
			}
		}
	}

	if !(allowNumber || allowInteger || allowNull || allowString || allowArray || allowObject) {
		return ANYVAL_PATTERN, nil
	}

	if allowNumber {
		floatRange := FloatRange{
			Start: math.Inf(-1),
			End:   math.Inf(1),
		}

		if schema.Minimum != nil {
			min, _ := schema.Minimum.Float64()
			floatRange.Start = min
		} else if schema.ExclusiveMinimum != nil {
			exclusiveMinimum, _ := schema.Minimum.Float64()
			floatRange.Start = math.Nextafter(exclusiveMinimum, math.Inf(1))
		}

		if schema.Maximum != nil {
			max, _ := schema.Maximum.Float64()
			floatRange.End = max
			floatRange.inclusiveEnd = true
		} else if schema.ExclusiveMaximum != nil {
			exclusiveMaximum, _ := schema.Maximum.Float64()
			floatRange.End = exclusiveMaximum
		}

		var multipleOf float64 = -1
		if schema.MultipleOf != nil {
			multipleOf, _ = schema.MultipleOf.Float64()
		}

		pattern := NewFloatRangePattern(floatRange, multipleOf)
		unionCases = append(unionCases, pattern)
	} else if allowInteger {
		var intRange IntRange

		if schema.Minimum != nil {
			min, _ := schema.Minimum.Float64()
			intRange.Start = int64(math.Floor(min))
		} else if schema.ExclusiveMinimum != nil {
			exclusiveMinimum, _ := schema.Minimum.Float64()
			min := math.Nextafter(exclusiveMinimum, math.Inf(1))
			intRange.Start = int64(math.Floor(min))
		}

		if schema.Maximum != nil {
			exclusiveMax, _ := schema.Maximum.Float64()
			intRange.inclusiveEnd = true
			intRange.End = int64(math.Ceil(exclusiveMax))
		} else if schema.ExclusiveMaximum != nil {
			max, _ := schema.Maximum.Float64()
			intRange.End = int64(math.Ceil(max))
		}

		var multipleOf int64 = -1
		if schema.MultipleOf != nil {
			n, _ := schema.MultipleOf.Float64()
			if !utils.IsWholeInt64(n) {
				return nil, errors.New("'multipleOf' should have an integer value")
			}
			multipleOf = int64(n)
		}

		var pattern Pattern
		if intRange == (IntRange{}) {
			pattern = INT_PATTERN
		} else {
			pattern = NewIntRangePattern(intRange, multipleOf)
		}
		unionCases = append(unionCases, pattern)
	}

	if allowNull {
		unionCases = append(unionCases, NIL_PATTERN)
	}

	if allowBoolean {
		unionCases = append(unionCases, BOOL_PATTERN)
	}

	if allowString {
		var pattern Pattern

		var lengthRange = IntRange{}
		var hasLengthRange bool

		if schema.MinLength != -1 {
			lengthRange.Start = int64(schema.MinLength)
			hasLengthRange = true

			if schema.MaxLength == -1 {
				lengthRange.End = math.MaxInt64
				lengthRange.inclusiveEnd = true
			}
		}

		if schema.MaxLength != -1 {
			lengthRange.End = int64(schema.MaxLength)
			lengthRange.inclusiveEnd = true
			hasLengthRange = true

			if schema.MinLength == -1 {
				lengthRange.Start = 0
			}
		}

		if schema.Pattern != nil {
			regexPattern := NewRegexPattern(schema.Pattern.String())

			if lengthRange != (IntRange{}) {
				regexPattern = regexPattern.WithLengthRange(lengthRange)
			}
			pattern = regexPattern
		} else if schema.Format != "" {
			if hasLengthRange {
				return nil, errors.New("'minLength' & 'maxLength' are not supported yet when 'format' is set")
			}

			switch schema.Format {
			case "time":
				//note: some informations are lost (length, ...)
				pattern = DATE_PATTERN
			case "duration":
				//note: some informations are lost (length, ...)
				pattern = DURATION_PATTERN
			case "email-address":
				//note: some informations are lost (length, ...)
				pattern = EMAIL_ADDR_PATTERN
			case "uri":
				pattern = NewUnionPattern([]Pattern{URL_PATTERN, STRLIKE_PATTERN}, nil)
			}
		}

		if pattern == nil {
			if hasLengthRange {
				pattern = NewLengthCheckingStringPattern(lengthRange.Start, lengthRange.InclusiveEnd())
			} else {
				pattern = STRLIKE_PATTERN
			}
		}

		unionCases = append(unionCases, pattern)
	}

	if allowObject {
		if schema.MinProperties >= 0 || schema.MaxProperties >= 0 {
			return nil, errors.New("'minProperties' & 'maxProperties' are not supported")
		}

		if schema.RegexProperties {
			return nil, errors.New("'regexProperties' is not supported")
		}

		if len(schema.PatternProperties) > 0 {
			return nil, errors.New("'patternProperties' is not supported yet")
		}

		if schema.UnevaluatedProperties != nil {
			return nil, errors.New("'unevaluatedProperties' is not supported yet")
		}

		if len(schema.DependentRequired) > 0 || len(schema.DependentSchemas) > 0 {
			return nil, errors.New("'dependentRequired' & 'dependentSchemas' are not supported yet")
		}

		if len(schema.Dependencies) > 0 {
			return nil, errors.New("'dependencies' is not supported yet")
		}

		exact := !schema.RegexProperties && len(schema.PatternProperties) == 0

		switch v := schema.AdditionalProperties.(type) {
		case *jsonschema.Schema:
			return nil, errors.New("'additionalProperties' is not fully supported yet, only 'additionalProperties': <bool> is allowed")
		case bool:
			if v {
				exact = false
			}
		case nil:
			exact = false
		}

		entries := make(map[string]Pattern)
		var optionalProperties map[string]struct{}

		for name, propSchema := range schema.Properties {
			propPattern, err := convertJsonSchemaToPattern(propSchema, nil, false)
			if err != nil {
				return nil, err
			}

			entries[name] = propPattern

			required := false
			for _, requiredPropName := range schema.Required {
				if requiredPropName == name {
					required = true
				}
			}

			if !required {
				if optionalProperties == nil {
					optionalProperties = map[string]struct{}{}
				}
				optionalProperties[name] = struct{}{}
			}
		}

		objectPattern := NewObjectPatternWithOptionalProps(!exact, entries, optionalProperties)
		unionCases = append(unionCases, objectPattern)
	}

	if allowArray {
		if (schema.MinItems > 0 && schema.MinItems != schema.MaxItems) ||
			(schema.MaxItems == 0 && schema.MaxItems != -1) {
			return nil, errors.New("'minItems' & 'maxItems' are not full supported yet")
		}

		//TODO: support schema.Items2020

		if schema.UniqueItems {
			return nil, errors.New("'uniqueItems' is not supported yet")
		}

		if schema.UnevaluatedItems != nil {
			return nil, errors.New("'UnevaluatedItems' is not supported yet")
		}

		//TODO: schema.ContainsEval ?

		switch schema.Items.(type) {
		case nil:
		default:
			return nil, errors.New("'additionalItems' is not supported yet")
		}

		var generalElementPattern Pattern = SERIALIZABLE_PATTERN
		var elementPatterns []Pattern

		switch items := schema.Items.(type) {
		case *jsonschema.Schema:
			var err error
			generalElementPattern, err = convertJsonSchemaToPattern(items, nil, false)
			if err != nil {
				return nil, err
			}
		case []*jsonschema.Schema:
			for _, item := range items {
				elementPattern, err := convertJsonSchemaToPattern(item, nil, false)
				if err != nil {
					return nil, err
				}
				elementPatterns = append(elementPatterns, elementPattern)
			}
		case nil:
			if schema.Contains != nil {
				var err error
				generalElementPattern, err = convertJsonSchemaToPattern(schema.Contains, nil, false)
				if err != nil {
					return nil, err
				}
			}
		}

		var listPattern Pattern
		if elementPatterns != nil {
			listPattern = NewListPattern(elementPatterns)
		} else {
			listPattern = NewListPatternOf(generalElementPattern)
		}

		unionCases = append(unionCases, listPattern)
	}

	if len(unionCases) == 1 {
		return unionCases[0], nil
	}

	return NewUnionPattern(unionCases, nil), nil
}

func convertJsonSchemaTypeToPattern(typename string) (Pattern, error) {
	pattern, ok := JSON_SCHEMA_TYPE_TO_PATTERN[typename]
	if ok {
		return pattern, nil
	}
	return nil, ErrInvalidOrUnsupportedJsonSchema
}
