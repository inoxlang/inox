package hsgen

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	assert.NotEmpty(t, BUILTIN_DEFINITIONS)
	assert.NotEmpty(t, COMMAND_NAMES)
	assert.NotEmpty(t, FEATURE_NAMES)
	assert.Equal(t, len(BUILTIN_DEFINITIONS), len(FEATURE_NAMES)+len(COMMAND_NAMES))
	fmt.Println(BUILTIN_DEFINITIONS[0])
}

func TestGenerate(t *testing.T) {

	t.Run("all definitions", func(t *testing.T) {
		result, err := Generate(Config{
			RequiredCommands:     COMMAND_NAMES,
			RequiredFeatureNames: FEATURE_NAMES,
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
			RequiredCommands: []string{},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
	})

	t.Run("first definition", func(t *testing.T) {
		firstDef := BUILTIN_DEFINITIONS[0]

		result, err := Generate(Config{
			RequiredFeatureNames: []string{firstDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, firstDef.Code)
	})

	t.Run("last definition", func(t *testing.T) {
		lastDef := BUILTIN_DEFINITIONS[len(BUILTIN_DEFINITIONS)-1]
		result, err := Generate(Config{
			RequiredCommands: []string{lastDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, lastDef.Code)
	})

	t.Run("second feature", func(t *testing.T) {
		secondFeatureDef := BUILTIN_DEFINITIONS[2]

		result, err := Generate(Config{
			RequiredFeatureNames: []string{secondFeatureDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, secondFeatureDef.Code)
	})

	t.Run("first feature and first command", func(t *testing.T) {
		firstFeatureDef := BUILTIN_DEFINITIONS[0]

		firstCmdIndex := slices.IndexFunc(BUILTIN_DEFINITIONS, func(d Definition) bool {
			return d.Kind == CommandDefinition
		})

		firstCmdDef := BUILTIN_DEFINITIONS[firstCmdIndex]

		result, err := Generate(Config{
			RequiredFeatureNames: []string{firstFeatureDef.Name},
			RequiredCommands:     []string{firstCmdDef.Name},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, firstFeatureDef.Code)
		assert.Contains(t, result, firstCmdDef.Code)
	})

}
