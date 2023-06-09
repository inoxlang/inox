package core

import (
	"math"
	"regexp/syntax"
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestEvalStringPatternNode(t *testing.T) {

	t.Run("single element : string literal", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		state := NewTreeWalkState(ctx)

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
		state := NewTreeWalkState(ctx)

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

	t.Run("single element : string literal (ocurrence modifier i '*')", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		state := NewTreeWalkState(ctx)

		patt, err := evalStringPatternNode(&parse.ComplexStringPatternPiece{
			Elements: []*parse.PatternPieceElement{
				{
					Ocurrence: parse.ZeroOrMoreOcurrence,
					Expr:      &parse.QuotedStringLiteral{Value: "s"},
				},
			},
		}, state, false)

		assert.NoError(t, err)
		assert.Equal(t, "((?:s)*)", patt.Regex())
		assert.True(t, patt.Test(nil, Str("s")))
		assert.True(t, patt.Test(nil, Str("ss")))
		assert.False(t, patt.Test(nil, Str("ssa")))
		assert.False(t, patt.Test(nil, Str("assa")))
	})

	t.Run("single element : string literal (ocurrence modifier i '=' 2)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		state := NewTreeWalkState(ctx)

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
		assert.Equal(t, "((?:s){2})", patt.Regex())
		assert.True(t, patt.Test(nil, Str("ss")))
		assert.False(t, patt.Test(nil, Str("ssa")))
		assert.False(t, patt.Test(nil, Str("ass")))
	})

	t.Run("two elements : one string literal + a pattern identifier (exact string pattern)", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		ctx.AddNamedPattern("b", NewExactStringPattern(Str("c")))
		state := NewTreeWalkState(ctx)
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
		state := NewTreeWalkState(ctx)

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

	t.Run("union of two multiple-element cases", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		state := NewTreeWalkState(ctx)

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

	lenRange := IntRange{
		Start:        0,
		End:          math.MaxInt64,
		inclusiveEnd: true,
		Step:         1,
	}

	t.Run("sequence with a singme non repeated element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		ctx.AddNamedPattern("subpatt", NewExactStringPattern(Str("a")))

		patt := &SequenceStringPattern{
			elements: []StringPattern{
				&DynamicStringPatternElement{"subpatt", ctx},
			},
			lengthRange:          lenRange,
			effectiveLengthRange: lenRange,
		}

		assert.True(t, patt.Test(nil, Str("a")))
	})

	t.Run("sequence with a single repeated element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		ctx.AddNamedPattern("subpatt", NewExactStringPattern(Str("a")))

		patt := &SequenceStringPattern{
			elements: []StringPattern{
				&RepeatedPatternElement{
					ocurrenceModifier: parse.AtLeastOneOcurrence,
					exactCount:        -1,
					element:           &DynamicStringPatternElement{"subpatt", ctx},
				},
			},
			lengthRange:          lenRange,
			effectiveLengthRange: lenRange,
		}

		assert.True(t, patt.Test(nil, Str("a")))
		assert.True(t, patt.Test(nil, Str("aa")))
		assert.False(t, patt.Test(nil, Str("ba")))
		assert.False(t, patt.Test(nil, Str("ab")))
	})

	t.Run("sequence with two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		ctx.AddNamedPattern("subpatt", NewExactStringPattern(Str("a")))

		patt := &SequenceStringPattern{
			elements: []StringPattern{
				&RepeatedPatternElement{
					ocurrenceModifier: parse.ZeroOrMoreOcurrence,
					exactCount:        -1,
					element:           &DynamicStringPatternElement{"subpatt", ctx},
				},
				NewExactStringPattern(Str("b")),
			},
			lengthRange:          lenRange,
			effectiveLengthRange: lenRange,
		}

		assert.True(t, patt.Test(nil, Str("ab")))
		assert.True(t, patt.Test(nil, Str("aab")))
		assert.False(t, patt.Test(nil, Str("ba")))
		assert.True(t, patt.Test(nil, Str("ab")))
	})

	t.Run("recursion", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		valuePattern := &UnionStringPattern{
			cases: []StringPattern{
				&DynamicStringPatternElement{"bool", ctx},
				&DynamicStringPatternElement{"list", ctx},
			},
		}
		ctx.AddNamedPattern("value", valuePattern)

		ctx.AddNamedPattern("bool", &UnionStringPattern{
			cases: []StringPattern{
				NewExactStringPattern(Str("true")),
				NewExactStringPattern(Str("false")),
			},
		})

		ctx.AddNamedPattern("list", &SequenceStringPattern{
			elements: []StringPattern{
				NewExactStringPattern(Str("[")),
				&RepeatedPatternElement{
					ocurrenceModifier: parse.ZeroOrMoreOcurrence,
					exactCount:        -1,
					element: &SequenceStringPattern{
						elements: []StringPattern{
							&DynamicStringPatternElement{"value", ctx},
							NewExactStringPattern(Str(",")),
						},
					},
				},
				NewExactStringPattern(Str("]")),
			},
			lengthRange:          lenRange,
			effectiveLengthRange: lenRange,
		})

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
		valuePattern := &UnionStringPattern{
			cases: []StringPattern{
				&DynamicStringPatternElement{"string", ctx},
				&DynamicStringPatternElement{"bool", ctx},
				&DynamicStringPatternElement{"list", ctx},
				&DynamicStringPatternElement{"object", ctx},
			},
		}
		ctx.AddNamedPattern("value", valuePattern)

		ctx.AddNamedPattern("bool", &UnionStringPattern{
			cases: []StringPattern{
				NewExactStringPattern(Str("true")),
				NewExactStringPattern(Str("false")),
			},
		})

		ctx.AddNamedPattern("string", NewExactStringPattern(Str(`"string"`)))

		ctx.AddNamedPattern("list", &SequenceStringPattern{
			elements: []StringPattern{
				NewExactStringPattern(Str("[")),
				&RepeatedPatternElement{
					ocurrenceModifier: parse.ZeroOrMoreOcurrence,
					exactCount:        -1,
					element: &SequenceStringPattern{
						elements: []StringPattern{
							&DynamicStringPatternElement{"value", ctx},
							NewExactStringPattern(Str(",")),
						},
					},
				},
				NewExactStringPattern(Str("]")),
			},
			lengthRange:          lenRange,
			effectiveLengthRange: lenRange,
		})

		ctx.AddNamedPattern("object", &SequenceStringPattern{
			elements: []StringPattern{
				NewExactStringPattern(Str("{")),
				&RepeatedPatternElement{
					ocurrenceModifier: parse.ZeroOrMoreOcurrence,
					exactCount:        -1,
					element: &SequenceStringPattern{
						elements: []StringPattern{
							&DynamicStringPatternElement{"string", ctx},
							NewExactStringPattern(Str(":")),
							&DynamicStringPatternElement{"value", ctx},
						},
					},
				},
				NewExactStringPattern(Str("}")),
			},
			lengthRange:          lenRange,
			effectiveLengthRange: lenRange,
		})

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

func TestSequenceStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {

		t.Run("single element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
				&RepeatedPatternElement{
					ocurrenceModifier: parse.AtLeastOneOcurrence,
					element:           NewExactStringPattern(Str("12")),
				},
			}, nil)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				inclusiveEnd: true,
				Start:        2,
				End:          math.MaxInt64,
				Step:         1,
			}, patt.LengthRange())
		})

		t.Run("two elements, first one has no maximum length", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
				&RepeatedPatternElement{
					ocurrenceModifier: parse.AtLeastOneOcurrence,
					element:           NewExactStringPattern(Str("12")),
				},
				NewExactStringPattern(Str("34")),
			}, nil)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				inclusiveEnd: true,
				Start:        4,
				End:          math.MaxInt64,
				Step:         1,
			}, patt.LengthRange())
		})

		t.Run("two elements, both have no maximum length", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
				&RepeatedPatternElement{
					ocurrenceModifier: parse.AtLeastOneOcurrence,
					element:           NewExactStringPattern(Str("12")),
				},
				&RepeatedPatternElement{
					ocurrenceModifier: parse.AtLeastOneOcurrence,
					element:           NewExactStringPattern(Str("12")),
				},
			}, nil)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, IntRange{
				inclusiveEnd: true,
				Start:        4,
				End:          math.MaxInt64,
				Step:         1,
			}, patt.LengthRange())
		})

	})

	t.Run(".MatchGroups()", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
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
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
				&RepeatedPatternElement{
					ocurrenceModifier: parse.AtLeastOneOcurrence,
					element:           NewExactStringPattern(Str("12")),
				},
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
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
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
			patt, err := NewSequenceStringPattern(nil, []StringPattern{
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
			Start:        1,
			End:          1,
			inclusiveEnd: true,
			Step:         1,
		}, patt.LengthRange())
	})

}

func TestIntRangeStringPattern(t *testing.T) {
	//TODO
}

func TestUnionStringPattern(t *testing.T) {

	t.Run(".LengthRange()", func(t *testing.T) {
		patt := &UnionStringPattern{
			cases: []StringPattern{
				NewExactStringPattern(Str("a")),
				NewExactStringPattern(Str("bc")),
			},
		}
		assert.Equal(t, IntRange{
			Start:        1,
			End:          2,
			inclusiveEnd: true,
			Step:         1,
		}, patt.LengthRange())
	})

	t.Run(".MatchGroups()", func(t *testing.T) {
		patt, _ := NewUnionStringPattern(nil, []StringPattern{
			utils.Must(NewSequenceStringPattern(nil, []StringPattern{
				NewExactStringPattern(Str("12")),
			}, []string{"number"})),

			utils.Must(NewSequenceStringPattern(nil, []StringPattern{
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
				Start: 0,
				End:   0,
			},
			`a`: {
				Start: 1,
				End:   1,
			},
			`a?`: {
				Start: 0,
				End:   1,
			},
			`a+`: {
				Start: 1,
				End:   math.MaxInt64,
			},
			`a*`: {
				Start: 0,
				End:   math.MaxInt64,
			},
			`a{0,1}`: {
				Start: 0,
				End:   1,
			},
			`a{0,2}`: {
				Start: 0,
				End:   2,
			},
			`a{1,2}`: {
				Start: 1,
				End:   2,
			},
			`a{1,3}`: {
				Start: 1,
				End:   3,
			},
			`.`: {
				Start: 1,
				End:   1,
			},
			`[a-z]`: {
				Start: 1,
				End:   1,
			},
			`(a|bc)`: {
				Start: 1,
				End:   2,
			},
		}

		for regex, expectedRange := range testCases {
			t.Run("`"+regex+"`", func(t *testing.T) {
				expectedRange.Step = 1
				expectedRange.inclusiveEnd = true

				patt := NewRegexPattern(regex)
				assert.Equal(t, expectedRange, patt.LengthRange())
			})
		}
	})

}

func TestTurnCapturingGroupsIntoNonCapturing(t *testing.T) {
	turn := func(s string) string {
		regex := utils.Must(syntax.Parse(s, syntax.Perl))
		return turnCapturingGroupsIntoNonCapturing(regex).String()
	}

	assert.Equal(t, "(?:)", turn("()"))
	assert.Equal(t, "(?:)", turn("(?:)"))
	assert.Equal(t, "a", turn("(?:a)"))
	assert.Equal(t, "a", turn("(a)"))
	assert.Equal(t, "\\Aa(?-m:$)", turn("^a$")) //equivalent, fix ?
	assert.Equal(t, "\\(\\)", turn("\\(\\)"))
	//assert.Equal(t, "", turn("\\\\(\\\\)"))
	assert.Equal(t, "[\\(-\\)]", turn("[()]"))

	assert.Equal(t, "[a-z]", turn("([a-z])"))
	assert.Equal(t, "(?:[a-z]0*)?c", turn("([a-z]0*)?c"))
	assert.Equal(t, "(?:[a-z]0*(?:ab)+)?c", turn("([a-z]0*(?:ab)+)?c"))

}
