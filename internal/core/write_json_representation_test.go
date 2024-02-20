package core

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"time"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestNilJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "null", getJSONRepr(t, Nil, ctx))
}

func TestBoolJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "true", getJSONRepr(t, True, ctx))
}

func TestRuneJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, `{"rune__value":"a"}`, getJSONRepr(t, Rune('a'), ctx))
	assert.Equal(t, `"a"`, getJSONRepr(t, Rune('a'), ctx, JSONSerializationConfig{
		Pattern: RUNE_PATTERN,
	}))

	//TODO: add more tests
}

func TestIntJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, `{"int__value":2}`, getJSONRepr(t, Int(2), ctx))
	assert.Equal(t, `2`, getJSONRepr(t, Int(2), ctx, JSONSerializationConfig{
		Pattern: INT_PATTERN,
	}))

	//int == JS_MIN_SAFE_INTEGER
	assert.Equal(t, fmt.Sprintf(`{"int__value":%d}`, JS_MIN_SAFE_INTEGER), getJSONRepr(t, Int(JS_MIN_SAFE_INTEGER), ctx))
	assert.Equal(t, fmt.Sprintf(`%d`, JS_MIN_SAFE_INTEGER), getJSONRepr(t, Int(JS_MIN_SAFE_INTEGER), ctx, JSONSerializationConfig{
		Pattern: INT_PATTERN,
	}))

	//int == JS_MIN_SAFE_INTEGER - 1
	assert.Equal(t, fmt.Sprintf(`{"int__value":"%d"}`, JS_MIN_SAFE_INTEGER-1), getJSONRepr(t, Int(JS_MIN_SAFE_INTEGER-1), ctx))
	assert.Equal(t, fmt.Sprintf(`"%d"`, JS_MIN_SAFE_INTEGER-1), getJSONRepr(t, Int(JS_MIN_SAFE_INTEGER-1), ctx, JSONSerializationConfig{
		Pattern: INT_PATTERN,
	}))

	//int == JS_MAX_SAFE_INTEGER
	assert.Equal(t, fmt.Sprintf(`{"int__value":%d}`, JS_MAX_SAFE_INTEGER), getJSONRepr(t, Int(JS_MAX_SAFE_INTEGER), ctx))
	assert.Equal(t, fmt.Sprintf(`%d`, JS_MAX_SAFE_INTEGER), getJSONRepr(t, Int(JS_MAX_SAFE_INTEGER), ctx, JSONSerializationConfig{
		Pattern: INT_PATTERN,
	}))

	//int == JS_MAX_SAFE_INTEGER + 1
	assert.Equal(t, fmt.Sprintf(`{"int__value":"%d"}`, JS_MAX_SAFE_INTEGER+1), getJSONRepr(t, Int(JS_MAX_SAFE_INTEGER+1), ctx))
	assert.Equal(t, fmt.Sprintf(`"%d"`, JS_MAX_SAFE_INTEGER+1), getJSONRepr(t, Int(JS_MAX_SAFE_INTEGER+1), ctx, JSONSerializationConfig{
		Pattern: INT_PATTERN,
	}))

	//TODO: add more tests
}

func TestFloatJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []struct {
		value          Float
		representation string
	}{
		{1.0, "1"},
		{1.001, "1.001"},
		{100.0, "100"},
		{100.00, "100"},
		{100.001, "100.001"},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			repr := getJSONRepr(t, testCase.value, ctx)
			assert.Equal(t, testCase.representation, repr)

			repr = getJSONRepr(t, testCase.value, ctx, JSONSerializationConfig{
				Pattern: FLOAT_PATTERN,
			})
			assert.Equal(t, testCase.representation, repr)
		})
	}
}

func TestStrJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	s := String("a\nb")

	expectedRepr := `"a\nb"`
	assert.Equal(t, expectedRepr, getJSONRepr(t, s, ctx))
	assert.Equal(t, expectedRepr, getJSONRepr(t, s, ctx, JSONSerializationConfig{
		Pattern: STR_PATTERN,
	}))
	assert.Equal(t, expectedRepr, getJSONRepr(t, s, ctx, JSONSerializationConfig{
		Pattern: STR_PATTERN,
	}))
}

func TestObjectJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("empty", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := &Object{}

		assert.Equal(t, `{"object__value":{}}`, getJSONRepr(t, obj, ctx))
		assert.Equal(t, `{}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: OBJECT_PATTERN,
		}))
		assert.Equal(t, `{}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: NewInexactObjectPattern(nil),
		}))
	})

	t.Run("single key: ambiguous value", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{"a\nb": Path("/")})

		assert.Equal(t, `{"object__value":{"a\nb":{"path__value":"/"}}}`, getJSONRepr(t, obj, ctx))
		assert.Equal(t, `{"a\nb":{"path__value":"/"}}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: OBJECT_PATTERN,
		}))
		assert.Equal(t, `{"a\nb":{"path__value":"/"}}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: PATH_PATTERN,
				},
			}),
		}))
	})

	t.Run("two keys", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{"a\nb": Int(1), "c\nd": Int(2)})

		assert.Equal(t, `{"a\nb":1,"c\nd":2}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a\nb",
					Pattern: INT_PATTERN,
				},
				{
					Name:    "c\nd",
					Pattern: INT_PATTERN,
				},
			}),
		}))
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{
			"a": NewWrappedValueList(Int(1), objFrom(ValMap{"b": Int(2)})),
		})

		assert.Equal(t, `{"a":[1,{"b":2}]}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name: "a",
					Pattern: NewListPattern([]Pattern{
						INT_PATTERN,
						NewInexactObjectPattern([]ObjectPatternEntry{{Name: "b", Pattern: INT_PATTERN}}),
					}),
				},
			}),
		}))
	})

	t.Run("cycle", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := &Object{}
		obj.SetProp(ctx, "self", obj)

	})

	t.Run("sensitive properties", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": String("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		assert.Equal(t, `{"object__value":{"a":{"int__value":1}}}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			ReprConfig: &ReprConfig{
				AllVisible: false,
			},
		}))
	})

	t.Run("sensitive properties: config with .allVisible == true", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{
			"a":        String("1"),
			"password": String("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		expectedRepr := `{"object__value":{"a":"1","e":{"emailaddr__value":"a@mail.com"},"password":"mypassword"}}`

		assert.Equal(t, expectedRepr, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			ReprConfig: &ReprConfig{
				AllVisible: true,
			},
		}))
	})

	t.Run("sensitive properties: value visibility with all keys to public", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{
			"a":        String("1"),
			"password": String("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		initializeObjectVisibility(obj, &ValueVisibility{
			publicKeys: []string{"a", "password", "e"},
		})

		expectedRepr := `{"object__value":{"a":"1","e":{"emailaddr__value":"a@mail.com"},"password":"mypassword"}}`

		assert.Equal(t, expectedRepr, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			ReprConfig: &ReprConfig{
				AllVisible: false,
			},
		}))
	})

	t.Run("url", func(t *testing.T) {
		reprTestCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		obj := objFrom(ValMap{})

		url := URL("https://example.com/objects/98484")
		utils.PanicIfErr(obj.SetURLOnce(reprTestCtx, url))

		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		expectedRepr := `{"object__value":{"_url_":"` + string(url) + `"}}`
		assert.Equal(t, expectedRepr, getJSONRepr(t, obj, ctx))
	})
}

func TestRecordJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("empty", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(nil)

		assert.Equal(t, `{"record__value":{}}`, getJSONRepr(t, rec, ctx))
		assert.Equal(t, `{}`, getJSONRepr(t, rec, ctx, JSONSerializationConfig{
			Pattern: RECORD_PATTERN,
		}))
	})

	t.Run("single key: ambiguous value", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := NewRecordFromMap(ValMap{"a\nb": Path("/")})

		assert.Equal(t, `{"record__value":{"a\nb":{"path__value":"/"}}}`, getJSONRepr(t, obj, ctx))
		assert.Equal(t, `{"a\nb":{"path__value":"/"}}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: RECORD_PATTERN,
		}))
		assert.Equal(t, `{"a\nb":{"path__value":"/"}}`, getJSONRepr(t, obj, ctx, JSONSerializationConfig{
			Pattern: NewInexactObjectPattern([]ObjectPatternEntry{
				{
					Name:    "a",
					Pattern: PATH_PATTERN,
				},
			}),
		}))
	})

	t.Run("two keys", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(ValMap{"a\nb": String("1"), "c\nd": String("2")})

		assert.Equal(t, `{"a\nb":"1","c\nd":"2"}`, getJSONRepr(t, rec, ctx, JSONSerializationConfig{
			Pattern: RECORD_PATTERN,
		}))
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(ValMap{
			"a": &Tuple{
				elements: []Serializable{Int(1), NewRecordFromMap(ValMap{"b": Int(2)})},
			},
		})

		assert.Equal(t, `{"a":[1,{"b":2}]}`, getJSONRepr(t, rec, ctx, JSONSerializationConfig{
			Pattern: NewInexactRecordPattern([]RecordPatternEntry{
				{
					Name: "a",
					Pattern: NewTuplePattern([]Pattern{
						INT_PATTERN,
						NewInexactRecordPattern([]RecordPatternEntry{{Name: "b", Pattern: INT_PATTERN}}),
					}),
				},
			}),
		}))
	})

	t.Run("sensitive properties", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(ValMap{
			"a":        String("1"),
			"password": String("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		assert.Equal(t, `{"a":"1"}`, getJSONRepr(t, rec, ctx, JSONSerializationConfig{
			ReprConfig: &ReprConfig{
				AllVisible: false,
			},
			Pattern: RECORD_PATTERN,
		}))
	})

}

func TestDictJSONRepresentation(t *testing.T) {
	//TODO:
	// 	t.Run("empty", func(t *testing.T) {
	// 		dict := NewDictionary(nil)

	// 		assert.Equal(t, `:{}`, getJSONRepr(t, dict, reprTestCtx))
	// 	})

	// 	t.Run("single string key", func(t *testing.T) {
	// 		dict := NewDictionary(map[string]Value{"\"a\\nb\"": Int(1)})

	// 		expectedRepr := `:{"a\nb":1}`
	// 		assert.Equal(t, expectedRepr, getJSONRepr(t, dict, reprTestCtx))
	// 	})

	// 	t.Run("two keys: one string & a path", func(t *testing.T) {
	// 		dict := NewDictionary(map[string]Value{"\"a\\nb\"": Int(1), "./path": Int(2)})

	// 		repr := getJSONRepr(t, dict, reprTestCtx)
	// 		var expectedRepr = `:{"a\nb":1,./path:2}`
	// 		if repr[2] == '.' {
	// 			expectedRepr = `:{./path:2,"a\nb":1}`
	// 		}

	// 		assert.Equal(t, expectedRepr, repr)
	// 	})

	// 	t.Run("one of entry's value has no representation", func(t *testing.T) {
	// 		dict := NewDictionary(map[string]Value{"\"a\\nb\"": &Reader{wrapped: bytes.NewReader(nil)}})

	// 	})

	// 	t.Run("cycle", func(t *testing.T) {
	// 		dict := NewDictionary(nil)
	// 		dict.Entries["self"] = dict
	// 		dict.Keys["self"] = Str("self")

	//	})
}

func TestKeyListJSONRepresentation(t *testing.T) {
	//TODO
	// t.Run("empty", func(t *testing.T) {
	// 	list := KeyList{}

	// 	assert.Equal(t, `.{}`, getJSONRepr(t, list, reprTestCtx))
	// })

	// t.Run("single key", func(t *testing.T) {
	// 	list := KeyList{"a"}

	// 	expectedRepr := `.{a}`
	// 	assert.Equal(t, expectedRepr, getJSONRepr(t, list, reprTestCtx))
	// })

	// t.Run("two keys: one string & a path", func(t *testing.T) {
	// 	list := KeyList{"a", "b"}

	// 	expectedRepr := `.{a,b}`
	// 	assert.Equal(t, expectedRepr, getJSONRepr(t, list, reprTestCtx))
	// })

}

func TestListJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("empty", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList()

		expectedRepr := `{"list__value":[]}`
		assert.Equal(t, expectedRepr, getJSONRepr(t, list, ctx))
	})

	t.Run("single element: ambiguous", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(Path("/"))

		//untyped
		assert.Equal(t, `{"list__value":[{"path__value":"/"}]}`, getJSONRepr(t, list, ctx))

		//loosely typed
		assert.Equal(t, `[{"path__value":"/"}]`, getJSONRepr(t, list, ctx, JSONSerializationConfig{
			Pattern: LIST_PATTERN,
		}))
		assert.Equal(t, `[{"path__value":"/"}]`, getJSONRepr(t, list, ctx, JSONSerializationConfig{
			Pattern: NewListPattern([]Pattern{ANYVAL_PATTERN}),
		}))
		assert.Equal(t, `[{"path__value":"/"}]`, getJSONRepr(t, list, ctx, JSONSerializationConfig{
			Pattern: NewListPatternOf(ANYVAL_PATTERN),
		}))

		//strongely typed
		assert.Equal(t, `["/"]`, getJSONRepr(t, list, ctx, JSONSerializationConfig{
			Pattern: NewListPatternOf(PATH_PATTERN),
		}))

		assert.Equal(t, `["/"]`, getJSONRepr(t, list, ctx, JSONSerializationConfig{
			Pattern: NewListPattern([]Pattern{PATH_PATTERN}),
		}))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(String("2"), String("a"))

		assert.Equal(t, `{"list__value":["2","a"]}`, getJSONRepr(t, list, ctx))
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(NewWrappedValueList(Int(2), objFrom(ValMap{"a": Int(1)})))

		expectedRepr := `{"list__value":[{"list__value":[{"int__value":2},{"object__value":{"a":{"int__value":1}}}]}]}`
		assert.Equal(t, expectedRepr, getJSONRepr(t, list, ctx))
	})

	t.Run("cycle", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(Int(0))
		list.set(NewContext(ContextConfig{}), 0, list)

	})

}

func TestByteSliceJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	//TODO
}

func TestOptionJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	//TODO
}

func TestPathJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []struct {
		value          string
		representation string
	}{
		{"/a", `"/a"`},
		{"/[a-z]", "\"/[a-z]\""},
		{"/]", "\"/]\""},
		{"/\\]", "\"/\\\\]\""},
		{"/ ", "\"/ \""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			pth := Path(testCase.value)

			assert.Equal(t, `{"path__value":`+testCase.representation+"}", getJSONRepr(t, pth, ctx))
			assert.Equal(t, testCase.representation, getJSONRepr(t, pth, ctx, JSONSerializationConfig{
				Pattern: PATHPATTERN_PATTERN,
			}))
		})
	}

}
func TestPathPatternJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []struct {
		value          string
		representation string
	}{
		{"/...", `"/..."`},
		{"/[a-z]", "\"/[a-z]\""},
		{"/]", "\"/]\""},
		{"/\\]", "\"/\\\\]\""},
		{"/ ", "\"/ \""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt := PathPattern(testCase.value)

			assert.Equal(t, `{"path-pattern__value":`+testCase.representation+"}", getJSONRepr(t, patt, ctx))
			assert.Equal(t, testCase.representation, getJSONRepr(t, patt, ctx, JSONSerializationConfig{
				Pattern: PATHPATTERN_PATTERN,
			}))
		})
	}

}

func TestURLRJSONepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	url := URL("https://example.com/")

	assert.Equal(t, `{"url__value":"https://example.com/"}`, getJSONRepr(t, url, ctx))
	assert.Equal(t, `"https://example.com/"`, getJSONRepr(t, url, ctx, JSONSerializationConfig{
		Pattern: URL_PATTERN,
	}))
}

func TestURLPatternJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []struct {
		value          string
		representation string
	}{
		{"https://example.com/...", `"https://example.com/..."`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt := URLPattern(testCase.value)

			assert.Equal(t, `{"url-pattern__value":`+testCase.representation+"}", getJSONRepr(t, patt, ctx))
			assert.Equal(t, testCase.representation, getJSONRepr(t, patt, ctx, JSONSerializationConfig{
				Pattern: URLPATTERN_PATTERN,
			}))
		})
	}
}

func TestHostJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	host := Host("https://example.com")

	assert.Equal(t, `{"host__value":"https://example.com"}`, getJSONRepr(t, host, ctx))
	assert.Equal(t, `"https://example.com"`, getJSONRepr(t, host, ctx, JSONSerializationConfig{
		Pattern: HOST_PATTERN,
	}))
}

func TestHostPatternJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []struct {
		value          string
		representation string
	}{
		{"https://example.com", `"https://example.com"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt := HostPattern(testCase.value)

			assert.Equal(t, `{"host-pattern__value":`+testCase.representation+"}", getJSONRepr(t, patt, ctx))
			assert.Equal(t, testCase.representation, getJSONRepr(t, patt, ctx, JSONSerializationConfig{
				Pattern: HOSTPATTERN_PATTERN,
			}))
		})
	}
}

func TestEmailAddressJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	testCases := []string{"foo@example.com", "foo.e.9@example.com", "foo+e%9@example.com", "foo%e+9@example.com"}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			addr := EmailAddress(testCase)

			assert.Equal(t, `{"emailaddr__value":"`+testCase+`"}`, getJSONRepr(t, addr, ctx))
			assert.Equal(t, `"`+testCase+`"`, getJSONRepr(t, addr, ctx, JSONSerializationConfig{
				Pattern: EMAIL_ADDR_PATTERN,
			}))
		})
	}

}

func TestIdentifierJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	ident := Identifier("a")

	assert.Equal(t, `{"ident__value":"a"}`, getJSONRepr(t, ident, ctx))
	assert.Equal(t, `"a"`, getJSONRepr(t, ident, ctx, JSONSerializationConfig{
		Pattern: IDENT_PATTERN,
	}))
}

func TestCheckedStringJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	//TODO
}

func TestByteCountJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	negative := ByteCount(-1)
	_, err := negative.Write(bytes.NewBuffer(nil), 0)
	assert.ErrorContains(t, err, "invalid byte rate")

	for _, testCase := range byteCountReprTestCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			assert.Equal(t, `{"byte-count__value":"`+testCase.representation+`"}`, getJSONRepr(t, testCase.value, ctx))
			assert.Equal(t, `"`+testCase.representation+`"`, getJSONRepr(t, testCase.value, ctx, JSONSerializationConfig{
				Pattern: BYTECOUNT_PATTERN,
			}))
		})
	}
}

func TestLineCountJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	n := LineCount(3)

	assert.Equal(t, `{"line-count__value":"3ln"}`, getJSONRepr(t, n, ctx))
	assert.Equal(t, `"3ln"`, getJSONRepr(t, n, ctx, JSONSerializationConfig{
		Pattern: LINECOUNT_PATTERN,
	}))
}

var byteRateJSONReprTestCases = []struct {
	value          ByteRate
	representation string
}{
	{3, "3"},
	{1_000, "1000"},
	{1_001, "1001"},
	{999_000, "999000"},
	{1_000_000, "1000000"},
	{1_001_000, "1001000"},
	{999_000_000, "999000000"},
	{1_000_000_000, "1000000000"},
	{1_001_000_000, "1001000000"},
	{1_001_001_000, "1001001000"},
	{1_001_001_001, "1001001001"},
}

func TestByteRateJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	negative := ByteRate(-1)
	_, err := negative.write(bytes.NewBuffer(nil))
	assert.ErrorIs(t, err, ErrNegByteRate)

	for _, testCase := range byteRateJSONReprTestCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			assert.Equal(t, `{"byte-rate__value":`+testCase.representation+"}", getJSONRepr(t, testCase.value, ctx))
			assert.Equal(t, testCase.representation, getJSONRepr(t, testCase.value, ctx, JSONSerializationConfig{
				Pattern: BYTERATE_PATTERN,
			}))
		})
	}
}

var freqJSONReprTestCases = []struct {
	value                  Frequency
	expectedRepresentation string
}{
	{3, "3"},
	{1_000, "1000"},
	{1_001, "1001"},
	{999_000, "999000"},
	{1_000_000, "1000000"},
	{1_001_000, "1001000"},
	{999_000_000, "999000000"},
	{1_000_000_000, "1000000000"},
	{1_001_000_000, "1001000000"},
	{1_001_001_000, "1001001000"},
	{1_001_001_001, "1001001001"},
}

func TestFrequencyJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	for _, testCase := range freqJSONReprTestCases {

		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			assert.Equal(t, `{"frequency__value":`+testCase.expectedRepresentation+`}`, getJSONRepr(t, testCase.value, ctx))
			assert.Equal(t, testCase.expectedRepresentation, getJSONRepr(t, testCase.value, ctx, JSONSerializationConfig{
				Pattern: FREQUENCY_PATTERN,
			}))
		})

	}
}

var durationJSONReprTestCases = []struct {
	value          Duration
	representation string
}{

	{Duration(time.Millisecond), "0.001"},
	{Duration(300 * time.Millisecond), "0.3"},
	{Duration(300 * time.Millisecond), "0.3"},
	{Duration(999 * time.Millisecond), "0.999"},
	{Duration(time.Second), "1"},
	{Duration(time.Second + time.Millisecond), "1.001"},
	{Duration(59 * time.Second), "59"},
	{Duration(time.Minute), "60"},
	{Duration(time.Minute + time.Millisecond), "60.001"},
	{Duration(time.Minute + time.Second), "61"},
	{Duration(59 * time.Minute), "3540"},
	{Duration(time.Hour), "3600"},
	{Duration(1000 * time.Hour), "3600000"},
	{Duration(time.Hour + time.Millisecond), "3600.001"},
	{Duration(time.Hour + time.Second), "3601"},
}

func TestDurationJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	for _, testCase := range durationJSONReprTestCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			assert.Equal(t, `{"duration__value":`+testCase.representation+`}`, getJSONRepr(t, testCase.value, ctx))
			assert.Equal(t, testCase.representation, getJSONRepr(t, testCase.value, ctx, JSONSerializationConfig{
				Pattern: DURATION_PATTERN,
			}))
		})
	}
}

func TestRuneRangeJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	runeRange := RuneRange{Start: 'a', End: 'z'}

	assert.Equal(t, `{"rune-range__value":{"start":"a","end":"z"}}`, getJSONRepr(t, runeRange, ctx))
	assert.Equal(t, `{"start":"a","end":"z"}`, getJSONRepr(t, runeRange, ctx, JSONSerializationConfig{
		Pattern: RUNE_RANGE_PATTERN,
	}))
}

func TestQuantityRangeJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	//TODO
}

func TestIntRangeJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("known start", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		intRange := IntRange{start: 0, end: 100, step: 1}

		assert.Equal(t, `{"int-range__value":{"start":0,"end":100}}`, getJSONRepr(t, intRange, ctx))
		assert.Equal(t, `{"start":0,"end":100}`, getJSONRepr(t, intRange, ctx, JSONSerializationConfig{
			Pattern: INT_RANGE_PATTERN,
		}))
	})

	t.Run("unknown start", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		intRange := IntRange{start: 0, end: 100, unknownStart: true, step: 1}

		assert.Equal(t, `{"int-range__value":{"end":100}}`, getJSONRepr(t, intRange, ctx))
		assert.Equal(t, `{"end":100}`, getJSONRepr(t, intRange, ctx, JSONSerializationConfig{
			Pattern: INT_RANGE_PATTERN,
		}))
	})

}

func TestFloatRangeJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("known start", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		floatRange := FloatRange{start: 0, end: 100, inclusiveEnd: true}

		assert.Equal(t, `{"float-range__value":{"start":0,"end":100}}`, getJSONRepr(t, floatRange, ctx))
		assert.Equal(t, `{"start":0,"end":100}`, getJSONRepr(t, floatRange, ctx, JSONSerializationConfig{
			Pattern: FLOAT_RANGE_PATTERN,
		}))
	})

	t.Run("unknown start", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		floatRange := FloatRange{start: 0, end: 100, unknownStart: true, inclusiveEnd: true}

		assert.Equal(t, `{"float-range__value":{"end":100}}`, getJSONRepr(t, floatRange, ctx))
		assert.Equal(t, `{"end":100}`, getJSONRepr(t, floatRange, ctx, JSONSerializationConfig{
			Pattern: FLOAT_RANGE_PATTERN,
		}))
	})

	t.Run("exclusive end", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		floatRange := FloatRange{start: 0, end: 100, inclusiveEnd: false}

		assert.Equal(t, `{"float-range__value":{"start":0,"exclusiveEnd":100}}`, getJSONRepr(t, floatRange, ctx))
		assert.Equal(t, `{"start":0,"exclusiveEnd":100}`, getJSONRepr(t, floatRange, ctx, JSONSerializationConfig{
			Pattern: FLOAT_RANGE_PATTERN,
		}))
	})
}

func TestTreedataJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	//TODO

}

func TestNamedSegmentPathPatternJSONRepresentation(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	//TODO
}

func getJSONRepr(t *testing.T, v Serializable, ctx *Context, reprConfig ...JSONSerializationConfig) string {
	if reprConfig == nil {
		reprConfig = append(reprConfig, JSONSerializationConfig{
			ReprConfig: &ReprConfig{AllVisible: true},
		})
	}

	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	err := v.WriteJSONRepresentation(ctx, stream, reprConfig[0], 0)
	if err != nil {
		assert.FailNow(t, "failed to get representation: "+err.Error())
	}
	return string(stream.Buffer())
}
