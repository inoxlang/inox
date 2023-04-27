package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"github.com/muesli/termenv"

	core "github.com/inoxlang/inox/internal/core"

	_ "embed"
)

const (
	INOX_APP_NAME = "inox"

	SHELL_STARTUP_SCRIPT_NAME = "startup.ix"
	STARTUP_SCRIPT_RELPATH    = INOX_APP_NAME + "/" + SHELL_STARTUP_SCRIPT_NAME
	STARTUP_SCRIPT_PERM       = 0o700
)

var (
	//go:embed default_startup.ix
	DEFAULT_STARTUP_SCRIPT_CODE string
	USER_HOME                   string
	FORCE_COLOR                 bool
	TRUECOLOR_COLORTERM         bool
	TERM_256COLOR_CAPABLE       bool

	// set if FORCE_COLOR | TRUECOLOR_COLORTERM | TERM_256COLOR_CAPABLE
	INITIAL_FG_COLOR core.Color
	INITIAL_BG_COLOR core.Color
)

func init() {
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
		number, _ := strconv.Atoi(s)
		FORCE_COLOR = len(s) == 0 || number != 0
	}

	//TERMCOLOR

	TRUECOLOR_COLORTERM = os.Getenv("COLORTERM") == "truecolor"

	//TERM

	term := os.Getenv("TERM")
	if strings.Contains(term, "256color") {
		TERM_256COLOR_CAPABLE = true
	}

	//

	if FORCE_COLOR || TRUECOLOR_COLORTERM || TERM_256COLOR_CAPABLE {
		INITIAL_BG_COLOR = core.ColorFromTermenvColor(termenv.BackgroundColor(), core.ColorFromTermenvColor(termenv.ANSIBlack))
		INITIAL_FG_COLOR = core.ColorFromTermenvColor(termenv.ForegroundColor(), core.ColorFromTermenvColor(termenv.ANSIWhite))
	}
}

// GetStartupScriptPath searches for the startup script, creates if if it does not exist and returns its path.
func GetStartupScriptPath() (string, error) {

	path, err := xdg.SearchConfigFile(STARTUP_SCRIPT_RELPATH)
	if err != nil {
		path, err = xdg.ConfigFile(STARTUP_SCRIPT_RELPATH)
		if err != nil {
			return "", err
		}

		code := strings.ReplaceAll(DEFAULT_STARTUP_SCRIPT_CODE, "/home/user/", USER_HOME)

		if err := os.WriteFile(path, []byte(code), STARTUP_SCRIPT_PERM); err != nil {
			return "", err
		}
	}

	return path, nil
}
