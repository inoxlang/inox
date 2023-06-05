package core

import (
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func TestColorConversion(t *testing.T) {

	black := ColorFromTermenvColor(termenv.ANSIBlack)
	assert.Equal(t, termenv.ANSIBlack, black.ToTermColor())

	green256 := ColorFromTermenvColor(termenv.ANSI256Color(22))
	assert.Equal(t, termenv.ANSI256Color(22), green256.ToTermColor())

	whiteTrueColor := ColorFromTermenvColor(termenv.RGBColor("#ffffff"))
	assert.Equal(t, termenv.RGBColor("#ffffff"), whiteTrueColor.ToTermColor())
}
