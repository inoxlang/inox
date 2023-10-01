package core

import (
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
)

func init() {
	utils.PanicIfErr(json.Unmarshal([]byte(jsonDraft7String), &jsonDraft7))
}

//go:embed testdata/jsonschema/*
var jsonSchemaData embed.FS

func TestConvertJsonSchemaToPattern(t *testing.T) {

	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	runTestSuites := func(t *testing.T, suites []jsonDrafTestSuite, notSupportedTests [][2]string) {
		for _, testSuite := range suites {
			t.Run(testSuite.Description, func(t *testing.T) {
				supportedSuite := true
				for _, skippedTest := range notSupportedTests {
					if testSuite.Description == skippedTest[0] && skippedTest[1] == "*" {
						supportedSuite = false
						break
					}
				}

				clear(jsonschema.Loaders)
				jsonschema.Loaders["file"] = func(_url string) (io.ReadCloser, error) {
					u, err := url.Parse(_url)
					if err != nil {
						return nil, err
					}

					if u.Path == "schema.json" || u.Path == "/schema.json" {
						return io.NopCloser(strings.NewReader(string(testSuite.Schema))), nil
					}
					return jsonSchemaData.Open(u.Path)
				}
				jsonschema.Loaders["http"] = func(url string) (io.ReadCloser, error) {
					path := strings.TrimPrefix(url, "http://localhost:1234")
					if path == "" || path == "/" {
						return io.NopCloser(strings.NewReader(string(testSuite.Schema))), nil
					}
					if path == url {
						return nil, errors.New("host is not localhost")
					}
					path = "testdata/jsonschema" + path
					return jsonSchemaData.Open(path)
				}

				pattern, err := ConvertJsonSchemaToPattern(string(testSuite.Schema))

				if !supportedSuite {
					if !assert.Error(t, err) {
						return
					}
					return
				}
				if !assert.NoError(t, err) {
					return
				}

				for _, test := range testSuite.Tests {
					supportedTest := true
					for _, skippedTest := range notSupportedTests {
						if testSuite.Description == skippedTest[0] && skippedTest[1] == test.Description {
							supportedTest = false
							break
						}
					}

					if !supportedTest {
						t.SkipNow()
					}

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
		runTestSuites(t, jsonDraft7.AllOf, nil)
	})

	t.Run("AnyOf", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.AnyOf, nil)
	})

	t.Run("BooleanSchema", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.BooleanSchema, nil)
	})

	t.Run("Const", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Const, nil)
	})

	t.Run("Contains", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Contains, [][2]string{
			{"contains with false if subschema", "*"},
			{"contains keyword with boolean schema false", "any non-empty array is invalid"},
			{"contains keyword with boolean schema false", "empty array is invalid"},
		})
	})

	t.Run("Definitions", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Definitions, [][2]string{
			{"validate definition against metaschema", "invalid definition schema"},
		})
	})

	t.Run("Dependencies", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Dependencies, [][2]string{
			{"dependencies with boolean subschemas", "*"},
			{"dependencies with escaped characters", "*"},       //TODO: support
			{"dependent subschema incompatible with root", "*"}, //TODO: support
		})
	})

	t.Run("Enum", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Enum, nil)
	})

	t.Run("ExclusiveMaximum", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.ExclusiveMaximum, nil)
	})

	t.Run("ExclusiveMinimum", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.ExclusiveMinimum, nil)
	})

	t.Run("Format", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.Format, nil)
	})

	t.Run("Id", func(t *testing.T) {
		//not supported
		t.SkipNow()
		runTestSuites(t, jsonDraft7.Id, nil)
	})

	t.Run("IfThenElse", func(t *testing.T) {
		//not supported
		t.SkipNow()
		runTestSuites(t, jsonDraft7.IfThenElse, nil)
	})

	t.Run("InfiniteLoopRecursion", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.InfiniteLoopRecursion, nil)
	})

	t.Run("Ref", func(t *testing.T) {
		t.SkipNow()
		runTestSuites(t, jsonDraft7.Ref, [][2]string{
			{"root pointer ref", "*"},
		})
	})

	t.Run("RefRemote", func(t *testing.T) {
		runTestSuites(t, jsonDraft7.RefRemote, nil)
	})
}

//go:embed testdata/jsonschema/json-draft7.json
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
