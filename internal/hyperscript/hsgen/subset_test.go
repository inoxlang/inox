package hsgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	assert.NotEmpty(t, COMMAND_DEFINITIONS)
}

func TestGenerate(t *testing.T) {

	t.Run("all commands", func(t *testing.T) {
		result, err := Generate(Config{
			Commands: COMMAND_NAMES,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Equal(t, len(HYPERSCRIPT_0_9_12_JS), len(result)) {
			return
		}
		assert.Equal(t, HYPERSCRIPT_0_9_12_JS, result)
	})

	t.Run("no commands", func(t *testing.T) {
		result, err := Generate(Config{
			Commands: []string{},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
	})

	t.Run("first command", func(t *testing.T) {
		firstCmdDef := COMMAND_DEFINITIONS[0]

		result, err := Generate(Config{
			Commands: []string{firstCmdDef.CommandName},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, firstCmdDef.Code)
	})

	t.Run("last command", func(t *testing.T) {
		lastCmdDef := COMMAND_DEFINITIONS[len(COMMAND_DEFINITIONS)-1]
		result, err := Generate(Config{
			Commands: []string{lastCmdDef.CommandName},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, lastCmdDef.Code)
	})

	t.Run("other command", func(t *testing.T) {
		cmdDef := COMMAND_DEFINITIONS[2]

		result, err := Generate(Config{
			Commands: []string{cmdDef.CommandName},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Greater(t, len(HYPERSCRIPT_0_9_12_JS), len(result))
		assert.Contains(t, result, cmdDef.Code)
	})
}
