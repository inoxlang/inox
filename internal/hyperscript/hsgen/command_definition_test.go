package hsgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommandDefinition(t *testing.T) {

	region := GetCommandDefinition(0, `parser.addCommand("a", {})`)

	assert.Equal(t, CommandDefinition{
		CommandName: "a",
		Start:       0,
		End:         26,
		Code:        `parser.addCommand("a", {})`,
	}, region)

	region = GetCommandDefinition(28, `parser.addCommand("a", {}); parser.addCommand("b", {})`)

	assert.Equal(t, CommandDefinition{
		CommandName: "b",
		Start:       28,
		End:         54,
		Code:        `parser.addCommand("b", {})`,
	}, region)
}
