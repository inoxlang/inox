package parse

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCutQuotedStringLiteral(t *testing.T) {

	t.Run("empty string", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`""`)).(*QuotedStringLiteral)

		cut, ok := CutQuotedStringLiteral(1, lit)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, stringCut{
			BeforeIndex:    "",
			AfterIndex:     "",
			IsIndexAtStart: true,
			IsIndexAtEnd:   true,
			IsStringEmpty:  true,
		}, cut)
	})

	t.Run("single-char string: ASCII", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`"a"`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:    "",
				AfterIndex:     "a",
				IsIndexAtStart: true,
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  "a",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})

	t.Run("single-char string: non-ASCII", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`"é"`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:    "",
				AfterIndex:     "é",
				IsIndexAtStart: true,
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  "é",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})

	t.Run("single-char string: space", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`" "`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:        "",
				AfterIndex:         " ",
				IsIndexAtStart:     true,
				HasSpaceAfterIndex: true,
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  " ",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})

	t.Run("two-char string: two ASCII chars", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`"aa"`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:    "",
				AfterIndex:     "aa",
				IsIndexAtStart: true,
			}, cut)
		})

		t.Run("in middle", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex: "a",
				AfterIndex:  "a",
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(3, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  "aa",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})

	t.Run("two-char string: two non-ASCII chars", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`"éé"`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:    "",
				AfterIndex:     "éé",
				IsIndexAtStart: true,
			}, cut)
		})

		t.Run("in middle", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex: "é",
				AfterIndex:  "é",
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(3, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  "éé",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})

	t.Run("two-char string: non-space char and space", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`"a "`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:    "",
				AfterIndex:     "a ",
				IsIndexAtStart: true,
			}, cut)
		})

		t.Run("in middle", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:        "a",
				AfterIndex:         " ",
				HasSpaceAfterIndex: true,
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(3, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  "a ",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})

	t.Run("three-char string: all non-space", func(t *testing.T) {
		lit := utils.MustGet(ParseExpression(`"aaa"`)).(*QuotedStringLiteral)

		t.Run("at start", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(1, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:    "",
				AfterIndex:     "aaa",
				IsIndexAtStart: true,
			}, cut)
		})

		t.Run("in lower middle", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(2, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex: "a",
				AfterIndex:  "aa",
			}, cut)
		})

		t.Run("in upper middle", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(3, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex: "aa",
				AfterIndex:  "a",
			}, cut)
		})

		t.Run("at end", func(t *testing.T) {
			cut, ok := CutQuotedStringLiteral(4, lit)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, stringCut{
				BeforeIndex:  "aaa",
				AfterIndex:   "",
				IsIndexAtEnd: true,
			}, cut)
		})
	})
}
