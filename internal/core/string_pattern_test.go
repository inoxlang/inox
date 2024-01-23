package core

import (
	"math"
	"strconv"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestEvalStringPatternNode(t *testing.T) {

	t.Run("single element : string literal", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ExactlyOneOcurrence,
					Expr:      &parse.QuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.IsType(t, (*SequenceStringPattern)(nil), patt)
		assert.Equal(t, "(s)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("s")))
		assert.False(t, patt.Test(nil, Str("ss")))
		assert.False(t, patt.Test(nil, Str("sa")))
		assert.False(t, patt.Test(nil, Str("as")))
	})

	t.Run("single element : rune range expression", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ExactlyOneOcurrence,
					Expr: &parse.RuneRangeExpression{
						Lower: &parse.RuneLiteral{Value: 'a'},
						Upper: &parse.RuneLiteral{Value: 'z'},
					},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "([a-z])", patt.Regex())
		assert.True(t, patt.Test(nil, Str("a")))
		assert.False(t, patt.Test(nil, Str("aa")))
	})

	t.Run("single element : single-char string literal (ocurrence modifier '*')", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ZeroOrMoreOcurrence,
					Expr:      &parse.QuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(s*)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("s")))
		assert.True(t, patt.Test(nil, Str("ss")))
		assert.False(t, patt.Test(nil, Str("ssa")))
		assert.False(t, patt.Test(nil, Str("assa")))
	})

	t.Run("single element : two-char string literal (ocurrence modifier '*')", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ZeroOrMoreOcurrence,
					Expr:      &parse.QuotedStringLiteral{Value: "ab"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "((?:ab)*)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("ab")))
		assert.True(t, patt.Test(nil, Str("abab")))
		assert.False(t, patt.Test(nil, Str("aba")))
	})

	t.Run("single element : repetition of a named pattern that is not defined yet", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		lazy := true
		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ExactlyOneOcurrence,
					Expr: &parse.PatternIdentifierLiteral{
						Name: "p",
					},
				},
			},
		}, state, lazy)

		ctx.AddNamedPattern("p", NewRegexPattern("[a-z]"))

		assert.NoError(t, err)
		assert.True(t, patt.HasRegex())
		assert.True(t, patt.Test(nil, Str("a")))
		assert.False(t, patt.Test(nil, Str("aa")))
	})

	t.Run("single element : single-char string literal (ocurrence modifier '=' 2)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence:           parse.ExactOcurrence,
					ExactOcurrenceCount: 2,
					Expr:                &parse.QuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(s{2})", patt.Regex())
		assert.True(t, patt.Test(nil, Str("ss")))
		assert.False(t, patt.Test(nil, Str("ssa")))
		assert.False(t, patt.Test(nil, Str("ass")))
	})

	t.Run("single element : two-char string literal (ocurrence modifier '=' 2)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence:           parse.ExactOcurrence,
					ExactOcurrenceCount: 2,
					Expr:                &parse.QuotedStringLiteral{Value: "ab"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "((?:ab){2})", patt.Regex())
		assert.True(t, patt.Test(nil, Str("abab")))
		assert.False(t, patt.Test(nil, Str("ab")))
		assert.False(t, patt.Test(nil, Str("ababab")))
	})

	t.Run("two elements : one string literal + a pattern identifier (exact string pattern)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("b", NewExactStringPattern(Str("c")))
		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ExactlyOneOcurrence,
					Expr:      &parse.QuotedStringLiteral{Value: "a"},
				},
				{
					Ocurrence: parse.ExactlyOneOcurrence,
					Expr:      &parse.PatternIdentifierLiteral{Name: "b"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(a)(c)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("ac")))
		assert.False(t, patt.Test(nil, Str("acb")))
		assert.False(t, patt.Test(nil, Str("bacb")))
	})

	t.Run("union of two single-element cases", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.PatternUnion{
			Cases: []parse.Node{
				&parse.QuotedStringLiteral{Value: "a"},
				&parse.QuotedStringLiteral{Value: "b"},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(a|b)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("a")))
		assert.True(t, patt.Test(nil, Str("b")))
		assert.False(t, patt.Test(nil, Str("ab")))
		assert.False(t, patt.Test(nil, Str("ba")))
	})

	t.Run("union of two named patterns that are not defined yet", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.PatternUnion{
			Cases: []parse.Node{
				&parse.PatternIdentifierLiteral{
					Name: "a",
				},
				&parse.PatternIdentifierLiteral{
					Name: "b",
				},
			},
		}, state, true)

		if !assert.NoError(t, err) {
			return
		}

		//define named patterns
		ctx.AddNamedPattern("a", NewRegexPattern("a"))
		ctx.AddNamedPattern("b", NewRegexPattern("b"))

		if !assert.True(t, patt.HasRegex()) {
			return
		}
		assert.Equal(t, "(a|b)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("a")))
		assert.True(t, patt.Test(nil, Str("b")))
		assert.False(t, patt.Test(nil, Str("ab")))
		assert.False(t, patt.Test(nil, Str("ba")))
	})

	t.Run("union of two multiple-element cases", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &parse.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&parse.PatternUnion{
			Cases: []parse.Node{
				&parse.ComplexStringPatternPiece{
					Elements: []*parse.PatternPieceElement{
						{
							Ocurrence: parse.ExactlyOneOcurrence,
							Expr:      &parse.QuotedStringLiteral{Value: "a"},
						},
						{
							Ocurrence: parse.ExactlyOneOcurrence,
							Expr:      &parse.QuotedStringLiteral{Value: "b"},
						},
					},
				},

				&parse.ComplexStringPatternPiece{
					Elements: []*parse.PatternPieceElement{
						{
							Ocurrence: parse.ExactlyOneOcurrence,
							Expr:      &parse.QuotedStringLiteral{Value: "c"},
						},
						{
							Ocurrence: parse.ExactlyOneOcurrence,
							Expr:      &parse.QuotedStringLiteral{Value: "d"},
						},
					},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "((a)(b)|(c)(d))", patt.Regex())
		assert.True(t, patt.Test(nil, Str("ab")))
		assert.True(t, patt.Test(nil, Str("cd")))
		assert.False(t, patt.Test(nil, Str("abcd")))
	})
}

func TestComplexPatternParsing(t *testing.T) {

	t.Run("sequence with a single non repeated element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("subpatt", NewExactStringPattern(Str("a")))

		patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{&DynamicStringPatternElement{"subpatt", ctx}}, []string{""})

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, patt.Test(nil, Str("a")))
	})

	t.Run("sequence with a single repeated element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("subpatt", NewExactStringPattern(Str("a")))

		patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
			newRepeatedPatternElement(parse.ZeroOrMoreOcurrence, -1, &DynamicStringPatternElement{"subpatt", ctx}),
		}, []string{""})

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, patt.Test(nil, Str("a")))
		assert.True(t, patt.Test(nil, Str("aa")))
		assert.False(t, patt.Test(nil, Str("ba")))
		assert.False(t, patt.Test(nil, Str("ab")))
	})

	t.Run("sequence with two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("subpatt", NewExactStringPattern(Str("a")))

		patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
			newRepeatedPatternElement(parse.ZeroOrMoreOcurrence, -1, &DynamicStringPatternElement{"subpatt", ctx}),
			NewExactStringPattern(Str("b")),
		}, []string{"", ""})

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, patt.Test(nil, Str("ab")))
		assert.True(t, patt.Test(nil, Str("aab")))
		assert.False(t, patt.Test(nil, Str("ba")))
		assert.True(t, patt.Test(nil, Str("ab")))
	})

	t.Run("recursion", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		valuePattern := utils.Must(NewUnionStringPattern(nil, []StringPattern{
			&DynamicStringPatternElement{"bool", ctx},
			&DynamicStringPatternElement{"list", ctx},
		}))

		ctx.AddNamedPattern("value", valuePattern)

		boolPattern := utils.Must(NewUnionStringPattern(nil, []StringPattern{
			NewExactStringPattern(Str("true")),
			NewExactStringPattern(Str("false")),
		}))

		ctx.AddNamedPattern("bool", boolPattern)

		//list pattern

		sequenceElements := []StringPattern{
			NewExactStringPattern(Str("[")),
			newRepeatedPatternElement(
				parse.ZeroOrMoreOcurrence,
				-1,
				utils.Must(NewSequenceStringPattern(
					nil,
					nil,
					[]StringPattern{
						&DynamicStringPatternElement{"value", ctx},
						NewExactStringPattern(Str(",")),
					}, []string{"", ""})),
			),
			NewExactStringPattern(Str("]")),
		}
		listPattern := utils.Must(NewSequenceStringPattern(nil, nil, sequenceElements, []string{"", "", ""}))

		ctx.AddNamedPattern("list", listPattern)

		assert.True(t, valuePattern.Test(nil, Str("true")))
		assert.True(t, valuePattern.Test(nil, Str("[]")))
		assert.True(t, valuePattern.Test(nil, Str("[true,]")))
		assert.True(t, valuePattern.Test(nil, Str("[[],]")))
		assert.True(t, valuePattern.Test(nil, Str("[[true,],]")))
		assert.True(t, valuePattern.Test(nil, Str("[[true,[],],]")))
		assert.True(t, valuePattern.Test(nil, Str("[[true,[true,],],]")))

		assert.False(t, valuePattern.Test(nil, Str("[][]")))
		assert.False(t, valuePattern.Test(nil, Str("[")))
		assert.False(t, valuePattern.Test(nil, Str("[true")))
		assert.False(t, valuePattern.Test(nil, Str("[true,")))
	})

	t.Run("complex recursion", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		valuePattern := utils.Must(NewUnionStringPattern(nil, []StringPattern{
			&DynamicStringPatternElement{"string", ctx},
			&DynamicStringPatternElement{"bool", ctx},
			&DynamicStringPatternElement{"list", ctx},
			&DynamicStringPatternElement{"object", ctx},
		}))

		ctx.AddNamedPattern("value", valuePattern)

		boolPattern := utils.Must(NewUnionStringPattern(nil, []StringPattern{
			NewExactStringPattern(Str("true")),
			NewExactStringPattern(Str("false")),
		}))

		ctx.AddNamedPattern("bool", boolPattern)

		ctx.AddNamedPattern("string", NewExactStringPattern(Str(`"string"`)))

		//list pattern

		sequenceElements := []StringPattern{
			NewExactStringPattern(Str("[")),
			newRepeatedPatternElement(
				parse.ZeroOrMoreOcurrence,
				-1,
				utils.Must(NewSequenceStringPattern(
					nil,
					nil,
					[]StringPattern{
						&DynamicStringPatternElement{"value", ctx},
						NewExactStringPattern(Str(",")),
					}, []string{"", ""})),
			),
			NewExactStringPattern(Str("]")),
		}
		listPattern := utils.Must(NewSequenceStringPattern(nil, nil, sequenceElements, []string{"", "", ""}))

		ctx.AddNamedPattern("list", listPattern)

		//object pattern

		sequenceElements = []StringPattern{
			NewExactStringPattern(Str("{")),
			newRepeatedPatternElement(
				parse.ZeroOrMoreOcurrence,
				-1,
				utils.Must(NewSequenceStringPattern(
					nil,
					nil,
					[]StringPattern{
						&DynamicStringPatternElement{"string", ctx},
						NewExactStringPattern(Str(":")),
						&DynamicStringPatternElement{"value", ctx},
					}, []string{"", "", ""})),
			),
			NewExactStringPattern(Str("}")),
		}

		objectPattern := utils.Must(NewSequenceStringPattern(nil, nil, sequenceElements, []string{"", "", ""}))

		ctx.AddNamedPattern("object", objectPattern)

		assert.True(t, valuePattern.Test(nil, Str("true")))
		assert.True(t, valuePattern.Test(nil, Str(`"string"`)))
		assert.True(t, valuePattern.Test(nil, Str("[]")))
		assert.True(t, valuePattern.Test(nil, Str("[true,]")))
		assert.True(t, valuePattern.Test(nil, Str("[[],]")))
		assert.True(t, valuePattern.Test(nil, Str("[[true,],]")))
		assert.True(t, valuePattern.Test(nil, Str("[[true,[],],]")))
		assert.True(t, valuePattern.Test(nil, Str(`{}`)))
		assert.True(t, valuePattern.Test(nil, Str("{}")))
		assert.True(t, valuePattern.Test(nil, Str(`{"string":true}`)))
		assert.True(t, valuePattern.Test(nil, Str(`{"string":[]}`)))
		assert.True(t, valuePattern.Test(nil, Str(`{"string":[{},]}`)))

		assert.False(t, valuePattern.Test(nil, Str("[][]")))
		assert.False(t, valuePattern.Test(nil, Str("{}{}")))
		assert.False(t, valuePattern.Test(nil, Str("[")))
		assert.False(t, valuePattern.Test(nil, Str("[true")))
		assert.False(t, valuePattern.Test(nil, Str("[true,")))
		assert.False(t, valuePattern.Test(nil, Str(`{"string"}`)))
		assert.False(t, valuePattern.Test(nil, Str(`{"string":}`)))
		assert.False(t, valuePattern.Test(nil, Str(`{"string":[}`)))
	})
}

func TestLengthCheckingStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {
		pattern := NewLengthCheckingStringPattern(0, 1)

		assert.Equal(t, IntRange{
			inclusiveEnd: true,
			start:        0,
			end:          1,
			step:         1,
		}, pattern.LengthRange())

	})

	t.Run("Test()", func(t *testing.T) {
		maxLen1 := NewLengthCheckingStringPattern(0, 1)

		assert.True(t, maxLen1.Test(nil, Str("")))
		assert.True(t, maxLen1.Test(nil, Str("a")))
		assert.False(t, maxLen1.Test(nil, Str("ab")))
		assert.False(t, maxLen1.Test(nil, Str("abc")))

		maxLen2 := NewLengthCheckingStringPattern(0, 2)

		assert.True(t, maxLen2.Test(nil, Str("")))
		assert.True(t, maxLen2.Test(nil, Str("a")))
		assert.True(t, maxLen2.Test(nil, Str("ab")))
		assert.False(t, maxLen2.Test(nil, Str("abc")))
		assert.False(t, maxLen2.Test(nil, Str("abcd")))

		minLen1MaxLen2 := NewLengthCheckingStringPattern(1, 2)

		assert.True(t, minLen1MaxLen2.Test(nil, Str("a")))
		assert.True(t, minLen1MaxLen2.Test(nil, Str("ab")))
		assert.False(t, minLen1MaxLen2.Test(nil, Str("")))
		assert.False(t, minLen1MaxLen2.Test(nil, Str("abc")))
		assert.False(t, minLen1MaxLen2.Test(nil, Str("abcd")))
	})
}

func TestSequenceStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {

		t.Run("single element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(parse.AtLeastOneOcurrence, -1, NewExactStringPattern(Str("12"))),
			}, nil)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				inclusiveEnd: true,
				start:        2,
				end:          math.MaxInt64,
				step:         1,
			}, patt.LengthRange())
		})

		t.Run("two elements, first one has no maximum length", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(parse.AtLeastOneOcurrence, -1, NewExactStringPattern(Str("12"))),
				NewExactStringPattern(Str("34")),
			}, nil)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				inclusiveEnd: true,
				start:        4,
				end:          math.MaxInt64,
				step:         1,
			}, patt.LengthRange())
		})

		t.Run("two elements, both have no maximum length", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(parse.AtLeastOneOcurrence, -1, NewExactStringPattern(Str("12"))),
				newRepeatedPatternElement(parse.AtLeastOneOcurrence, -1, NewExactStringPattern(Str("12"))),
			}, nil)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				inclusiveEnd: true,
				start:        4,
				end:          math.MaxInt64,
				step:         1,
			}, patt.LengthRange())
		})

	})

	t.Run(".MatchGroups()", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(Str("12")),
			}, []string{"number"})
			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, Str("12"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      Str("12"),
				"number": Str("12"),
			}, result)
		})

		t.Run("single repeated element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(parse.AtLeastOneOcurrence, -1, NewExactStringPattern(Str("12"))),
			}, []string{"number"})

			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, Str("1212"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      Str("1212"),
				"number": Str("1212"),
			}, result)
		})

		t.Run("two named elements", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(Str("12")),
				NewExactStringPattern(Str("AB")),
			}, []string{"digits", "letters"})
			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, Str("12AB"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":       Str("12AB"),
				"digits":  Str("12"),
				"letters": Str("AB"),
			}, result)
		})

		t.Run("two elements, first is named", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(Str("12")),
				NewExactStringPattern(Str("AB")),
			}, []string{"digits", ""})

			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, Str("12AB"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      Str("12AB"),
				"digits": Str("12"),
			}, result)
		})
	})
}

func TestRuneRangeStringPattern(t *testing.T) {
	t.Run(".LengthRange()", func(t *testing.T) {
		patt := &RuneRangeStringPattern{
			runes: RuneRange{
				Start: 'a',
				End:   'b',
			},
		}

		assert.Equal(t, IntRange{
			start:        1,
			end:          1,
			inclusiveEnd: true,
			step:         1,
		}, patt.LengthRange())
	})

}

func TestIntRangeStringPattern(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	max := int64(math.MaxInt64)
	min := int64(math.MinInt64)

	minS := strconv.FormatInt(min, 10)
	minPlusOneS := strconv.FormatInt(min+1, 10)
	maxS := strconv.FormatInt(max, 10)
	maxMinusOneS := strconv.FormatInt(max-1, 10)
	maxCharCount := int64(len(minS))

	assertTestAndParse := func(t *testing.T, stringPattern StringPattern, s string) {
		assert.True(t, stringPattern.Test(ctx, Str(s)))

		v, err := stringPattern.Parse(ctx, s)
		if !assert.NoError(t, err) {
			return
		}

		n, err := strconv.ParseInt(s, 10, 64)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, Int(n), v)
	}

	assertDoesNotTestAndParse := func(t *testing.T, stringPattern StringPattern, s string) {
		assert.False(t, stringPattern.Test(ctx, Str(s)))

		_, err := stringPattern.Parse(ctx, s)
		assert.Error(t, err)
	}

	pattern := NewIntRangeStringPattern(min, 0, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(min+1, 0, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, minS)
	assertDoesNotTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(min, max, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, maxS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "1")
	assertTestAndParse(t, pattern, "2")
	assertDoesNotTestAndParse(t, pattern, "-0")

	pattern = NewIntRangeStringPattern(min, max-1, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, maxMinusOneS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "1")
	assertTestAndParse(t, pattern, "2")
	assertDoesNotTestAndParse(t, pattern, maxS)
	assertDoesNotTestAndParse(t, pattern, "-0")

	pattern = NewIntRangeStringPattern(0, 0, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 1), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertDoesNotTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(0, 1, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 1), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(1, 2, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 1), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(1, 9, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 1), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertDoesNotTestAndParse(t, pattern, "10")

	pattern = NewIntRangeStringPattern(1, 10, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 2), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "10")
	assertDoesNotTestAndParse(t, pattern, "11")

	pattern = NewIntRangeStringPattern(1, 99, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 2), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "11")
	assertTestAndParse(t, pattern, "99")
	assertDoesNotTestAndParse(t, pattern, "100")

	pattern = NewIntRangeStringPattern(1, 100, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 3), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "11")
	assertTestAndParse(t, pattern, "99")
	assertTestAndParse(t, pattern, "100")

	pattern = NewIntRangeStringPattern(1, max, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, int64(utils.CountDigits(max))), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "11")
	assertTestAndParse(t, pattern, "99")
	assertTestAndParse(t, pattern, "100")
	assertTestAndParse(t, pattern, strconv.FormatInt(max, 10))

	// pattern = NewIntRangeStringPattern(min, -1, nil)
	// assert.Equal(t, NewIncludedEndIntRange(2, int64(1+utils.CountDigits(min))), pattern.LengthRange())
	// assertDoesNotTestAndParse(t, pattern, "0")
	// assertDoesNotTestAndParse(t, pattern, "1")
	// assertDoesNotTestAndParse(t, pattern, "-0")
	// assertTestAndParse(t, pattern, "-1")
	// assertTestAndParse(t, pattern, "-100")
	// assertDoesNotTestAndParse(t, pattern, strconv.FormatInt(min, 10))
	// assertTestAndParse(t, pattern, strconv.FormatInt(min+1, 10))

	pattern = NewIntRangeStringPattern(-1, 1, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 2), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(-1, 9, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 2), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertDoesNotTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertDoesNotTestAndParse(t, pattern, "-9")
	assertDoesNotTestAndParse(t, pattern, "10")
	assertDoesNotTestAndParse(t, pattern, "-10")

	pattern = NewIntRangeStringPattern(-9, 9, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 2), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "-9")
	assertDoesNotTestAndParse(t, pattern, "10")
	assertDoesNotTestAndParse(t, pattern, "-10")

	pattern = NewIntRangeStringPattern(-10, 9, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 3), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "-9")
	assertDoesNotTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "-10")

	pattern = NewIntRangeStringPattern(-10, 10, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 3), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "-9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "-10")

	pattern = NewIntRangeStringPattern(-10, 99, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 3), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "-9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "-10")
	assertTestAndParse(t, pattern, "99")
	assertDoesNotTestAndParse(t, pattern, "-11")
	assertDoesNotTestAndParse(t, pattern, "100")

	pattern = NewIntRangeStringPattern(-10, 100, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 3), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "-9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "-10")
	assertTestAndParse(t, pattern, "99")
	assertDoesNotTestAndParse(t, pattern, "-11")
	assertTestAndParse(t, pattern, "100")

	pattern = NewIntRangeStringPattern(-100, 100, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 4), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "-2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "-9")
	assertTestAndParse(t, pattern, "10")
	assertTestAndParse(t, pattern, "-10")
	assertTestAndParse(t, pattern, "99")
	assertTestAndParse(t, pattern, "-11")
	assertTestAndParse(t, pattern, "100")
	assertTestAndParse(t, pattern, "-100")
}

func TestFloatRangeStringPattern(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	max := float64(math.MaxFloat64)
	min := -float64(math.MaxFloat64)

	minS := strconv.FormatFloat(min, 'g', -1, 64)
	maxS := strconv.FormatFloat(max, 'g', -1, 64)

	assertTestAndParse := func(t *testing.T, stringPattern StringPattern, s string) {
		assert.True(t, stringPattern.Test(ctx, Str(s)))

		v, err := stringPattern.Parse(ctx, s)
		if !assert.NoError(t, err) {
			return
		}

		n, err := strconv.ParseFloat(s, 64)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, Float(n), v)
	}

	assertDoesNotTestAndParse := func(t *testing.T, stringPattern StringPattern, s string) {
		assert.False(t, stringPattern.Test(ctx, Str(s)))

		_, err := stringPattern.Parse(ctx, s)
		assert.Error(t, err)
	}

	pattern := NewFloatRangeStringPattern(min, 0, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 1+int64(MAX_CHAR_COUNT_MAXIMUM_FLOAT_64)), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "00")
	assertTestAndParse(t, pattern, "0.0")
	assertTestAndParse(t, pattern, "0.")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "-01")
	assertTestAndParse(t, pattern, "-1.0")
	assertTestAndParse(t, pattern, "-1.")
	assertTestAndParse(t, pattern, "-0.0")
	assertTestAndParse(t, pattern, "-0.")
	assertDoesNotTestAndParse(t, pattern, "1.0")
	assertDoesNotTestAndParse(t, pattern, "1.")
	assertDoesNotTestAndParse(t, pattern, "2.0")
	assertDoesNotTestAndParse(t, pattern, "2.")

	pattern = NewFloatRangeStringPattern(min, max, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, 1+int64(MAX_CHAR_COUNT_MAXIMUM_FLOAT_64)), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, maxS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "0.0")
	assertTestAndParse(t, pattern, "00")
	assertTestAndParse(t, pattern, "00.0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "-01")
	assertTestAndParse(t, pattern, "-1.0")
	assertTestAndParse(t, pattern, "-0.0")
	assertTestAndParse(t, pattern, "1.0")
	assertTestAndParse(t, pattern, "2.0")

	pattern = NewFloatRangeStringPattern(0, max, nil)
	assert.Equal(t, NewIncludedEndIntRange(1, int64(MAX_CHAR_COUNT_MAXIMUM_FLOAT_64)), pattern.LengthRange())
	assertTestAndParse(t, pattern, maxS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "0.0")
	assertTestAndParse(t, pattern, "00")
	assertTestAndParse(t, pattern, "00.0")
	assertTestAndParse(t, pattern, "0.")
	assertTestAndParse(t, pattern, "1.0")
	assertTestAndParse(t, pattern, "2.0")
	assertDoesNotTestAndParse(t, pattern, minS)
	assertDoesNotTestAndParse(t, pattern, "-1.0")
	assertDoesNotTestAndParse(t, pattern, "-1.")
}

func TestUnionStringPattern(t *testing.T) {
	t.Run(".LengthRange()", func(t *testing.T) {
		patt := utils.Must(NewUnionStringPattern(nil, []StringPattern{
			NewExactStringPattern(Str("a")),
			NewExactStringPattern(Str("bc")),
		}))
		assert.Equal(t, IntRange{
			start:        1,
			end:          2,
			inclusiveEnd: true,
			step:         1,
		}, patt.LengthRange())
	})

	t.Run(".MatchGroups()", func(t *testing.T) {
		patt, _ := NewUnionStringPattern(nil, []StringPattern{
			utils.Must(NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(Str("12")),
			}, []string{"number"})),

			utils.Must(NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(Str("ab")),
			}, []string{"string"})),
		})

		t.Run("matching string", func(t *testing.T) {
			result, ok, err := patt.MatchGroups(nil, Str("12"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      Str("12"),
				"number": Str("12"),
			}, result)
		})

		t.Run("matching string with additional characters", func(t *testing.T) {
			result, ok, err := patt.MatchGroups(nil, Str("123"))
			assert.NoError(t, err)
			assert.False(t, ok)
			assert.Nil(t, result)
		})

	})
}

func TestRegexPattern(t *testing.T) {
	t.Run(".LengthRange()", func(t *testing.T) {
		testCases := map[string]IntRange{
			``: {
				start: 0,
				end:   0,
			},
			`a`: {
				start: 1,
				end:   1,
			},
			`a?`: {
				start: 0,
				end:   1,
			},
			`a+`: {
				start: 1,
				end:   math.MaxInt64,
			},
			`a*`: {
				start: 0,
				end:   math.MaxInt64,
			},
			`a{0,1}`: {
				start: 0,
				end:   1,
			},
			`a{0,2}`: {
				start: 0,
				end:   2,
			},
			`a{1,2}`: {
				start: 1,
				end:   2,
			},
			`a{1,3}`: {
				start: 1,
				end:   3,
			},
			`.`: {
				start: 1,
				end:   1,
			},
			`[a-z]`: {
				start: 1,
				end:   1,
			},
			`(a|bc)`: {
				start: 1,
				end:   2,
			},
		}

		for regex, expectedRange := range testCases {
			t.Run("`"+regex+"`", func(t *testing.T) {
				expectedRange.step = 1
				expectedRange.inclusiveEnd = true

				patt := NewRegexPattern(regex)
				assert.Equal(t, expectedRange, patt.LengthRange())
			})
		}
	})

}
