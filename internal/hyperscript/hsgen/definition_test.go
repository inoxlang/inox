package hsgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefinition(t *testing.T) {

	t.Run("base case", func(t *testing.T) {
		region := GetDefinition(0, CommandDefinition, `parser.addCommand("a", {})`)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "a",
			Start: 0,
			End:   26,
			Code:  `parser.addCommand("a", {})`,
		}, region)
	})

	t.Run("trailing semicolon", func(t *testing.T) {
		region := GetDefinition(0, CommandDefinition, `parser.addCommand("a", {});`)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "a",
			Start: 0,
			End:   27,
			Code:  `parser.addCommand("a", {});`,
		}, region)
	})

	t.Run("feature", func(t *testing.T) {
		region := GetDefinition(0, FeatureDefinition, `parser.addFeature("a", {})`)

		assert.Equal(t, Definition{
			Kind:  FeatureDefinition,
			Name:  "a",
			Start: 0,
			End:   26,
			Code:  `parser.addFeature("a", {})`,
		}, region)

	})

	t.Run("long definition", func(t *testing.T) {
		code := `parser.addCommand("a", {a:"` + string(strings.Repeat("x", 100_000)) + `" })`
		region := GetDefinition(0, CommandDefinition, code)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "a",
			Start: 0,
			End:   len(code),
			Code:  code,
		}, region)
	})

	t.Run("multiple definitions separated by a space", func(t *testing.T) {
		code := `parser.addCommand("a", {}); parser.addCommand("b", {})`

		region := GetDefinition(0, CommandDefinition, code)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "a",
			Start: 0,
			End:   27,
			Code:  `parser.addCommand("a", {});`,
		}, region)

		region = GetDefinition(28, CommandDefinition, code)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "b",
			Start: 28,
			End:   54,
			Code:  `parser.addCommand("b", {})`,
		}, region)

	})

	t.Run("multiple definitions separated by a linefeed", func(t *testing.T) {
		code := "parser.addCommand(\"a\", {});\nparser.addCommand(\"b\", {})"

		region := GetDefinition(0, CommandDefinition, code)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "a",
			Start: 0,
			End:   27,
			Code:  `parser.addCommand("a", {});`,
		}, region)

		region = GetDefinition(28, CommandDefinition, code)

		assert.Equal(t, Definition{
			Kind:  CommandDefinition,
			Name:  "b",
			Start: 28,
			End:   54,
			Code:  `parser.addCommand("b", {})`,
		}, region)

	})

}
