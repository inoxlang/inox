package hsgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefinition(t *testing.T) {

	region := GetDefinition(0, CommandDefinition, `parser.addCommand("a", {})`)

	assert.Equal(t, Definition{
		Kind:  CommandDefinition,
		Name:  "a",
		Start: 0,
		End:   26,
		Code:  `parser.addCommand("a", {})`,
	}, region)

	region = GetDefinition(0, FeatureDefinition, `parser.addFeature("a", {})`)

	assert.Equal(t, Definition{
		Kind:  FeatureDefinition,
		Name:  "a",
		Start: 0,
		End:   26,
		Code:  `parser.addFeature("a", {})`,
	}, region)

	region = GetDefinition(28, CommandDefinition, `parser.addCommand("a", {}); parser.addCommand("b", {})`)

	assert.Equal(t, Definition{
		Kind:  CommandDefinition,
		Name:  "b",
		Start: 28,
		End:   54,
		Code:  `parser.addCommand("b", {})`,
	}, region)
}
