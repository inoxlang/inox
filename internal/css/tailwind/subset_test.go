package tailwind

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetRulesetsFromSubset(t *testing.T) {
	if TAILWIND_SUBSET_RULESETS == nil {
		utils.PanicIfErr(InitSubset())
	}

	t.Run("prefix with matches", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".h")
		if !assert.NotEmpty(t, rulesets) {
			return
		}
		for _, ruleset := range rulesets {
			assert.True(t, strings.HasPrefix(ruleset.BaseName, ".h"))
		}
	})

	t.Run("exact match that is also a prefix", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".flex")
		assert.Greater(t, len(rulesets), 1)
	})

	t.Run("prefix with no matches", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".z")
		assert.Empty(t, rulesets)
	})

	t.Run("class name without escaped dot", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".h-0.5")
		if !assert.NotEmpty(t, rulesets) {
			return
		}
		assert.Equal(t, len(rulesets), 1)
	})

	t.Run("class name with escaped dot", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".h-0\\.5")
		if !assert.NotEmpty(t, rulesets) {
			return
		}
		assert.Equal(t, len(rulesets), 1)
	})

	t.Run("class name without escaped slash", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".h-1/2")
		if !assert.NotEmpty(t, rulesets) {
			return
		}
		assert.Equal(t, len(rulesets), 1)
	})

	t.Run("class name with escaped slash", func(t *testing.T) {
		rulesets := GetRulesetsFromSubset(".h-1\\/2")
		if !assert.NotEmpty(t, rulesets) {
			return
		}
		assert.Equal(t, len(rulesets), 1)
	})
}
