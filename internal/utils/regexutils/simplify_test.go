package regexutils

import (
	"regexp/syntax"
	"testing"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestTurnAAStarToAplus(t *testing.T) {
	assert.Equal(t, parseRegex("a+").String(), turnAAStarIntoAplus(parseRegex("aa*")).String())
	assert.Equal(t, parseRegex("a+a+").String(), turnAAStarIntoAplus(parseRegex("aa*aa*")).String())
	assert.Equal(t, parseRegex("aa+").String(), turnAAStarIntoAplus(parseRegex("aaa*")).String())

	//Non-capturing groups.
	assert.Equal(t, parseRegex("(?:ab)+").String(), turnAAStarIntoAplus(parseRegex("(?:ab)(?:ab)*")).String())
	assert.Equal(t, parseRegex("(?:ab)(?:ab)+").String(), turnAAStarIntoAplus(parseRegex("(?:ab)(?:ab)(?:ab)*")).String())

	//Capturing groups are not supported.
	assert.Panics(t, func() {
		turnAAStarIntoAplus(parseRegex("(ab)(ab)*"))
	})
}

func parseRegex(s string) *syntax.Regexp {
	return utils.Must(syntax.Parse(s, syntax.Perl))
}
