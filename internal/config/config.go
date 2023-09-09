package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/adrg/xdg"

	core "github.com/inoxlang/inox/internal/core"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"

	_ "embed"
)

const (
	INOX_APP_NAME = "inox"

	SHELL_STARTUP_SCRIPT_NAME = "startup.ix"
	STARTUP_SCRIPT_RELPATH    = INOX_APP_NAME + "/" + SHELL_STARTUP_SCRIPT_NAME
	STARTUP_SCRIPT_PERM       = 0o700

	DEFAULT_TRUSTED_RISK_SCORE = core.MEDIUM_RISK_SCORE_LEVEL - 1

	//
	//	===== METRICS & PERF =====
	//

	METRICS_PERF_ENV_VAR_NAME = "METRICS_PERF_BUCKET"
)

var (
	//go:embed default_startup.ix
	DEFAULT_STARTUP_SCRIPT_CODE string
	USER_HOME                   string

	//
	//	===== METRICS & PERF =====
	//

	METRICS_PERF_BUCKET_NAME       string
	METRICS_PERF_BUCKET_ACCESS_KEY string
	METRICS_PERF_BUCKET_ENDPOINT   core.Host
	METRICS_PERF_BUCKET_SECRET_KEY *core.Secret

	//
	//	===== OUTPUT =====
	//

	FORCE_COLOR           bool
	TRUECOLOR_COLORTERM   bool
	TERM_256COLOR_CAPABLE bool
	NO_COLOR              bool
	SHOULD_COLORIZE       bool

	// set if SHOULD_COLORIZE
	INITIAL_COLORS_SET bool
	INITIAL_FG_COLOR   core.Color
	INITIAL_BG_COLOR   core.Color

	DEFAULT_LOG_PRINT_CONFIG = &core.PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 10,
			Colorize: false,
			Compact:  true,
		},
	}

	STR_CONVERSION_PRETTY_PRINT_CONFIG = &core.PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 10,
			Colorize: false,
			Compact:  true,
		},
	}

	DEFAULT_PRETTY_PRINT_CONFIG *core.PrettyPrintConfig
)

func init() {
	targetSpecificInit()
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.Cancel()

	//
	//	===== OUTPUT =====
	//

	DEFAULT_PRETTY_PRINT_CONFIG = &core.PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 7,
			Colorize: SHOULD_COLORIZE,
			Colors: utils.If(INITIAL_COLORS_SET && INITIAL_BG_COLOR.IsDarkBackgroundColor(),
				&pprint.DEFAULT_DARKMODE_PRINT_COLORS,
				&pprint.DEFAULT_LIGHTMODE_PRINT_COLORS,
			),
			Compact:                     false,
			Indent:                      []byte{' ', ' '},
			PrintDecodedTopLevelStrings: true,
		},
	}

	//
	//	===== METRICS & PERF =====

	bucketConfig, ok := os.LookupEnv(METRICS_PERF_ENV_VAR_NAME)
	os.Unsetenv(METRICS_PERF_ENV_VAR_NAME)
	if ok && bucketConfig != "" {
		config := struct {
			Name      string `json:"name"`
			Endpoint  string `json:"endpoint"`
			AccessKey string `json:"accessKey"`
			SecretKey string `json:"secretKey"`
		}{}
		if err := json.Unmarshal([]byte(bucketConfig), &config); err != nil {
			panic(fmt.Errorf("failed to unmarshal configuration of metrics-perf bucket: %w", err))
		}

		if config.Name == "" {
			panic(errors.New("empty/missing name for metrics-perf bucket"))
		}

		if config.Endpoint == "" {
			panic(errors.New("empty/missing endpoint for metrics-perf bucket"))
		}

		if config.AccessKey == "" {
			panic(errors.New("empty/missing access key for metrics-perf bucket"))
		}

		if config.SecretKey == "" {
			panic(errors.New("empty/missing secret key for metrics-perf bucket"))
		}

		METRICS_PERF_BUCKET_NAME = config.Name
		METRICS_PERF_BUCKET_ENDPOINT = core.Host(config.Endpoint)
		METRICS_PERF_BUCKET_ACCESS_KEY = config.AccessKey
		METRICS_PERF_BUCKET_SECRET_KEY = utils.Must(core.SECRET_STRING_PATTERN.NewSecret(ctx, config.SecretKey))
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
