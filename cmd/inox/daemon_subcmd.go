package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/inoxprocess/binary"
	"github.com/rs/zerolog"
)

func Inoxd(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	//read & check arguments
	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var configOrConfigFile string

	flags.StringVar(&configOrConfigFile, "config", "", "JSON configuration or JSON file")

	if showHelp(flags, mainSubCommandArgs, outW) { //only show help
		return
	}

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(errW, "daemon:", err)
		return
	}

	var daemonConfig inoxd.DaemonConfig

	configOrConfigFile = strings.TrimSpace(configOrConfigFile)
	if configOrConfigFile != "" {
		if configOrConfigFile[0] == '{' {
			err := json.Unmarshal([]byte(configOrConfigFile), &daemonConfig)
			if err != nil {
				fmt.Fprintln(errW, "daemon: failed to unmarshal configuration argument", err)
				return ERROR_STATUS_CODE
			}
		} else {
			content, err := os.ReadFile(configOrConfigFile)
			if err != nil {
				fmt.Fprintln(errW, "daemon: failed to read configuration file:", err)
				return ERROR_STATUS_CODE
			}
			err = json.Unmarshal(content, &daemonConfig)
			if err != nil {
				fmt.Fprintln(errW, "daemon: failed to unmarshal configuration file:", err)
				return ERROR_STATUS_CODE
			}
		}
	}

	daemonConfig.InoxBinaryPath = binary.INOX_BINARY_PATH

	inoxd.Inoxd(inoxd.InoxdArgs{
		Config: daemonConfig,
		GoCtx:  context.Background(),
		Logger: zerolog.New(errW),
	})

	return 0
}
