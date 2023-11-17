package core

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestNilRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "nil", getReprAllVisible(t, Nil, ctx))
	node := assertParseExpression(t, "nil")
	assert.Equal(t, Nil, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestBoolRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "true", getReprAllVisible(t, True, ctx))
	node := assertParseExpression(t, "true")
	assert.Equal(t, True, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestRuneRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "'a'", getReprAllVisible(t, Rune('a'), ctx))
	node := assertParseExpression(t, "'a'")
	assert.Equal(t, Rune('a'), utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestIntRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "2", getReprAllVisible(t, Int(2), ctx))
	node := assertParseExpression(t, "2")
	assert.Equal(t, Int(2), utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestFloatRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

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

			repr := getReprAllVisible(t, testCase.value, ctx)
			assert.Equal(t, testCase.representation, repr)

			node := assertParseExpression(t, repr)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestStrRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("newline character", func(t *testing.T) {
		s := Str("a\nb")

		expectedRepr := `"a\nb"`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, s, ctx))
		node := assertParseExpression(t, expectedRepr)
		assert.Equal(t, s, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
	})

	t.Run("html unsafe characters", func(t *testing.T) {
		s := Str("<script></script>")

		expectedRepr := `"<script></script>"`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, s, ctx))
		node := assertParseExpression(t, expectedRepr)
		assert.Equal(t, s, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
	})
}

func TestObjectRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := &Object{}

		assert.Equal(t, `{}`, getReprAllVisible(t, obj, ctx))
		node := assertParseExpression(t, `{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{"a\nb": Int(1)})

		expectedRepr := `{"a\nb":1}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, obj, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{"a\nb": Int(1), "c\nd": Int(2)})

		expectedRepr := `{"a\nb":1,"c\nd":2}`
		repr := getReprAllVisible(t, obj, ctx)
		if repr[2] == 'c' {
			expectedRepr = `{"c\nd":2,"a\nb":1}`
		}
		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{
			"a": NewWrappedValueList(Int(1), objFrom(ValMap{"b": Int(2)})),
		})

		expectedRepr := `{"a":[1,{"b":2}]}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, obj, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, obj, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("cycle", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := &Object{}
		obj.SetProp(ctx, "self", obj)
	})

	t.Run("sensitive properties", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		expectedRepr := `{"a":1}`

		assert.Equal(t, expectedRepr, getReprAllVisible(t, obj, ctx, &ReprConfig{
			AllVisible: false,
		}))
	})

	t.Run("sensitive properties: config with .allVisible == true", func(t *testing.T) {
		reprTestCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer reprTestCtx.CancelGracefully()

		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		expectedRepr := `{"a":1,"e":EmailAddress"a@mail.com","password":"mypassword"}`

		assert.Equal(t, expectedRepr, getReprAllVisible(t, obj, reprTestCtx, &ReprConfig{
			AllVisible: true,
		}))
	})

	t.Run("sensitive properties: value visibility with all keys to public", func(t *testing.T) {
		reprTestCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer reprTestCtx.CancelGracefully()

		obj := objFrom(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		initializeObjectVisibility(obj, &ValueVisibility{
			publicKeys: []string{"a", "password", "e"},
		})

		expectedRepr := `{"a":1,"e":EmailAddress"a@mail.com","password":"mypassword"}`

		assert.Equal(t, expectedRepr, getReprAllVisible(t, obj, reprTestCtx, &ReprConfig{
			AllVisible: false,
		}))
	})

	t.Run("id", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		obj := objFrom(ValMap{})

		url := URL("https://example.com/objects/98484")
		utils.PanicIfErr(obj.SetURLOnce(ctx, url))

		reprTestCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer reprTestCtx.CancelGracefully()

		expectedRepr := `{"_url_":` + string(url) + "}"
		assert.Equal(t, expectedRepr, getReprAllVisible(t, obj, reprTestCtx))

		//parsing the representation & evaluating the AST Nodes is not done
		//because metaproperty keys are not allowed in properties.
	})
}

func TestRecordRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(nil)

		assert.Equal(t, `#{}`, getReprAllVisible(t, rec, ctx))
		node := assertParseExpression(t, `#{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(ValMap{"a\nb": Int(1)})

		reprTestCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer reprTestCtx.CancelGracefully()

		expectedRepr := `#{"a\nb":1}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, rec, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys", func(t *testing.T) {
		rec := NewRecordFromMap(ValMap{"a\nb": Int(1), "c\nd": Int(2)})

		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		expectedRepr := `#{"a\nb":1,"c\nd":2}`
		repr := getReprAllVisible(t, rec, ctx)
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
				elements: []Serializable{Int(1), NewRecordFromMap(ValMap{"b": Int(2)})},
			},
		})

		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		expectedRepr := `#{"a":#[1,#{"b":2}]}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, rec, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, rec, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("sensitive properties", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		rec := NewRecordFromMap(ValMap{
			"a":        Int(1),
			"password": Str("mypassword"),
			"e":        EmailAddress("a@mail.com"),
		})

		expectedRepr := `#{"a":1}`

		assert.Equal(t, expectedRepr, getReprAllVisible(t, rec, ctx, &ReprConfig{
			AllVisible: false,
		}))
	})

}

func TestDictRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		dict := NewDictionary(nil)

		assert.Equal(t, `:{}`, getReprAllVisible(t, dict, ctx))
		node := assertParseExpression(t, ":{}")

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, dict, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single string key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		dict := NewDictionary(map[string]Serializable{"\"a\\nb\"": Int(1)})

		expectedRepr := `:{"a\nb":1}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, dict, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, dict, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys: one string & a path", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		dict := NewDictionary(map[string]Serializable{"\"a\\nb\"": Int(1), "./path": Int(2)})

		repr := getReprAllVisible(t, dict, ctx)
		var expectedRepr = `:{"a\nb":1,./path:2}`
		if repr[2] == '.' {
			expectedRepr = `:{./path:2,"a\nb":1}`
		}

		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, dict, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("cycle", func(t *testing.T) {
		dict := NewDictionary(nil)
		dict.entries["self"] = dict
		dict.keys["self"] = Str("self")
	})
}

func TestKeyListRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := KeyList{}

		assert.Equal(t, `.{}`, getReprAllVisible(t, list, ctx))
		node := assertParseExpression(t, `.{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := KeyList{"a"}

		expectedRepr := `.{a}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, list, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys: one string & a path", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := KeyList{"a", "b"}

		expectedRepr := `.{a,b}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, list, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestListRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList()

		expectedRepr := `[]`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, list, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(Int(2))

		expectedRepr := `[2]`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, list, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(Int(2), Path("./path"))

		expectedRepr := `[2,./path]`
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(NewWrappedValueList(Int(2), objFrom(ValMap{"a": Int(1)})))

		expectedRepr := `[[2,{"a":1}]]`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, list, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, list, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("cycle", func(t *testing.T) {
		list := NewWrappedValueList(Int(0))
		list.set(NewContext(ContextConfig{}), 0, list)

	})

}

func TestObjectPatternRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		patt := NewInexactObjectPattern(map[string]Pattern{})

		assert.Equal(t, `%{}`, getReprAllVisible(t, patt, ctx))
		node := assertParseExpression(t, `%{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		patt := NewInexactObjectPattern(map[string]Pattern{
			"a\nb": NewExactValuePattern(Int(1)),
		})

		expectedRepr := `%{"a\nb":%(1)}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, patt, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two keys", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		patt := NewInexactObjectPattern(map[string]Pattern{
			"a\nb": NewExactValuePattern(Int(1)),
			"c\nd": NewInexactObjectPattern(map[string]Pattern{}),
		})

		expectedRepr := `%{"a\nb":%(1),"c\nd":%{}}`
		repr := getReprAllVisible(t, patt, ctx)
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
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		patt := NewInexactObjectPattern(map[string]Pattern{
			"a": NewListPattern([]Pattern{
				NewExactValuePattern(Int(1)),
				NewExactValuePattern(NewRecordFromMap(ValMap{"b": Int(2)})),
			}),
		})

		expectedRepr := `%{"a":%[%(1),%(#{"b":2})]}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, patt, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, patt, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestListPatternRepresentation(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		pattern := NewListPattern(nil)

		expectedRepr := `%[]`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, pattern, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		pattern := NewListPattern([]Pattern{NewExactValuePattern(Int(2))})

		expectedRepr := `%[%(2)]`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, pattern, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two elements", func(t *testing.T) {
		pattern := NewListPattern([]Pattern{
			NewExactValuePattern(Int(2)),
			NewExactValuePattern(Path("./path")),
		})

		expectedRepr := `%[%(2),%(./path)]`
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("one element has no representation", func(t *testing.T) {
		//TODO
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		pattern := NewListPattern([]Pattern{
			NewExactValuePattern(NewTuple([]Serializable{
				Int(2),
				NewRecordFromMap(ValMap{"a": Int(1)}),
			})),
		})

		expectedRepr := `%[%(#[2,#{"a":1}])]`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, pattern, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, pattern, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestByteSliceRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	assert.Equal(t, "0x[]", getReprAllVisible(t, &ByteSlice{}, ctx))

	assert.Equal(t, "0x[12]", getReprAllVisible(t, &ByteSlice{Bytes: []byte{0x12}}, ctx))
}

func TestOptionRepresentation(t *testing.T) {
	t.Run("single letter name", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		opt := Option{Name: "v", Value: True}

		expectedRepr := `-v`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, opt, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, opt, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("multi letter name", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		opt := Option{Name: "verbose", Value: True}

		expectedRepr := `--verbose`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, opt, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, opt, utils.Must(TreeWalkEval(node, state)))
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
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			pth := Path(testCase.value)

			assert.Equal(t, testCase.representation, getReprAllVisible(t, pth, ctx))

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
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt := PathPattern(testCase.value)

			assert.Equal(t, testCase.representation, getReprAllVisible(t, patt, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, patt, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}

}

func TestURLRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	url := URL("https://example.com/")

	expectedRepr := "https://example.com/"
	assert.Equal(t, expectedRepr, getReprAllVisible(t, url, ctx))

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
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt := URLPattern(testCase.value)

			assert.Equal(t, testCase.representation, getReprAllVisible(t, patt, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, patt, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestHostRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	host := Host("https://example.com")

	expectedRepr := "https://example.com"
	assert.Equal(t, expectedRepr, getReprAllVisible(t, host, ctx))

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
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt := HostPattern(testCase.value)

			assert.Equal(t, testCase.representation, getReprAllVisible(t, patt, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, patt, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestEmailAddressRepresentation(t *testing.T) {

	testCases := []string{"foo@example.com", "foo.e.9@example.com", "foo+e%9@example.com", "foo%e+9@example.com"}
	expectedPartiallyHiddenValues := []string{"f**@example.com", "f******@example.com", "f******@example.com", "f******@example.com"}

	for i, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			addr := EmailAddress(testCase)

			expectedRepr := `EmailAddress"` + testCase + `"`
			assert.Equal(t, expectedRepr, getReprAllVisible(t, addr, ctx))

			expectedPartiallyHiddenRepr := expectedPartiallyHiddenValues[i]
			assert.Equal(t, `EmailAddress"`+expectedPartiallyHiddenRepr+`"`, getReprAllVisible(t, addr, ctx, &ReprConfig{AllVisible: false}))
		})
	}

}

func TestIdentifierRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	ident := Identifier("a")

	expectedRepr := "#a"

	assert.Equal(t, expectedRepr, getReprAllVisible(t, ident, ctx))
	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, ident, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
}

func TestCheckedStringRepresentation(t *testing.T) {
	reprTestCtx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer reprTestCtx.CancelGracefully()

	pattern := &ExactValuePattern{value: Str("foo")}
	str := CheckedString{str: "foo", matchingPatternName: "ident_name", matchingPattern: pattern}

	expectedRepr := "%ident_name`foo`"

	assert.Equal(t, expectedRepr, getReprAllVisible(t, str, reprTestCtx))
	node := assertParseExpression(t, expectedRepr)

	ctx := NewContext(ContextConfig{})
	ctx.AddNamedPattern("ident_name", pattern)

	state := NewTreeWalkState(ctx)
	assert.Equal(t, str, utils.Must(TreeWalkEval(node, state)))
	//TODO: test more complex cases
}

var byteCountReprTestCases = []struct {
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

func TestByteCountRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	negative := ByteCount(-1)
	assert.ErrorContains(t, negative.WriteRepresentation(ctx, nil, nil, 0), "invalid byte rate")

	for _, testCase := range byteCountReprTestCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {

			assert.Equal(t, testCase.representation, getReprAllVisible(t, testCase.value, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestLineCountRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	n := LineCount(3)

	expectedRepr := "3ln"
	assert.Equal(t, expectedRepr, getReprAllVisible(t, n, ctx))

	node := assertParseExpression(t, expectedRepr)
	assert.Equal(t, n, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
	//TODO: test more complex cases
}

var byteRateReprTestCases = []struct {
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

func TestByteRateRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	negative := ByteRate(-1)
	assert.ErrorIs(t, negative.WriteRepresentation(ctx, nil, nil, 0), ErrNoRepresentation)

	for _, testCase := range byteRateReprTestCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {

			assert.Equal(t, testCase.representation, getReprAllVisible(t, testCase.value, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

var simpleRateReprTestCases = []struct {
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

func TestSimpleRateRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	negative := SimpleRate(-1)
	assert.ErrorIs(t, negative.WriteRepresentation(ctx, nil, nil, 0), ErrNoRepresentation)

	for _, testCase := range simpleRateReprTestCases {
		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {

			assert.Equal(t, testCase.representation, getReprAllVisible(t, testCase.value, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

var durationReprTestCases = []struct {
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

func TestDurationRepresentation(t *testing.T) {
	for _, testCase := range durationReprTestCases {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run(strconv.Itoa(int(testCase.value)), func(t *testing.T) {

			assert.Equal(t, testCase.representation, getReprAllVisible(t, testCase.value, ctx))

			node := assertParseExpression(t, testCase.representation)
			assert.Equal(t, testCase.value, utils.Must(evalSimpleValueLiteral(node.(parse.SimpleValueLiteral), nil)))
		})
	}
}

func TestRuneRangeRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	runeRange := RuneRange{Start: 'a', End: 'z'}

	expectedRepr := "'a'..'z'"
	assert.Equal(t, expectedRepr, getReprAllVisible(t, runeRange, ctx))

	node := assertParseExpression(t, expectedRepr)
	state := NewTreeWalkState(NewContext(ContextConfig{}))
	assert.Equal(t, runeRange, utils.Must(TreeWalkEval(node, state)))
}

func TestQuantityRangeRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("unknown start", func(t *testing.T) {
		qtyRange := QuantityRange{start: nil, end: Duration(time.Hour), inclusiveEnd: true, unknownStart: true}

		expectedRepr := "..1h"
		assert.Equal(t, expectedRepr, getReprAllVisible(t, qtyRange, ctx))

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
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		intRange := IntRange{start: 0, end: 100, inclusiveEnd: true, step: 1}

		expectedRepr := "0..100"
		assert.Equal(t, expectedRepr, getReprAllVisible(t, intRange, ctx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, intRange, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("unknown start", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		intRange := IntRange{start: 0, end: 100, unknownStart: true, inclusiveEnd: true, step: 1}

		expectedRepr := "..100"
		assert.Equal(t, expectedRepr, getReprAllVisible(t, intRange, ctx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, intRange, utils.Must(TreeWalkEval(node, state)))
	})
}

func TestFloatRangeRepresentation(t *testing.T) {
	t.Run("known start", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		floatRange := FloatRange{start: 0, end: 100, inclusiveEnd: true}

		expectedRepr := "0.0..100.0"
		assert.Equal(t, expectedRepr, getReprAllVisible(t, floatRange, ctx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, floatRange, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("unknown start", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		floatRange := FloatRange{start: 0, end: 100, unknownStart: true, inclusiveEnd: true}

		expectedRepr := "..100.0"
		assert.Equal(t, expectedRepr, getReprAllVisible(t, floatRange, ctx))

		node := assertParseExpression(t, expectedRepr)
		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, floatRange, utils.Must(TreeWalkEval(node, state)))
	})
}

func TestUdataRepresentation(t *testing.T) {
	t.Run("only root", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		udata := &UData{Root: Int(1)}

		assert.Equal(t, `udata 1{}`, getReprAllVisible(t, udata, ctx))
		node := assertParseExpression(t, `udata 1{}`)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("single hiearchy entry with no children", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		udata := &UData{Root: Int(1), HiearchyEntries: []UDataHiearchyEntry{{Value: Int(2)}}}

		expectedRepr := `udata 1{2}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, udata, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("two hiearchy entries with no children", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		udata := &UData{
			Root: Int(1),
			HiearchyEntries: []UDataHiearchyEntry{
				{Value: Int(2)},
				{Value: Int(3)},
			},
		}

		expectedRepr := `udata 1{2,3}`
		repr := getReprAllVisible(t, udata, ctx)
		assert.Equal(t, expectedRepr, repr)
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

	t.Run("deep", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

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

		expectedRepr := `udata 1{2,3{4}}`
		assert.Equal(t, expectedRepr, getReprAllVisible(t, udata, ctx))
		node := assertParseExpression(t, expectedRepr)

		state := NewTreeWalkState(NewContext(ContextConfig{}))
		assert.Equal(t, udata, utils.Must(TreeWalkEval(node, state)))
	})

}

func TestNamedSegmentPathPatternRepresentation(t *testing.T) {

	//TODO: finish test
}

func TestIntRangePatternRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	intRangePattern := NewIncludedEndIntRangePattern(1, 2, -1)

	expectedRepr := `%int(1..2)`
	assert.Equal(t, expectedRepr, getReprAllVisible(t, intRangePattern, ctx))
	node := assertParseExpression(t, expectedRepr)

	state := NewTreeWalkState(NewContext(ContextConfig{}))

	state.Global.Ctx.AddNamedPattern("int", INT_PATTERN)
	assert.Equal(t, intRangePattern, utils.Must(TreeWalkEval(node, state)))
}

func TestFileModeRepresentation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	fileMode := FileMode(os.ModeDir | 0o777)

	expectedRepr := fmt.Sprintf("FileMode(%d)", fileMode)
	assert.Equal(t, expectedRepr, getReprAllVisible(t, fileMode, ctx))
}

func assertParseExpression(t *testing.T, s string) parse.Node {
	n, ok := parse.ParseExpression(s)
	assert.True(t, ok, "failed to parsed '"+s+"'")
	return n
}

func getReprAllVisible(t *testing.T, v Serializable, ctx *Context, reprConfig ...*ReprConfig) string {
	buff := bytes.NewBuffer(nil)
	if reprConfig == nil {
		reprConfig = append(reprConfig, &ReprConfig{AllVisible: true})
	}

	err := v.WriteRepresentation(ctx, buff, reprConfig[0], 0)
	if err != nil {
		assert.FailNow(t, "failed to get representation: "+err.Error())
	}
	return buff.String()
}
