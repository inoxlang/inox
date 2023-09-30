package core

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func init() {
	utils.PanicIfErr(json.Unmarshal([]byte(jsonDraft7String), &jsonDraft7))
}

func TestConvertJsonSchemaToPattern(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	runTestSuites := func(t *testing.T, suites []jsonDrafTestSuite) {
		for _, testSuite := range suites {
			t.Run(testSuite.Description, func(t *testing.T) {
				pattern, err := ConvertJsonSchemaToPattern(string(testSuite.Schema))
				if !assert.NoError(t, err) {
					return
				}

				for _, test := range testSuite.Tests {
					t.Run(test.Description, func(t *testing.T) {

						result, err := ParseJSONRepresentation(ctx, string(test.Data), pattern)
						if test.Valid {
							if !assert.NoError(t, err) {
								return
							}
							assert.NotNil(t, result)
						} else {
							if !assert.Error(t, err) {
								return
							}
						}
					})
				}
			})
		}
	}

	t.Run("AllOf", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.AllOf)
	})

	t.Run("AnyOf", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.AnyOf)
	})

	t.Run("BooleanSchema", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.BooleanSchema)
	})

	t.Run("Const", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.Const)
	})

	t.Run("Contains", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.Contains)
	})

	t.Run("Definitions", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.Definitions)
	})

	t.Run("Dependencies", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Dependencies)
	})
}

//go:embed testdata/json-draft7.json
var jsonDraft7String string
var jsonDraft7 struct {
	AdditionalItems       []jsonDrafTestSuite `json:"additionalItems"`
	AdditionalProperties  []jsonDrafTestSuite `json:"additionalProperties"`
	AllOf                 []jsonDrafTestSuite `json:"allOf"`
	AnyOf                 []jsonDrafTestSuite `json:"anyOf"`
	BooleanSchema         []jsonDrafTestSuite `json:"boolean_schema"`
	Const                 []jsonDrafTestSuite `json:"const"`
	Contains              []jsonDrafTestSuite `json:"contains"`
	Definitions           []jsonDrafTestSuite `json:"definitions"`
	Dependencies          []jsonDrafTestSuite `json:"dependencies"`
	Enum                  []jsonDrafTestSuite `json:"enum"`
	ExclusiveMaximum      []jsonDrafTestSuite `json:"exclusiveMaximum"`
	ExclusiveMinimum      []jsonDrafTestSuite `json:"exclusiveMinimum"`
	Format                []jsonDrafTestSuite `json:"format"`
	Id                    []jsonDrafTestSuite `json:"id"`
	IfThenElse            []jsonDrafTestSuite `json:"if-then-else"`
	InfiniteLoopRecursion []jsonDrafTestSuite `json:"infinite-loop-recursion"`
	Items                 []jsonDrafTestSuite `json:"items"`
	MaxItems              []jsonDrafTestSuite `json:"maxItems"`
	MaxLength             []jsonDrafTestSuite `json:"maxLength"`
	MaxProperties         []jsonDrafTestSuite `json:"maxProperties"`
	Maximum               []jsonDrafTestSuite `json:"maximum"`
	MinItems              []jsonDrafTestSuite `json:"minItems"`
	MinLength             []jsonDrafTestSuite `json:"minLength"`
	MinProperties         []jsonDrafTestSuite `json:"minProperties"`
	Minimum               []jsonDrafTestSuite `json:"minimum"`
	MultipleOf            []jsonDrafTestSuite `json:"multipleOf"`
	Not                   []jsonDrafTestSuite `json:"not"`
	OneOf                 []jsonDrafTestSuite `json:"oneOf"`
	Pattern               []jsonDrafTestSuite `json:"pattern"`
	PatternProperties     []jsonDrafTestSuite `json:"patternProperties"`
	Properties            []jsonDrafTestSuite `json:"properties"`
	PropertyNames         []jsonDrafTestSuite `json:"propertyNames"`
	Ref                   []jsonDrafTestSuite `json:"ref"`
	RefRemote             []jsonDrafTestSuite `json:"refRemote"`
	Required              []jsonDrafTestSuite `json:"required"`
	Type                  []jsonDrafTestSuite `json:"type"`
	UniqueItems           []jsonDrafTestSuite `json:"uniqueItems"`
	UnknownKeywords       []jsonDrafTestSuite `json:"unknownKeywords"`
}

type jsonDrafTestSuite struct {
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Tests       []jsonDraftTest `json:"tests"`
}

type jsonDraftTest struct {
	Description string          `json:"description"`
	Valid       bool            `json:"valid"`
	Data        json.RawMessage `json:"data"`
}
