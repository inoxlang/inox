//go:build unix

package config

import (
	"os"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/muesli/termenv"
)

const (
	UNIX = true
	WASM = false
)

func targetSpecificInit() {
	// HOME

	HOME, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	if HOME[len(HOME)-1] != '/' {
		HOME += "/"
	}

	USER_HOME = HOME

	// FORCE COLOR

	if s, ok := os.LookupEnv("FORCE_COLOR"); ok {
		FORCE_COLOR = len(s) != 0 && s != "false" && s != "0"
	}

	//TERMCOLOR

	TRUECOLOR_COLORTERM = os.Getenv("COLORTERM") == "truecolor"

	//NO_COLOR

	if s, ok := os.LookupEnv("NO_COLOR"); ok {
		NO_COLOR = len(s) != 0 && s != "false" && s != "0"
	}
	//TERM

	term := os.Getenv("TERM")
	if strings.Contains(term, "256color") {
		TERM_256COLOR_CAPABLE = true
	}

	//

	SHOULD_COLORIZE = !NO_COLOR && (FORCE_COLOR || TRUECOLOR_COLORTERM || TERM_256COLOR_CAPABLE)

	if SHOULD_COLORIZE {
		INITIAL_COLORS_SET = true
		INITIAL_BG_COLOR = core.ColorFromTermenvColor(termenv.BackgroundColor(), core.ColorFromTermenvColor(termenv.ANSIBlack))
		INITIAL_FG_COLOR = core.ColorFromTermenvColor(termenv.ForegroundColor(), core.ColorFromTermenvColor(termenv.ANSIWhite))
	}
}
