package config

import (
	"os"
	"strings"

	"github.com/adrg/xdg"

	core "github.com/inoxlang/inox/internal/core"

	_ "embed"
)

const (
	INOX_APP_NAME = "inox"

	SHELL_STARTUP_SCRIPT_NAME = "startup.ix"
	STARTUP_SCRIPT_RELPATH    = INOX_APP_NAME + "/" + SHELL_STARTUP_SCRIPT_NAME
	STARTUP_SCRIPT_PERM       = 0o700

	DEFAULT_TRUSTED_RISK_SCORE = core.MEDIUM_RISK_SCORE_LEVEL - 1
)

var (
	//go:embed default_startup.ix
	DEFAULT_STARTUP_SCRIPT_CODE string
	USER_HOME                   string
	FORCE_COLOR                 bool
	TRUECOLOR_COLORTERM         bool
	TERM_256COLOR_CAPABLE       bool
	NO_COLOR                    bool
	SHOULD_COLORIZE             bool

	// set if SHOULD_COLORIZE
	INITIAL_COLORS_SET bool
	INITIAL_FG_COLOR   core.Color
	INITIAL_BG_COLOR   core.Color
)

func init() {
	targetSpecificInit()
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
