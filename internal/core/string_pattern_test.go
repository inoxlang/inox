package core

import (
	"math"
	"strconv"
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/parse"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestEvalStringPatternNode(t *testing.T) {

	t.Run("single element : double-quoted string literal", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr:       &ast.DoubleQuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.IsType(t, (*SequenceStringPattern)(nil), patt)
		assert.Equal(t, "s", patt.Regex())
		assert.True(t, patt.Test(nil, String("s")))
		assert.False(t, patt.Test(nil, String("ss")))
		assert.False(t, patt.Test(nil, String("sa")))
		assert.False(t, patt.Test(nil, String("as")))
	})

	t.Run("single element : multiline string literal", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr:       &ast.MultilineStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.IsType(t, (*SequenceStringPattern)(nil), patt)
		assert.Equal(t, "s", patt.Regex())
		assert.True(t, patt.Test(nil, String("s")))
		assert.False(t, patt.Test(nil, String("ss")))
		assert.False(t, patt.Test(nil, String("sa")))
		assert.False(t, patt.Test(nil, String("as")))
	})

	t.Run("single element : string literal with group name", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr:       &ast.DoubleQuotedStringLiteral{Value: "s"},
					GroupName: &ast.PatternGroupName{
						Name: "g",
					},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.IsType(t, (*SequenceStringPattern)(nil), patt)
		assert.Equal(t, "(s)", patt.Regex())
		assert.True(t, patt.Test(nil, String("s")))
		assert.False(t, patt.Test(nil, String("ss")))
		assert.False(t, patt.Test(nil, String("sa")))
		assert.False(t, patt.Test(nil, String("as")))
	})

	t.Run("single element : rune range expression", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr: &ast.RuneRangeExpression{
						Lower: &ast.RuneLiteral{Value: 'a'},
						Upper: &ast.RuneLiteral{Value: 'z'},
					},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "[a-z]", patt.Regex())
		assert.True(t, patt.Test(nil, String("a")))
		assert.False(t, patt.Test(nil, String("aa")))
	})

	t.Run("single element : single-char string literal (ocurrence modifier '*')", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ZeroOrMoreOccurrences,
					Expr:       &ast.DoubleQuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "s*", patt.Regex())
		assert.True(t, patt.Test(nil, String("s")))
		assert.True(t, patt.Test(nil, String("ss")))
		assert.False(t, patt.Test(nil, String("ssa")))
		assert.False(t, patt.Test(nil, String("assa")))
	})

	t.Run("single element : two-char string literal (ocurrence modifier '*')", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ZeroOrMoreOccurrences,
					Expr:       &ast.DoubleQuotedStringLiteral{Value: "ab"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(?:ab)*", patt.Regex())
		assert.True(t, patt.Test(nil, String("ab")))
		assert.True(t, patt.Test(nil, String("abab")))
		assert.False(t, patt.Test(nil, String("aba")))
	})

	t.Run("single element : repetition of a named pattern that is not defined yet", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		lazy := true
		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr: &ast.PatternIdentifierLiteral{
						Name: "p",
					},
				},
			},
		}, state, lazy)

		ctx.AddNamedPattern("p", NewRegexPattern("[a-z]"))

		assert.NoError(t, err)
		assert.True(t, patt.HasRegex())
		assert.True(t, patt.Test(nil, String("a")))
		assert.False(t, patt.Test(nil, String("aa")))
	})

	t.Run("single element : single-char string literal (ocurrence modifier '=' 2)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier:          ast.ExactOccurrenceCount,
					ExactOcurrenceCount: 2,
					Expr:                &ast.DoubleQuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "ss", patt.Regex())
		assert.True(t, patt.Test(nil, String("ss")))
		assert.False(t, patt.Test(nil, String("ssa")))
		assert.False(t, patt.Test(nil, String("ass")))
	})

	t.Run("single element : two-char string literal (ocurrence modifier '=' 2)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier:          ast.ExactOccurrenceCount,
					ExactOcurrenceCount: 2,
					Expr:                &ast.DoubleQuotedStringLiteral{Value: "ab"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "abab", patt.Regex())
		assert.True(t, patt.Test(nil, String("abab")))
		assert.False(t, patt.Test(nil, String("ab")))
		assert.False(t, patt.Test(nil, String("ababab")))
	})

	t.Run("two elements : one string literal + a pattern identifier (exact string pattern)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("b", NewExactStringPattern(String("c")))
		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.ComplexStringPatternPiece{
			Elements: []*ast.PatternPieceElement{
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr:       &ast.DoubleQuotedStringLiteral{Value: "a"},
				},
				{
					Quantifier: ast.ExactlyOneOccurrence,
					Expr:       &ast.PatternIdentifierLiteral{Name: "b"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "ac", patt.Regex())
		assert.True(t, patt.Test(nil, String("ac")))
		assert.False(t, patt.Test(nil, String("acb")))
		assert.False(t, patt.Test(nil, String("bacb")))
	})

	t.Run("union of two single-element cases", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.PatternUnion{
			Cases: []ast.Node{
				&ast.DoubleQuotedStringLiteral{Value: "a"},
				&ast.DoubleQuotedStringLiteral{Value: "b"},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(a|b)", patt.Regex())
		assert.True(t, patt.Test(nil, String("a")))
		assert.True(t, patt.Test(nil, String("b")))
		assert.False(t, patt.Test(nil, String("ab")))
		assert.False(t, patt.Test(nil, String("ba")))
	})

	t.Run("union of two named patterns that are not defined yet", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.PatternUnion{
			Cases: []ast.Node{
				&ast.PatternIdentifierLiteral{
					Name: "a",
				},
				&ast.PatternIdentifierLiteral{
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
		assert.True(t, patt.Test(nil, String("a")))
		assert.True(t, patt.Test(nil, String("b")))
		assert.False(t, patt.Test(nil, String("ab")))
		assert.False(t, patt.Test(nil, String("ba")))
	})

	t.Run("union of two multiple-element cases", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		state := NewTreeWalkState(ctx)
		chunk := &parse.ChunkStackItem{Chunk: &parse.ParsedChunkSource{Node: &ast.Chunk{}}}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)

		patt, err := evalStringPatternNode(&ast.PatternUnion{
			Cases: []ast.Node{
				&ast.ComplexStringPatternPiece{
					Elements: []*ast.PatternPieceElement{
						{
							Quantifier: ast.ExactlyOneOccurrence,
							Expr:       &ast.DoubleQuotedStringLiteral{Value: "a"},
						},
						{
							Quantifier: ast.ExactlyOneOccurrence,
							Expr:       &ast.DoubleQuotedStringLiteral{Value: "b"},
						},
					},
				},

				&ast.ComplexStringPatternPiece{
					Elements: []*ast.PatternPieceElement{
						{
							Quantifier: ast.ExactlyOneOccurrence,
							Expr:       &ast.DoubleQuotedStringLiteral{Value: "c"},
						},
						{
							Quantifier: ast.ExactlyOneOccurrence,
							Expr:       &ast.DoubleQuotedStringLiteral{Value: "d"},
						},
					},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "(ab|cd)", patt.Regex())
		assert.True(t, patt.Test(nil, String("ab")))
		assert.True(t, patt.Test(nil, String("cd")))
		assert.False(t, patt.Test(nil, String("abcd")))
	})
}

func TestComplexPatternParsing(t *testing.T) {

	t.Run("sequence with a single non repeated element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("subpatt", NewExactStringPattern(String("a")))

		patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{&DynamicStringPatternElement{"subpatt", ctx}}, []string{""})

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, patt.Test(nil, String("a")))
	})

	t.Run("sequence with a single repeated element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("subpatt", NewExactStringPattern(String("a")))

		patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
			newRepeatedPatternElement(ast.ZeroOrMoreOccurrences, -1, &DynamicStringPatternElement{"subpatt", ctx}),
		}, []string{""})

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, patt.Test(nil, String("a")))
		assert.True(t, patt.Test(nil, String("aa")))
		assert.False(t, patt.Test(nil, String("ba")))
		assert.False(t, patt.Test(nil, String("ab")))
	})

	t.Run("sequence with two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("subpatt", NewExactStringPattern(String("a")))

		patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
			newRepeatedPatternElement(ast.ZeroOrMoreOccurrences, -1, &DynamicStringPatternElement{"subpatt", ctx}),
			NewExactStringPattern(String("b")),
		}, []string{"", ""})

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, patt.Test(nil, String("ab")))
		assert.True(t, patt.Test(nil, String("aab")))
		assert.False(t, patt.Test(nil, String("ba")))
		assert.True(t, patt.Test(nil, String("ab")))
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
			NewExactStringPattern(String("true")),
			NewExactStringPattern(String("false")),
		}))

		ctx.AddNamedPattern("bool", boolPattern)

		//list pattern

		sequenceElements := []StringPattern{
			NewExactStringPattern(String("[")),
			newRepeatedPatternElement(
				ast.ZeroOrMoreOccurrences,
				-1,
				utils.Must(NewSequenceStringPattern(
					nil,
					nil,
					[]StringPattern{
						&DynamicStringPatternElement{"value", ctx},
						NewExactStringPattern(String(",")),
					}, []string{"", ""})),
			),
			NewExactStringPattern(String("]")),
		}
		listPattern := utils.Must(NewSequenceStringPattern(nil, nil, sequenceElements, []string{"", "", ""}))

		ctx.AddNamedPattern("list", listPattern)

		assert.True(t, valuePattern.Test(nil, String("true")))
		assert.True(t, valuePattern.Test(nil, String("[]")))
		assert.True(t, valuePattern.Test(nil, String("[true,]")))
		assert.True(t, valuePattern.Test(nil, String("[[],]")))
		assert.True(t, valuePattern.Test(nil, String("[[true,],]")))
		assert.True(t, valuePattern.Test(nil, String("[[true,[],],]")))
		assert.True(t, valuePattern.Test(nil, String("[[true,[true,],],]")))

		assert.False(t, valuePattern.Test(nil, String("[][]")))
		assert.False(t, valuePattern.Test(nil, String("[")))
		assert.False(t, valuePattern.Test(nil, String("[true")))
		assert.False(t, valuePattern.Test(nil, String("[true,")))
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
			NewExactStringPattern(String("true")),
			NewExactStringPattern(String("false")),
		}))

		ctx.AddNamedPattern("bool", boolPattern)

		ctx.AddNamedPattern("string", NewExactStringPattern(String(`"string"`)))

		//list pattern

		sequenceElements := []StringPattern{
			NewExactStringPattern(String("[")),
			newRepeatedPatternElement(
				ast.ZeroOrMoreOccurrences,
				-1,
				utils.Must(NewSequenceStringPattern(
					nil,
					nil,
					[]StringPattern{
						&DynamicStringPatternElement{"value", ctx},
						NewExactStringPattern(String(",")),
					}, []string{"", ""})),
			),
			NewExactStringPattern(String("]")),
		}
		listPattern := utils.Must(NewSequenceStringPattern(nil, nil, sequenceElements, []string{"", "", ""}))

		ctx.AddNamedPattern("list", listPattern)

		//object pattern

		sequenceElements = []StringPattern{
			NewExactStringPattern(String("{")),
			newRepeatedPatternElement(
				ast.ZeroOrMoreOccurrences,
				-1,
				utils.Must(NewSequenceStringPattern(
					nil,
					nil,
					[]StringPattern{
						&DynamicStringPatternElement{"string", ctx},
						NewExactStringPattern(String(":")),
						&DynamicStringPatternElement{"value", ctx},
					}, []string{"", "", ""})),
			),
			NewExactStringPattern(String("}")),
		}

		objectPattern := utils.Must(NewSequenceStringPattern(nil, nil, sequenceElements, []string{"", "", ""}))

		ctx.AddNamedPattern("object", objectPattern)

		assert.True(t, valuePattern.Test(nil, String("true")))
		assert.True(t, valuePattern.Test(nil, String(`"string"`)))
		assert.True(t, valuePattern.Test(nil, String("[]")))
		assert.True(t, valuePattern.Test(nil, String("[true,]")))
		assert.True(t, valuePattern.Test(nil, String("[[],]")))
		assert.True(t, valuePattern.Test(nil, String("[[true,],]")))
		assert.True(t, valuePattern.Test(nil, String("[[true,[],],]")))
		assert.True(t, valuePattern.Test(nil, String(`{}`)))
		assert.True(t, valuePattern.Test(nil, String("{}")))
		assert.True(t, valuePattern.Test(nil, String(`{"string":true}`)))
		assert.True(t, valuePattern.Test(nil, String(`{"string":[]}`)))
		assert.True(t, valuePattern.Test(nil, String(`{"string":[{},]}`)))

		assert.False(t, valuePattern.Test(nil, String("[][]")))
		assert.False(t, valuePattern.Test(nil, String("{}{}")))
		assert.False(t, valuePattern.Test(nil, String("[")))
		assert.False(t, valuePattern.Test(nil, String("[true")))
		assert.False(t, valuePattern.Test(nil, String("[true,")))
		assert.False(t, valuePattern.Test(nil, String(`{"string"}`)))
		assert.False(t, valuePattern.Test(nil, String(`{"string":}`)))
		assert.False(t, valuePattern.Test(nil, String(`{"string":[}`)))
	})
}

func TestLengthCheckingStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {
		pattern := NewLengthCheckingStringPattern(0, 1)

		assert.Equal(t, IntRange{
			start: 0,
			end:   1,
			step:  1,
		}, pattern.LengthRange())

	})

	t.Run("Test()", func(t *testing.T) {
		maxLen1 := NewLengthCheckingStringPattern(0, 1)

		assert.True(t, maxLen1.Test(nil, String("")))
		assert.True(t, maxLen1.Test(nil, String("a")))
		assert.False(t, maxLen1.Test(nil, String("ab")))
		assert.False(t, maxLen1.Test(nil, String("abc")))

		maxLen2 := NewLengthCheckingStringPattern(0, 2)

		assert.True(t, maxLen2.Test(nil, String("")))
		assert.True(t, maxLen2.Test(nil, String("a")))
		assert.True(t, maxLen2.Test(nil, String("ab")))
		assert.False(t, maxLen2.Test(nil, String("abc")))
		assert.False(t, maxLen2.Test(nil, String("abcd")))

		minLen1MaxLen2 := NewLengthCheckingStringPattern(1, 2)

		assert.True(t, minLen1MaxLen2.Test(nil, String("a")))
		assert.True(t, minLen1MaxLen2.Test(nil, String("ab")))
		assert.False(t, minLen1MaxLen2.Test(nil, String("")))
		assert.False(t, minLen1MaxLen2.Test(nil, String("abc")))
		assert.False(t, minLen1MaxLen2.Test(nil, String("abcd")))
	})
}

func TestSequenceStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {

		t.Run("single element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(ast.AtLeastOneOccurrence, -1, NewExactStringPattern(String("12"))),
			}, KeyList{""})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				start: 2,
				end:   math.MaxInt64,
				step:  1,
			}, patt.LengthRange())
		})

		t.Run("two elements, first one has no maximum length", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(ast.AtLeastOneOccurrence, -1, NewExactStringPattern(String("12"))),
				NewExactStringPattern(String("34")),
			}, KeyList{"", ""})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				start: 4,
				end:   math.MaxInt64,
				step:  1,
			}, patt.LengthRange())
		})

		t.Run("two elements, both have no maximum length", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(ast.AtLeastOneOccurrence, -1, NewExactStringPattern(String("12"))),
				newRepeatedPatternElement(ast.AtLeastOneOccurrence, -1, NewExactStringPattern(String("12"))),
			}, KeyList{"", ""})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				start: 4,
				end:   math.MaxInt64,
				step:  1,
			}, patt.LengthRange())
		})

	})

	t.Run(".MatchGroups()", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(String("12")),
			}, []string{"number"})
			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, String("12"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      String("12"),
				"number": String("12"),
			}, result)
		})

		t.Run("single repeated element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				newRepeatedPatternElement(ast.AtLeastOneOccurrence, -1, NewExactStringPattern(String("12"))),
			}, []string{"number"})

			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, String("1212"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      String("1212"),
				"number": String("1212"),
			}, result)
		})

		t.Run("two named elements", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(String("12")),
				NewExactStringPattern(String("AB")),
			}, []string{"digits", "letters"})
			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, String("12AB"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":       String("12AB"),
				"digits":  String("12"),
				"letters": String("AB"),
			}, result)
		})

		t.Run("two elements, first is named", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(String("12")),
				NewExactStringPattern(String("AB")),
			}, []string{"digits", ""})

			if !assert.NoError(t, err) {
				return
			}

			result, ok, err := patt.MatchGroups(nil, String("12AB"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      String("12AB"),
				"digits": String("12"),
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
			start: 1,
			end:   1,
			step:  1,
		}, patt.LengthRange())
	})

}

func TestIntRangeStringPattern(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	max := int64(math.MaxInt64)
	min := int64(math.MinInt64)

	minS := strconv.FormatInt(min, 10)
	minPlusOneS := strconv.FormatInt(min+1, 10)
	maxS := strconv.FormatInt(max, 10)
	maxMinusOneS := strconv.FormatInt(max-1, 10)
	maxCharCount := int64(len(minS))

	assertTestAndParse := func(t *testing.T, stringPattern StringPattern, s string) {
		assert.True(t, stringPattern.Test(ctx, String(s)))

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
		assert.False(t, stringPattern.Test(ctx, String(s)))

		_, err := stringPattern.Parse(ctx, s)
		assert.Error(t, err)
	}

	pattern := NewIntRangeStringPattern(min, 0, nil)
	assert.Equal(t, NewIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(min+1, 0, nil)
	assert.Equal(t, NewIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, minS)
	assertDoesNotTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(min, max, nil)
	assert.Equal(t, NewIntRange(1, maxCharCount), pattern.LengthRange())
	assertTestAndParse(t, pattern, minS)
	assertTestAndParse(t, pattern, minPlusOneS)
	assertTestAndParse(t, pattern, maxS)
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "1")
	assertTestAndParse(t, pattern, "2")
	assertDoesNotTestAndParse(t, pattern, "-0")

	pattern = NewIntRangeStringPattern(min, max-1, nil)
	assert.Equal(t, NewIntRange(1, maxCharCount), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 1), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertDoesNotTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(0, 1, nil)
	assert.Equal(t, NewIntRange(1, 1), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(1, 2, nil)
	assert.Equal(t, NewIntRange(1, 1), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(1, 9, nil)
	assert.Equal(t, NewIntRange(1, 1), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertDoesNotTestAndParse(t, pattern, "10")

	pattern = NewIntRangeStringPattern(1, 10, nil)
	assert.Equal(t, NewIntRange(1, 2), pattern.LengthRange())
	assertDoesNotTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertDoesNotTestAndParse(t, pattern, "-1")
	assertTestAndParse(t, pattern, "2")
	assertTestAndParse(t, pattern, "9")
	assertTestAndParse(t, pattern, "10")
	assertDoesNotTestAndParse(t, pattern, "11")

	pattern = NewIntRangeStringPattern(1, 99, nil)
	assert.Equal(t, NewIntRange(1, 2), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 3), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, int64(utils.CountDigits(max))), pattern.LengthRange())
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
	// assert.Equal(t, NewIntRange(2, int64(1+utils.CountDigits(min))), pattern.LengthRange())
	// assertDoesNotTestAndParse(t, pattern, "0")
	// assertDoesNotTestAndParse(t, pattern, "1")
	// assertDoesNotTestAndParse(t, pattern, "-0")
	// assertTestAndParse(t, pattern, "-1")
	// assertTestAndParse(t, pattern, "-100")
	// assertDoesNotTestAndParse(t, pattern, strconv.FormatInt(min, 10))
	// assertTestAndParse(t, pattern, strconv.FormatInt(min+1, 10))

	pattern = NewIntRangeStringPattern(-1, 1, nil)
	assert.Equal(t, NewIntRange(1, 2), pattern.LengthRange())
	assertTestAndParse(t, pattern, "0")
	assertTestAndParse(t, pattern, "1")
	assertDoesNotTestAndParse(t, pattern, "-0")
	assertTestAndParse(t, pattern, "-1")
	assertDoesNotTestAndParse(t, pattern, "2")

	pattern = NewIntRangeStringPattern(-1, 9, nil)
	assert.Equal(t, NewIntRange(1, 2), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 2), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 3), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 3), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 3), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 3), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 4), pattern.LengthRange())
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
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	max := float64(math.MaxFloat64)
	min := -float64(math.MaxFloat64)

	minS := strconv.FormatFloat(min, 'g', -1, 64)
	maxS := strconv.FormatFloat(max, 'g', -1, 64)

	assertTestAndParse := func(t *testing.T, stringPattern StringPattern, s string) {
		assert.True(t, stringPattern.Test(ctx, String(s)))

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
		assert.False(t, stringPattern.Test(ctx, String(s)))

		_, err := stringPattern.Parse(ctx, s)
		assert.Error(t, err)
	}

	pattern := NewFloatRangeStringPattern(min, 0, nil)
	assert.Equal(t, NewIntRange(1, 1+int64(MAX_CHAR_COUNT_MAXIMUM_FLOAT_64)), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, 1+int64(MAX_CHAR_COUNT_MAXIMUM_FLOAT_64)), pattern.LengthRange())
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
	assert.Equal(t, NewIntRange(1, int64(MAX_CHAR_COUNT_MAXIMUM_FLOAT_64)), pattern.LengthRange())
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
			NewExactStringPattern(String("a")),
			NewExactStringPattern(String("bc")),
		}))
		assert.Equal(t, IntRange{
			start: 1,
			end:   2,
			step:  1,
		}, patt.LengthRange())
	})

	t.Run(".MatchGroups()", func(t *testing.T) {
		patt, _ := NewUnionStringPattern(nil, []StringPattern{
			utils.Must(NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(String("12")),
			}, []string{"number"})),

			utils.Must(NewSequenceStringPattern(nil, nil, []StringPattern{
				NewExactStringPattern(String("ab")),
			}, []string{"string"})),
		})

		t.Run("matching string", func(t *testing.T) {
			result, ok, err := patt.MatchGroups(nil, String("12"))
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, map[string]Serializable{
				"0":      String("12"),
				"number": String("12"),
			}, result)
		})

		t.Run("matching string with additional characters", func(t *testing.T) {
			result, ok, err := patt.MatchGroups(nil, String("123"))
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

				patt := NewRegexPattern(regex)
				assert.Equal(t, expectedRange, patt.LengthRange())
			})
		}
	})

}
