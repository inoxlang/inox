package core

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

// TODO: move into tests
var reprTestCtx = NewContext(ContextConfig{})
var reptTestState = NewGlobalState(reprTestCtx)

func TestNilRepresentation(t *testing.T) {
	assert.True(t, Nil.HasRepresentation(nil, nil))
	assert.Equal(t, "nil", getRepr(t, Nil, reprTestCtx))
	node := assertParseExpression(t, "nil")
	assert.Equal(t, Nil, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestBoolRepresentation(t *testing.T) {
	assert.True(t, True.HasRepresentation(nil, nil))
	assert.Equal(t, "true", getRepr(t, True, reprTestCtx))
	node := assertParseExpression(t, "true")
	assert.Equal(t, True, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestRuneRepresentation(t *testing.T) {
	assert.True(t, Rune('a').HasRepresentation(nil, nil))
	assert.Equal(t, "'a'", getRepr(t, Rune('a'), reprTestCtx))
	node := assertParseExpression(t, "'a'")
	assert.Equal(t, Rune('a'), utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestIntRepresentation(t *testing.T) {
	assert.True(t, Int(2).HasRepresentation(nil, nil))
	assert.Equal(t, "2", getRepr(t, Int(2), reprTestCtx))
	node := assertParseExpression(t, "2")
	assert.Equal(t, Int(2), utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestFloatRepresentation(t *testing.T) {
	testCases := []struct {
		value          Float
		representation string
	}{
		{1.0, "1.0"},
		{1.001, "1.001"},
		{100.0, "100.0"},
		{100.00, "100.0"},
		{100.001, "100.001"},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			assert.True(t, testCase.value.HasRepresentation(nil, nil))
			repr := getRepr(t, testCase.value, reprTestCtx)
			assert.Equal(t, testCase.representation, repr)

			node := assertParseExpression(t, repr)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestStrRepresentation(t *testing.T) {
	s := Str("a\nb")

	assert.True(t, s.HasRepresentation(nil, nil))
	expectedRepr := `"a\nb"`
	assert.Equal(t, expectedRepr, getRepr(t, s, reprTestCtx))
	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, s, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestObjectRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		obj := &Object{}
		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		assert.Equal(t, `{}`, getRepr(t, obj, reprTestCtx))
		node := assertParseExpression(t, `{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		obj := objFrom(ValMap{"a\nb": Int(1)})
		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `{"a\nb":1}`
		assert.Equal(t, expectedRepr, getRepr(t, obj, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys", func(t *testing.T) {
		obj := objFrom(ValMap{"a\nb": Int(1), "c\nd": Int(2)})
		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `{"a\nb":1,"c\nd":2}`
		repr := getRepr(t, obj, reprTestCtx)
		if repr[2] == 'c' {
			expectedRepr = `{"c\nd":2,"a\nb":1}`
		}
		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one of entry's value has no representation", func(t *testing.T) {
		obj := objFrom(ValMap{"a\nb": &Reader{wrapped: bytes.NewReader(nil)}})
		assert.False(t, obj.HasRepresentation(map[uintptr]int{}, nil))
	})

	t.Run("deep", func(t *testing.T) {
		obj := objFrom(ValMap{
			"a": NewWrappedValueList(Int(1), objFrom(ValMap{"b": Int(2)})),
		})
		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `{"a":[1,{"b":2}]}`
		assert.Equal(t, expectedRepr, getRepr(t, obj, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("cycle", func(t *testing.T) {
		obj := &Object{}
		obj.SetProp(reprTestCtx, "self", obj)
		assert.False(t, obj.HasRepresentation(map[uintptr]int{}, nil))
	})

	t.Run("sensitive properties", func(t *testing.T) {
		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})
		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `{"a":1}`

		assert.Equal(t, expectedRepr, getRepr(t, obj, reprTestCtx, &ReprConfig{
			AllVisible: false,
		}))
	})

	t.Run("sensitive properties: config with .allVisible == true", func(t *testing.T) {
		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})
		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `{"a":1,"e":a@mail.com,"password":"mypassword"}`

		assert.Equal(t, expectedRepr, getRepr(t, obj, reprTestCtx, &ReprConfig{
			AllVisible: true,
		}))
	})

	t.Run("sensitive properties: value visibility with all keys to public", func(t *testing.T) {
		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		initializeObjectVisibility(obj, &ValueVisibility{
			publicKeys: []string{"a", "password", "e"},
		})

		assert.True(t, obj.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `{"a":1,"e":a@mail.com,"password":"mypassword"}`

		assert.Equal(t, expectedRepr, getRepr(t, obj, reprTestCtx, &ReprConfig{
			AllVisible: false,
		}))
	})
}

func TestRecordRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		rec := NewRecordFromMap(nil)
		assert.True(t, rec.HasRepresentation(map[uintptr]int{}, nil))
		assert.Equal(t, `#{}`, getRepr(t, rec, reprTestCtx))
		node := assertParseExpression(t, `#{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		rec := NewRecordFromMap(ValMap{"a\nb": Int(1)})
		assert.True(t, rec.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `#{"a\nb":1}`
		assert.Equal(t, expectedRepr, getRepr(t, rec, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys", func(t *testing.T) {
		rec := NewRecordFromMap(ValMap{"a\nb": Int(1), "c\nd": Int(2)})
		assert.True(t, rec.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `#{"a\nb":1,"c\nd":2}`
		repr := getRepr(t, rec, reprTestCtx)
		if repr[2] == 'c' {
			expectedRepr = `#{"c\nd":2,"a\nb":1}`
		}
		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("deep", func(t *testing.T) {
		rec := NewRecordFromMap(ValMap{
			"a": &Tuple{
				elements: []Value{Int(1), NewRecordFromMap(ValMap{"b": Int(2)})},
			},
		})
		assert.True(t, rec.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `#{"a":#[1,#{"b":2}]}`
		assert.Equal(t, expectedRepr, getRepr(t, rec, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("sensitive properties", func(t *testing.T) {
		rec := NewRecordFromMap(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})
		assert.True(t, rec.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `#{"a":1}`

		assert.Equal(t, expectedRepr, getRepr(t, rec, reprTestCtx, &ReprConfig{
			AllVisible: false,
		}))
	})

}

func TestDictRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dict := NewDictionary(nil)
		assert.True(t, dict.HasRepresentation(map[uintptr]int{}, nil))
		assert.Equal(t, `:{}`, getRepr(t, dict, reprTestCtx))
		node := assertParseExpression(t, ":{}")

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, dict, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single string key", func(t *testing.T) {
		dict := NewDictionary(map[string]Value{"\"a\\nb\"": Int(1)})
		assert.True(t, dict.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `:{"a\nb":1}`
		assert.Equal(t, expectedRepr, getRepr(t, dict, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, dict, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys: one string & a path", func(t *testing.T) {
		dict := NewDictionary(map[string]Value{"\"a\\nb\"": Int(1), "./path": Int(2)})
		assert.True(t, dict.HasRepresentation(map[uintptr]int{}, nil))

		repr := getRepr(t, dict, reprTestCtx)
		var expectedRepr = `:{"a\nb":1,./path:2}`
		if repr[2] == '.' {
			expectedRepr = `:{./path:2,"a\nb":1}`
		}

		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, dict, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one of entry's value has no representation", func(t *testing.T) {
		dict := NewDictionary(map[string]Value{"\"a\\nb\"": &Reader{wrapped: bytes.NewReader(nil)}})
		assert.False(t, dict.HasRepresentation(map[uintptr]int{}, nil))
	})

	t.Run("cycle", func(t *testing.T) {
		dict := NewDictionary(nil)
		dict.Entries["self"] = dict
		dict.Keys["self"] = Str("self")

		assert.False(t, dict.HasRepresentation(map[uintptr]int{}, nil))
	})
}

func TestKeyListRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		list := KeyList{}
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		assert.Equal(t, `.{}`, getRepr(t, list, reprTestCtx))
		node := assertParseExpression(t, `.{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		list := KeyList{"a"}
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `.{a}`
		assert.Equal(t, expectedRepr, getRepr(t, list, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys: one string & a path", func(t *testing.T) {
		list := KeyList{"a", "b"}
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `.{a,b}`
		assert.Equal(t, expectedRepr, getRepr(t, list, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestListRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		list := NewWrappedValueList()
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `[]`
		assert.Equal(t, expectedRepr, getRepr(t, list, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single element", func(t *testing.T) {
		list := NewWrappedValueList(Int(2))
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `[2]`
		assert.Equal(t, expectedRepr, getRepr(t, list, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two elements", func(t *testing.T) {
		list := NewWrappedValueList(Int(2), Path("./path"))
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `[2,./path]`
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one element has no representation", func(t *testing.T) {
		list := NewWrappedValueList(Int(2), &Reader{wrapped: bytes.NewReader(nil)})
		assert.False(t, list.HasRepresentation(map[uintptr]int{}, nil))
	})

	t.Run("deep", func(t *testing.T) {
		list := NewWrappedValueList(NewWrappedValueList(Int(2), objFrom(ValMap{"a": Int(1)})))
		assert.True(t, list.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `[[2,{"a":1}]]`
		assert.Equal(t, expectedRepr, getRepr(t, list, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("cycle", func(t *testing.T) {
		list := NewWrappedValueList(Int(0))
		list.set(NewContext(ContextConfig{}), 0, list)
		assert.False(t, list.HasRepresentation(map[uintptr]int{}, nil))
	})

}

func TestObjectPatternRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		patt := NewInexactObjectPattern(map[string]Pattern{})
		assert.True(t, patt.HasRepresentation(map[uintptr]int{}, nil))
		assert.Equal(t, `%{}`, getRepr(t, patt, reprTestCtx))
		node := assertParseExpression(t, `%{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		patt := NewInexactObjectPattern(map[string]Pattern{
			"a\nb": NewExactValuePattern(Int(1)),
		})
		assert.True(t, patt.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%{"a\nb":%(1)}`
		assert.Equal(t, expectedRepr, getRepr(t, patt, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys", func(t *testing.T) {
		patt := NewInexactObjectPattern(map[string]Pattern{
			"a\nb": NewExactValuePattern(Int(1)),
			"c\nd": NewInexactObjectPattern(map[string]Pattern{}),
		})

		assert.True(t, patt.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%{"a\nb":%(1),"c\nd":%{}}`
		repr := getRepr(t, patt, reprTestCtx)
		// if repr[2] == 'c' {
		// 	expectedRepr = `{"c\nd":2,"a\nb":1}`
		// }
		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one of entry's value has no representation", func(t *testing.T) {
		//TODO
	})

	t.Run("deep", func(t *testing.T) {
		patt := NewInexactObjectPattern(map[string]Pattern{
			"a": NewListPattern([]Pattern{
				NewExactValuePattern(Int(1)),
				NewExactValuePattern(NewRecordFromMap(ValMap{"b": Int(2)})),
			}),
		})

		assert.True(t, patt.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%{"a":%[%(1),%(#{"b":2})]}`
		assert.Equal(t, expectedRepr, getRepr(t, patt, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestListPatternRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		pattern := NewListPattern(nil)
		assert.True(t, pattern.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%[]`
		assert.Equal(t, expectedRepr, getRepr(t, pattern, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single element", func(t *testing.T) {
		pattern := NewListPattern([]Pattern{NewExactValuePattern(Int(2))})
		assert.True(t, pattern.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%[%(2)]`
		assert.Equal(t, expectedRepr, getRepr(t, pattern, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two elements", func(t *testing.T) {
		pattern := NewListPattern([]Pattern{
			NewExactValuePattern(Int(2)),
			NewExactValuePattern(Path("./path")),
		})
		assert.True(t, pattern.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%[%(2),%(./path)]`
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one element has no representation", func(t *testing.T) {
		//TODO
	})

	t.Run("deep", func(t *testing.T) {
		pattern := NewListPattern([]Pattern{
			NewExactValuePattern(NewTuple([]Value{
				Int(2),
				NewRecordFromMap(ValMap{"a": Int(1)}),
			})),
		})

		assert.True(t, pattern.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `%[%(#[2,#{"a":1}])]`
		assert.Equal(t, expectedRepr, getRepr(t, pattern, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestByteSliceRepresentation(t *testing.T) {
	assert.True(t, (&ByteSlice{}).HasRepresentation(nil, nil))
	assert.Equal(t, "0x[]", getRepr(t, &ByteSlice{}, reprTestCtx))

	assert.True(t, (&ByteSlice{Bytes: []byte{0x12}}).HasRepresentation(nil, nil))
	assert.Equal(t, "0x[12]", getRepr(t, &ByteSlice{Bytes: []byte{0x12}}, reprTestCtx))
}

func TestOptionRepresentation(t *testing.T) {
	t.Run("single letter name", func(t *testing.T) {
		opt := Option{Name: "v", Value: True}
		assert.True(t, opt.HasRepresentation(nil, nil))
		expectedRepr := `-v`
		assert.Equal(t, expectedRepr, getRepr(t, opt, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, opt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("multi letter name", func(t *testing.T) {
		opt := Option{Name: "verbose", Value: True}
		assert.True(t, opt.HasRepresentation(nil, nil))
		expectedRepr := `--verbose`
		assert.Equal(t, expectedRepr, getRepr(t, opt, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, opt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("value has no representation", func(t *testing.T) {
		opt := Option{Name: "verbose", Value: &Reader{wrapped: bytes.NewReader(nil)}}
		assert.False(t, opt.HasRepresentation(nil, nil))
	})

}

func TestPathRepresentation(t *testing.T) {

	testCases := []struct {
		value          string
		representation string
	}{
		{"/a", "/a"},
		{"/[a-z]", "/`[a-z]`"},
		{"/]", "/`]`"},
		{"/\\]", "/`\\]`"},
		{"/ ", "/` `"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			pth := Path(testCase.value)

			assert.True(t, pth.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, pth, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, pth, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}

}
func TestPathPatternRepresentation(t *testing.T) {

	testCases := []struct {
		value          string
		representation string
	}{
		{"/...", "%/..."},
		{"/[a-z]", "%/`[a-z]`"},
		{"/]", "%/`]`"},
		{"/\\]", "%/`\\]`"},
		{"/ ", "%/` `"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			patt := PathPattern(testCase.value)

			assert.True(t, patt.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, patt, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, patt, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}

}

func TestURLRepresentation(t *testing.T) {
	url := URL("https://example.com/")
	assert.True(t, url.HasRepresentation(nil, nil))
	expectedRepr := "https://example.com/"
	assert.Equal(t, expectedRepr, getRepr(t, url, reprTestCtx))

	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, url, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
	//TODO: test more complex cases
}

func TestURLPatternRepresentation(t *testing.T) {
	testCases := []struct {
		value          string
		representation string
	}{
		{"https://example.com/...", "%https://example.com/..."},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			patt := URLPattern(testCase.value)

			assert.True(t, patt.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, patt, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, patt, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestHostRepresentation(t *testing.T) {
	host := Host("https://example.com")
	assert.True(t, host.HasRepresentation(nil, nil))
	expectedRepr := "https://example.com"
	assert.Equal(t, expectedRepr, getRepr(t, host, reprTestCtx))

	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, host, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
	//TODO: test more complex cases
}

func TestHostPatternRepresentation(t *testing.T) {
	testCases := []struct {
		value          string
		representation string
	}{
		{"https://example.com", "%https://example.com"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			patt := HostPattern(testCase.value)

			assert.True(t, patt.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, patt, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, patt, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestEmailAddressRepresentation(t *testing.T) {

	testCases := []string{"foo@example.com", "foo.e.9@example.com", "foo+e%9@example.com", "foo%e+9@example.com"}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			addr := EmailAddress(testCase)
			assert.True(t, addr.HasRepresentation(nil, nil))
			expectedRepr := testCase
			assert.Equal(t, expectedRepr, getRepr(t, addr, reprTestCtx))

			node := assertParseExpression(t, expectedRepr)
			assert.Equal(t, addr, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}

}

func TestIdentifierRepresentation(t *testing.T) {
	ident := Identifier("a")
	assert.True(t, ident.HasRepresentation(nil, nil))
	expectedRepr := "#a"

	assert.Equal(t, expectedRepr, getRepr(t, ident, reprTestCtx))
	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, ident, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestCheckedStringRepresentation(t *testing.T) {
	pattern := &ExactValuePattern{value: Str("foo")}
	str := CheckedString{str: "foo", matchingPatternName: "ident_name", matchingPattern: pattern}
	assert.True(t, str.HasRepresentation(nil, nil))
	expectedRepr := "%ident_name`foo`"

	assert.Equal(t, expectedRepr, getRepr(t, str, reprTestCtx))
	node := assertParseExpression(t, expectedRepr)

	ctx := NewContext(ContextConfig{})
	ctx.AddNamedPattern("ident_name", pattern)

	state := NewTreeWalkState(ctx)
	assert.Equal(t, str, utils.Must(TreeWalkEval(node, state)))
	//TODO: test more complex cases
}

func TestByteCountRepresentation(t *testing.T) {
	negative := ByteCount(-1)
	assert.ErrorIs(t, negative.WriteRepresentation(reprTestCtx, nil, nil, nil), ErrNoRepresentation)

	testCases := []struct {
		value          ByteCount
		representation string
	}{
		{3, "3B"},
		{1_000, "1kB"},
		{1_001, "1001B"},
		{999_000, "999kB"},
		{1_000_000, "1MB"},
		{1_001_000, "1001kB"},
		{999_000_000, "999MB"},
		{1_000_000_000, "1GB"},
		{1_001_000_000, "1001MB"},
		{1_001_001_000, "1001001kB"},
		{1_001_001_001, "1001001001B"},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			assert.True(t, testCase.value.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, testCase.value, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestLineCountRepresentation(t *testing.T) {
	n := LineCount(3)
	assert.True(t, n.HasRepresentation(nil, nil))
	expectedRepr := "3ln"
	assert.Equal(t, expectedRepr, getRepr(t, n, reprTestCtx))

	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, n, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
	//TODO: test more complex cases
}

func TestByteRateRepresentation(t *testing.T) {
	negative := ByteRate(-1)
	assert.ErrorIs(t, negative.WriteRepresentation(reprTestCtx, nil, nil, nil), ErrNoRepresentation)

	testCases := []struct {
		value          ByteRate
		representation string
	}{
		{3, "3B/s"},
		{1_000, "1kB/s"},
		{1_001, "1001B/s"},
		{999_000, "999kB/s"},
		{1_000_000, "1MB/s"},
		{1_001_000, "1001kB/s"},
		{999_000_000, "999MB/s"},
		{1_000_000_000, "1GB/s"},
		{1_001_000_000, "1001MB/s"},
		{1_001_001_000, "1001001kB/s"},
		{1_001_001_001, "1001001001B/s"},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			assert.True(t, testCase.value.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, testCase.value, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestSimpleRateRepresentation(t *testing.T) {
	negative := SimpleRate(-1)
	assert.ErrorIs(t, negative.WriteRepresentation(reprTestCtx, nil, nil, nil), ErrNoRepresentation)

	testCases := []struct {
		value          SimpleRate
		representation string
	}{
		{3, "3x/s"},
		{1_000, "1kx/s"},
		{1_001, "1001x/s"},
		{999_000, "999kx/s"},
		{1_000_000, "1Mx/s"},
		{1_001_000, "1001kx/s"},
		{999_000_000, "999Mx/s"},
		{1_000_000_000, "1Gx/s"},
		{1_001_000_000, "1001Mx/s"},
		{1_001_001_000, "1001001kx/s"},
		{1_001_001_001, "1001001001x/s"},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			assert.True(t, testCase.value.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, testCase.value, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestDurationRepresentation(t *testing.T) {
	negative := SimpleRate(-1)
	assert.ErrorIs(t, negative.WriteRepresentation(reprTestCtx, nil, nil, nil), ErrNoRepresentation)

	testCases := []struct {
		value          Duration
		representation string
	}{

		{Duration(time.Millisecond), "1ms"},
		{Duration(300 * time.Millisecond), "300ms"},
		{Duration(300 * time.Millisecond), "300ms"},
		{Duration(999 * time.Millisecond), "999ms"},
		{Duration(time.Second), "1s"},
		{Duration(time.Second + time.Millisecond), "1s1ms"},
		{Duration(59 * time.Second), "59s"},
		{Duration(time.Minute), "1mn"},
		{Duration(time.Minute + time.Millisecond), "1mn1ms"},
		{Duration(time.Minute + time.Second), "1mn1s"},
		{Duration(59 * time.Minute), "59mn"},
		{Duration(time.Hour), "1h"},
		{Duration(1000 * time.Hour), "1000h"},
		{Duration(time.Hour + time.Millisecond), "1h1ms"},
		{Duration(time.Hour + time.Second), "1h1s"},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {
			assert.True(t, testCase.value.HasRepresentation(nil, nil))
			assert.Equal(t, testCase.representation, getRepr(t, testCase.value, reprTestCtx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestRuneRangeRepresentation(t *testing.T) {
	runeRange := RuneRange{Start: 'a', End: 'z'}
	assert.True(t, runeRange.HasRepresentation(nil, nil))
	expectedRepr := "'a'..'z'"
	assert.Equal(t, expectedRepr, getRepr(t, runeRange, reprTestCtx))

	node := assertParseExpression(t, expectedRepr)
	state := NewTreeWalkState(NewContext(ContextConfig{}))
	assert.Equal(t, runeRange, utils.Must(TreeWalkEval(node, state)))
}

func TestQuantityRangeRepresentation(t *testing.T) {
	t.Run("unknown start", func(t *testing.T) {
		qtyRange := QuantityRange{Start: nil, End: Duration(time.Hour), inclusiveEnd: true, unknownStart: true}
		assert.True(t, qtyRange.HasRepresentation(nil, nil))
		expectedRepr := "..1h"
		assert.Equal(t, expectedRepr, getRepr(t, qtyRange, reprTestCtx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, qtyRange, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("known start", func(t *testing.T) {
		//TODO: fix parsing of quantity range with representable start & end
		t.Skip()
	})
}

func TestIntRangeRepresentation(t *testing.T) {
	t.Run("known start", func(t *testing.T) {
		intRange := IntRange{Start: 0, End: 100, inclusiveEnd: true, Step: 1}
		assert.True(t, intRange.HasRepresentation(nil, nil))
		expectedRepr := "0..100"
		assert.Equal(t, expectedRepr, getRepr(t, intRange, reprTestCtx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, intRange, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("unknown start", func(t *testing.T) {
		intRange := IntRange{Start: 0, End: 100, unknownStart: true, inclusiveEnd: true, Step: 1}
		assert.True(t, intRange.HasRepresentation(nil, nil))
		expectedRepr := "..100"
		assert.Equal(t, expectedRepr, getRepr(t, intRange, reprTestCtx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, intRange, utils.Must(TreeWalkEval(node, state)))
	})
}

func TestUdataRepresentation(t *testing.T) {
	t.Run("only root", func(t *testing.T) {
		udata := &UData{Root: Int(1)}
		assert.True(t, udata.HasRepresentation(map[uintptr]int{}, nil))
		assert.Equal(t, `udata 1{}`, getRepr(t, udata, reprTestCtx))
		node := assertParseExpression(t, `udata 1{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single hiearchy entry with no children", func(t *testing.T) {
		udata := &UData{Root: Int(1), HiearchyEntries: []UDataHiearchyEntry{{Value: Int(2)}}}

		assert.True(t, udata.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `udata 1{2}`
		assert.Equal(t, expectedRepr, getRepr(t, udata, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two hiearchy entries with no children", func(t *testing.T) {
		udata := &UData{
			Root: Int(1),
			HiearchyEntries: []UDataHiearchyEntry{
				{Value: Int(2)},
				{Value: Int(3)},
			},
		}
		assert.True(t, udata.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `udata 1{2,3}`
		repr := getRepr(t, udata, reprTestCtx)
		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one of entry's value has no representation", func(t *testing.T) {
		udata := &UData{Root: Int(1), HiearchyEntries: []UDataHiearchyEntry{{Value: &Routine{}}}}
		assert.False(t, udata.HasRepresentation(map[uintptr]int{}, nil))
	})

	t.Run("deep", func(t *testing.T) {
		udata := &UData{
			Root: Int(1),
			HiearchyEntries: []UDataHiearchyEntry{
				{Value: Int(2)},
				{
					Value: Int(3),
					Children: []UDataHiearchyEntry{
						{Value: Int(4)},
					},
				},
			},
		}
		assert.True(t, udata.HasRepresentation(map[uintptr]int{}, nil))
		expectedRepr := `udata 1{2,3{4}}`
		assert.Equal(t, expectedRepr, getRepr(t, udata, reprTestCtx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestNamedSegmentPathPatternRepresentation(t *testing.T) {
	assert.True(t, (&NamedSegmentPathPattern{}).HasRepresentation(nil, nil))
	//TODO: finish test
}

func assertParseExpression(t *testing.T, s string) parse.Node {
	n, ok := parse.ParseExpression(s)
	assert.True(t, ok, "failed to parsed '"+s+"'")
	return n
}

func getRepr(t *testing.T, v Value, ctx *Context, reprConfig ...*ReprConfig) string {
	buff := bytes.NewBuffer(nil)
	encountered := map[uintptr]int{}

	if reprConfig == nil {
		reprConfig = append(reprConfig, &ReprConfig{AllVisible: true})
	}

	err := v.WriteRepresentation(ctx, buff, encountered, reprConfig[0])
	if err != nil {
		assert.FailNow(t, "failed to get representation: "+err.Error())
	}
	return buff.String()
}
