package codecompletion

import (
	"bytes"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestWriteInputs(t *testing.T) {

	t.Run("{name: str}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern:  core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
		})

		assert.Regexp(t, `<input name="name".*type="text" required/>`, buf.String())
	})

	t.Run("#{name: str}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern:  core.NewInexactRecordPattern([]core.RecordPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
		})

		assert.Regexp(t, `<input name="name".*type="text" required/>`, buf.String())
	})

	t.Run("{name?: str}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern:  core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN, IsOptional: true}}),
		})

		assert.Regexp(t, `<input name="name".*type="text"/>`, buf.String())
	})

	t.Run("#{name?: str}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern:  core.NewInexactRecordPattern([]core.RecordPatternEntry{{Name: "name", Pattern: core.STR_PATTERN, IsOptional: true}}),
		})

		assert.Regexp(t, `<input name="name".*type="text"/>`, buf.String())
	})

	t.Run("{name: str, age: int}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{Name: "name", Pattern: core.STR_PATTERN},
				{Name: "age", Pattern: core.INT_PATTERN},
			}),
		})

		s := buf.String()

		assert.Regexp(t, `<input name="name".*type="text" required/>`, s)
		assert.Regexp(t, `<input name="age".*type="number" step="1" required/>`, s)
	})

	t.Run("{user: {name: str, age: int}}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{
					Name: "user",
					Pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
						{Name: "name", Pattern: core.STR_PATTERN},
						{Name: "age", Pattern: core.INT_PATTERN},
					}),
				},
			}),
		})

		s := buf.String()

		assert.Regexp(t, `<input name="user.name".*type="text" required/>`, s)
		assert.Regexp(t, `<input name="user.age".*type="number" step="1" required/>`, s)
	})

	t.Run("{pair: [float, float]}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{
					Name:    "pair",
					Pattern: core.NewListPattern([]core.Pattern{core.FLOAT_PATTERN, core.FLOAT_PATTERN}),
				},
			}),
		})

		s := buf.String()

		assert.Regexp(t, `<input name="pair\[0\]".*type="number" required/>`, s)
		assert.Regexp(t, `<input name="pair\[1\]".*type="number" required/>`, s)
	})

	t.Run("{pair: #[float, float]}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{
					Name:    "pair",
					Pattern: core.NewTuplePattern([]core.Pattern{core.FLOAT_PATTERN, core.FLOAT_PATTERN}),
				},
			}),
		})

		s := buf.String()

		assert.Regexp(t, `<input name="pair\[0\]".*type="number" required/>`, s)
		assert.Regexp(t, `<input name="pair\[1\]".*type="number" required/>`, s)
	})

	t.Run("{pair: [{name: str}, {name: str}]}", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		writeHtmlInputs(buf, formInputGeneration{
			required: true,
			pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
				{
					Name: "pair",
					Pattern: core.NewListPattern([]core.Pattern{
						core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
						core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
					}),
				},
			}),
		})

		s := buf.String()

		assert.Regexp(t, `<input name="pair\[0\].name".*type="text" required/>`, s)
		assert.Regexp(t, `<input name="pair\[1\].name".*type="text" required/>`, s)
	})
}
