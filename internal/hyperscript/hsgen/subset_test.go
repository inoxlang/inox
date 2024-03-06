package hsgen

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	assert.NotEmpty(t, DEFINITIONS)
	assert.NotEmpty(t, COMMAND_NAMES)
	assert.NotEmpty(t, FEATURE_NAMES)
	assert.Equal(t, len(DEFINITIONS), len(FEATURE_NAMES)+len(COMMAND_NAMES))
	fmt.Println(DEFINITIONS[0])
}

func TestGenerate(t *testing.T) {

	t.Run("all definitions", func(t *testing.T) {
		result, err := Generate(Config{
			Commands:     COMMAND_NAMES,
			FeatureNames: FEATURE_NAMES,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Equal(t, len(HYPERSCRIPT_0_9_12_JS), len(result)) {
			return
		}
		assert.Equal(t, HYPERSCRIPT_0_9_12_JS, result)
	})

	t.Run("no definitions", func(t *testing.T) {
		result, err := Generate(Config{
			Commands: []string{},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
	})

	t.Run("first definition", func(t *testing.T) {
		firstDef := DEFINITIONS[0]

		result, err := Generate(Config{
			FeatureNames: []string{firstDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, firstDef.Code)
	})

	t.Run("last definition", func(t *testing.T) {
		lastDef := DEFINITIONS[len(DEFINITIONS)-1]
		result, err := Generate(Config{
			Commands: []string{lastDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, lastDef.Code)
	})

	t.Run("second feature", func(t *testing.T) {
		secondFeatureDef := DEFINITIONS[2]

		result, err := Generate(Config{
			FeatureNames: []string{secondFeatureDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, secondFeatureDef.Code)
	})

	t.Run("first feature and first command", func(t *testing.T) {
		firstFeatureDef := DEFINITIONS[0]

		firstCmdIndex := slices.IndexFunc(DEFINITIONS, func(d Definition) bool {
			return d.Kind == CommandDefinition
		})

		firstCmdDef := DEFINITIONS[firstCmdIndex]

		result, err := Generate(Config{
			FeatureNames: []string{firstFeatureDef.Name},
			Commands:     []string{firstCmdDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, firstFeatureDef.Code)
		assert.Contains(t, result, firstCmdDef.Code)
	})

}
