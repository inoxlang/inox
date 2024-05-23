package regexutils

import (
	"regexp/syntax"
	"testing"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestTurnCapturingGroupsIntoNonCapturing(t *testing.T) {
	turn := func(s string) string {
		regex := utils.Must(syntax.Parse(s, syntax.Perl))
		return TurnCapturingGroupsIntoNonCapturing(regex).String()
	}

	assert.Equal(t, "(?:)", turn("()"))
	assert.Equal(t, "(?:)", turn("(?:)"))
	assert.Equal(t, "a", turn("(?:a)"))
	assert.Equal(t, "a", turn("(a)"))
	//	assert.Equal(t, "\\Aa(?-m:$)", turn("^a$")) //equivalent, fix ?
	assert.Equal(t, "(?-m:\\Aa$)", turn("^a$")) //equivalent, fix ?
	assert.Equal(t, "\\(\\)", turn("\\(\\)"))
	//assert.Equal(t, "", turn("\\\\(\\\\)"))
	//assert.Equal(t, "[\\(-\\)]", turn("[()]"))
	assert.Equal(t, "[\\(\\)]", turn("[()]"))

	assert.Equal(t, "[a-z]", turn("([a-z])"))
	assert.Equal(t, "(?:[a-z]0*)?c", turn("([a-z]0*)?c"))
	assert.Equal(t, "(?:[a-z]0*(?:ab)+)?c", turn("([a-z]0*(?:ab)+)?c"))

	assert.Equal(t, "aa", turn("aa"))
	assert.Equal(t, "aa*", turn("aa*"))
	assert.Equal(t, "aa+a*", turn("aa+a*"))
	assert.Equal(t, "(?:aa+)+a*", turn("(aa+)+a*"))
	assert.Equal(t, "(?:aa+)+a*b*", turn("(aa+)+a*b*"))
}
