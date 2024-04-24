package codecompletion

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestStringifyExpectedValue(t *testing.T) {

	emptyChunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{CodeString: ""}))

	t.Run("string", func(t *testing.T) {
		t.Run("concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewString("a"),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `"a"`, s)
		})

		t.Run("non-concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.ANY_STRING,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})
	})

	t.Run("integer", func(t *testing.T) {
		t.Run("positive", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInt(1),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `1`, s)

		})

		t.Run("negative", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInt(-1),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `-1`, s)
		})

		t.Run("unknown", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.ANY_INT,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})
	})

	t.Run("float", func(t *testing.T) {
		t.Run("whole: positive", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewFloat(1),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `1.0`, s)

		})

		t.Run("whole: negative", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewFloat(-1),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `-1.0`, s)
		})

		t.Run("not whole: negative", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewFloat(1.5),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `1.5`, s)
		})

		t.Run("unknown", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.ANY_FLOAT,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})
	})

	t.Run("boolean", func(t *testing.T) {

		t.Run("true", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.TRUE,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `true`, s)
		})

		t.Run("false", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.FALSE,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `false`, s)
		})

		t.Run("unknown", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.ANY_BOOL,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})
	})

	t.Run("path", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {

			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewPath("/"),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `/`, s)
		})

		t.Run("path needing to be quoted", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewPath("/]"),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "/`]`", s)
		})

		//path needing to be quoted

		t.Run("unknown", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.ANY_PATH,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})
	})

	t.Run("path pattern", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewPathPattern("/..."),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `%/...`, s)
		})

		t.Run("path pattern needing to be quoted", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewPathPattern("/]/..."),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "%/`]/...`", s)
		})

		t.Run("unknown", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.ANY_PATH_PATTERN,
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})

	})

	t.Run("object", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewEmptyObject(),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `{}`, s)
		})

		t.Run("one property: ident-like name and concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInexactObject2(map[string]symbolic.Serializable{"a": symbolic.INT_1}),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "{\na: 1\n}", s) //TODO: remove linefeeds ?
		})

		t.Run("one property: non ident-like name and concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInexactObject2(map[string]symbolic.Serializable{"c fé": symbolic.INT_1}),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "{\n\"c fé\": 1\n}", s) //TODO: remove linefeeds ?
		})

		t.Run("one property: non concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInexactObject2(map[string]symbolic.Serializable{"a": symbolic.ANY_INT}),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "{\na: \n}", s) //TODO: remove linefeeds ?
		})
	})

	t.Run("record", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewEmptyRecord(),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `#{}`, s)
		})

		t.Run("one property: ident-like name and concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInexactRecord(map[string]symbolic.Serializable{"a": symbolic.INT_1}, nil),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "#{\na: 1\n}", s) //TODO: remove linefeeds ?
		})

		t.Run("one property: non ident-like name and concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInexactRecord(map[string]symbolic.Serializable{"c fé": symbolic.INT_1}, nil),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "#{\n\"c fé\": 1\n}", s) //TODO: remove linefeeds ?
		})

		t.Run("one property: non concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewInexactObject2(map[string]symbolic.Serializable{"a": symbolic.ANY_INT}),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "{\na: \n}", s) //TODO: remove linefeeds ?
		})
	})

	t.Run("dictionary", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewDictionary(nil, nil),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `:{}`, s)
		})

		t.Run("one entry: concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewDictionary(
					map[string]symbolic.Serializable{`"a"`: symbolic.NewInt(1)},
					map[string]symbolic.Serializable{`"a"`: symbolic.NewString(`"a"`)},
				),
				search: completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, ":{\n\"a\": 1\n}", s) //TODO: remove linefeeds ?
		})

		t.Run("one entry: non concretizable value", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewDictionary(
					map[string]symbolic.Serializable{`"a"`: symbolic.ANY_INT},
					map[string]symbolic.Serializable{`"a"`: symbolic.NewString(`"a"`)},
				),
				search: completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, ":{\n\"a\": \n}", s) //TODO: remove linefeeds ?
		})

		t.Run("two entries: concretizable values", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewDictionary(
					map[string]symbolic.Serializable{`"a"`: symbolic.NewInt(1), `"b"`: symbolic.NewInt(2)},
					map[string]symbolic.Serializable{`"a"`: symbolic.NewString(`"a"`), `"b"`: symbolic.NewString(`"b"`)},
				),
				search: completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, ":{\n\"a\": 1\n\"b\": 2\n}", s) //TODO: remove linefeeds ?
		})
	})

	t.Run("list", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewList(),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `[]`, s)
		})

		t.Run("one element: concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewList(symbolic.NewInt(1)),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `[1]`, s)
		})

		t.Run("one element: non concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewList(symbolic.ANY_INT),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})

		t.Run("two elements: concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewList(symbolic.NewInt(1), symbolic.NewInt(2)),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "[\n1\n2\n]", s)
		})
	})
	t.Run("tuple", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewTuple(),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `#[]`, s)
		})

		t.Run("one element: concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewTuple(symbolic.NewInt(1)),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, `#[1]`, s)
		})

		t.Run("one element: non concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewTuple(symbolic.ANY_INT),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.False(t, ok)
			assert.False(t, isGuess)
			assert.Empty(t, s)
		})

		t.Run("two elements: concretizable", func(t *testing.T) {
			s, isGuess, ok := getExpectedValueCompletion(expectedValueCompletionComputationConfig{
				expectedOrGuessedValue: symbolic.NewTuple(symbolic.NewInt(1), symbolic.NewInt(2)),
				search:                 completionSearch{chunk: emptyChunk},
			})

			assert.True(t, ok)
			assert.False(t, isGuess)
			assert.Equal(t, "#[\n1\n2\n]", s)
		})
	})
}
