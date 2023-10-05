package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var (
	ErrInvalidOrUnsupportedJsonSchema       = errors.New("invalid or unsupported JSON Schema")
	ErrRecursiveJSONSchemaNotSupported      = errors.New("recursive JSON schema are not supported")
	ErrJSONSchemaMixingIntFloatNotSupported = errors.New("JSON schemas mixing integers and floats are not supported")

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

// ConvertJsonSchemaToPattern converts a JSON schema definition to an Inox pattern,
// all schemas are not supported and the resulting pattern might be stricter.
func ConvertJsonSchemaToPattern(schemaBytes string) (Pattern, error) {
	compiler := jsonschema.NewCompiler()
	url := "schema.json"

	if err := compiler.AddResource(url, strings.NewReader(schemaBytes)); err != nil {
		return nil, err
	}
	compiler.Draft = jsonschema.Draft7

	schema, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}

	return convertJsonSchemaToPattern(schema, nil, false, false, 0)
}

func convertJsonSchemaToPattern(schema *jsonschema.Schema, baseSchema *jsonschema.Schema, ignoreSpecial bool, removeIgnore bool, depth int) (Pattern, error) {
	//baseSchema is the parent schema of children of not, allOf, anyOf, oneOf, it is nil in other cases.
	//We choose to ignore

	if depth > 10 {
		return nil, ErrRecursiveJSONSchemaNotSupported
	}

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

	if schema.Ref != nil {
		return convertJsonSchemaToPattern(schema.Ref, nil, false, removeIgnore, depth+1)
	}

	if !ignoreSpecial {
		if schema.Not != nil {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true, false, depth+1)
			if err != nil {
				return nil, err
			}

			negation, err := convertJsonSchemaToPattern(schema.Not, schema, false, false, depth+1)
			if err != nil {
				return nil, err
			}

			return NewDifferencePattern(basePattern, negation), nil
		}

		if len(schema.AllOf) > 0 {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true, false, depth+1)
			if err != nil {
				return nil, err
			}

			var intersectionCases []Pattern

			if basePattern != ANYVAL_PATTERN {
				intersectionCases = append(intersectionCases, basePattern)
			}

			hasIntPattern := false
			hasFloatPattern := false

			for _, t := range schema.AllOf {
				case_, err := convertJsonSchemaToPattern(t, schema, false, true, depth+1)
				if err != nil {
					return nil, err
				}

				isFloatPattern := isFloatPattern(case_)
				isIntPattern := isIntPattern(case_)

				if hasIntPattern && isFloatPattern {
					return nil, ErrJSONSchemaMixingIntFloatNotSupported
				} else if hasFloatPattern && isIntPattern {
					return nil, ErrJSONSchemaMixingIntFloatNotSupported
				}

				hasIntPattern = isIntPattern
				hasFloatPattern = isFloatPattern

				intersectionCases = append(intersectionCases, case_)
			}

			if len(intersectionCases) == 1 {
				return intersectionCases[0], nil
			}

			return NewIntersectionPattern(intersectionCases, nil), nil
		}

		if len(schema.AnyOf) > 0 {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true, false, depth+1)
			if err != nil {
				return nil, err
			}

			var unionCases []Pattern
			hasIntPattern := false
			hasFloatPattern := false

			for _, t := range schema.AnyOf {
				case_, err := convertJsonSchemaToPattern(t, schema, false, true, depth+1)
				if err != nil {
					return nil, err
				}

				isFloatPattern := isFloatPattern(case_)
				isIntPattern := isIntPattern(case_)

				if hasIntPattern && isFloatPattern {
					return nil, ErrJSONSchemaMixingIntFloatNotSupported
				} else if hasFloatPattern && isIntPattern {
					return nil, ErrJSONSchemaMixingIntFloatNotSupported
				}

				hasIntPattern = isIntPattern
				hasFloatPattern = isFloatPattern

				unionCases = append(unionCases, case_)
			}

			unionPattern := NewUnionPattern(unionCases, nil)

			if basePattern == ANYVAL_PATTERN {
				if len(unionCases) == 1 {
					return unionCases[0], nil
				}
				return unionPattern, nil
			}
			return NewIntersectionPattern([]Pattern{basePattern, unionPattern}, nil), nil
		}

		if len(schema.OneOf) > 0 {
			basePattern, err := convertJsonSchemaToPattern(schema, nil, true, false, depth+1)
			if err != nil {
				return nil, err
			}

			var disjointUnionCases []Pattern
			hasIntPattern := false
			hasFloatPattern := false

			for _, t := range schema.OneOf {
				case_, err := convertJsonSchemaToPattern(t, schema, false, true, depth+1)
				if err != nil {
					return nil, err
				}

				isFloatPattern := isFloatPattern(case_)
				isIntPattern := isIntPattern(case_)

				if hasIntPattern && isFloatPattern {
					return nil, ErrJSONSchemaMixingIntFloatNotSupported
				} else if hasFloatPattern && isIntPattern {
					return nil, ErrJSONSchemaMixingIntFloatNotSupported
				}

				hasIntPattern = isIntPattern
				hasFloatPattern = isFloatPattern

				disjointUnionCases = append(disjointUnionCases, case_)
			}

			disjointUnionPattern := NewDisjointUnionPattern(disjointUnionCases, nil)

			if basePattern == ANYVAL_PATTERN {
				return disjointUnionPattern, nil
			}

			return simplifyIntersection([]Pattern{basePattern, disjointUnionPattern}), nil
		}

		if schema.If != nil || schema.Then != nil || schema.Else != nil {
			return nil, errors.New("conditional schemas are not supported yet")
		}
	}

	if len(schema.Constant) > 0 {
		constant := schema.Constant[0]

		return convertConstSchemaValueToPattern(constant)
	}

	if len(schema.Enum) > 0 {
		var err error

		unionCases := make([]Pattern, len(schema.Enum))
		for i, constant := range schema.Enum {
			unionCases[i], err = convertConstSchemaValueToPattern(constant)
			if err != nil {
				return nil, err
			}
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

	ignoreNonArray := false
	ignoreNonNumber := false
	ignoreNonString := false
	ignoreNonObject := false

	var unionCases []Pattern

	switch len(schema.Types) {
	case 0:
		if schema.Contains != nil {
			ignoreNonArray = true
			allowArray = schema.Contains.Always == nil || *schema.Contains.Always
			break
		}

		if schema.MinItems >= 0 || schema.MaxItems >= 0 {
			ignoreNonArray = true
			allowArray = true
		}

		if schema.Maximum != nil || schema.ExclusiveMaximum != nil || schema.Minimum != nil ||
			schema.ExclusiveMinimum != nil || schema.MultipleOf != nil {
			ignoreNonNumber = true
			allowNumber = true
		}

		if schema.Pattern != nil || schema.Format != "" || schema.MinLength != -1 || schema.MaxLength != -1 {
			ignoreNonString = true
			allowString = true
		}

		//note: schema.ContainsEval true for objects, why ?

		if schema.AdditionalItems != nil || schema.Items != nil ||
			schema.MaxItems >= 0 || schema.PrefixItems != nil || schema.Contains != nil ||
			schema.MinContains != 1 || schema.MaxContains >= 0 ||
			schema.UnevaluatedItems != nil {
			allowArray = true
		}

		if schema.Dependencies != nil {
			allowNumber = true
			allowInteger = true
			allowNull = true
			allowBoolean = true
			allowString = true
			allowObject = true
			allowArray = true
			break
		}

		if schema.MinProperties >= 0 || schema.MaxProperties >= 0 || schema.Required != nil ||
			schema.Properties != nil || schema.PropertyNames != nil || schema.RegexProperties ||
			schema.PatternProperties != nil || schema.AdditionalProperties != nil ||
			schema.DependentRequired != nil {
			ignoreNonObject = true
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

	if !(allowBoolean || allowNumber || allowInteger || allowNull || allowString || allowArray || allowObject) {
		return ANYVAL_PATTERN, nil
	}

	if allowNumber {
		hasSpecifiedRange := false
		floatRange := FloatRange{
			Start: math.Inf(-1),
			End:   math.Inf(1),
		}

		if schema.Minimum != nil {
			hasSpecifiedRange = true
			min, _ := schema.Minimum.Float64()
			floatRange.Start = min
		} else if schema.ExclusiveMinimum != nil {
			hasSpecifiedRange = true
			exclusiveMinimum, _ := schema.ExclusiveMinimum.Float64()
			floatRange.Start = math.Nextafter(exclusiveMinimum, math.Inf(1))
		}

		if schema.Maximum != nil {
			hasSpecifiedRange = true
			max, _ := schema.Maximum.Float64()
			floatRange.End = max
			floatRange.inclusiveEnd = true
		} else if schema.ExclusiveMaximum != nil {
			hasSpecifiedRange = true
			exclusiveMaximum, _ := schema.ExclusiveMaximum.Float64()

			if math.IsInf(exclusiveMaximum, 1) {
				floatRange.End = exclusiveMaximum
			} else {
				floatRange.End = math.Nextafter(exclusiveMaximum, math.Inf(-1))
			}
		}

		var multipleOf float64 = -1
		if schema.MultipleOf != nil {
			multipleOf, _ = schema.MultipleOf.Float64()
		}

		if multipleOf == -1 && !hasSpecifiedRange {
			unionCases = append(unionCases, FLOAT_PATTERN)
		} else {
			pattern := NewFloatRangePattern(floatRange, multipleOf)
			unionCases = append(unionCases, pattern)
		}
	} else if allowInteger {
		hasRange := false
		intRange := IntRange{
			Start:        math.MinInt64,
			End:          math.MaxInt64,
			inclusiveEnd: true,
			Step:         1,
		}

		if schema.Minimum != nil {
			min, _ := schema.Minimum.Float64()
			intRange.Start = int64(math.Floor(min))
			hasRange = true
		} else if schema.ExclusiveMinimum != nil {
			exclusiveMinimum, _ := schema.Minimum.Float64()
			min := math.Nextafter(exclusiveMinimum, math.Inf(1))
			intRange.Start = int64(math.Floor(min))
			hasRange = true
		}

		if schema.Maximum != nil {
			exclusiveMax, _ := schema.Maximum.Float64()
			intRange.inclusiveEnd = true
			intRange.End = int64(math.Ceil(exclusiveMax))
			hasRange = true
		} else if schema.ExclusiveMaximum != nil {
			max, _ := schema.Maximum.Float64()
			intRange.End = int64(math.Ceil(max))
			hasRange = true
		}

		var multipleOf int64 = -1
		var multipleOfFloat Float = -1

		if schema.MultipleOf != nil {
			n, _ := schema.MultipleOf.Float64()
			if utils.IsWholeInt64(n) {
				multipleOf = int64(n)
			} else {
				multipleOfFloat = Float(n)
			}
		}

		var pattern Pattern
		if !hasRange {
			pattern = INT_PATTERN
		} else if multipleOfFloat != -1 {
			pattern = NewIntRangePatternFloatMultiple(intRange, multipleOfFloat)
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

			if hasLengthRange {
				regexPattern = regexPattern.WithLengthRange(lengthRange)
			}
			pattern = regexPattern
		} else if schema.Format != "" {
			// if hasLengthRange {
			// 	return nil, errors.New("'minLength' & 'maxLength' are not supported yet when 'format' is set")
			// }

			// switch schema.Format {
			// case "time":
			// 	//note: some informations are lost (length, ...)
			// 	pattern = DATE_PATTERN
			// case "duration":
			// 	//note: some informations are lost (length, ...)
			// 	pattern = DURATION_PATTERN
			// case "email-address":
			// 	//note: some informations are lost (length, ...)
			// 	pattern = EMAIL_ADDR_PATTERN
			// case "uri":
			// 	pattern = NewUnionPattern([]Pattern{URL_PATTERN, STRLIKE_PATTERN}, nil)
			// }
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
		anyObject := true

		if schema.MinProperties >= 0 || schema.MaxProperties >= 0 {
			anyObject = false
			return nil, errors.New("'minProperties' & 'maxProperties' are not supported")
		}

		if schema.RegexProperties {
			anyObject = false
			return nil, errors.New("'regexProperties' is not supported")
		}

		if len(schema.PatternProperties) > 0 {
			anyObject = false
			return nil, errors.New("'patternProperties' is not supported yet")
		}

		if schema.UnevaluatedProperties != nil {
			anyObject = false
			return nil, errors.New("'unevaluatedProperties' is not supported yet")
		}

		if len(schema.DependentRequired) > 0 || len(schema.DependentSchemas) > 0 {
			anyObject = false
			return nil, errors.New("'dependentRequired' & 'dependentSchemas' are not supported yet")
		}

		exact := !schema.RegexProperties && len(schema.PatternProperties) == 0

		switch v := schema.AdditionalProperties.(type) {
		case *jsonschema.Schema:
			anyObject = false
			return nil, errors.New("'additionalProperties' is not fully supported yet, only 'additionalProperties': <bool> is allowed")
		case bool:
			anyObject = false
			if v {
				exact = false
			}
		case nil:
			exact = false
		}

		entries := make(map[string]Pattern)
		var optionalProperties map[string]struct{}

		for name, propSchema := range schema.Properties {
			anyObject = false

			propPattern, err := convertJsonSchemaToPattern(propSchema, nil, false, false, depth+1)
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

		for _, requiredPropName := range schema.Required {
			anyObject = false

			if _, ok := entries[requiredPropName]; !ok {
				entries[requiredPropName] = ANYVAL_PATTERN
			}
		}

		var dependencies map[string]propertyDependencies

		if len(schema.Dependencies) > 0 {
			anyObject = false

			dependencies = map[string]propertyDependencies{}

			for dependentKey, deps := range schema.Dependencies {
				switch d := deps.(type) {
				case []string:
					dependencies[dependentKey] = propertyDependencies{requiredKeys: d}
				case *jsonschema.Schema:
					if d.Always != nil {
						return nil, errors.New("'dependencies' with boolean schemas are not supported")
					}

					var propDependencies propertyDependencies

					if d.Properties == nil || d.AdditionalProperties != nil ||
						d.RegexProperties || d.UnevaluatedProperties != nil ||
						d.PatternProperties != nil || d.MaxProperties != -1 ||
						d.MinProperties != -1 || d.DependentRequired != nil {
						return nil, errors.New("'dependencies' with schemas are not fully supported")
					}

					dependenciesPattern, err := convertJsonSchemaToPattern(d, nil, false, false, depth+1)
					if err != nil {
						return nil, fmt.Errorf("failed to convert dependency pattern for property %q", dependentKey)
					}

					propDependencies.pattern = dependenciesPattern
					dependencies[dependentKey] = propDependencies

					//make sure the dependent key is present in the entries
					if _, ok := entries[dependentKey]; !ok {
						entries[dependentKey] = SERIALIZABLE_PATTERN
						if optionalProperties == nil {
							optionalProperties = map[string]struct{}{}
						}
						optionalProperties[dependentKey] = struct{}{}
					}
				default:
				}
			}
		}

		if anyObject {
			unionCases = append(unionCases, OBJECT_PATTERN)
		} else {
			objectPattern := NewObjectPatternWithOptionalProps(!exact, entries, optionalProperties)
			if len(dependencies) > 0 {
				objectPattern = objectPattern.WithDependencies(dependencies)
			}
			unionCases = append(unionCases, objectPattern)
		}
	}

	if allowArray {
		//TODO: support schema.Items2020

		if schema.UniqueItems {
			return nil, errors.New("'uniqueItems' is not supported yet")
		}

		if schema.UnevaluatedItems != nil {
			return nil, errors.New("'UnevaluatedItems' is not supported yet")
		}

		//TODO: schema.ContainsEval ?

		switch schema.AdditionalItems.(type) {
		case nil:
		default:
			return nil, errors.New("'additionalItems' is not supported yet")
		}

		var generalElementPattern Pattern = SERIALIZABLE_PATTERN
		var elementPatterns []Pattern

		switch items := schema.Items.(type) {
		case *jsonschema.Schema:
			var err error
			generalElementPattern, err = convertJsonSchemaToPattern(items, nil, false, false, depth+1)
			if err != nil {
				return nil, err
			}
		case []*jsonschema.Schema:
			for _, item := range items {
				elementPattern, err := convertJsonSchemaToPattern(item, nil, false, false, depth+1)
				if err != nil {
					return nil, err
				}
				elementPatterns = append(elementPatterns, elementPattern)
			}
		case nil:
			generalElementPattern = SERIALIZABLE_PATTERN
		}

		var listPattern *ListPattern
		if elementPatterns != nil {
			listPattern = NewListPattern(elementPatterns)
		} else {
			listPattern = NewListPatternOf(generalElementPattern)
		}

		if schema.MinItems != -1 || schema.MaxItems != -1 {
			minItems := schema.MinItems
			if minItems < 0 {
				minItems = 0
			}
			maxItems := schema.MaxItems
			if maxItems < 0 {
				maxItems = math.MaxInt64
			}
			listPattern = listPattern.WithMinMaxElements(minItems, maxItems)
		}

		if schema.Contains == nil {
			unionCases = append(unionCases, listPattern)
		} else if schema.Contains.Always == nil {
			containedElementPattern, err := convertJsonSchemaToPattern(schema.Contains, nil, false, false, depth+1)
			if err != nil {
				return nil, err
			}
			listPattern = listPattern.WithElement(containedElementPattern)
			unionCases = append(unionCases, listPattern)
		} else if *schema.Contains.Always {
			if listPattern.MinElementCount() == 0 {
				unionCases = append(unionCases, listPattern.WithMinElements(1))
			} else {
				unionCases = append(unionCases, listPattern)
			}
		} else {
			panic(ErrUnreachable)
		}
	}

	if len(unionCases) == 1 {
		if !removeIgnore {
			if ignoreNonArray {
				if !allowArray {
					return NewDifferencePattern(ANYVAL_PATTERN, LIST_PATTERN), nil
				}
				unionCases = append(unionCases, NewDifferencePattern(ANYVAL_PATTERN, LIST_PATTERN))
				return NewDisjointUnionPattern(unionCases, nil), nil
			} else if ignoreNonObject {
				unionCases = append(unionCases, NewDifferencePattern(ANYVAL_PATTERN, OBJECT_PATTERN))
				return NewDisjointUnionPattern(unionCases, nil), nil
			} else if ignoreNonNumber {
				unionCases = append(unionCases, NewDifferencePattern(ANYVAL_PATTERN, FLOAT_PATTERN))
				return NewDisjointUnionPattern(unionCases, nil), nil
			} else if ignoreNonString {
				unionCases = append(unionCases, NewDifferencePattern(ANYVAL_PATTERN, STRLIKE_PATTERN))
				return NewDisjointUnionPattern(unionCases, nil), nil
			}
		}

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

func convertConstSchemaValueToPattern(jsonValue any) (_ Pattern, err error) {
	switch c := jsonValue.(type) {
	case map[string]any:
		entries := map[string]Pattern{}

		for k, v := range c {
			if k == "$id" {
				return nil, errors.New("const values with an $id property are not supported yet")
			}
			entries[k], err = convertConstSchemaValueToPattern(v)
			if err != nil {
				return nil, err
			}
		}

		return NewExactObjectPattern(entries), nil
	case []any:
		var elements []Pattern

		for _, e := range c {
			p, err := convertConstSchemaValueToPattern(e)
			if err != nil {
				return nil, err
			}
			elements = append(elements, p)
		}

		return NewListPattern(elements), nil
	case json.Number:
		float, _ := c.Float64()
		return NewExactValuePattern(Float(float)), nil
	case float64:
		return NewExactValuePattern(Float(c)), nil
	case nil:
		return NIL_PATTERN, nil
	case bool:
		return NewExactValuePattern(Bool(c)), nil
	case string:
		return NewExactValuePattern(Str(c)), nil
	default:
		return nil, fmt.Errorf("cannot convert value of type %T to Inox Value", c)
	}
}
