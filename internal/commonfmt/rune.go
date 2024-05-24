package commonfmt

import (
	"fmt"

	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	QUOTED_BELL_RUNE   = []byte("'\\b'")
	QUOTED_FFEED_RUNE  = []byte("'\\f'")
	QUOTED_NL_RUNE     = []byte("'\\n'")
	QUOTED_CR_RUNE     = []byte("'\\r'")
	QUOTED_TAB_RUNE    = []byte("'\\t'")
	QUOTED_VTAB_RUNE   = []byte("'\\v'")
	QUOTED_SQUOTE_RUNE = []byte("'\\''")
	QUOTED_ASLASH_RUNE = []byte("'\\\\'")
)

func FmtRune(r rune) string {
	var b []byte
	switch r {
	case '\b':
		b = QUOTED_BELL_RUNE
	case '\f':
		b = QUOTED_FFEED_RUNE
	case '\n':
		b = QUOTED_NL_RUNE
	case '\r':
		b = QUOTED_CR_RUNE
	case '\t':
		b = QUOTED_TAB_RUNE
	case '\v':
		b = QUOTED_VTAB_RUNE
	case '\'':
		b = QUOTED_SQUOTE_RUNE
	case '\\':
		b = QUOTED_ASLASH_RUNE
	default:
		b = utils.StringAsBytes(fmt.Sprintf("'%c'", r))
	}

	return string(b)
}
