package tailwind

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetRulesetsFromSubset(t *testing.T) {
	if TAILWIND_SUBSET_RULESETS == nil {
		utils.PanicIfErr(InitTailCSS())
	}

	rulesets := GetRulesetsFromSubset(".h")
	if !assert.NotEmpty(t, rulesets) {
		return
	}
	for _, ruleset := range rulesets {
		assert.True(t, strings.HasPrefix(ruleset.Name, ".h"))
	}
}
