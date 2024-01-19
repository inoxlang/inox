package core

import (
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestParseJSONRepresentation(t *testing.T) {
	t.Parallel()

	//TODO: check that removing one '}', or ']' or closing '"' always yields an error.
	//The removed characters should not be inside a string and all cases should be
	//tested, this will require analyzing the structure of the JSON.

	t.Run("strings", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"str__value":"a"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Str("a"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"a"`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Str("a"), v)
		}
	})

	t.Run("booleans", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//boolean
		v, err := ParseJSONRepresentation(ctx, `{"bool__value":true}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, True, v)
		}

		v, err = ParseJSONRepresentation(ctx, `true`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, True, v)
		}

		v, err = ParseJSONRepresentation(ctx, `false`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, False, v)
		}
	})

	t.Run("integers", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"int__value":1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `1`, INT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		//if no schema is specified a float is expected
		v, err = ParseJSONRepresentation(ctx, `1`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		intPattern := NewIntRangePattern(IntRange{start: 0, end: 2, inclusiveEnd: true, step: 1}, -1)
		v, err = ParseJSONRepresentation(ctx, `1`, intPattern)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		_, err = ParseJSONRepresentation(ctx, `-1`, intPattern)
		if !assert.Error(t, err, ErrNotMatchingSchemaIntFound) {
			return
		}

		_, err = ParseJSONRepresentation(ctx, `3`, intPattern)
		if !assert.ErrorIs(t, err, ErrNotMatchingSchemaIntFound) {
			return
		}

		//int (as string)
		v, err = ParseJSONRepresentation(ctx, `{"int__value":"1"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1"`, INT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Int(1), v)
		}
	})

	t.Run("floats", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"float__value":1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `1.0`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		floatPattern := NewFloatRangePattern(FloatRange{start: 0, end: 2, inclusiveEnd: true}, -1)
		v, err = ParseJSONRepresentation(ctx, `1`, floatPattern)
		if assert.NoError(t, err) {
			assert.Equal(t, Float(1), v)
		}

		_, err = ParseJSONRepresentation(ctx, `-1`, floatPattern)
		if !assert.Error(t, err, ErrNotMatchingSchemaFloatFound) {
			return
		}

		_, err = ParseJSONRepresentation(ctx, `3`, floatPattern)
		if !assert.ErrorIs(t, err, ErrNotMatchingSchemaFloatFound) {
			return
		}
	})

	t.Run("line counts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"line-count__value":"1ln"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, LineCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1ln"`, LINECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, LineCount(1), v)
		}
	})

	t.Run("byte counts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"byte-count__value":"1B"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1B"`, BYTECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteCount(1), v)
		}
	})

	t.Run("rune counts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"rune-count__value":"1rn"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, RuneCount(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"1rn"`, RUNECOUNT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, RuneCount(1), v)
		}
	})

	t.Run("paths", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("base cases", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `{"path__value":"/"}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, Path("/"), v)
			}

			v, err = ParseJSONRepresentation(ctx, `"/"`, PATH_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, Path("/"), v)
			}
		})

		t.Run("invalid empty", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `""`, PATH_PATTERN)
			assert.ErrorIs(t, err, ErrEmptyPath)
			assert.Nil(t, v)
		})

		t.Run("invalid path start", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `".bashrc"`, PATH_PATTERN)
			assert.ErrorIs(t, err, ErrPathWithInvalidStart)
			assert.Nil(t, v)

			v, err = ParseJSONRepresentation(ctx, `".../a"`, PATH_PATTERN)
			assert.ErrorIs(t, err, ErrPathWithInvalidStart)
			assert.Nil(t, v)

			v, err = ParseJSONRepresentation(ctx, `"a"`, PATH_PATTERN)
			assert.ErrorIs(t, err, ErrPathWithInvalidStart)
			assert.Nil(t, v)

			v, err = ParseJSONRepresentation(ctx, `"a/b"`, PATH_PATTERN)
			assert.ErrorIs(t, err, ErrPathWithInvalidStart)
			assert.Nil(t, v)
		})
	})

	t.Run("schemes", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("base cases", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `{"scheme__value":"https"}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, Scheme("https"), v)
			}

			v, err = ParseJSONRepresentation(ctx, `"https"`, SCHEME_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, Scheme("https"), v)
			}
		})

		t.Run("empty", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `""`, SCHEME_PATTERN)
			assert.ErrorIs(t, err, ErrEmptyScheme)
			assert.Nil(t, v)
		})

		t.Run("invalid first char", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"+http"`, SCHEME_PATTERN)
			assert.ErrorIs(t, err, ErrSchemeWithInvalidStart)
			assert.Nil(t, v)
		})

		t.Run("space is name", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"ht tp""`, SCHEME_PATTERN)
			assert.ErrorIs(t, err, ErrUnexpectedCharsInScheme)
			assert.Nil(t, v)
		})

		t.Run("unexpected char: `.`", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"http.x"`, SCHEME_PATTERN)
			assert.ErrorIs(t, err, ErrUnexpectedCharsInScheme)
			assert.Nil(t, v)
		})

		t.Run("unexpected char: `:`", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"http:"`, SCHEME_PATTERN)
			assert.ErrorIs(t, err, ErrUnexpectedCharsInScheme)
			assert.Nil(t, v)
		})
	})

	t.Run("hosts", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("base cases", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `{"host__value":"https://example.com"}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, Host("https://example.com"), v)
			}

			v, err = ParseJSONRepresentation(ctx, `"https://example.com"`, HOST_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, Host("https://example.com"), v)
			}
		})

		t.Run("empty", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `""`, HOST_PATTERN)
			assert.ErrorIs(t, err, ErrEmptyHost)
			assert.Nil(t, v)
		})

		t.Run("no scheme", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"://example.com"`, HOST_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, Host("://example.com"), v)
			}
		})

		t.Run("unexpected char in scheme", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"ht}tp://example.com"`, HOST_PATTERN)
			assert.ErrorIs(t, err, ErrInvalidHost)
			assert.Nil(t, v)
		})

		t.Run("missing name after scheme", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"http://"`, HOST_PATTERN)
			assert.ErrorIs(t, err, ErrMissingHostHostNameInHost)
			assert.Nil(t, v)
		})

		t.Run("missing name after scheme (port)", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"http://:80"`, HOST_PATTERN)
			assert.ErrorIs(t, err, ErrMissingHostHostNameInHost)
			assert.Nil(t, v)
		})

		t.Run("unexpected char in name", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"http://example}.com"`, HOST_PATTERN)
			assert.ErrorIs(t, err, ErrInvalidHost)
			assert.Nil(t, v)
		})

		t.Run("space in name", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"http://example .com"`, HOST_PATTERN)
			assert.ErrorIs(t, err, ErrInvalidHost)
			assert.Nil(t, v)
		})
	})

	t.Run("urls", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("base cases", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `{"url__value":"https://example.com/"}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, URL("https://example.com/"), v)
			}

			v, err = ParseJSONRepresentation(ctx, `"https://example.com/"`, URL_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, URL("https://example.com/"), v)
			}
		})

		t.Run("empty", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `""`, URL_PATTERN)
			assert.ErrorIs(t, err, ErrEmptyURL)
			assert.Nil(t, v)
		})

		t.Run("unexpected char in scheme", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"ht}tp://example.com/"`, URL_PATTERN)
			assert.ErrorIs(t, err, ErrInvalidURL)
			assert.Nil(t, v)
		})

		t.Run("no hostname", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"https:///"`, URL_PATTERN)
			assert.ErrorIs(t, err, ErrMissingURLHostName)
			assert.Nil(t, v)
		})

		t.Run("no hostname before port", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"https://:80"`, URL_PATTERN)
			assert.ErrorIs(t, err, ErrMissingURLHostName)
			assert.Nil(t, v)
		})

		t.Run("space in hostname", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"https://examp le/index.html"`, URL_PATTERN)
			assert.ErrorIs(t, err, ErrInvalidURL)
			assert.Nil(t, v)
		})

		t.Run("space in path", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"https://example/ index.html"`, URL_PATTERN)
			assert.ErrorIs(t, err, ErrUnexpectedSpaceInURL)
			assert.Nil(t, v)
		})

		t.Run("no URL-specific feature", func(t *testing.T) {
			v, err := ParseJSONRepresentation(ctx, `"https://example.com"`, URL_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, URL("https://example.com/"), v)
			}
		})
	})

	t.Run("path patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"path-pattern__value":"/..."}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, PathPattern("/..."), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"/..."`, PATHPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, PathPattern("/..."), v)
		}
	})

	t.Run("host patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"host-pattern__value":"https://*.com"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, HostPattern("https://*.com"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://*.com"`, HOSTPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, HostPattern("https://*.com"), v)
		}
	})

	t.Run("url patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"url-pattern__value":"https://example.com/..."}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, URLPattern("https://example.com/..."), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"https://example.com/..."`, URLPATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, URLPattern("https://example.com/..."), v)
		}
	})

	t.Run("property names", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"propname__value":"len"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, PropertyName("len"), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"len"`, PROPNAME_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, PropertyName("len"), v)
		}

		_, err = ParseJSONRepresentation(ctx, `""`, PROPNAME_PATTERN)
		assert.ErrorIs(t, err, ErrEmptyPropertyName)

		_, err = ParseJSONRepresentation(ctx, `" "`, PROPNAME_PATTERN)
		assert.ErrorIs(t, err, ErrUnexpectedCharsInPropertyName)
	})

	t.Run("long value-paths", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"long-value-path__value":[{"propname__value":"name"}, {"propname__value":"len"}]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, NewLongValuePath([]ValuePathSegment{PropertyName("name"), PropertyName("len")}), v)
		}

		v, err = ParseJSONRepresentation(ctx, `[{"propname__value":"name"}, {"propname__value":"len"}]`, LONG_VALUEPATH_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, NewLongValuePath([]ValuePathSegment{PropertyName("name"), PropertyName("len")}), v)
		}

		_, err = ParseJSONRepresentation(ctx, `[]`, LONG_VALUEPATH_PATTERN)
		assert.ErrorIs(t, err, ErrEmptyLongValuePath)

		_, err = ParseJSONRepresentation(ctx, `[{"propname__value":"name"}]`, LONG_VALUEPATH_PATTERN)
		assert.ErrorIs(t, err, ErrSingleSegmentLongValuePath)
	})

	t.Run("objects", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		obj, err := ParseJSONRepresentation(ctx, `{"object__value":{}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"a":"1"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"object__value":{"_url_":"ldb://main/users/0"}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap(nil))
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
			assert.Equal(t, map[string]Value{}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":"1"}`, OBJECT_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Str("1")}, obj.(*Object).ValueEntryMap(nil))
		}

		//{a: int} pattern
		pattern := NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1)}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":1,"b":"c"}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1), "b": Str("c")}, obj.(*Object).ValueEntryMap(nil))
		}

		//{a: {b: int}} pattern
		pattern = NewInexactObjectPattern([]ObjectPatternEntry{
			{
				Name:    "a",
				Pattern: NewInexactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}}),
			},
		})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":{}}`, pattern)
		if assert.ErrorContains(t, err, "failed to parse value of object property \"a\": the following properties are missing: b") {
			assert.Nil(t, obj)
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":{"b": 1}}`, pattern)
		if assert.NoError(t, err) {
			entries := obj.(*Object).ValueEntryMap(nil)
			if assert.Contains(t, entries, "a") {
				assert.Equal(t, map[string]Value{"b": Int(1)}, entries["a"].(*Object).ValueEntryMap(nil))
			}
		}

		//{a: []int} pattern ([]int has a default value)
		pattern = NewInexactObjectPattern([]ObjectPatternEntry{
			{
				Name:    "a",
				Pattern: NewListPatternOf(INT_PATTERN),
			},
		})

		obj, err = ParseJSONRepresentation(ctx, `{}`, pattern) //auto fix
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewWrappedValueList()}, obj.(*Object).ValueEntryMap(nil))
		}

		obj, err = ParseJSONRepresentation(ctx, `{"a":[1]}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewWrappedIntList(Int(1))}, obj.(*Object).ValueEntryMap(nil))
		}
	})

	t.Run("records", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

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

		//#{a: int} pattern
		pattern := NewInexactRecordPattern([]RecordPatternEntry{
			{
				Name:    "a",
				Pattern: INT_PATTERN,
			},
		})

		rec, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, rec)
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": Int(1)}, rec.(*Record).ValueEntryMap())
		}

		//#{a: #{b: int}} pattern
		pattern = NewInexactRecordPattern([]RecordPatternEntry{
			{
				Name: "a",
				Pattern: NewInexactRecordPattern([]RecordPatternEntry{
					{
						Name:    "b",
						Pattern: INT_PATTERN,
					},
				}),
			},
		})

		rec, err = ParseJSONRepresentation(ctx, `{}`, pattern)
		if assert.ErrorContains(t, err, "the following properties are missing: a") {
			assert.Nil(t, rec)
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":{}}`, pattern)
		if assert.ErrorContains(t, err, "failed to parse value of record property a: the following properties are missing: b") {
			assert.Nil(t, rec)
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":{"b": 1}}`, pattern)
		if assert.NoError(t, err) {
			entries := rec.(*Record).ValueEntryMap()
			if assert.Contains(t, entries, "a") {
				assert.Equal(t, map[string]Value{"b": Int(1)}, entries["a"].(*Record).ValueEntryMap())
			}
		}

		//#{a: #[]int} pattern (#[]int has a default value)
		pattern = NewInexactRecordPattern([]RecordPatternEntry{
			{
				Name:    "a",
				Pattern: NewTuplePatternOf(INT_PATTERN),
			},
		})

		rec, err = ParseJSONRepresentation(ctx, `{}`, pattern) //auto fix
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewTuple(nil)}, rec.(*Record).ValueEntryMap())
		}

		rec, err = ParseJSONRepresentation(ctx, `{"a":[1]}`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, map[string]Value{"a": NewTuple([]Serializable{Int(1)})}, rec.(*Record).ValueEntryMap())
		}
	})

	t.Run("lists", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		list, err := ParseJSONRepresentation(ctx, `{"list__value":[]}`, nil)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `{"list__value":["1"]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, list.(*List).GetOrBuildElements(ctx))
		}

		//%list patteren
		list, err = ParseJSONRepresentation(ctx, `[]`, LIST_PATTERN)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, LIST_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, list.(*List).GetOrBuildElements(ctx))
		}

		//[]int pattern
		pattern := NewListPatternOf(INT_PATTERN)

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*IntList)(nil), list.(*List).underlyingList)
		}

		list, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*IntList)(nil), list.(*List).underlyingList)
		}

		//[]bool pattern
		pattern = NewListPatternOf(BOOL_PATTERN)

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*BoolList)(nil), list.(*List).underlyingList)
		}

		list, err = ParseJSONRepresentation(ctx, `[true]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{True}, list.(*List).GetOrBuildElements(ctx))
			assert.IsType(t, (*BoolList)(nil), list.(*List).underlyingList)
		}

		//[] pattern
		pattern = NewListPattern([]Pattern{})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `["1"]`, pattern)
		if assert.ErrorContains(t, err, "JSON array too many elements (1), at most 0 element(s) were expected") {
			assert.Nil(t, list)
		}

		//[int] pattern
		pattern = NewListPattern([]Pattern{INT_PATTERN})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, list.(*List).GetOrBuildElements(ctx))
		}

		list, err = ParseJSONRepresentation(ctx, `[1,2]`, pattern)
		if assert.ErrorContains(t, err, "JSON array too many elements (2), at most 1 element(s) were expected") {
			assert.Nil(t, list)
		}

		//[int, int] pattern
		pattern = NewListPattern([]Pattern{INT_PATTERN, INT_PATTERN})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (0), 2 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (1), 2 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[1, 2]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1), Int(2)}, list.(*List).GetOrBuildElements(ctx))
		}

		//[[int]] pattern
		pattern = NewListPattern([]Pattern{NewListPattern([]Pattern{INT_PATTERN})})

		list, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many or not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, list)
		}

		list, err = ParseJSONRepresentation(ctx, `[[1]]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{NewWrappedValueList(Int(1))}, list.(*List).GetOrBuildElements(ctx))
		}
	})

	t.Run("tuples", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		//no pattern
		tuple, err := ParseJSONRepresentation(ctx, `{"tuple__value":[]}`, nil)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `{"tuple__value":["1"]}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//%tuple patteren
		tuple, err = ParseJSONRepresentation(ctx, `[]`, TUPLE_PATTERN)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `["1"]`, TUPLE_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Str("1")}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[]int pattern
		pattern := NewTuplePatternOf(INT_PATTERN)

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[] pattern
		pattern = NewTuplePattern([]Pattern{})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.NoError(t, err) {
			assert.Empty(t, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many elements (1), 0 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		//[int] pattern
		pattern = NewTuplePattern([]Pattern{INT_PATTERN})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1,2]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has too many elements (2), 1 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		//[int, int] pattern
		pattern = NewTuplePattern([]Pattern{INT_PATTERN, INT_PATTERN})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (0), 2 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (1), 2 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[1, 2]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{Int(1), Int(2)}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}

		//[[int]] pattern
		pattern = NewTuplePattern([]Pattern{NewTuplePattern([]Pattern{INT_PATTERN})})

		tuple, err = ParseJSONRepresentation(ctx, `[]`, pattern)
		if assert.ErrorContains(t, err, "JSON array has not enough elements (0), 1 element(s) were expected") {
			assert.Nil(t, tuple)
		}

		tuple, err = ParseJSONRepresentation(ctx, `[[1]]`, pattern)
		if assert.NoError(t, err) {
			assert.Equal(t, []Serializable{NewTuple([]Serializable{Int(1)})}, tuple.(*Tuple).GetOrBuildElements(ctx))
		}
	})

	t.Run("integer ranges", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: INT_RANGE_PATTERN}

		t.Run("base case", func(t *testing.T) {
			intRange := NewIncludedEndIntRange(0, 10)
			serialized := MustGetJSONRepresentationWithConfig(intRange, ctx, config)

			v, err := ParseJSONRepresentation(ctx, `{"int-range__value":`+serialized+`}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, intRange, v)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, intRange, v)
			}
		})

		t.Run("unknown start", func(t *testing.T) {
			intRange := NewUnknownStartIntRange(10, true)
			serialized := MustGetJSONRepresentationWithConfig(intRange, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, intRange, v)
			}
		})

		t.Run("exclusive end", func(t *testing.T) {
			intRange := NewIntRange(0, 10, false)
			serialized := MustGetJSONRepresentationWithConfig(intRange, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, intRange, v)
			}
		})
	})

	t.Run("float ranges", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: FLOAT_RANGE_PATTERN}

		t.Run("base case", func(t *testing.T) {
			floatRange := NewIncludedEndFloatRange(0, 10)
			serialized := MustGetJSONRepresentationWithConfig(floatRange, ctx, config)

			v, err := ParseJSONRepresentation(ctx, `{"float-range__value":`+serialized+`}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, floatRange, v)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, floatRange, v)
			}
		})

		t.Run("unknown start", func(t *testing.T) {
			floatRange := NewUnknownStartFloatRange(10, true)
			serialized := MustGetJSONRepresentationWithConfig(floatRange, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, floatRange, v)
			}
		})

		t.Run("exclusive end", func(t *testing.T) {
			floatRange := NewFloatRange(0, 10, false)
			serialized := MustGetJSONRepresentationWithConfig(floatRange, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, floatRange, v)
			}
		})
	})

	t.Run("integer range patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: INT_RANGE_PATTERN_PATTERN}

		t.Run("base case", func(t *testing.T) {
			pattern := NewIntRangePattern(NewIncludedEndIntRange(0, 10), 0)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, `{"int-range-pattern__value":`+serialized+`}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("unknown start", func(t *testing.T) {
			pattern := NewIntRangePattern(NewUnknownStartIntRange(10, true), 0)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("exclusive end", func(t *testing.T) {
			pattern := NewIntRangePattern(NewIntRange(0, 10, false), 0)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("multipleOf: float64(2.5)", func(t *testing.T) {
			pattern := NewIntRangePatternFloatMultiple(NewIntRange(0, 10, false), 2.5)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("multipleOf: int64(2)", func(t *testing.T) {
			pattern := NewIntRangePattern(NewIntRange(0, 10, false), 2)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, INT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})
	})

	t.Run("float range patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: FLOAT_RANGE_PATTERN_PATTERN}

		t.Run("base case", func(t *testing.T) {
			pattern := NewFloatRangePattern(NewIncludedEndFloatRange(0, 10), 0)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, `{"float-range-pattern__value":`+serialized+`}`, nil)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("unknown start", func(t *testing.T) {
			pattern := NewFloatRangePattern(NewUnknownStartFloatRange(10, true), 0)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("exclusive end", func(t *testing.T) {
			pattern := NewFloatRangePattern(NewFloatRange(0, 10, false), 0)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("multipleOf: float64(2.5)", func(t *testing.T) {
			pattern := NewFloatRangePattern(NewFloatRange(0, 10, false), 2)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})

		t.Run("multipleOf: float64(2)", func(t *testing.T) {
			pattern := NewFloatRangePattern(NewFloatRange(0, 10, false), 2)
			serialized := MustGetJSONRepresentationWithConfig(pattern, ctx, config)

			v, err := ParseJSONRepresentation(ctx, serialized, FLOAT_RANGE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assert.Equal(t, pattern, v)
			}
		})
	})

	t.Run("unions", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("integers & strings", func(t *testing.T) {
			pattern := NewUnionPattern([]Pattern{INT_PATTERN, STR_PATTERN}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			//priority to first pattern
			val, err = ParseJSONRepresentation(ctx, `"1"`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `" 1"`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, Str(" 1"), val)
			}
		})

		t.Run("integers", func(t *testing.T) {
			range1 := IntRange{start: 0, end: 2, inclusiveEnd: true, step: 1}
			range2 := IntRange{start: 1, end: 3, inclusiveEnd: true, step: 1}

			pattern1 := NewUnionPattern([]Pattern{NewIntRangePattern(range1, -1), NewIntRangePattern(range2, -1)}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `3`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(3), val)
			}

			_, err = ParseJSONRepresentation(ctx, `2`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(2), val)
			}
		})

		t.Run("objects", func(t *testing.T) {
			pattern := NewUnionPattern([]Pattern{
				NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}}),
				NewInexactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}}),
			}, nil)

			val, err := ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1)}), val)
			}

			val, err = ParseJSONRepresentation(ctx, `{"b":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"b": Int(1)}), val)
			}

			_, err = ParseJSONRepresentation(ctx, `{"a":1,"b":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Int(1)}), val)
			}
		})

		t.Run("lists", func(t *testing.T) {
			pattern := NewUnionPattern([]Pattern{
				NewListPatternOf(INT_PATTERN),
				NewListPatternOf(BOOL_PATTERN),
			}, nil)

			val, err := ParseJSONRepresentation(ctx, `[]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedIntListFrom([]Int{}), val)
			}

			val, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedIntList(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `[true]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedBoolList(True), val)
			}
		})
	})

	t.Run("disjoint unions", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("integers & strings", func(t *testing.T) {
			pattern1 := NewUnionPattern([]Pattern{INT_PATTERN, STR_PATTERN}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			//priority to first pattern
			val, err = ParseJSONRepresentation(ctx, `"1"`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `" 1"`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Str(" 1"), val)
			}
		})

		t.Run("integers", func(t *testing.T) {
			range1 := IntRange{start: 0, end: 2, inclusiveEnd: true, step: 1}
			range2 := IntRange{start: 2, end: 3, inclusiveEnd: true, step: 1}

			pattern1 := NewDisjointUnionPattern([]Pattern{NewIntRangePattern(range1, -1), NewIntRangePattern(range2, -1)}, nil)

			val, err := ParseJSONRepresentation(ctx, `1`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `3`, pattern1)
			if !assert.NoError(t, err) {
				assert.Equal(t, Int(3), val)
			}

			_, err = ParseJSONRepresentation(ctx, `2`, pattern1)
			assert.ErrorIs(t, err, ErrJsonNotMatchingSchema)
		})

		t.Run("objects", func(t *testing.T) {
			//no pattern
			val, err := ParseJSONRepresentation(ctx, `{"a":1,"b":1}`, nil)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Int(1)}), val)
			}

			pattern := NewDisjointUnionPattern([]Pattern{
				NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: INT_PATTERN}}),
				NewInexactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}}),
			}, nil)

			val, err = ParseJSONRepresentation(ctx, `{"a":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"a": Int(1)}), val)
			}

			val, err = ParseJSONRepresentation(ctx, `{"b":1}`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewObjectFromMapNoInit(ValMap{"b": Int(1)}), val)
			}

			//TODO: fix implementation
			return
			_, err = ParseJSONRepresentation(ctx, `{"a":1,"b":1}`, pattern)
			assert.ErrorIs(t, err, ErrJsonNotMatchingSchema)
		})

		t.Run("lists", func(t *testing.T) {
			//no pattern
			val, err := ParseJSONRepresentation(ctx, `[1, 2]`, nil)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedValueList(Int(1), Int(2)), val)
			}

			pattern := NewDisjointUnionPattern([]Pattern{
				NewListPatternOf(INT_PATTERN),
				NewListPatternOf(BOOL_PATTERN),
			}, nil)

			_, err = ParseJSONRepresentation(ctx, `[]`, pattern)
			assert.ErrorIs(t, err, ErrJsonNotMatchingSchema)

			val, err = ParseJSONRepresentation(ctx, `[1]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedIntList(1), val)
			}

			val, err = ParseJSONRepresentation(ctx, `[true]`, pattern)
			if !assert.NoError(t, err) {
				assert.Equal(t, NewWrappedBoolList(True), val)
			}
		})
	})

	t.Run("exact value patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"exact-value-pattern__value":{"float__value":0.1}}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, NewExactValuePattern(Float(0.1)), v)
		}

		v, err = ParseJSONRepresentation(ctx, `{"float__value":0.1}`, EXACT_VALUE_PATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, NewExactValuePattern(Float(0.1)), v)
		}
	})

	t.Run("exact string patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"exact-string-pattern__value":"x"}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, NewExactStringPattern(Str("x")), v)
		}

		v, err = ParseJSONRepresentation(ctx, `"x"`, EXACT_STRING_PATTERN_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, NewExactStringPattern(Str("x")), v)
		}
	})

	t.Run("frequencies", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"frequency__value":0.1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Frequency(0.1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `0.1`, FREQUENCY_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Frequency(0.1), v)
		}
	})

	t.Run("byte-rates", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"byte-rate__value":1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteRate(1), v)
		}

		v, err = ParseJSONRepresentation(ctx, `1`, BYTERATE_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, ByteRate(1), v)
		}
	})

	t.Run("durations", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		v, err := ParseJSONRepresentation(ctx, `{"duration__value":0.1}`, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, Duration(time.Second/10), v)
		}

		v, err = ParseJSONRepresentation(ctx, `0.1`, DURATION_PATTERN)
		if assert.NoError(t, err) {
			assert.Equal(t, Duration(time.Second/10), v)
		}
	})

	t.Run("object patterns", func(t *testing.T) {
		//TODO: add tests for patterns with complex constraints.

		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: OBJECT_PATTERN_PATTERN}

		t.Run("empty inexact", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, `{"object-pattern__value":`+serialized+"}", nil)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("empty exact", func(t *testing.T) {
			pattern := NewExactObjectPattern([]ObjectPatternEntry{})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (simple) prop", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (complex) prop", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name: "a",
					Pattern: NewInexactObjectPattern([]ObjectPatternEntry{
						{
							Name:    "b",
							Pattern: INT_PATTERN,
						},
					}),
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("two props", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
				},
				{
					Name:    "b",
					Pattern: INT_PATTERN,
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("optional prop", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:       "a",
					Pattern:    INT_PATTERN,
					IsOptional: true,
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("prop with one required key", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
					Dependencies: PropertyDependencies{
						RequiredKeys: []string{"b"},
					},
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("prop with required keys", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
					Dependencies: PropertyDependencies{
						RequiredKeys: []string{"b", "c"},
					},
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("prop with required pattern", func(t *testing.T) {
			pattern := NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
					Dependencies: PropertyDependencies{
						Pattern: NewInexactObjectPattern([]ObjectPatternEntry{}),
					},
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, OBJECT_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})
	})

	t.Run("record patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: RECORD_PATTERN_PATTERN}

		t.Run("empty inexact", func(t *testing.T) {
			pattern := NewInexactRecordPattern([]RecordPatternEntry{})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, `{"record-pattern__value":`+serialized+"}", nil)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, RECORD_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("empty exact", func(t *testing.T) {
			pattern := NewExactRecordPattern([]RecordPatternEntry{})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, RECORD_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (simple) prop", func(t *testing.T) {
			pattern := NewInexactRecordPattern([]RecordPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, RECORD_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (complex) prop", func(t *testing.T) {
			pattern := NewInexactRecordPattern([]RecordPatternEntry{
				{
					Name: "a",
					Pattern: NewInexactRecordPattern([]RecordPatternEntry{
						{
							Name:    "b",
							Pattern: INT_PATTERN,
						},
					}),
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, RECORD_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("two props", func(t *testing.T) {
			pattern := NewInexactRecordPattern([]RecordPatternEntry{
				{
					Name:    "a",
					Pattern: INT_PATTERN,
				},
				{
					Name:    "b",
					Pattern: INT_PATTERN,
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, RECORD_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("optional prop", func(t *testing.T) {
			pattern := NewInexactRecordPattern([]RecordPatternEntry{
				{
					Name:       "a",
					Pattern:    INT_PATTERN,
					IsOptional: true,
				},
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, RECORD_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

	})

	t.Run("list patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: LIST_PATTERN_PATTERN}

		t.Run("empty", func(t *testing.T) {
			pattern := NewListPattern([]Pattern{})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, `{"list-pattern__value":`+serialized+"}", nil)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (simple) element", func(t *testing.T) {
			pattern := NewListPattern([]Pattern{INT_PATTERN})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (complex) element", func(t *testing.T) {
			pattern := NewListPattern([]Pattern{
				NewListPattern([]Pattern{INT_PATTERN}),
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("two elements", func(t *testing.T) {
			pattern := NewListPattern([]Pattern{INT_PATTERN, STR_PATTERN})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("general (simple) element", func(t *testing.T) {
			pattern := NewListPatternOf(INT_PATTERN)
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("general (complex) element", func(t *testing.T) {
			pattern := NewListPatternOf(NewListPatternOf(INT_PATTERN))
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("specific minimal element count", func(t *testing.T) {
			pattern := NewListPatternOf(NewListPatternOf(INT_PATTERN)).WithMinElements(10)
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("specific maximal element count", func(t *testing.T) {
			pattern := NewListPatternOf(NewListPatternOf(INT_PATTERN)).WithMinMaxElements(0, 10)
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("specific minimal and maximal element counts", func(t *testing.T) {
			pattern := NewListPatternOf(NewListPatternOf(INT_PATTERN)).WithMinMaxElements(5, 10)
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, LIST_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})
	})

	t.Run("list patterns", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		config := JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG, Pattern: TUPLE_PATTERN_PATTERN}

		t.Run("empty", func(t *testing.T) {
			pattern := NewTuplePattern([]Pattern{})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, `{"tuple-pattern__value":`+serialized+"}", nil)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}

			v, err = ParseJSONRepresentation(ctx, serialized, TUPLE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (simple) element", func(t *testing.T) {
			pattern := NewTuplePattern([]Pattern{INT_PATTERN})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, TUPLE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("single (complex) element", func(t *testing.T) {
			pattern := NewTuplePattern([]Pattern{
				NewTuplePattern([]Pattern{INT_PATTERN}),
			})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, TUPLE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("two elements", func(t *testing.T) {
			pattern := NewTuplePattern([]Pattern{INT_PATTERN, STR_PATTERN})
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, TUPLE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("general (simple) element", func(t *testing.T) {
			pattern := NewTuplePatternOf(INT_PATTERN)
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, TUPLE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})

		t.Run("general (complex) element", func(t *testing.T) {
			pattern := NewTuplePatternOf(NewTuplePatternOf(INT_PATTERN))
			serialized := utils.Must(GetJSONRepresentationWithConfig(pattern, ctx, config))

			v, err := ParseJSONRepresentation(ctx, serialized, TUPLE_PATTERN_PATTERN)
			if assert.NoError(t, err) {
				assertEqualInoxValues(t, pattern, v, ctx)
			}
		})
	})

}
